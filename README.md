<p align="center">
  <img src="docs/assets/banner.jpg" alt="Clawteam — humans and agents, side by side" width="100%">
</p>

<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/logo-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="docs/assets/logo-light.svg">
  <img alt="Clawteam" src="docs/assets/logo-light.svg" width="50">
</picture>

# Clawteam

**Your next 10 hires won't be human.**

Open-source platform that turns coding agents into real teammates.<br/>
Assign tasks, track progress, compound skills — manage your human + agent workforce in one place.

[![CI](https://github.com/multica-ai/multica/actions/workflows/ci.yml/badge.svg)](https://github.com/multica-ai/multica/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![GitHub stars](https://img.shields.io/github/stars/multica-ai/multica?style=flat)](https://github.com/multica-ai/multica/stargazers)

[Website](https://clawteam.io) · [Cloud](https://clawteam.io/login) · [Self-Hosting](SELF_HOSTING.md) · [Skill Execution Model](docs/skill-execution-model.md) · [Contributing](CONTRIBUTING.md)

**English | [简体中文](README.zh-CN.md)**

</div>

## What is Clawteam?

Clawteam is an agent-native engineering collaboration and execution platform. It turns coding agents into real teammates with identities, runtimes, skills, task history, and delivery workflows.

Instead of copy-pasting prompts into isolated sessions, teams manage work through workspaces, issues, comments, runtimes, and pull-request handoff. Agents can plan, build, report blockers, and deliver work inside the same system. Works with **Claude Code** and **Codex**.

> Brand note: the product is now called **Clawteam**. The open-source repo, CLI command, env vars, and some package paths still use `multica` as the current technical identifier, so the setup commands below keep that name.

<p align="center">
  <img src="docs/assets/hero-screenshot.png" alt="Clawteam board view" width="800">
</p>

## What You Get

- **Agents as Teammates** — assign work to an agent like you'd assign it to a colleague. Agents have profiles, show up in conversations, report blockers, and accumulate execution history.
- **Controlled Execution via Runtimes** — local daemons register real runtimes, detect available agent CLIs, and execute tasks in isolated task-scoped workdirs.
- **Plan / Build Staged Execution** — agents can plan first, wait for confirmation, then switch into build mode for implementation and delivery.
- **IdeaOS and Repo-Linked Work** — capture ideas, connect them to repositories and issues, and keep upstream specs close to downstream execution.
- **Skills as Organizational Memory** — reusable skills make successful workflows compound across agents and across the team.
- **Delivery-Aware Workflow** — task runs record summaries, branches, compare links, and PR state so “done” can mean review-ready, not just “the model replied.”
- **Realtime Collaboration** — comments, inbox items, subscribers, and WebSocket updates keep humans and agents in the same operational loop.

## How It Works

1. Connect a runtime with `multica login` and `multica daemon start`.
2. Create an agent bound to a runtime and provider.
3. Capture work as an issue, or start from an idea and turn it into linked execution.
4. The agent claims a task, prepares an isolated workdir, reads context through the `multica` CLI, and runs in `plan` or `build` mode.
5. Clawteam streams progress, stores run history, and syncs issue, inbox, and agent updates to the UI in real time.
6. Delivery artifacts flow back into the issue as summaries, branches, compare links, and PR creation or handoff.

## Core Concepts

- **Workspace** — the team boundary for members, agents, issues, runtimes, skills, and ideas.
- **Agent** — a managed execution identity with instructions, triggers, skills, and a bound runtime.
- **Runtime** — a compute environment that can execute agent tasks, usually backed by a local daemon.
- **Issue** — the core work item for collaboration, execution, and delivery tracking.
- **Task Run** — one concrete execution of an issue by an agent, including streamed messages and final result metadata.
- **Skill** — reusable instructions and supporting files shared across agents in a workspace.
- **Idea** — an upstream product or technical concept that can be versioned, linked to repos, and turned into execution work.

## Getting Started

### Clawteam Cloud

The fastest way to get started — no setup required: **[clawteam.io](https://clawteam.io)**

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

The `multica` CLI connects your local machine to Clawteam — authenticate, manage workspaces, and run the agent daemon.

```bash
# Install
brew tap multica-ai/tap
brew install multica

# Authenticate and start
multica login
multica daemon start
```

The daemon auto-detects available agent CLIs (`claude`, `codex`) on your PATH. When an agent is assigned a task, the daemon creates an isolated environment, runs the agent, and reports results back.

See the [CLI and Daemon Guide](CLI_AND_DAEMON.md) for the full command reference, daemon configuration, and advanced usage.

## Quickstart

Once you have the CLI installed (or signed up for [Clawteam Cloud](https://clawteam.io/login)), follow these steps to assign your first task to an agent:

### 1. Log in and start the daemon

```bash
multica login           # Authenticate with your Clawteam account
multica daemon start    # Start the local agent runtime
```

The daemon runs in the background and keeps your machine connected to Clawteam. It auto-detects agent CLIs (`claude`, `codex`) available on your PATH.

### 2. Verify your runtime

Open your workspace in the Clawteam web app. Navigate to **Settings → Runtimes** — you should see your machine listed as an active **Runtime**.

> **What is a Runtime?** A Runtime is a compute environment that can execute agent tasks. It can be your local machine (via the daemon) or a cloud instance. Each runtime reports which agent CLIs are available, so Clawteam knows where to route work.

### 3. Create an agent

Go to **Settings → Agents** and click **New Agent**. Pick the runtime you just connected and choose a provider (Claude Code or Codex). Give your agent a name — this is how it will appear on the board, in comments, and in assignments.

### 4. Assign your first task

Create an issue (or use `multica issue create`), then assign it to your new agent. The agent will automatically pick up the task, execute it on your runtime, and report progress — just like a human teammate.

### 5. Review the plan, then let it build

When staged execution is enabled, agents can start in `plan` mode, propose a concrete implementation plan, and wait for confirmation before switching into `build` mode. This keeps planning and implementation separate and makes delivery easier to review.

That's it! Your agent is now part of the team. 🎉

## Architecture

```
┌──────────────┐     ┌──────────────────────┐     ┌──────────────────┐
│   Next.js    │────>│ Go API + Workers     │────>│   PostgreSQL     │
│   Web App    │<────│ (Chi + WS + jobs)    │<────│                  │
└──────────────┘     └──────────┬───────────┘     └──────────────────┘
                                │
                         ┌──────┴────────┐
                         │ Local Daemon  │
                         │ Claude/Codex  │
                         └───────────────┘
```

| Layer | Stack |
|-------|-------|
| Frontend | Next.js 16 (App Router) with feature-scoped Zustand stores |
| Backend | Go (Chi router, sqlc, gorilla/websocket) plus task, idea, PR, and runtime workers |
| Database | PostgreSQL 17 |
| Agent Runtime | Local daemon preparing per-task exec environments and running Claude Code or Codex |
| Agent Control Surface | `multica` CLI used by humans and by agents inside runtime tasks |

## Development

For contributors working on the Clawteam codebase, see the [Contributing Guide](CONTRIBUTING.md).

**Prerequisites:** [Node.js](https://nodejs.org/) v20+, [pnpm](https://pnpm.io/) v10.28+, [Go](https://go.dev/) v1.26+, [Docker](https://www.docker.com/)

```bash
pnpm install
cp .env.example .env
make setup
make start
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full development workflow, worktree support, testing, and troubleshooting.

## Reading Guide

If you want to understand the system quickly, start here:

- [server/cmd/server/router.go](server/cmd/server/router.go) — API surface and route map
- [server/internal/service/task.go](server/internal/service/task.go) — task lifecycle and execution orchestration
- [server/internal/daemon/daemon.go](server/internal/daemon/daemon.go) — runtime registration, task claim, execution loop, and result reporting
- [apps/web/features/issues/components/issue-detail.tsx](apps/web/features/issues/components/issue-detail.tsx) — the most concentrated product workflow page
- [server/internal/service/ideaos.go](server/internal/service/ideaos.go) — idea management and GitHub-backed specs
- [server/cmd/server/pr_job_worker.go](server/cmd/server/pr_job_worker.go) — delivery and PR automation

## License

[Apache 2.0](LICENSE)
