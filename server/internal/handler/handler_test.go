package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

var testHandler *Handler
var testPool *pgxpool.Pool
var testUserID string
var testWorkspaceID string

const (
	handlerTestEmail         = "handler-test@multica.ai"
	handlerTestName          = "Handler Test User"
	handlerTestWorkspaceSlug = "handler-tests"
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://multica:multica@localhost:5432/multica?sslmode=disable"
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Printf("Skipping tests: could not connect to database: %v\n", err)
		os.Exit(0)
	}
	if err := pool.Ping(ctx); err != nil {
		fmt.Printf("Skipping tests: database not reachable: %v\n", err)
		pool.Close()
		os.Exit(0)
	}

	queries := db.New(pool)
	hub := realtime.NewHub()
	go hub.Run()
	bus := events.New()
	emailSvc := service.NewEmailService()
	testHandler = New(queries, pool, hub, bus, emailSvc, nil, nil)
	testPool = pool

	testUserID, testWorkspaceID, err = setupHandlerTestFixture(ctx, pool)
	if err != nil {
		fmt.Printf("Failed to set up handler test fixture: %v\n", err)
		pool.Close()
		os.Exit(1)
	}

	code := m.Run()
	if err := cleanupHandlerTestFixture(context.Background(), pool); err != nil {
		fmt.Printf("Failed to clean up handler test fixture: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	pool.Close()
	os.Exit(code)
}

func setupHandlerTestFixture(ctx context.Context, pool *pgxpool.Pool) (string, string, error) {
	if err := cleanupHandlerTestFixture(ctx, pool); err != nil {
		return "", "", err
	}

	var userID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO "user" (name, email)
		VALUES ($1, $2)
		RETURNING id
	`, handlerTestName, handlerTestEmail).Scan(&userID); err != nil {
		return "", "", err
	}

	var workspaceID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO workspace (name, slug, description, issue_prefix)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, "Handler Tests", handlerTestWorkspaceSlug, "Temporary workspace for handler tests", "HAN").Scan(&workspaceID); err != nil {
		return "", "", err
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO member (workspace_id, user_id, role)
		VALUES ($1, $2, 'owner')
	`, workspaceID, userID); err != nil {
		return "", "", err
	}

	var runtimeID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent_runtime (
			workspace_id, daemon_id, name, runtime_mode, provider, status, device_info, metadata, last_seen_at
		)
		VALUES ($1, NULL, $2, 'cloud', $3, 'online', $4, '{}'::jsonb, now())
		RETURNING id
	`, workspaceID, "Handler Test Runtime", "handler_test_runtime", "Handler test runtime").Scan(&runtimeID); err != nil {
		return "", "", err
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO agent (
			workspace_id, name, description, runtime_mode, runtime_config,
			runtime_id, visibility, max_concurrent_tasks, owner_id, tools, triggers
		)
		VALUES ($1, $2, '', 'cloud', '{}'::jsonb, $3, 'workspace', 1, $4, '[]'::jsonb, '[]'::jsonb)
	`, workspaceID, "Handler Test Agent", runtimeID, userID); err != nil {
		return "", "", err
	}

	return userID, workspaceID, nil
}

func cleanupHandlerTestFixture(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `DELETE FROM workspace WHERE slug = $1`, handlerTestWorkspaceSlug); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, `DELETE FROM "user" WHERE email = $1`, handlerTestEmail); err != nil {
		return err
	}
	return nil
}

func createTestAgentWithProvider(t *testing.T, provider string) string {
	t.Helper()
	displayName := provider
	if displayName != "" {
		displayName = strings.ToUpper(displayName[:1]) + displayName[1:]
	}

	var runtimeID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_runtime (
			workspace_id, daemon_id, name, runtime_mode, provider, status, device_info, metadata, last_seen_at
		)
		VALUES ($1, NULL, $2, 'local', $3, 'online', $4, '{}'::jsonb, now())
		RETURNING id
	`, testWorkspaceID, fmt.Sprintf("Handler %s Runtime", provider), provider, fmt.Sprintf("%s runtime", provider)).Scan(&runtimeID); err != nil {
		t.Fatalf("insert runtime: %v", err)
	}

	var agentID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent (
			workspace_id, name, description, runtime_mode, runtime_config,
			runtime_id, visibility, max_concurrent_tasks, owner_id, tools, triggers
		)
		VALUES ($1, $2, '', 'local', '{}'::jsonb, $3, 'workspace', 1, $4, '[]'::jsonb, '[]'::jsonb)
		RETURNING id
	`, testWorkspaceID, fmt.Sprintf("%s Test Agent", displayName), runtimeID, testUserID).Scan(&agentID); err != nil {
		t.Fatalf("insert agent: %v", err)
	}

	return agentID
}

