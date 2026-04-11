# Server Instructions

These instructions apply to `server/`.

## Architecture

- Entry points live in `cmd/`.
- HTTP behavior lives in `internal/handler/`.
- Workflow orchestration belongs in `internal/service/`.
- Daemon and execution-environment code lives in `internal/daemon/`.
- SQL source of truth is `pkg/db/queries/`; generated output goes to `pkg/db/generated/`.
- Agent adapters live in `pkg/agent/`.

## Backend Rules

- Keep handlers thin. Put orchestration, lifecycle logic, and cross-entity behavior in services.
- Preserve workspace scoping. Queries and handlers must continue to respect `workspace_id` and membership checks.
- When editing routes or auth flows, verify the middleware chain in `cmd/server/router.go`.
- Do not hand-edit `pkg/db/generated/`. Change SQL in `pkg/db/queries/` and regenerate with `make sqlc`.
- Keep task execution semantics intact: `plan` and `build` modes, execution stages, task status transitions, and workdir reuse are coupled.
- The daemon prompt is intentionally minimal. Detailed runtime instructions belong in `internal/daemon/execenv/`.
- If you change runtime instruction generation, update `internal/daemon/execenv/execenv_test.go` in the same change.

## Migrations And Queries

- Put schema changes in `migrations/`.
- Keep SQLC query changes and the Go code that depends on them in the same change.
- Prefer explicit, domain-oriented query names and parameter structs.

## Commands

```bash
cd server && go run ./cmd/server
cd server && go run ./cmd/migrate up
cd server && go test ./...
cd server && go test ./internal/handler -run TestName
make sqlc
make test
```

## Testing

- Add or update Go tests when changing handlers, services, daemon behavior, SQL-backed flows, or agent execution behavior.
- Prefer focused package tests over broad test runs unless the user asks for full verification.
- For daemon and execenv changes, assert behavior and stable invariants instead of brittle full-document snapshots.
