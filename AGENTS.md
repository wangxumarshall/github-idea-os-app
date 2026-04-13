# AGENTS.md

## Project Overview (YOU MUST read first)

Multica is an AI-native task management platform that turns coding agents (Claude Code / Codex) into real teammates. Core capability: assign a GitHub Issue to an agent, and the agent auto-claims it, writes code, reports blockers, updates status, and submits a PR.

- **Frontend**: Next.js 16 App Router (`apps/web/`)
- **Backend**: Go — Chi router + sqlc + gorilla/websocket + PostgreSQL 17 + pgvector (`server/`)
- **Agent Runtime**: Local daemon that detects `claude` / `codex` CLI and executes tasks
- **E2E**: Playwright (`e2e/`)
- **Architecture principle**: Preserve existing architecture; prefer deleting obsolete paths over dual-write

## Instruction Hierarchy (IMPORTANT — nearest-wins)

Codex reads the **nearest** AGENTS.md first. Sub-directory files **override** root rules.

- **Root** (this file): global rules
- `server/AGENTS.md`: Go backend, daemon, sqlc, migrations
- `apps/web/AGENTS.md`: Next.js frontend
- `e2e/AGENTS.md`: Playwright tests

YOU MUST read this file first, then read the nearest child AGENTS.md for the area you are editing.

## Key Commands (YOU MUST use these exact commands)

| Scenario | Command |
|----------|---------|
| Main-checkout first setup | `test -f .env \|\| cp .env.example .env` then `make setup-main` |
| Main-checkout start | `make start-main` |
| Main-checkout stop | `make stop-main` |
| Worktree first setup | `make worktree-env` then `make setup-worktree` |
| Worktree start | `make start-worktree` |
| Worktree stop | `make stop-worktree` |
| Backend-only dev | `make dev` |
| Local daemon | `make daemon` |
| Shared PostgreSQL up | `make db-up` |
| Shared PostgreSQL down | `make db-down` |
| Backend build | `make build` |
| Web build | `pnpm build` |
| Web preview | `pnpm --filter @multica/web start` |
| Web typecheck | `pnpm typecheck` |
| Web lint | `pnpm lint` |
| Web lint autofix | `pnpm --filter @multica/web lint -- --fix` |
| Web unit tests | `pnpm test` |
| All backend tests | `cd server && go test ./...` |
| Focused backend test | `cd server && go test ./internal/handler -run TestName` |
| Focused web test | `pnpm --filter @multica/web test -- path/to/file.test.tsx` |
| Focused E2E spec | `pnpm exec playwright test e2e/issues.spec.ts` |
| Main-checkout full verification | `make check-main` |
| Worktree full verification | `make check-worktree` |

## Repository Structure

- `server/`: Go API, CLI, daemon, migrations, SQLC queries, generated DB code, agent runtime
- `apps/web/`: Next.js routes, feature modules, shared frontend utilities, Vitest tests
- `e2e/`: Playwright specs plus API-backed fixtures and helpers
- `scripts/`: env bootstrapping, PostgreSQL provisioning, verification scripts

## Agent Behavioral Guidelines (NEVER violate)

### 1. Think Before Coding

YOU MUST NOT assume. YOU MUST NOT hide confusion. Surface tradeoffs.

- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them — NEVER pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what is confusing. Ask.

### 2. Simplicity First

Minimum code that solves the problem. Nothing speculative.

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

### 3. Surgical Changes

Touch only what you MUST. Clean up only your own mess.

- NEVER "improve" adjacent code, comments, or formatting.
- NEVER refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it — NEVER delete it unless asked.
- Remove imports/variables/functions that YOUR changes made unused.
- NEVER remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

### 4. Goal-Driven Execution

Define success criteria. Loop until verified.

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

## Code Style & Working Rules (NEVER violate)

