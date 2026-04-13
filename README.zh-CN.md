<p align="center">
  <img src="docs/assets/banner.jpg" alt="Multica — 人类与 AI，并肩前行" width="100%">
</p>

<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/logo-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="docs/assets/logo-light.svg">
  <img alt="Multica" src="docs/assets/logo-light.svg" width="50">
</picture>

# Multica

**面向现有 Agent CLI 的本地优先自动产品工厂。**

开源系统，围绕 `idea -> spec -> repo -> root task -> fanout -> implement/verify -> delivery` 持续生产产品。<br/>
把记忆、policy、repo/worktree 控制和 swarm 执行留在本地 runtime；把云端限制为轻控制平面。

[![CI](https://github.com/multica-ai/multica/actions/workflows/ci.yml/badge.svg)](https://github.com/multica-ai/multica/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![GitHub stars](https://img.shields.io/github/stars/multica-ai/multica?style=flat)](https://github.com/multica-ai/multica/stargazers)

[官网](https://multica.ai) · [云服务](https://multica.ai/app) · [自部署指南](SELF_HOSTING.md) · [参与贡献](CONTRIBUTING.md)

**[English](README.md) | 简体中文**

</div>

## Multica 是什么？

Multica 不是一个挂了几个 bot 的 AI Jira。它是一个本地优先的自动产品工厂，负责完整主循环 `idea -> spec -> repo -> root task -> fanout -> implement/verify -> delivery`。Issue、看板和 Agent 身份只是这条生产链的操作界面，不是产品本体。

本地 daemon/runtime 才是智能核心。任务级 workdir、repo/worktree 控制、统一记忆、policy 执行、swarm 协调、session 复用，以及未来的 agent 自演进，都应该放在这里。云端则有意保持轻量，只负责登录、工作区同步、排队、可视化和远程入口。

**Claude Code**、**Codex**、**OpenCode**、**Trae CLI** 和 **Hermes Agent** 在这套系统里是执行引擎。Multica 位于它们之上，提供统一的 task、memory、policy、repo checkout、worktree 隔离、swarm 原语和观测层，而不是替代它们原生能力。

<p align="center">
  <img src="docs/assets/hero-screenshot.png" alt="Multica 看板视图" width="800">
</p>

## V1 方向

- **自动产品工厂** — 主循环应该是 `idea/spec -> repo -> task graph -> execution -> delivery`，而不只是 issue 管理。
- **本地优先 + 云协同** — 群体智能、统一记忆、swarm 协调、policy，以及未来的自演进，应尽量留在本地 runtime；云端只承担登录、工作区同步、排队、可视化和远程入口。
- **先适配现有 CLI** — 把 Claude Code、Codex、OpenCode 等工具视为执行底座，而不是 Multica 原生 agent 上的可选集成。

## 功能特性

- **端到端产品主循环** — IdeaOS 将版本化 idea 文档、项目仓库、根任务、fanout 流程和交付流程串起来，让产品意图在一套系统里持续推进。
- **本地运行时内核** — 你的本地 daemon 是执行核心，不是一个简单转发器。
- **现有 CLI 作为执行引擎** — Claude Code、Codex、OpenCode、Trae CLI 和 Hermes Agent 运行在 Multica 编排层之下，负责实际执行。
- **统一 CLI 编排层** — Multica 在这些 CLI 之上补齐统一的 task、event、memory、policy、repo/worktree 和 capability 模型。
- **透明记忆层** — 当前记忆系统采用 [memsearch](https://github.com/zilliztech/memsearch)，让 Agent 记忆以可审查、可直接编辑的 Markdown 形式存在，并兼容所有已支持的 CLI。
- **任务级执行环境** — 每个任务都有独立 workdir、policy 文件、运行时原生指令注入，以及受控的仓库 checkout 流程。
- **执行记忆** — 成功和失败运行会写回紧凑执行记忆，并在后续同 issue 任务中重新注入。
- **轻量 swarm 原语** — 后端已经建模 parent/child task 与 `swarm_role`，在完整 swarm 产品化前先具备可拆解能力。
- **更强的沙箱基础** — daemon 可在宿主机或 Docker 中运行 agent，并通过 policy 合约和进程级 driver 抽象约束执行。
- **轻量云控制平面** — 云端负责身份、工作区、队列、同步和观测，复杂执行保持本地优先。
- **统一运行时视图** — 一个控制台管理本地 daemon 与云端 runtime，自动检测可用 CLI，实时监控。
- **多工作区** — 按团队组织工作，工作区级别隔离。每个工作区有独立的 Agent、Issue 和设置。

## 记忆系统路线图

- **起步 / 验证阶段：`memsearch`** — 当前采用 [memsearch](https://github.com/zilliztech/memsearch) 作为记忆层，因为它几乎零成本接入、完美兼容当前支持的所有 Agent CLI，并且能把记忆保持为人类可审查、可直接编辑的 Markdown。
- **生产 / 自演进阶段：`memsearch + OpenViking`** — 保留 `memsearch` 作为透明 Markdown 层，再向上扩展 [OpenViking](https://github.com/volcengine/OpenViking)，提供 `viking://` 抽象、`L0/L1/L2` 记忆分层以及自动提取能力，支撑真正的群体智能和 Agent 自我进化。
- **终局目标：透明 + 统一 + 自演进三合一** — 把 `memsearch` 的 Markdown 目录映射到 `viking://user/memories/`，让 Multica 在同一套系统里同时获得透明可编辑记忆、统一地址空间和自演进记忆栈。

## 快速开始

### Multica 云服务

最快的上手方式，无需任何配置：**[multica.ai](https://multica.ai)**

### Docker 自部署

```bash
git clone https://github.com/multica-ai/multica.git
cd multica
cp .env.example .env
# 编辑 .env — 至少修改 JWT_SECRET

docker compose up -d                              # 启动 PostgreSQL
cd server && go run ./cmd/migrate up && cd ..     # 运行数据库迁移
make start                                         # 启动应用
```

完整部署文档请参阅 [自部署指南](SELF_HOSTING.md)。

## CLI

`multica` CLI 将你的本地机器连接到 Multica — 用于认证、管理工作区、查看 idea/issue，以及运行 Agent daemon。

```bash
# 安装
brew tap multica-ai/tap
brew install multica

# 认证并启动
multica login
multica daemon start
```

daemon 会自动检测 PATH 中可用的 Agent CLI（`claude`、`codex`、`opencode`、`trae-cli`、`hermes`）。当有工作被分配时，daemon 会创建隔离环境、注入 policy/context/skills、运行选定 CLI，并将结果回传。

完整命令参考请参阅 [CLI 与 Daemon 指南](CLI_AND_DAEMON.md)。

## 快速上手

安装好 CLI（或注册 [Multica 云服务](https://multica.ai)）后，按以下步骤启动你的第一次本地优先产品执行：

### 1. 登录并启动 daemon

```bash
multica login           # 使用你的 Multica 账号认证
multica daemon start    # 启动本地 Agent 运行时
```

daemon 在后台运行，保持你的机器与 Multica 的连接。它会自动检测 PATH 中可用的 Agent CLI（`claude`、`codex`、`opencode`、`trae-cli`、`hermes`）。

### 2. 确认运行时已连接

在 Multica Web 端打开你的工作区，进入 **设置 → 运行时（Runtimes）**，你应该能看到你的机器已作为一个活跃的 **Runtime** 出现在列表中。

> **什么是 Runtime（运行时）？** Runtime 是 Multica 准备记忆、policy、repo/worktree 上下文并执行任务的边界。它可以是你的本地机器（通过 daemon 连接），也可以是云端环境，但 V1 明确以本地优先：本地 runtime 才是智能内核所在的位置。

### 3. 创建 Agent

进入 **设置 → Agents**，点击 **新建 Agent**。选择你刚连接的 Runtime，选择该 Runtime 上可用的 Provider，并为 Agent 起个名字——它将以这个名字出现在看板、评论和任务分配中。

### 4. 从 Issue 或 Idea 开始

在看板上创建一个 Issue（或通过 `multica issue create` 命令创建），或者在 IdeaOS 中创建一个 idea 作为项目真相源。先驱动根任务，再在需要时 fanout 子任务。本地 runtime 会接手这些工作，附带 repo 和记忆上下文执行，并将进度回传到 Multica。

至此，你已经拥有一条连接到 Multica 的本地优先执行闭环。

## 架构

Multica 在架构边界上是明确的本地优先：

- **本地运行时** — 记忆、policy、repo/worktree 控制、swarm 协调和执行的智能内核。
- **云端控制平面** — 身份、工作区同步、排队、可视化和远程入口。
- **外部 Agent CLI** — 由 Multica 编排而不是被 Multica 替代的执行引擎。

```
┌─────────────────────────┐      ┌────────────────────────────────────┐
│      本地运行时         │<---->│            云端控制平面            │
│ daemon + workdirs       │      │ 前端 + API + PostgreSQL            │
│ memory/policy/swarm     │      │ auth/sync/queue/visibility         │
└─────────────┬───────────┘      └────────────────────────────────────┘
              │
┌─────────────┴───────────────────────────────────────────────────────┐
│ Claude Code / Codex / OpenCode / Trae CLI / Hermes Agent           │
│ 在 Multica 编排层之下运行的执行引擎                                │
└─────────────────────────────────────────────────────────────────────┘
```

| 层级 | 职责 | 技术栈 |
|------|------|--------|
| 本地运行时 | 任务执行、记忆、policy、repo/worktree 控制与 swarm 协调的智能内核 | 本地 daemon |
| 执行引擎 | 实际完成任务工作的现有 Agent CLI | Claude Code、Codex、OpenCode、Trae CLI、Hermes Agent |
| 前端 | 云端 UI 与操作台 | Next.js 16 (App Router) |
| 后端 | 工作区、队列、实时事件与运行时协同的轻控制平面 | Go (Chi router, sqlc, gorilla/websocket) |
| 数据库 | 持久化状态、运行时元数据与执行记忆 | PostgreSQL 17 with pgvector |

## 运行时内核现状

这个仓库已经更接近“本地优先编排内核”，而不是传统意义上的任务管理应用。Web 端保持相对轻量；真正的执行逻辑主要位于 `server/` 和本地 daemon 中。

## 开发

参与 Multica 代码贡献，请参阅 [贡献指南](CONTRIBUTING.md)。

**环境要求：** [Node.js](https://nodejs.org/) v20+, [pnpm](https://pnpm.io/) v10.28+, [Go](https://go.dev/) v1.26+, [Docker](https://www.docker.com/)

```bash
pnpm install
cp .env.example .env
make setup
make start
```

完整的开发流程、worktree 支持、测试和问题排查请参阅 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 开源协议

[Apache 2.0](LICENSE)
