# Findings & Decisions

## Requirements
- Build a GitHub-backed “IdeaOS” feature for this project.
- Core UX must be minimal: list ideas, create idea, edit idea, auto-save.
- GitHub is the system of record and version history.
- User wants the product direction, but the implementation must be realistic in the current codebase.

## Current Codebase Findings
- The project is an existing Go backend + Next.js frontend product, not a Flutter/mobile app.
- Workspace records already support arbitrary JSON in `settings` and `repos`, making them a natural place for IdeaOS configuration.
- The frontend already has a polished markdown editor (`ContentEditor`) that can be reused.
- The settings area already has a repositories tab, so IdeaOS configuration can fit existing product structure.
- The app already has established dashboard navigation patterns and workspace-scoped state stores.
- `workspace.settings` is returned to the frontend as-is today, so any GitHub PAT stored there must be redacted in responses.
- Existing workspace update flows patch specific fields and do not currently send full `settings`, reducing the risk of clobbering hidden IdeaOS secrets.
- The sidebar navigation is static and straightforward to extend with a new `Ideas` entry.
- The backend handler tests are constructed from `handler.New(...)`, so adding a handler-owned GitHub service/client is testable as long as it can be swapped after construction.

## MVP Translation
- Deliver a web-first MVP inside the existing dashboard.
- Add an `Ideas` feature rather than a separate standalone app.
- Use backend GitHub proxy endpoints instead of browser-direct GitHub calls.
- Store GitHub configuration in the workspace settings / repos JSON.
- Keep auto-structuring modest in MVP:
  - frontmatter generation
  - updated timestamps
  - title normalization
  - tags read/write support

## Implementation Decisions
| Decision | Rationale |
|----------|-----------|
| Use dedicated IdeaOS config endpoints instead of generic `updateWorkspace(settings)` for PAT writes | Prevent accidental token exposure and preserve hidden token state |
| Store non-secret config and secret token together in workspace settings, but redact token from normal workspace responses | Avoid a schema migration while keeping browser responses safe |
| Build GitHub integration against the Contents API only | Covers MVP list/read/create/update requirements with minimal complexity |
| Model each idea as `ideas/<slug>.md` with YAML frontmatter + markdown body | Matches the user spec and keeps file format portable |

## Open Questions
- Whether GitHub auth should be PAT-based or OAuth-backed for MVP.
- Whether idea tags should be manually editable in MVP or purely inferred from content.
- Whether to save every idea to `ideas/<slug>.md` only, or also support nested folders.
