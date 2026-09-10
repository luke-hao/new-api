"""Strict SQLite snapshot -> application-created PostgreSQL schema importer."""

import argparse
import base64
from datetime import datetime, timezone
from decimal import Decimal
import hashlib
import json
import os
import sqlite3
import time

import psycopg
from psycopg import sql
from psycopg.types.json import Json, Jsonb

from ops import ROOT, digest, record, save


def canonical(value, datatype):
    if value is None:
        return None
    if datatype == 'boolean':
        if value not in (0, 1, False, True):
            raise ValueError('Invalid boolean value')
        return bool(value)
    if datatype in ('json', 'jsonb'):
        return json.loads(value) if isinstance(value, (str, bytes, bytearray)) else value
    if datatype == 'bytea':
        return {'base64': base64.b64encode(bytes(value)).decode('ascii')}
    if datatype in ('smallint', 'integer', 'bigint'):
        converted = int(value)
        if Decimal(str(value)) != converted:
            raise ValueError('Non-integral integer value')
        return converted
    if datatype == 'numeric':
        return str(Decimal(str(value)).normalize())
    if datatype in ('real', 'double precision'):
        return float(value)
    if datatype.startswith('timestamp'):
        dt = value if isinstance(value, datetime) else datetime.fromisoformat(value)
        if datatype == 'timestamp with time zone':
            if dt.tzinfo is None:
                dt = dt.replace(tzinfo=timezone.utc)
            dt = dt.astimezone(timezone.utc)
        return dt.isoformat(timespec='microseconds')
    if datatype in ('text', 'character varying', 'character') and isinstance(value, bytes):
        value = value.decode('utf-8')
    if datatype == 'character':
        return value.rstrip(' ')
    return value


def converted(value, datatype):
    normalized = canonical(value, datatype)
    if value is None:
        return None
    if datatype == 'json':
        return Json(normalized)
    if datatype == 'jsonb':
        return Jsonb(normalized)
    if datatype == 'numeric':
        return Decimal(normalized)
    if datatype.startswith('timestamp'):
        return datetime.fromisoformat(normalized)
    if datatype == 'bytea':
        return bytes(value)
    return normalized


