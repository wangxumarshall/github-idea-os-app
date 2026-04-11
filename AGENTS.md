# Repository Instructions

This repository uses layered `AGENTS.md` files. Start with this file, then follow the nearest child `AGENTS.md` for the area you are editing.

## Project Snapshot

Multica is an AI-native task management platform where coding agents act as teammates.

- Backend: Go API in `server/` using Chi, sqlc, PostgreSQL, WebSockets, and a local daemon for agent execution.
- Frontend: Next.js App Router app in `apps/web/` with feature-based state and UI modules.
- E2E: Playwright tests in `e2e/`.
- Secondary app: `apps/mobile/` exists, but the main product surface in this repo is the web app plus Go backend.

## Instruction Layout

- `server/AGENTS.md` applies to backend, daemon, sqlc, migrations, and runtime execution code.
- `apps/web/AGENTS.md` applies to the Next.js app, shared frontend utilities, and web tests.
- `e2e/AGENTS.md` applies to Playwright coverage and test fixtures.

If a child file conflicts with this one, follow the child file for that subtree.

## Working Rules

- Explore the existing code path first. Match the current architecture before adding new abstractions.
- Keep instructions, docs, and code aligned. If you change an execution workflow, update the instruction files or tests that describe it.
- Do not hand-edit generated code in `server/pkg/db/generated/`.
- Keep comments in code in English only.
- Unless explicitly requested, do not add compatibility shims, dual-write logic, fallback paths, or legacy adapters.
- Avoid broad refactors unless the task requires them.
- Prefer deleting obsolete paths over preserving both old and new behavior when the product is not yet live.

## Commands

Top-level commands:

```bash
make setup
make start
make stop
make check
make test
make build
pnpm install
pnpm build
pnpm typecheck
pnpm test
```

Worktree-aware flows:

```bash
make worktree-env
make setup-worktree
make start-worktree
```

## Verification

- Do not run heavy verification by default. Run checks when the user asks for verification or when the task explicitly requires proof.
- When verification is requested, prefer the narrowest relevant check first, then finish with the broader command that matches the changed surface.
- Full-repo verification is `make check`.

## Commits

- Keep commits atomic and grouped by logical intent.
- Use conventional messages such as `feat(web): ...`, `fix(cli): ...`, `refactor(daemon): ...`, `test(server): ...`, `docs: ...`, or `chore(scope): ...`.

## Practical Defaults

- Prefer `rg` for search.
- Prefer repo-local patterns over inventing new ones.
- Keep root instructions short; put subsystem detail in child `AGENTS.md` files.
