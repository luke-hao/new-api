#!/bin/sh
set -eu
/opt/new-api/backups/20260910-postgres-rehearsal/venv/bin/python - <<'PY'
import json,pathlib,subprocess,sys
sys.path.insert(0,'/opt/new-api-src/current/scripts/pg-rehearsal')
import cdc
root=pathlib.Path('/opt/new-api/backups/20260910-online-migration')
config=json.loads((root/'live-config.json').read_text())
item=json.loads(subprocess.check_output(['docker','inspect',config['source_container']]))[0]
assert item['State']['Running']
assert (item['Id'],item['State']['StartedAt'])==(config['container_id'],config['container_started'])
with cdc.source(config['source'],True) as db:
    generation,_=cdc.validate(db)
    assert generation==config['generation']
PY
systemctl start newapi-sqlite-shadow
systemctl is-active --quiet newapi-sqlite-shadow
/opt/new-api/backups/20260910-postgres-rehearsal/venv/bin/python - <<'PY'
import json,pathlib,time
p=pathlib.Path('/opt/new-api/backups/20260910-online-migration/live-status.json')
start=time.time()
for _ in range(30):
    if p.exists() and p.stat().st_mtime>=start-1:
        state=json.loads(p.read_text())
        if state.get('ok'):
            print(json.dumps(state));break
    time.sleep(1)
else:raise RuntimeError('No fresh successful sync status')
PY
