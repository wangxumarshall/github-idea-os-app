package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
)

func testAgentID(t *testing.T) string {
	t.Helper()

	var agentID string
	if err := testPool.QueryRow(context.Background(), `
		SELECT id::text
		FROM agent
		WHERE workspace_id = $1
		ORDER BY created_at ASC
		LIMIT 1
	`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatalf("load test agent: %v", err)
	}
	return agentID
}

func ensureTestGitHubAccount(t *testing.T, login string) string {
	t.Helper()

	encryptedToken, err := auth.EncryptString("ghp_test")
	if err != nil {
		t.Fatalf("encrypt GitHub token: %v", err)
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
	`, testUserID, int64(1001), login, encryptedToken).Scan(&accountID); err != nil {
		t.Fatalf("upsert GitHub account: %v", err)
	}

	return accountID
}

func TestIdeaOSConfigRoundTripPersistsDefaultAgentIDs(t *testing.T) {
	agentID := testAgentID(t)

	w := httptest.NewRecorder()
	req := newRequest(http.MethodPut, "/api/ideas/config", map[string]any{
		"repo_url":          "https://github.com/test-owner/ideas",
		"default_agent_ids": []string{agentID},
	})
	testHandler.UpdateIdeaOSConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateIdeaOSConfig: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated IdeaOSConfigResponse
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if len(updated.DefaultAgentIDs) != 1 || updated.DefaultAgentIDs[0] != agentID {
		t.Fatalf("expected saved default agent %q, got %#v", agentID, updated.DefaultAgentIDs)
	}

	w = httptest.NewRecorder()
	req = newRequest(http.MethodGet, "/api/ideas/config", nil)
	testHandler.GetIdeaOSConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetIdeaOSConfig: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var fetched IdeaOSConfigResponse
	if err := json.NewDecoder(w.Body).Decode(&fetched); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if len(fetched.DefaultAgentIDs) != 1 || fetched.DefaultAgentIDs[0] != agentID {
		t.Fatalf("expected round-tripped default agent %q, got %#v", agentID, fetched.DefaultAgentIDs)
	}
}

func TestCreateIdeaAssignsConfiguredDefaultAgentToRootIssue(t *testing.T) {
	agentID := testAgentID(t)
	ensureTestGitHubAccount(t, "test-owner")

	w := httptest.NewRecorder()
	req := newRequest(http.MethodPut, "/api/ideas/config", map[string]any{
		"repo_url":          "https://github.com/test-owner/ideas",
		"default_agent_ids": []string{agentID},
	})
	testHandler.UpdateIdeaOSConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateIdeaOSConfig: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || !strings.Contains(r.URL.Path, "/contents/ideas/") {
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
			return
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode GitHub request: %v", err)
		}
		contentValue, _ := body["content"].(string)
		content, err := base64.StdEncoding.DecodeString(contentValue)
		if err != nil {
			t.Fatalf("decode GitHub content: %v", err)
		}
		if !strings.Contains(string(content), "Build a Local Codex bootstrap flow.") {
			t.Fatalf("expected idea markdown to include raw input, got:\n%s", string(content))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": map[string]any{
				"name": "idea.md",
				"path": "ideas/path/idea.md",
				"sha":  "sha-idea",
			},
		})
	}))
	defer github.Close()

	prevBaseURL := testHandler.IdeaOS.BaseURL
	prevClient := testHandler.IdeaOS.HTTPClient
	testHandler.IdeaOS.BaseURL = github.URL
	testHandler.IdeaOS.HTTPClient = github.Client()
	defer func() {
		testHandler.IdeaOS.BaseURL = prevBaseURL
		testHandler.IdeaOS.HTTPClient = prevClient
	}()

	w = httptest.NewRecorder()
	req = newRequest(http.MethodPost, "/api/ideas", map[string]any{
		"raw_input":     "Build a Local Codex bootstrap flow.",
		"selected_name": "Local Codex Bootstrap",
	})
	testHandler.CreateIdea(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIdea: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created service.IdeaDocument
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.RootIssueID == "" {
		t.Fatal("expected root issue to be linked")
	}

	issue, err := testHandler.Queries.GetIssue(context.Background(), parseUUID(created.RootIssueID))
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if !issue.AssigneeType.Valid || issue.AssigneeType.String != "agent" {
		t.Fatalf("expected root issue assignee_type=agent, got %#v", issue.AssigneeType)
	}
	if !issue.AssigneeID.Valid || uuidToString(issue.AssigneeID) != agentID {
		t.Fatalf("expected root issue assignee_id=%q, got %#v", agentID, issue.AssigneeID)
	}
}

func TestDeleteIdeaDeletesEmptyIdea(t *testing.T) {
	accountID := ensureTestGitHubAccount(t, "test-owner-delete")
	if _, err := testPool.Exec(context.Background(), `
		UPDATE workspace
		SET settings = jsonb_build_object(
			'ideaos', jsonb_build_object(
				'repo_url', 'https://github.com/test-owner-delete/ideas',
				'branch', 'main',
				'directory', 'ideas',
				'repo_visibility', 'private',
				'default_agent_ids', '[]'::jsonb
			)
		)
		WHERE id = $1
	`, testWorkspaceID); err != nil {
		t.Fatalf("update workspace settings: %v", err)
	}

	record, err := testHandler.IdeaStore.InsertIdea(context.Background(), testPool, service.IdeaRecord{
		WorkspaceID:       testWorkspaceID,
		OwnerUserID:       testUserID,
		GitHubAccountID:   accountID,
		SeqNo:             9991,
		Code:              "idea9991",
		SlugSuffix:        "delete-empty",
		SlugFull:          "idea9991-delete-empty",
		Title:             "Delete Empty Idea",
		Summary:           "Temp idea",
		Tags:              []string{"tmp"},
		IdeaPath:          "ideas/idea9991-delete-empty.md",
		MarkdownSHA:       "sha-delete-me",
		ProjectRepoName:   "idea9991-delete-empty",
		ProjectRepoURL:    "https://github.com/test-owner-delete/idea9991-delete-empty",
		ProjectRepoStatus: "creating",
	})
	if err != nil {
		t.Fatalf("insert idea: %v", err)
	}

	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || !strings.Contains(r.URL.Path, "/contents/ideas/idea9991-delete-empty.md") {
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer github.Close()

	prevBaseURL := testHandler.IdeaOS.BaseURL
	prevClient := testHandler.IdeaOS.HTTPClient
	testHandler.IdeaOS.BaseURL = github.URL
	testHandler.IdeaOS.HTTPClient = github.Client()
	defer func() {
		testHandler.IdeaOS.BaseURL = prevBaseURL
		testHandler.IdeaOS.HTTPClient = prevClient
	}()

	w := httptest.NewRecorder()
	req := newRequest(http.MethodDelete, "/api/ideas/"+record.SlugFull, nil)
	req = withURLParam(req, "slug", record.SlugFull)
	testHandler.DeleteIdea(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DeleteIdea: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	if _, err := testHandler.IdeaStore.GetIdeaByID(context.Background(), testPool, record.ID); err == nil {
		t.Fatal("expected idea to be deleted")
	}
}

func TestDeleteIdeaRejectsNonEmptyIdea(t *testing.T) {
	accountID := ensureTestGitHubAccount(t, "test-owner-delete-issue")
	record, err := testHandler.IdeaStore.InsertIdea(context.Background(), testPool, service.IdeaRecord{
		WorkspaceID:       testWorkspaceID,
		OwnerUserID:       testUserID,
		GitHubAccountID:   accountID,
		SeqNo:             9992,
		Code:              "idea9992",
		SlugSuffix:        "delete-non-empty",
		SlugFull:          "idea9992-delete-non-empty",
		Title:             "Delete Non Empty Idea",
		Summary:           "Temp idea",
		IdeaPath:          "ideas/idea9992-delete-non-empty.md",
		MarkdownSHA:       "sha-delete-non-empty",
		ProjectRepoName:   "idea9992-delete-non-empty",
		ProjectRepoURL:    "https://github.com/test-owner-delete/idea9992-delete-non-empty",
		ProjectRepoStatus: "creating",
	})
	if err != nil {
		t.Fatalf("insert idea: %v", err)
	}

	if _, err := testPool.Exec(context.Background(), `
		INSERT INTO issue (
			workspace_id, title, description, status, priority,
			assignee_type, assignee_id, creator_type, creator_id,
			parent_issue_id, position, due_date, number, repo_url, idea_id, execution_stage
		) VALUES (
			$1, $2, NULL, 'todo', 'medium',
			NULL, NULL, 'member', $3,
			NULL, 0, NULL, 9992, NULL, $4, 'idle'
		)
	`, testWorkspaceID, "Idea child issue", testUserID, record.ID); err != nil {
		t.Fatalf("insert issue: %v", err)
	}

	w := httptest.NewRecorder()
	req := newRequest(http.MethodDelete, "/api/ideas/"+record.SlugFull, nil)
	req = withURLParam(req, "slug", record.SlugFull)
	testHandler.DeleteIdea(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("DeleteIdea: expected 409, got %d: %s", w.Code, w.Body.String())
	}
}
