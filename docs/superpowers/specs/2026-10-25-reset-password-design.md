# Reset Password via Email — Design

Date: 2026-10-25
Status: Approved (decisions recorded below; user may amend)

## Problem

The backend exposes `/forgot-password` and `/reset-password`, but the flow cannot work:

1. `AuthService.ResetPassword` hashes the new password and discards it (`_ = hash`), marking only the token used. The old password keeps working.
2. `UserRepository` has no method to persist a password, so there is nothing to call.
3. The email contains a bare hex token, not a clickable link. `FRONTEND_URL` is configured but unused.
4. SMTP failures are swallowed by `fmt.Printf`, so a broken mailer is indistinguishable from success.
5. The frontend has no `/forgot-password` or `/reset-password` page and no link from Login.
6. Root-path auth endpoints collide with SPA routes in dev: Vite proxies `/login` and `/register` to Go, so `GET :5173/login` returns `405 {"error":"method not allowed"}` instead of the login page. A reset link is always a direct navigation, so a root `/reset-password` endpoint would break the feature it serves.

## Decisions

| Decision | Choice | Why |
|---|---|---|
| API paths | Move all auth endpoints to `/api/auth/*` | Removes the SPA/proxy collision; fixes direct navigation and reload for `/login` and `/register` as a side effect. |
| Email delivery | Real SMTP when configured; otherwise log the reset link at WARN | Works today with placeholder credentials and becomes real mail with no code change. |
| Link shape | `${FRONTEND_URL}/reset-password?token=<hex>` | Standard, bookmarkable, survives refresh. |
| Token lifetime | 1 hour, single-use | Current behavior; sufficient for a mail round trip. |
| Password policy | Minimum 8 characters on reset | Low friction, blocks trivial passwords. |
| Abuse control | In-memory rate limit, 5 requests per email+IP per 15 minutes | No Redis, no new dependency. |

## API Contract

All auth endpoints are public (no JWT). Existing protected routes are unchanged.

| Method | Path | Body | Success | Errors |
|---|---|---|---|---|
| POST | `/api/auth/register` | `{email, password}` | `201` User | `400`, `409` |
| POST | `/api/auth/login` | `{email, password}` | `200 {token, user}` | `400`, `401` |
| POST | `/api/auth/forgot-password` | `{email}` | `200 {message}` | `400`, `429` |
| POST | `/api/auth/reset-password` | `{token, password}` | `200 {message}` | `400` |

Removed: `/register`, `/login`, `/forgot-password`, `/reset-password` at the root.

Response bodies:

- forgot success: `{"message":"if the email exists, a reset link will be sent"}` — identical for unknown emails (no enumeration).
- reset success: `{"message":"password reset successful"}`
- invalid/used/expired token: `400 {"error":"invalid or expired token"}`
- password under 8 chars: `400 {"error":"password must be at least 8 characters"}`
- rate limited: `429 {"error":"too many requests"}`

Routing note: Go 1.22 `ServeMux` picks the most specific pattern, so exact `/api/auth/reset-password` registered on the root mux takes precedence over the `/api/` prefix wrapped in `JWTAuth`. Public auth handlers therefore bypass JWT without extra branching.

## Backend

**`internal/service/auth.go`**

- `UserRepository` gains `UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error`.
- `TokenRepository` gains `InvalidateUserTokens(ctx context.Context, userID uuid.UUID) error`.
- `RequestPasswordReset`: validate email is non-empty at the handler; call `InvalidateUserTokens` for the user before inserting a new token so only the newest link is live; keep returning `nil` for unknown emails.
- `ResetPassword`, in order:
  1. `GetTokenByToken` → `ErrInvalidToken` if absent.
  2. Reject if `UsedAt != nil` or `time.Now().After(ExpiresAt)`.
  3. Reject if `len(password) < 8` → new sentinel `ErrPasswordTooShort`.
  4. `bcrypt.GenerateFromPassword`.
  5. `userRepo.UpdatePassword(token.UserID, hash)` — the missing step.
  6. `InvalidateUserTokens(user.TokenUserID)` so no sibling token survives.
  7. Return nil.

**`internal/db/repository.go`**

- `UpdatePassword`: `UPDATE users SET password_hash = $2, updated_at = NOW() WHERE id = $1`; return `ErrUserNotFound` when `CommandTag.RowsAffected() == 0`.
- `InvalidateUserTokens`: `UPDATE password_reset_tokens SET used_at = NOW() WHERE user_id = $1 AND used_at IS NULL`.
- Keep the `var _ service.UserRepository = (*UserRepo)(nil)` compile-time assertions passing.

