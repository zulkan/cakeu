# Ledger-Backed Account Balances — Design

Date: 2026-11-07
Status: Draft — pending review.

## Summary

Today an account has one mutable `accounts.balance` column that is set at creation, overwritten by the inline edit form, and adjusted by transactions. The same number is labelled "Starting balance", "Balance", and "Current balance", and a manual overwrite silently breaks reconciliation with the transaction history.

This design makes the **ledger the single source of truth**:

- Creating an account with an opening balance writes a locked `opening` income entry (or expense entry, if negative).
- Correcting a balance writes a locked `adjustment` entry for the difference.
- Editing/deleting a normal transaction changes the derived balance automatically.
- `accounts.balance` is dropped; current balance is always `SUM(income) - SUM(expense)` over the account's entries.
- Opening and adjustment entries are visible in history but excluded from income/expense analytics, budgets, reports, and CSV export.
- Multi-currency aggregates return **one sum per currency** instead of adding different currencies together.

## Decisions

| # | Decision | Choice | Rationale |
|---|---|---|---|
| 1 | Source of truth | Ledger. Drop `accounts.balance`. | A stored total that can be overwritten is the root cause of the confusion. Deriving it makes drift impossible. |
| 2 | Opening balance | A locked `opening` entry created with the account. Positive = income, negative = expense. | The starting amount becomes explainable history instead of a hidden constant. |
| 3 | Manual correction | A locked `adjustment` entry created by a dedicated endpoint. No direct balance edit. | Preserves an audit trail; deleting/editing old transactions can no longer corrupt the total. |
| 4 | Existing accounts | Backfill `opening = stored balance − net(existing entries)`, dated at `LEAST(account creation, earliest entry)`. | Keeps every current balance identical while making all accounts reconcile. |
| 5 | Negative opening | Allowed (credit cards, overdrafts), stored as an expense entry. | Transactions can already push a balance negative; the opening must be able to represent debt. |
| 6 | System entries | Visible in transaction history, but not editable/deletable, and excluded from income/expense analytics, budgets, reports, and CSV export. | The entry explains the balance without inflating income or breaking export/import round-trips. |
| 7 | System-entry modeling | `transactions.kind` + nullable `category_id`. | No fake categories in pickers, no category-deletion risk. Cost is a `LEFT JOIN` and nullable fields. |
| 8 | Currency | Locked once the account has at least one entry (`entry_count > 0`). | Changing currency would silently reinterpret every past amount; no FX conversion is in scope. |
| 9 | Multi-currency scope | Per-currency dashboard balances and monthly totals; each transaction displays its own account currency. Reports and budgets remain single-currency follow-ups. | The most visible mixed-currency sums are fixed now; the larger reporting/budget change gets its own plan. |
| 10 | CSV export | Normal entries only. | Keeps the documented `date,type,amount,description,account,category` format importable. |
| 11 | API compatibility | Create takes `opening_balance`; update rejects `balance`; new adjustment endpoint. | Clear separation between starting amount, current amount, and correction. |

## Approaches considered

**A. `kind` column + nullable category (chosen).**
Add `transactions.kind` (`normal|opening|adjustment`) and allow `category_id = NULL` for system entries. Queries use `LEFT JOIN categories`, analytics filter `kind = 'normal'`, and the UI renders a system label instead of a category name. No fake data, no hidden categories, one explicit marker.

**B. Hidden per-user system categories.**
Keep `category_id NOT NULL` and create protected "Opening Balance" categories. Smaller join change, but it pollutes the category model, requires `is_system` flags and deletion guards, and every picker/report must remember to exclude them.

**C. Separate adjustments table.**
Keep opening/adjustment rows outside `transactions`. Cleaner separation but balance reads must union two sources, and it contradicts the chosen model that the opening balance is a real ledger entry.

## Data model

### `transactions`

Add:

- `kind VARCHAR(16) NOT NULL DEFAULT 'normal' CHECK (kind IN ('normal','opening','adjustment'))`
- `category_id` becomes nullable.
- Indexes: `(kind)` and `(account_id, kind)`.

System entries:

