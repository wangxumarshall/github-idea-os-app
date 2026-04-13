<p align="center">
  <img src="docs/assets/banner.jpg" alt="Multica — humans and agents, side by side" width="100%">
</p>

<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/logo-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="docs/assets/logo-light.svg">
  <img alt="Multica" src="docs/assets/logo-light.svg" width="50">
</picture>

# Multica

**Local-first automatic product factory for existing agent CLIs.**

Open-source system for shipping products through the loop `idea -> spec -> repo -> root task -> fanout -> implement/verify -> delivery`.<br/>
Keep memory, policy, repo/worktree control, and swarm execution in the local runtime; use the cloud as a thin control plane.

[![CI](https://github.com/multica-ai/multica/actions/workflows/ci.yml/badge.svg)](https://github.com/multica-ai/multica/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![GitHub stars](https://img.shields.io/github/stars/multica-ai/multica?style=flat)](https://github.com/multica-ai/multica/stargazers)

[Website](https://multica.ai) · [Cloud](https://multica.ai/app) · [Self-Hosting](SELF_HOSTING.md) · [Contributing](CONTRIBUTING.md)

**English | [简体中文](README.zh-CN.md)**

</div>

## What is Multica?

Multica is not an AI Jira with bots attached. It is a local-first automatic product factory that owns the loop `idea -> spec -> repo -> root task -> fanout -> implement/verify -> delivery`. Issues, boards, and agent identities are operator surfaces for that loop, not the core product.

The local daemon/runtime is the intelligence core. It is where task-scoped workdirs, repo/worktree control, unified memory, policy enforcement, swarm coordination, session reuse, and future self-evolution belong. The cloud is intentionally thin: login, workspace sync, queueing, visualization, and remote entry.

**Claude Code**, **Codex**, **OpenCode**, **Trae CLI**, and **Hermes Agent** are execution engines inside that system. Multica is the orchestration layer above them, adding unified task, memory, policy, repo checkout, worktree isolation, swarm primitives, and observability instead of replacing their native capabilities.

<p align="center">
  <img src="docs/assets/hero-screenshot.png" alt="Multica board view" width="800">
</p>

## V1 Direction

- **Automatic Product Factory** — the primary loop is `idea/spec -> repo -> task graph -> execution -> delivery`, not just issue tracking.
- **Local-First + Cloud Coordination** — keep group intelligence, unified memory, swarm coordination, policy, and future self-evolution in the local runtime; keep the cloud limited to login, workspace sync, queueing, visualization, and remote entry.
- **Adapt Existing Agent CLIs** — treat Claude Code, Codex, OpenCode, and similar tools as the execution substrate, not as optional integrations into a Multica-native agent.

## Features

- **Product Loop, End to End** — IdeaOS links versioned idea docs, project repos, root tasks, fanout flows, and delivery workflows so product intent can move through one continuous execution system.
- **Local Runtime Kernel** — the daemon on your machine is the execution core, not a thin relay.
- **Existing CLIs as Execution Engines** — Claude Code, Codex, OpenCode, Trae CLI, and Hermes Agent run as the execution layer under Multica orchestration.
- **Unified CLI Orchestration** — Multica adds one task, event, memory, policy, repo/worktree, and capability model above those CLIs.
- **Transparent Memory Layer** — the current memory system uses [memsearch](https://github.com/zilliztech/memsearch), keeping agent memory in reviewable Markdown that works across all supported CLIs.
- **Task-Scoped Execution Environments** — each task gets its own workdir, policy file, runtime-native instruction injection, and controlled repository checkout flow.
- **Execution Memory** — completed and failed runs write back compact execution memory that can be re-injected into future tasks on the same issue.
- **Light Swarm Primitives** — parent/child task relationships and swarm roles are modeled in the backend so work can be decomposed before the full swarm product is built out.
- **Stronger Sandbox Foundation** — the daemon can launch agents on the host or inside Docker, with a policy contract and process-level sandbox driver abstraction.
- **Thin Cloud Control Plane** — the cloud handles identity, workspaces, queues, sync, and observability while complex execution stays local-first.
- **Unified Runtimes** — one dashboard for local daemons and cloud runtimes, with auto-detection of available CLIs and real-time monitoring.
- **Multi-Workspace** — organize work across teams with workspace-level isolation. Each workspace has its own agents, issues, and settings.

## Memory System Roadmap

- **Start / validation phase: `memsearch`** — use [memsearch](https://github.com/zilliztech/memsearch) as the current memory layer because it is effectively zero-cost to adopt, works with every supported agent CLI, and keeps memories as transparent Markdown that humans can inspect and edit directly.
- **Production / self-evolving phase: `memsearch + OpenViking`** — keep `memsearch` as the transparent Markdown layer, then extend upward with [OpenViking](https://github.com/volcengine/OpenViking) for the `viking://` abstraction, `L0/L1/L2` memory tiers, and automatic extraction needed for real swarm intelligence and agent self-evolution.
- **End state: transparent + unified + self-evolving memory** — map the `memsearch` Markdown directory into `viking://user/memories/` so Multica gets transparent human-editable memory, one unified address space, and a self-evolving memory stack in the same system.

## Getting Started

### Multica Cloud

The fastest way to get started — no setup required: **[multica.ai](https://multica.ai)**

### Self-Host with Docker

```bash
git clone https://github.com/multica-ai/multica.git
cd multica
cp .env.example .env
# Edit .env — at minimum, change JWT_SECRET

docker compose up -d                              # Start PostgreSQL
cd server && go run ./cmd/migrate up && cd ..     # Run migrations
make start                                         # Start the app
```

See the [Self-Hosting Guide](SELF_HOSTING.md) for full instructions.

## CLI

The `multica` CLI connects your local machine to Multica — authenticate, manage workspaces, inspect ideas and issues, and run the agent daemon.

```bash
# Install
brew tap multica-ai/tap
brew install multica

# Authenticate and start
multica login
multica daemon start
```

The daemon auto-detects available agent CLIs (`claude`, `codex`, `opencode`, `trae-cli`, `hermes`) on your PATH. When work is assigned, the daemon creates an isolated environment, injects policy/context/skills, runs the selected CLI through the configured sandbox driver, and reports results back.

See the [CLI and Daemon Guide](CLI_AND_DAEMON.md) for the full command reference, daemon configuration, and advanced usage.

## Quickstart

Once you have the CLI installed (or signed up for [Multica Cloud](https://multica.ai)), follow these steps to start your first local-first product run:

### 1. Log in and start the daemon

```bash
multica login           # Authenticate with your Multica account
multica daemon start    # Start the local agent runtime
```

The daemon runs in the background and keeps your machine connected to Multica. It auto-detects agent CLIs (`claude`, `codex`, `opencode`, `trae-cli`, `hermes`) available on your PATH.

### 2. Verify your runtime

Open your workspace in the Multica web app. Navigate to **Settings → Runtimes** — you should see your machine listed as an active **Runtime**.

> **What is a Runtime?** A Runtime is the execution boundary where Multica prepares memory, policy, repo/worktree context, and task runs. It can be your local machine (via the daemon) or a cloud-hosted environment, but V1 is intentionally local-first: the local runtime is where the intelligence kernel lives.

### 3. Create an agent

Go to **Settings → Agents** and click **New Agent**. Pick the runtime you just connected and choose a provider that exists on that runtime. Give your agent a name — this is how it will appear on the board, in comments, and in assignments.

### 4. Start from an issue or idea

Create an issue from the board (or via `multica issue create`), or create an idea in IdeaOS and use it as the source of truth for the project. Use that to drive a root task, then fan out child tasks when needed. The local runtime picks up the work, attaches repo and memory context, executes it with the selected CLI, and reports progress back to Multica.

That's it. You now have a local-first execution loop connected to Multica.

## Architecture

Multica is local-first by design:

- **Local runtime** — the intelligence kernel for memory, policy, repo/worktree control, swarm coordination, and execution.
- **Cloud control plane** — identity, workspace sync, queueing, visualization, and remote entry.
- **External agent CLIs** — execution engines orchestrated by Multica rather than replaced by it.

```
┌─────────────────────────┐      ┌────────────────────────────────────┐
│      Local Runtime      │<---->│        Cloud Control Plane         │
│ daemon + workdirs       │      │ frontend + API + PostgreSQL        │
│ memory/policy/swarm     │      │ auth/sync/queue/visibility         │
└─────────────┬───────────┘      └────────────────────────────────────┘
              │
┌─────────────┴───────────────────────────────────────────────────────┐
│ Claude Code / Codex / OpenCode / Trae CLI / Hermes Agent           │
│ execution engines under Multica orchestration                      │
└─────────────────────────────────────────────────────────────────────┘
```

| Layer | Role | Stack |
|-------|------|-------|
| Local Runtime | Intelligence kernel for task execution, memory, policy, repo/worktree control, and swarm coordination | Local daemon |
| Execution Engines | Existing agent CLIs that actually perform the task work | Claude Code, Codex, OpenCode, Trae CLI, Hermes Agent |
| Frontend | Cloud UI and operator surface | Next.js 16 (App Router) |
| Backend | Thin control plane for workspaces, queues, realtime, and runtime coordination | Go (Chi router, sqlc, gorilla/websocket) |
| Database | Persistent state, runtime metadata, and execution memory | PostgreSQL 17 with pgvector |

## Runtime Kernel Status

This repository is already closer to a local-first orchestration kernel than a classic task-management app. The web app remains intentionally light; most of the real execution logic lives in `server/` and the local daemon.

### Functionality

- **Provider capabilities**
  - Each runtime now publishes capability metadata such as structured streaming, native resume, plan-mode support, approval callbacks, and trajectory-based execution.
  - The daemon and server persist those capabilities in runtime metadata so scheduling and diagnostics no longer depend entirely on provider name checks.
- **Canonical task events**
  - Task message persistence now stores canonical metadata such as `provider`, `call_id`, `status`, and `level`.
  - The daemon forwards `status` and `log` events in addition to text, thinking, tool use, and tool result messages.
- **Task policy contract**
  - Every execution environment now contains `.agent_context/policy.json`.
  - Agents receive the policy path via `MULTICA_POLICY_FILE`, and runtime-native instructions explicitly treat the policy file as a hard contract.
- **Execution memory**
  - Successful runs persist `run_summary` memory.
  - Failed runs persist `failure_pattern` memory.
  - Recent memories for the same issue are injected into later task environments so agents can continue work without relying only on provider-side session state.
- **Light swarm fan-out**
  - The backend now supports parent/child tasks and per-child `swarm_role`.
  - `POST /api/issues/{id}/fanout` and `multica issue fanout` can enqueue child tasks under a running parent task.
  - Child tasks appear in issue task history and preserve their parent linkage.
- **Sandbox driver abstraction**
  - Agent processes can run with `host` or `docker` drivers.
  - Docker mode wraps the agent CLI in `docker run`, mounts the task workdir and required runtime state, and preserves access to the local daemon workflows required by `multica repo checkout`.
- **Hermes Agent integration**
  - Hermes is now a first-class provider in the daemon and runtime registry.
  - Multica creates per-task `HERMES_HOME` state and can launch Hermes through the same backend runtime path as the other providers.

### Implementation

- **Core runtime**
  - `server/pkg/agent/` now contains provider capability metadata, a process-launch abstraction, and a Hermes backend alongside the existing Claude/Codex/OpenCode/Trae backends.
  - `server/internal/daemon/` uses those abstractions to register richer runtime metadata and to launch agents through sandbox-aware process builders.
- **Execution environments**
  - `server/internal/daemon/execenv/` now writes policy files, recent run memory, swarm context, and provider-specific home directories such as `codex-home`, `trae-home`, and `hermes-home`.
- **Persistence**
  - `task_message.metadata` stores canonical runtime event details.
  - `run_memory` stores issue-scoped execution memory records.
  - `agent_task_queue` now includes `parent_task_id` and `swarm_role`.
- **Queue semantics**
  - Fan-out child tasks are allowed in the queue, but the existing issue-level execution model is preserved.
  - Root tasks still obey the original pending-task coalescing rule; child tasks do not replace or weaken that invariant.

### Value

- **Broader provider coverage**
  - The daemon is no longer tied to only two coding agents.
  - New providers can be added behind one backend contract instead of reworking the whole task lifecycle.
- **Safer backend evolution**
  - Policy files, capability metadata, canonical events, and process drivers create stable seams for later work on stronger sandboxing, better scheduling, and more reliable orchestration.
- **Better task continuity**
  - Execution memory allows Multica to recover useful context even when provider-native sessions are unavailable or intentionally disabled.
- **Swarm without a frontend rewrite**
  - Parent/child task modeling is now available in the backend and API, which lets the runtime evolve before a larger web-product redesign.

### Current Limits And TODO

- **Fan-out is modeled, not fully parallelized**
  - Child tasks can be created and tracked, but issue execution is still serialized.
  - This is intentional: the rest of the stack still assumes one active issue execution state at a time.
- **Swarm aggregation is not complete**
  - There is no parent-task rollup result, child-task quorum logic, or reviewer/tester aggregation yet.
  - The web app does not yet have dedicated swarm controls or grouped child-task views.
- **Docker sandboxing is a foundation, not a full isolation story**
  - Docker mode currently exists to harden process launch and provide a cleaner execution boundary.
  - It still relies on local-daemon workflows and host networking when task execution needs `multica repo checkout` or other daemon-backed actions.
- **Hermes support is functional but basic**
  - Hermes can run as a provider and gets isolated task state.
  - Automatic extraction and persistence of fresh Hermes session identifiers is still TODO, so Hermes is not yet advertised as native-resume capable.
- **Execution memory is intentionally minimal**
  - The current memory layer stores compact run outcomes and failure patterns.
  - It is not yet a full long-horizon semantic memory, graph memory, or evaluation-driven self-evolution system.

If you are evaluating the backend runtime work specifically, the current shape is:

- production-worthy foundation for unified multi-provider execution
- usable backend API/CLI support for fan-out task creation
- stronger process launch and policy semantics than the original daemon
- clear upgrade path toward richer swarm orchestration, stronger sandboxing, and deeper memory/evaluation systems

## Development

For contributors working on the Multica codebase, see the [Contributing Guide](CONTRIBUTING.md).

**Prerequisites:** [Node.js](https://nodejs.org/) v20+, [pnpm](https://pnpm.io/) v10.28+, [Go](https://go.dev/) v1.26+, [Docker](https://www.docker.com/)

```bash
pnpm install
cp .env.example .env
make setup
make start
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full development workflow, worktree support, testing, and troubleshooting.

## License

[Apache 2.0](LICENSE)
