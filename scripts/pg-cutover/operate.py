"""Site-specific maintenance operations; each destructive phase has preconditions."""
import argparse,hashlib,json,os,shutil,sqlite3,subprocess,time
from pathlib import Path
from legacy_state import inspect
from legacy_ready import validate
ROOT=Path('/opt/new-api/backups/pg-cutover-20260910T115338Z')
SOURCE=Path('/opt/new-api-src/current')
RUNTIME=Path('/opt/new-api')
CADDY=Path('/opt/reverse-proxy/Caddyfile')
LEGACY='b05864e84a477a82f13bf0917fbf4ef0b9f41f5d550a40a184eb7d40c40ea04d'
RULE=['-p','tcp','--dport','3000','-m','addrtype','--dst-type','LOCAL','-m','comment','--comment','newapi-pg-maintenance','-j','REDIRECT','--to-ports','13001']
def run(args):
    p=subprocess.run(args,capture_output=True,text=True)
    with (ROOT/'operations.jsonl').open('a') as f:f.write(json.dumps({'at':time.time(),'command':args,'exit':p.returncode,'stdout':p.stdout[-3000:],'stderr':p.stderr[-3000:]})+'\n')
    if p.returncode:raise RuntimeError('Command failed: '+repr(args)+' '+p.stderr[-600:])
    return p.stdout

def item(name='new-api'):return json.loads(run(['docker','inspect',name]))[0]
def record(name,data):(ROOT/name).write_text(json.dumps(data,indent=2)+'\n')
def mode(value):
    p=ROOT/'maintenance/mode.tmp';p.write_text(value+'\n');p.replace(ROOT/'maintenance/mode')
def fence():
    assert item()['Id']==LEGACY
    assert hashlib.sha256(CADDY.read_bytes()).hexdigest()=='9ae23791d33fd031124464b62f85996de0f2269465b68bca256a97d8b939dd52'
    (ROOT/'maintenance').mkdir(mode=0o700,exist_ok=True);mode('drain')
    run(['systemd-run','--unit=newapi-pg-maintenance','--property=Restart=on-failure','--property=UMask=0077','python3',str(SOURCE/'scripts/pg-cutover/maintenance.py'),'--root',str(ROOT/'maintenance')])
    for _ in range(30):
        p=subprocess.run(['curl','-s','-o','/dev/null','-w','%{http_code}','http://127.0.0.1:13001/api/status'],capture_output=True,text=True)
        if p.stdout=='503':break
        time.sleep(.1)
    else:raise RuntimeError('Maintenance responder not ready')
    old=CADDY.read_text();assert old.count('reverse_proxy 127.0.0.1:3000')==6
    proposed=old.replace('reverse_proxy 127.0.0.1:3000','reverse_proxy 127.0.0.1:13001')
    (ROOT/'Caddyfile.maintenance').write_text(proposed)
    CADDY.write_text(proposed)
    try:run(['docker','exec','reverse-proxy-caddy','caddy','validate','--config','/etc/caddy/Caddyfile','--adapter','caddyfile'])
    except Exception:CADDY.write_text(old);raise
    run(['docker','exec','reverse-proxy-caddy','caddy','reload','--config','/etc/caddy/Caddyfile','--adapter','caddyfile'])
    run(['iptables','-t','nat','-I','PREROUTING','1']+RULE)
    record('maintenance-start.json',{'utc':time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime()),'time':time.time(),'callback_mode':'drain'})
    print('MAINTENANCE ACTIVE; existing requests and callbacks still settle on SQLite',flush=True)

