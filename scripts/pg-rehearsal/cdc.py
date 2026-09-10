"""SQLite transactional change capture and atomic offline PostgreSQL reconciliation.

The destination must have no application writers. A successful poll proves a
consistent source snapshot, not that an application has drained its memory.
"""
import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import sqlite3
import subprocess
import time
import uuid

import psycopg
from psycopg import sql
from migrate import converted, add_hash

PREFIX = '_newapi_cdc_'
META = PREFIX + 'meta'
EVENTS = PREFIX + 'events'


def qi(value):
    return '"' + value.replace('"', '""') + '"'


def literal(value):
    return "'" + value.replace("'", "''") + "'"


class ClosingSQLite(sqlite3.Connection):
    def __exit__(self, *args):
        try:
            return super().__exit__(*args)
        finally:
            self.close()


def source(path, readonly=False):
    connection = sqlite3.connect(Path(path).resolve().as_uri() + ('?mode=ro' if readonly else '?mode=rw'),
                                 uri=True, timeout=1, isolation_level=None, factory=ClosingSQLite)
    return connection


def tables(db):
    result = {}
    for name, ddl in db.execute("SELECT name,sql FROM sqlite_schema WHERE type='table' ORDER BY name"):
        if name.startswith(('sqlite_', PREFIX)):
            continue
        fields = list(db.execute('PRAGMA table_info(' + qi(name) + ')'))
        pk = [r[1] for r in sorted(fields, key=lambda r: r[5]) if r[5]]
        if not pk:
            raise ValueError('Primary key required: ' + name)
        unique = []
        for index in db.execute('PRAGMA index_list(' + qi(name) + ')').fetchall():
            if not index[2]:
                continue
            keys = [r for r in db.execute('PRAGMA index_xinfo(' + qi(index[1]) + ')') if r[5]]
            if any(r[1] < 0 or r[2] is None for r in keys):
                raise ValueError('Expression unique index needs custom capture: ' + index[1])
            unique.append([(r[2], r[4]) for r in keys])
        result[name] = {'columns': [r[1] for r in fields], 'pk': pk, 'ddl': ddl,
                        'unique': sorted(unique)}
    return result


def signature(metadata):
    return hashlib.sha256(json.dumps(metadata, sort_keys=True).encode()).hexdigest()


def trigger_sql(metadata):
    definitions = {}
    for table, info in metadata.items():
        def emit(row):
            key = 'json_array(' + ','.join(row + '.' + qi(c) for c in info['pk']) + ')'
            nulls = ' OR '.join(row + '.' + qi(c) + ' IS NULL' for c in info['pk'])
            # Reject nullable/float/blob primary keys before capturing an ambiguous key.
            invalid = ' OR '.join('typeof(' + row + '.' + qi(c) + ") NOT IN ('integer','text')" for c in info['pk'])
            return ('SELECT CASE WHEN ' + nulls + ' OR ' + invalid + " THEN RAISE(ABORT,'cdc invalid primary key') END; "
                    'INSERT INTO ' + qi(EVENTS) + '(table_name,key_json) VALUES (' + literal(table) + ',' + key + '); ')
        for operation, rows in [('INSERT', ['NEW']), ('UPDATE', ['OLD', 'NEW']), ('DELETE', ['OLD'])]:
            name = PREFIX + table + '_' + operation.lower()
            definitions[name] = 'CREATE TRIGGER ' + qi(name) + ' AFTER ' + operation + ' ON ' + qi(table) + ' BEGIN ' + ''.join(emit(r) for r in rows) + 'END'
        # SQLite REPLACE can silently delete a conflicting row without invoking
        # its DELETE trigger when recursive_triggers is off. Capture its old key
        # before INSERT/UPDATE. False-positive dirty keys are harmless.
        conflicts = []
        for unique in info['unique']:
            if [c for c, _ in unique] == info['pk']:
                continue
            where = ' AND '.join(qi(c) + ' COLLATE ' + qi(collation) + '=NEW.' + qi(c) for c, collation in unique)
            key = 'json_array(' + ','.join(map(qi, info['pk'])) + ')'
            conflicts.append('INSERT INTO ' + qi(EVENTS) + '(table_name,key_json) SELECT ' +
                             literal(table) + ',' + key + ' FROM ' + qi(table) + ' WHERE ' + where + '; ')
        if conflicts:
            for operation in ('INSERT', 'UPDATE'):
                name = PREFIX + table + '_before_' + operation.lower()
                definitions[name] = 'CREATE TRIGGER ' + qi(name) + ' BEFORE ' + operation + ' ON ' + qi(table) + ' BEGIN ' + ''.join(conflicts) + 'END'
    return definitions


