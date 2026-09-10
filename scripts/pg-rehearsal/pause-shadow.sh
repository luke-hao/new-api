#!/bin/sh
set -eu
systemctl stop newapi-sqlite-shadow
python3 - <<'PY'
import hashlib,json,pathlib,subprocess,sqlite3
root=pathlib.Path('/opt/new-api/backups/20260910-online-migration')
baseline=json.loads((root/'production-baseline.json').read_text())
item=json.loads(subprocess.check_output(['docker','inspect','new-api']))[0]
assert item['Id']==baseline['container_id']
assert item['State']['StartedAt']==baseline['started']
assert item['State']['Running']
assert hashlib.sha256(pathlib.Path('/opt/new-api/docker-compose.yml').read_bytes()).hexdigest()==baseline['compose_sha256']
with sqlite3.connect('file:/opt/new-api/data/one-api.db?mode=ro',uri=True,timeout=2) as db:
    assert db.execute("SELECT count(*) FROM sqlite_schema WHERE name='_newapi_cdc_events'").fetchone()[0]==1
print('Shadow paused; production unchanged; capture retained for lossless resume')
PY
test "$(curl -sS --max-time 10 -o /dev/null -w '%{http_code}' http://127.0.0.1:3000/api/status)" = 200
