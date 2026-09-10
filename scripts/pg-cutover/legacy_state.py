"""Read-only drain evidence for the exact a84217d production executable.

This does not pause, alter or stop the process. A live sample is diagnostic only.
A definitive snapshot requires the Docker cgroup to be paused by the operator.
Only function names, queue lengths and state flags are emitted, never heap data.
"""
import argparse,bisect,collections,hashlib,json,os,struct
from pathlib import Path
BINARY_SHA256="8009903a587eae94058377b59ba099b814a4b8c37c20b96952f7d7744b2b61c2"

def inspect(pid):
    binary=Path(f"/proc/{pid}/exe").read_bytes()
    if hashlib.sha256(binary).hexdigest()!=BINARY_SHA256:raise ValueError("Executable differs from reviewed legacy build")
    off=0x47c7a80
    n,_,_,fn,_,_,_,pc=struct.unpack_from("<8Q",binary,off+8)
    funcs=[]
    for i in range(n):
        entry,fo=struct.unpack_from("<II",binary,off+pc+i*8)
        no=struct.unpack_from("<i",binary,off+pc+fo+4)[0];start=off+fn+no
        funcs.append((0x401000+entry,binary[start:binary.index(0,start)].decode()))
    starts=[x[0] for x in funcs]
    def name(addr):
        i=bisect.bisect_right(starts,addr)-1
        return funcs[i][1] if 0<=i<len(funcs) and addr<0x1744000 else "unknown"
    mem=os.open(f"/proc/{pid}/mem",os.O_RDONLY)
    def read(addr,n):
        data=os.pread(mem,n,addr)
        if len(data)!=n:raise ValueError("Short memory read")
        return data
    def u64(addr):return struct.unpack("<Q",read(addr,8))[0]
    def u32(addr):return struct.unpack("<I",read(addr,4))[0]
    try:
        ptr=u64(0x5b7db30);count=u64(0x5b7db38)
        if not 1<=count<=1000000:raise ValueError("Invalid goroutine list")
        rows=[];states=collections.Counter();errors=[]
        for (g,) in struct.iter_unpack("<Q",read(ptr,count*8)):
            status=u32(g+144)&~0x1000
            if status==6:continue
            states[status]+=1
            lo,hi=struct.unpack("<QQ",read(g,16));sp=u64(g+56);pcval=u64(g+64);bp=u64(g+96)
            if status==3:sp=u64(g+104);pcval=u64(g+112);bp=u64(g+120)
            stack=[name(pcval)];seen=set()
            for _ in range(160):
                if not lo<=bp<=hi-16 or bp in seen:break
                seen.add(bp);nextbp,ret=struct.unpack("<QQ",read(bp,16));stack.append(name(ret-1));bp=nextbp
            app=[s for s in stack if "QuantumNous/new-api/" in s]
            http=any("gin.(*Engine).ServeHTTP" in s or "net/http.serverHandler.ServeHTTP" in s for s in stack)
            rows.append({"id":u64(g+152),"status":status,"top":stack[:4],"stack_complete":stack[-1]=="runtime.goexit","stack_tail":stack[-2:],"application":app,"http_handler":http,"pool_worker":any("gopool.(*worker).run" in s for s in stack)})
        maps=u64(0x5b7d9f0);length=u64(0x5b7d9f8)
        assert length==5
        batch=[u64(m) if m else 0 for (m,) in struct.iter_unpack("<Q",read(maps,40))]
        locks=u64(0x5b7da10)
        cache=u64(0x5b7a5e0);pool=u64(0x5b7c748)
        result={"pid":pid,"binary_sha256":BINARY_SHA256,"batch_entries":batch,"batch_locks":[u32(locks+i*8) for i in range(5)],"dashboard_entries":u64(cache) if cache else 0,"dashboard_lock":u32(0x5ba4cb0),"pool_task_head_present":bool(u64(pool+32)),"pool_task_count":u32(pool+56),"pool_workers":u32(pool+60),"goroutine_states":dict(states),"http_handlers":sum(r["http_handler"] for r in rows),"application_goroutines":[r for r in rows if r["application"] or r["http_handler"]],"other_active":[r for r in rows if r["status"] not in (4,6) and not r["application"]],"sample_requires_external_freeze":True,"all_goroutines":rows}
        return result
    finally:os.close(mem)

if __name__=="__main__":
    p=argparse.ArgumentParser();p.add_argument("--pid",required=True,type=int);p.add_argument("--report")
    a=p.parse_args();r=inspect(a.pid);out=json.dumps(r,indent=2)
    if a.report:Path(a.report).write_text(out+"\n")
    print(json.dumps({k:v for k,v in r.items() if k not in ("application_goroutines","all_goroutines","other_active")},indent=2))
