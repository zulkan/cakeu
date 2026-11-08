# Chat persistence, 50k token budget, and change-password UI

Date: 2026-11-08
Status: design approved; implementation pending

## Problem

1. AI chat messages live only in `fe/src/stores/chatStore.ts` memory, so a page refresh loses the whole conversation.
2. There is no way to clear a conversation as a unit.
3. LLM usage is unbounded; long conversations keep growing the prompt with no product-level limit.
4. The backend already exposes `POST /api/auth/change-password`, but the SPA has no UI for it.

## Decisions

- Token budget is measured as **conversation size**: the sum of estimated tokens of all persisted user + assistant messages. Not provider usage, not cumulative billing.
- Exactly **one conversation per user** (matches today's single-thread UI), so a single table is enough. No `chat_conversations` table.
- Change-password lives on a **dedicated protected page** at `/change-password`.
- Limit is a service constant of **50,000 tokens**.
- The check happens **before** the LLM call: at or above the limit, the request is rejected; otherwise the turn runs even if it crosses the limit. The next send is blocked until the chat is cleared.
- `POST /api/chat` becomes server-owned: the client sends only the new message, not the whole history.

## Data model (migration `010_chat_history`)

Up:

```sql
CREATE TABLE IF NOT EXISTS chat_messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    seq BIGSERIAL NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    content TEXT NOT NULL,
    token_count INTEGER NOT NULL CHECK (token_count >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_user_seq
    ON chat_messages(user_id, seq);
```

Down:

```sql
DROP INDEX IF EXISTS idx_chat_messages_user_seq;
DROP TABLE IF EXISTS chat_messages;
```

Notes:

- `seq BIGSERIAL` guarantees stable user→assistant ordering. Ordering by `created_at` is unsafe because both rows of a turn are inserted in one transaction and `NOW()` is the transaction timestamp, so they can share a value. `uuid_generate_v4()` is random, so `id` cannot order either.
- Usage is derived with `SUM(token_count)` scoped by `user_id`. No denormalized counter to drift.
- One table because there is exactly one thread per user; a conversation anchor would be unused.
- `ON DELETE CASCADE` from `users` removes a user's history.

## API contracts

All routes stay behind `JWTAuth`.

### `GET /api/chat/history`

```json
{
  "messages": [
    { "id": "uuid", "role": "user", "content": "hi", "created_at": "2026-11-08T10:00:00Z" }
  ],
  "token_usage": { "used": 0, "limit": 50000, "remaining": 50000, "limit_reached": false }
}
```

Empty when the user has no messages.

### `POST /api/chat` (breaking change)

Request:

```json
{ "content": "how much did I spend?", "file_id": "optional", "sheet": "optional" }
```

Response:

```json
{
  "reply": "...",
  "tool_calls": [{ "name": "list_transactions", "summary": "..." }],
  "pending_action": { "id": "...", "summary": "..." },
  "token_usage": { "used": 1234, "limit": 50000, "remaining": 48766, "limit_reached": false }
}
```

The server builds the LLM context from its own stored messages. The client no longer sends a `messages` array.

### `POST /api/chat/actions/{id}/confirm`

Executes the pending action and returns the existing summary plus the current `token_usage`. It does **not** persist a new assistant message: the summary string is identical to the assistant message already stored when the pending action was proposed, so persisting it again would create a duplicate bubble.

### `POST /api/chat/actions/{id}/cancel`

Unchanged: removes the pending action, adds no message.

### `DELETE /api/chat/history`

Deletes the user's messages, drops that user's in-memory pending actions and attached files, and returns `{ "message": "chat cleared" }`.

### Errors

| Condition | Status | Message |
|---|---|---|
| `used >= 50000` before the LLM call | 429 | `chat token limit reached; clear the chat to continue` |
| Empty `content` | 400 | `content is required` |
| Existing chat errors | unchanged | unchanged |

## Token accounting

`estimateChatTokens(text string) int`:

1. Iterate runes.
2. Count ASCII runes and non-ASCII runes separately.
3. Return `ceil(ascii/4) + nonAscii`, minimum 1.

This is an approximation, labeled with `≈` in the UI. It is a product budget, not a billing meter. System prompt, tool schemas, file-attachment context and intermediate tool messages are not persisted and therefore not counted. Document this in the UI as an approximate conversation count.

Lifecycle:

1. Read `used = SUM(token_count)` for the user.
2. If `used >= 50000`: return `ErrChatTokenLimitReached`; nothing is persisted.
3. Build context from stored messages + the new `content` (+ optional file context).
4. Run the existing 5-iteration tool loop.
5. On success, persist the user message and the final assistant text in one transaction with their estimated counts.
6. Return `token_usage` computed from `used + userTokens + assistantTokens`. It may exceed the limit; that is allowed and only blocks the next send.

Concurrency: the cap is best-effort. Two simultaneous sends can both pass the pre-check and overshoot by at most the number of in-flight turns. The existing 20/minute per-user rate limiter is a counter, not a concurrency gate. A per-user mutex is deliberately not added: it would serialize LLM calls (up to the LLM timeout) and would not work across instances. If strictness is ever needed, wrap only the check + insert in `pg_advisory_xact_lock(hashtext($user_id))`, never the LLM call.

Persistence failure: `Handle` returns an error and the turn is not saved (one transaction). `Confirm` never writes, so it has no persistence failure mode.

## Backend implementation

### `be/internal/migrations/010_chat_history.{up,down}.sql`

As above.

### `be/internal/model/chat.go`

- `ChatRequest`: replace `Messages []ChatMessage` with `Content string` (`json:"content"`); keep `FileID`, `Sheet`.
- Add:

```go
type ChatTokenUsage struct {
    Used         int  `json:"used"`
    Limit        int  `json:"limit"`
    Remaining    int  `json:"remaining"`
    LimitReached bool `json:"limit_reached"`
}

type ChatHistoryMessage struct {
    ID        uuid.UUID `json:"id"`
    Role      string    `json:"role"`
    Content   string    `json:"content"`
    CreatedAt time.Time `json:"created_at"`
}

type NewChatMessage struct {
    Role       string
    Content    string
    TokenCount int
}

type ChatHistoryResponse struct {
    Messages   []ChatHistoryMessage `json:"messages"`
    TokenUsage ChatTokenUsage       `json:"token_usage"`
}
```

- Add `TokenUsage *ChatTokenUsage `json:"token_usage,omitempty"`` to `ChatResponse`.
- Remove `ChatMessage` only if nothing else uses it (update tests accordingly).

### `be/internal/service/chat.go`

- Add sentinels: `ErrChatTokenLimitReached`, `ErrChatMessageRequired`.
- Add `const chatTokenLimit = 50_000`.
- Extend `ChatService`:
  - field `history ChatHistoryRepository`
  - constructor parameter (keep the existing argument order, append `history` before `client`)
  - `History(ctx, userID) (*model.ChatHistoryResponse, error)`
  - `Clear(ctx, userID) error`
  - `Handle` rewritten to server-owned history (details below)
  - `Confirm` returns `(*model.ChatResponse, error)` and persists nothing
- New interface (declared in `service`, implemented in `db`):

```go
type ChatHistoryRepository interface {
    ListMessages(ctx context.Context, userID uuid.UUID) ([]model.ChatHistoryMessage, error)
    AppendMessages(ctx context.Context, userID uuid.UUID, messages []model.NewChatMessage) error
    Clear(ctx context.Context, userID uuid.UUID) error
    TokenTotal(ctx context.Context, userID uuid.UUID) (int, error)
}
```

`Handle` steps:

1. `content := strings.TrimSpace(req.Content)`; empty → `ErrChatMessageRequired`.
2. `used := history.TokenTotal`; error → wrapped error.
3. `used >= chatTokenLimit` → `ErrChatTokenLimitReached`.
4. `stored := history.ListMessages`; prepend system prompt; append optional file context; append stored messages; append the new user message.
5. Run the existing loop, but replace each early success `return response, nil` with a shared `finishTurn(ctx, userID, content, used, response)` helper.
6. `finishTurn` persists `{user content, assistant reply}` with estimates, then sets `response.TokenUsage`.

`Confirm`:

1. `takePending`, execute the action, return the same summary as today.
2. Read the current `TokenTotal` and set `response.TokenUsage` (no token change).
3. Do not append a message.

`Clear` also removes `s.pending` entries and `s.files` entries whose `userID` matches.

### `be/internal/service/chat_history.go` (new)

`estimateChatTokens` plus `chatTokenUsage(used int) model.ChatTokenUsage` (`remaining` clamped at 0, `LimitReached = used >= chatTokenLimit`).

### `be/internal/db/chat.go` (new)

`ChatHistoryRepo{ pool *pgxpool.Pool }` with `var _ service.ChatHistoryRepository = (*ChatHistoryRepo)(nil)`.

- `ListMessages`: `SELECT id, role, content, created_at FROM chat_messages WHERE user_id = $1 ORDER BY seq`.
- `TokenTotal`: `SELECT COALESCE(SUM(token_count), 0) FROM chat_messages WHERE user_id = $1`.
- `AppendMessages`: one transaction; insert each message with `user_id`.
- `Clear`: `DELETE FROM chat_messages WHERE user_id = $1`.

### `be/internal/http/chat.go`

- Extend the `ChatService` interface with `History` and `Clear`; update `Confirm`'s return type.
- Add `handleChatHistory` and `handleChatClear`.
- `respondChatError`: add the 429 and 400 cases (`ErrChatTokenLimitReached`, `ErrChatMessageRequired`).
- `handleChatConfirm` returns the service's `ChatResponse` directly.

### `be/internal/http/server.go`

```go
apiMux.HandleFunc("GET /api/chat/history", handleChatHistory(chat))
apiMux.HandleFunc("DELETE /api/chat/history", handleChatClear(chat))
```

Change-password hardening: create `changePasswordLimiter := newRateLimiter(5, 15*time.Minute)` and pass it to `handleChangePassword(auth, limiter)`. Inside the handler, after resolving `userID`:

```go
if limiter != nil && !limiter.allow("change-password|"+userID.String()) {
    respondError(w, http.StatusTooManyRequests, "too many requests")
    return
}
```

The backend endpoint otherwise already exists (`server.go` route + `handleChangePassword`, `service.AuthService.ChangePassword`, 8–72 byte policy); no other backend auth changes.

### `be/cmd/server/main.go`

```go
chatHistoryRepo := db.NewChatHistoryRepo(dbClient.Pool())
chatService := service.NewChatService(..., chatHistoryRepo, llmClient, logger)
```

## Frontend implementation

### `fe/src/types/chat.ts`

Add `ChatTokenUsage`, `ChatHistoryMessage`, `ChatHistoryResponse`; add `token_usage?: ChatTokenUsage` to `ChatResponse`.

### `fe/src/lib/chatApi.ts`

- Add `ChatApiError extends Error { status: number }`; `readError` throws it.
- Generic `request<T>(method, path, body?)` using `CHAT_TIMEOUT_MS`.
- `send(content, fileId?, sheet?)` → `POST /chat`.
- `history()` → `GET /chat/history`.
- `clear()` → `DELETE /chat/history`.

### `fe/src/stores/chatStore.ts`

- State: `tokenUsage: ChatTokenUsage | null`, `isLoadingHistory`, `isClearing`.
- `loadHistory()`: fetch, map to `ChatEntry` (`toolCalls: []`), set entries + usage.
- `sendMessage`: stop calling `toHistory`; send `content`; update `tokenUsage` from every response. On `ChatApiError` 429, set `tokenUsage.limit_reached = true`.
- `confirmAction`: update `tokenUsage` from the response.
- `clear`: async; call `chatApi.clear()`, reset entries/usage/attachment; expose `isClearing`.
- Delete `toHistory`/`sendHistory`/`ChatMessage` import if unused.

### `fe/src/pages/Chat.tsx`

- `useEffect` → `loadHistory()` on mount.
- Toolbar above the message area: token pill `≈ used / limit` plus a progress bar; "Clear chat" ghost button with `Trash2` behind `ConfirmDialog`.
- `limitReached` banner: "Token limit reached. Clear the chat to continue."
- Disable textarea, attachment and send when `isLoading || isUploading || isLoadingHistory || limitReached`.
- Update the `ConfirmDialog` copy: it now permanently deletes the conversation from the server, not "from this browser".
- Keep the existing empty state, markdown, pending-action and attachment flows.

### Change password

`fe/src/stores/authStore.ts`:

```ts
changePassword: (currentPassword: string, newPassword: string) => Promise<string>;
```

Calls `POST /api/auth/change-password` with `Authorization: Bearer <token>` and body `{ current_password, new_password }`; throws the server message on failure; returns the success message.

**Do not route this through `fe/src/lib/api.ts` `request()`**: that helper calls `handleUnauthorized()` on every 401, and the backend returns 401 for a wrong current password, which would log the user out. Use a dedicated `fetch` inside the store.

`fe/src/pages/ChangePassword.tsx` (new):

- `PageHeader` + `Card`, `react-hook-form` + `zod`.
- Fields: current password, new password, confirm new password; show/hide toggles.
- Validation: required; new ≥ 8 characters; new ≤ 72 **bytes** (`new TextEncoder().encode(value).length`); confirmation matches.
- Inline `role="alert"` error; success toast via `useToastStore.getState().add(..., 'success')`; form reset.
- "Back to dashboard" secondary link.

`fe/src/App.tsx`: add `<Route path="/change-password" element={<ChangePassword />} />` inside the protected `AppShell` group.

`fe/src/components/layout/AppShell.tsx`: `UserCard` takes `onNavigate?: () => void` and renders a "Change password" `NavLink` (icon `KeyRound`) above Logout; the mobile drawer passes `() => setDrawerOpen(false)`.

The session stays active after the change. JWTs are stateless and not revoked server-side, so client-side logout would be cosmetic; documented as a limitation.

## Edge cases

- **Refresh mid-turn**: only completed turns are persisted; an in-flight turn is lost.
- **Pending action after refresh**: the assistant's question text is restored, but the pending action keeps its 5-minute single-use TTL and is not resurrected, so the prompt has no buttons. Accepted limitation; the user repeats the request. The same applies after a server restart.
- **Clear during a pending action**: pending actions and attached files for that user are dropped; a late confirm returns 404 as today.
- **Clear racing an in-flight send**: the send can insert after the delete in a cross-tab scenario. The UI disables Clear while sending; accepted low-severity edge.
- **Overshoot**: allowed once; `limit_reached` becomes true in the same response, so the UI locks immediately.
- **LLM error**: nothing is persisted and the token total is unchanged (the client keeps the optimistic user bubble until refresh).
- **Confirm**: never writes, so no duplicate bubbles and no unbudgeted tokens.

## Testing

Backend:

- `estimateChatTokens` table test (ASCII, CJK, empty).
- Service: blocked at exactly 50k; 49,999 allowed and may overshoot; history returns stored messages + usage; clear wipes messages and in-memory pending/files; confirm returns usage and persists nothing; empty content → sentinel.
- Handler: new request shape; `GET /api/chat/history`; `DELETE /api/chat/history`; 429 mapping.
- Update existing chat service/handler tests for the new constructor argument and request shape (`be/internal/service/chat_test.go` constructor and ~18 `ChatRequest` sites; `be/internal/http/chat_test.go` fake service).
- Add a service test for the new change-password rate limiter if the handler seam allows it.
- `gofmt`, `go vet ./...`, `go test ./...`.

Known gap: `be` has no Postgres-backed tests, so the new `ChatHistoryRepo` SQL (SUM, insert, `seq` ordering) is not covered by `go test ./...`. Add an integration test gated on `DATABASE_URL` only if a test database is available; otherwise record the gap.

Frontend:

- `Chat` tests: history is fetched on mount; clear calls the API and resets the view; limit reached disables input and shows the banner; send uses `content` not `messages`.
- `ChangePassword` page tests: validation, mismatch, byte limit, success toast, server error.
- `authStore` test: `changePassword` sends the Bearer token and returns the message; failure throws.
- `npm run lint`, `npm run build`, `npx vitest run`.

## Out of scope

- Multiple conversations / rename / delete individual threads.
- Server-side revocation of JWTs after a password change.
- Persisting tool-call badges, pending actions, or uploaded chat files across refreshes.
- Configurable token limit via environment variable.
- Pagination of history (the 50k cap bounds the payload).
