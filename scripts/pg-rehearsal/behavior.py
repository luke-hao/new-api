"""Exercise the same immutable application image on two isolated database copies."""

import argparse
import concurrent.futures
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import secrets
import sqlite3
import subprocess
import threading
import time
import urllib.error
import urllib.request

import psycopg
from psycopg import sql

from ops import ROOT, APP_PREFIX, owned, record, save

MODEL = 'migration-rehearsal'


def database(kind):
    if kind == 'sqlite':
        return sqlite3.connect(ROOT / 'sqlite-data/one-api.db', timeout=10)
    return psycopg.connect(host=str(ROOT / 'socket'), user='rehearsal', dbname='newapi_test')


def execute(db, query, params=()):
    return db.execute(query.replace('?', '%s') if isinstance(db, psycopg.Connection) else query, params)


def fixture():
    assert not (ROOT / 'fixture.json').exists()
    values = {'api_key': secrets.token_hex(24), 'admin_key': secrets.token_hex(14), 'model': MODEL}
    for kind in ('sqlite', 'postgres'):
        with database(kind) as db:
            execute(db, 'UPDATE channels SET status=2')
            execute(db, 'UPDATE abilities SET enabled=?', (False,))
            ids = {t: execute(db, 'SELECT coalesce(max(id),0)+1 FROM ' + t).fetchone()[0]
                   for t in ('users', 'tokens', 'channels')}
            if 'ids' in values:
                assert values['ids'] == ids
            else:
                values['ids'] = ids
            execute(db, '''INSERT INTO users
                (id,username,password,display_name,role,status,email,quota,used_quota,request_count,
                 "group",aff_code,aff_count,aff_quota,aff_history,inviter_id,setting,access_token)
                VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)''',
                (ids['users'], 'pg_rehearsal', 'not-a-login-password', 'PG rehearsal', 100, 1, '',
                 100000000, 0, 0, 'default', 'pg_rehearsal', 0, 0, 0, 0, '{}', values['admin_key']))
            execute(db, '''INSERT INTO tokens
                (id,user_id,"key",status,name,created_time,accessed_time,expired_time,remain_quota,
                 unlimited_quota,model_limits_enabled,model_limits,allow_ips,used_quota,"group",cross_group_retry)
                VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)''',
                (ids['tokens'], ids['users'], values['api_key'], 1, 'PG rehearsal', int(time.time()),
                 0, -1, 100000000, False, False, '', '', 0, '', False))
            execute(db, '''INSERT INTO channels
                (id,type,"key",status,name,weight,base_url,models,"group",used_quota,priority,auto_ban,
                 other_info,setting,settings,channel_info)
                VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)''',
                (ids['channels'], 1, 'synthetic-upstream-key', 1, 'PG rehearsal mock', 100,
                 'http://127.0.0.1:18081', MODEL, 'default', 0, 0, 0, '{}', '{}', '{}',
                 b'{}' if kind == 'sqlite' else '{}'))
            execute(db, '''INSERT INTO abilities ("group",model,channel_id,enabled,priority,weight)
                           VALUES (?,?,?,?,?,?)''', ('default', MODEL, ids['channels'], True, 0, 100))
            for key, entry, val in [('ModelRatio', MODEL, 1), ('CompletionRatio', MODEL, 1), ('GroupRatio', 'default', 1)]:
                row = execute(db, 'SELECT value FROM options WHERE "key"=?', (key,)).fetchone()
                mapping = json.loads(row[0]) if row and row[0] else {}
                mapping[entry] = val
                execute(db, 'INSERT INTO options ("key",value) VALUES (?,?) ON CONFLICT("key") DO UPDATE SET value=excluded.value',
                        (key, json.dumps(mapping)))
            db.commit()
    save('fixture.json', values)
    print('FIXTURE READY: synthetic user/token/channel in both test copies', flush=True)


class Mock(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
        if any(m.get('content') == 'FAIL' for m in body.get('messages', [])):
            output = json.dumps({'error': {'message': 'synthetic upstream error', 'type': 'server_error', 'code': 'rehearsal_error'}}).encode()
            self.send_response(500)
            self.send_header('Content-Type', 'application/json')
        elif body.get('stream'):
            chunks = [
                {'id': 'chatcmpl-rehearsal', 'object': 'chat.completion.chunk', 'created': 1, 'model': MODEL,
                 'choices': [{'index': 0, 'delta': {'role': 'assistant', 'content': 'rehearsal-ok'}, 'finish_reason': None}]},
                {'id': 'chatcmpl-rehearsal', 'object': 'chat.completion.chunk', 'created': 1, 'model': MODEL,
                 'choices': [{'index': 0, 'delta': {}, 'finish_reason': 'stop'}],
                 'usage': {'prompt_tokens': 10, 'completion_tokens': 5, 'total_tokens': 15}},
            ]
            output = (''.join('data: ' + json.dumps(chunk) + '\n\n' for chunk in chunks) + 'data: [DONE]\n\n').encode()
            self.send_response(200)
            self.send_header('Content-Type', 'text/event-stream')
        else:
            output = json.dumps({'id': 'chatcmpl-rehearsal', 'object': 'chat.completion', 'created': 1,
                                 'model': MODEL, 'choices': [{'index': 0, 'message': {'role': 'assistant',
                                 'content': 'rehearsal-ok'}, 'finish_reason': 'stop'}],
                                 'usage': {'prompt_tokens': 10, 'completion_tokens': 5, 'total_tokens': 15}}).encode()
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(output)))
        self.end_headers()
        self.wfile.write(output)


