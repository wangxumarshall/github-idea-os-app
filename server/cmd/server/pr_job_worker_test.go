package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func writeJSONFixture(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func testAgentIDForWorkspace(t *testing.T, workspaceID string) string {
	t.Helper()
	var agentID string
	if err := testPool.QueryRow(context.Background(), `
		SELECT id::text
		FROM agent
		WHERE workspace_id = $1
		ORDER BY created_at ASC
		LIMIT 1
	`, workspaceID).Scan(&agentID); err != nil {
		t.Fatalf("load agent id: %v", err)
	}
	return agentID
}

func ensureGitHubAccountForUser(t *testing.T, userID, login, token string) string {
	t.Helper()
	encryptedToken, err := auth.EncryptString(token)
	if err != nil {
		t.Fatalf("encrypt token: %v", err)
	}
	var accountID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO github_account (
			user_id, github_user_id, login, access_token_encrypted, token_type, scope
		)
		VALUES ($1, $2, $3, $4, 'bearer', 'repo')
		ON CONFLICT (user_id) DO UPDATE SET
			github_user_id = EXCLUDED.github_user_id,
			login = EXCLUDED.login,
			access_token_encrypted = EXCLUDED.access_token_encrypted,
			token_type = EXCLUDED.token_type,
			scope = EXCLUDED.scope,
			updated_at = now()
		RETURNING id::text
	`, userID, int64(424242), login, encryptedToken).Scan(&accountID); err != nil {
		t.Fatalf("upsert github account: %v", err)
	}
	return accountID
}

func TestProcessIssuePRJobCreatesPullRequestForIssueCreator(t *testing.T) {
	queries := db.New(testPool)
	bus := events.New()
	store := service.NewIssuePRStore()
	taskService := service.NewTaskService(queries, nil, bus)
	oauth := service.NewGitHubOAuthService()
	gh := service.NewGitHubIdeaOSService()

	agentID := testAgentIDForWorkspace(t, testWorkspaceID)
	ensureGitHubAccountForUser(t, testUserID, "integration-user", "ghp_test")

	var issueID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, position, repo_url)
		VALUES ($1, $2, 'in_review', 'medium', 'member', $3, 0, $4)
		RETURNING id::text
	`, testWorkspaceID, "PR worker test issue", testUserID, "https://github.com/example/repo").Scan(&issueID); err != nil {
		t.Fatalf("insert issue: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM issue_pr_job WHERE issue_id = $1`, issueID)
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE issue_id = $1`, issueID)
		testPool.Exec(context.Background(), `DELETE FROM activity_log WHERE issue_id = $1`, issueID)
		testPool.Exec(context.Background(), `DELETE FROM comment WHERE issue_id = $1`, issueID)
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
	})

	resultJSON, _ := json.Marshal(protocol.TaskCompletedPayload{
		Output:     "Completed implementation.",
		Summary:    "Delivery ready, but PR creation requires handoff.",
		BranchName: "agent/local-codex/pr-job-test",
		CompareURL: "https://github.com/example/repo/compare/main...agent/local-codex/pr-job-test?expand=1",
	})

	var taskID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_task_queue (
			agent_id, runtime_id, issue_id, status, priority, dispatched_at, started_at, completed_at, result
		)
		SELECT id, runtime_id, $1::uuid, 'completed', 2, now(), now(), now(), $2::jsonb
		FROM agent
		WHERE id = $3::uuid
		RETURNING id::text
	`, issueID, string(resultJSON), agentID).Scan(&taskID); err != nil {
		t.Fatalf("insert completed task: %v", err)
	}

	if err := store.Enqueue(context.Background(), testPool, issueID, map[string]any{"trigger": "test"}); err != nil {
		t.Fatalf("enqueue pr job: %v", err)
	}
	job, err := store.ClaimNext(context.Background(), testPool)
	if err != nil {
		t.Fatalf("claim pr job: %v", err)
	}
	if job == nil {
		t.Fatal("expected claimed issue PR job")
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/example/repo":
			writeJSONFixture(w, map[string]any{
				"name":           "repo",
				"default_branch": "main",
				"html_url":       "https://github.com/example/repo",
				"owner": map[string]any{
					"login": "example",
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/repos/example/repo/pulls":
			writeJSONFixture(w, []map[string]any{})
		case r.Method == http.MethodPost && r.URL.Path == "/repos/example/repo/pulls":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode pull request request: %v", err)
			}
			if req["head"] != "agent/local-codex/pr-job-test" {
				t.Fatalf("expected head branch, got %#v", req["head"])
			}
			if req["base"] != "main" {
				t.Fatalf("expected base branch main, got %#v", req["base"])
			}
			writeJSONFixture(w, map[string]any{
				"number":   17,
				"html_url": "https://github.com/example/repo/pull/17",
				"state":    "open",
				"title":    req["title"],
			})
		default:
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		}
	})

	server := httptest.NewServer(handler)
	defer server.Close()
	gh.BaseURL = server.URL
	gh.HTTPClient = server.Client()
	oauth.HTTPClient = server.Client()

	processIssuePRJob(context.Background(), testPool, queries, bus, taskService, store, oauth, gh, job)

	rows, err := queries.ListTasksByIssue(context.Background(), util.ParseUUID(issueID))
	if err != nil {
		t.Fatalf("ListTasksByIssue: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected task rows")
	}
	var payload protocol.TaskCompletedPayload
	if err := json.Unmarshal(rows[0].Result, &payload); err != nil {
		t.Fatalf("unmarshal task result: %v", err)
	}
	if payload.PRURL != "https://github.com/example/repo/pull/17" {
		t.Fatalf("expected PR URL to be written back, got %q", payload.PRURL)
	}
	if payload.DeliveryState != "delivered" {
		t.Fatalf("expected delivery_state delivered, got %q", payload.DeliveryState)
	}
	if payload.DeliveryCommentID == "" {
		t.Fatal("expected delivery comment id to be persisted")
	}

	activities := listActivitiesForIssue(t, queries, issueID)
	if len(activities) == 0 || activities[0].Action != "delivery_ready" {
		t.Fatalf("expected latest activity delivery_ready, got %#v", activities)
	}

	var commentCount int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM comment WHERE issue_id = $1`, issueID).Scan(&commentCount); err != nil {
		t.Fatalf("count comments: %v", err)
	}
	if commentCount != 1 {
		t.Fatalf("expected 1 delivery summary comment, got %d", commentCount)
	}

	var jobStatus string
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM issue_pr_job WHERE id = $1`, job.ID).Scan(&jobStatus); err != nil {
		t.Fatalf("load job status: %v", err)
	}
	if jobStatus != "completed" {
		t.Fatalf("expected completed job status, got %q", jobStatus)
	}

	if taskID == "" {
		t.Fatal("expected task id to be created")
	}
}

