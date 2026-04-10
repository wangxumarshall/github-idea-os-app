package daemon

import (
	"fmt"
	"strings"
)

// BuildPrompt constructs the task prompt for an agent CLI.
// Keep this minimal — detailed instructions live in CLAUDE.md / AGENTS.md
// injected by execenv.InjectRuntimeConfig.
func BuildPrompt(task Task) string {
	var b strings.Builder
	mode := strings.TrimSpace(task.Mode)
	if mode == "" {
		mode = "build"
	}
	b.WriteString("You are running as a local coding agent for a Multica workspace.\n\n")
	fmt.Fprintf(&b, "Your execution mode is: %s\n\n", mode)
	fmt.Fprintf(&b, "Your assigned issue ID is: %s\n\n", task.IssueID)
	if mode == "plan" {
		fmt.Fprintf(&b, "Start by running `multica issue get %s --output json`, then produce a planning-only response with no code changes.\n", task.IssueID)
	} else {
		fmt.Fprintf(&b, "Start by running `multica issue get %s --output json`, then implement the confirmed plan.\n", task.IssueID)
	}
	return b.String()
}
