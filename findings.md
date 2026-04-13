# Findings: Planning Workflow Constraint

## Current Repository State

- The root `AGENTS.md` did not previously contain an explicit rule requiring the `planning-with-files` skill.
- The repository already has repo-root planning files named `task_plan.md`, `findings.md`, and `progress.md`.
- The `planning-with-files` skill uses those filenames as its canonical working-memory files.

## Constraint To Encode

- Plan-mode tasks, research tasks, and other multi-step tasks should be treated as planning workflows.
- The constraint needs to be stated as a hard rule using wording like `YOU MUST`.
- The rule belongs in the root `AGENTS.md`, because it applies across `server/`, `apps/web/`, and `e2e/`.

## Naming Note

- The user referred to `plan.md` / `finding.md` / `progress.md`.
- The installed skill's actual canonical files are `task_plan.md`, `findings.md`, and `progress.md`.
- The repository rule should therefore use the skill's real filenames to avoid fighting the automation.
