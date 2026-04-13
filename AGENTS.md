# AGENTS.md

## Project Overview (YOU MUST read first)

Multica is an AI-native task management platform with a Go backend (`server/`), a Next.js App Router frontend (`apps/web/`), Playwright E2E coverage (`e2e/`), and a local daemon/CLI that runs coding agents as workspace-scoped teammates.

## Instruction Hierarchy (IMPORTANT)

- Read this file first, then read the nearest child `AGENTS.md` for the area you are editing.
- The nearest `AGENTS.md` wins for files in its subtree.
- Root `AGENTS.md`: global rules for the whole repository.
- `server/AGENTS.md`: Go backend, CLI, daemon, SQLC, migrations, and runtime execution code.
- `apps/web/AGENTS.md`: Next.js frontend, shared web utilities, and web unit tests.
- `e2e/AGENTS.md`: Playwright specs, fixtures, and helpers.

## Key Commands (YOU MUST use these exact commands)

- Main checkout first setup: `cp .env.example .env` then `make setup-main`
- Worktree first setup: `make worktree-env` then `make setup-worktree`
- Current checkout setup: `make setup`
- Current checkout start: `make start`
- Current checkout stop: `make stop`
- Backend-only dev: `make dev`
- Local daemon: `make daemon`
- Shared PostgreSQL: `make db-up` and `make db-down`
- Backend build: `make build`
- Web build: `pnpm build`
- Web preview after build: `pnpm --filter @multica/web start`
- Web typecheck: `pnpm typecheck`
- Web lint: `pnpm lint`
- Web lint with autofix when needed: `pnpm --filter @multica/web lint -- --fix`
- Web unit tests: `pnpm test`
- All backend tests: `cd server && go test ./...`
- Focused backend test: `cd server && go test ./internal/handler -run TestName`
- Focused web test file: `pnpm --filter @multica/web test -- path/to/file.test.tsx`
- Focused E2E spec: `pnpm exec playwright test e2e/issues.spec.ts`
- Main-checkout full verification: `make check-main`
- Worktree full verification: `make check-worktree`
- Current-checkout full verification: `make check`

## Repository Structure

- `server/`: Go API, CLI, daemon, migrations, SQLC queries, generated DB code, and agent runtime integration.
- `apps/web/`: Next.js routes, feature modules, shared frontend utilities, and Vitest tests.
- `e2e/`: Playwright specs plus API-backed fixtures and helpers.
- `scripts/`: env bootstrapping, PostgreSQL provisioning, and verification scripts.

## Code Style & Working Rules (IMPORTANT - NEVER violate)

- YOU MUST inspect the existing code path before adding abstractions.
- YOU MUST match the current architecture before inventing a new layer.
- YOU MUST keep docs, scripts, and tests aligned when workflow or behavior changes.
- YOU MUST keep comments in code in English only.
- NEVER hand-edit generated code in `server/pkg/db/generated/`.
- NEVER add compatibility shims, dual-write logic, fallback paths, or legacy adapters unless the user explicitly asks for them.
- Avoid broad refactors unless the task requires them.
- Prefer deleting obsolete code over preserving both old and new behavior while the product is still evolving.
- In `apps/web`, preserve TypeScript `strict` mode.
- In `apps/web`, use function components and hooks. Do not add class components.
- All new features MUST include automated tests.
- NEVER introduce new ad-hoc `console.*` logging. Use the existing logger modules instead.
- In `apps/web`, avoid new `any`. If an external boundary forces it, isolate it and leave a short justification nearby.
- For new web files, prefer import order `React/Next -> third-party -> internal`, while still matching the surrounding file when editing existing code.

## Planning Workflow (IMPORTANT)

