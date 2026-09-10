"""Copy a quiescent SQLite snapshot into a dedicated PostgreSQL database."""
import argparse,hashlib,json,sqlite3,time,sys
from pathlib import Path
import psycopg
from psycopg import sql
sys.path.insert(0,str(Path(__file__).resolve().parent.parent/'pg-rehearsal'))
from migrate import converted,add_hash

def qi(value):return '"'+value.replace('"','""')+'"'

def migrate(source,socket,database,report):
    src=sqlite3.connect(Path(source).resolve().as_uri()+'?mode=ro&immutable=1',uri=True)
    assert src.execute('PRAGMA quick_check').fetchone()[0]=='ok'
    dst=psycopg.connect(host=socket,user='postgres',dbname=database)
    dst.execute("SET TIME ZONE 'UTC'")
    dst.execute("SET work_mem='4MB'")
    dst.execute('SET max_parallel_workers_per_gather=0')
    tables=[r[0] for r in src.execute("SELECT name FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE '_newapi_cdc_%' ORDER BY name")]
    assert tables and database=='newapi'
    for table in tables:
        assert dst.execute(sql.SQL('SELECT NOT EXISTS (SELECT 1 FROM {} LIMIT 1)').format(sql.Identifier(table))).fetchone()[0], 'Target contains data: '+table
    results=[];start=time.monotonic()
    for table in tables:
        info=list(src.execute('PRAGMA table_info('+qi(table)+')'));cols=[r[1] for r in info];pk=[r[1] for r in sorted(info,key=lambda r:r[5]) if r[5]]
        metadata=dict(dst.execute("SELECT column_name,data_type FROM information_schema.columns WHERE table_schema='public' AND table_name=%s",(table,)).fetchall());assert set(cols)<=metadata.keys() and pk
        for col,limit in dst.execute("SELECT column_name,character_maximum_length FROM information_schema.columns WHERE table_schema='public' AND table_name=%s AND data_type='character'",(table,)):
            bad=src.execute('SELECT COUNT(*) FROM '+qi(table)+' WHERE '+qi(col)+' IS NOT NULL AND (length('+qi(col)+')>? OR '+qi(col)+' != rtrim('+qi(col)+',\' \'))',(limit,)).fetchone()[0]
            assert not bad, 'Lossy fixed-character conversion: '+table+'.'+col
        types=[metadata[c] for c in cols]
        query='SELECT '+','.join(map(qi,cols))+' FROM '+qi(table)+' ORDER BY '+','.join(qi(c)+' COLLATE BINARY' for c in pk)
        digest=hashlib.sha256();count=0
        with dst.cursor().copy(sql.SQL('COPY {} ({}) FROM STDIN').format(sql.Identifier(table),sql.SQL(',').join(map(sql.Identifier,cols)))) as stream:
            for row in src.execute(query):
                add_hash(digest,row,types);stream.write_row(tuple(converted(v,t) for v,t in zip(row,types)));count+=1
        order=sql.SQL(',').join(sql.SQL('{} COLLATE "C"').format(sql.Identifier(c)) if metadata[c] in ('text','character varying','character') else sql.Identifier(c) for c in pk)
        target_digest=hashlib.sha256();target_count=0
        with dst.cursor(name='verify_'+table) as cursor:
            cursor.itersize=1000;cursor.execute(sql.SQL('SELECT {} FROM {} ORDER BY {}').format(sql.SQL(',').join(map(sql.Identifier,cols)),sql.Identifier(table),order))
            for row in cursor:add_hash(target_digest,row,types);target_count+=1
        assert count==target_count and digest.digest()==target_digest.digest(), 'Mismatch: '+table
        if 'id' in cols:
            seq=dst.execute('SELECT pg_get_serial_sequence(%s,%s)',(table,'id')).fetchone()[0]
            if seq:
                maximum=dst.execute(sql.SQL('SELECT max(id) FROM {}').format(sql.Identifier(table))).fetchone()[0]
                dst.execute('SELECT setval(%s,%s,%s)',(seq,max(maximum or 1,1),maximum is not None))
        results.append({'table':table,'rows':count,'sha256':digest.hexdigest()});print('VERIFIED',table,count,flush=True)
    dst.commit();dst.close();src.close()
    result={'passed':True,'tables':results,'seconds':round(time.monotonic()-start,3),'source':str(source),'database':database}
    Path(report).write_text(json.dumps(result,indent=2)+'\n');return result

if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--source',required=True);p.add_argument('--socket',required=True);p.add_argument('--database',required=True);p.add_argument('--report',required=True)
    a=p.parse_args();r=migrate(a.source,a.socket,a.database,a.report);print('FULL_MIGRATION_PASS',len(r['tables']),r['seconds'])
