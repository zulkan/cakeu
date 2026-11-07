# Ledger-Backed Account Balances Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the mutable `accounts.balance` with a ledger-derived balance: an opening entry at account creation, locked adjustment entries for corrections, and per-currency dashboard sums.

**Architecture:** `transactions.kind` distinguishes `normal`, `opening`, and `adjustment` entries. `accounts.balance` is dropped; account reads aggregate the ledger. Opening/adjustment entries are visible in history but excluded from analytics, budgets, reports, and CSV export. Dashboard balances and monthly totals are grouped by `accounts.currency`; each transaction response carries its account currency.

**Tech Stack:** Go 1.26, pgx v5, golang-migrate, `http.ServeMux`; React 19, TypeScript 6, Vite 8, Tailwind 4, zustand, react-hook-form, zod, vitest.

## Global Constraints

- Backend commands run from `be/`; frontend commands run from `fe/`.
- Migration files are `be/internal/migrations/005_ledger_backed_balances.{up,down}.sql`.
- `gofmt` is the only Go formatting rule. `npm run lint` uses `--max-warnings 0`.
- Do not use `as any`, `@ts-ignore`, or `@ts-expect-error`.
- Never stage or commit unrelated working-tree changes (`be/internal/service/import.go`, `import_test.go`, `smart_import_test.go`, `be/tmp_inspect/`, `budgeting 2026.xlsx`).
- Do not commit or push unless the user asks. The plan includes commit steps as checkpoints; skip them if the user has not authorized new commits.
- `CategoryID` stays `uuid.UUID` in the Go model; system entries use the zero UUID and are detected by `kind`.
- System entry kinds: `normal`, `opening`, `adjustment`.
- System entries are immutable via API; reports/budgets/export/dashboard-monthly exclude them.
- Currency locks when `entry_count > 0`.

---

### Task 1: Migration and model contracts

**Files:**
- Create: `be/internal/migrations/005_ledger_backed_balances.up.sql`
- Create: `be/internal/migrations/005_ledger_backed_balances.down.sql`
- Modify: `be/internal/model/finance.go`
- Test: `be/internal/model` has no tests; migration is verified manually against a seeded DB.

**Interfaces:**
- Produces: `model.Account.OpeningBalance`, `model.Account.EntryCount`, `model.Transaction.Kind`, `model.Transaction.Currency`, `model.CurrencyTotal`, `model.MonthlyCurrencyTotal`, `model.AdjustAccountRequest`, `model.CreateAccountRequest.OpeningBalance`, `model.UpdateAccountRequest.Balance` (deprecated guard), `model.TransactionFilters.Kind`.

- [ ] **Step 1: Write the up migration**

Use the SQL in the spec (`docs/superpowers/specs/2026-11-07-ledger-backed-account-balances-design.md`, section “Migration 005”): add `kind`, make `category_id` nullable, backfill opening entries with the `NOT EXISTS` guard, drop `accounts.balance`, add indexes.

- [ ] **Step 2: Write the down migration**

Re-add `accounts.balance`, recompute from all entries, delete `kind <> 'normal'`, drop `kind`, restore `category_id NOT NULL`.

- [ ] **Step 3: Update the Go models**

```go
type Account struct {
	ID             uuid.UUID `json:"id"`
	UserID         uuid.UUID `json:"user_id"`
	Name           string    `json:"name"`
	Balance        float64   `json:"balance"`
	OpeningBalance float64   `json:"opening_balance"`
	EntryCount     int       `json:"entry_count"`
	Currency       string    `json:"currency"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Transaction struct {
	// existing fields...
	Kind         string    `json:"kind"`
	CategoryID   uuid.UUID `json:"category_id"`
	CategoryName string    `json:"category_name"`
	Currency     string    `json:"currency"`
}

type CreateAccountRequest struct {
	Name           string  `json:"name" example:"Checking Account"`
	OpeningBalance float64 `json:"opening_balance" example:"1000.00"`
	Currency       string  `json:"currency" example:"USD"`
}

type UpdateAccountRequest struct {
	Name     *string  `json:"name,omitempty" example:"Savings Account"`
	Currency *string  `json:"currency,omitempty" example:"EUR"`
	Balance  *float64 `json:"balance,omitempty"` // rejected: use the adjustment endpoint
}

type AdjustAccountRequest struct {
	TargetBalance float64 `json:"target_balance" example:"2500.00"`
	Description   string  `json:"description,omitempty" example:"Matched bank statement"`
}

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