- For plan-mode work, research, or other multi-step tasks expected to take more than a few tool calls, YOU MUST use the `planning-with-files` skill.
- The required repo-root planning files are `task_plan.md`, `findings.md`, and `progress.md`.
- Before substantive work, YOU MUST read the existing planning files if they already exist. If the task is new, create or replace them for the current task.
- After each phase, major discovery, or verification step, YOU MUST update the planning files.
- This repository is large enough that complex work MUST be tracked end-to-end through those planning files.
- IMPORTANT: simple tasks do NOT need the heavyweight planning workflow. Small `AGENTS.md` edits, `git add` / `git commit`, one-off command runs, and other single-file or low-risk housekeeping changes may proceed without `planning-with-files`.

## Worktree, Env, And Database Rules (IMPORTANT)

- The main checkout uses `.env`.
- Every Git worktree MUST use `.env.worktree`.
- NEVER copy `.env` into a worktree.
- Generic `make` targets prefer `.env` over `.env.worktree`. In a worktree, use the `*-worktree` targets unless you intentionally override `ENV_FILE`.
- All checkouts share the same PostgreSQL container at `localhost:5432`.
- Isolation happens at the database level, not by starting a separate Docker Compose project.
- `make worktree-env` generates a unique `POSTGRES_DB`, `PORT`, and `FRONTEND_PORT` from the worktree path.
- `make worktree-env` refuses to overwrite an existing `.env.worktree`; use `FORCE=1 make worktree-env` only when you intentionally want to regenerate it.
- `make setup`, `make start`, `make dev`, `make test`, `make migrate-up`, `make migrate-down`, and `make check` all ensure the target database exists before continuing.

## Workflow & Verification (Done definition)

- After any web code change, YOU MUST run `pnpm typecheck`, `pnpm lint`, and the narrowest relevant web test command.
- After any backend code change, YOU MUST run the narrowest relevant `go test` command and broaden to `cd server && go test ./...` when the change crosses packages.
- After any E2E change, YOU MUST run the narrowest relevant Playwright spec.
- For cross-surface or user-visible flow changes, finish with `make check` unless the user explicitly wants a narrower proof point.
- IMPORTANT: `make check` runs TypeScript typecheck, Vitest, Go tests, and Playwright. It does NOT run `pnpm lint`.
- Before opening a PR, the relevant checks MUST pass. For web work that means `pnpm lint`, `pnpm test`, and `pnpm typecheck`; for broader workflow changes that also means `make check`.
- Work is not done until the relevant lint, tests, and typecheck pass, and the changed flow has been sanity-checked manually when possible.

## Security (IMPORTANT - NEVER violate)

- NEVER hardcode secrets, API keys, tokens, database credentials, or cookies in code.
- NEVER modify, rotate, or commit production credentials from this repository.
- Environment-dependent values MUST come from `.env`, `.env.worktree`, Docker secrets, or deployment secrets.
- NEVER leak private workspace data, auth tokens, or copied production config into logs, fixtures, tests, or screenshots.
- Do not introduce deprecated or end-of-life libraries when a supported repo-local pattern already exists.

## Commits And PRs

- Keep commits atomic and grouped by logical intent.
- Use Conventional Commits such as `feat(web): ...`, `fix(server): ...`, `refactor(daemon): ...`, `test(e2e): ...`, or `docs: ...`.
- PR titles SHOULD use the format `[scope] short summary`.
- NEVER commit code that still fails the relevant lint, typecheck, or tests.
- PRs SHOULD include a short change summary plus the exact verification commands that were run. Include screenshots for UI changes.

## Gotchas

- `make db-down` stops PostgreSQL but keeps the Docker volume and local databases.
- `docker compose down -v` deletes the shared PostgreSQL volume and wipes the main database plus every worktree database.
- Direct Playwright runs do not start backend or frontend. `make check` is the only built-in flow that can start missing services for E2E.
- The local daemon depends on `claude` / `codex` CLI availability in `PATH`.
- There is no repo-wide lint step inside `make check`. Do not assume broad verification covered lint.
- Keep this root file short. Put subtree-specific detail in the nearest child `AGENTS.md`.