- `kind = 'opening' | 'adjustment'`
- `category_id = NULL`
- `type = 'income'` when the signed value is positive, `'expense'` when negative.
- `amount = ABS(value)` (the existing `amount > 0` check stays).
- `description = 'Opening balance' | 'Balance adjustment'` or a user-supplied adjustment note.
- `date = account creation date` for opening entries, `CURRENT_DATE` for adjustments.

### `accounts`

Drop `balance`. Keep `id`, `user_id`, `name`, `currency`, `created_at`, `updated_at`.

### Account response

`model.Account` keeps the JSON field `balance`, but it is now derived and read-only:

- `balance` — current balance across all entry kinds.
- `opening_balance` — signed sum of `kind = 'opening'` entries (one entry in practice).
- `entry_count` — number of ledger entries; drives the currency lock.
- `currency`, `created_at`, `updated_at` — unchanged.

Query:

```sql
SELECT a.id, a.user_id, a.name, a.currency, a.created_at, a.updated_at,
       COALESCE(SUM(CASE WHEN t.type = 'income' THEN t.amount ELSE -t.amount END), 0) AS balance,
       COALESCE(SUM(
           CASE WHEN t.kind = 'opening'
                THEN CASE WHEN t.type = 'income' THEN t.amount ELSE -t.amount END
           END
       ), 0) AS opening_balance,
       COUNT(t.id) AS entry_count
FROM accounts a
LEFT JOIN transactions t ON t.account_id = a.id
WHERE a.user_id = $1
GROUP BY a.id
ORDER BY a.created_at ASC, a.name ASC
```

`a.id` is the primary key, so PostgreSQL allows the other `a.*` columns in the select list.

### Transaction response

Add:

- `kind` (`normal|opening|adjustment`)
- `currency` (from the account, so each row formats correctly)
- `category_id` becomes nullable
- `category_name` may be empty for system entries

`GetByID`, `List`, and `Create`/`Update` return these fields. List queries switch `JOIN categories` to `LEFT JOIN categories`, and join `accounts` to obtain `currency`.

## Migration `005`

Up:

```sql
ALTER TABLE transactions
    ADD COLUMN kind VARCHAR(16) NOT NULL DEFAULT 'normal'
    CHECK (kind IN ('normal', 'opening', 'adjustment'));

ALTER TABLE transactions ALTER COLUMN category_id DROP NOT NULL;

INSERT INTO transactions
    (user_id, account_id, category_id, type, amount, description, date, kind)
SELECT
    a.user_id,
    a.id,
    NULL,
    CASE WHEN a.balance - COALESCE(n.net, 0) >= 0 THEN 'income' ELSE 'expense' END,
    ABS(a.balance - COALESCE(n.net, 0)),
    'Opening balance',
    LEAST(a.created_at::date, COALESCE(n.first_date, a.created_at::date)),
    'opening'
FROM accounts a
LEFT JOIN (
    SELECT account_id,
           SUM(CASE WHEN type = 'income' THEN amount ELSE -amount END) AS net,
           MIN(date) AS first_date
    FROM transactions
    GROUP BY account_id
) n ON n.account_id = a.id
WHERE a.balance - COALESCE(n.net, 0) <> 0
  AND NOT EXISTS (
      SELECT 1 FROM transactions t
      WHERE t.account_id = a.id AND t.kind = 'opening'
  );

ALTER TABLE accounts DROP COLUMN balance;

CREATE INDEX idx_transactions_kind ON transactions(kind);
CREATE INDEX idx_transactions_account_kind ON transactions(account_id, kind);
```

Backfill examples:

| Stored balance | Existing net | Opening entry | Derived after migration |
|---|---|---|---|
| 1000 | 0 | +1000 income | 1000 |
| 1000 | +200 | +800 income | 1000 |
| 1000 | -1200 | +2200 income | 1000 |
| -50 | 0 | -50 expense | -50 |
| 0 | 0 | none | 0 |

The insert is guarded against an existing `opening` entry, so re-running the statement manually cannot duplicate it. `golang-migrate` runs each migration in a transaction.

Down:

1. Re-add `accounts.balance NUMERIC(15,2) NOT NULL DEFAULT 0`.
2. Recompute it from all entries (including system entries) so current values are preserved.
3. Delete `kind <> 'normal'` entries.
4. Drop `kind`.
5. Restore `category_id NOT NULL`.