Add `Kind string \`json:"kind,omitempty"\`` to `TransactionFilters`.

- [ ] **Step 4: Run `gofmt` and commit the migration/model step**

Run: `cd be && gofmt -w internal/model/finance.go`
Expected: no output, exit 0.

---

### Task 2: Repository interfaces and sentinels

**Files:**
- Modify: `be/internal/service/finance.go`

**Interfaces:**
- Produces: `AccountRepository.Create(ctx, userID, name, openingBalance, currency)`, `AccountRepository.Update(ctx, userID, id, name, currency)`, `AccountRepository.AdjustBalance(ctx, userID, id, targetBalance, description)`, `TransactionRepository.MonthlyTotalsByCurrency(...)`, `AccountService.AdjustAccountBalance(...)`, and the new sentinel errors.

- [ ] **Step 1: Add constants and errors**

```go
const (
	EntryKindNormal     = "normal"
	EntryKindOpening    = "opening"
	EntryKindAdjustment = "adjustment"
)

var (
	ErrUseAdjustmentEndpoint = errors.New("use the account adjustment endpoint to change a balance")
	ErrCurrencyLocked        = errors.New("currency is locked after the first entry")
	ErrSystemEntryImmutable  = errors.New("system entries cannot be modified")
	ErrInvalidBalance        = errors.New("balance must be a finite number")
)
```

- [ ] **Step 2: Change the interfaces**

```go
type AccountRepository interface {
	Create(ctx context.Context, userID uuid.UUID, name string, openingBalance float64, currency string) (*model.Account, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Account, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Account, error)
	Update(ctx context.Context, userID uuid.UUID, id uuid.UUID, name *string, currency *string) (*model.Account, error)
	AdjustBalance(ctx context.Context, userID uuid.UUID, id uuid.UUID, targetBalance float64, description string) (*model.Account, error)
	Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
}

type TransactionRepository interface {
	// existing methods, with MonthlyTotals replaced by:
	MonthlyTotalsByCurrency(ctx context.Context, userID uuid.UUID, month time.Time) ([]model.MonthlyCurrencyTotal, error)
}

type AccountService interface {
	// existing methods, plus:
	AdjustAccountBalance(ctx context.Context, userID uuid.UUID, id uuid.UUID, req model.AdjustAccountRequest) (*model.Account, error)
}
```

- [ ] **Step 3: Compile to see the expected breakage**

Run: `cd be && go build ./...`
Expected: failures in `internal/db`, `internal/http`, and tests. Keep the list; it is the work queue for the next tasks.

---

### Task 3: Account repository (derived balance, atomic create/adjust)

**Files:**
- Modify: `be/internal/db/finance.go`

**Interfaces:**
- Consumes: interfaces from Task 2.
- Produces: derived `Account` reads, atomic create-with-opening, atomic adjustment, currency lock.

- [ ] **Step 1: Replace `AccountRepo.Create`**

Begin a transaction; insert `accounts (user_id, name, currency)`; if `openingBalance != 0`, insert a `transactions` row with `category_id NULL`, `type` income/expense by sign, `amount ABS(openingBalance)`, `description 'Opening balance'`, `date CURRENT_DATE`, `kind 'opening'`. Commit, then `return r.GetByID(ctx, id)`.

- [ ] **Step 2: Replace `GetByID` and `ListByUser` with derived queries**

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
WHERE a.id = $1
GROUP BY a.id
```

`ListByUser` is the same with `WHERE a.user_id = $1 GROUP BY a.id ORDER BY a.created_at ASC, a.name ASC`.

- [ ] **Step 3: Replace `Update`**

Remove the balance parameter. Use an atomic currency guard:

```sql
UPDATE accounts a
SET name = COALESCE($3::varchar, a.name),
    currency = COALESCE($4::varchar, a.currency),
    updated_at = NOW()
WHERE a.id = $1 AND a.user_id = $2
  AND ($4::varchar IS NULL OR NOT EXISTS (
      SELECT 1 FROM transactions t WHERE t.account_id = a.id
  ))
