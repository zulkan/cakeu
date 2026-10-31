# AI Chat CRUD + CSV Import/Export — Design

Date: 2026-11-01
Status: Implemented and verified (2026-11-01).

## Summary

Two features requested together. They share no code and differ sharply in risk, so they are specified as separate phases that can ship independently:

- **Phase A — CSV export/import** for transactions. Deterministic, fully testable, and gives users a data backup before an LLM is allowed to mutate anything.
- **Phase B — AI chat** that performs CRUD by calling the existing finance services as tools.

Both phases depend on one prerequisite fix described below, which is a live security defect today.

## Decisions

| # | Decision | Choice | Rationale |
|---|---|---|---|
| 1 | LLM provider | Hosted OpenAI-compatible **cfrouter**: `LLM_BASE_URL` (default `https://api.cfrouter.com/v1`), `LLM_MODEL` (default `deepseek-v4.1-flash-1017`), `LLM_API_KEY` (required) | Runs on hosted inference, so the app stays deployable on a weak cloud instance with no local model or GPU. Verified working, including tool calling. |
| 2 | How the AI acts | Tool calling over existing services. No generated SQL. | Keeps validation, balance updates, and authorization in Go. Generated SQL would bypass all three. |
| 3 | Destructive actions | Two-step confirmation with a server-side pending-action token. | A model mistake must not be able to delete data in one turn. |
| 4 | CSV scope | Transactions export/import. Account and category referenced by name. | This is the data users actually want to move; accounts/categories are few and stable. |
| 5 | Import safety | Dry-run preview, then explicit commit. No auto-created accounts/categories in v1. | Prevents silent data corruption from typos in a spreadsheet. |
| 6 | Chat history | Stateless server; client sends the recent message window. | No new schema, no conversation retention/PII surface in v1. |
| 7 | Streaming | Non-streaming in v1; SSE listed as a follow-up. | Tool-calling loops are far easier to test deterministically. |

## Prerequisite: IDOR on delete/update (security defect, fix first)

Verified in the current code: deletes and account updates are scoped by ID only.

- `AccountRepo.Delete`: `DELETE FROM accounts WHERE id = $1`
- `CategoryRepo.Delete`: `DELETE FROM categories WHERE id = $1`
- `TransactionRepo.Delete`: transactional balance rollback keyed on `id`
- `AccountRepo.Update`: `UPDATE accounts ... WHERE id = $1`
- Handlers pass no authenticated user into any of these

Any logged-in user can therefore delete or modify another user's records if they obtain a UUID. This must be corrected before an LLM is given delete tools, because otherwise the model — or a prompt-injection payload — becomes an ID-probing oracle.

Fix: thread the authenticated user through the mutating paths and add `AND user_id = $N` to each statement.

- `AccountService.DeleteAccount(ctx, userID, id)`, `UpdateAccount(ctx, userID, id, req)`
- `CategoryService.DeleteCategory(ctx, userID, id)`
- `TransactionService.DeleteTransaction(ctx, userID, id)`
- Repos take `userID` and filter on it; zero rows affected returns the existing not-found sentinel.
- Handlers extract the user from `JWTAuth` context (already available) and pass it down.

This is a small, contained change with unit tests; it should land as its own commit.

## Phase A — CSV export/import

### Export

- `GET /api/transactions/export.csv` (JWT), optional filters reusing `TransactionFilters` (date range, account, category, type).
- Streams `text/csv; charset=utf-8` with `Content-Disposition: attachment; filename="transactions-YYYY-MM-DD.csv"`.
- Fixed header order: `date,type,amount,description,account,category`.
- Dates as `YYYY-MM-DD`; amounts as plain decimal (no currency symbol, 2 dp); account and category as their names.
- Uses a dedicated repository query joining accounts and categories, ordered by date ascending, streamed in batches rather than loading everything into memory.

### Import

Two endpoints so nothing is written blind:

1. `POST /api/transactions/import/preview` — multipart `file`, parses and validates, writes nothing. Returns per-row results and a summary.
2. `POST /api/transactions/import` — multipart `file` plus `confirm=true`, executes rows that passed validation.

