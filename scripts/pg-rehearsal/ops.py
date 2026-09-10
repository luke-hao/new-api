"""Offline PostgreSQL rehearsal operations. Production is observation-only."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import secrets
import shlex
import shutil
import sqlite3
import subprocess
import time

ROOT = Path('/opt/new-api/backups/20260910-postgres-rehearsal')
SOURCE = Path('/opt/new-api-src/current')
DB = 'new-api-pg-rehearsal-db'
LABEL = 'newapi.pg-rehearsal=20260910'
APP_PREFIX = 'new-api-pg-rehearsal-'


def record(event):
    with (ROOT / 'verification.jsonl').open('a') as f:
        f.write(json.dumps({'utc': time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime()), **event}) + '\n')


def run(args, *, input=None, check=True, quiet=False, timeout=300):
    result = subprocess.run(args, input=input, text=True, capture_output=True, timeout=timeout)
    record({'command': shlex.join(map(str, args)), 'input': input,
            'stdout': result.stdout, 'stderr': result.stderr, 'exit_status': result.returncode})
    if not quiet:
        print(result.stdout, end='', flush=True)
        if result.returncode:
            print(result.stderr, end='', flush=True)
    if check and result.returncode:
        raise RuntimeError('Command failed: ' + shlex.join(args))
    return result


def digest(path):
    with Path(path).open('rb') as f:
        return hashlib.file_digest(f, 'sha256').hexdigest()


def save(name, obj):
    (ROOT / name).write_text(json.dumps(obj, indent=2) + '\n')


def psql(query, database='newapi_import', **kwargs):
    return run(['docker', 'exec', '-i', DB, 'psql', '-X', '-qAt', '-v',
                'ON_ERROR_STOP=1', '-U', 'rehearsal', '-d', database], input=query, **kwargs).stdout.strip()


def inspect(name):
    return json.loads(subprocess.check_output(['docker', 'inspect', name]))[0]


def owned(name):
    item = inspect(name)
    assert item['Config'].get('Labels', {}).get('newapi.pg-rehearsal') == '20260910', name
    assert item['HostConfig']['NetworkMode'] == 'none', name
    assert not item['HostConfig']['PortBindings'], name
    return item


def production():
    item = inspect('new-api')
    result = {'image': item['Image'], 'tag': item['Config']['Image'],
              'started': item['State']['StartedAt'], 'restarts': item['RestartCount'],
              'compose_sha256': digest('/opt/new-api/docker-compose.yml'),
              'source_head': subprocess.check_output(['git', '-C', str(SOURCE), 'rev-parse', 'HEAD'], text=True).strip()}
    print('PRODUCTION ' + json.dumps(result), flush=True)
    record({'production': result})
    for path, expected in [('/api/status', '200'), ('/v1/models', '401')]:
        result_http = run(['curl', '-sS', '--max-time', '10', '-o', '/dev/null', '-w', '%{http_code}',
                           'http://127.0.0.1:3000' + path])
        assert result_http.stdout == expected
    if (ROOT / 'production-baseline.json').exists():
        baseline = json.loads((ROOT / 'production-baseline.json').read_text())
        for key in ('image', 'started', 'restarts', 'compose_sha256'):
            assert result[key] == baseline[key], 'Production changed: ' + key
    return result


def wait_app(name):
    for _ in range(60):
        item = owned(name)
        if not item['State']['Running']:
            run(['docker', 'logs', '--tail', '80', name], quiet=True)
            raise RuntimeError(name + ' exited; see verification.jsonl')
        response = run(['nsenter', '-t', str(item['State']['Pid']), '-n', 'curl', '-sS',
                        '--max-time', '2', '-o', '/dev/null', '-w', '%{http_code}',
                        'http://127.0.0.1:3000/api/status'], check=False, quiet=True)
        if response.returncode == 0 and response.stdout == '200':
            print(name + ' HTTP 200 (network=none)', flush=True)
            return
        time.sleep(1)
    raise TimeoutError(name)


def start_app(kind):
    name = APP_PREFIX + kind
    state = json.loads((ROOT / 'state.json').read_text())
    env = json.loads((ROOT / 'app-env.json').read_text())
    data = ROOT / (kind + '-data')
    data.mkdir(exist_ok=True)
    if kind == 'sqlite':
        env['SQLITE_PATH'] = '/data/one-api.db?_busy_timeout=30000'
    else:
        database = 'newapi_import' if kind == 'schema' else 'newapi_test'
        env['SQL_DSN'] = 'postgresql://rehearsal@/' + database + '?host=/var/run/postgresql&sslmode=disable'
    envfile = ROOT / (kind + '.env')
    envfile.write_text(''.join(k + '=' + v + '\n' for k, v in env.items()))
    run(['docker', 'run', '-d', '--name', name, '--label', LABEL, '--network', 'none',
         '--cpus', '1', '--memory', '2g', '--pids-limit', '256', '--env-file', str(envfile),
         '-v', str(ROOT / 'socket') + ':/var/run/postgresql', '-v', str(data) + ':/data',
         state['app_image'], '--log-dir', '/data/logs'])
    wait_app(name)


def prepare():
    assert not (ROOT / 'snapshot.json').exists(), 'Snapshot already prepared'
    baseline = production()
    save('production-baseline.json', baseline)
    original = ROOT / 'original'
    original.mkdir(exist_ok=True)
    shutil.copy2('/opt/new-api/docker-compose.yml', original / 'docker-compose.yml')
    env = dict(e.split('=', 1) for e in inspect('new-api')['Config']['Env'])
    stage_env = {k: env[k] for k in ('CRYPTO_SECRET', 'SESSION_SECRET', 'CANVAS_SSO_SECRET') if k in env}
    stage_env.update({'TZ': 'Asia/Shanghai', 'BATCH_UPDATE_ENABLED': 'true',
                      'BATCH_UPDATE_INTERVAL': '1', 'SQL_MAX_OPEN_CONNS': '10',
                      'SQL_MAX_IDLE_CONNS': '5', 'GIN_MODE': 'release', 'UPDATE_TASK': 'false'})
    save('app-env.json', stage_env)
    pg_image = inspect('upstreamhub-postgres')['Image']
    save('state.json', {'app_image': baseline['image'], 'pg_image': pg_image})
    started = time.monotonic()
    snapshot = original / 'one-api.snapshot.db'
    if snapshot.exists():
        snapshot.rename(original / ('one-api.incomplete-' + str(int(time.time())) + '.db'))
    with sqlite3.connect('file:/opt/new-api/data/one-api.db?mode=ro', uri=True, timeout=2) as src:
        # Pin the WAL read snapshot so concurrent commits do not restart backup.
        src.execute('BEGIN')
        src.execute('SELECT count(*) FROM sqlite_schema').fetchone()
        with sqlite3.connect(snapshot) as dst:
            def progress(status, remaining, total):
                if time.monotonic() - started > 180:
                    raise TimeoutError('Online backup exceeded 180 seconds')
            src.backup(dst, pages=1024, progress=progress, sleep=0.02)
    snapshot.chmod(0o440)
    info = {'method': 'sqlite3.Connection.backup', 'source': 'file:/opt/new-api/data/one-api.db?mode=ro',
            'path': str(snapshot), 'bytes': snapshot.stat().st_size, 'seconds': time.monotonic() - started,
            'sha256': digest(snapshot)}
    with sqlite3.connect('file:' + str(snapshot) + '?mode=ro&immutable=1', uri=True) as db:
        info['quick_check'] = [r[0] for r in db.execute('PRAGMA quick_check')]
    assert info['quick_check'] == ['ok']
    save('snapshot.json', info)
    record({'snapshot': info})
    print('SNAPSHOT ' + json.dumps(info), flush=True)
    for directory in ('socket', 'pgdata'):
        (ROOT / directory).mkdir(exist_ok=True)
    pg_env = ROOT / 'postgres.env'
    pg_env.write_text('POSTGRES_USER=rehearsal\nPOSTGRES_DB=newapi_import\nPOSTGRES_PASSWORD=' +
                      secrets.token_hex(32) + '\nPOSTGRES_INITDB_ARGS=--auth-local=trust --auth-host=scram-sha-256\n')
    run(['docker', 'run', '-d', '--name', DB, '--label', LABEL, '--network', 'none',
         '--cpus', '1', '--memory', '2g', '--shm-size', '256m', '--pids-limit', '256',
         '--env-file', str(pg_env), '-v', str(ROOT / 'pgdata') + ':/var/lib/postgresql/data',
         '-v', str(ROOT / 'socket') + ':/var/run/postgresql', pg_image,
         '-c', 'max_connections=30', '-c', 'shared_buffers=256MB'])
    for _ in range(60):
        result = run(['docker', 'exec', DB, 'pg_isready', '-U', 'rehearsal', '-d', 'newapi_import'],
                     check=False, quiet=True)
        if result.returncode == 0:
            break
        time.sleep(1)
    else:
        raise TimeoutError('PostgreSQL startup')
    start_app('schema')
    run(['docker', 'stop', '-t', '10', APP_PREFIX + 'schema'])
    schema = run(['docker', 'exec', DB, 'pg_dump', '-U', 'rehearsal', '-d', 'newapi_import',
                  '--schema-only', '--no-owner', '--no-privileges'], quiet=True).stdout
    (ROOT / 'schema.sql').write_text(schema)
    production()


def clone():
    assert json.loads((ROOT / 'data-verification.json').read_text())['passed']
    psql('CREATE DATABASE newapi_test TEMPLATE newapi_import;', database='postgres')
    data = ROOT / 'sqlite-data'
    data.mkdir(exist_ok=True)
    run(['cp', '--reflink=auto', str(ROOT / 'original/one-api.snapshot.db'), str(data / 'one-api.db')])
    (data / 'one-api.db').chmod(0o600)


def rollback():
    for kind in ('postgres', 'sqlite', 'schema', 'db'):
        name = APP_PREFIX + kind
        owned(name)
        run(['docker', 'stop', '-t', '10', name])
    production()
    print('ROLLBACK PASS: rehearsal stopped; data retained; production unchanged', flush=True)


def resume():
    for kind in ('db', 'postgres', 'sqlite'):
        name = APP_PREFIX + kind
        owned(name)
        run(['docker', 'start', name])
        if kind != 'db':
            wait_app(name)
        else:
            for _ in range(60):
                status = run(['docker', 'exec', DB, 'pg_isready', '-U', 'rehearsal', '-d', 'newapi_import'],
                             check=False, quiet=True)
                if status.returncode == 0:
                    break
                time.sleep(1)
            else:
                raise TimeoutError('PostgreSQL resume')
    production()


def main():
    os.umask(0o077)
    ROOT.mkdir(parents=True, exist_ok=True)
    ROOT.chmod(0o700)
    parser = argparse.ArgumentParser()
    parser.add_argument('action', choices=['prepare', 'clone', 'start-postgres', 'start-sqlite',
                                          'production', 'rollback', 'resume'])
    args = parser.parse_args()
    if args.action.startswith('start-'):
        start_app(args.action.split('-', 1)[1])
    else:
        globals()[args.action]()


if __name__ == '__main__':
    main()
