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
