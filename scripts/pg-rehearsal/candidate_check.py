"""Run the committed candidate binary with isolated SQLite/PostgreSQL fixtures."""
import argparse
import json
import os
from pathlib import Path
import sqlite3
import subprocess
import time

import psycopg
import behavior as b
from rehearse_cdc import ROOT, run, record

OLD=Path('/opt/new-api/backups/20260910-postgres-rehearsal')
STAGE=ROOT/'candidate'
PREFIX='new-api-migration-ready-'


def database(kind):
    if kind=='sqlite':return sqlite3.connect(STAGE/'sqlite-data/one-api.db',timeout=10)
    return psycopg.connect(host=str(OLD/'socket'),user='rehearsal',dbname='newapi_sync_candidate')


b.ROOT=STAGE
b.database=database
b.record=record
b.save=lambda name,obj:(STAGE/name).write_text(json.dumps(obj,indent=2)+'\n')


def initialize(binary):
    STAGE.mkdir(mode=0o700)
    (STAGE/'sqlite-data').mkdir(mode=0o700)
    run(['cp','--reflink=auto',str(OLD/'original/one-api.snapshot.db'),str(STAGE/'sqlite-data/one-api.db')])
    (STAGE/'sqlite-data/one-api.db').chmod(0o600)
    run(['docker','exec','new-api-pg-rehearsal-db','psql','-X','-v','ON_ERROR_STOP=1','-U','rehearsal','-d','postgres',
         '-c','CREATE DATABASE newapi_sync_candidate TEMPLATE newapi_import;'])
    b.fixture()
    f=json.loads((STAGE/'fixture.json').read_text())
    for kind in ('sqlite','postgres'):
        with database(kind) as db:
            b.execute(db,'UPDATE users SET quota=1000000 WHERE id=?',(f['ids']['users'],))
            b.execute(db,'UPDATE tokens SET remain_quota=1000000 WHERE id=?',(f['ids']['tokens'],))
            db.commit()
        control=STAGE/('control-'+kind);control.mkdir(mode=0o700)
        data=STAGE/(kind+'-data');data.mkdir(exist_ok=True,mode=0o700)
        env=json.loads((OLD/'app-env.json').read_text())
        env['BATCH_UPDATE_INTERVAL']='60'
        env['MIGRATION_CONTROL_SOCKET']='/control/control.sock'
        if kind=='sqlite':env['SQLITE_PATH']='/data/one-api.db?_busy_timeout=30000'
        else:env['SQL_DSN']='postgresql://rehearsal@/newapi_sync_candidate?host=/var/run/postgresql&sslmode=disable'
        envfile=STAGE/(kind+'.env');envfile.write_text(''.join(k+'='+v+'\n' for k,v in env.items()))
        image=json.loads((OLD/'state.json').read_text())['app_image']
        run(['docker','run','-d','--name',PREFIX+kind,'--label','newapi.migration-candidate=20260910','--network','none',
             '--cpus','1','--memory','2g','--pids-limit','256','--env-file',str(envfile),
             '-v',str(data)+':/data','-v',str(control)+':/control','-v',str(OLD/'socket')+':/var/run/postgresql',
             '-v',str(Path(binary).resolve())+':/candidate-new-api:ro','--entrypoint','/candidate-new-api',image,'--log-dir','/data/logs'])


def check():
    results={}
    try:
        for kind in ('sqlite','postgres'):
            name=PREFIX+kind
            instance=json.loads(subprocess.check_output(['docker','inspect',name]))[0]
            assert instance['HostConfig']['NetworkMode']=='none'
            assert instance['Config']['Labels']['newapi.migration-candidate']=='20260910'
            pid=instance['State']['Pid']
            for _ in range(30):
                result=subprocess.run(['nsenter','-t',str(pid),'-n','curl','-sS','--max-time','1','-o','/dev/null','-w','%{http_code}',
                                       'http://127.0.0.1:3000/api/status'],capture_output=True,text=True)
                if result.stdout=='200':break
                time.sleep(1)
            else:raise RuntimeError('candidate not ready')
            run(['nsenter','-t',str(pid),'-n',str(OLD/'venv/bin/python'),str(Path(__file__).resolve()),'client','--kind',kind],timeout=180)
            socket=str(STAGE/('control-'+kind)/'control.sock')
            output=run(['curl','--unix-socket',socket,'-sS','--max-time','35','-X','POST','http://localhost/flush'])
            control=json.loads(output)
            assert control['batch']=={'pending_batch':False,'queued_entries':0},control
            assert control['refunds']=={'pending':0,'failed':0},control
            assert control['cutover_ready'] is False
            ledger=b.balances(kind)
            assert ledger['user']==[999820,180,12],ledger
            assert ledger['token']==[999820,180],ledger
            assert ledger['channel']==[180],ledger
            assert ledger['logs']==[[2,15,10,5]]*12,ledger
            output2=run(['curl','--unix-socket',socket,'-sS','--max-time','35','-X','POST','http://localhost/flush'])
            assert b.balances(kind)==ledger
            results[kind]={'ledger':ledger,'control':control,'idempotent_flush':True}
        assert results['sqlite']['ledger']==results['postgres']['ledger']
        report={'passed':True,'engines':results,'production_deployed':False}
        (ROOT/'candidate-verification.json').write_text(json.dumps(report,indent=2)+'\n')
        record({'candidate_verification':report})
        print('CANDIDATE PASS: 12 requests/engine, async refund drain, explicit flush, 180 quota exactly, repeated flush unchanged',flush=True)
    finally:
        for kind in ('sqlite','postgres'):
            run(['docker','stop','-t','10',PREFIX+kind])


if __name__=='__main__':
    os.umask(0o077)
    p=argparse.ArgumentParser();p.add_argument('action',choices=['initialize','check','client']);p.add_argument('--binary');p.add_argument('--kind')
    args=p.parse_args()
    if args.action=='initialize':initialize(args.binary)
    elif args.action=='client':b.client(args.kind)
    else:check()
