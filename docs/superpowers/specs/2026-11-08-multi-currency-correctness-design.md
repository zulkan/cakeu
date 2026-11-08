# Multi-Currency Correctness — Design

Date: 2026-11-08
Status: Approved (user selected "group by currency everywhere" on 2026-11-08)

## Problem

The dashboard already separates balances and monthly totals per currency, but the rest of the
analytics surface does not:

- `GET /api/reports/cashflow` sums every transaction in the range regardless of account currency
  (`be/internal/db/report.go:25-34`).
- `GET /api/reports/categories` sums totals per category across currencies and computes `share`
  against that mixed total (`be/internal/db/report.go:57-65`, `be/internal/service/report.go:67-76`).
- `GET /api/reports/merchants` sums descriptions across currencies and applies one global limit
  (`be/internal/db/report.go:88-96`).
- Budgets hold one amount per `(user, category, month)` while spending is summed across every
  account (`be/internal/db/budget.go:102-111`), so a USD budget is compared against USD + IDR + EUR.

Separately, the cashflow and merchant queries never filter `transactions.kind`. Opening and
adjustment entries therefore count as income/expense in those two reports, which contradicts the
ledger design (`docs/superpowers/specs/2026-11-07-ledger-backed-account-balances-design.md`,
decision 6). Category totals accidentally exclude them only because system entries have a NULL
category.

## Decisions

| # | Decision | Choice | Rationale |
|---|---|---|---|
| 1 | Currency source | Derived from the owning account; no `currency` column on `transactions` | Account currency is already authoritative and locked once entries exist. Denormalizing adds a backfill and a second source of truth for no benefit until FX exists. |
| 2 | Report shape | Keep arrays; add a `currency` field to every row | Smallest breaking change. Clients group rows; no nested envelope to migrate. |
| 3 | Budget identity | `(user_id, category_id, month, currency)` | A budget is an amount in one currency. Mixing is the bug. |
| 4 | CSV export | Unchanged six columns | Account names are unique per user, so the account name already identifies the currency. Adding a column would break the documented import round-trip. |
| 5 | Aggregate rule | No query and no UI element may sum across currencies | Same rule the dashboard already follows. |
| 6 | System entries | Add `t.kind = 'normal'` to cashflow, merchant and budget-spending queries | Restores the documented ledger contract. |
| 7 | Merchant limit | Apply `limit` per currency | A 10-row limit over three currencies would otherwise hide two currencies entirely. |
| 8 | FX conversion | Out of scope | Needs a rate source and a reporting-currency policy. Not required for correctness. |

## Data model — migration 007

`budgets`:

1. `ADD COLUMN currency VARCHAR(3)` (nullable first).
2. Backfill each existing row with the most frequent currency among that user's expense
   transactions in the same category and month; if none, the currency of the user's earliest
   account; if the user has no accounts, `'USD'`.
3. `ALTER COLUMN currency SET NOT NULL`.
4. Drop the old unique constraint on `(user_id, category_id, month)`.
5. Add unique `(user_id, category_id, month, currency)`.
6. Add index `(user_id, month, currency)`.

Down migration is lossy and must say so: keep the largest-amount row per
`(user_id, category_id, month)`, delete the duplicates, drop the new constraint/index/column, and
restore the old unique constraint. Losing one currency's budget amount on rollback is acceptable;
silently failing to restore the unique constraint is not.

## Query changes

- Cashflow: join `accounts a`, filter `t.kind = 'normal'`, group by `a.currency` and month.
- Category totals: join `accounts a`, filter `t.kind = 'normal'`, group by currency, category, type.
- Merchants: join `accounts a`, filter `kind = 'normal'`, rank rows per currency with
  `ROW_NUMBER() OVER (PARTITION BY a.currency ORDER BY SUM(t.amount) DESC)`, keep rank <= limit.
- Budget spending: join `accounts a` and filter `a.currency = $currency` in addition to the
  existing user/month/type predicates; add `kind = 'normal'`.

Share percentages in `reportService.Categories` must be computed per `(currency, type)`, not per
`type` alone.

## API changes

- `GET /api/reports/cashflow` → `[{ "currency": "USD", "period": "2026-11", "income": 0, "expense": 0, "net": 0 }]`
  — one row per `(currency, period)`.
- `GET /api/reports/categories` → adds `"currency"`; `share` is within `(currency, type)`.
- `GET /api/reports/merchants` → adds `"currency"`; `limit` applies per currency.
- `GET /api/budgets?month=YYYY-MM` → status rows add `"currency"`.
- `PUT /api/budgets` → requires `"currency"` (3-letter code). Omitting it is a 400, not a default.
- `DELETE /api/budgets/{category_id}?month=&currency=` → `currency` required.
- Dashboard, transactions, accounts and import are unchanged.

## Frontend

- `fe/src/types/report.ts` and `fe/src/types/budget.ts` gain `currency`.
- `Reports.tsx` gains a currency selector built from the distinct currencies in the response,
  defaulting to the currency with the most rows. All three report components receive only the
  selected currency's rows. When more than one currency exists, show "Showing USD — 2 other
  currencies available"; never render a combined total.
- `CashflowChart`, `CategoryBreakdown` and `TopMerchants` keep their current props; the page
  filters before passing data.
- `BudgetSection` and `BudgetEditorModal` gain a currency selector populated from the distinct
  account currencies. The editor defaults to the currency of the existing budget being edited,
  otherwise the first account currency. Saving sends `currency`.
- Empty state when the selected currency has no rows: reuse `EmptyState`.

## Edge cases

- A currency with no budgets simply shows no budget rows; it is not an error.
- Deleting every account in a currency leaves that currency's budgets intact and reporting zero
  spending. Acceptable and explainable.
- Backfill ambiguity for historical budgets is documented in the migration; there is no perfect
  answer because the old data never recorded a currency.
- A user with one currency sees byte-identical behavior apart from the added `currency` field.
- Opening and adjustment entries disappear from cashflow/merchants after the kind filter. This is a
  behavior change, and it is the intended one.

## Testing

- Service: share computed per currency; cashflow `net` per row; merchant limit per currency.
- Repository (Postgres): fixture with USD and IDR rows — each aggregate returns separate rows and
  never a cross-currency sum; opening/adjustment rows are excluded.
- Handler: response contains `currency`; `PUT /api/budgets` without `currency` returns 400.
- Frontend: selector filters rows; no component receives mixed-currency data; budget save sends
  `currency`; existing report/budget tests updated for the new field.
- Acceptance: `go test ./...` (except the known pre-existing timeout test), `npx vitest run`,
  `npm run lint`, `npm run build` all pass.

## Out of scope

FX rates, a single reporting currency, net-worth-in-one-currency, per-transaction currency overrides,
and currency display formatting beyond the existing per-account formatting.