func TestTaskCompletedListenerEnqueuesPRJobForReviewReadyResultEvenWhenIssueBlocked(t *testing.T) {
	queries := db.New(testPool)
	bus := events.New()
	store := service.NewIssuePRStore()

	var issueID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, position, repo_url)
		VALUES ($1, $2, 'blocked', 'medium', 'member', $3, 0, $4)
		RETURNING id::text
	`, testWorkspaceID, "PR listener race test issue", testUserID, "https://github.com/example/repo").Scan(&issueID); err != nil {
		t.Fatalf("insert issue: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM issue_pr_job WHERE issue_id = $1`, issueID)
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
	})

	registerPullRequestListeners(bus, queries, store, testPool)

	bus.Publish(events.Event{
		Type:        protocol.EventTaskCompleted,
		WorkspaceID: testWorkspaceID,
		ActorType:   "system",
		ActorID:     "",
		Payload: map[string]any{
			"task_id":  "00000000-0000-0000-0000-000000000099",
			"agent_id": testAgentIDForWorkspace(t, testWorkspaceID),
			"issue_id": issueID,
			"status":   "completed",
			"result": map[string]any{
				"branch_name":    "agent/local-codex/review-ready",
				"compare_url":    "https://github.com/example/repo/compare/main...agent/local-codex/review-ready?expand=1",
				"delivery_state": "completed",
			},
		},
	})

	var count int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM issue_pr_job WHERE issue_id = $1 AND status = 'queued'`, issueID).Scan(&count); err != nil {
		t.Fatalf("count queued issue pr jobs: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 queued issue_pr_job, got %d", count)
	}
}