**`internal/http/server.go`**

- Register the four handlers on `mux` at `/api/auth/...`; delete the root registrations.
- Map `ErrPasswordTooShort` to `400`.
- Wrap `handleForgotPassword` in the rate limiter.
- Keep Swagger annotations, updating `@Router` paths.

**`internal/http/ratelimit.go` (new)**

- `newRateLimiter(limit int, window time.Duration) *rateLimiter` with a mutex-guarded `map[string]*bucket`.
- Key: normalized email + client IP (`RemoteAddr`).
- Prune expired buckets on access; return `false` when over limit.

**`internal/service/email.go`**

- `EmailConfig` gains `FrontendURL string`.
- `NewEmailService(config EmailConfig, logger *slog.Logger)`; nil logger falls back to `slog.Default()`.
- `SendPasswordResetEmail(ctx, to, resetToken)` builds `link := strings.TrimRight(FrontendURL,"/") + "/reset-password?token=" + resetToken` and sends subject "Reset your Catatku password" with a plain-text body and a minimal HTML body containing the link.
- **Dev fallback:** if `Username`/`Password` is empty or begins with `your-`, log `slog.LevelWarn` "password reset link (SMTP not configured)" with `to` and `link`, then return nil.
- TLS by port: `465` → implicit TLS (`SendWithTLS`); anything else → `Send` with `smtp.PlainAuth` so STARTTLS on 587 works.

**`cmd/server/main.go`**

- Pass `cfg.FrontendURL` into `EmailConfig` and the `*slog.Logger` into `NewEmailService`.

## Frontend

**`vite.config.ts`** — proxy only `/api`; delete the `/login` and `/register` entries so page routes reach the SPA.

**`src/stores/authStore.ts`**

- `login`/`register` move to `/api/auth/login` and `/api/auth/register`.
- Add `forgotPassword(email: string): Promise<string>` → `POST /api/auth/forgot-password`, returns the server message, throws `Error` with the server message on failure.
- Add `resetPassword(token: string, password: string): Promise<string>` → `POST /api/auth/reset-password`.

**`src/pages/ForgotPassword.tsx` (new)** — email form with zod validation; on success show a neutral panel ("If that email exists, a reset link is on its way") and a link back to Login. Uses `AuthLayout`.

**`src/pages/ResetPassword.tsx` (new)** — reads `token` from `useSearchParams`; new password + confirm (min 8, must match); submits, then navigates to `/login` with a success flag. Missing token or `400` shows an error panel with "Request a new link" → `/forgot-password`.

**`src/pages/Login.tsx`** — add a "Forgot password?" link next to the password field.

**`src/App.tsx`** — add public routes `/forgot-password` and `/reset-password` alongside `/login` and `/register`.

## Security

- Reset link never appears in an HTTP response; only in server logs, and only under the dev fallback.
- Forgot-password is uniform for known and unknown emails.
- Tokens are 32 random bytes, hex-encoded, single-use, 1 hour, and invalidated on reissue and on successful reset.
- No password or token is logged.
- Rate limiting is in-memory and per-process; documented as unsuitable for multi-instance deployment.

## Testing

Backend:

- Service unit tests with fakes: password hash persisted and verifiable (new password matches, old does not); expired token rejected; used token rejected; password under 8 rejected; prior tokens invalidated on issue and on reset; unknown email returns nil.
- Handler tests: forgot always 200 including unknown email; reset 400 on bad token and short password; rate limiter returns 429 after 5 requests; auth routes reachable without a JWT while `/api/accounts` still returns 401.
- Email test: link format uses `FrontendURL` and the token; placeholder SMTP produces no network call.

Frontend:

- `ForgotPassword` page: validation, success panel, server error.
- `ResetPassword` page: missing token error, short password validation, mismatch validation, success redirect, invalid-token error.
- `authStore`: paths are `/api/auth/*`; new actions throw on failure.
- Existing Login/Register tests updated for the new endpoint paths.

## Out of Scope

- Email verification, 2FA, refresh tokens, session revocation.
- Changing the register password policy.
- Distributed rate limiting.
- Real SMTP credentials (the user supplies them later; the dev fallback covers today).

## Verification Plan

1. `cd be && go build ./... && go vet ./... && go test ./... && gofmt -l .`
2. `cd fe && npm run build && npm run lint && npm test -- --run`
3. Live: start Postgres-backed server, register a user, request a reset, take the link from `be/logs/app.log`, complete the reset through the API, confirm the old password fails and the new one logs in.
