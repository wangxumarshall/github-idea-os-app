# Web Instructions

These instructions apply to `apps/web/`. Follow root `AGENTS.md` first, then this file for frontend-specific rules.

## Architecture

- `app/` is the App Router shell layer. Route files must stay thin.
- `features/` holds domain logic, stores, and feature-specific UI.
- `shared/` holds cross-feature API clients, types, hooks, and utilities.
- `components/ui/` contains shared UI primitives. Reuse them before creating new ones.
- `test/setup.ts` is the shared Vitest setup entry point.

## Key Commands (YOU MUST use these exact commands)

- Run web dev server: `pnpm --filter @multica/web dev`
- Build web app: `pnpm --filter @multica/web build`
- Preview web build: `pnpm --filter @multica/web start`
- Typecheck: `pnpm --filter @multica/web typecheck`
- Lint: `pnpm --filter @multica/web lint`
- Lint with autofix when needed: `pnpm --filter @multica/web lint -- --fix`
- Run all web tests: `pnpm --filter @multica/web test`
- Run a focused web test file: `pnpm --filter @multica/web test -- path/to/file.test.tsx`

## Web Rules (IMPORTANT - NEVER violate)

- YOU MUST keep route files thin and move domain logic into `features/`.
- Prefer re-exporting feature entry points from `app/` when that pattern already exists for the route.
- Within a feature, prefer relative imports. Across features or into shared code, use the `@/` alias.
- Use function components and hooks. Do not introduce class components.
- Use one Zustand store per feature or domain. Do not add React Context for state that belongs in a store.
- Stores MUST NOT use router hooks. Keep navigation in components.
- Cross-store reads belong inside actions via `useOtherStore.getState()`.
- Preserve the established dependency direction: `workspace -> auth`, `realtime -> auth`, `issues -> workspace`.
- Use `apps/web/shared/logger.ts` for new logging. Do not add new `console.*` calls outside the logger implementation or temporary debugging you remove before finishing.
- Keep TypeScript strict. Do not add new `any` unless an external boundary forces it; if that happens, isolate it and leave a short justification.
- For new files, prefer imports ordered `React/Next -> third-party -> internal`, while still matching the surrounding file when editing existing code.
- Reuse shared UI primitives and design tokens before inventing custom colors or components.

## Testing And Verification

- Use Vitest with Testing Library.
- Mock external or third-party boundaries only.
- Route behavior, store logic, editor behavior, and shared UI contract changes MUST add or update colocated tests.
- After web changes, YOU MUST run `pnpm --filter @multica/web typecheck`, `pnpm --filter @multica/web lint`, and the narrowest relevant test command.
- For cross-page or user-visible flows, finish with the relevant Playwright spec or `make check`.

## Gotchas

- `playwright.config.ts` lives at the repo root, not inside `e2e/`.
- Direct Playwright runs do not start services for you.
- Existing code still has some legacy `console.*` usage. Do not copy that pattern into new code.
- `FRONTEND_PORT` drives `pnpm --filter @multica/web dev`.
- `apps/web/tsconfig.json` is strict and includes the whole app, so unrelated type errors can surface during web changes.
