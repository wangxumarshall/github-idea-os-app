package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIdeaOSSettingsRoundTrip(t *testing.T) {
	svc := NewGitHubIdeaOSService()

	raw, publicCfg, err := svc.UpdateSettings(nil, IdeaOSConfigUpdate{
		RepoURL:         "https://github.com/example/ideas",
		Branch:          "main",
		Directory:       "ideas",
		RepoVisibility:  "private",
		DefaultAgentIDs: &[]string{"agent-1", "agent-2"},
	})
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	if publicCfg.RepoVisibility != "private" {
		t.Fatalf("expected repo visibility private, got %q", publicCfg.RepoVisibility)
	}
	if len(publicCfg.DefaultAgentIDs) != 2 || publicCfg.DefaultAgentIDs[0] != "agent-1" || publicCfg.DefaultAgentIDs[1] != "agent-2" {
		t.Fatalf("expected default agent IDs to round-trip, got %#v", publicCfg.DefaultAgentIDs)
	}

	cfg := IdeaOSConfigFromSettings(raw)
	if cfg.RepoURL != "https://github.com/example/ideas" {
		t.Fatalf("expected repo URL to round-trip, got %q", cfg.RepoURL)
	}
	if len(cfg.DefaultAgentIDs) != 2 || cfg.DefaultAgentIDs[0] != "agent-1" || cfg.DefaultAgentIDs[1] != "agent-2" {
		t.Fatalf("expected default agent IDs in config, got %#v", cfg.DefaultAgentIDs)
	}

	sanitized := svc.SanitizeSettings(raw)
	settings, ok := sanitized.(map[string]any)
	if !ok {
		t.Fatalf("expected sanitized settings map, got %T", sanitized)
	}
	idea, ok := settings["ideaos"].(map[string]any)
	if !ok {
		t.Fatal("expected ideaos section in sanitized settings")
	}
	if _, exists := idea["github_token"]; exists {
		t.Fatal("expected github_token to be removed from sanitized settings")
	}
	if idea["repo_visibility"] != "private" {
		t.Fatalf("expected repo_visibility=private, got %#v", idea["repo_visibility"])
	}
	defaultAgentIDs, ok := idea["default_agent_ids"].([]string)
	if !ok || len(defaultAgentIDs) != 2 {
		t.Fatalf("expected sanitized default_agent_ids, got %#v", idea["default_agent_ids"])
	}
}

