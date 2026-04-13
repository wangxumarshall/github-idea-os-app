# Findings: AGENTS.md Hierarchy Rewrite

## Official Guidance

- Official AGENTS guidance emphasizes nearest-file precedence, colocated instructions, short high-signal documents, direct wording, and exact commands.
- Verification, security, and gotchas are high-value sections and should be explicit.
- The rewrite should only document commands and workflows that actually exist in this repository.

## Repository Facts To Preserve

- The repo uses a layered instruction hierarchy: root `AGENTS.md`, plus `server/AGENTS.md`, `apps/web/AGENTS.md`, and `e2e/AGENTS.md`.
- `CONTRIBUTING.md` is the authoritative source for `.env`, `.env.worktree`, worktree isolation, and shared-PostgreSQL behavior.
- Generic `make` targets prefer `.env` over `.env.worktree`, so copying `.env` into a worktree is unsafe.
- `scripts/init-worktree-env.sh` generates unique `POSTGRES_DB`, `PORT`, and `FRONTEND_PORT` values from the worktree path and refuses to overwrite `.env.worktree` unless `FORCE=1`.
- `scripts/ensure-postgres.sh` auto-starts the shared PostgreSQL container and auto-creates the target database.
- `scripts/check.sh` runs TypeScript typecheck, Vitest, Go tests, and Playwright. It does not run `pnpm lint`.

## Frontend Facts

- `apps/web/tsconfig.json` is `strict: true`.
- The web app uses Next.js App Router, feature modules under `features/`, shared utilities under `shared/`, and Vitest with Testing Library.
- A shared frontend logger exists at `apps/web/shared/logger.ts`.
- The codebase still contains some legacy `console.*` usage, so the new instructions should forbid new ad-hoc `console.*` usage without pretending the repo is already fully clean.
- Playwright config is at repo root in `playwright.config.ts`.

## Backend Facts

- Backend responsibilities are split across `cmd/`, `internal/handler/`, `internal/service/`, `internal/daemon/`, `pkg/db/queries/`, and `pkg/db/generated/`.
- SQLC output is generated from `server/pkg/db/queries/` and `server/migrations/` into `server/pkg/db/generated/`.
- A shared backend logger exists at `server/internal/logger/logger.go`, and the backend otherwise uses `log/slog`.
- Runtime instruction generation is coupled to `internal/daemon/execenv/`.

## Risks To Avoid

- Do not mention `docs/code-review.md` or `docs/architecture.md`; those files are not present in this repository.
- Do not claim `make check` runs lint.
- Do not invent unsupported commands such as a repo-wide autofix target unless the command truly exists.