RETURNING a.id
```

If no row is returned, check `SELECT 1 FROM accounts WHERE id = $1 AND user_id = $2`: no row → `ErrAccountNotFound`; row exists → `ErrCurrencyLocked`. Return `r.GetByID`.

- [ ] **Step 4: Add `AdjustBalance`**

Begin a transaction; `SELECT id FROM accounts WHERE id = $1 AND user_id = $2 FOR UPDATE`; compute the signed sum of all entries; round the delta to 2 decimals; if nonzero, insert a `kind='adjustment'` entry (`category_id NULL`, type by sign, `description` default `"Balance adjustment"`, `date CURRENT_DATE`); commit; return `r.GetByID`.

- [ ] **Step 5: Run `go build ./internal/db`**

Expected: only transaction-balance removal errors remain (Task 4).

---

### Task 4: Transaction repository (kind, currency, remove balance writes)

**Files:**
- Modify: `be/internal/db/finance.go`
- Delete: `TestBalanceDeltas` in `be/internal/db/transaction_test.go`

**Interfaces:**
- Produces: normal-entry CRUD with `kind`/`currency`, system-entry protection, normal-only export/analytics.

- [ ] **Step 1: Add a lock helper**

```go
func lockAccounts(ctx context.Context, tx pgx.Tx, userID uuid.UUID, ids []uuid.UUID) error {
	sorted := append([]uuid.UUID(nil), ids...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].String() < sorted[j].String() })
	for _, id := range sorted {
		if _, err := tx.Exec(ctx, `SELECT 1 FROM accounts WHERE id = $1 AND user_id = $2 FOR UPDATE`, id, userID); err != nil {
			return fmt.Errorf("lock account: %w", err)
		}
	}
	return nil
}
```

- [ ] **Step 2: `Create`**

Lock the account row, insert with `kind` (default `normal`), return `kind` and `currency`. Fetch account name/currency/category name with:

```sql
SELECT a.name, a.currency, COALESCE(c.name, '')
FROM accounts a
LEFT JOIN categories c ON c.id = $2
WHERE a.id = $1
```

Delete the `UPDATE accounts SET balance` block.

- [ ] **Step 3: `Update` and `Delete`**

- `Update`: select the existing row `FOR UPDATE` including `kind`; if `kind != 'normal'` return `ErrSystemEntryImmutable`. Lock old/new accounts. Keep the transaction `UPDATE`; remove all balance deltas.
- `Delete`: select `kind`; reject non-normal; lock the account; delete; remove the balance restore block.

- [ ] **Step 4: `List`, `GetByID`, `CreateBatch`, `ForEachExportRow`**

- `List`/`GetByID`: `LEFT JOIN categories`, select `t.kind`, `a.currency`, and `COALESCE(t.category_id, '00000000-0000-0000-0000-000000000000'::uuid)`.
- Add `if filters.Kind != "" { addCondition("t.kind = $%d", filters.Kind) }`.
- `CreateBatch`: insert normal entries only; lock all affected accounts in sorted order; remove balance updates.
- `ForEachExportRow`: add `AND t.kind = 'normal'`.

- [ ] **Step 5: Replace `MonthlyTotals`**

```sql
SELECT a.currency,
       COALESCE(SUM(CASE WHEN t.type = 'income' THEN t.amount ELSE 0 END), 0),
       COALESCE(SUM(CASE WHEN t.type = 'expense' THEN t.amount ELSE 0 END), 0)
FROM transactions t
JOIN accounts a ON a.id = t.account_id
WHERE t.user_id = $1
  AND t.kind = 'normal'
  AND t.date >= DATE_TRUNC('month', $2::date)
  AND t.date < DATE_TRUNC('month', $2::date) + INTERVAL '1 month'