func TestRenderAndParseIdeaDocument(t *testing.T) {
	rendered, err := RenderIdeaDocument(IdeaDocument{
		IdeaSummary: IdeaSummary{
			Slug:      "ai-pr-review",
			Title:     "AI PR Review",
			CreatedAt: "2026-04-08",
		},
		Content: "做一个 PR review agent，分析架构风险。\n\n- 解析 AST\n",
	})
	if err != nil {
		t.Fatalf("RenderIdeaDocument: %v", err)
	}

	if !strings.Contains(rendered, "title: AI PR Review") {
		t.Fatalf("expected title in frontmatter, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "tags:") {
		t.Fatalf("expected tags in frontmatter, got:\n%s", rendered)
	}

	doc, err := ParseIdeaDocument("ideas/ai-pr-review.md", "sha-1", rendered)
	if err != nil {
		t.Fatalf("ParseIdeaDocument: %v", err)
	}
	if doc.Title != "AI PR Review" {
		t.Fatalf("expected title to round-trip, got %q", doc.Title)
	}
	if doc.Slug != "ai-pr-review" {
		t.Fatalf("expected slug ai-pr-review, got %q", doc.Slug)
	}
	if doc.SHA != "sha-1" {
		t.Fatalf("expected SHA to round-trip, got %q", doc.SHA)
	}
	if len(doc.Tags) == 0 {
		t.Fatal("expected inferred tags")
	}
}

func TestGitHubIdeaOSServiceCRUD(t *testing.T) {
	files := map[string]string{
		"ideas/repo-brain/repo-brain.md": frontmatterFixture("Repo Brain", "2026-04-01", "2026-04-07", []string{"github", "devtools"}, "# Notes\n\nTrack repository knowledge."),
	}
	shas := map[string]string{
		"ideas/repo-brain/repo-brain.md": "sha-repo-brain",
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ghp_test" {
			http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
			return
		}

		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/contents/ideas"):
			writeJSONFixture(w, []map[string]any{
				{"name": "repo-brain", "path": "ideas/repo-brain", "type": "dir", "sha": "sha-dir"},
			})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/contents/ideas/repo-brain"):
			writeJSONFixture(w, []map[string]any{
				{"name": "repo-brain.md", "path": "ideas/repo-brain/repo-brain.md", "type": "file", "sha": shas["ideas/repo-brain/repo-brain.md"]},
			})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/contents/ideas/repo-brain/repo-brain.md"):
			writeJSONFixture(w, map[string]any{
				"name":     "repo-brain.md",
				"path":     "ideas/repo-brain/repo-brain.md",
				"sha":      shas["ideas/repo-brain/repo-brain.md"],
				"encoding": "base64",
				"content":  base64.StdEncoding.EncodeToString([]byte(files["ideas/repo-brain/repo-brain.md"])),
			})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/contents/ideas/new-idea/new-idea.md"):
			var req githubPutContentRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode create request: %v", err)
			}
			content, _ := base64.StdEncoding.DecodeString(req.Content)
			files["ideas/new-idea/new-idea.md"] = string(content)
			shas["ideas/new-idea/new-idea.md"] = "sha-new"
			writeJSONFixture(w, map[string]any{
				"content": map[string]any{
					"name": "new-idea.md",
					"path": "ideas/new-idea/new-idea.md",
					"sha":  "sha-new",
				},
			})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/contents/ideas/new-idea/new-idea.md"):
			writeJSONFixture(w, map[string]any{
				"name":     "new-idea.md",
				"path":     "ideas/new-idea/new-idea.md",
				"sha":      shas["ideas/new-idea/new-idea.md"],
				"encoding": "base64",
				"content":  base64.StdEncoding.EncodeToString([]byte(files["ideas/new-idea/new-idea.md"])),
			})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/contents/ideas/repo-brain/repo-brain.md"):
			var req githubPutContentRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode update request: %v", err)
			}
			content, _ := base64.StdEncoding.DecodeString(req.Content)
			files["ideas/repo-brain/repo-brain.md"] = string(content)
			shas["ideas/repo-brain/repo-brain.md"] = "sha-updated"
			writeJSONFixture(w, map[string]any{
				"content": map[string]any{
					"name": "repo-brain.md",
					"path": "ideas/repo-brain/repo-brain.md",
					"sha":  "sha-updated",
				},
			})
		default:
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		}
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	svc := &GitHubIdeaOSService{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}
	cfg := IdeaOSConfig{
		RepoURL:     "https://github.com/example/ideas",
		Branch:      "main",
		Directory:   "ideas",
		GitHubToken: "ghp_test",
	}

	ideas, err := svc.ListIdeas(context.Background(), cfg)
	if err != nil {
		t.Fatalf("ListIdeas: %v", err)
	}
	if len(ideas) != 1 || ideas[0].Slug != "repo-brain" {
		t.Fatalf("unexpected ideas list: %#v", ideas)
	}

	created, err := svc.CreateIdea(context.Background(), cfg, "New Idea")
	if err != nil {
		t.Fatalf("CreateIdea: %v", err)
	}
	if created.Slug != "new-idea" {
		t.Fatalf("expected new slug, got %q", created.Slug)
	}

	updated, err := svc.UpdateIdea(context.Background(), cfg, UpdateIdeaInput{
		Slug:          "repo-brain",
		Title:         "Repo Brain",
		Content:       "# Notes\n\nUpdated content.",
		BaseTitle:     "Repo Brain",
		BaseContent:   "# Notes\n\nTrack repository knowledge.\n\n# Project Repo\n\n- Status: creating\n",
		Tags:          []string{"github", "devtools"},
		BaseTags:      []string{"github", "devtools"},
		CreatedAt:     "2026-04-01",
		BaseCreatedAt: "2026-04-01",
		SHA:           "sha-repo-brain",
	})
	if err != nil {
		t.Fatalf("UpdateIdea: %v", err)
	}
	if updated.SHA != "sha-updated" {
		t.Fatalf("expected updated sha, got %q", updated.SHA)
	}
	if !strings.Contains(updated.Content, "Updated content.") {
		t.Fatalf("expected updated content, got %q", updated.Content)
	}
}

