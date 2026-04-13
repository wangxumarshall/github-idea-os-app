# Task Plan: Enforce Planning Workflow In AGENTS.md

## Goal

Add a mandatory planning-workflow rule to the root `AGENTS.md` so plan-mode and multi-step work in this repository must use the `planning-with-files` skill and keep the repo-root planning files updated.

## Current Phase

Phase 4 complete

## Phases

### Phase 1: Discovery
- [x] Read the current root `AGENTS.md`
- [x] Read the `planning-with-files` skill instructions
- [x] Confirm the canonical planning filenames used by the skill
- **Status:** complete

### Phase 2: Edit
- [x] Add a new `Planning Workflow (IMPORTANT)` section to `AGENTS.md`
- [x] State that `planning-with-files` is mandatory for plan-mode and multi-step work
- [x] State that `task_plan.md`, `findings.md`, and `progress.md` are required
- **Status:** complete

### Phase 3: Verification
- [x] Re-read the updated `AGENTS.md`
- [x] Confirm the new rule is phrased as a hard constraint
- [x] Summarize the change to the user
- **Status:** complete

### Phase 4: Delivery
- [x] Stage the AGENTS and planning-file updates
- [x] Commit the change with a conventional docs commit message
- **Status:** complete

## Decisions Made

| Decision | Rationale |
|----------|-----------|
| Use the skill's real filenames: `task_plan.md`, `findings.md`, `progress.md` | Those are the canonical files the skill expects and auto-reads |
| Add the rule only to the root `AGENTS.md` | The planning requirement is global and should apply to the whole repo |

## Errors Encountered

| Error | Attempt | Resolution |
|-------|---------|------------|
| Earlier broad rewrite attempts were interrupted mid-edit | 1 | Apply only the requested planning constraint in a small, focused patch |
