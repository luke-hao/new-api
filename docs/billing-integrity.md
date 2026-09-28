# Billing integrity

Wallet and token reservations use conditional database writes in one transaction.
Cached balances cannot authorize spending, including unlimited tokens and high-balance accounts.
Subscription reservations also include the token debit in their transaction.

`billing_adjustments` stores unique settlement/refund intents. Applying an intent
updates the wallet or subscription, token, user usage statistics and task quota
together. Missing financial rows roll back the adjustment; missing channel rows
do not block settlement. Soft-deleted wallet/token rows remain payable.
Task terminal transitions commit with a fixed-key adjustment to prevent duplicate
callbacks and to retain refunds after a temporary financial-write failure.

The worker retries pending adjustments every 15 seconds. When the database cannot
accept an ordinary request intent, a synced local journal under the mounted
`/data/billing-pending` directory retains it for replay. `BILLING_JOURNAL_DIR` can
override this path; keep it on persistent storage. Journal entries contain account
IDs and amounts, never credentials or prompts. Replaying an existing ID is a no-op.
Keep journal files and the ledger during upgrades and rollbacks.

Operational checks:

```sql
SELECT status, count(*), min(created_at), max(attempts)
FROM billing_adjustments GROUP BY status;
SELECT id, user_id, delta, attempts, last_error
FROM billing_adjustments WHERE status = 'pending' ORDER BY created_at;
```

Inspect persistent journal entries if a database outage occurred. Never repair by
blindly subtracting all consume logs from the current wallet: precharges, refunds,
subscriptions, deleted logs and live requests must be accounted for. In particular,
current negative balances are receivables, not proof of losses from one incident.

Payment callbacks, redemptions and subscription purchases use GORM v2 row locking.
Checkins retain their uniqueness and financial credit in one transaction on all
supported databases. Administrator balance replacement rejects a stale balance.
Legacy Midjourney submissions now reserve before dispatch; failed/free submissions
cannot create a refundable amount that was never charged.

Validation covers concurrent balance exhaustion, duplicate payment/redemption/task
callbacks, missing/deleted financial rows, deleted channels, retry after failure,
local journal replay, realtime incremental accounting, and stale admin replacement.
Tests run against SQLite and PostgreSQL. MySQL uses the same GORM operations but
has not been exercised against a live MySQL server in this change.

Scope limits: actual provider usage can exceed an estimate, and a provider may omit
usage or fail after charging. Precharge and confirmed-usage settlement reduce risk
but cannot promise zero unpaid provider cost. A process/host loss before final
usage is observed, or simultaneous database and journal-storage failure, still
requires reconciliation with provider records. Consume logs use a separate logging
path and are not a complete financial ledger. This review does not certify every
provider adapter or every price configuration.
