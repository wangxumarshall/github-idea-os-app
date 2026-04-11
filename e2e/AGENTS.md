# E2E Instructions

These instructions apply to `e2e/`.

## Test Shape

- Keep tests self-contained.
- Use `TestApiClient` for setup and teardown.
- Create fixtures through the API, then validate behavior through the browser UI.
- Clean up test data in `afterEach`.

## Practical Rules

- Avoid coupling E2E tests to frontend implementation details or imported app code.
- Prefer stable locators and explicit waits around navigation or async UI updates.
- Each test should own the data it depends on. Do not rely on state created by a different test.
- Direct Playwright runs expect backend and frontend to already be running.

## Commands

```bash
pnpm exec playwright test e2e/issues.spec.ts
pnpm exec playwright test e2e/auth.spec.ts
make check
```

## Notes

- `make check` can orchestrate missing services, but it is a heavy command. Use it only when the user asks for broad verification.
- Keep fixtures lightweight. `e2e/fixtures.ts` should remain build-decoupled from the web app and use raw API calls.