def validate(db):
    meta = db.execute('SELECT generation,schema_hash FROM ' + qi(META) + ' WHERE id=1').fetchone()
    if not meta:
        raise ValueError('Missing capture metadata')
    schema = tables(db)
    if signature(schema) != meta[1]:
        raise ValueError('Source schema drift: rebuild the shadow baseline')
    expected = trigger_sql(schema)
    actual = dict(db.execute("SELECT name,sql FROM sqlite_schema WHERE type='trigger' AND name LIKE ?", (PREFIX + '%',)))
    if actual != expected:
        raise ValueError('Capture triggers changed or missing')
    return meta[0], schema


def install(path):
    with source(path) as db:
        found = db.execute('SELECT 1 FROM sqlite_schema WHERE name=?', (META,)).fetchone()
        if found:
            generation, metadata = validate(db)
            db.rollback()
            return {'already_installed': True, 'generation': generation, 'tables': len(metadata)}
        metadata = tables(db)
        if not metadata:
            raise ValueError('No source tables')
        for table, info in metadata.items():
            # Existing values are checked once, using primary keys only.
            for key in info['pk']:
                invalid = db.execute('SELECT 1 FROM ' + qi(table) + ' WHERE ' + qi(key) +
                                     " IS NULL OR typeof(" + qi(key) + ") NOT IN ('integer','text') LIMIT 1").fetchone()
                if invalid:
                    raise ValueError('Unsupported primary key in ' + table)
        # Potentially large validation reads happen before taking the write lock.
        db.execute('BEGIN IMMEDIATE')
        if tables(db) != metadata:
            raise ValueError('Source schema changed during install preflight')
        db.execute('CREATE TABLE ' + qi(META) + '(id INTEGER PRIMARY KEY CHECK(id=1),generation TEXT NOT NULL,schema_hash TEXT NOT NULL)')
        db.execute('CREATE TABLE ' + qi(EVENTS) + '(seq INTEGER PRIMARY KEY AUTOINCREMENT,table_name TEXT NOT NULL,key_json TEXT NOT NULL)')
        generation = str(uuid.uuid4())
        db.execute('INSERT INTO ' + qi(META) + ' VALUES (1,?,?)', (generation, signature(metadata)))
        for ddl in trigger_sql(metadata).values():
            db.execute(ddl)
        db.commit()
        return {'generation': generation, 'tables': len(metadata), 'triggers': len(trigger_sql(metadata))}


def uninstall(path, generation):
    with source(path) as db:
        db.execute('BEGIN IMMEDIATE')
        actual, metadata = validate(db)
        if generation != actual:
            raise ValueError('Capture generation mismatch')
        for name in trigger_sql(metadata):
            db.execute('DROP TRIGGER ' + qi(name))
        db.execute('DROP TABLE ' + qi(EVENTS))
        db.execute('DROP TABLE ' + qi(META))
        db.commit()


def snapshot(path, output):
    output = Path(output)
    if output.exists():
        raise FileExistsError(output)
    started = time.monotonic()
    with source(path, True) as db:
        db.execute('BEGIN')
        generation, metadata = validate(db)
        high = db.execute('SELECT coalesce(max(seq),0) FROM ' + qi(EVENTS)).fetchone()[0]
        with sqlite3.connect(output) as backup:
            def progress(*_):
                if time.monotonic() - started > 120:
                    raise TimeoutError('Snapshot deadline')
            db.backup(backup, pages=1024, progress=progress, sleep=0.01)
        db.rollback()
    output.chmod(0o400)
    return {'generation': generation, 'watermark': high, 'tables': len(metadata),
            'seconds': round(time.monotonic() - started, 3), 'snapshot': str(output)}


def connect(config):
    if not config['database'].startswith('newapi_sync_'):
        raise ValueError('Destination must be an isolated newapi_sync_* database')
    db = psycopg.connect(host=config['socket'], user='rehearsal', dbname=config['database'])
    db.execute("SET statement_timeout='10min'")
    db.execute("SET lock_timeout='5s'")
    db.execute("SET TIME ZONE 'UTC'")
    return db


def target_types(db, metadata):
    refs = db.execute("SELECT count(*) FROM pg_constraint WHERE contype='f' AND connamespace='public'::regnamespace").fetchone()[0]
    if refs:
        raise ValueError('Foreign keys require an explicit constraint-order migration design')
    result = {}
    for table, info in metadata.items():
        fields = dict(db.execute("SELECT column_name,data_type FROM information_schema.columns WHERE table_schema='public' AND table_name=%s", (table,)))
        if not set(info['columns']).issubset(fields):
            raise ValueError('Missing PostgreSQL columns: ' + table)
        result[table] = [fields[c] for c in info['columns']]
    return result


