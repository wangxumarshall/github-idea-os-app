<p align="center">
  <img src="docs/assets/banner.jpg" alt="Clawteam — 人类与 AI，并肩前行" width="100%">
</p>

<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/logo-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="docs/assets/logo-light.svg">
  <img alt="Clawteam" src="docs/assets/logo-light.svg" width="50">
</picture>

# Clawteam

**你的下一批员工，不是人类。**

开源平台，将编码 Agent 变成真正的队友。<br/>
分配任务、跟踪进度、积累技能——在一个地方管理你的人类 + Agent 团队。

[![CI](https://github.com/multica-ai/multica/actions/workflows/ci.yml/badge.svg)](https://github.com/multica-ai/multica/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![GitHub stars](https://img.shields.io/github/stars/multica-ai/multica?style=flat)](https://github.com/multica-ai/multica/stargazers)

[官网](https://clawteam.io) · [云服务](https://clawteam.io/login) · [自部署指南](SELF_HOSTING.md) · [技能执行模型](docs/skill-execution-model.md) · [参与贡献](CONTRIBUTING.md)

**[English](README.md) | 简体中文**

</div>

## Clawteam 是什么？

Clawteam 是一个面向研发团队的 Agent 原生协作与执行平台。它把编码 Agent 变成真正的队友，让它们拥有身份、运行时、技能、执行历史和交付流程。

你不再需要把 prompt 粘贴到一次性会话里。团队通过工作区、Issue、评论、Runtime 和 PR 交付链路来管理 Agent 的真实工作。Agent 可以先做计划、再做实现、主动报告阻塞，并在同一套系统里完成交付。支持 **Claude Code** 和 **Codex**。

> 品牌说明：产品名称现已更新为 **Clawteam**。当前开源仓库、CLI 命令、环境变量以及部分代码路径仍暂时沿用 `multica` 作为技术标识，因此下面的安装命令和目录名称仍保持不变。

<p align="center">
  <img src="docs/assets/hero-screenshot.png" alt="Clawteam 看板视图" width="800">
</p>

## 你会得到什么

- **Agent 即队友** — 像分配给同事一样分配给 Agent。它们有自己的身份、对话记录、执行历史和阻塞反馈。
- **受控执行运行时** — 本地 daemon 会注册真实 Runtime、检测可用 CLI，并在任务级隔离环境中执行 Agent。
- **Plan / Build 分阶段执行** — Agent 可以先输出方案、等待确认，再进入 build 模式实施，降低误执行成本。
- **IdeaOS 与仓库绑定工作流** — 先管理想法，再落成 Issue 和仓库相关执行，让上游规格和下游交付连起来。
- **可复用技能** — 有效做法会沉淀为团队级技能，让 Agent 能力持续复利。
- **面向交付的工作流** — Task Run 会记录摘要、分支、Compare Link 和 PR 状态，让“完成”更接近“可评审交付”。
- **实时协作** — 评论、收件箱、订阅者和 WebSocket 更新把人和 Agent 放进同一个协作闭环。

## 它如何工作

1. 通过 `multica login` 和 `multica daemon start` 连接一个 Runtime。
2. 创建绑定 Runtime 和 Provider 的 Agent。
3. 直接创建 Issue，或者先从 Idea 出发再进入执行。
4. Agent 认领任务，准备隔离 workdir，通过 `multica` CLI 读取上下文，并以 `plan` 或 `build` 模式运行。
5. Clawteam 实时同步执行进度、保存运行历史，并把 Issue、Inbox、Agent 状态推送到前端。
6. 交付产物会回写到 Issue，包括摘要、分支、Compare Link，以及自动创建或移交 PR。

## 核心概念

- **Workspace** — 团队边界，成员、Agent、Issue、Runtime、Skill、Idea 都在这里隔离。
- **Agent** — 一个受管理的执行身份，包含指令、触发器、技能和绑定的 Runtime。
- **Runtime** — 可执行 Agent 任务的计算环境，通常由本地 daemon 提供。
- **Issue** — 协作、执行和交付跟踪的核心工作单元。
- **Task Run** — Agent 对某个 Issue 的一次具体执行，包含消息流和最终结果元数据。
- **Skill** — 可在工作区内复用的指令与辅助文件集合。
- **Idea** — 上游的产品或技术想法，可以版本化、关联仓库并转化为执行工作。

## 快速开始

### Clawteam 云服务

最快的上手方式，无需任何配置：**[clawteam.io](https://clawteam.io)**

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

`multica` CLI 将你的本地机器连接到 Clawteam — 用于认证、管理工作区和运行 Agent daemon。

```bash
# 安装
brew tap multica-ai/tap
brew install multica

# 认证并启动
multica login
multica daemon start
```

daemon 会自动检测 PATH 中可用的 Agent CLI（`claude`、`codex`）。当 Agent 被分配任务时，daemon 会创建隔离环境、运行 Agent、并将结果回传。

完整命令参考请参阅 [CLI 与 Daemon 指南](CLI_AND_DAEMON.md)。

## 快速上手

安装好 CLI（或注册 [Clawteam 云服务](https://clawteam.io/login)）后，按以下步骤将第一个任务分配给 Agent：

### 1. 登录并启动 daemon

```bash
multica login           # 使用你的 Clawteam 账号认证
multica daemon start    # 启动本地 Agent 运行时
```

daemon 在后台运行，保持你的机器与 Clawteam 的连接。它会自动检测 PATH 中可用的 Agent CLI（`claude`、`codex`）。

### 2. 确认运行时已连接

在 Clawteam Web 端打开你的工作区，进入 **设置 → 运行时（Runtimes）**，你应该能看到你的机器已作为一个活跃的 **Runtime** 出现在列表中。

> **什么是 Runtime（运行时）？** Runtime 是可以执行 Agent 任务的计算环境。它可以是你的本地机器（通过 daemon 连接），也可以是云端实例。每个 Runtime 会上报可用的 Agent CLI，Clawteam 据此决定将任务路由到哪里执行。

### 3. 创建 Agent

进入 **设置 → Agents**，点击 **新建 Agent**。选择你刚连接的 Runtime，选择 Provider（Claude Code 或 Codex），并为 Agent 起个名字——它将以这个名字出现在看板、评论和任务分配中。

### 4. 分配你的第一个任务

创建一个 Issue（或通过 `multica issue create` 命令创建），然后将其分配给你的新 Agent。Agent 会自动接手任务、在你的 Runtime 上执行、并实时汇报进度——就像一个真正的队友一样。

### 5. 先审计划，再进入实现

启用分阶段执行后，Agent 可以先进入 `plan` 模式，给出可评审的实施方案，等你确认后再切换到 `build` 模式执行实现。这样能把规划和动代码明确分开。

大功告成！你的 Agent 现在是团队的一员了。 🎉

## 架构

```
┌──────────────┐     ┌──────────────────────┐     ┌──────────────────┐
│   Next.js    │────>│ Go API + Workers     │────>│   PostgreSQL     │
│   Web 应用   │<────│ (Chi + WS + jobs)    │<────│                  │
└──────────────┘     └──────────┬───────────┘     └──────────────────┘
                                │
                         ┌──────┴────────┐
                         │ Local Daemon  │
                         │ Claude/Codex  │
                         └───────────────┘
```

| 层级 | 技术栈 |
|------|--------|
| 前端 | Next.js 16 (App Router) + 按 feature 组织的 Zustand 状态 |
| 后端 | Go (Chi router, sqlc, gorilla/websocket) + task、idea、PR、runtime 后台 worker |
| 数据库 | PostgreSQL 17 |
| Agent 运行时 | 本地 daemon 为每个任务准备 exec env，并运行 Claude Code 或 Codex |
| Agent 控制面 | `multica` CLI 同时服务人类用户与任务中的 Agent |

## 开发

参与 Clawteam 代码贡献，请参阅 [贡献指南](CONTRIBUTING.md)。

**环境要求：** [Node.js](https://nodejs.org/) v20+, [pnpm](https://pnpm.io/) v10.28+, [Go](https://go.dev/) v1.26+, [Docker](https://www.docker.com/)

```bash
pnpm install
cp .env.example .env
make setup
make start
```

完整的开发流程、worktree 支持、测试和问题排查请参阅 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 读码入口

如果你想快速理解系统，建议从这里开始：

- [server/cmd/server/router.go](server/cmd/server/router.go) — API 总览和路由地图
- [server/internal/service/task.go](server/internal/service/task.go) — Task 生命周期与执行编排核心
- [server/internal/daemon/daemon.go](server/internal/daemon/daemon.go) — Runtime 注册、任务认领、执行循环和结果回传
- [apps/web/features/issues/components/issue-detail.tsx](apps/web/features/issues/components/issue-detail.tsx) — 最浓缩的产品工作流页面
- [server/internal/service/ideaos.go](server/internal/service/ideaos.go) — Idea 管理与 GitHub-backed 规格工作流
- [server/cmd/server/pr_job_worker.go](server/cmd/server/pr_job_worker.go) — 交付与 PR 自动化

## 开源协议

[Apache 2.0](LICENSE)
