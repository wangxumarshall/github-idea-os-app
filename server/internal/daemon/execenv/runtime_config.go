package execenv

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InjectRuntimeConfig writes the meta skill content into the runtime-specific
// config file so the agent discovers its environment through its native mechanism.
//
// For Claude:   writes {workDir}/CLAUDE.md  (skills discovered natively from .claude/skills/)
// For Codex:    writes {workDir}/AGENTS.md  (skills discovered natively via CODEX_HOME)
// For OpenCode: writes {workDir}/AGENTS.md  (skills discovered natively from .config/opencode/skills/)
func InjectRuntimeConfig(workDir, provider string, ctx TaskContextForEnv) error {
	content := buildMetaSkillContent(provider, ctx)

	switch provider {
	case "claude":
		return os.WriteFile(filepath.Join(workDir, "CLAUDE.md"), []byte(content), 0o644)
	case "codex", "opencode":
		return os.WriteFile(filepath.Join(workDir, "AGENTS.md"), []byte(content), 0o644)
	default:
		// Unknown provider — skip config injection, prompt-only mode.
		return nil
	}
}

// buildMetaSkillContent generates the meta skill markdown that teaches the agent
// about the Multica runtime environment and available CLI tools.
func buildMetaSkillContent(provider string, ctx TaskContextForEnv) string {
	var b strings.Builder

	b.WriteString("# Multica Runtime\n\n")
	b.WriteString("You are running inside a task-scoped Multica workdir.\n\n")

	// Inject agent identity instructions before workflow commands.
	if strings.TrimSpace(ctx.AgentInstructions) != "" {
		b.WriteString("## Agent Identity\n\n")
		b.WriteString(ctx.AgentInstructions)
		b.WriteString("\n\n")
	}

	b.WriteString("## Multica CLI\n\n")
	b.WriteString("Use the `multica` CLI for all Multica data and actions. Do not use `curl`, `wget`, or direct HTTP calls against Multica URLs.\n\n")
	b.WriteString("Always use `--output json` on read commands.\n\n")

	b.WriteString("### Read Commands\n")
	b.WriteString("- `multica issue get <id> --output json` — Get full issue details (title, description, status, priority, assignee)\n")
	b.WriteString("- `multica idea get <slug> --output json` — Get full idea details when an issue belongs to an idea\n")
	b.WriteString("- `multica issue list [--status X] [--priority X] [--assignee X] --output json` — List issues in workspace\n")
	b.WriteString("- `multica issue comment list <issue-id> [--limit N] [--offset N] [--since <RFC3339>] --output json` — List comments on an issue (supports pagination; includes id, parent_id for threading)\n")
	b.WriteString("- `multica workspace get --output json` — Get workspace details and context\n")
	b.WriteString("- `multica agent list --output json` — List agents in workspace\n")
	b.WriteString("- `multica issue runs <issue-id> --output json` — List all execution runs for an issue (status, timestamps, errors)\n")
	b.WriteString("- `multica issue run-messages <task-id> [--since <seq>] --output json` — List messages for a specific execution run (supports incremental fetch)\n")
	b.WriteString("- `multica attachment download <id> [-o <dir>]` — Download an attachment file locally by ID\n\n")

	b.WriteString("### Write Commands\n")
	b.WriteString("- `multica issue comment add <issue-id> --content \"...\" [--parent <comment-id>]` — Post a comment (use --parent to reply to a specific comment)\n")
	b.WriteString("- `multica issue status <id> <status>` — Update issue status (todo, in_progress, in_review, done, blocked)\n")
	b.WriteString("- `multica issue update <id> [--title X] [--description X] [--priority X]` — Update issue fields\n\n")

	// Inject available repositories section.
	if len(ctx.Repos) > 0 || ctx.SelectedRepoURL != "" {
		b.WriteString("## Repositories\n\n")
		b.WriteString("Use `multica repo checkout <url>` to create a task-scoped worktree for a repository.\n\n")
		if ctx.SelectedRepoURL != "" {
			b.WriteString("**Preferred repository for this issue**\n\n")
			fmt.Fprintf(&b, "- URL: %s\n", ctx.SelectedRepoURL)
			if ctx.SelectedRepoDescription != "" {
				fmt.Fprintf(&b, "- Description: %s\n", ctx.SelectedRepoDescription)
			}
			b.WriteString("- Constraint: You must only use this repository for code changes in this issue.\n\n")
		}
		if len(ctx.Repos) > 0 {
			b.WriteString("| URL | Description |\n")
			b.WriteString("|-----|-------------|\n")
			for _, repo := range ctx.Repos {
				desc := repo.Description
				if desc == "" {
					desc = "—"
				}
				fmt.Fprintf(&b, "| %s | %s |\n", repo.URL, desc)
			}
			b.WriteString("\n")
		}
		b.WriteString("The checkout command creates a dedicated git worktree and branch for this task.\n\n")
	}

	b.WriteString("## Execution Rules\n\n")

	mode := strings.TrimSpace(ctx.Mode)
	if mode == "plan" {
		step := 1
		b.WriteString("You are in plan mode. Produce a concrete implementation plan without making code changes.\n\n")
		b.WriteString("Non-negotiable:\n")
		b.WriteString("- Do NOT edit files\n")
		b.WriteString("- Do NOT commit or push code\n")
		b.WriteString("- Do NOT create or request a PR\n")
		b.WriteString("- Do NOT change issue status just because the plan is done\n")
		b.WriteString("- Do NOT mark the issue blocked unless you hit a real blocker that prevents planning\n\n")
		fmt.Fprintf(&b, "%d. Run `multica issue get %s --output json`\n", step, ctx.IssueID)
		step++
		if ctx.IdeaSlug != "" {
			fmt.Fprintf(&b, "%d. Run `multica idea get %s --output json` and treat the full idea markdown as the source of truth\n", step, ctx.IdeaSlug)
			step++
		}
		fmt.Fprintf(&b, "%d. Run `multica issue comment list %s --output json`\n", step, ctx.IssueID)
		step++
		if ctx.TriggerCommentID != "" {
			fmt.Fprintf(&b, "%d. Revise the previous plan using the triggering comment (ID: `%s`) as feedback\n", step, ctx.TriggerCommentID)
			step++
			fmt.Fprintf(&b, "%d. Stay in planning mode instead of switching into implementation\n", step)
		} else {
			fmt.Fprintf(&b, "%d. Identify scope, affected areas, sequencing, risks, and acceptance checks\n", step)
		}
		step++
		fmt.Fprintf(&b, "%d. Inspect the repository only as much as needed to ground the plan\n", step)
		step++
		fmt.Fprintf(&b, "%d. Return a review-ready plan with goal summary, implementation steps, risks or open questions, and validation approach\n\n", step)
	} else if ctx.TriggerCommentID != "" {
		step := 1
		b.WriteString("This build task was triggered by a comment. Address the requested follow-up and reply with the result when useful.\n\n")
		fmt.Fprintf(&b, "%d. Run `multica issue get %s --output json`\n", step, ctx.IssueID)
		step++
		if ctx.IdeaSlug != "" {
			fmt.Fprintf(&b, "%d. Run `multica idea get %s --output json`\n", step, ctx.IdeaSlug)
			step++
		}
		fmt.Fprintf(&b, "%d. Run `multica issue comment list %s --output json`\n", step, ctx.IssueID)
		step++
		fmt.Fprintf(&b, "%d. Address the triggering comment (ID: `%s`)\n", step, ctx.TriggerCommentID)
		step++
		fmt.Fprintf(&b, "%d. Run `multica issue status %s in_progress`\n", step, ctx.IssueID)
		step++
		fmt.Fprintf(&b, "%d. If code changes are needed, check out the allowed repository if required, then implement, commit, and push the branch\n", step)
		step++
		fmt.Fprintf(&b, "%d. Do not treat delayed PR visibility as a blocker; server-side PR automation may still be running\n", step)
		step++
		fmt.Fprintf(&b, "%d. When delivery artifacts are ready, run `multica issue status %s in_review`\n", step, ctx.IssueID)
		step++
		fmt.Fprintf(&b, "%d. Reply in-thread only if useful: `multica issue comment add %s --parent %s --content \"...\"`\n", step, ctx.IssueID, ctx.TriggerCommentID)
		step++
		fmt.Fprintf(&b, "%d. Only run `multica issue status %s blocked` for real implementation blockers\n\n", step, ctx.IssueID)
	} else {
		step := 1
		b.WriteString("You are in build mode. Implement the confirmed plan and move the issue toward review.\n\n")
		fmt.Fprintf(&b, "%d. Run `multica issue get %s --output json`\n", step, ctx.IssueID)
		step++
		if ctx.IdeaSlug != "" {
			fmt.Fprintf(&b, "%d. Run `multica idea get %s --output json` and treat the full idea markdown as the source of truth\n", step, ctx.IdeaSlug)
			step++
		}
		fmt.Fprintf(&b, "%d. Run `multica issue status %s in_progress`\n", step, ctx.IssueID)
		step++
		fmt.Fprintf(&b, "%d. Read comments for any final human instructions before coding\n", step)
		step++
		fmt.Fprintf(&b, "%d. If the task requires code changes:\n", step)
		if len(ctx.Repos) > 0 || ctx.SelectedRepoURL != "" {
			if ctx.SelectedRepoURL != "" {
				fmt.Fprintf(&b, "   a. Run `multica repo checkout %s`\n", ctx.SelectedRepoURL)
				b.WriteString("   a1. Do not check out any other repository for this issue\n")
			} else {
				b.WriteString("   a. Run `multica repo checkout <url>` to check out the appropriate repository\n")
			}
			b.WriteString("   b. `cd` into the checked-out directory\n")
			b.WriteString("   c. Implement the changes and commit\n")
		} else {
			b.WriteString("   a. Create a new branch\n")
			b.WriteString("   b. Implement the changes and commit\n")
		}
		b.WriteString("   c. Push the branch to the remote\n")
		b.WriteString("   d. Push a review-ready branch; the server will attempt PR creation automatically after the issue moves to review\n")
		fmt.Fprintf(&b, "   e. If you already know the PR URL, post it as a comment: `multica issue comment add %s --content \"PR: <url>\"`\n", ctx.IssueID)
		b.WriteString("   f. If no PR is visible yet, do not treat that alone as a blocker\n")
		step++
		fmt.Fprintf(&b, "%d. If the task does not require code (e.g. research, documentation), post your findings as a comment\n", step)
		step++
		fmt.Fprintf(&b, "%d. When delivery artifacts are ready, run `multica issue status %s in_review`\n", step, ctx.IssueID)
		step++
		fmt.Fprintf(&b, "%d. Only run `multica issue status %s blocked` for real implementation blockers\n\n", step, ctx.IssueID)
	}

	if len(ctx.AgentSkills) > 0 {
		b.WriteString("## Installed Skills\n\n")
		switch provider {
		case "claude", "codex", "opencode":
			b.WriteString("These skills are installed natively for this provider:\n\n")
		default:
			b.WriteString("Detailed skill instructions are in `.agent_context/skills/`. Each subdirectory contains a `SKILL.md`.\n\n")
		}
		for _, skill := range ctx.AgentSkills {
			fmt.Fprintf(&b, "- **%s**\n", skill.Name)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Mentions\n\n")
	b.WriteString("When referencing issues or people in comments, use the mention format so they render as interactive links:\n\n")
	b.WriteString("- **Issue**: `[MUL-123](mention://issue/<issue-id>)` — renders as a clickable link to the issue\n")
	b.WriteString("- **Member**: `[@Name](mention://member/<user-id>)` — renders as a styled mention and sends a notification\n")
	b.WriteString("- **Agent**: `[@Name](mention://agent/<agent-id>)` — renders as a styled mention\n\n")
	b.WriteString("Use `multica issue list --output json` to look up issue IDs, and `multica workspace members --output json` for member IDs.\n\n")

	b.WriteString("## Attachments\n\n")
	b.WriteString("Issues and comments may include file attachments (images, documents, etc.).\n")
	b.WriteString("Use the download command to fetch attachment files locally:\n\n")
	b.WriteString("```\nmultica attachment download <attachment-id>\n```\n\n")
	b.WriteString("This downloads the file to the current directory and prints the local path. Use `-o <dir>` to save elsewhere.\n")
	b.WriteString("After downloading, you can read the file directly (e.g. view an image, read a document).\n\n")

	b.WriteString("## Missing Capabilities\n\n")
	b.WriteString("If the `multica` CLI cannot perform a required platform action, do not work around it. Post a comment mentioning the workspace owner instead.\n\n")

	b.WriteString("## Delivery Style\n\n")
	b.WriteString("Keep comments concise and natural — state the outcome, not the process.\n")
	b.WriteString("Good: \"Delivery ready. PR: https://...\"\n")
	b.WriteString("Good: \"Delivery ready. Branch pushed; PR automation should pick this up. Compare: https://...\"\n")
	b.WriteString("Bad: \"1. Read the issue 2. Found the bug in auth.go 3. Created branch 4. ...\"\n")

	return b.String()
}