func insertCompletedPlanTask(t *testing.T, issueID, agentID string, result map[string]any) {
	t.Helper()

	var runtimeID string
	if err := testPool.QueryRow(context.Background(), `
		SELECT runtime_id
		FROM agent
		WHERE id = $1
	`, agentID).Scan(&runtimeID); err != nil {
		t.Fatalf("load runtime: %v", err)
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}

	if _, err := testPool.Exec(context.Background(), `
		INSERT INTO agent_task_queue (
			agent_id, runtime_id, issue_id, status, priority, mode,
			dispatched_at, started_at, completed_at, result
		) VALUES (
			$1, $2, $3, 'completed', 2, 'plan',
			now(), now(), now(), $4
		)
	`, agentID, runtimeID, issueID, resultJSON); err != nil {
		t.Fatalf("insert completed plan task: %v", err)
	}
}

func newRequest(method, path string, body any) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	return req
}

func withURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestIssueCRUD(t *testing.T) {
	// Create
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":    "Test issue from Go test",
		"status":   "todo",
		"priority": "medium",
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssue: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created IssueResponse
	json.NewDecoder(w.Body).Decode(&created)
	if created.Title != "Test issue from Go test" {
		t.Fatalf("CreateIssue: expected title 'Test issue from Go test', got '%s'", created.Title)
	}
	if created.Status != "todo" {
		t.Fatalf("CreateIssue: expected status 'todo', got '%s'", created.Status)
	}
	issueID := created.ID

	// Get
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues/"+issueID, nil)
	req = withURLParam(req, "id", issueID)
	testHandler.GetIssue(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetIssue: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var fetched IssueResponse
	json.NewDecoder(w.Body).Decode(&fetched)
	if fetched.ID != issueID {
		t.Fatalf("GetIssue: expected id '%s', got '%s'", issueID, fetched.ID)
	}

	// Update - partial (only status)
	w = httptest.NewRecorder()
	status := "in_progress"
	req = newRequest("PUT", "/api/issues/"+issueID, map[string]any{
		"status": status,
	})
	req = withURLParam(req, "id", issueID)
	testHandler.UpdateIssue(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateIssue: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated IssueResponse
	json.NewDecoder(w.Body).Decode(&updated)
	if updated.Status != "in_progress" {
		t.Fatalf("UpdateIssue: expected status 'in_progress', got '%s'", updated.Status)
	}
	if updated.Title != "Test issue from Go test" {
		t.Fatalf("UpdateIssue: title should be preserved, got '%s'", updated.Title)
	}
	if updated.Priority != "medium" {
		t.Fatalf("UpdateIssue: priority should be preserved, got '%s'", updated.Priority)
	}

	// List
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues?workspace_id="+testWorkspaceID, nil)
	testHandler.ListIssues(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListIssues: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var listResp map[string]any
	json.NewDecoder(w.Body).Decode(&listResp)
	issues := listResp["issues"].([]any)
	if len(issues) == 0 {
		t.Fatal("ListIssues: expected at least 1 issue")
	}

	// Delete
	w = httptest.NewRecorder()
	req = newRequest("DELETE", "/api/issues/"+issueID, nil)
	req = withURLParam(req, "id", issueID)
	testHandler.DeleteIssue(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DeleteIssue: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify deleted
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues/"+issueID, nil)
	req = withURLParam(req, "id", issueID)
	testHandler.GetIssue(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GetIssue after delete: expected 404, got %d", w.Code)
	}
}

func TestConfirmPlanQueuesBuildTask(t *testing.T) {
	ctx := context.Background()

	var agentID string
	if err := testPool.QueryRow(ctx, `
		SELECT id
		FROM agent
		WHERE workspace_id = $1
		ORDER BY created_at ASC
		LIMIT 1
	`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("load agent: %v", err)
	}

	legacyIdea, err := testHandler.IdeaStore.EnsureLegacyIdea(ctx, testPool, testWorkspaceID)
	if err != nil {
		t.Fatalf("ensure legacy idea: %v", err)
	}

	var issueID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO issue (
			workspace_id, title, description, status, priority,
			assignee_type, assignee_id, creator_type, creator_id,
			parent_issue_id, position, due_date, number, repo_url, idea_id, execution_stage
		) VALUES (
			$1, $2, NULL, 'todo', 'medium',
			'agent', $3, 'member', $4,
			NULL, 0, NULL, 9001, NULL, $5, 'plan_ready'
		)
		RETURNING id
	`, testWorkspaceID, "Plan confirmation test", agentID, testUserID, legacyIdea.ID).Scan(&issueID); err != nil {
		t.Fatalf("insert issue: %v", err)
	}
	insertCompletedPlanTask(t, issueID, agentID, map[string]any{
		"output":                 "Final plan",
		"summary":                "Ready to implement.",
		"plan_status":            "ready",
		"plan_requires_decision": false,
	})

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues/"+issueID+"/confirm-plan", nil)
	req = withURLParam(req, "id", issueID)
	testHandler.ConfirmPlan(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ConfirmPlan: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if updated.ExecutionStage != service.ExecutionStageBuildReady {
		t.Fatalf("expected execution_stage %q, got %q", service.ExecutionStageBuildReady, updated.ExecutionStage)
	}

	var mode string
	if err := testPool.QueryRow(ctx, `
		SELECT mode
		FROM agent_task_queue
		WHERE issue_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, issueID).Scan(&mode); err != nil {
		t.Fatalf("load queued task: %v", err)
	}
	if mode != service.TaskModeBuild {
		t.Fatalf("expected queued task mode %q, got %q", service.TaskModeBuild, mode)
	}
}

func TestCreateIssueQueuesCodexPlanTask(t *testing.T) {
	codexAgentID := createTestAgentWithProvider(t, "codex")
	legacyIdea, err := testHandler.IdeaStore.EnsureLegacyIdea(context.Background(), testPool, testWorkspaceID)
	if err != nil {
		t.Fatalf("ensure legacy idea: %v", err)
	}

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":         "Codex staged execution warning",
		"status":        "todo",
		"priority":      "medium",
		"assignee_type": "agent",
		"assignee_id":   codexAgentID,
		"idea_id":       legacyIdea.ID,
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssue: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode issue response: %v", err)
	}

	var mode string
	if err := testPool.QueryRow(context.Background(), `
		SELECT mode
		FROM agent_task_queue
		WHERE issue_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, parseUUID(created.ID)).Scan(&mode); err != nil {
		t.Fatalf("load queued task: %v", err)
	}
	if mode != service.TaskModePlan {
		t.Fatalf("expected queued Codex task mode %q, got %q", service.TaskModePlan, mode)
	}
}

func TestConfirmPlanQueuesBuildForCodexAgent(t *testing.T) {
	codexAgentID := createTestAgentWithProvider(t, "codex")
	legacyIdea, err := testHandler.IdeaStore.EnsureLegacyIdea(context.Background(), testPool, testWorkspaceID)
	if err != nil {
		t.Fatalf("ensure legacy idea: %v", err)
	}

	var issueID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO issue (
			workspace_id, title, description, status, priority,
			assignee_type, assignee_id, creator_type, creator_id,
			parent_issue_id, position, due_date, number, repo_url, idea_id, execution_stage
		) VALUES (
			$1, $2, NULL, 'todo', 'medium',
			'agent', $3, 'member', $4,
			NULL, 0, NULL, 9002, NULL, $5, 'plan_ready'
		)
		RETURNING id
	`, testWorkspaceID, "Codex confirm plan test", codexAgentID, testUserID, legacyIdea.ID).Scan(&issueID); err != nil {
		t.Fatalf("insert issue: %v", err)
	}
	insertCompletedPlanTask(t, issueID, codexAgentID, map[string]any{
		"output":                 "Codex final plan",
		"summary":                "Ready to implement.",
		"plan_status":            "ready",
		"plan_requires_decision": false,
	})

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues/"+issueID+"/confirm-plan", nil)
	req = withURLParam(req, "id", issueID)
	testHandler.ConfirmPlan(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ConfirmPlan: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var mode string
	if err := testPool.QueryRow(context.Background(), `
		SELECT mode
		FROM agent_task_queue
		WHERE issue_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, issueID).Scan(&mode); err != nil {
		t.Fatalf("load queued task: %v", err)
	}
	if mode != service.TaskModeBuild {
		t.Fatalf("expected queued Codex task mode %q, got %q", service.TaskModeBuild, mode)
	}
}

func TestPlanThreadReplyQueuesAnotherPlanRun(t *testing.T) {
	legacyIdea, err := testHandler.IdeaStore.EnsureLegacyIdea(context.Background(), testPool, testWorkspaceID)
	if err != nil {
		t.Fatalf("ensure legacy idea: %v", err)
	}

	var agentID string
	if err := testPool.QueryRow(context.Background(), `
		SELECT id
		FROM agent
		WHERE workspace_id = $1
		ORDER BY created_at ASC
		LIMIT 1
	`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("load agent: %v", err)
	}

	var issueID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO issue (
			workspace_id, title, description, status, priority,
			assignee_type, assignee_id, creator_type, creator_id,
			parent_issue_id, position, due_date, number, repo_url, idea_id, execution_stage
		) VALUES (
			$1, $2, NULL, 'todo', 'medium',
			'agent', $3, 'member', $4,
			NULL, 0, NULL, 9003, NULL, $5, 'plan_ready'
		)
		RETURNING id
	`, testWorkspaceID, "Plan feedback issue", agentID, testUserID, legacyIdea.ID).Scan(&issueID); err != nil {
		t.Fatalf("insert issue: %v", err)
	}

	var rootCommentID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO comment (issue_id, workspace_id, author_type, author_id, content, type, parent_id)
		VALUES ($1, $2, 'agent', $3, 'Initial plan', 'system', NULL)
		RETURNING id
	`, issueID, testWorkspaceID, agentID).Scan(&rootCommentID); err != nil {
		t.Fatalf("insert root plan comment: %v", err)
	}
	insertCompletedPlanTask(t, issueID, agentID, map[string]any{
		"output":                      "Draft plan",
		"summary":                     "Need a revision.",
		"plan_status":                 "draft",
		"plan_requires_decision":      true,
		"plan_questions":              []string{"Should we update the sidebar copy?"},
		"plan_comment_id":             rootCommentID,
		"plan_thread_root_comment_id": rootCommentID,
		"plan_revision":               1,
	})

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues/"+issueID+"/comments", map[string]any{
		"content":   "Please revise the plan and also loop in [@Other](mention://member/00000000-0000-0000-0000-000000000001).",
		"parent_id": rootCommentID,
	})
	req = withURLParam(req, "id", issueID)
	testHandler.CreateComment(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateComment: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var mode string
	if err := testPool.QueryRow(context.Background(), `
		SELECT mode
		FROM agent_task_queue
		WHERE issue_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, issueID).Scan(&mode); err != nil {
		t.Fatalf("load task: %v", err)
	}
	if mode != service.TaskModePlan {
		t.Fatalf("expected plan task, got %q", mode)
	}
}

func TestTopLevelCommentDoesNotQueuePlanRun(t *testing.T) {
	legacyIdea, err := testHandler.IdeaStore.EnsureLegacyIdea(context.Background(), testPool, testWorkspaceID)
	if err != nil {
		t.Fatalf("ensure legacy idea: %v", err)
	}

	var agentID string
	if err := testPool.QueryRow(context.Background(), `
		SELECT id
		FROM agent
		WHERE workspace_id = $1
		ORDER BY created_at ASC
		LIMIT 1
	`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("load agent: %v", err)
	}

	var issueID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO issue (
			workspace_id, title, description, status, priority,
			assignee_type, assignee_id, creator_type, creator_id,
			parent_issue_id, position, due_date, number, repo_url, idea_id, execution_stage
		) VALUES (
			$1, $2, NULL, 'todo', 'medium',
			'agent', $3, 'member', $4,
			NULL, 0, NULL, 9004, NULL, $5, 'plan_ready'
		)
		RETURNING id
	`, testWorkspaceID, "Plan top-level comment issue", agentID, testUserID, legacyIdea.ID).Scan(&issueID); err != nil {
		t.Fatalf("insert issue: %v", err)
	}

	var rootCommentID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO comment (issue_id, workspace_id, author_type, author_id, content, type, parent_id)
		VALUES ($1, $2, 'agent', $3, 'Initial plan', 'system', NULL)
		RETURNING id
	`, issueID, testWorkspaceID, agentID).Scan(&rootCommentID); err != nil {
		t.Fatalf("insert root plan comment: %v", err)
	}
	insertCompletedPlanTask(t, issueID, agentID, map[string]any{
		"output":                      "Draft plan",
		"summary":                     "Need a revision.",
		"plan_status":                 "draft",
		"plan_requires_decision":      true,
		"plan_questions":              []string{"Should we update the sidebar copy?"},
		"plan_comment_id":             rootCommentID,
		"plan_thread_root_comment_id": rootCommentID,
		"plan_revision":               1,
	})

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues/"+issueID+"/comments", map[string]any{
		"content": "Thanks, I will think about it.",
	})
	req = withURLParam(req, "id", issueID)
	testHandler.CreateComment(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateComment: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var taskCount int
	if err := testPool.QueryRow(context.Background(), `
		SELECT count(*)
		FROM agent_task_queue
		WHERE issue_id = $1
	`, issueID).Scan(&taskCount); err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if taskCount != 1 {
		t.Fatalf("expected only the seeded completed plan task, got %d tasks", taskCount)
	}
}

