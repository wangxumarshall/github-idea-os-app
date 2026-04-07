# Task Plan: GitHub IdeaOS MVP

## Goal
Add a production-usable MVP inside the existing Multica web app that lets a user configure a GitHub repository, browse idea files under `ideas/`, create a new idea, edit idea content in a minimal editor, and auto-save changes back to GitHub with commits.

## Current Phase
Phase 6

## Phases

### Phase 1: Requirements & Discovery
- [x] Audit existing workspace repo settings, editor, navigation, and backend patterns
- [x] Decide the smallest GitHub-backed MVP that fits current architecture
- [x] Document findings in findings.md
- **Status:** complete

### Phase 2: Data Model & API Design
- [x] Define IdeaOS workspace configuration shape
- [x] Define backend GitHub proxy endpoints for list/read/create/update
- [x] Define frontend data model and page structure
- **Status:** complete

### Phase 3: Backend Implementation
- [x] Add GitHub content client and frontmatter helpers
- [x] Add IdeaOS handlers and routes
- [x] Reuse workspace update flow for IdeaOS configuration
- [x] Add tests for GitHub content translation and handlers
- **Status:** complete

### Phase 4: Frontend Implementation
- [x] Add Ideas list page
- [x] Add Idea editor page with autosave
- [x] Add new-idea flow
- [x] Add workspace settings UI for GitHub IdeaOS config
- [x] Add sidebar navigation entry
- **Status:** complete

### Phase 5: Verification & Runtime Validation
- [x] Run targeted frontend/backend tests
- [x] Run broader typecheck and test suites
- [x] Rebuild and restart services
- [x] Verify the live UI and GitHub API path behavior
- **Status:** complete

### Phase 6: Delivery
- [x] Summarize the shipped MVP
- [x] Document current limits and next upgrades
- **Status:** in_progress

## MVP Scope Decision
- Implement as a new web feature inside the current Next.js + Go app, not as a separate Flutter app.
- Use GitHub Contents API through the existing backend for safety.
- Store GitHub IdeaOS configuration at the workspace level.
- Support a single configured GitHub repo/path pair for the MVP.
- Keep AI auto-structuring lightweight for now: title normalization, timestamps, tags parsing from frontmatter, and generated file template.

## Key Questions
1. Where should GitHub repo configuration live in the current data model and UI?
2. What is the cleanest backend integration point for reading/writing repo files without introducing a large new auth subsystem?
3. Which parts of the user’s broader vision belong in this MVP versus future phases?

## Decisions Made
| Decision | Rationale |
|----------|-----------|
| Build inside existing Multica web app | Fastest path to a working feature in this repo |
| Backend mediates GitHub API access | Avoid exposing GitHub tokens directly to browser code |
| One repo / one `ideas/` directory for MVP | Matches user’s core spec and keeps scope controlled |

## Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
| Existing planning files were for an unrelated deployment task | 1 | Replaced them with a fresh IdeaOS implementation plan |