`category_id` can only be restored after system entries are removed; normal entries always have a category.

## Balance and analytics rules

| Query | Include system entries? | Notes |
|---|---|---|
| Account list / account detail | Yes | Derived balance and opening balance. |
| Dashboard total balance | Yes | Aggregated per currency. |
| Dashboard monthly income/expense/net | No (`kind = 'normal'`) | Aggregated per currency. |
| Dashboard recent transactions | No | Real activity only. |
| Dashboard transaction count | No | Current UTC month, normal entries only (also fixes the existing all-time count behind a "this month" label). |
| Transactions page list | Yes | System rows render locked. |
| Transaction update/delete | Reject system entries | Service and repository both check. |
| CSV export | No | Keeps the export round-trippable through import. |
| Reports (cashflow/category/merchant) | No | Add `kind = 'normal'`; still single-currency (follow-up). |
| Budget spending | No | Add `kind = 'normal'`; still single-currency (follow-up). |
| Recurring / import writes | N/A | They produce `normal` entries only. |

## API changes

### `POST /api/accounts`

```json
{ "name": "Everyday checking", "opening_balance": 1250.0, "currency": "USD" }
```

- `opening_balance` is signed and may be negative; NaN/Infinity are rejected. A supplied `balance` is rejected with a clear 400 pointing to `opening_balance`.
- Response is the derived account shape (`balance`, `opening_balance`, `entry_count`).

### `PATCH /api/accounts/{id}`

```json
{ "name": "Main checking" }
```

- Accepts `name` and `currency` only.
- `balance` is kept in the request struct solely to detect it and return `400` with "use POST /api/accounts/{id}/adjustments".
- Currency change returns `409 currency is locked after the first entry` when `entry_count > 0`.

### `POST /api/accounts/{id}/adjustments`

```json
{ "target_balance": 950.0, "description": "Matched bank statement" }
```

- Validates `target_balance` as finite (NaN/Infinity rejected); computes `delta = target_balance - current_balance`, rounded to 2 decimals.
- Inserts a locked adjustment entry when `delta != 0`; returns the updated account.
- `description` defaults to `"Balance adjustment"`.

### Transactions

- List/get responses include `kind`, `currency`, nullable `category_id`, and possibly empty `category_name`.
- `PATCH` and `DELETE` on a system entry return `409 system entries cannot be modified`.

## Backend changes

| File | Change |
|---|---|
| `be/internal/migrations/005_ledger_backed_balances.up.sql` | Schema change and backfill above. |
| `be/internal/migrations/005_ledger_backed_balances.down.sql` | Restore `balance`, drop `kind`, restore `category_id NOT NULL`. |
| `be/internal/model/finance.go` | `Account` gains `OpeningBalance`, `EntryCount`; `Transaction` gains `Kind`, `Currency`, nullable `CategoryID`; `CreateAccountRequest` uses `OpeningBalance`; add `AdjustAccountRequest`; `TransactionFilters` gains `Kind`. |
| `be/internal/service/finance.go` | Remove balance deltas; create account + opening entry atomically; add `AdjustAccountBalance`; reject balance updates and system-entry mutations; lock currency. |
| `be/internal/db/finance.go` | Derived account queries; atomic create-with-opening and adjust; remove all `UPDATE accounts SET balance` statements and `balanceDeltas`; `LEFT JOIN categories`; return `kind`/`currency`; filter dashboard recent/count by `kind = 'normal'`. |
| `be/internal/db/report.go` | Add `kind = 'normal'` to cashflow, category, and merchant queries. |
| `be/internal/db/budget.go` | Add `kind = 'normal'` to spending. |
| `be/internal/http/finance.go` | New adjustment handler; reject `balance` on update; parse `opening_balance`; map new sentinels. |
| `be/internal/http/server.go` | Route `POST /api/accounts/{id}/adjustments`. |
| `be/internal/service/chat_accounts.go`, `chat_tools.go` | Rename create arg to `opening_balance`; remove `balance` from update; add `adjust_account_balance` tool. |
| `be/internal/db/transaction_test.go` | Delete `balanceDeltas` tests; add ledger/immutability tests. |
| `be/internal/service/finance_test.go`, `be/internal/http/finance_test.go` | Update fakes and add coverage below. |