Validation per row: required columns present, `type` in `income|expense`, `amount` a positive decimal, `date` parseable, account name exists for this user, category name exists for this user and its `type` matches the row's `type`.

Response shape:

```json
{
  "total": 120,
  "valid": 117,
  "invalid": 3,
  "errors": [{"row": 14, "message": "unknown account \"Kas\""}],
  "preview": [{"row": 2, "date": "2026-10-01", "type": "expense", "amount": 42.5, "description": "Groceries"}]
}
```

Rules:
- Maximum upload 5 MB and 5,000 rows.
- CSV parsed with `encoding/csv` (RFC 4180); BOM stripped; header matched case-insensitively.
- Unknown account/category is an error, never auto-created.
- Import is transactional per request: all valid rows commit, or none do.
- Balance effects go through the existing `CreateTransaction` path so account balances stay consistent.
- `dedupe=true` (optional) skips rows matching an existing transaction on date + amount + description + account.

### Frontend

- Export button on the Transactions page (plain link, browser handles the download).
- Import dialog: file picker → preview table with valid/invalid counts and error rows → "Import N transactions" enabled only when `valid > 0`.
- No drag-and-drop in v1.

## Phase B — AI chat CRUD

### Architecture

```
React /chat  ->  POST /api/chat (JWT)  ->  ChatService
                                            |-- LLM client (OpenAI-compatible)
                                            |-- tool registry -> existing services
                                            |-- pending-action store (delete confirmations)
```

The LLM never sees a database, SQL, or another user's data. It receives tool schemas, emits tool calls, and the Go layer executes them with the authenticated `userID`.

### Tools (v1)

| Tool | Behavior | Confirmation |
|---|---|---|
| `list_accounts` | Return the user's accounts (id, name, balance, currency) | none |
| `list_categories` | Return the user's categories (id, name, type) | none |
| `list_transactions` | Filters: date range, account name, category name, type, limit (max 100) | none |
| `get_dashboard_summary` | Totals for the current month | none |
| `create_transaction` | account + category resolved by NAME, amount, type, date, description | none |
| `create_account` | name, balance, currency | none |
| `create_category` | name, type | none |
| `update_account` | account by name, optional new name/balance | none |
| `delete_transaction` | by id from a preceding `list_transactions` | **required** |
| `delete_account` | by id from a preceding `list_accounts` | **required** |
| `delete_category` | by id from a preceding `list_categories` | **required** |

Design choices that reduce risk:
- The model identifies accounts and categories **by name**, never by raw UUID, so it cannot enumerate IDs.
- Delete tools only accept an ID the model obtained from a preceding list call in the same conversation.
- Each tool caps its result size (list: 100 rows) to bound prompt size and cost.

### Destructive-action flow

1. Model calls `delete_transaction`.
2. Server does **not** delete. It stores a pending action (user, tool, args, expiry 5 min) and returns `{requires_confirmation: true, pending_id, summary: "Delete \"Groceries\" 42.50 on 2026-10-01?"}`.
3. The UI renders a confirm/cancel control.
4. User confirms → `POST /api/chat/actions/{pending_id}/confirm` executes the stored action, re-checking ownership; it is single-use and expires.
5. User cancels → `POST /api/chat/actions/{pending_id}/cancel` discards it.

Pending actions live in memory, are keyed by user, and are never reusable.

### API

- `POST /api/chat` — body `{messages: [{role, content}], ...}`; returns `{reply, tool_calls: [...], pending_action?: {...}}`.
- `POST /api/chat/actions/{id}/confirm`, `/cancel`.
- Non-streaming. Tool loop capped at 5 iterations to prevent runaway calls.
- Rate limit: 20 requests per user per minute (reuse the existing limiter, keyed by user id).

### Prompting and prompt-injection defense

- System prompt states: the assistant only performs finance operations via tools, never invents numbers, and treats user content as data rather than instructions.
- Authorization is enforced in Go on every tool call using the JWT subject. Model output is untrusted input.
- Destructive tools cannot execute without an explicit human confirmation, so injected text alone cannot delete data.
- Tool results are truncated and escaped before being fed back to the model.
- Errors from services are returned to the model as structured failures, not stack traces.

### Frontend

