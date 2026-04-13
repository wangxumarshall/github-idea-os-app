# Task Plan: Rewrite AGENTS.md Hierarchy

## Goal

Rewrite the root and child `AGENTS.md` files so they align with official OpenAI Codex and agents.md guidance, stay faithful to this repository's real commands and workflows, and clearly encode worktree isolation, verification, security, and project-specific gotchas.

## Current Phase

Phase 5 complete

## Phases

### Phase 1: Discovery And Audit
- [x] Read the current root and child `AGENTS.md` files
- [x] Read `CONTRIBUTING.md`
- [x] Read `Makefile`, package scripts, and helper scripts
- [x] Review official AGENTS.md / Codex guidance
- [x] Capture repo-specific constraints and gotchas
- **Status:** complete

### Phase 2: Draft The New Hierarchy
- [x] Define the root structure and section layout
- [x] Decide what belongs in root vs child files
- [x] Rewrite root `AGENTS.md`
- [x] Rewrite `server/AGENTS.md`
- [x] Rewrite `apps/web/AGENTS.md`
- [x] Rewrite `e2e/AGENTS.md`
- **Status:** complete

### Phase 3: Verify Consistency
- [x] Re-read all updated AGENTS files
- [x] Confirm commands, paths, and claims are accurate
- [x] Check for contradictions between root and children
- **Status:** complete

### Phase 4: Delivery
- [x] Summarize the rewrite and key changes
- [x] Include the official sources used
- **Status:** complete

### Phase 5: Commit
- [x] Stage the AGENTS and planning-file updates
- [x] Commit the current rewrite with a conventional docs message
- **Status:** complete

## Decisions Made

| Decision | Rationale |
|----------|-----------|
| Keep the root file concise and global | Matches official AGENTS guidance and reduces instruction conflicts |
| Put architecture and local command details in the nearest child file | Improves relevance and adherence for subtree-specific work |
| Encode `.env` / `.env.worktree` / shared PostgreSQL rules explicitly | These are repo-critical and easy to get wrong |
| Document that `make check` does not run lint | The current repo behavior differs from common templates and must not be misrepresented |
| Keep `planning-with-files` mandatory for complex tasks, but exempt simple housekeeping tasks | Matches the current root instruction policy |

## Errors Encountered

| Error | Attempt | Resolution |
|-------|---------|------------|
| Earlier AGENTS rewrite attempts were interrupted mid-edit | 1 | Restart from the current workspace state and apply a full clean rewrite |
