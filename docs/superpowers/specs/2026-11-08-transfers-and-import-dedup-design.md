# Transfers & Import Deduplication — Design

Date: 2026-11-08
Status: Draft — pending user review.

## Problem

Two gaps remain after the currency pass:

1. **No transfer primitive.** Moving money between accounts requires creating an income entry on one
   account and an unrelated expense entry on the other. The balances end up right, but the two entries
   are uncorrelated: deleting one breaks the pair, editing one is allowed, and reports cannot tell they
   are the same movement.
2. **Re-importing a file duplicates the ledger.** The import preview validates rows but never checks
   whether an identical row already exists, so uploading the same workbook twice silently doubles it.

## Decisions

| # | Decision | Choice | Rationale |
|---|---|---|---|
| 1 | Transfer modelling | Two `transactions` rows with `kind = 'transfer'` and a shared `transfer_group_id` | Reuses the ledger, the derived balance and the existing `kind` exclusions. No new balance path. |
| 2 | Leg shape | Source row `type='expense'`, destination row `type='income'`, both `amount = ABS(value)`, `category_id = NULL` | Balance derivation already works; category NULL keeps transfers out of category reports for free. |
| 3 | Atomicity | Both legs inserted in one DB transaction, both accounts locked in sorted-UUID order | Matches `CreateBatch`/`Update` locking and prevents deadlocks. |
| 4 | Cross-currency | Rejected with 400 | A transfer between USD and IDR needs an FX rate and a gain/loss policy. Out of scope until FX exists. |
| 5 | Editing | Transfers are immutable, like opening/adjustment entries; delete + recreate to change one | Extending `PATCH /transactions/{id}` to move both legs is a second write path with no current need. |
| 6 | Deletion | `DELETE /api/transfers/{group_id}` deletes both legs | One operation, no orphan. |
| 7 | Account deletion | 409 when the account has any transfer leg | Cascade would delete one leg and leave the other, corrupting the counterpart balance. |
| 8 | Analytics | Transfers excluded from reports, budgets, dashboard monthly totals and CSV export | `kind = 'normal'` already excludes them once the transfer kind is added. Balances include them. |
| 9 | Duplicate detection | SHA-256 fingerprint over user + date + type + amount(2dp) + trimmed description + account name, computed in Go from parsed rows | Deterministic, no model involvement, no schema change. |
| 10 | Duplicate behaviour | Preview reports duplicates; commit skips them unless `allow_duplicates=true`; within one file the first occurrence wins | Safe default: uploads are idempotent. |
| 11 | Dedup scope | `kind='normal'` rows only | Opening/adjustment/transfer rows are system-generated and must never block an import. |

## Data model — migration 008

`transactions`:

- `ADD COLUMN transfer_group_id UUID`, nullable.
- Replace the `kind` CHECK with one that also allows `'transfer'`.
- Add a partial index: `CREATE INDEX ... ON transactions(user_id, transfer_group_id) WHERE transfer_group_id IS NOT NULL`.

Down: drop the index and column, restore the original CHECK (rows with `kind='transfer'` must be
deleted first — the down migration must state that it is lossy).

## API

- `POST /api/transfers`
  body `{ "from_account_id": "...", "to_account_id": "...", "amount": 100.00, "date": "YYYY-MM-DD", "description": "optional" }`
  - 400 when the accounts are the same, amount <= 0, date invalid, or currencies differ.
  - 404 when either account is not owned by the caller.
  - 201 with both created transactions and the `transfer_group_id`.
- `DELETE /api/transfers/{group_id}` — 204; 404 when no leg belongs to the caller.
- `DELETE /api/accounts/{id}` — new 409 when a transfer leg exists on the account.
- `Transaction` gains `transfer_group_id` (nullable string).

## Frontend

- Accounts page: a `Transfer` action opening a small modal (from account, to account, amount, date,
  description). Only same-currency pairs are offered; if none exists, the action is disabled with a hint.
- Transactions list: transfer rows render a "Transfer" badge instead of a category, and no edit action.
  A delete on either leg deletes the pair (via `DELETE /api/transfers/{group_id}`).
- Import dialog: the preview shows "N duplicates will be skipped"; a checkbox "Import duplicates anyway"
  maps to `allow_duplicates`.

## Edge cases

- Deleting an account that only has non-transfer entries keeps the current cascade behaviour.
- A transfer where one account is later deleted is impossible because deletion is blocked while a leg exists.
- Re-running the same import after deleting rows in between correctly re-imports only the missing rows.
- Two genuinely identical transactions on the same day (same amount, description, account) are
  indistinguishable by design; the "import duplicates anyway" flag is the escape hatch.

## Testing

- Transfers: create updates both balances by the exact amount; atomic rollback when one insert fails;
  same-account, cross-currency and foreign-account rejections; pair deletion; account-delete 409.
- Dedup: preview flags an existing row; commit skips it; `allow_duplicates` imports it; in-file duplicates
  keep the first; different account/description/amount are not duplicates.
- Regression: transfers do not appear in reports, budgets, dashboard monthly totals or CSV export.

## Out of scope

FX conversion, cross-currency transfers, editing a transfer in place, split transactions, receipts and
other attachments.