- New `/chat` route in the authenticated group, plus sidebar entry.
- Message list with user/assistant bubbles matching the existing soft-orange design tokens.
- Tool activity rendered as compact chips ("Looked up 12 transactions").
- Confirmation card for pending destructive actions.
- Suggested prompts on an empty state ("How much did I spend on food last month?").
- Errors surface as inline messages; the composer stays enabled.

### Configuration

- `LLM_BASE_URL` (default `https://api.cfrouter.com/v1`), `LLM_MODEL` (default `deepseek-v4.1-flash-1017`), `LLM_API_KEY` (required for chat), `LLM_TIMEOUT_SECONDS` (default 60), `LLM_MAX_TOKENS` (default 1024).
- Documented in `be/.env.example`. The key is read server-side only and never reaches the browser.
- Deployment needs outbound HTTPS egress to `api.cfrouter.com`; no host daemon, no `extra_hosts`, no GPU.
- If `LLM_API_KEY` is unset or the provider is unreachable, chat endpoints return `503` with a clear message; every other feature, including CSV, keeps working.
- Prompt size is bounded: tool results are capped (lists to 100 rows, summaries to a few hundred tokens) and the client message window is trimmed. The provider bills a fixed overhead of roughly 1.5k prompt tokens per call even for trivial input, so the loop is capped and tool results are kept small.

### Provider capability: verified

A live probe against `api.cfrouter.com/v1` with `deepseek-v4.1-flash-1017` confirmed:

- A plain chat completion returns `200`.
- A request with `tools` plus `tool_choice: "auto"` returns `finish_reason: "tool_calls"` and a well-formed `tool_calls` array.

Function calling is therefore supported, and Phase B uses the standard tool-calling loop. No JSON-protocol fallback is required.

## Testing

Phase A:
- Export: header order, date/amount formatting, filter application, empty result, streaming path.
- Import preview: valid rows, unknown account, type mismatch between category and transaction, malformed CSV, oversized file, missing columns, BOM handling, dedupe.
- Import commit: all-or-nothing, balances updated exactly once, per-row error reporting.
- Frontend: preview table rendering, disabled commit when `valid == 0`, error display.

Phase B:
- Tool execution against fake services: correct user scoping, name→id resolution, unknown name errors.
- Tool loop: cap at 5 iterations, model error handling, malformed tool arguments rejected.
- Delete confirmation: pending action created but nothing deleted; confirm executes once; second confirm fails; expiry; another user cannot confirm.
- Prompt-injection regression: user text demanding deletion yields a pending action requiring confirmation, never an immediate delete.
- LLM client: timeout, non-200, malformed JSON.
- Frontend: message send, tool chips, confirmation card, error state.

Security tests:
- After the IDOR fix, deleting another user's account/category/transaction returns not-found and does not mutate data.

## Out of Scope

- Streaming responses (SSE), voice, multi-conversation history persisted in the database.
- Importing accounts/categories.
- Auto-creating missing accounts/categories on import.
- AI access to email, auth, or any data other than the caller's finance records.
- Fine-tuning, embeddings, RAG.

## Rollout

1. IDOR fix (own commit, tests).
2. Phase A CSV (backend then frontend).
3. Phase B chat (backend tools + confirmation, then frontend).
4. Update `AGENTS.md` and `.env.example` for the LLM variables and new routes.

## Implementation status (2026-11-01)

Shipped and verified:

- IDOR fix on account/category/transaction update and delete, with cross-user tests and a live two-user check.
- CSV export and import (preview + commit, atomic batch, 5 MB / 5000 row caps), verified live including rollback on a failed batch.
- Chat backend with tool calling, a 5-iteration cap, per-user rate limiting, and single-use delete confirmations; verified live against `deepseek-v4.1-flash-1017`.
- Chat frontend at `/chat` with tool chips and a confirmation card.

Implemented tools: `list_accounts`, `list_categories`, `list_transactions`, `get_dashboard_summary`, `create_transaction`, `create_account`, `create_category`, `update_account`, `delete_transaction`, `delete_account`, `delete_category`.

All destructive tools share one pending-action store keyed by kind, so confirmation, single use, expiry and per-user isolation behave identically for transactions, accounts and categories. There is no update or delete path that executes straight from model output.