func TestConfirmPlanRejectsDraftPlan(t *testing.T) {
	ctx := context.Background()
	var agentID string
	if err := testPool.QueryRow(ctx, `
		SELECT id
		FROM agent
		WHERE workspace_id = $1
		ORDER BY created_at ASC
		LIMIT 1
	`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("load agent: %v", err)
	}
	legacyIdea, err := testHandler.IdeaStore.EnsureLegacyIdea(ctx, testPool, testWorkspaceID)
	if err != nil {
		t.Fatalf("ensure legacy idea: %v", err)
	}

	var issueID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO issue (
			workspace_id, title, description, status, priority,
			assignee_type, assignee_id, creator_type, creator_id,
			parent_issue_id, position, due_date, number, repo_url, idea_id, execution_stage
		) VALUES (
			$1, $2, NULL, 'todo', 'medium',
			'agent', $3, 'member', $4,
			NULL, 0, NULL, 9005, NULL, $5, 'plan_ready'
		)
		RETURNING id
	`, testWorkspaceID, "Draft plan issue", agentID, testUserID, legacyIdea.ID).Scan(&issueID); err != nil {
		t.Fatalf("insert issue: %v", err)
	}
	insertCompletedPlanTask(t, issueID, agentID, map[string]any{
		"output":                 "Draft plan",
		"summary":                "Need a decision.",
		"plan_status":            "draft",
		"plan_requires_decision": true,
		"plan_questions":         []string{"Should we change the sidebar layout?"},
	})

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues/"+issueID+"/confirm-plan", nil)
	req = withURLParam(req, "id", issueID)
	testHandler.ConfirmPlan(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("ConfirmPlan: expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCommentCRUD(t *testing.T) {
	// Create an issue first
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": "Comment test issue",
	})
	testHandler.CreateIssue(w, req)
	var issue IssueResponse
	json.NewDecoder(w.Body).Decode(&issue)
	issueID := issue.ID

	// Create comment
	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/issues/"+issueID+"/comments", map[string]any{
		"content": "Test comment from Go test",
	})
	req = withURLParam(req, "id", issueID)
	testHandler.CreateComment(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateComment: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// List comments
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues/"+issueID+"/comments", nil)
	req = withURLParam(req, "id", issueID)
	testHandler.ListComments(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListComments: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var comments []CommentResponse
	json.NewDecoder(w.Body).Decode(&comments)
	if len(comments) != 1 {
		t.Fatalf("ListComments: expected 1 comment, got %d", len(comments))
	}
	if comments[0].Content != "Test comment from Go test" {
		t.Fatalf("ListComments: expected content 'Test comment from Go test', got '%s'", comments[0].Content)
	}

	// Cleanup
	w = httptest.NewRecorder()
	req = newRequest("DELETE", "/api/issues/"+issueID, nil)
	req = withURLParam(req, "id", issueID)
	testHandler.DeleteIssue(w, req)
}

func TestAgentCRUD(t *testing.T) {
	// List agents
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/agents?workspace_id="+testWorkspaceID, nil)
	testHandler.ListAgents(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListAgents: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var agents []AgentResponse
	json.NewDecoder(w.Body).Decode(&agents)
	if len(agents) == 0 {
		t.Fatal("ListAgents: expected at least 1 agent")
	}

	// Update agent status
	agentID := agents[0].ID
	w = httptest.NewRecorder()
	req = newRequest("PUT", "/api/agents/"+agentID, map[string]any{
		"status": "idle",
	})
	req = withURLParam(req, "id", agentID)
	testHandler.UpdateAgent(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateAgent: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated AgentResponse
	json.NewDecoder(w.Body).Decode(&updated)
	if updated.Status != "idle" {
		t.Fatalf("UpdateAgent: expected status 'idle', got '%s'", updated.Status)
	}
	if updated.Name != agents[0].Name {
		t.Fatalf("UpdateAgent: name should be preserved, got '%s'", updated.Name)
	}
}

func TestWorkspaceCRUD(t *testing.T) {
	// List workspaces
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/workspaces", nil)
	testHandler.ListWorkspaces(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListWorkspaces: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var workspaces []WorkspaceResponse
	json.NewDecoder(w.Body).Decode(&workspaces)
	if len(workspaces) == 0 {
		t.Fatal("ListWorkspaces: expected at least 1 workspace")
	}

	// Get workspace
	wsID := workspaces[0].ID
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/workspaces/"+wsID, nil)
	req = withURLParam(req, "id", wsID)
	testHandler.GetWorkspace(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetWorkspace: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSendCode(t *testing.T) {
	w := httptest.NewRecorder()
	body := map[string]string{"email": "sendcode-test@multica.ai"}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)
	req := httptest.NewRequest("POST", "/auth/send-code", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.SendCode(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("SendCode: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["message"] == "" {
		t.Fatal("SendCode: expected non-empty message")
	}

	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM verification_code WHERE email = $1`, "sendcode-test@multica.ai")
	})
}