def copy_rows(target, table, info, types, rows):
    stmt = sql.SQL('COPY {} ({}) FROM STDIN').format(sql.Identifier(table), sql.SQL(',').join(map(sql.Identifier, info['columns'])))
    count = 0
    with target.cursor().copy(stmt) as stream:
        for row in rows:
            stream.write_row(tuple(converted(v, datatype) for v, datatype in zip(row, types)))
            count += 1
    return count


def bootstrap(config, path):
    with source(path, True) as src, connect(config) as dst:
        src.execute('BEGIN')
        generation, metadata = validate(src)
        high = src.execute('SELECT coalesce(max(seq),0) FROM ' + qi(EVENTS)).fetchone()[0]
        types = target_types(dst, metadata)
        dst.execute('CREATE SCHEMA IF NOT EXISTS _newapi_sync')
        dst.execute('CREATE TABLE IF NOT EXISTS _newapi_sync.state (id integer PRIMARY KEY CHECK(id=1),generation text NOT NULL,schema_hash text NOT NULL,watermark bigint NOT NULL)')
        if dst.execute('SELECT 1 FROM _newapi_sync.state').fetchone():
            raise ValueError('Shadow baseline already initialized')
        dst.execute(sql.SQL('TRUNCATE {} RESTART IDENTITY').format(sql.SQL(',').join(map(sql.Identifier, metadata))))
        counts = {}
        for table, info in metadata.items():
            rows = src.execute('SELECT ' + ','.join(map(qi, info['columns'])) + ' FROM ' + qi(table))
            counts[table] = copy_rows(dst, table, info, types[table], rows)
            print(json.dumps({'bootstrap_table': table, 'rows': counts[table]}), flush=True)
        dst.execute('INSERT INTO _newapi_sync.state VALUES (1,%s,%s,%s)', (generation, signature(metadata), high))
        dst.commit()
        src.rollback()
        return {'generation': generation, 'watermark': high, 'rows': sum(counts.values()), 'tables': len(counts)}


def sync_once(config, failpoint=None):
    started = time.monotonic()
    if 'source_container' in config:
        instance = json.loads(subprocess.check_output(['docker', 'inspect', config['source_container']], timeout=10))[0]
        if (instance['Id'], instance['State']['StartedAt']) != (config['container_id'], config['container_started']):
            raise ValueError('Source instance restarted: startup migrations can remove capture triggers')
    with source(config['source'], True) as src, connect(config) as dst:
        dst.execute('SELECT pg_advisory_xact_lock(714021090)')
        state = dst.execute('SELECT generation,schema_hash,watermark FROM _newapi_sync.state WHERE id=1 FOR UPDATE').fetchone()
        if state is None:
            raise ValueError('Shadow needs a baseline')
        src.execute('BEGIN')
        generation, metadata = validate(src)
        if (generation, signature(metadata)) != state[:2]:
            raise ValueError('Wrong capture generation or schema')
        high = src.execute('SELECT coalesce(max(seq),0) FROM ' + qi(EVENTS)).fetchone()[0]
        if high < state[2]:
            raise ValueError('Source event history was rolled back or truncated')
        count = src.execute('SELECT count(*) FROM ' + qi(EVENTS) + ' WHERE seq>? AND seq<=?', (state[2], high)).fetchone()[0]
        if count != high - state[2]:
            raise ValueError('Capture event gap: rebuild shadow from a new snapshot')
        changes = list(src.execute('SELECT DISTINCT table_name,key_json FROM ' + qi(EVENTS) + ' WHERE seq>? AND seq<=?', (state[2], high)))
        # Never split an arbitrary source transaction at a row/event batch limit.
        if len(changes) > config.get('max_dirty_keys', 200000):
            raise ValueError('Backlog too large for bounded reconciliation; bootstrap a new shadow')
        types = target_types(dst, metadata)
        groups = {}
        for table, encoded in changes:
            if table not in metadata:
                raise ValueError('Unknown capture table')
            key = json.loads(encoded)
            if len(key) != len(metadata[table]['pk']):
                raise ValueError('Bad captured key')
            groups.setdefault(table, []).append(tuple(key))
        for table, keys in groups.items():
            info = metadata[table]
            where = sql.SQL(' AND ').join(sql.SQL('{}=%s').format(sql.Identifier(c)) for c in info['pk'])
            with dst.cursor() as cursor:
                cursor.executemany(sql.SQL('DELETE FROM {} WHERE {}').format(sql.Identifier(table), where), keys)
        if failpoint == 'before_copy':
            raise RuntimeError('Injected failure before target COPY')
        copied = 0
        for table, keys in groups.items():
            info = metadata[table]
            query = 'SELECT ' + ','.join(map(qi, info['columns'])) + ' FROM ' + qi(table) + ' WHERE ' + ' AND '.join(qi(c) + '=?' for c in info['pk'])
            def rows():
                for key in keys:
                    row = src.execute(query, key).fetchone()
                    if row is not None:
                        yield row
            copied += copy_rows(dst, table, info, types[table], rows())
        dst.execute('UPDATE _newapi_sync.state SET watermark=%s WHERE id=1', (high,))
        if failpoint == 'before_commit':
            raise RuntimeError('Injected failure before target commit')
        dst.commit()
        if failpoint == 'after_commit':
            raise RuntimeError('Injected lost acknowledgement after target commit')
        src.rollback()
    return {'from': state[2], 'to': high, 'dirty_keys': len(changes), 'rows_copied': copied,
            'seconds': round(time.monotonic() - started, 3), 'cutover_ready': False}