def add_hash(hasher, row, types):
    payload = [canonical(v, t) for v, t in zip(row, types)]
    hasher.update(json.dumps(payload, ensure_ascii=True, separators=(',', ':'),
                             sort_keys=True, allow_nan=False).encode('ascii') + b'\n')


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser()
    parser.add_argument('--verify-only', action='store_true')
    args = parser.parse_args()
    snapshot = ROOT / 'original/one-api.snapshot.db'
    assert digest(snapshot) == json.loads((ROOT / 'snapshot.json').read_text())['sha256']
    src = sqlite3.connect('file:' + str(snapshot) + '?mode=ro&immutable=1', uri=True)
    target = psycopg.connect(host=str(ROOT / 'socket'), user='rehearsal', dbname='newapi_import')
    target.execute("SET statement_timeout = '15min'")
    target.execute("SET TIME ZONE 'UTC'")
    tables = [r[0] for r in src.execute("SELECT name FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")]
    columns = {}
    for table in tables:
        metadata = target.execute("SELECT column_name, data_type, character_maximum_length FROM information_schema.columns WHERE table_schema='public' AND table_name=%s ORDER BY ordinal_position", (table,)).fetchall()
        types = {r[0]: r[1] for r in metadata}
        source_cols = [r[1] for r in src.execute('PRAGMA table_info("' + table + '")')]
        missing = set(source_cols) - types.keys()
        if missing:
            raise ValueError('Missing target columns: ' + table + ' ' + repr(sorted(missing)))
        for col, datatype, length in metadata:
            if col in source_cols and datatype == 'character':
                for (value,) in src.execute('SELECT "' + col + '" FROM "' + table + '" WHERE "' + col + '" IS NOT NULL'):
                    assert len(value) <= length and value == value.rstrip(' '), 'Lossy CHAR conversion: ' + table + '.' + col
        columns[table] = [(col, types[col]) for col in source_cols]
    if not args.verify_only:
        target.execute(sql.SQL('TRUNCATE {} RESTART IDENTITY').format(sql.SQL(',').join(map(sql.Identifier, tables))))
    results = []
    started = time.monotonic()
    for table in tables:
        t0 = time.monotonic()
        if args.verify_only:
            results.append({'table': table, 'rows': src.execute('SELECT count(*) FROM "' + table + '"').fetchone()[0]})
            continue
        cols, types = zip(*columns[table])
        identifiers = sql.SQL(',').join(map(sql.Identifier, cols))
        query = sql.SQL('COPY {} ({}) FROM STDIN').format(sql.Identifier(table), identifiers)
        quoted = ','.join('"' + c.replace('"', '""') + '"' for c in cols)
        count = 0
        with target.cursor().copy(query) as copy:
            for row in src.execute('SELECT ' + quoted + ' FROM "' + table + '"'):
                try:
                    copy.write_row(tuple(converted(v, t) for v, t in zip(row, types)))
                except Exception as exc:
                    raise ValueError('Conversion failed at ' + table + ' row ' + str(count + 1)) from exc
                count += 1
                if count % 100000 == 0:
                    print('COPY ' + table + ' ' + str(count), flush=True)
        result = {'table': table, 'rows': count, 'import_seconds': round(time.monotonic() - t0, 3)}
        results.append(result)
        print('IMPORTED ' + json.dumps(result), flush=True)
    target.commit()
    for result in results:
        table = result['table']
        cols, types = zip(*columns[table])
        pk = [r[1] for r in sorted(src.execute('PRAGMA table_info("' + table + '")'), key=lambda r: r[5]) if r[5]]
        assert pk, 'A deterministic primary key is required: ' + table
        order_sqlite = ','.join('"' + c + '" COLLATE BINARY' for c in pk)
        order_pg = sql.SQL(',').join(sql.SQL('{} COLLATE "C"').format(sql.Identifier(c))
                                   if dict(columns[table])[c] in ('text', 'character varying', 'character')
                                   else sql.Identifier(c) for c in pk)
        src_hash, dst_hash = hashlib.sha256(), hashlib.sha256()
        quoted = ','.join('"' + c + '"' for c in cols)
        for row in src.execute('SELECT ' + quoted + ' FROM "' + table + '" ORDER BY ' + order_sqlite):
            add_hash(src_hash, row, types)
        count = 0
        with target.cursor(name='verify_' + table) as cursor:
            cursor.itersize = 5000
            cursor.execute(sql.SQL('SELECT {} FROM {} ORDER BY {}').format(
                sql.SQL(',').join(map(sql.Identifier, cols)), sql.Identifier(table), order_pg))
            for row in cursor:
                add_hash(dst_hash, row, types)
                count += 1
        result.update({'target_rows': count, 'source_sha256': src_hash.hexdigest(),
                       'target_sha256': dst_hash.hexdigest()})
        assert count == result['rows'] and src_hash.digest() == dst_hash.digest(), table
        if 'id' in cols:
            sequence = target.execute('SELECT pg_get_serial_sequence(%s, %s)', (table, 'id')).fetchone()[0]
            if sequence:
                maximum = target.execute(sql.SQL('SELECT max(id) FROM {}').format(sql.Identifier(table))).fetchone()[0]
                target.execute('SELECT setval(%s, %s, %s)', (sequence, max(maximum or 1, 1), maximum is not None))
        print('VERIFIED ' + table + ' rows=' + str(count) + ' sha256=' + src_hash.hexdigest(), flush=True)
    target.commit()
    target.autocommit = True
    target.execute('ANALYZE')
    report = {'passed': True, 'tables': results, 'seconds': round(time.monotonic() - started, 3),
              'source_snapshot_sha256': digest(snapshot), 'normalization': 'Strict typed values, UTC timestamps, canonical JSON, PostgreSQL CHAR padding removed after source length/whitespace validation; no skipped rows'}
    save('data-verification.json', report)
    record({'data_verification': report})
    print('IMPORT AND FULL DATA VERIFICATION PASS', flush=True)
    target.close()
    src.close()


if __name__ == '__main__':
    main()
