# Online migration preparation

The CDC and admission tools prepare a shadow database. They do not perform a
production cutover or prove that application memory has drained.

`cdc.py` installs SQLite capture tables and triggers in a short write transaction
after read-only primary-key validation. Business writes and their dirty keys
commit together. Each reconciliation reads one consistent source snapshot and
applies all dirty keys through that snapshot's watermark in one PostgreSQL
transaction, together with the checkpoint. UPDATE, DELETE, composite-key moves,
REPLACE implicit deletes and rollback are covered. All old dirty destination
rows are removed before copying current rows, allowing unique-value swaps.

The destination must have no independent application writers. The tool accepts
only `newapi_sync_*` destinations. There is no pruning of capture events yet;
capture is a temporary migration facility, not a permanent log retention system.
The watcher reports errors and never advances its checkpoint after a failed
transaction. A source instance change, missing triggers, schema drift or event
gap invalidates readiness and requires a reviewed fresh baseline.

This production version's startup AutoMigrate can rebuild SQLite tables and drop
their triggers. Install capture after startup, never restart the source during
capture, and do not automatically repair missing triggers while pretending the
old baseline is current. Rebuild from a new consistent snapshot after repair.

`cmd/migration-gate` builds a loopback HTTP proxy with a permission-restricted
Unix control socket. It starts paused. POST `/pause` closes admission; existing
responses, including SSE, keep running. POST `/drain` waits up to 30 seconds for
active responses. `/switch?target=http://127.0.0.1:PORT` requires paused admission
and zero active responses. `/resume` reopens admission. GET `/status` reports
active and waiting counts. Queue length and time are bounded; overflow/timeouts
return 503 and cancelled requests never execute. Queued requests are not durable
across a proxy crash, and no automatic replay is performed.

Example control call (run only against the private rehearsal socket):

```sh
curl --unix-socket /PRIVATE_PATH/control.sock -X POST http://localhost/pause
```

Production routing is not wired to this proxy during shadow preparation. Ingress
coverage must include the public HTTPS proxy, exposed port 3000, existing
keep-alive connections and callbacks before it can be used for cutover.

## Remaining application work

The current process queues batch quota updates in memory, logs some write errors
without retaining the failed batch, and refunds some requests asynchronously.
HTTP drain alone cannot prove completion of these operations. Implement and
exercise an explicit flush/error protocol, tracked asynchronous refunds and
background-writer fencing before any database handoff. Final reconciliation,
sequence refresh, cache refresh and rollback after new PostgreSQL writes also
remain mandatory. Do not infer readiness from a quiet event log or a delay.

## Tests

Run Python tests in `scripts/pg-rehearsal` with the private Psycopg environment
while the existing isolated PostgreSQL container is running:

```sh
python -m unittest -v test_cdc test_migrate
go test -race ./pkg/migrationgate ./cmd/migration-gate
```

`rehearse_cdc.py` additionally uses the existing full SQLite test copy and the
immutable old application image to produce real synthetic relay/billing writes,
then verifies all 29 tables and uninstalls capture from that copy. It does not
write the live database.
