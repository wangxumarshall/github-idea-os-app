# Skill Execution Model

This note explains what a skill is in Multica today, how it is implemented, how it works with agents and runtimes, and what problem the design is trying to solve.

## Summary

In the current implementation, a skill is a workspace-scoped reusable capability package.

It contains:
- a primary markdown body, stored as the skill's `content`
- optional supporting files, stored separately per path
- metadata such as `name`, `description`, and `config`

An agent does not inline its skills into its own record. Instead, it references workspace skills through the `agent_skill` junction table. When a runtime claims a task for that agent, the daemon resolves the assigned skills and writes them into the task's execution environment using the native discovery path of the underlying provider.

That is the key design choice: Multica stores skills centrally, assigns them to agents by reference, and materializes them into runtime-native filesystem layouts only at execution time.

## What A Skill Does

A skill is meant to package repeatable operational knowledge so that multiple agents can execute the same practice consistently.

Typical examples in the product copy and current implementation are:
- code review conventions
- deployment procedures
- migration workflows
- repository-specific conventions

In practice, a skill can contain more than just prose. It can also include supporting files such as templates, examples, or helper assets alongside `SKILL.md`.

Current management surfaces:
- Web UI for creating, importing, editing, viewing, and deleting skills
- CLI commands for listing, creating, importing, updating, deleting, and managing skill files
- HTTP API handlers for workspace-scoped CRUD and agent assignment

Relevant implementation entrypoints:
- `apps/web/features/skills/components/skills-page.tsx`
- `server/cmd/multica/cmd_skill.go`
- `server/internal/handler/skill.go`

## Data Model

The current backend model is split into three parts:

### `skill`

The `skill` row stores the reusable capability definition itself:
- `workspace_id`
- `name`
- `description`
- `content`
- `config`
- `created_by`
- timestamps

`content` is the main skill body and is treated as the `SKILL.md` content when the daemon writes the skill into an execution environment.

### `skill_file`

Supporting files are stored separately in `skill_file`:
- `skill_id`
- `path`
- `content`

This allows a skill to behave like a small directory tree rather than a single text blob.

### `agent_skill`

Agent assignment is modeled as a many-to-many relationship:
- one workspace skill can be reused by many agents
- one agent can have many skills

The SQL source of truth is `server/pkg/db/queries/skill.sql`.

## Import And Authoring

Skills can be created manually or imported from external skill ecosystems.

### Manual authoring

Users can create a skill directly in the UI or CLI, provide a name and description, then edit the main content and supporting files. The UI treats skills as reusable definitions owned by the workspace, not by a single agent.

### Import sources

The current server supports importing from:
- `clawhub.ai`
- `skills.sh`

Import behavior:
- detect the source from the URL
- fetch the remote `SKILL.md`
- infer or parse name and description where possible
- collect supporting files when the source exposes them
- store the imported result as a local Multica skill plus `skill_file` rows

This matters because imported skills stop being an external reference after import. They become first-class workspace assets that Multica can assign, version through normal edits, and inject into runtime environments.

The import implementation lives in `server/internal/handler/skill.go`.

## How Skills Attach To Agents

Skills are assigned from the workspace skill library to a specific agent.

The user-facing workflow is:
1. create or import a workspace skill
2. open an agent
3. add one or more workspace skills to that agent

The current agent skills UI is intentionally simple:
- the Skills page manages the skill definitions
- the Agent page manages which of those skills are attached to a given agent

When the frontend updates agent skills, it sends the complete desired set of `skill_ids` to `PUT /api/agents/{id}/skills`.

The backend then:
1. clears the existing `agent_skill` rows for that agent
2. inserts the requested set
3. publishes an agent status event with the updated skills

This full-replacement approach keeps the assignment state explicit and avoids ambiguous partial updates.

Relevant files:
- `apps/web/app/(dashboard)/agents/page.tsx`
- `apps/web/shared/api/client.ts`
- `server/internal/handler/skill.go`

## How Skills Reach A Runtime

The runtime never reads skills directly from the database. The flow goes through the task claim and daemon setup path.

### 1. Task claim response includes the agent's resolved skills

When a runtime claims a task, the server builds a task response that includes fresh agent data:
- agent identity
- agent instructions
- agent skills with their supporting files

This resolution happens in the task service and daemon claim handler rather than in the runtime itself.

Relevant files:
- `server/internal/service/task.go`
- `server/internal/handler/daemon.go`

### 2. The daemon converts skills into execution-environment context

When the daemon starts the task, it converts each skill into the `execenv` representation:
- skill name
- main content
- supporting files

The daemon then prepares or reuses the task-scoped execution environment and writes the skill files into the right location for the selected provider.