def client(kind):
    fixture_data = json.loads((ROOT / 'fixture.json').read_text())
    events = []
    lock = threading.Lock()
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))

    def request(path, auth=None, body=None, expected=200):
        headers = {'Content-Type': 'application/json', 'Accept-Language': 'en'}
        if auth == 'admin':
            headers.update({'Authorization': 'Bearer ' + fixture_data['admin_key'],
                            'New-Api-User': str(fixture_data['ids']['users'])})
        elif auth == 'api':
            headers['Authorization'] = 'Bearer sk-' + fixture_data['api_key']
        data = json.dumps(body).encode() if body is not None else None
        req = urllib.request.Request('http://127.0.0.1:3000' + path, data=data, headers=headers)
        started = time.monotonic()
        try:
            response = opener.open(req, timeout=30)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            output = response.read()
            status = response.status
        event = {'kind': kind, 'method': req.get_method(), 'path': path, 'auth': auth,
                 'body': body, 'status': status, 'elapsed_ms': round((time.monotonic() - started)*1000, 2),
                 'response_bytes': len(output), 'response_sha256': hashlib.sha256(output).hexdigest()}
        if path == '/v1/chat/completions' or status != expected:
            event['response'] = output.decode('utf-8', errors='replace')
        with lock:
            events.append(event)
            record({'behavior_http': event})
        assert status == expected, kind + ' ' + path + ' HTTP ' + str(status)
        if body and body.get('stream'):
            assert b'[DONE]' in output and b'rehearsal-ok' in output
        elif expected == 200:
            payload = json.loads(output)
            assert payload.get('success', True) is not False, kind + ' ' + path + ' success=false'
            if path == '/v1/chat/completions':
                assert payload['choices'][0]['message']['content'] == 'rehearsal-ok'
        return output

    server = ThreadingHTTPServer(('127.0.0.1', 18081), Mock)
    worker = threading.Thread(target=server.serve_forever, daemon=True)
    worker.start()
    try:
        request('/api/status')
        request('/v1/models', expected=401)
        request('/api/user/self', auth='admin')
        request('/api/token/?p=1&page_size=5', auth='admin')
        request('/api/log/?p=1&page_size=5', auth='admin')
        request('/api/channel/?p=1&page_size=5', auth='admin')
        request('/v1/models', auth='api')
        body = {'model': MODEL, 'messages': [{'role': 'user', 'content': 'rehearsal'}], 'max_tokens': 16}
        request('/v1/chat/completions', auth='api', body=body)
        request('/v1/chat/completions', auth='api', body={**body, 'stream': True, 'stream_options': {'include_usage': True}})
        with concurrent.futures.ThreadPoolExecutor(max_workers=5) as pool:
            jobs = [pool.submit(request, '/v1/chat/completions', 'api', body) for _ in range(10)]
            for job in jobs:
                job.result()
        request('/v1/chat/completions', auth='api', body={**body, 'messages': [{'role': 'user', 'content': 'FAIL'}]}, expected=500)
    finally:
        server.shutdown()
        server.server_close()
        save('http-' + kind + '.json', events)
    print('HTTP ' + kind + ' PASS: auth, admin reads, normal/SSE/concurrent relay, upstream failure', flush=True)


def balances(kind):
    f = json.loads((ROOT / 'fixture.json').read_text())
    with database(kind) as db:
        user = execute(db, 'SELECT quota,used_quota,request_count FROM users WHERE id=?', (f['ids']['users'],)).fetchone()
        token = execute(db, 'SELECT remain_quota,used_quota FROM tokens WHERE id=?', (f['ids']['tokens'],)).fetchone()
        channel = execute(db, 'SELECT used_quota FROM channels WHERE id=?', (f['ids']['channels'],)).fetchone()
        logs = execute(db, 'SELECT type,quota,prompt_tokens,completion_tokens FROM logs WHERE user_id=? AND type=2 ORDER BY id', (f['ids']['users'],)).fetchall()
        return {'user': list(user), 'token': list(token), 'channel': list(channel), 'logs': [list(r) for r in logs]}


def check():
    results = {}
    for kind in ('sqlite', 'postgres'):
        name = APP_PREFIX + kind
        pid = owned(name)['State']['Pid']
        command = ['nsenter', '-t', str(pid), '-n', str(ROOT / 'venv/bin/python'), str(Path(__file__).resolve()), 'client', '--kind', kind]
        process = subprocess.run(command, text=True, capture_output=True, timeout=180)
        record({'command': command, 'stdout': process.stdout, 'stderr': process.stderr, 'exit_status': process.returncode})
        print(process.stdout, end='', flush=True)
        if process.returncode:
            print(process.stderr, flush=True)
            raise RuntimeError(kind + ' behavior checks failed')
        for _ in range(20):
            result = balances(kind)
            if result['user'][2] == 12 and len(result['logs']) == 12:
                break
            time.sleep(1)
        results[kind] = result
        assert result['user'] == [99999820, 180, 12], result['user']
        assert result['token'] == [99999820, 180], result['token']
        assert result['channel'] == [180], result['channel']
        assert result['logs'] == [[2, 15, 10, 5]] * 12, result['logs']
    assert results['sqlite'] == results['postgres']
    save('behavior-verification.json', {'passed': True, 'results': results,
                                       'expected_successes_per_engine': 12, 'expected_quota_each': 15,
                                       'failed_upstream_charge': 0, 'synthetic_only': True})
    print('BILLING PARITY PASS: 12 requests * 15 quota; failed request refunded; all ledgers match', flush=True)


if __name__ == '__main__':
    os.umask(0o077)
    parser = argparse.ArgumentParser()
    parser.add_argument('action', choices=['fixture', 'check', 'client'])
    parser.add_argument('--kind', choices=['sqlite', 'postgres'])
    args = parser.parse_args()
    if args.action == 'client':
        client(args.kind)
    else:
        globals()[args.action]()
