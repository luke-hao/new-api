"""Package a verified offline import and record production ingress checks."""

import json
import os
from pathlib import Path
import shutil
import sqlite3
import subprocess
import time
from urllib.parse import urlsplit

from ops import ROOT, SOURCE, DB, APP_PREFIX, digest, owned, production, psql, record, run, save


def ingress():
    with sqlite3.connect('file:/opt/new-api/data/one-api.db?mode=ro', uri=True, timeout=2) as db:
        base = db.execute("SELECT value FROM options WHERE key='ServerAddress'").fetchone()[0].rstrip('/')
    uri = urlsplit(base)
    assert uri.scheme == 'https' and uri.hostname and not uri.username and not uri.password
    results = []
    for via, extra in [('public_dns', []), ('local_caddy', ['--resolve', uri.hostname + ':443:127.0.0.1'])]:
        for path in ('/api/status', '/v1/models'):
            command = ['curl', '-sS', '--max-time', '20', '-o', '/dev/null', '-w', '%{http_code}', *extra, base + path]
            result = run(command, check=False)
            results.append({'via': via, 'path': path, 'http_status': result.stdout, 'exit_status': result.returncode})
    save('ingress-verification.json', results)


def dump_database():
    assert json.loads((ROOT / 'data-verification.json').read_text())['passed']
    target = ROOT / 'newapi-postgres.dump'
    assert not target.exists()
    args = ['docker', 'exec', DB, 'pg_dump', '-U', 'rehearsal', '-d', 'newapi_import',
            '-Fc', '--no-owner', '--no-privileges']
    started = time.monotonic()
    with target.open('xb') as output:
        result = subprocess.run(args, stdout=output, stderr=subprocess.PIPE, timeout=600)
    record({'command': args, 'stdout_file': str(target), 'stdout_bytes': target.stat().st_size,
            'stdout_sha256': digest(target), 'stderr': result.stderr.decode(), 'exit_status': result.returncode,
            'seconds': time.monotonic() - started})
    assert result.returncode == 0
    print('POSTGRES ARCHIVE ' + str(target.stat().st_size) + ' bytes', flush=True)
    run(['docker', 'cp', str(target), DB + ':/tmp/newapi-postgres.dump'])
    listing = run(['docker', 'exec', DB, 'pg_restore', '--list', '/tmp/newapi-postgres.dump'], quiet=True).stdout
    (ROOT / 'dump-contents.txt').write_text(listing)
    psql('CREATE DATABASE newapi_restore;', database='postgres')
    run(['docker', 'exec', DB, 'pg_restore', '--exit-on-error', '--no-owner', '--no-privileges',
         '-U', 'rehearsal', '-d', 'newapi_restore', '/tmp/newapi-postgres.dump'], timeout=600)
    tables = json.loads((ROOT / 'data-verification.json').read_text())['tables']
    query = ' UNION ALL '.join("SELECT '" + row['table'] + "', count(*) FROM \"" + row['table'] + '\"' for row in tables) + ';'
    output = psql(query, database='newapi_restore')
    counts = dict(line.split('|') for line in output.splitlines())
    for row in tables:
        assert int(counts[row['table']]) == row['rows'], row['table']
    save('restore-verification.json', {'passed': True, 'archive_sha256': digest(target),
                                      'restored_tables': len(tables), 'counts': counts})
    print('ARCHIVE RESTORE PASS: ' + str(len(tables)) + ' table counts match', flush=True)


def finalize():
    shutil.copy2(SOURCE / 'scripts/pg-rehearsal/rollback.sh', ROOT / 'rollback.sh')
    (ROOT / 'rollback.sh').chmod(0o700)
    run([str(ROOT / 'rollback.sh')])
    state = production()
    containers = []
    for kind in ('db', 'schema', 'sqlite', 'postgres'):
        item = owned(APP_PREFIX + kind)
        assert not item['State']['Running']
        containers.append({'name': item['Name'], 'network': item['HostConfig']['NetworkMode'],
                           'running': item['State']['Running'], 'image': item['Image']})
    modified = ROOT / 'modified'
    modified.mkdir(exist_ok=True)
    for path in (SOURCE / 'scripts/pg-rehearsal').iterdir():
        if path.is_file():
            shutil.copy2(path, modified / path.name)
    save('final-state.json', {'production': state, 'rehearsal_containers': containers,
                              'cutover_performed': False, 'incremental_sync_installed': False})
    events = [json.loads(line) for line in (ROOT / 'verification.jsonl').read_text().splitlines()]
    lines = ['PostgreSQL offline migration rehearsal', 'Production cutover: NOT PERFORMED',
             'Live incremental replication: NOT INSTALLED',
             'Production data and configuration were not modified.',
             'All rehearsal containers are stopped. PostgreSQL data and dump are retained.', '']
    for event in events:
        lines.append('UTC ' + event['utc'])
        if 'command' in event:
            lines.append('COMMAND ' + str(event['command']))
            if event.get('input') is not None:
                lines.extend(['STDIN', event['input']])
            lines.extend(['STDOUT', event.get('stdout', '[binary output: ' + event.get('stdout_file', '') + ']'),
                          'STDERR', event.get('stderr', ''), 'EXIT ' + str(event.get('exit_status'))])
        else:
            lines.append(json.dumps(event, ensure_ascii=True))
        lines.append('')
    (ROOT / 'verification.txt').write_text('\n'.join(lines))
    files = [p for p in ROOT.iterdir() if p.is_file() and p.name not in
             ('checksums.sha256', 'app-env.json', 'postgres.env', 'sqlite.env', 'postgres.env', 'schema.env', 'fixture.json', 'pip.pyz')]
    files += [p for p in modified.iterdir() if p.is_file()]
    files += [ROOT / 'original/one-api.snapshot.db', ROOT / 'original/docker-compose.yml']
    manifest = ''.join(digest(p) + '  ' + str(p.relative_to(ROOT)) + '\n' for p in sorted(set(files)))
    (ROOT / 'checksums.sha256').write_text(manifest)
    result = subprocess.run(['sha256sum', '-c', 'checksums.sha256'], cwd=ROOT, text=True, capture_output=True)
    print(result.stdout, end='', flush=True)
    assert result.returncode == 0, result.stderr


if __name__ == '__main__':
    import argparse
    os.umask(0o077)
    parser = argparse.ArgumentParser()
    parser.add_argument('action', choices=['ingress', 'dump_database', 'finalize'])
    globals()[parser.parse_args().action]()