GROUP BY a.currency
ORDER BY a.currency
```

Return `[]model.MonthlyCurrencyTotal`.

- [ ] **Step 6: Delete `balanceDeltas` and its test**

Remove the function and `be/internal/db/transaction_test.go`'s `TestBalanceDeltas`; replace with a pure test for `lockAccounts` ordering only if it can be tested without a DB (otherwise delete the file).

- [ ] **Step 7: Add `kind = 'normal'` to analytics**

`be/internal/db/report.go`: add `AND t.kind = 'normal'` to cashflow, category, merchant queries.
`be/internal/db/budget.go`: add `AND kind = 'normal'` to `SpentByCategory`.

- [ ] **Step 8: Run `go build ./internal/db`**

Expected: success.

---

### Task 5: Service layer (create, update, adjust, dashboard)

**Files:**
- Modify: `be/internal/service/finance.go`
- Modify: `be/internal/service/import.go` (no change if `CreateBatch` handles kinds)
- Test: `be/internal/service/finance_test.go`

**Interfaces:**
- Consumes: repositories from Tasks 3–4.
- Produces: `CreateAccount` with opening, `UpdateAccount` guard, `AdjustAccountBalance`, normal-only dashboard.

- [ ] **Step 1: `CreateAccount`**

Validate the name and currency; reject NaN/Inf opening with `ErrInvalidBalance`; call `repo.Create(ctx, userID, name, req.OpeningBalance, currency)`.

- [ ] **Step 2: `UpdateAccount`**

If `req.Balance != nil` return `ErrUseAdjustmentEndpoint`. Validate name/currency. Call `repo.Update(ctx, userID, id, req.Name, req.Currency)`. Map `ErrCurrencyLocked`.

- [ ] **Step 3: `AdjustAccountBalance`**

Validate finite target; trim `description`; call `repo.AdjustBalance`. Map not-found.

- [ ] **Step 4: `UpdateTransaction` / `DeleteTransaction`**

- In `UpdateTransaction`, after the ownership check: `if existing.Kind != "" && existing.Kind != EntryKindNormal { return nil, ErrSystemEntryImmutable }`. Treat empty kind as normal for old fakes.
- `DeleteTransaction`: rely on the repository's `ErrSystemEntryImmutable`; map it unchanged.

- [ ] **Step 5: `GetDashboardSummary`**

```go
accounts, err := s.accounts.ListByUser(ctx, userID)
byCurrency := map[string]float64{}
for _, a := range accounts { byCurrency[a.Currency] += a.Balance }
balances := make([]model.CurrencyTotal, 0, len(byCurrency))
for currency, total := range byCurrency { balances = append(balances, model.CurrencyTotal{Currency: currency, Total: total}) }
sort.Slice(balances, func(i, j int) bool { return balances[i].Currency < balances[j].Currency })

monthly, err := s.repo.MonthlyTotalsByCurrency(ctx, userID, time.Now().UTC())
for i := range monthly { monthly[i].Net = monthly[i].Income - monthly[i].Expense }
if monthly == nil { monthly = []model.MonthlyCurrencyTotal{} }

