# E2E Instructions

These instructions apply to `e2e/` and the repo-root `playwright.config.ts`. Follow root `AGENTS.md` first, then this file for Playwright-specific rules.

## Test Shape

- Keep tests self-contained.
- Use `TestApiClient` for setup and teardown.
- Create fixtures through the API, then validate behavior through the browser UI.
- Clean up created data in `afterEach`.
- Each test must own the data it depends on.

## Key Commands (YOU MUST use these exact commands)

- Run all E2E specs: `pnpm exec playwright test`
- Run a focused E2E spec: `pnpm exec playwright test e2e/issues.spec.ts`
- Run a focused E2E spec against a specific frontend URL: `PLAYWRIGHT_BASE_URL=http://localhost:3000 pnpm exec playwright test e2e/navigation.spec.ts`
- Broad cross-surface verification: `make check`

## Practical Rules (IMPORTANT - NEVER violate)

- Avoid coupling E2E tests to frontend implementation details or imported app code.
- Prefer stable locators and explicit waits around navigation or async UI updates.
- NEVER rely on state created by another test.
- NEVER seed E2E state by editing the database directly when the API fixture path can do it.
- Keep fixtures lightweight. `e2e/fixtures.ts` and `e2e/helpers.ts` should remain API-oriented and build-decoupled from the web app.
- Prefer API setup and browser verification. That is the contract of this suite.

## Verification And Gotchas

- Direct Playwright commands expect backend and frontend to already be running.
- `make check` can start missing services for E2E, but it is a heavy command.
- Playwright base URL resolves from `PLAYWRIGHT_BASE_URL`, then `FRONTEND_ORIGIN`, then defaults to `http://localhost:3000`.
- When a spec changes shared auth, navigation, or issue flows, run the narrowest related spec first, then broaden if needed.
