# PostgreSQL Rehearsal

This tooling runs only on the current production host. It does not switch
production traffic, change production environment variables, restart production,
or install triggers in the live SQLite database.

## Isolation

- Artifacts: `/opt/new-api/backups/20260910-postgres-rehearsal`, mode `0700`.
- Scripts: `/opt/new-api-src/current/scripts/pg-rehearsal`.
- All rehearsal containers have `--network none` and no published ports.
- PostgreSQL is reached over a Unix socket mounted only into rehearsal containers.
- The API image is pinned to the observed production image ID; no application
  rebuild or production deployment is performed.
- The host may enter the isolated application's network namespace for tests.
- Copied credentials stay inside the private remote artifact directory.
- Mock tests disable copied channels and use a synthetic user, token and loopback
  upstream only in disposable test copies. The verified import remains separate.

## Commands

Run from the canonical source directory, with root access for private artifacts:

```sh
python3 scripts/pg-rehearsal/ops.py prepare
/opt/new-api/backups/20260910-postgres-rehearsal/venv/bin/python scripts/pg-rehearsal/migrate.py
python3 scripts/pg-rehearsal/ops.py clone
/opt/new-api/backups/20260910-postgres-rehearsal/venv/bin/python scripts/pg-rehearsal/behavior.py fixture
python3 scripts/pg-rehearsal/ops.py start-postgres
python3 scripts/pg-rehearsal/ops.py start-sqlite
/opt/new-api/backups/20260910-postgres-rehearsal/venv/bin/python scripts/pg-rehearsal/behavior.py check
sh scripts/pg-rehearsal/rollback.sh
```

`migrate.py` replaces tables only in the fixed `newapi_import` rehearsal database.
It is a full snapshot importer, not an incremental synchronizer. Run it before
creating test clones. `--verify-only` repeats full comparisons without reimporting.
`behavior.py check` expects fresh fixtures and is deliberately single-use.

Rollback stops the label-verified, network-isolated rehearsal containers,
retains all databases, and verifies production image/start time/config hash and
HTTP status. `ops.py resume` restarts the same rehearsal objects; it never starts
or changes the production container.

## Verification Semantics

The source is a SQLite Online Backup API snapshot with a pinned read transaction.
Holding that reader allows writes but temporarily prevents reclamation of WAL
pages. The backup has a 180-second deadline. The immutable snapshot is hashed and
checked with `PRAGMA quick_check`.

The application creates the PostgreSQL schema before data import. Import uses
Psycopg COPY with explicit column lists and typed conversion, in one transaction.
Every source column must exist in the destination; no rows are skipped. All
tables are then compared by primary-key order, row count and SHA-256 of canonical
typed row values. JSON objects and binary encodings are normalized; timestamps
are normalized to UTC; PostgreSQL fixed-width CHAR padding is ignored only after
checking source strings have neither overflow nor trailing spaces. IDs and
sequences are reconciled after successful comparison.

The source snapshot predates all subsequent production writes. A successful
rehearsal does not mean the destination is up to date with production.

## Production Cutover Gates

Before a production cutover, implement and rehearse all of the following:

1. Transaction-consistent incremental replication covering INSERT, UPDATE and
   DELETE for every mutable table. ID-only polling does not cover quota changes.
2. Controlled admission of new requests, with bounded waiting and client-timeout
   behavior, while existing SSE requests settle.
3. Explicit quiescence of background writers and confirmed flush of in-memory
   batch quota updates. Current application startup uses `server.Run`; merely
   stopping the container is not a verified flush protocol.
4. Final replication watermark, ledger reconciliation, cache refresh, identity
   sequence reset, and single-writer enforcement before admitting new traffic.
5. Reverse synchronization or reconciliation for rollback after PostgreSQL has
   accepted writes. Reverting directly to the stale SQLite snapshot loses writes.
6. Payment callback, subscription renewal, interrupted-stream billing, long-lived
   SSE, and realistic sustained-load tests beyond this bounded rehearsal.

No zero-downtime production guarantee is implied by these offline tests.
