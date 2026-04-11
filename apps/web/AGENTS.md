# Web Instructions

These instructions apply to `apps/web/`.

## Architecture

- `app/` is the App Router shell layer. Keep route files thin.
- `features/` holds business logic by domain.
- `shared/` holds cross-feature API, types, and utilities.
- `test/` holds shared frontend test setup.

## Web Rules

- Prefer re-exporting feature pages from `app/` instead of placing business logic in route files.
- Within a feature, use relative imports. Across features or into shared code, use the `@/` alias.
- Use one Zustand store per feature domain. Do not add React Context for shared data that belongs in a store.
- Stores must not use router hooks. Keep navigation in components.
- Cross-store reads should use `useOtherStore.getState()` inside actions.
- Preserve the dependency direction already documented in the repo: workspace -> auth, realtime -> auth, issues -> workspace.

## UI Rules

- Prefer shadcn components and design tokens over custom primitives and hardcoded colors.
- Keep feature-specific UI inside its feature module.
- Pay attention to overflow, truncation, spacing, and alignment.
- Follow the repo style already in use: strict TypeScript, 2-space indentation, double quotes, semicolons, PascalCase components, camelCase hooks and helpers.

## Commands

```bash
pnpm --filter @multica/web dev
pnpm --filter @multica/web build
pnpm --filter @multica/web typecheck
pnpm --filter @multica/web test
pnpm --filter @multica/web lint
```

## Testing

- Use Vitest with Testing Library.
- Mock external or third-party boundaries only.
- Add or update colocated tests when changing route behavior, store logic, editor behavior, or shared UI contracts.
