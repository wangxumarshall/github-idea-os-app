# E2E AGENTS.md

Applies to `e2e/` and repo-root `playwright.config.ts`. Read root `AGENTS.md` first, then this file for Playwright-specific rules.

## Test Shape

- Keep tests self-contained
- Use `TestApiClient` for setup and teardown
- Create fixtures through the API, then validate behavior through the browser UI
- Clean up created data in `afterEach`
- Each test MUST own the data it depends on

## Key Commands (YOU MUST use these)

| Task | Command |
|------|---------|
| All E2E specs | `pnpm exec playwright test` |
| Focused spec | `pnpm exec playwright test e2e/issues.spec.ts` |
| Spec with custom URL | `PLAYWRIGHT_BASE_URL=http://localhost:3000 pnpm exec playwright test e2e/navigation.spec.ts` |
| Cross-surface verification | `make check` |

## Practical Rules (NEVER violate)

- NEVER couple E2E tests to frontend implementation details or imported app code
- Prefer stable locators and explicit waits around navigation or async UI updates
- NEVER rely on state created by another test
- NEVER seed E2E state by editing the database directly when the API fixture path can do it
- Keep fixtures lightweight — `e2e/fixtures.ts` and `e2e/helpers.ts` MUST remain API-oriented and build-decoupled from the web app
- Prefer API setup and browser verification — that is the contract of this suite

## Verification

- When a spec changes shared auth, navigation, or issue flows, run the narrowest related spec first, then broaden if needed
- Direct Playwright commands expect backend and frontend to already be running
- `make check` can start missing services for E2E, but it is a heavy command — prefer focused specs during development

## Gotchas

- Playwright base URL resolves from `PLAYWRIGHT_BASE_URL` -> `FRONTEND_ORIGIN` -> defaults to `http://localhost:3000`
- `playwright.config.ts` lives at the repo root, NOT inside `e2e/`
- `make check` is the only built-in flow that auto-starts missing services — direct `pnpm exec playwright test` does NOT
- E2E tests create their own workspace and issue fixtures; they do NOT depend on seed data