func TestSendCodeRateLimit(t *testing.T) {
	const email = "ratelimit-test@multica.ai"
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM verification_code WHERE email = $1`, email)
	})

	// First request should succeed
	w := httptest.NewRecorder()
	body := map[string]string{"email": email}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)
	req := httptest.NewRequest("POST", "/auth/send-code", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.SendCode(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("SendCode (first): expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Second request within 60s should be rate limited
	w = httptest.NewRecorder()
	buf.Reset()
	json.NewEncoder(&buf).Encode(body)
	req = httptest.NewRequest("POST", "/auth/send-code", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.SendCode(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("SendCode (second): expected 429, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVerifyCode(t *testing.T) {
	const email = "verify-test@multica.ai"
	ctx := context.Background()

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM verification_code WHERE email = $1`, email)
		user, err := testHandler.Queries.GetUserByEmail(ctx, email)
		if err == nil {
			workspaces, listErr := testHandler.Queries.ListWorkspaces(ctx, user.ID)
			if listErr == nil {
				for _, workspace := range workspaces {
					_ = testHandler.Queries.DeleteWorkspace(ctx, workspace.ID)
				}
			}
		}
		testPool.Exec(ctx, `DELETE FROM "user" WHERE email = $1`, email)
	})

	// Send code first
	w := httptest.NewRecorder()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]string{"email": email})
	req := httptest.NewRequest("POST", "/auth/send-code", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.SendCode(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("SendCode: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Read code from DB
	dbCode, err := testHandler.Queries.GetLatestVerificationCode(ctx, email)
	if err != nil {
		t.Fatalf("GetLatestVerificationCode: %v", err)
	}

	// Verify with correct code
	w = httptest.NewRecorder()
	buf.Reset()
	json.NewEncoder(&buf).Encode(map[string]string{"email": email, "code": dbCode.Code})
	req = httptest.NewRequest("POST", "/auth/verify-code", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.VerifyCode(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("VerifyCode: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp LoginResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Token == "" {
		t.Fatal("VerifyCode: expected non-empty token")
	}
	if resp.User.Email != email {
		t.Fatalf("VerifyCode: expected email '%s', got '%s'", email, resp.User.Email)
	}
}

func TestVerifyCodeWrongCode(t *testing.T) {
	const email = "wrong-code-test@multica.ai"
	ctx := context.Background()

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM verification_code WHERE email = $1`, email)
	})

	// Send code
	w := httptest.NewRecorder()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]string{"email": email})
	req := httptest.NewRequest("POST", "/auth/send-code", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.SendCode(w, req)

	// Verify with wrong code
	w = httptest.NewRecorder()
	buf.Reset()
	json.NewEncoder(&buf).Encode(map[string]string{"email": email, "code": "000000"})
	req = httptest.NewRequest("POST", "/auth/verify-code", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.VerifyCode(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("VerifyCode (wrong code): expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVerifyCodeBruteForceProtection(t *testing.T) {
	const email = "bruteforce-test@multica.ai"
	ctx := context.Background()

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM verification_code WHERE email = $1`, email)
	})

	// Send code
	w := httptest.NewRecorder()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]string{"email": email})
	req := httptest.NewRequest("POST", "/auth/send-code", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.SendCode(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("SendCode: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Read actual code so we can try it after lockout
	dbCode, err := testHandler.Queries.GetLatestVerificationCode(ctx, email)
	if err != nil {
		t.Fatalf("GetLatestVerificationCode: %v", err)
	}

	// Exhaust all 5 attempts with wrong codes
	for i := 0; i < 5; i++ {
		w = httptest.NewRecorder()
		buf.Reset()
		json.NewEncoder(&buf).Encode(map[string]string{"email": email, "code": "000000"})
		req = httptest.NewRequest("POST", "/auth/verify-code", &buf)
		req.Header.Set("Content-Type", "application/json")
		testHandler.VerifyCode(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("attempt %d: expected 400, got %d", i+1, w.Code)
		}
	}

	// Now even the correct code should be rejected (code is locked out)
	w = httptest.NewRecorder()
	buf.Reset()
	json.NewEncoder(&buf).Encode(map[string]string{"email": email, "code": dbCode.Code})
	req = httptest.NewRequest("POST", "/auth/verify-code", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.VerifyCode(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("after lockout: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVerifyCodeCreatesWorkspace(t *testing.T) {
	const email = "workspace-verify-test@multica.ai"
	ctx := context.Background()

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM verification_code WHERE email = $1`, email)
		user, err := testHandler.Queries.GetUserByEmail(ctx, email)
		if err == nil {
			workspaces, listErr := testHandler.Queries.ListWorkspaces(ctx, user.ID)
			if listErr == nil {
				for _, workspace := range workspaces {
					_ = testHandler.Queries.DeleteWorkspace(ctx, workspace.ID)
				}
			}
		}
		testPool.Exec(ctx, `DELETE FROM "user" WHERE email = $1`, email)
	})

	// Send code
	w := httptest.NewRecorder()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]string{"email": email})
	req := httptest.NewRequest("POST", "/auth/send-code", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.SendCode(w, req)

	// Read code from DB
	dbCode, err := testHandler.Queries.GetLatestVerificationCode(ctx, email)
	if err != nil {
		t.Fatalf("GetLatestVerificationCode: %v", err)
	}

	// Verify
	w = httptest.NewRecorder()
	buf.Reset()
	json.NewEncoder(&buf).Encode(map[string]string{"email": email, "code": dbCode.Code})
	req = httptest.NewRequest("POST", "/auth/verify-code", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.VerifyCode(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("VerifyCode: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	user, err := testHandler.Queries.GetUserByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}

	workspaces, err := testHandler.Queries.ListWorkspaces(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}
	if len(workspaces) != 1 {
		t.Fatalf("ListWorkspaces: expected 1 workspace, got %d", len(workspaces))
	}
	if !strings.Contains(workspaces[0].Name, "Workspace") {
		t.Fatalf("expected auto-created workspace name, got %q", workspaces[0].Name)
	}
}

func TestPasswordLogin(t *testing.T) {
	const email = "admin-password-test@multica.ai"
	const password = "admin-password-secret"
	ctx := context.Background()

	t.Setenv("SUPER_ADMIN_EMAIL", email)
	t.Setenv("SUPER_ADMIN_PASSWORD", password)

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM verification_code WHERE email = $1`, email)
		user, err := testHandler.Queries.GetUserByEmail(ctx, email)
		if err == nil {
			workspaces, listErr := testHandler.Queries.ListWorkspaces(ctx, user.ID)
			if listErr == nil {
				for _, workspace := range workspaces {
					_ = testHandler.Queries.DeleteWorkspace(ctx, workspace.ID)
				}
			}
		}
		testPool.Exec(ctx, `DELETE FROM "user" WHERE email = $1`, email)
	})

	w := httptest.NewRecorder()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]string{
		"email":    email,
		"password": password,
	})
	req := httptest.NewRequest("POST", "/auth/password-login", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.PasswordLogin(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PasswordLogin: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp LoginResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Token == "" {
		t.Fatal("PasswordLogin: expected non-empty token")
	}
	if resp.User.Email != email {
		t.Fatalf("PasswordLogin: expected email %q, got %q", email, resp.User.Email)
	}

	user, err := testHandler.Queries.GetUserByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}

	workspaces, err := testHandler.Queries.ListWorkspaces(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}
	if len(workspaces) != 1 {
		t.Fatalf("ListWorkspaces: expected 1 workspace, got %d", len(workspaces))
	}
}

func TestPasswordLoginWrongPassword(t *testing.T) {
	t.Setenv("SUPER_ADMIN_EMAIL", "admin-password-test@multica.ai")
	t.Setenv("SUPER_ADMIN_PASSWORD", "admin-password-secret")

	w := httptest.NewRecorder()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]string{
		"email":    "admin-password-test@multica.ai",
		"password": "wrong-password",
	})
	req := httptest.NewRequest("POST", "/auth/password-login", &buf)
	req.Header.Set("Content-Type", "application/json")
	testHandler.PasswordLogin(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("PasswordLogin (wrong password): expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestResolveActor(t *testing.T) {
	ctx := context.Background()

	// Look up the agent created by the test fixture.
	var agentID string
	err := testPool.QueryRow(ctx,
		`SELECT id FROM agent WHERE workspace_id = $1 AND name = $2`,
		testWorkspaceID, "Handler Test Agent",
	).Scan(&agentID)
	if err != nil {
		t.Fatalf("failed to find test agent: %v", err)
	}

	// Create a task for the agent so we can test X-Task-ID validation.
	var issueID string
	err = testPool.QueryRow(ctx,
		`INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, number, position)
		 VALUES ($1, 'resolveActor test', 'todo', 'none', 'member', $2, 9999, 0)
		 RETURNING id`, testWorkspaceID, testUserID,
	).Scan(&issueID)
	if err != nil {
		t.Fatalf("failed to create test issue: %v", err)
	}

	// Look up runtime_id for the agent.
	var runtimeID string
	err = testPool.QueryRow(ctx, `SELECT runtime_id FROM agent WHERE id = $1`, agentID).Scan(&runtimeID)
	if err != nil {
		t.Fatalf("failed to get agent runtime_id: %v", err)
	}

	var taskID string
	err = testPool.QueryRow(ctx,
		`INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, priority)
		 VALUES ($1, $2, $3, 'queued', 0)
		 RETURNING id`, agentID, runtimeID, issueID,
	).Scan(&taskID)
	if err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM agent_task_queue WHERE id = $1`, taskID)
		testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issueID)
	})

	tests := []struct {
		name          string
		agentIDHeader string
		taskIDHeader  string
		wantActorType string
		wantIsAgent   bool
	}{
		{
			name:          "no headers returns member",
			wantActorType: "member",
		},
		{
			name:          "valid agent ID returns agent",
			agentIDHeader: agentID,
			wantActorType: "agent",
			wantIsAgent:   true,
		},
		{
			name:          "non-existent agent ID returns member",
			agentIDHeader: "00000000-0000-0000-0000-000000000099",
			wantActorType: "member",
		},
		{
			name:          "valid agent + valid task returns agent",
			agentIDHeader: agentID,
			taskIDHeader:  taskID,
			wantActorType: "agent",
			wantIsAgent:   true,
		},
		{
			name:          "valid agent + wrong task returns member",
			agentIDHeader: agentID,
			taskIDHeader:  "00000000-0000-0000-0000-000000000099",
			wantActorType: "member",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newRequest("GET", "/test", nil)
			if tt.agentIDHeader != "" {
				req.Header.Set("X-Agent-ID", tt.agentIDHeader)
			}
			if tt.taskIDHeader != "" {
				req.Header.Set("X-Task-ID", tt.taskIDHeader)
			}

			actorType, actorID := testHandler.resolveActor(req, testUserID, testWorkspaceID)

			if actorType != tt.wantActorType {
				t.Errorf("actorType = %q, want %q", actorType, tt.wantActorType)
			}
			if tt.wantIsAgent {
				if actorID != tt.agentIDHeader {
					t.Errorf("actorID = %q, want agent %q", actorID, tt.agentIDHeader)
				}
			} else {
				if actorID != testUserID {
					t.Errorf("actorID = %q, want user %q", actorID, testUserID)
				}
			}
		})
	}
}

func TestDaemonRegisterMissingWorkspaceReturns404(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/daemon/register", bytes.NewBufferString(`{
		"workspace_id":"00000000-0000-0000-0000-000000000001",
		"daemon_id":"local-daemon",
		"device_name":"test-machine",
		"runtimes":[{"name":"Local Codex","type":"codex","version":"1.0.0","status":"online"}]
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)

	testHandler.DaemonRegister(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("DaemonRegister: expected 404, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "workspace not found") {
		t.Fatalf("DaemonRegister: expected workspace not found error, got %s", w.Body.String())
	}
}
