package service

import (
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestBuildTaskDeliverySummaryCommentUsesStructuredMetadata(t *testing.T) {
	comment := buildTaskDeliverySummaryComment(protocol.TaskCompletedPayload{
		Summary:       "Delivery ready, but PR creation requires handoff.",
		CompareURL:    "https://github.com/example/repo/compare/main...feat/test?expand=1",
		BranchName:    "feat/test",
		DeliveryState: "handoff_required",
		HandoffReason: "GitHub CLI is not logged in in the task environment.",
		Output:        "Implemented the app and pushed feat/test. gh is not logged in.",
	})

	if comment == "" {
		t.Fatal("expected delivery summary comment")
	}
	if comment == "Implemented the app and pushed feat/test. gh is not logged in." {
		t.Fatal("expected structured summary instead of raw output")
	}
	if !containsAll(comment,
		"Delivery ready, but PR creation requires handoff.",
		"Branch: `feat/test`",
		"Compare: https://github.com/example/repo/compare/main...feat/test?expand=1",
		"Handoff: GitHub CLI is not logged in in the task environment.",
	) {
		t.Fatalf("unexpected delivery summary comment:\n%s", comment)
	}
}

func TestBuildTaskPlanCommentIncludesRevisionAndThreadPrompt(t *testing.T) {
	comment := buildTaskPlanComment(protocol.TaskCompletedPayload{
		Summary:      "Finalized implementation approach.",
		Output:       "1. Update the issue detail card.\n2. Add confirm-plan dispatch.\n3. Verify with tests.",
		PlanRevision: 3,
		PlanStatus:   "ready",
	})

	if comment == "" {
		t.Fatal("expected plan comment")
	}
	if !containsAll(comment,
		"Plan revision 3",
		"Finalized implementation approach.",
		"1. Update the issue detail card.",
		"Reply in this thread",
		"Confirm Plan",
	) {
		t.Fatalf("unexpected plan comment:\n%s", comment)
	}
}

func TestBuildTaskPlanCommentShowsOpenDecisions(t *testing.T) {
	comment := buildTaskPlanComment(protocol.TaskCompletedPayload{
		Summary:              "Need your input.",
		Output:               "Draft plan body.",
		PlanRevision:         2,
		PlanStatus:           "draft",
		PlanRequiresDecision: true,
		PlanQuestions:        []string{"Should we keep the badge compact?", "Should mobile open as sheet or drawer?"},
	})

	if !containsAll(comment,
		"Plan revision 2",
		"Need your input.",
		"Open Decisions",
		"Should we keep the badge compact?",
		"Confirm Plan will stay disabled",
	) {
		t.Fatalf("unexpected draft plan comment:\n%s", comment)
	}
}

func containsAll(text string, parts ...string) bool {
	for _, part := range parts {
		if part == "" {
			continue
		}
		if !strings.Contains(text, part) {
			return false
		}
	}
	return true
}
