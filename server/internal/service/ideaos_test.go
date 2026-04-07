package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIdeaOSSettingsRoundTrip(t *testing.T) {
	svc := NewGitHubIdeaOSService()

	raw, publicCfg, err := svc.UpdateSettings(nil, IdeaOSConfigUpdate{
		RepoURL:        "https://github.com/example/ideas",
		Branch:         "main",
		Directory:      "ideas",
		RepoVisibility: "private",
	})
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	if publicCfg.RepoVisibility != "private" {
		t.Fatalf("expected repo visibility private, got %q", publicCfg.RepoVisibility)
	}

	cfg := IdeaOSConfigFromSettings(raw)
	if cfg.RepoURL != "https://github.com/example/ideas" {
		t.Fatalf("expected repo URL to round-trip, got %q", cfg.RepoURL)
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
		"ideas/repo-brain.md": frontmatterFixture("Repo Brain", "2026-04-01", "2026-04-07", []string{"github", "devtools"}, "# Notes\n\nTrack repository knowledge."),
	}
	shas := map[string]string{
		"ideas/repo-brain.md": "sha-repo-brain",
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ghp_test" {
			http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
			return
		}

		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/contents/ideas"):
			writeJSONFixture(w, []map[string]any{
				{"name": "repo-brain.md", "path": "ideas/repo-brain.md", "type": "file", "sha": shas["ideas/repo-brain.md"]},
			})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/contents/ideas/repo-brain.md"):
			writeJSONFixture(w, map[string]any{
				"name":     "repo-brain.md",
				"path":     "ideas/repo-brain.md",
				"sha":      shas["ideas/repo-brain.md"],
				"encoding": "base64",
				"content":  base64.StdEncoding.EncodeToString([]byte(files["ideas/repo-brain.md"])),
			})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/contents/ideas/new-idea.md"):
			var req githubPutContentRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode create request: %v", err)
			}
			content, _ := base64.StdEncoding.DecodeString(req.Content)
			files["ideas/new-idea.md"] = string(content)
			shas["ideas/new-idea.md"] = "sha-new"
			writeJSONFixture(w, map[string]any{
				"content": map[string]any{
					"name": "new-idea.md",
					"path": "ideas/new-idea.md",
					"sha":  "sha-new",
				},
			})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/contents/ideas/new-idea.md"):
			writeJSONFixture(w, map[string]any{
				"name":     "new-idea.md",
				"path":     "ideas/new-idea.md",
				"sha":      shas["ideas/new-idea.md"],
				"encoding": "base64",
				"content":  base64.StdEncoding.EncodeToString([]byte(files["ideas/new-idea.md"])),
			})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/contents/ideas/repo-brain.md"):
			var req githubPutContentRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode update request: %v", err)
			}
			content, _ := base64.StdEncoding.DecodeString(req.Content)
			files["ideas/repo-brain.md"] = string(content)
			shas["ideas/repo-brain.md"] = "sha-updated"
			writeJSONFixture(w, map[string]any{
				"content": map[string]any{
					"name": "repo-brain.md",
					"path": "ideas/repo-brain.md",
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
		Slug:      "repo-brain",
		Title:     "Repo Brain",
		Content:   "# Notes\n\nUpdated content.",
		CreatedAt: "2026-04-01",
		SHA:       "sha-repo-brain",
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
