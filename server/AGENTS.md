# Server AGENTS.md

Applies to `server/`. Read root `AGENTS.md` first, then this file for backend-specific rules.

## Architecture

- `cmd/server/` — HTTP server entry point and worker bootstrap
- `cmd/multica/` — CLI and daemon command surface
- `cmd/migrate/` — schema migration execution
- `internal/handler/` — HTTP boundary (keep thin)
- `internal/service/` — orchestration, lifecycle rules, cross-entity behavior
- `internal/daemon/` — execution environment, repo cache, usage tracking, runtime control
- `pkg/db/queries/` — SQL source of truth
- `pkg/db/generated/` — SQLC output (NEVER hand-edit)
- `pkg/agent/` — provider adapters (Claude, Codex)

## Key Commands (YOU MUST use these)

| Task | Command |
|------|---------|
| Run server | `cd server && go run ./cmd/server` |
| Run CLI | `cd server && go run ./cmd/multica` |
| Migrations up | `cd server && go run ./cmd/migrate up` |
| Migrations down | `cd server && go run ./cmd/migrate down` |
| All backend tests | `cd server && go test ./...` |
| Focused test | `cd server && go test ./internal/handler -run TestName` |
| Regenerate SQLC | `make sqlc` |

## Server Rules (NEVER violate)

- YOU MUST keep handlers thin — put orchestration and lifecycle logic in services
- YOU MUST preserve `workspace_id` scoping and membership checks in handlers, queries, and event flows
- YOU MUST inspect `cmd/server/router.go` when changing routes, auth, or middleware-dependent behavior
- NEVER hand-edit `pkg/db/generated/` — edit SQL in `pkg/db/queries/` or schema in `migrations/`, then run `make sqlc`
- Keep task execution semantics intact: `plan` vs `build`, execution stages, task status transitions, and workdir reuse are coupled behavior
- The daemon prompt is intentionally minimal — detailed runtime instructions belong in `internal/daemon/execenv/`
- If you change runtime instruction generation, YOU MUST update `internal/daemon/execenv/execenv_test.go` in the same change
- Use `server/internal/logger/logger.go` or `log/slog` for logging — NEVER introduce ad-hoc logging styles

## Verification

- Handler, service, daemon, runtime, and SQL-backed changes MUST add or update Go tests
- Query or migration changes MUST keep SQL, generated SQLC output, and the calling Go code in the same change
- Prefer focused package tests first; broaden to `cd server && go test ./...` when the change crosses packages or affects startup, auth, realtime, or task execution
- If you change migrations or DB bootstrapping, run the relevant migrate command against the active env file

## Security

- NEVER leak provider tokens, daemon auth state, or private workspace data into logs or task messages
- Migrations are the schema source of truth — NEVER patch generated structs to compensate for schema drift
- Root worktree and `.env` rules still apply to every backend command

## Gotchas

- `make sqlc` overwrites all files in `pkg/db/generated/` — always edit SQL in `pkg/db/queries/` instead
- Changing a migration file that has already been applied requires a new migration; NEVER edit existing migration files post-apply
- The daemon requires `claude` / `codex` CLI in `PATH` at runtime
- Go test caching can mask failures — use `-count=1` to force re-runs when needed