func TestGitHubIdeaOSServiceUpdateIdeaRetriesManagedRepoConflict(t *testing.T) {
	const filePath = "ideas/repo-brain/repo-brain.md"

	baseDoc := IdeaDocument{
		IdeaSummary: IdeaSummary{
			Code:              "idea0001",
			Slug:              "repo-brain",
			Title:             "Repo Brain",
			CreatedAt:         "2026-04-01",
			ProjectRepoStatus: "creating",
		},
		Content: "# Notes\n\nInitial content.",
	}
	baseMarkdown, err := RenderIdeaDocument(baseDoc)
	if err != nil {
		t.Fatalf("RenderIdeaDocument(base): %v", err)
	}
	baseParsed, err := ParseIdeaDocument(filePath, "sha-stale", baseMarkdown)
	if err != nil {
		t.Fatalf("ParseIdeaDocument(base): %v", err)
	}

	latestDoc := baseDoc
	latestDoc.ProjectRepoURL = "https://github.com/example/repo-brain"
	latestDoc.ProjectRepoStatus = "ready"
	latestMarkdown, err := RenderIdeaDocument(latestDoc)
	if err != nil {
		t.Fatalf("RenderIdeaDocument(latest): %v", err)
	}

	files := map[string]string{filePath: latestMarkdown}
	shas := map[string]string{filePath: "sha-ready"}
	conflicted := false

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/contents/"+filePath):
			writeJSONFixture(w, map[string]any{
				"name":     "repo-brain.md",
				"path":     filePath,
				"sha":      shas[filePath],
				"encoding": "base64",
				"content":  base64.StdEncoding.EncodeToString([]byte(files[filePath])),
			})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/contents/"+filePath):
			var req githubPutContentRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode update request: %v", err)
			}
			if !conflicted {
				conflicted = true
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"message": filePath + " does not match sha-stale",
				})
				return
			}
			if req.SHA != "sha-ready" {
				t.Fatalf("expected retry to use latest sha, got %q", req.SHA)
			}
			content, _ := base64.StdEncoding.DecodeString(req.Content)
			files[filePath] = string(content)
			shas[filePath] = "sha-updated"
			writeJSONFixture(w, map[string]any{
				"content": map[string]any{
					"name": "repo-brain.md",
					"path": filePath,
					"sha":  "sha-updated",
				},
			})
		default:
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		}
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	svc := &GitHubIdeaOSService{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}
	cfg := IdeaOSConfig{
		RepoURL:     "https://github.com/example/ideas",
		Branch:      "main",
		Directory:   "ideas",
		GitHubToken: "ghp_test",
	}

	updated, err := svc.UpdateIdea(context.Background(), cfg, UpdateIdeaInput{
		Slug:              "repo-brain",
		Code:              "idea0001",
		Title:             "Repo Brain",
		Content:           strings.Replace(baseParsed.Content, "Initial content.", "Updated content.", 1),
		BaseTitle:         "Repo Brain",
		BaseContent:       baseParsed.Content,
		Tags:              baseParsed.Tags,
		BaseTags:          baseParsed.Tags,
		CreatedAt:         "2026-04-01",
		BaseCreatedAt:     "2026-04-01",
		SHA:               "sha-stale",
		ProjectRepoURL:    "",
		ProjectRepoStatus: "creating",
	})
	if err != nil {
		t.Fatalf("UpdateIdea: %v", err)
	}

	if updated.SHA != "sha-updated" {
		t.Fatalf("expected updated sha, got %q", updated.SHA)
	}
	if !strings.Contains(files[filePath], "Updated content.") {
		t.Fatalf("expected saved markdown to include updated content, got:\n%s", files[filePath])
	}
	if !strings.Contains(files[filePath], "project_repo_status: ready") {
		t.Fatalf("expected saved markdown to preserve ready repo status, got:\n%s", files[filePath])
	}
	if !strings.Contains(files[filePath], "- Repository: https://github.com/example/repo-brain") {
		t.Fatalf("expected saved markdown to preserve repo link, got:\n%s", files[filePath])
	}
}