## Frontend changes

| File | Change |
|---|---|
| `fe/src/types/finance.ts` | Account adds `opening_balance`, `entry_count`; Transaction adds `kind`, `currency`, nullable category; `CreateAccountRequest` uses `opening_balance`; add `AdjustAccountRequest` and per-currency dashboard types. |
| `fe/src/lib/api.ts` | `adjustAccountBalance(id, data)`. |
| `fe/src/pages/Accounts.tsx` | Wire adjustment modal; refetch after adjustment. |
| `fe/src/components/accounts/AccountFormModal.tsx` | "Opening balance" label, helper text, negative allowed. |
| `fe/src/components/accounts/AccountCard.tsx` | Current balance read-only; opening-balance subline; Edit name only; "Adjust balance" action; currency lock indicator. |
| `fe/src/components/accounts/AdjustBalanceModal.tsx` | New: current balance, target balance, computed delta preview, optional note. |
| `fe/src/components/transactions/TransactionList.tsx` | Format each row with `transaction.currency`; render `opening`/`adjustment` rows as locked with no edit/delete. |
| `fe/src/components/transactions/TransactionFormModal.tsx` | Show the selected account's currency next to Amount. |
| `fe/src/pages/Transactions.tsx` | Remove the single global currency; keep import dialog fallback. |
| `fe/src/pages/Dashboard.tsx` | Render per-currency balances; per-currency monthly breakdown; recent rows use their own currency; note that budgets/reports assume one currency when more than one exists. |
| `fe/src/stores/accountStore.ts` | Add `adjustBalance` and update state with the returned account. |
| Tests | Update Accounts, Transactions, Dashboard, and store tests. |

## Multi-currency contract

`DashboardSummary` becomes:

```go
type CurrencyTotal struct {
    Currency string  `json:"currency"`
    Total    float64 `json:"total"`
}

type MonthlyCurrencyTotal struct {
    Currency string  `json:"currency"`
    Income   float64 `json:"income"`
    Expense  float64 `json:"expense"`
    Net      float64 `json:"net"`
}

type DashboardSummary struct {
    Balances           []CurrencyTotal        `json:"balances"`
    Monthly            []MonthlyCurrencyTotal `json:"monthly"`
    TransactionCount   int                    `json:"transaction_count"`
    RecentTransactions []Transaction          `json:"recent_transactions"`
}
```

- Balances are aggregated in Go from `ListByUser` (each account already carries its derived balance), grouped by `currency`, sorted by currency code.
- Monthly totals are grouped by `accounts.currency` in SQL:

```sql
SELECT a.currency,
       COALESCE(SUM(CASE WHEN t.type = 'income' THEN t.amount ELSE 0 END), 0) AS income,
       COALESCE(SUM(CASE WHEN t.type = 'expense' THEN t.amount ELSE 0 END), 0) AS expense
FROM transactions t
JOIN accounts a ON a.id = t.account_id
WHERE t.user_id = $1
  AND t.kind = 'normal'
  AND t.date >= DATE_TRUNC('month', $2::date)
  AND t.date < DATE_TRUNC('month', $2::date) + INTERVAL '1 month'
GROUP BY a.currency
ORDER BY a.currency
```

- The dashboard hero lists every currency balance; there is no cross-currency total.
- The monthly section renders one row/card per currency. With a single currency the existing stat cards and bar chart stay; with multiple currencies the bars are replaced by the per-currency breakdown.
- Recent transactions and the transaction list format each row with `transaction.currency`.
- `BudgetSection` keeps a `primaryCurrency` (largest balance) and shows an inline note when more than one currency exists: "Budgets and reports currently assume a single currency."
- Reports and budgets remain single-currency and are the next follow-up plan.

## Concurrency

- Adjustment: begin a transaction, `SELECT ... FROM accounts WHERE id = $1 AND user_id = $2 FOR UPDATE`, compute the current derived sum, insert the delta entry, commit, then re-read the account.
- Transaction create/update/delete: lock the affected account row(s) before writing, in sorted order for updates that move between accounts.
- Batch import: lock all affected accounts in sorted order before inserting.
- This serializes adjustment against concurrent writes so an adjustment cannot compute a stale current balance.

## Chat

