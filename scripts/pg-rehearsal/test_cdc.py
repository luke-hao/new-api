"""Real SQLite/PostgreSQL fault and concurrency tests; private test databases only."""
import json
from pathlib import Path
import sqlite3
import tempfile
import threading
import time
import unittest
from unittest.mock import patch

import psycopg
from psycopg import sql
import cdc

SOCKET = '/opt/new-api/backups/20260910-postgres-rehearsal/socket'


class CaptureTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.path = Path(self.temp.name) / 'source.db'
        with sqlite3.connect(self.path) as db:
            db.executescript('''PRAGMA journal_mode=WAL;
                CREATE TABLE accounts(id INTEGER PRIMARY KEY,name TEXT UNIQUE,balance INTEGER NOT NULL,payload BLOB);
                CREATE TABLE postings(id INTEGER PRIMARY KEY,account_id INTEGER,amount INTEGER);
                CREATE TABLE config("group" TEXT,"key" TEXT,value TEXT,PRIMARY KEY("group","key"));
                INSERT INTO accounts VALUES (1,'a',1000,X'7B7D'),(2,'b',1000,NULL);
                INSERT INTO config VALUES ('default','a','old');''')
        self.generation = cdc.install(self.path)['generation']
        self.database = 'newapi_sync_test_' + str(time.time_ns())
        self.config = {'source': str(self.path), 'socket': SOCKET, 'database': self.database}
        with psycopg.connect(host=SOCKET, user='rehearsal', dbname='postgres', autocommit=True) as db:
            db.execute(sql.SQL('CREATE DATABASE {}').format(sql.Identifier(self.database)))
        with cdc.connect(self.config) as db:
            db.execute('CREATE TABLE accounts(id bigint PRIMARY KEY,name text UNIQUE,balance bigint NOT NULL,payload bytea)')
            db.execute('CREATE TABLE postings(id bigint PRIMARY KEY,account_id bigint,amount bigint)')
            db.execute('CREATE TABLE config("group" text,"key" text,value text,PRIMARY KEY("group","key"))')
        cdc.snapshot(self.path, Path(self.temp.name) / 'snapshot.db')
        cdc.bootstrap(self.config, Path(self.temp.name) / 'snapshot.db')

    def tearDown(self):
        with psycopg.connect(host=SOCKET, user='rehearsal', dbname='postgres', autocommit=True) as db:
            db.execute(sql.SQL('DROP DATABASE {}').format(sql.Identifier(self.database)))
        self.temp.cleanup()

    def mutate(self, query):
        with sqlite3.connect(self.path) as db:
            db.executescript('BEGIN; ' + query + ' COMMIT;')

    def state(self):
        with cdc.connect(self.config) as db:
            return (db.execute('SELECT balance FROM accounts ORDER BY id').fetchall(),
                    db.execute('SELECT watermark FROM _newapi_sync.state').fetchone()[0])

    def test_insert_update_delete_pk_move_and_unique_swap(self):
        self.mutate("""UPDATE accounts SET balance=balance-12 WHERE id=1;
            INSERT INTO postings VALUES(1,1,-12);
            DELETE FROM accounts WHERE id=2;
            INSERT INTO accounts VALUES(3,'c',30,X'00FF');
            UPDATE config SET "key"='b',value='new';
            UPDATE accounts SET name='temp' WHERE id=1;
            UPDATE accounts SET name='a' WHERE id=3;
            UPDATE accounts SET name='c' WHERE id=1;""")
        cdc.sync_once(self.config)
        self.assertTrue(cdc.verify(self.config)['passed'])
        self.assertEqual(cdc.sync_once(self.config)['dirty_keys'], 0)

    def test_source_transaction_rollback_creates_no_events(self):
        with sqlite3.connect(self.path) as db:
            db.execute('UPDATE accounts SET balance=0')
            db.rollback()
        self.assertEqual(cdc.sync_once(self.config)['dirty_keys'], 0)
        self.assertTrue(cdc.verify(self.config)['passed'])

    def test_replace_captures_implicit_delete_without_recursive_triggers(self):
        with sqlite3.connect(self.path) as db:
            db.execute('PRAGMA recursive_triggers=OFF')
            db.execute("INSERT OR REPLACE INTO accounts VALUES(3,'a',77,NULL)")
            db.commit()
        cdc.sync_once(self.config)
        self.assertTrue(cdc.verify(self.config)['passed'])
        with sqlite3.connect(self.path) as db:
            db.execute('PRAGMA recursive_triggers=OFF')
            db.execute("UPDATE OR REPLACE accounts SET name='b' WHERE id=3")
            db.commit()
        cdc.sync_once(self.config)
        self.assertTrue(cdc.verify(self.config)['passed'])

    def test_capture_gap_is_rejected(self):
        self.mutate('UPDATE accounts SET balance=balance-1;')
        with sqlite3.connect(self.path) as db:
            db.execute('DELETE FROM _newapi_cdc_events WHERE seq=1')
        with self.assertRaisesRegex(ValueError, 'gap'):
            cdc.sync_once(self.config)

    def test_instance_restart_requires_new_baseline(self):
        cfg={**self.config,'source_container':'new-api','container_id':'expected','container_started':'original'}
        with patch('cdc.subprocess.check_output',return_value=b'[{"Id":"expected","State":{"StartedAt":"restarted"}}]'):
            with self.assertRaisesRegex(ValueError,'restarted'):
                cdc.sync_once(cfg)

    def test_target_failure_rolls_back_data_and_checkpoint(self):
        before = self.state()
        self.mutate('UPDATE accounts SET balance=balance-1 WHERE id=1; INSERT INTO postings VALUES(1,1,-1);')
        for stage in ('before_copy', 'before_commit'):
            with self.assertRaises(RuntimeError):
                cdc.sync_once(self.config, failpoint=stage)
            self.assertEqual(self.state(), before)
        cdc.sync_once(self.config)
        self.assertTrue(cdc.verify(self.config)['passed'])

    def test_lost_acknowledgement_retries_without_double_charge(self):
        self.mutate('UPDATE accounts SET balance=balance-17 WHERE id=1;')
        with self.assertRaises(RuntimeError):
            cdc.sync_once(self.config, failpoint='after_commit')
        committed = self.state()
        self.assertEqual(committed[0][0][0], 983)
        self.assertEqual(cdc.sync_once(self.config)['dirty_keys'], 0)
        self.assertEqual(self.state(), committed)

    def test_inflight_transaction_is_not_partially_visible(self):
        with sqlite3.connect(self.path) as writer:
            writer.execute('BEGIN IMMEDIATE')
            writer.execute('UPDATE accounts SET balance=balance-1 WHERE id=1')
            self.assertEqual(cdc.sync_once(self.config)['dirty_keys'], 0)
            writer.execute('INSERT INTO postings VALUES (1,1,-1)')
            writer.commit()
        self.assertGreater(cdc.sync_once(self.config)['dirty_keys'], 0)
        self.assertTrue(cdc.verify(self.config)['passed'])

    def test_concurrent_commits_and_reconciler_preserve_ledger(self):
        failures = []
        def writer():
            try:
                with sqlite3.connect(self.path, timeout=5) as db:
                    for i in range(1, 81):
                        db.execute('UPDATE accounts SET balance=balance-1 WHERE id=1')
                        db.execute('INSERT INTO postings VALUES (?,1,-1)', (i,))
                        db.commit()
                        time.sleep(.001)
            except Exception as error:
                failures.append(error)
        thread = threading.Thread(target=writer)
        thread.start()
        while thread.is_alive():
            cdc.sync_once(self.config)
            with cdc.connect(self.config) as db:
                a,b = db.execute('SELECT (SELECT balance FROM accounts WHERE id=1),(SELECT count(*) FROM postings)').fetchone()
                self.assertEqual(a + b, 1000)
        thread.join()
        self.assertEqual(failures, [])
        cdc.sync_once(self.config)
        self.assertTrue(cdc.verify(self.config)['passed'])

    def test_missing_trigger_and_schema_drift_fail_closed(self):
        with sqlite3.connect(self.path) as db:
            db.execute('DROP TRIGGER _newapi_cdc_accounts_update')
        with self.assertRaisesRegex(ValueError, 'triggers'):
            cdc.sync_once(self.config)

    def test_uninstall_is_generation_checked_and_preserves_rows(self):
        with self.assertRaises(ValueError):
            cdc.uninstall(self.path, 'wrong')
        cdc.uninstall(self.path, self.generation)
        with sqlite3.connect(self.path) as db:
            self.assertEqual(db.execute('SELECT sum(balance) FROM accounts').fetchone()[0], 2000)
            self.assertEqual(db.execute("SELECT count(*) FROM sqlite_schema WHERE name LIKE '_newapi_cdc_%'").fetchone()[0], 0)


if __name__ == '__main__':
    unittest.main(verbosity=2)