func TestGitHubIdeaOSServiceUpdateIdeaReturnsConflictOnUserEdit(t *testing.T) {
	const filePath = "ideas/repo-brain/repo-brain.md"

	baseDoc := IdeaDocument{
		IdeaSummary: IdeaSummary{
			Slug:      "repo-brain",
			Title:     "Repo Brain",
			CreatedAt: "2026-04-01",
		},
		Content: "# Notes\n\nInitial content.",
	}
	baseMarkdown, err := RenderIdeaDocument(baseDoc)
	if err != nil {
		t.Fatalf("RenderIdeaDocument(base): %v", err)
	}
	baseParsed, err := ParseIdeaDocument(filePath, "sha-stale", baseMarkdown)
	if err != nil {
		t.Fatalf("ParseIdeaDocument(base): %v", err)
	}

	remoteDoc := baseDoc
	remoteDoc.Content = "# Notes\n\nRemote edit."
	remoteMarkdown, err := RenderIdeaDocument(remoteDoc)
	if err != nil {
		t.Fatalf("RenderIdeaDocument(remote): %v", err)
	}

	files := map[string]string{filePath: remoteMarkdown}
	shas := map[string]string{filePath: "sha-remote"}
	conflicted := false

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/contents/"+filePath):
			writeJSONFixture(w, map[string]any{
				"name":     "repo-brain.md",
				"path":     filePath,
				"sha":      shas[filePath],
				"encoding": "base64",
				"content":  base64.StdEncoding.EncodeToString([]byte(files[filePath])),
			})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/contents/"+filePath):
			var req githubPutContentRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode update request: %v", err)
			}
			if !conflicted {
				conflicted = true
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"message": filePath + " does not match sha-stale",
				})
				return
			}
			t.Fatal("expected conflicting update not to retry put")
		default:
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		}
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	svc := &GitHubIdeaOSService{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}
	cfg := IdeaOSConfig{
		RepoURL:     "https://github.com/example/ideas",
		Branch:      "main",
		Directory:   "ideas",
		GitHubToken: "ghp_test",
	}

	_, err = svc.UpdateIdea(context.Background(), cfg, UpdateIdeaInput{
		Slug:          "repo-brain",
		Title:         "Repo Brain",
		Content:       "# Notes\n\nMy local edit.",
		BaseTitle:     "Repo Brain",
		BaseContent:   baseParsed.Content,
		Tags:          baseParsed.Tags,
		BaseTags:      baseParsed.Tags,
		CreatedAt:     "2026-04-01",
		BaseCreatedAt: "2026-04-01",
		SHA:           "sha-stale",
	})
	if err == nil {
		t.Fatal("expected conflict error")
	}

	var conflictErr *IdeaConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected IdeaConflictError, got %T: %v", err, err)
	}
}

func TestSyncProjectRepoSpecWritesRootAgentsFile(t *testing.T) {
	const expectedMarkdown = "# Idea Delivery\n\nUse this repository for implementation.\n"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/contents/AGENTS.md"):
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/contents/AGENTS.md"):
			var req githubPutContentRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode put request: %v", err)
			}
			content, err := base64.StdEncoding.DecodeString(req.Content)
			if err != nil {
				t.Fatalf("decode content: %v", err)
			}
			if string(content) != expectedMarkdown {
				t.Fatalf("expected AGENTS.md content %q, got %q", expectedMarkdown, string(content))
			}
			writeJSONFixture(w, map[string]any{
				"content": map[string]any{
					"name": "AGENTS.md",
					"path": "AGENTS.md",
					"sha":  "sha-agents",
				},
			})
		default:
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		}
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	svc := &GitHubIdeaOSService{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	sha, err := svc.SyncProjectRepoSpec(
		context.Background(),
		"ghp_test",
		"https://github.com/example/repo-brain",
		expectedMarkdown,
		"",
		"sync idea AGENTS: idea0001-repo-brain",
	)
	if err != nil {
		t.Fatalf("SyncProjectRepoSpec: %v", err)
	}
	if sha != "sha-agents" {
		t.Fatalf("expected sha-agents, got %q", sha)
	}
}

func TestSyncProjectRepoSpecReturnsConflictWhenAgentsFileChanged(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/contents/AGENTS.md"):
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"message": "AGENTS.md does not match sha-stale",
			})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/contents/AGENTS.md"):
			writeJSONFixture(w, map[string]any{
				"name":     "AGENTS.md",
				"path":     "AGENTS.md",
				"sha":      "sha-latest",
				"encoding": "base64",
				"content":  base64.StdEncoding.EncodeToString([]byte("# Manual instructions\n\nDo not overwrite.\n")),
			})
		default:
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		}
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	svc := &GitHubIdeaOSService{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	_, err := svc.SyncProjectRepoSpec(
		context.Background(),
		"ghp_test",
		"https://github.com/example/repo-brain",
		"# Idea Delivery\n\nSync this file.\n",
		"sha-stale",
		"sync idea AGENTS: idea0001-repo-brain",
	)
	if err == nil {
		t.Fatal("expected conflict error")
	}

	var conflictErr *IdeaProjectSpecConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected IdeaProjectSpecConflictError, got %T: %v", err, err)
	}
}

func frontmatterFixture(title, created, updated string, tags []string, body string) string {
	doc, _ := RenderIdeaDocument(IdeaDocument{
		IdeaSummary: IdeaSummary{
			Slug:      normalizeIdeaSlug(title),
			Title:     title,
			CreatedAt: created,
			UpdatedAt: updated,
			Tags:      tags,
		},
		Content: body,
	})
	return doc
}

func writeJSONFixture(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
