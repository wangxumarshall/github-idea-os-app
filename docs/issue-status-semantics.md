# Issue Status Semantics

This note defines the intended meaning of the current issue statuses without changing the status machine itself.

## Current Statuses

### `backlog`
- Meaning: captured work that is not yet ready to start.
- Entry criteria: the issue is worth keeping, but it is still unscheduled, underspecified, or lower priority than current work.
- Exit criteria: move to `todo` when the issue is ready to be acted on, or close/cancel if it is no longer relevant.
- Value: separates idea capture from execution-ready work.
- Risk: can overlap with `todo` when teams do not define "ready".
- Simplification view: this is the main candidate for future simplification if the product later chooses a smaller workflow.

### `todo`
- Meaning: prioritized and actionable work that can be started now.
- Entry criteria: scope is clear enough, dependencies are acceptable, and someone could reasonably pick it up.
- Exit criteria: move to `in_progress` when active implementation or active investigation begins.
- Value: distinguishes "ready now" from "captured for later".
- Risk: becomes redundant if teams never use `backlog` with a strict readiness bar.
- Simplification view: keep for now, but revisit together with `backlog` in a future migration.

### `in_progress`
- Meaning: active work is underway.
- Entry criteria: someone is actively implementing, investigating, or executing the issue.
- Exit criteria: move to `in_review` when delivery is ready for review, or `blocked` when work cannot proceed.
- Value: essential signal for active ownership.
- Risk: can be overloaded if planning and implementation are not separated.
- Simplification view: should remain.

### `in_review`
- Meaning: implementation is ready enough for review, validation, or delivery checks.
- Entry criteria: the agent or human has produced a reviewable branch or equivalent delivery artifact.
- Exit criteria: move to `done` when accepted, or back to `in_progress` when changes are requested.
- Value: this is the correct workflow hook for server-side PR automation and review handoff.
- Risk: can be undermined if agents treat temporary automation lag as a blocker.
- Simplification view: should remain.

### `done`
- Meaning: accepted and complete.
- Entry criteria: review is complete, work is accepted, and no further delivery action is pending.
- Exit criteria: reopen to `todo` or `in_progress` only if new work is discovered.
- Value: essential terminal state.
- Risk: low.
- Simplification view: should remain.

### `blocked`
- Meaning: work cannot proceed because of a real blocker.
- Entry criteria: a concrete dependency, permission, missing information, environment issue, or product decision prevents progress.
- Exit criteria: move back to `in_progress` once the blocker is removed.
- Value: communicates genuine interruption and need for intervention.
- Risk: high if misused as a generic waiting state.
- Simplification view: should remain, but usage must be tighter.

## Observations

- The current set is defensible.
- The real ambiguity is `backlog` vs `todo`.
- `blocked` should never mean "waiting for automation", "waiting for confirmation", or "I cannot see the PR yet".
- Plan/build gating should live in a separate execution-stage field, not in the status enum.

## Recommendation

- Keep the current statuses for now.
- Tighten their semantics in product logic, UI copy, and agent instructions.
- Revisit `backlog` vs `todo` only in a dedicated future migration after plan/build execution staging is stable.
