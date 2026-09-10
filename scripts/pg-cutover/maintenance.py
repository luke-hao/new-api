"""Temporary maintenance responder. Callback journals remain private on the host.
Unacknowledged payment notifications return 503, never an artificial success.
"""
import argparse,http.client,json,os,threading,time,uuid
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlsplit
CALLBACKS={"/api/user/epay/notify","/api/subscription/epay/notify","/api/stripe/webhook","/api/creem/webhook","/api/waffo/webhook"}
def callback(path):return path in CALLBACKS or path.startswith('/api/waffo-pancake/webhook/')
class Handler(BaseHTTPRequestHandler):
    protocol_version="HTTP/1.1"
    def log_message(self,*args):pass
    def serve(self):
        path=urlsplit(self.path).path
        if callback(path) and self.command in ('GET','POST'):
            size=int(self.headers.get('Content-Length','0'))
            if 0<=size<=1048576 and not self.headers.get('Transfer-Encoding'):
                self.connection.settimeout(15)
                try:body=self.rfile.read(size)
                except OSError:self.respond(503,b'retry');return
                if len(body)==size:
                    if (ROOT/'mode').read_text().strip()=='drain':
                        try:
                            conn=http.client.HTTPConnection('127.0.0.1',3000,timeout=30)
                            headers={k:v for k,v in self.headers.items() if k.lower() not in ('connection','transfer-encoding','content-length')}
                            conn.request(self.command,self.path,body,headers);response=conn.getresponse();data=response.read()
                            self.respond(response.status,data,response.getheader('Content-Type','text/plain'));conn.close();return
                        except OSError:pass
                    record={'method':self.command,'path':self.path,'headers':dict(self.headers),'body_hex':body.hex(),'received_at':time.time()}
                    name=ROOT/'callbacks'/(str(time.time_ns())+'-'+uuid.uuid4().hex+'.json')
                    with name.open('x') as f:json.dump(record,f);f.flush();os.fsync(f.fileno())
                    fd=os.open(str(name.parent),os.O_RDONLY);os.fsync(fd);os.close(fd)
        data=json.dumps({'error':{'type':'maintenance','message':'数据库维护中，请稍后重试。请求尚未受理，不产生扣费。'}},ensure_ascii=False).encode()
        self.respond(503,data,'application/json; charset=utf-8')
    def respond(self,status,body,kind='text/plain'):
        self.send_response(status);self.send_header('Content-Type',kind);self.send_header('Content-Length',str(len(body)))
        self.send_header('Cache-Control','no-store');self.send_header('Connection','close')
        if status==503:self.send_header('Retry-After','120')
        self.end_headers()
        try:self.wfile.write(body)
        except OSError:pass
        self.close_connection=True
    do_GET=serve;do_POST=serve;do_PUT=serve;do_DELETE=serve;do_PATCH=serve;do_OPTIONS=serve;do_HEAD=serve
if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--root',required=True);p.add_argument('--port',type=int,default=13001);a=p.parse_args()
    os.umask(0o077);ROOT=Path(a.root);(ROOT/'callbacks').mkdir(exist_ok=True,parents=True,mode=0o700)
    ThreadingHTTPServer(('0.0.0.0',a.port),Handler).serve_forever()
