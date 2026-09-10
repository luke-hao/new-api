"""Exercise frozen inspection against the isolated legacy rollback fixture."""
import json,subprocess,sys,time
from pathlib import Path
from legacy_state import inspect
from legacy_ready import validate
ROOT=Path("/opt/new-api/backups/pg-cutover-20260910T115338Z")
NAME="new-api-cutover-check-reverse"
def sample(pid):
    subprocess.run(["docker","pause",NAME],check=True,stdout=subprocess.DEVNULL)
    try:r=inspect(pid);v=validate(r);return r,v
    finally:subprocess.run(["docker","unpause",NAME],check=True,stdout=subprocess.DEVNULL)
def main():
    item=json.loads(subprocess.check_output(["docker","inspect",NAME]))[0]
    assert item["HostConfig"]["NetworkMode"]=="none"
    pid=item["State"]["Pid"]
    code=r"""
import sys,time,json,threading,urllib.request,urllib.error
from pathlib import Path
sys.path.insert(0,'/opt/new-api-src/current/scripts/pg-rehearsal')
import behavior as b
f=json.loads(Path('/opt/new-api/backups/pg-cutover-20260910T115338Z/image-parity/fixture.json').read_text())
class Slow(b.Mock):
 def do_POST(self):
  marker=Path('/opt/new-api/backups/pg-cutover-20260910T115338Z/legacy-test-entered')
  if not marker.exists():
   marker.write_text('1'); time.sleep(5)
  super().do_POST()
s=b.ThreadingHTTPServer(('127.0.0.1',18081),Slow)
threading.Thread(target=s.serve_forever,daemon=True).start()
body={'model':b.MODEL,'messages':[{'role':'user','content':'FAIL'}],'max_tokens':16}
req=urllib.request.Request('http://127.0.0.1:3000/v1/chat/completions',data=json.dumps(body).encode(),headers={'Authorization':'Bearer sk-'+f['api_key'],'Content-Type':'application/json'})
try:urllib.request.urlopen(req,timeout=45)
except urllib.error.HTTPError as e:assert e.code==500
else:raise AssertionError('expected mock failure')
s.shutdown()
"""
    marker=ROOT/"legacy-test-entered"
    marker.unlink(missing_ok=True)
    proc=subprocess.Popen(["nsenter","-t",str(pid),"-n","/opt/new-api/backups/20260910-postgres-rehearsal/venv/bin/python","-c",code],stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
    for _ in range(100):
        if marker.exists():break
        if proc.poll() is not None:raise RuntimeError(proc.communicate()[1])
        time.sleep(.1)
    else:raise RuntimeError("mock not entered")
    busy,blocked=sample(pid)
    assert not blocked["passed"] and busy["http_handlers"]>0
    out,err=proc.communicate(timeout=60);assert proc.returncode==0,err
    for _ in range(15):
        idle,ready=sample(pid)
        if ready["passed"]:break
        time.sleep(2)
    else:raise AssertionError(ready)
    report={"passed":True,"active_request_rejected":blocked,"idle_request_ready":ready,"idle_queues":{k:idle[k] for k in ("batch_entries","dashboard_entries","pool_task_count")}}
    (ROOT/"legacy-drain-verification.json").write_text(json.dumps(report,indent=2)+"\n")
    print(json.dumps(report,indent=2))
if __name__=="__main__":main()
