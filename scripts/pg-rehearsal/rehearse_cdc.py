"""Full production-shaped copy test; never opens the live DB for writing."""
from concurrent.futures import ThreadPoolExecutor
import hashlib
import json
import os
from pathlib import Path
import sqlite3
import subprocess
import time

import cdc
from ops import ROOT as OLD, owned, run as oldrun

ROOT = Path('/opt/new-api/backups/20260910-online-migration')


def record(obj):
    with (ROOT / 'verification.jsonl').open('a') as f:
        f.write(json.dumps(obj) + '\n')


def run(args, timeout=300):
    p = subprocess.run(args, text=True, capture_output=True, timeout=timeout)
    record({'command':args,'stdout':p.stdout,'stderr':p.stderr,'exit_status':p.returncode})
    print(p.stdout, end='', flush=True)
    if p.returncode:
        print(p.stderr, flush=True)
        raise RuntimeError('command failed')
    return p.stdout


def main():
    os.umask(0o077)
    ROOT.mkdir(exist_ok=True)
    cfg = {'source':str(OLD/'sqlite-data/one-api.db'),'socket':str(OLD/'socket'),
           'database':'newapi_sync_rehearsal_v2'}
    app=owned('new-api-pg-rehearsal-sqlite')
    if app['State']['Running']:
        raise RuntimeError('Test app unexpectedly running before rehearsal')
    run(['docker','start','new-api-pg-rehearsal-sqlite'])
    for _ in range(30):
        app=owned('new-api-pg-rehearsal-sqlite')
        check=subprocess.run(['nsenter','-t',str(app['State']['Pid']),'-n','curl','-sS','--max-time','1','-o','/dev/null','-w','%{http_code}','http://127.0.0.1:3000/api/status'],capture_output=True,text=True)
        if check.stdout=='200': break
        time.sleep(1)
    else: raise RuntimeError('Test app startup failed')
    if not (ROOT/'rehearsal-config-v2.json').exists():
        (ROOT/'rehearsal-config-v2.json').write_text(json.dumps(cfg))
        run(['docker','exec','new-api-pg-rehearsal-db','psql','-X','-v','ON_ERROR_STOP=1','-U','rehearsal','-d','postgres',
             '-c','CREATE DATABASE newapi_sync_rehearsal_v2 TEMPLATE newapi_import;'])
        result=cdc.install(cfg['source']); record({'install':result}); print(result,flush=True)
        result=cdc.snapshot(cfg['source'],ROOT/'cdc-baseline-v2.db'); record({'snapshot':result}); print(result,flush=True)
        result=cdc.bootstrap(cfg,ROOT/'cdc-baseline-v2.db'); record({'bootstrap':result}); print(result,flush=True)
    samples=[]
    try:
        with ThreadPoolExecutor(max_workers=1) as pool:
            job=pool.submit(run,['nsenter','-t',str(app['State']['Pid']),'-n',str(OLD/'venv/bin/python'),
                                '/opt/new-api-src/current/scripts/pg-rehearsal/behavior.py','client','--kind','sqlite'],180)
            while not job.done():
                result=cdc.sync_once(cfg); samples.append(result); print('SYNC '+json.dumps(result),flush=True)
                time.sleep(.1)
            job.result()
        # This delay is only test-fixture settling, never a production drain proof.
        time.sleep(3)
    finally:
        run(['docker','stop','-t','10','new-api-pg-rehearsal-sqlite'])
    result=cdc.sync_once(cfg); samples.append(result)
    # Include primary-key changes and real deletes without touching customer rows.
    with sqlite3.connect(cfg['source']) as db:
        db.execute("INSERT INTO options(key,value) VALUES ('rehearsal_cdc_temp','a')")
        db.commit()
        db.execute("UPDATE options SET key='rehearsal_cdc_moved',value='b' WHERE key='rehearsal_cdc_temp'")
        db.commit()
    cdc.sync_once(cfg)
    with sqlite3.connect(cfg['source']) as db:
        db.execute("DELETE FROM options WHERE key='rehearsal_cdc_moved'")
        db.commit()
    cdc.sync_once(cfg)
    result=cdc.verify(cfg)
    (ROOT/'full-cdc-verification.json').write_text(json.dumps(result,indent=2)+'\n')
    record({'full_cdc':result,'poll_samples':samples})
    generation=result['generation']
    cdc.uninstall(cfg['source'],generation)
    with sqlite3.connect(cfg['source']) as db:
        remaining=db.execute("SELECT count(*) FROM sqlite_schema WHERE name LIKE '_newapi_cdc_%'").fetchone()[0]
    if remaining: raise RuntimeError('Rollback left capture objects')
    record({'capture_rollback':'PASS','production_modified':False})
    print('FULL COPY CDC AND CAPTURE ROLLBACK PASS tables='+str(len(result['tables'])),flush=True)


if __name__=='__main__':main()
