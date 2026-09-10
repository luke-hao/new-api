# Maintenance SQLite → PostgreSQL cutover

The legacy a84217d process has no shutdown/flush endpoint. `legacy_state.py`
reads only queue lengths and stack metadata from the reviewed executable.
Its SHA256 guard rejects every other binary. `legacy_ready.py` accepts only
empty quota/dashboard queues, no queued pool work, waiting reviewed background
loops, and no active application work. A definitive sample requires an externally
paused Docker cgroup; a running sample is diagnostic only. Requests still reading
an incomplete body before `Distribute` invokes `controller.Relay` have not entered
precharge and may be disconnected during maintenance. Unknown work blocks cutover.

Before pausing: route NewAPI domains and new direct port-3000 connections to the
maintenance responder. Leave already accepted streams running. During the initial
drain, payment callbacks continue to the old process; set responder mode `hold`
before the final frozen sample. Private callback journals are fsynced before a
503 response. Providers receive no artificial acknowledgement and can retry.
Review and replay retained notifications through the original signed handlers.

Stop shadow CDC and backup timers, pause the legacy container, and inspect. If
not ready, unpause immediately and continue waiting. If ready, keep it paused,
create a SQLite online-backup snapshot and run quick_check. The immutable snapshot
is the sole source for `migrate_snapshot.py`, which imports all business tables,
checks every row using canonical hashes, and commits one PostgreSQL transaction.
Capture triggers/internal CDC tables are intentionally excluded. Preserve the
snapshot and the old container until target verification passes.

Boot the committed new image on PostgreSQL behind maintenance. Verify HTTP 200,
unauthenticated 401, PostgreSQL connectivity, balances and orders. Test the
previous image on the **same PostgreSQL database**, then reapply the new image.
This image rollback preserves all new billing writes. Do not restore a stale
SQLite database after PostgreSQL has accepted traffic. `reverse_snapshot.py`
creates a fresh full SQLite copy when a database rollback is needed; it preserves
GORM JSON byte storage and validates every table. Both writers must be drained
and fenced before using a reverse copy.

Enable the engine-aware R2 backup, make a backup, verify a restore, then restore
normal ingress and timers. Synchronize only reviewed source/non-sensitive compose
through the approved public `luke-hao/new-api` repository. Environment files,
callback bodies, databases and runtime reports containing identifiers stay on the
server under private backup directories.