def freeze():
    assert item()['Id']==LEGACY and (ROOT/'maintenance-start.json').exists()
    mode('hold')
    run(['systemctl','stop','newapi-sqlite-shadow.service'])
    run(['systemctl','stop','ns1009363-r2-backup@latest.timer','ns1009363-r2-backup@previous.timer','ns1009363-r2-maintenance.timer'])
    pid=item()['State']['Pid'];deadline=time.monotonic()+1800;attempt=0
    while time.monotonic()<deadline:
        attempt+=1
        try:
            sample=inspect(pid);v=validate(sample)
        except (OSError,ValueError,AssertionError):time.sleep(2);continue
        if v['passed']:
            run(['docker','pause','new-api'])
            try:
                sample=inspect(pid);v=validate(sample)
                if v['passed']:
                    record('legacy-final-frozen.json',sample);record('legacy-final-ready.json',v)
                    print('FROZEN AND READY',json.dumps(v),flush=True);return
            except Exception:
                run(['docker','unpause','new-api']);raise
            run(['docker','unpause','new-api'])
        if attempt%5==1:print('DRAIN',json.dumps({'http':sample['http_handlers'],'batch':sample['batch_entries'],'dashboard':sample['dashboard_entries'],'blockers':v['blocking_reasons']}),flush=True)
        time.sleep(3)
    raise TimeoutError('Drain deadline exceeded; use abort to restore SQLite ingress')

def snapshot():
    i=item();assert i['Id']==LEGACY and i['State']['Paused']
    v=validate(inspect(i['State']['Pid']));assert v['passed'],v
    target=ROOT/'final-sqlite.db';assert not target.exists()
    src=sqlite3.connect('file:/opt/new-api/data/one-api.db?mode=ro',uri=True,timeout=10);dst=sqlite3.connect(target)
    src.backup(dst,pages=4096);assert dst.execute('PRAGMA quick_check').fetchone()[0]=='ok'
    counts={t:dst.execute('SELECT COUNT(*) FROM "'+t+'"').fetchone()[0] for t in ['users','tokens','channels','top_ups','logs']}
    dst.close();src.close();target.chmod(0o600)
    record('final-sqlite-snapshot.json',{'passed':True,'sha256':hashlib.sha256(target.read_bytes()).hexdigest(),'counts':counts})
    print('FINAL SQLITE SNAPSHOT',counts,flush=True)

def retire():
    i=item();assert i['Id']==LEGACY and i['State']['Paused']
    assert json.loads((ROOT/'final-import-verification.json').read_text())['passed']
    run(['docker','update','--restart=no','new-api'])
    run(['docker','kill','--signal=KILL','new-api'])
    run(['docker','rename','new-api','new-api-sqlite-retired-20260910'])
    print('LEGACY WRITER RETIRED',flush=True)

def restore_ingress():
    assert CADDY.read_bytes()==(ROOT/'Caddyfile.maintenance').read_bytes()
    CADDY.write_bytes((ROOT/'original/Caddyfile').read_bytes())
    run(['docker','exec','reverse-proxy-caddy','caddy','reload','--config','/etc/caddy/Caddyfile','--adapter','caddyfile'])
    run(['iptables','-t','nat','-D','PREROUTING']+RULE)
    run(['systemctl','start','ns1009363-r2-backup@latest.timer','ns1009363-r2-backup@previous.timer','ns1009363-r2-maintenance.timer'])
    run(['systemctl','stop','newapi-pg-maintenance.service'])

def resume():
    i=item();env=dict(x.split('=',1) for x in i['Config']['Env']);assert env.get('SQL_DSN','').startswith('postgres')
    assert json.loads((ROOT/'pre-open-verification.json').read_text())['passed']
    restore_ingress();record('maintenance-end.json',{'time':time.time(),'image':i['Config']['Image'],'database':'PostgreSQL'})
    print('POSTGRESQL TRAFFIC OPEN',flush=True)
def abort():
    i=item();assert i['Id']==LEGACY,'Retired writer: use the recorded rollback procedure'
    if i['State']['Paused']:run(['docker','unpause','new-api'])
    mode('drain');restore_ingress()
    run(['systemctl','start','newapi-sqlite-shadow.service'])
    print('SQLITE INGRESS RESTORED',flush=True)
if __name__=='__main__':
    os.umask(0o077);p=argparse.ArgumentParser();p.add_argument('action',choices=['fence','freeze','snapshot','retire','resume','abort']);a=p.parse_args();globals()[a.action]()
