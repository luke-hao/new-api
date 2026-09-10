"""Create and verify a SQLite rollback copy from a quiescent PostgreSQL snapshot.
Never overwrites an existing file. The output is for a stopped application only.
"""
import argparse,datetime,decimal,hashlib,json,sqlite3,sys,time
from pathlib import Path
import psycopg
from psycopg import sql
sys.path.insert(0,str(Path(__file__).resolve().parent.parent/"pg-rehearsal"))
from migrate import add_hash,canonical
from migrate_snapshot import qi

def reverse(template,output,socket,database,report):
    dest=Path(output).resolve()
    if dest.exists():raise ValueError("Output already exists")
    baseline=sqlite3.connect(Path(template).resolve().as_uri()+"?mode=ro&immutable=1",uri=True)
    pg=psycopg.connect(host=socket,user="postgres",dbname=database)
    pg.execute("SET TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY")
    pg.execute("SET TIME ZONE 'UTC'");pg.execute("SET work_mem='4MB'")
    dst=sqlite3.connect(dest);dst.execute("PRAGMA journal_mode=DELETE");dst.execute("PRAGMA synchronous=FULL")
    schemas=list(baseline.execute("SELECT name,sql FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE '_newapi_cdc_%' ORDER BY name"))
    for table,ddl in schemas:dst.execute(ddl)
    # The upgraded application may add receipts. They contain no customer amounts.
    tables=set(r[0] for r in pg.execute("SELECT tablename FROM pg_tables WHERE schemaname='public'"))
    known={x[0] for x in schemas}
    extra=tables-known
    if extra-{"batch_quota_receipts"}:raise ValueError("Unmapped PostgreSQL tables: "+repr(extra))
    if "batch_quota_receipts" in extra:
        ddl='CREATE TABLE batch_quota_receipts (id varchar(36) PRIMARY KEY, created_at bigint)'
        dst.execute(ddl);schemas.append(("batch_quota_receipts",ddl))
    results=[];start=time.monotonic();dst.execute("BEGIN")
    for table,_ in schemas:
        info=list(dst.execute('PRAGMA table_info('+qi(table)+')'));cols=[r[1] for r in info];pk=[r[1] for r in sorted(info,key=lambda x:x[5]) if r[5]];assert pk
        meta=dict(pg.execute("SELECT column_name,data_type FROM information_schema.columns WHERE table_schema='public' AND table_name=%s",(table,)))
        if set(cols)!=set(meta):raise ValueError("Schema mismatch: "+table)
        types=[meta[c] for c in cols];declared=[r[2].lower() for r in info]
        def to_sqlite(v,t,decl):
            if v is None:return None
            if t in ('json','jsonb'):
                text=json.dumps(v,ensure_ascii=False,separators=(',',':'))
                # GORM's JSON Valuer returns []byte, and ChannelInfo.Scan expects []byte.
                return text.encode('utf-8') if decl in ('json','jsonb') else text
            if isinstance(v,bool):return int(v)
            if isinstance(v,datetime.datetime):return v.isoformat(' ')
            if isinstance(v,decimal.Decimal):return str(v)
            return canonical(v,t) if t=='character' else v
        order=sql.SQL(',').join(sql.SQL('{} COLLATE "C"').format(sql.Identifier(c)) if meta[c] in ('text','character varying','character') else sql.Identifier(c) for c in pk)
        digest=hashlib.sha256();count=0
        with pg.cursor(name='reverse_'+table) as cur:
            cur.itersize=1000;cur.execute(sql.SQL('SELECT {} FROM {} ORDER BY {}').format(sql.SQL(',').join(map(sql.Identifier,cols)),sql.Identifier(table),order))
            batch=[]
            for row in cur:
                add_hash(digest,row,types);batch.append(tuple(to_sqlite(v,t,d) for v,t,d in zip(row,types,declared)));count+=1
                if len(batch)==1000:dst.executemany('INSERT INTO '+qi(table)+' VALUES ('+','.join('?' for _ in cols)+')',batch);batch=[]
            if batch:dst.executemany('INSERT INTO '+qi(table)+' VALUES ('+','.join('?' for _ in cols)+')',batch)
        check=hashlib.sha256();seen=0
        for row in dst.execute('SELECT '+','.join(map(qi,cols))+' FROM '+qi(table)+' ORDER BY '+','.join(qi(c)+' COLLATE BINARY' for c in pk)):add_hash(check,row,types);seen+=1
        assert seen==count and digest.digest()==check.digest(),table
        results.append({'table':table,'rows':count,'sha256':digest.hexdigest()});print('REVERSE_VERIFIED',table,count,flush=True)
    for (ddl,) in baseline.execute("SELECT sql FROM sqlite_schema WHERE type='index' AND sql IS NOT NULL AND tbl_name NOT LIKE '_newapi_cdc_%'"):dst.execute(ddl)
    dst.commit();assert dst.execute('PRAGMA quick_check').fetchone()[0]=='ok'
    dst.close();pg.close();baseline.close();dest.chmod(0o600)
    r={'passed':True,'tables':results,'seconds':round(time.monotonic()-start,3),'json_storage':'GORM byte blobs','output':str(dest)}
    Path(report).write_text(json.dumps(r,indent=2)+'\n');return r
if __name__=='__main__':
    p=argparse.ArgumentParser()
    for k in ('template','output','socket','database','report'):p.add_argument('--'+k,required=True)
    a=p.parse_args();r=reverse(a.template,a.output,a.socket,a.database,a.report);print('REVERSE_PASS',r['seconds'])
