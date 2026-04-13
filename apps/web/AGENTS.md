# Web AGENTS.md

Applies to `apps/web/`. Read root `AGENTS.md` first, then this file for frontend-specific rules.

## Architecture

- `app/` — App Router shell layer (route files MUST stay thin)
- `features/` — domain logic, stores, feature-specific UI
- `shared/` — cross-feature API clients, types, hooks, utilities
- `components/ui/` — shared UI primitives (reuse before creating new ones)
- `test/setup.ts` — shared Vitest setup entry point

## Key Commands (YOU MUST use these)

| Task | Command |
|------|---------|
| Dev server | `pnpm --filter @multica/web dev` |
| Build | `pnpm --filter @multica/web build` |
| Preview build | `pnpm --filter @multica/web start` |
| Typecheck | `pnpm --filter @multica/web typecheck` |
| Lint | `pnpm --filter @multica/web lint` |
| Lint autofix | `pnpm --filter @multica/web lint -- --fix` |
| All web tests | `pnpm --filter @multica/web test` |
| Focused test | `pnpm --filter @multica/web test -- path/to/file.test.tsx` |

## Web Rules (NEVER violate)

- YOU MUST keep route files thin — move domain logic into `features/`
- Prefer re-exporting feature entry points from `app/` when that pattern already exists
- Within a feature, prefer relative imports; across features or into shared code, use the `@/` alias
- Use function components and hooks — NEVER introduce class components
- Use one Zustand store per feature or domain — do not add React Context for state that belongs in a store
- Stores MUST NOT use router hooks — keep navigation in components
- Cross-store reads belong inside actions via `useOtherStore.getState()`
- Preserve the established dependency direction: `workspace -> auth`, `realtime -> auth`, `issues -> workspace`
- Use `apps/web/shared/logger.ts` for logging — NEVER add new `console.*` calls outside the logger implementation
- Keep TypeScript strict — NEVER add new `any` unless an external boundary forces it; if so, isolate it and leave a short justification
- For new files, prefer import order `React/Next -> third-party -> internal`; match surrounding file when editing existing code
- Reuse shared UI primitives and design tokens before inventing custom colors or components

## Verification

- Use Vitest with Testing Library; mock external or third-party boundaries only
- Route behavior, store logic, editor behavior, and shared UI contract changes MUST add or update colocated tests
- After web changes, YOU MUST run `pnpm --filter @multica/web typecheck` + `pnpm --filter @multica/web lint` + narrowest relevant test
- For cross-page or user-visible flows, finish with the relevant Playwright spec or `make check`

## Gotchas

- `playwright.config.ts` lives at the repo root, NOT inside `e2e/`
- Direct Playwright runs do NOT start services for you
- Existing code still has some legacy `console.*` usage — NEVER copy that pattern into new code
- `FRONTEND_PORT` drives `pnpm --filter @multica/web dev`
- `apps/web/tsconfig.json` is strict and includes the whole app, so unrelated type errors can surface during web changes
- `pnpm build` changes `.next` to production assets, breaking hot reload — use `pnpm --filter @multica/web dev` during development
