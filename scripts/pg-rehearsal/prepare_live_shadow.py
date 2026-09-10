"""Prepare a live shadow with no production restarts or traffic changes."""
import hashlib
import json
import os
from pathlib import Path
import sqlite3
import subprocess
import time

import cdc
from rehearse_cdc import ROOT, record, run

REPO = Path('/opt/new-api-src/current')
PYTHON = '/opt/new-api/backups/20260910-postgres-rehearsal/venv/bin/python'
SERVICE = 'newapi-sqlite-shadow'


def production():
    info=json.loads(subprocess.check_output(['docker','inspect','new-api']))[0]
    baseline=json.loads((ROOT/'production-baseline.json').read_text())
    assert info['Id']==baseline['container_id'] and info['State']['StartedAt']==baseline['started']
    assert info['Image']==baseline['image']
    assert hashlib.sha256(Path('/opt/new-api/docker-compose.yml').read_bytes()).hexdigest()==baseline['compose_sha256']
    for path,want in [('/api/status','200'),('/v1/models','401')]:
        output=run(['curl','-sS','--max-time','10','-o','/dev/null','-w','%{http_code}','http://127.0.0.1:3000'+path])
        assert output==want
    return info


def prepare():
    assert json.loads((ROOT/'full-cdc-verification.json').read_text())['passed']
    if (ROOT/'live-config.json').exists():
        raise RuntimeError('Live preparation already exists; inspect current state before continuing')
    instance=production()
    path='/opt/new-api/data/one-api.db'
    original=ROOT/'original/live-before-capture.db'
    assert not original.exists()
    start=time.monotonic()
    with cdc.source(path,True) as src:
        src.execute('BEGIN'); src.execute('SELECT count(*) FROM sqlite_schema').fetchone()
        (ROOT/'original/schema-before.sql').write_text('\n'.join(r[0]+';' for r in src.execute('SELECT sql FROM sqlite_schema WHERE sql IS NOT NULL ORDER BY name')))
        with sqlite3.connect(original) as dst:
            def progress(*_):
                if time.monotonic()-start>120:raise TimeoutError('Backup deadline')
            src.backup(dst,pages=1024,progress=progress,sleep=.01)
        src.rollback()
    original.chmod(0o400)
    with original.open('rb') as f: sha=hashlib.file_digest(f,'sha256').hexdigest()
    record({'live_backup':str(original),'sha256':sha,'seconds':time.monotonic()-start})
    config={'source':path,'socket':'/opt/new-api/backups/20260910-postgres-rehearsal/socket',
            'database':'newapi_sync_live','source_container':'new-api','container_id':instance['Id'],
            'container_started':instance['State']['StartedAt'],'status_file':str(ROOT/'live-status.json'),
            'interval':2,'max_dirty_keys':200000}
    run(['docker','exec','new-api-pg-rehearsal-db','psql','-X','-v','ON_ERROR_STOP=1','-U','rehearsal','-d','postgres',
         '-c','CREATE DATABASE newapi_sync_live TEMPLATE newapi_import;'])
    result=cdc.install(path)
    config['generation']=result['generation']
    (ROOT/'live-config.json').write_text(json.dumps(config,indent=2)+'\n')
    record({'live_capture_install':result});print('CAPTURE '+json.dumps(result),flush=True)
    production()
    result=cdc.snapshot(path,ROOT/'live-baseline.db');record({'live_snapshot':result});print('SNAPSHOT '+json.dumps(result),flush=True)
    result=cdc.bootstrap(config,ROOT/'live-baseline.db');record({'live_bootstrap':result})
    baseline_config={**config,'source':str(ROOT/'live-baseline.db')}
    result=cdc.verify(baseline_config)
    (ROOT/'live-baseline-verification.json').write_text(json.dumps(result,indent=2)+'\n')
    record({'live_baseline_verification':result})
    result=cdc.sync_once(config);record({'live_first_sync':result});print('FIRST LIVE SYNC '+json.dumps(result),flush=True)
    unit='''[Unit]
Description=New API SQLite to isolated PostgreSQL shadow
After=docker.service
Requires=docker.service

[Service]
Type=simple
User=root
UMask=0077
WorkingDirectory=/opt/new-api-src/current/scripts/pg-rehearsal
ExecStart='''+PYTHON+''' /opt/new-api-src/current/scripts/pg-rehearsal/cdc.py watch --config '''+str(ROOT/'live-config.json')+'''
Restart=on-failure
RestartSec=5
Nice=15
IOSchedulingClass=best-effort
IOSchedulingPriority=7
MemoryMax=1G
CPUQuota=100%
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
'''
    service=Path('/etc/systemd/system/'+SERVICE+'.service')
    assert not service.exists()
    service.write_text(unit)
    (ROOT/'newapi-sqlite-shadow.service').write_text(unit)
    run(['systemctl','daemon-reload'])
    run(['systemctl','start',SERVICE])
    # Deliberately not enabled at boot: application restarts invalidate capture.
    for _ in range(30):
        status=ROOT/'live-status.json'
        if status.exists():
            current=json.loads(status.read_text())
            if current.get('ok'):break
        time.sleep(1)
    else:raise RuntimeError('Watcher did not become healthy')
    production()
    run(['systemctl','is-active',SERVICE])
    print('LIVE SHADOW ACTIVE '+json.dumps(current),flush=True)


def rollback():
    cfg=json.loads((ROOT/'live-config.json').read_text())
    run(['systemctl','stop',SERVICE])
    # Refuse to remove unknown/rebuilt capture. Inspect any failed assertion.
    cdc.uninstall(cfg['source'],cfg['generation'])
    production()
    record({'live_capture_rollback':'PASS','data_retained':True,'production_restarted':False})
    print('Shadow watcher stopped and capture removed; production kept running',flush=True)


if __name__=='__main__':
    import argparse
    os.umask(0o077)
    parser=argparse.ArgumentParser();parser.add_argument('action',choices=['prepare','rollback'])
    globals()[parser.parse_args().action]()