- `create_account` takes `name`, `opening_balance`, `currency`.
- `update_account` takes `account`, optional `name`, optional `currency`; `balance` is removed.
- New `adjust_account_balance` takes `account`, `target_balance`, optional `description`; it calls `AdjustAccountBalance` directly (no pending-action token, matching other non-destructive updates).
- Tool descriptions and the system prompt explain that an opening balance creates a locked opening entry and that corrections create adjustment entries.

## Tests

Backend:

- Migration/backfill: positive, negative, zero, account with no history, backdated transactions, idempotent re-run.
- Create account: opening 1000 creates one `opening` income entry and derived balance 1000; opening 0 creates no entry; negative opening creates an expense entry.
- Derived balance: create/update/delete transaction; move a transaction between accounts; import batch; recurring run; adjustment.
- System entries: update/delete rejected; reports, budgets, monthly totals, dashboard recent/count, and CSV export exclude them.
- Currency: change allowed with zero entries; 409 with entries.
- Adjustment: delta calculation, zero delta creates no entry, negative target works.
- API: new route, 400 for `balance` on update, 409 mapping, response shape.
- Multi-currency: balances and monthly totals return separate rows; transaction response includes the account currency.

Frontend:

- Accounts: create sends `opening_balance`; negative opening accepted; inline balance edit removed; adjust modal previews the delta and calls the new endpoint; currency lock shown.
- Transactions: system rows show a lock and no actions; each row uses its own currency.
- Dashboard: multiple currency balances render without a combined total; monthly breakdown per currency; recent rows use their own currency.
- Stores: adjustment updates the account in state.

The repository has no integration-test database harness today, so the migration/backfill check is a documented manual verification against a seeded database before merge.

## Rollout and compatibility

- This is a breaking schema/API change. Deploy backend and frontend together; do not run an old backend against the new schema (the `balance` column is gone).
- Back up the database before applying migration `005`.
- The down migration restores a usable stored balance, but it cannot restore the removed column values independently of the ledger; it recomputes them.
- No rolling deploy is supported for this change.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Derived balance adds aggregation cost | Existing `idx_transactions_account_id` supports the join; personal-finance volumes are small. Add `(account_id, kind)` for analytics. |
| Nullable category breaks a query that still uses `JOIN` | Audit every transaction query; add tests for system rows in list, get, create, update, and export. |
| Users mistake the opening entry for income | Lock icon and distinct "Opening"/"Adjustment" label; excluded from income totals; helper text on creation and adjustment. |
| Currency lock frustrates users who picked the wrong currency | Clear helper at creation; recreate the account (delete/import) until an FX conversion feature exists. |
| Reports/budgets still mix currencies | Documented follow-up; dashboard shows a visible note when more than one currency exists. |
| CSV export omits system entries | Intentional to preserve import round-trip; the account card still shows the full balance and history. |

## Acceptance criteria

1. Creating an account with opening 1000 produces one locked `opening` income entry; the card shows current 1000 and opening 1000.
2. Adding an expense of 100 shows 900; deleting it shows 1000 again.
3. Adjusting to 950 creates an adjustment entry for the difference; deleting an older transaction afterwards changes the total by exactly that transaction's signed amount, never by a stale overwrite.
4. An existing account with stored balance `B` and existing net `N` gets an opening entry of `B - N`; after migration its derived balance is exactly `B`.
5. A negative opening balance is accepted and displayed, stored as an expense entry, and excluded from income analytics.
6. Reports, budgets, dashboard monthly totals/recent list, and CSV export exclude opening/adjustment entries.
7. `PATCH /api/accounts/{id}` cannot change the balance; `POST /api/accounts/{id}/adjustments` records corrections.
8. Currency changes are rejected once the account has an entry.
9. The dashboard returns one balance and one monthly row per currency; no two currencies are added together.
10. Every transaction row is formatted in its own account currency.

## Out of scope

- Reports (cashflow/category/merchant) and budgets per currency. They continue to aggregate across currencies until a follow-up plan.
- FX conversion and historical exchange rates.
- A dedicated adjustment-history screen; adjustment rows are visible in the normal transaction history.
- Category deletion UX. With derived balances it no longer corrupts the total, but deleting a category still cascades its normal transactions silently.
