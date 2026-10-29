# AGENTS.md

Two independent apps in one workspace: `be/` (Go 1.26 HTTP API, module `goskeleton`) and `fe/` (React 19 + Vite 8 + TS 6 SPA, branded "FinBank Portal"). They are **not wired together yet** — the SPA authenticates against mock data.

## Commands

Backend commands must run with cwd `be/`; the repo root has no `go.mod`, so `go build ./...` there fails.

```bash
cd be
go build ./...                              # wiring lives in cmd/server
go test ./...                               # unit tests only; no Postgres needed
go test ./internal/service -run TestLogin   # single test
go vet ./...
go run ./cmd/resetpassword -email user@example.com   # set a password directly; prompts on stdin
```

No Makefile, CI, or Go lint config — `gofmt` is the only formatting rule.

```bash
cd fe
npm run dev                                            # Vite dev server
npm test                                               # vitest watch; use `npx vitest run` for one shot
npx vitest run src/pages/__tests__/Login.test.tsx      # single test file
npm run lint                                           # ESLint 10 flat config, --max-warnings 0
npm run build                                          # tsc -b && vite build
```

## Backend

- `be/cmd/server/main.go` is the only entrypoint: godotenv → config → slog logger → pgx pool → **auto-runs migrations** → services → handler → graceful shutdown. A running Postgres with `uuid-ossp` available is required just to boot.
- Run the binary from `be/`: `.env`, `internal/migrations` (`file://internal/migrations`) and `logs/` are resolved relative to cwd. `be/logs/app.log` is append-only JSON; logs also go to stderr.
- Routes are plain `http.ServeMux` in `be/internal/http/server.go`: public `POST /api/auth/register`, `/api/auth/login`, `/api/auth/forgot-password`, `/api/auth/reset-password`, plus `/health` and `/swagger/`. Everything else under `/api/` (accounts, categories, transactions, dashboard) is wrapped in `JWTAuth`; `CORS(allowedOrigins)` wraps the whole mux. Auth lives under `/api/auth/*` rather than at the root because the Vite dev proxy would otherwise swallow the SPA's `/login` and `/reset-password` page routes on direct navigation.
- `/health` returns 403 for any non-loopback `RemoteAddr` (intentional), so remote/container health checks fail while local ones pass.
- Dependency direction is deliberately inverted: `be/internal/db/repository.go` imports `goskeleton/internal/service` to compile-check the `UserRepository`/`TokenRepository` interfaces and reuse sentinel errors (`ErrEmailExists`, `ErrInvalidToken`, …). Define new interfaces/sentinels in `service`, not `db`, or you create an import cycle.
- Config (`be/internal/config/config.go`): `DATABASE_URL`, or `DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME` (defaults `root/example/goskeleton` at localhost:5432); `PORT` (8080), `JWT_SECRET`, `SMTP_HOST/PORT/USER/PASSWORD/FROM`, `FRONTEND_URL` (defined but unused). `.env` is optional.
- Swagger: annotations are on the handlers, but `be/docs/docs.go` is only a package stub and no generated spec exists — `/swagger/` serves nothing until you run `swag init` from `be/`.
- Handlers depend on small interfaces (`AuthService`, `HealthChecker`) and unit tests inject fakes; keep that seam instead of constructing services inside handlers.

## Frontend

- Auth is real: `src/stores/authStore.ts` (zustand + `persist`, localStorage key `auth-storage`) calls `POST /login` and `POST /register` through `src/lib/api.ts` and persists the JWT as `state.token`; `src/lib/api.ts` re-reads that token and sends `Authorization: Bearer <token>`. `src/data/mockUsers.ts` no longer exists — there is no mock path left.
- Routes live in `src/App.tsx`: `/` redirects to `/login`; public `/login`, `/register`, `/forgot-password`, `/reset-password`; the `components/ProtectedRoute.tsx`-guarded group `/dashboard`, `/transactions`, `/accounts`, `/categories`; `/home` redirects to `/dashboard`. Reset links point at `/reset-password?token=…`, so that page must stay reachable even while a session is persisted. Add pages there rather than introducing another router.
- Tailwind 4 is CSS-first: `src/styles.css` is `@import "tailwindcss"` plus an `@theme` block holding the design tokens (`brand-*` soft orange, `warm-*`, `income`/`expense`, `surface*`). There is no `tailwind.config.js`; don't create one — add tokens to `@theme`.
- Unit tests: vitest `projects` config in `vite.config.ts` (jsdom, `src/test/setup.ts`), include `src/**/*.test.{ts,tsx}`, stories excluded. `*.stories.tsx` exist but `.storybook/` was never created, so `npm run storybook` is not usable.
- `npm run build` type-checks with `tsc -b` project references; `noUnusedLocals`/`noUnusedParameters` are errors.
- `fe/README.md` is the stock Vite template (talks about Oxlint); the real linter is the ESLint 10 flat config in `eslint.config.js`.
- `fe/node_modules/.bin` holds proper symlinks. They had been flattened into file copies, so each bin's relative imports resolved against `node_modules/.bin/` and `npm run dev` died with `ERR_MODULE_NOT_FOUND: .../node_modules/dist/node/cli.js`; `vitest`, `tsc`, and `eslint` failed the same way. All npm scripts work now (`npm run dev`, `npm run build`, `npm run lint`, `npm test -- --run`). If they regress to copies, run `npm install` in `fe/` or re-link every `.bin` entry to its package's `bin` target.

## Repo state

- No commits yet and no `.gitignore`: `be/logs/`, `fe/node_modules/`, `fe/dist/`, and `.omo/` are on disk and untracked. Never `git add -A` / `git add .`; stage explicit paths.

## Known gaps (don't assume these work)

- `isDuplicateEmail` returns true for any error, so any DB failure during registration surfaces as `409 email already registered`.
- Password-reset email delivery needs a Sumopod-verified sender domain. `SMTP_FROM=admin@zulkan.my.id` and all three DNS records are published (`v=DKIM1` at `trx_ke._domainkey`, root SPF including `spf.kirim.email`, and `sumo-verification=…`), yet Sumopod still answers `450 Relay failed: Custom sender domain ownership not verified` until the domain is marked Verified in Email SMTP → Custom Domain (click "Verify DNS settings"). Until then no reset email is delivered; blank the SMTP user/password to make the backend log the reset link at WARN instead — pull it from `be/logs/app.log` to test locally.
- `ForgotPassword` rate limiting is in-memory and per-process, so it does not hold across multiple instances.