now := time.Now().UTC()
start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
end := start.AddDate(0, 1, 0).AddDate(0, 0, -1)
recent, _, err := s.repo.List(ctx, model.TransactionFilters{UserID: &userID, Kind: EntryKindNormal, Page: 1, PageSize: 10})
_, count, err := s.repo.List(ctx, model.TransactionFilters{UserID: &userID, Kind: EntryKindNormal, DateFrom: &start, DateTo: &end, Page: 1, PageSize: 1})
```

Return `DashboardSummary{Balances: balances, Monthly: monthly, TransactionCount: count, RecentTransactions: recent}` with non-nil slices.

- [ ] **Step 6: Update the service tests**

- Rename `fakeAccountRepo.lastCreate.balance` → `openingBalance`; update `Create`/`Update` signatures; add `AdjustBalance`.
- Add `fakeTransactionRepo.MonthlyTotalsByCurrency` returning `[]model.MonthlyCurrencyTotal`.
- Update `TestAccountServiceCreateAccountDefaultsCurrency` to use `OpeningBalance`.
- Add tests: opening create, negative opening, adjustment delta, currency lock mapping, system-entry update rejection, dashboard per-currency balances/monthly/count.
- Update `TestTransactionServiceGetDashboardSummary` for the new fields.

- [ ] **Step 7: Run `go test ./internal/service`**

Expected: PASS.

---

### Task 6: HTTP API and chat tools

**Files:**
- Modify: `be/internal/http/finance.go`
- Modify: `be/internal/http/server.go`
- Modify: `be/internal/http/finance_test.go`
- Modify: `be/internal/service/chat.go`, `chat_accounts.go`, `chat_tools.go`
- Modify: `be/internal/service/chat_test.go`

**Interfaces:**
- Consumes: `AccountService.AdjustAccountBalance`.
- Produces: `POST /api/accounts/{id}/adjustments`, `opening_balance` create, `balance` update rejection, dashboard arrays, chat adjustment tool.

- [ ] **Step 1: Add the service method to the HTTP interface**

```go
AdjustAccountBalance(ctx context.Context, userID uuid.UUID, id uuid.UUID, req model.AdjustAccountRequest) (*model.Account, error)
```

- [ ] **Step 2: Add the handler and route**

```go
apiMux.HandleFunc("POST /api/accounts/{id}/adjustments", handleAdjustAccountBalance(accounts))
```

The handler parses the path UUID and `model.AdjustAccountRequest`, calls the service, and returns 200 with the account.

- [ ] **Step 3: Guard create/update**

- Create: decode with `decoder.DisallowUnknownFields()` so `balance` is rejected.
- Update: `if req.Balance != nil { respondError(w, 400, service.ErrUseAdjustmentEndpoint.Error()); return }`.
- Update Swagger comments: create uses `opening_balance`; the adjustment endpoint is documented.

- [ ] **Step 4: Extend `respondServiceError`**

Map `ErrUseAdjustmentEndpoint` and `ErrInvalidBalance` to 400; `ErrCurrencyLocked` and `ErrSystemEntryImmutable` to 409.

- [ ] **Step 5: Chat tools**

- `create_account`: `opening_balance` (number, no minimum).
- `update_account`: remove `balance`; description “Rename an account or change its currency (before the first entry).”
- Add `adjust_account_balance` with `account`, `target_balance`, optional `description`.
- `executeTool` case and `ChatService.adjustAccountBalance` implementation.
- Update the system prompt with the opening/adjustment rule.

- [ ] **Step 6: Update HTTP/chat tests**

- `fakeAccountService` adds `AdjustAccountBalance`; create test asserts `OpeningBalance`.
- Add handler tests: create with `balance` → 400; update with `balance` → 400; adjustment route returns 200 and forwards the request; currency lock → 409; system entry → 409.
- Update dashboard test to assert `balances` and `monthly`.
- `chat_test.go` fakes use the new `fakeAccountRepo` signatures.

- [ ] **Step 7: Run `go test ./...`**

Expected: PASS.

---

### Task 7: Frontend types, API client, stores

**Files:**
- Modify: `fe/src/types/finance.ts`
- Modify: `fe/src/lib/api.ts`
- Modify: `fe/src/stores/accountStore.ts`
- Test: `fe/src/stores/__tests__/financeStores.test.ts`

**Interfaces:**
- Produces: `Account.opening_balance`, `Account.entry_count`, `Transaction.kind`, `Transaction.currency`, `CreateAccountRequest.opening_balance`, `AdjustAccountRequest`, `CurrencyTotal`, `MonthlyCurrencyTotal`, `DashboardSummary.balances/monthly`, `api.adjustAccountBalance`, `accountStore.adjustBalance`.

- [ ] **Step 1: Update the TypeScript types**

Use the shapes in the spec. Keep `category_id: string` (zero UUID for system entries) and `category_name: string`. `Transaction.kind: 'normal' | 'opening' | 'adjustment'`.

- [ ] **Step 2: Add the API call**

```ts
adjustAccountBalance: (id: string, data: AdjustAccountRequest) =>
  request<Account>(`/accounts/${id}/adjustments`, { method: 'POST', body: JSON.stringify(data) }),
