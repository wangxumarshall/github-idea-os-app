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

	b.WriteString("# Multica Agent Runtime\n\n")
	b.WriteString("You are a coding agent in the Multica platform. Use the `multica` CLI to interact with the platform.\n\n")

	// Inject agent identity instructions before workflow commands.
	if ctx.AgentInstructions != "" {
		b.WriteString("## Agent Identity\n\n")
		b.WriteString(ctx.AgentInstructions)
		b.WriteString("\n\n")
	}

	b.WriteString("## Available Commands\n\n")
	b.WriteString("**Always use `--output json` for all read commands** to get structured data with full IDs.\n\n")
	b.WriteString("### Read\n")
	b.WriteString("- `multica issue get <id> --output json` — Get full issue details (title, description, status, priority, assignee)\n")
	b.WriteString("- `multica idea get <slug> --output json` — Get full idea details when an issue belongs to an idea\n")
	b.WriteString("- `multica issue list [--status X] [--priority X] [--assignee X] --output json` — List issues in workspace\n")
	b.WriteString("- `multica issue comment list <issue-id> [--limit N] [--offset N] [--since <RFC3339>] --output json` — List comments on an issue (supports pagination; includes id, parent_id for threading)\n")
	b.WriteString("- `multica workspace get --output json` — Get workspace details and context\n")
	b.WriteString("- `multica agent list --output json` — List agents in workspace\n")
	b.WriteString("- `multica issue runs <issue-id> --output json` — List all execution runs for an issue (status, timestamps, errors)\n")
	b.WriteString("- `multica issue run-messages <task-id> [--since <seq>] --output json` — List messages for a specific execution run (supports incremental fetch)\n")
	b.WriteString("- `multica attachment download <id> [-o <dir>]` — Download an attachment file locally by ID\n\n")

	b.WriteString("### Write\n")
	b.WriteString("- `multica issue comment add <issue-id> --content \"...\" [--parent <comment-id>]` — Post a comment (use --parent to reply to a specific comment)\n")
	b.WriteString("- `multica issue status <id> <status>` — Update issue status (todo, in_progress, in_review, done, blocked)\n")
	b.WriteString("- `multica issue update <id> [--title X] [--description X] [--priority X]` — Update issue fields\n\n")

	// Inject available repositories section.
	if len(ctx.Repos) > 0 || ctx.SelectedRepoURL != "" {
		b.WriteString("## Repositories\n\n")
		b.WriteString("The following code repositories are available for this task.\n")
		b.WriteString("Use `multica repo checkout <url>` to check out a repository into your working directory.\n\n")
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
		b.WriteString("The checkout command creates a git worktree with a dedicated branch. You can check out one or more repos as needed.\n\n")
	}

	b.WriteString("### Workflow\n\n")

	mode := strings.TrimSpace(ctx.Mode)
	if mode == "plan" {
		b.WriteString("You are in native planning mode. Your job is to understand the issue deeply and produce a concrete implementation plan.\n\n")
		b.WriteString("Hard constraints:\n")
		b.WriteString("- Do NOT edit files\n")
		b.WriteString("- Do NOT commit or push code\n")
		b.WriteString("- Do NOT create or request a PR\n")
		b.WriteString("- Do NOT change issue status just because planning is complete\n")
		b.WriteString("- Do NOT mark the issue blocked unless you hit a real blocker that prevents planning\n\n")
		fmt.Fprintf(&b, "1. Run `multica issue get %s --output json` to understand the issue\n", ctx.IssueID)
		if ctx.IdeaSlug != "" {
			fmt.Fprintf(&b, "2. Run `multica idea get %s --output json` and treat the full idea markdown as the source of truth\n", ctx.IdeaSlug)
			fmt.Fprintf(&b, "3. Run `multica issue comment list %s --output json` to read the discussion\n", ctx.IssueID)
		} else {
			fmt.Fprintf(&b, "2. Run `multica issue comment list %s --output json` to read the discussion\n", ctx.IssueID)
		}
		b.WriteString("   - Use pagination when needed to focus on the latest instructions\n")
		if ctx.TriggerCommentID != "" {
			fmt.Fprintf(&b, "4. Pay special attention to the triggering comment (ID: `%s`) and treat it as feedback on the previous draft plan\n", ctx.TriggerCommentID)
			b.WriteString("   - Revise the plan instead of switching into implementation\n")
		} else {
			b.WriteString("4. Identify implementation scope, affected files/modules, sequencing, risks, and acceptance checks\n")
		}
		b.WriteString("5. If a repository is relevant, inspect its current structure only as needed to produce a grounded plan\n")
		b.WriteString("6. Return a final response that is ready for human confirmation, including:\n")
		b.WriteString("   - goal summary\n")
		b.WriteString("   - implementation steps\n")
		b.WriteString("   - risks or open questions\n")
		b.WriteString("   - validation approach\n\n")
		b.WriteString("End with explicit prompts for any decisions that still need user confirmation.\n\n")
	} else if ctx.TriggerCommentID != "" {
		// Comment-triggered build-mode: focus on requested follow-up work and reply.
		b.WriteString("**This build task was triggered by a comment.** Your primary job is to address the request and reply with the result.\n\n")
		fmt.Fprintf(&b, "1. Run `multica issue get %s --output json` to understand the issue context\n", ctx.IssueID)
		if ctx.IdeaSlug != "" {
			fmt.Fprintf(&b, "2. Run `multica idea get %s --output json` to load the full idea context\n", ctx.IdeaSlug)
			fmt.Fprintf(&b, "3. Run `multica issue comment list %s --output json` to read the conversation\n", ctx.IssueID)
		} else {
			fmt.Fprintf(&b, "2. Run `multica issue comment list %s --output json` to read the conversation\n", ctx.IssueID)
		}
		b.WriteString("   - If the output is very large or truncated, use pagination: `--limit 30` or `--since <timestamp>`\n")
		fmt.Fprintf(&b, "4. Find the triggering comment (ID: `%s`) and address what is being asked\n", ctx.TriggerCommentID)
		fmt.Fprintf(&b, "5. Run `multica issue status %s in_progress`\n", ctx.IssueID)
		b.WriteString("6. If code changes are needed, implement them, commit, and push the branch\n")
		b.WriteString("7. The server handles PR creation after review; lack of immediate PR visibility is not a blocker\n")
		fmt.Fprintf(&b, "8. When delivery artifacts are ready, run `multica issue status %s in_review`\n", ctx.IssueID)
		fmt.Fprintf(&b, "9. Reply in-thread only if useful for the human conversation: `multica issue comment add %s --parent %s --content \"...\"`\n", ctx.IssueID, ctx.TriggerCommentID)
		fmt.Fprintf(&b, "10. Only run `multica issue status %s blocked` for real implementation blockers\n\n", ctx.IssueID)
	} else {
		// Assignment-triggered build-mode: full implementation workflow.
		b.WriteString("You are in build mode. Implement the confirmed plan and manage the issue toward review.\n\n")
		fmt.Fprintf(&b, "1. Run `multica issue get %s --output json` to understand your task\n", ctx.IssueID)
		if ctx.IdeaSlug != "" {
			fmt.Fprintf(&b, "2. Run `multica idea get %s --output json` and treat the full idea markdown as the source of truth\n", ctx.IdeaSlug)
			fmt.Fprintf(&b, "3. Run `multica issue status %s in_progress`\n", ctx.IssueID)
			b.WriteString("4. Read comments for any final human instructions before coding\n")
		} else {
			fmt.Fprintf(&b, "2. Run `multica issue status %s in_progress`\n", ctx.IssueID)
			b.WriteString("3. Read comments for any final human instructions before coding\n")
		}
		b.WriteString("5. If the task requires code changes:\n")
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
		b.WriteString("   f. If no PR is visible yet, do not treat that alone as a blocker — the server-side PR automation may still be running\n")
		b.WriteString("6. If the task does not require code (e.g. research, documentation), post your findings as a comment\n")
		fmt.Fprintf(&b, "7. When delivery artifacts are ready, run `multica issue status %s in_review`\n", ctx.IssueID)
		fmt.Fprintf(&b, "8. Only run `multica issue status %s blocked` for real implementation blockers; lack of immediate PR creation is not by itself a blocker\n\n", ctx.IssueID)
	}

	if len(ctx.AgentSkills) > 0 {
		b.WriteString("## Skills\n\n")
		switch provider {
		case "claude":
			// Claude discovers skills natively from .claude/skills/ — just list names.
			b.WriteString("You have the following skills installed (discovered automatically):\n\n")
		case "codex", "opencode":
			// Codex and OpenCode discover skills natively from their respective paths — just list names.
			b.WriteString("You have the following skills installed (discovered automatically):\n\n")
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

	b.WriteString("## Important: Always Use the `multica` CLI\n\n")
	b.WriteString("All interactions with Multica platform resources — including issues, comments, attachments, images, files, and any other platform data — **must** go through the `multica` CLI. ")
	b.WriteString("Do NOT use `curl`, `wget`, or any other HTTP client to access Multica URLs or APIs directly. ")
	b.WriteString("Multica resource URLs require authenticated access that only the `multica` CLI can provide.\n\n")
	b.WriteString("If you need to perform an operation that is not covered by any existing `multica` command, ")
	b.WriteString("do NOT attempt to work around it. Instead, post a comment mentioning the workspace owner to request the missing functionality.\n\n")

	b.WriteString("## Output\n\n")
	b.WriteString("Keep comments concise and natural — state the outcome, not the process.\n")
	b.WriteString("Good: \"Delivery ready. PR: https://...\"\n")
	b.WriteString("Good: \"Delivery ready. Branch pushed; PR automation should pick this up. Compare: https://...\"\n")
	b.WriteString("Bad: \"1. Read the issue 2. Found the bug in auth.go 3. Created branch 4. ...\"\n")

	return b.String()
}
