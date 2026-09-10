"""Reopen and package online migration preparation evidence without secrets."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time

from rehearse_cdc import ROOT,run,record
from prepare_live_shadow import production

SOURCE=Path('/opt/new-api-src/current')


def sha(path):
    with path.open('rb') as f:return hashlib.file_digest(f,'sha256').hexdigest()


def main():
    os.umask(0o077)
    pause=SOURCE/'scripts/pg-rehearsal/pause-shadow.sh'
    resume=SOURCE/'scripts/pg-rehearsal/resume-shadow.sh'
    for p in (pause,resume):p.chmod(0o755)
    run(['sh',str(pause)])
    run(['sh',str(resume)])
    instance=production()
    for name in ('new-api-migration-ready-sqlite','new-api-migration-ready-postgres','new-api-pg-rehearsal-sqlite','new-api-pg-rehearsal-postgres'):
        item=json.loads(subprocess.check_output(['docker','inspect',name]))[0]
        assert not item['State']['Running'],name
    run(['git','-C',str(SOURCE),'diff','--check'])
    run(['git','-C',str(SOURCE),'status','--short','--branch'])
    command=['git','-C',str(SOURCE),'diff','--binary','f0653f82bd70b246eb96dfb241fb4b93017984dd','HEAD']
    result=subprocess.run(command,text=True,capture_output=True,check=True)
    patch=result.stdout
    record({'command':command,'stdout':patch,'stderr':result.stderr,'exit_status':result.returncode})
    (ROOT/'complete.patch').write_text(patch)
    run(['git','-C',str(SOURCE),'apply','--reverse','--check',str(ROOT/'complete.patch')])
    files=run(['git','-C',str(SOURCE),'diff','--name-only','f0653f82bd70b246eb96dfb241fb4b93017984dd','HEAD']).splitlines()
    for relative in files:
        path=SOURCE/relative
        dest=ROOT/'modified'/relative
        dest.parent.mkdir(parents=True,exist_ok=True);shutil.copy2(path,dest)
    shutil.copy2(pause,ROOT/'rollback.sh')
    shutil.copy2(resume,ROOT/'resume.sh')
    state=json.loads((ROOT/'live-status.json').read_text())
    current={'utc':time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime()),
             'production_image':instance['Config']['Image'],'production_started':instance['State']['StartedAt'],
             'production_restarts':instance['RestartCount'],'shadow_status':state,'cutover_performed':False,
             'candidate_deployed':False,'public_routing_changed':False,
             'tooling_commit':(ROOT/'tooling-commit.txt').read_text().strip(),
             'candidate_commit':(ROOT/'candidate-commit.txt').read_text().strip()}
    (ROOT/'final-online-state.json').write_text(json.dumps(current,indent=2)+'\n')
    record({'final_online_state':current})
    lines=['Online migration preparation; production remains SQLite.','All paths and statements below refer to observed results.','']
    for raw in (ROOT/'verification.jsonl').read_text().splitlines():
        event=json.loads(raw)
        if 'command' in event:
            lines.extend(['COMMAND '+repr(event['command']),'STDOUT',event.get('stdout',''),
                          'STDERR',event.get('stderr',''),'EXIT '+str(event.get('exit_status')),''])
        else:lines.extend([json.dumps(event,ensure_ascii=True),''])
    (ROOT/'verification.txt').write_text('\n'.join(lines))
    include=['complete.patch','verification.txt','verification.jsonl','rollback.sh','resume.sh',
             'production-baseline.json','full-cdc-verification.json','live-baseline-verification.json',
             'live-reconciliation-verification.json','candidate-verification.json','final-online-state.json',
             'newapi-sqlite-shadow.service','tooling-commit.txt','candidate-commit.txt',
             'migration-gate-c8d0ee7','new-api-readiness-'+current['candidate_commit'][:7],
             'original/live-before-capture.db','live-baseline.db','live-reconciliation.db']
    paths=[ROOT/s for s in include]
    paths.extend(p for p in (ROOT/'modified').rglob('*') if p.is_file())
    manifest=''.join(sha(path)+'  '+str(path.relative_to(ROOT))+'\n' for path in sorted(paths))
    (ROOT/'checksums.sha256').write_text(manifest)
    result=subprocess.run(['sha256sum','-c','checksums.sha256'],cwd=ROOT,text=True,capture_output=True)
    print(result.stdout,end='',flush=True)
    assert result.returncode==0,result.stderr
    print('ONLINE PREPARATION VERIFIED; shadow active; source and ingress unchanged',flush=True)


if __name__=='__main__':main()