```

- [ ] **Step 3: Add the store action**

```ts
adjustBalance: async (id, data) => {
  const updated = await api.adjustAccountBalance(id, data);
  set((s) => ({ accounts: s.accounts.map((a) => (a.id === id ? updated : a)) }));
},
```

- [ ] **Step 4: Run `cd fe && npx tsc -b`**

Expected: type errors only in the UI files that still use the old fields. Those are fixed in Tasks 8–10.

---

### Task 8: Accounts UI

**Files:**
- Modify: `fe/src/components/accounts/AccountFormModal.tsx`
- Modify: `fe/src/components/accounts/AccountCard.tsx`
- Create: `fe/src/components/accounts/AdjustBalanceModal.tsx`
- Modify: `fe/src/pages/Accounts.tsx`
- Test: `fe/src/pages/__tests__/Accounts.test.tsx`

**Interfaces:**
- Consumes: `accountStore.adjustBalance`, `AdjustAccountRequest`.
- Produces: opening-balance creation, read-only current balance, name-only edit, adjustment flow, currency lock.

- [ ] **Step 1: `AccountFormModal`**

Label the field “Opening balance”; helper text “The amount already in this account before any transactions. It is recorded as a locked opening entry.” Allow negative (`min` removed; zod validates finite only). Submit `opening_balance`.

- [ ] **Step 2: `AdjustBalanceModal`**

Fields: actual/target balance (number, negative allowed), optional note. Show current balance, target, and computed delta. Submit `{ target_balance, description }`. Handle loading/error.

- [ ] **Step 3: `AccountCard`**

- Remove the balance input from the inline editor; edit name only.
- Show current balance read-only, opening balance as a small subline.
- Add “Adjust balance” action that opens the modal.
- Show currency as read-only with a lock hint when `entry_count > 0`.

- [ ] **Step 4: `Accounts.tsx`**

Wire the adjustment modal, call `adjustBalance`, and keep delete behavior.

- [ ] **Step 5: Update tests**

Create test asserts `opening_balance`; add negative opening; remove the inline-balance assertion; add adjust-flow and currency-lock tests.

- [ ] **Step 6: Run `npx vitest run src/pages/__tests__/Accounts.test.tsx`**

Expected: PASS.

---

### Task 9: Transactions UI

**Files:**
- Modify: `fe/src/components/transactions/TransactionList.tsx`
- Modify: `fe/src/components/transactions/TransactionFormModal.tsx`
- Modify: `fe/src/pages/Transactions.tsx`
- Test: `fe/src/pages/__tests__/Transactions.test.tsx`

**Interfaces:**
- Consumes: `Transaction.kind`, `Transaction.currency`.
- Produces: per-row currency, locked system rows, account-currency amount label.

- [ ] **Step 1: `TransactionList`**

Remove the `currency` prop. Format each row with `transaction.currency`. For `kind !== 'normal'`, render a neutral “Opening”/“Adjustment” label instead of the category, and hide the edit/delete buttons (show a lock icon).

- [ ] **Step 2: `TransactionFormModal`**

Show the selected account’s currency next to Amount: `Amount (USD)`. System entries never reach this modal.

- [ ] **Step 3: `Transactions.tsx`**

Remove the single `currency` variable from `TransactionList`. Keep a fallback currency for `ImportCsvDialog` (first account).

- [ ] **Step 4: Update tests**

Assert each row uses its own currency; system rows have no edit/delete buttons.

- [ ] **Step 5: Run `npx vitest run src/pages/__tests__/Transactions.test.tsx`**

Expected: PASS.

---

### Task 10: Dashboard multi-currency

**Files:**
- Modify: `fe/src/pages/Dashboard.tsx`
- Modify: `fe/src/components/dashboard/MonthlyBars.tsx` (or its caller)
- Test: `fe/src/pages/__tests__/Dashboard.test.tsx`

**Interfaces:**
- Consumes: `DashboardSummary.balances`, `DashboardSummary.monthly`, `Transaction.currency`.
- Produces: per-currency balances and monthly breakdown; no cross-currency total; per-row transaction currency.

- [ ] **Step 1: Replace the single total**

Render every `balances` row in the hero. If empty, show `0` in the primary currency.

- [ ] **Step 2: Replace the single monthly section**

With one currency, keep the existing four stat cards and bars. With multiple currencies, render a table/cards with rows per currency: income, expense, net.

- [ ] **Step 3: Recent transactions and budgets**

Format each recent row with `transaction.currency`. Keep `BudgetSection` on a `primaryCurrency` (largest balance) and show the multi-currency note when `balances.length > 1`.

- [ ] **Step 4: Update tests**

Add a multi-currency summary: two balances and two monthly rows, no combined total; assert each recent row uses its own currency.

- [ ] **Step 5: Run `npx vitest run src/pages/__tests__/Dashboard.test.tsx`**

Expected: PASS.

---

### Task 11: Documentation and full verification

**Files:**
- Modify: `AGENTS.md`
- Modify: `docs/superpowers/specs/2026-11-07-ledger-backed-account-balances-design.md` only if implementation details differ.

- [ ] **Step 1: Update `AGENTS.md`**

Document: ledger-derived balances, opening/adjustment entries, `POST /api/accounts/{id}/adjustments`, `opening_balance`, currency lock, per-currency dashboard, system entries excluded from analytics/export, and the reports/budgets limitation.

- [ ] **Step 2: Backend verification**

```bash
cd be
gofmt -l .
go build ./...
go vet ./...
go test ./...
```

Expected: empty `gofmt -l`, exit 0 for all commands.

- [ ] **Step 3: Frontend verification**

```bash
cd fe
npx tsc -b
npm run lint
npx vitest run
npm run build
```

Expected: exit 0; no warnings.

- [ ] **Step 4: Manual migration smoke test**

Against a disposable Postgres with `uuid-ossp`:
1. Apply migrations through 004.
2. Insert a user, an account with `balance=1000`, and transactions with net `+200`.
3. Apply 005.
4. Assert the account’s derived balance is 1000 and one `opening` entry of 800 exists.
5. Apply the down migration and assert `accounts.balance` is 1000 again.

- [ ] **Step 6: Report remaining known gaps**

Reports and budgets still aggregate across currencies; category deletion still cascades transactions silently.