def verify(config):
    """Full comparison: only valid when the source does not advance during poll."""
    with source(config['source'], True) as src, connect(config) as dst:
        src.execute('BEGIN')
        generation, metadata = validate(src)
        high = src.execute('SELECT coalesce(max(seq),0) FROM ' + qi(EVENTS)).fetchone()[0]
        state = dst.execute('SELECT generation,watermark FROM _newapi_sync.state WHERE id=1').fetchone()
        if state != (generation, high):
            raise ValueError('Source advanced since last poll; not a cutover-ready comparison')
        types = target_types(dst, metadata)
        output = []
        for table, info in metadata.items():
            q = 'SELECT ' + ','.join(map(qi, info['columns'])) + ' FROM ' + qi(table) + ' ORDER BY ' + ','.join(qi(c) + ' COLLATE BINARY' for c in info['pk'])
            source_hash, dest_hash = hashlib.sha256(), hashlib.sha256()
            a_count = b_count = 0
            for row in src.execute(q):
                add_hash(source_hash, row, types[table]); a_count += 1
            fields = dict(zip(info['columns'], types[table]))
            order = sql.SQL(',').join(sql.SQL('{} COLLATE "C"').format(sql.Identifier(c)) if fields[c] in ('text', 'character varying', 'character') else sql.Identifier(c) for c in info['pk'])
            with dst.cursor(name='v_' + table) as cursor:
                cursor.execute(sql.SQL('SELECT {} FROM {} ORDER BY {}').format(sql.SQL(',').join(map(sql.Identifier, info['columns'])), sql.Identifier(table), order))
                for row in cursor:
                    add_hash(dest_hash, row, types[table]); b_count += 1
            if a_count != b_count or source_hash.digest() != dest_hash.digest():
                raise ValueError('Data mismatch: ' + table)
            output.append({'table': table, 'rows': a_count, 'sha256': source_hash.hexdigest()})
        return {'passed': True, 'generation': generation, 'watermark': high, 'tables': output}


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser()
    parser.add_argument('action', choices=['install', 'snapshot', 'bootstrap', 'sync', 'watch', 'verify', 'uninstall'])
    parser.add_argument('--config', required=True)
    parser.add_argument('--snapshot')
    parser.add_argument('--generation')
    args = parser.parse_args()
    config = json.loads(Path(args.config).read_text())
    with open(args.config + '.lock', 'w') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        if args.action == 'install':
            output = install(config['source'])
        elif args.action == 'uninstall':
            uninstall(config['source'], args.generation); output = {'uninstalled': True}
        elif args.action == 'snapshot':
            output = snapshot(config['source'], args.snapshot)
        elif args.action == 'bootstrap':
            output = bootstrap(config, args.snapshot)
        elif args.action == 'verify':
            output = verify(config)
        elif args.action == 'sync':
            output = sync_once(config)
        else:
            while True:
                try:
                    output = sync_once(config)
                    output['ok'] = True
                except Exception as error:
                    output = {'ok': False, 'error_type': type(error).__name__}
                output['utc'] = time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime())
                text = json.dumps(output)
                status = Path(config['status_file'])
                temporary = status.with_suffix('.tmp')
                temporary.write_text(text + '\n'); temporary.replace(status)
                print(text, flush=True)
                time.sleep(config.get('interval', 2))
        print(json.dumps(output), flush=True)


if __name__ == '__main__':
    main()