Relevant files:
- `server/internal/daemon/daemon.go`
- `server/internal/daemon/execenv/execenv.go`

### 3. `execenv` writes skills to provider-native locations

The current implementation uses provider-specific discovery paths:

- Claude: `{workDir}/.claude/skills/{skill-name}/SKILL.md`
- OpenCode: `{workDir}/.config/opencode/skills/{skill-name}/SKILL.md`
- Codex: `{task-scoped CODEX_HOME}/skills/{skill-name}/SKILL.md`
- Fallback or unknown providers: `{workDir}/.agent_context/skills/{skill-name}/SKILL.md`

Supporting files are written under the same skill directory beside `SKILL.md`.

This behavior is implemented in:
- `server/internal/daemon/execenv/context.go`
- `server/internal/daemon/execenv/codex_home.go`

The tests in `server/internal/daemon/execenv/execenv_test.go` verify the expected layout for Claude, OpenCode, and Codex-specific environment setup.

## Why Codex Uses A Task-Scoped `CODEX_HOME`

Codex is handled differently from Claude and OpenCode.

Instead of writing skills into the task workdir, Multica creates a task-scoped `CODEX_HOME` and seeds it from the shared user home:
- auth is symlinked so existing login state is reused
- config files are copied so task-level changes do not mutate the shared home
- skills are written into that task-local `skills/` directory

This achieves two goals:
- Codex can discover skills through its native home-based mechanism
- task execution does not pollute the operator's global `~/.codex/skills/`

That is an isolation choice, not just a path difference.

## What `CLAUDE.md` And `AGENTS.md` Do

Multica also writes a runtime-specific top-level instruction file:
- Claude gets `CLAUDE.md`
- Codex and OpenCode get `AGENTS.md`

This file is not the skill itself.

Its role is to describe the Multica runtime environment:
- use `multica` CLI instead of direct API calls
- understand plan vs build mode
- know which repository is preferred
- know which skills are installed
- follow issue and delivery workflow rules

In other words:
- skills hold reusable capability definitions
- `CLAUDE.md` or `AGENTS.md` holds task-scoped runtime guidance

The implementation lives in `server/internal/daemon/execenv/runtime_config.go`.

## Skills Are Not Inlined Into The Task Prompt

The task prompt is intentionally minimal.

`server/internal/daemon/prompt.go` builds only the basic assignment prompt:
- execution mode
- issue ID
- the first CLI action to take

Detailed runtime instructions live in `CLAUDE.md` or `AGENTS.md`, and skills are discovered through the filesystem layout described above.

This separation is also enforced by tests. `server/internal/daemon/daemon_test.go` explicitly checks that skill content is not embedded directly into the prompt.

That matters because prompt text is transient, while skills are treated as structured runtime context that the provider can discover natively.

## Why Skills Belong To Agents, Not Runtimes

In the current design:
- the agent owns the working style and reusable capabilities
- the runtime owns the compute environment and available provider CLI

This lets the same agent definition run on compatible runtimes without redefining its skills every time. The runtime is the execution container; the agent is the behavioral unit.

If skills were bound to runtimes instead:
- they would describe machine configuration rather than agent behavior
- the same agent could behave differently just because it ran elsewhere
- workspace-level skill reuse would be weaker

Binding skills to agents keeps behavior portable, while provider-specific injection keeps execution compatible.

## Product And Engineering Value

This design provides several concrete benefits.

### Reuse

Teams can write a deployment practice, code-review checklist, or repo convention once and attach it to many agents.

### Composition

An agent can combine:
- its own identity and instructions
- task-scoped runtime guidance
- multiple reusable skills

That is more maintainable than putting everything in one long prompt.

### Provider adaptation

Multica can support multiple agent providers without forcing one universal prompt format. Each provider gets skills through the path it already knows how to read.

### Isolation

Skill materialization happens per task environment. This is especially visible in Codex with task-scoped `CODEX_HOME`, but the same principle applies across providers: runtime context is prepared for the task instead of globally mutating the host environment.

### Operational consistency

Because the daemon always resolves an agent's current skills at task claim time, execution behavior matches the latest workspace configuration rather than a stale local copy.

## End-To-End Effect

The overall effect is:

1. a team creates or imports a skill into the workspace
2. one or more agents are assigned that skill
3. a runtime claims a task for one of those agents
4. the server includes the resolved skills in the task payload
5. the daemon writes those skills into the provider-native execution environment
6. the underlying agent CLI discovers and uses them during task execution

So the purpose of the current skill system is not just to store reusable instructions. It is to turn team knowledge into an execution-ready capability layer that can be attached to agents, transported through task orchestration, and activated inside real runtime environments.