- YOU MUST inspect the existing code path before adding abstractions
- YOU MUST match the current architecture before inventing a new layer
- YOU MUST keep docs, scripts, and tests aligned when workflow or behavior changes
- YOU MUST keep code comments in English only
- NEVER hand-edit generated code in `server/pkg/db/generated/`
- NEVER add compatibility shims, dual-write logic, fallback paths, or legacy adapters unless explicitly asked
- NEVER do broad refactors unless the task requires them
- Prefer deleting obsolete code over preserving both old and new behavior (product not yet live)
- All new features MUST include automated tests
- NEVER introduce new ad-hoc `console.*` logging — use existing logger modules
- YOU MUST output all responses in Chinese (中文) — all final answers, explanations, and summaries must be in Chinese regardless of the input language

## Worktree, Env & Database Rules (IMPORTANT)

- Main checkout uses `.env`; every worktree MUST use `.env.worktree`
- NEVER copy `.env` into a worktree — it causes database conflicts
- Generic `make` targets prefer `.env` over `.env.worktree`; in a worktree use `*-worktree` targets
- All checkouts share the same PostgreSQL container at `localhost:5432`
- Isolation happens at the **database level**, not by starting a separate Docker Compose project
- `make worktree-env` generates a unique `POSTGRES_DB`, `PORT`, and `FRONTEND_PORT` from the worktree path
- `make worktree-env` refuses to overwrite an existing `.env.worktree`; use `FORCE=1 make worktree-env` only when intentionally regenerating
- `make setup`, `make start`, `make dev`, `make test`, `make migrate-up`, `make migrate-down`, and `make check` all ensure the target database exists before continuing

## Workflow & Verification (Done definition)

- After any web code change, YOU MUST run `pnpm typecheck` + `pnpm lint` + narrowest relevant web test
- After any backend code change, YOU MUST run the narrowest relevant `go test`; broaden to `cd server && go test ./...` when the change crosses packages
- After any E2E change, YOU MUST run the narrowest relevant Playwright spec
- For cross-surface or user-visible flow changes, finish with `make check-main` (main checkout) or `make check-worktree` (worktree)
- IMPORTANT: `make check` / `make check-main` / `make check-worktree` run TypeScript typecheck, Vitest, Go tests, and Playwright — they do NOT run `pnpm lint`
- Before opening a PR: `pnpm lint` + `pnpm test` + `pnpm typecheck` MUST pass; for broader changes also `make check-main` or `make check-worktree`
- Work is not done until lint + tests + typecheck pass and the changed flow is sanity-checked manually when possible

## Planning Workflow (IMPORTANT)

- For multi-step tasks expected to take more than a few tool calls, YOU MUST use the `planning-with-files` skill
- Required repo-root planning files: `task_plan.md`, `findings.md`, `progress.md`
- Before substantive work, YOU MUST read existing planning files if they exist; for new tasks, create or replace them
- After each phase or verification step, YOU MUST update the planning files
- IMPORTANT: simple tasks (single-file edits, git operations, one-off commands) do NOT need the planning workflow

## Security (IMPORTANT — NEVER violate)

- NEVER hardcode secrets, API keys, tokens, database credentials, or cookies in code
- NEVER modify, rotate, or commit production credentials from this repository
- Environment-dependent values MUST come from `.env`, `.env.worktree`, Docker secrets, or deployment secrets
- NEVER leak private workspace data, auth tokens, or copied production config into logs, fixtures, tests, or screenshots
- Do not introduce deprecated or end-of-life libraries when a supported repo-local pattern already exists

## Commits & PRs

- Keep commits atomic, grouped by logical intent
- Use Conventional Commits: `feat(web): ...`, `fix(server): ...`, `refactor(daemon): ...`, `test(e2e): ...`, `docs: ...`
- PR titles SHOULD use `[scope] short summary`
- NEVER commit code that still fails lint, typecheck, or tests
- PRs SHOULD include a short change summary plus the exact verification commands run; include screenshots for UI changes

## Gotchas

- `make db-down` stops PostgreSQL but keeps the Docker volume and local databases
- `docker compose down -v` deletes the shared PostgreSQL volume and wipes the main database plus every worktree database
- Direct Playwright runs do NOT start backend or frontend — use `make check*` targets for built-in flows
- The local daemon depends on `claude` / `codex` CLI availability in `PATH`
- There is NO repo-wide lint step inside `make check` — do not assume broad verification covered lint
- Keep this root file short; put subtree-specific detail in the nearest child `AGENTS.md`
