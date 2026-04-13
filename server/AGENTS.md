# Server Instructions

These instructions apply to `server/`. Follow root `AGENTS.md` first, then this file for backend-specific rules.

## Architecture

- `cmd/server/` is the HTTP server entry point and worker bootstrap.
- `cmd/multica/` is the CLI and daemon command surface.
- `cmd/migrate/` owns schema migration execution.
- `internal/handler/` is the HTTP boundary. Keep it thin.
- `internal/service/` owns orchestration, lifecycle rules, and cross-entity behavior.
- `internal/daemon/` owns execution environment, repo cache, usage tracking, and runtime control.
- `pkg/db/queries/` is the SQL source of truth.
- `pkg/db/generated/` is SQLC output.
- `pkg/agent/` contains provider adapters such as Claude and Codex.

## Key Commands (YOU MUST use these exact commands)

- Run server: `cd server && go run ./cmd/server`
- Run CLI: `cd server && go run ./cmd/multica`
- Run migrations up: `cd server && go run ./cmd/migrate up`
- Run migrations down: `cd server && go run ./cmd/migrate down`
- Run all backend tests: `cd server && go test ./...`
- Run a focused backend test: `cd server && go test ./internal/handler -run TestName`
- Regenerate SQLC: `make sqlc`

## Server Rules (IMPORTANT - NEVER violate)

- YOU MUST keep handlers thin. Put orchestration and lifecycle logic in services.
- YOU MUST preserve `workspace_id` scoping and membership checks in handlers, queries, and event flows.
- YOU MUST inspect `cmd/server/router.go` when changing routes, auth, or middleware-dependent behavior.
- NEVER hand-edit `pkg/db/generated/`; edit SQL in `pkg/db/queries/` or schema in `migrations/`, then run `make sqlc`.
- Keep task execution semantics intact: `plan` vs `build`, execution stages, task status transitions, and workdir reuse are coupled behavior.
- The daemon prompt is intentionally minimal. Detailed runtime instructions belong in `internal/daemon/execenv/`.
- If you change runtime instruction generation, update `internal/daemon/execenv/execenv_test.go` in the same change.
- Use `server/internal/logger/logger.go` or `log/slog` for logging. Do not introduce ad-hoc logging styles.

## Verification

- Handler, service, daemon, runtime, and SQL-backed changes MUST add or update Go tests.
- Query or migration changes MUST keep SQL, generated SQLC output, and the calling Go code in the same change.
- Prefer focused package tests first. Finish with `cd server && go test ./...` when the change crosses packages or affects startup, auth, realtime, or task execution.
- If you change migrations or DB bootstrapping, run the relevant migrate command against the active env file.

## Security And Gotchas

- Migrations are the schema source of truth. Do not patch generated structs to compensate for schema drift.
- Do not leak provider tokens, daemon auth state, or private workspace data into logs or task messages.
- Root worktree and `.env` rules still apply to every backend command.
