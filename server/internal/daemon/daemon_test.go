package daemon

import (
	"net/http"
	"strings"
	"testing"
)

func TestNormalizeServerBaseURL(t *testing.T) {
	t.Parallel()

	got, err := NormalizeServerBaseURL("ws://localhost:8080/ws")
	if err != nil {
		t.Fatalf("NormalizeServerBaseURL returned error: %v", err)
	}
	if got != "http://localhost:8080" {
		t.Fatalf("expected http://localhost:8080, got %s", got)
	}
}

func TestBuildPromptContainsIssueID(t *testing.T) {
	t.Parallel()

	issueID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	prompt := BuildPrompt(Task{
		IssueID: issueID,
		Mode:    "plan",
		Agent: &AgentData{
			Name: "Local Codex",
			Skills: []SkillData{
				{Name: "Concise", Content: "Be concise."},
			},
		},
	})

	// Prompt should contain the issue ID and CLI hint.
	for _, want := range []string{
		issueID,
		"multica issue get",
		"planning-only",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}

	// Skills should NOT be inlined in the prompt (they're in runtime config).
	for _, absent := range []string{"## Agent Skills", "Be concise."} {
		if strings.Contains(prompt, absent) {
			t.Fatalf("prompt should NOT contain %q (skills are in runtime config)", absent)
		}
	}
}

func TestBuildPromptNoIssueDetails(t *testing.T) {
	t.Parallel()

	prompt := BuildPrompt(Task{
		IssueID: "test-id",
		Mode:    "build",
		Agent:   &AgentData{Name: "Test"},
	})

	// Prompt should not contain issue title/description (agent fetches via CLI).
	for _, absent := range []string{"**Issue:**", "**Summary:**"} {
		if strings.Contains(prompt, absent) {
			t.Fatalf("prompt should NOT contain %q — agent fetches details via CLI", absent)
		}
	}
}

func TestBuildPromptIncludesExecutionMode(t *testing.T) {
	t.Parallel()

	planPrompt := BuildPrompt(Task{IssueID: "issue-1", Mode: "plan"})
	if !strings.Contains(planPrompt, "execution mode is: plan") {
		t.Fatalf("expected plan prompt to mention plan mode, got %q", planPrompt)
	}

	buildPrompt := BuildPrompt(Task{IssueID: "issue-2", Mode: "build"})
	if !strings.Contains(buildPrompt, "execution mode is: build") {
		t.Fatalf("expected build prompt to mention build mode, got %q", buildPrompt)
	}
}

func TestAnalyzePlanOutputDraft(t *testing.T) {
	t.Parallel()

	status, requiresDecision, questions := analyzePlanOutput(`
## Plan

1. Update the issue detail UI.

## Open Questions

- Should we keep the badge compact?
- Should mobile use a drawer?
`)

	if status != "draft" {
		t.Fatalf("expected draft, got %q", status)
	}
	if !requiresDecision {
		t.Fatal("expected requiresDecision=true")
	}
	if len(questions) != 2 {
		t.Fatalf("expected 2 questions, got %#v", questions)
	}
}

func TestAnalyzePlanOutputReady(t *testing.T) {
	t.Parallel()

	status, requiresDecision, questions := analyzePlanOutput(`
## Final Plan

1. Update the issue detail UI.
2. Add backend gating.
3. Verify with tests.
`)

	if status != "ready" {
		t.Fatalf("expected ready, got %q", status)
	}
	if requiresDecision {
		t.Fatal("expected requiresDecision=false")
	}
	if len(questions) != 0 {
		t.Fatalf("expected no questions, got %#v", questions)
	}
}

func TestIsWorkspaceNotFoundError(t *testing.T) {
	t.Parallel()

	err := &requestError{
		Method:     http.MethodPost,
		Path:       "/api/daemon/register",
		StatusCode: http.StatusNotFound,
		Body:       `{"error":"workspace not found"}`,
	}
	if !isWorkspaceNotFoundError(err) {
		t.Fatal("expected workspace not found error to be recognized")
	}

	if isWorkspaceNotFoundError(&requestError{StatusCode: http.StatusInternalServerError, Body: `{"error":"workspace not found"}`}) {
		t.Fatal("did not expect 500 to be treated as workspace not found")
	}
}
