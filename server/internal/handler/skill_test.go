package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setGitHubAPIBaseURL(t *testing.T, base string) {
	t.Helper()
	prev := githubAPIBaseURL
	githubAPIBaseURL = base
	t.Cleanup(func() {
		githubAPIBaseURL = prev
	})
}

func TestDetectImportSourceSupportsGitHub(t *testing.T) {
	source, normalized, err := detectImportSource("github.com/acme/skills-repo/tree/main/skills/code-review")
	if err != nil {
		t.Fatalf("detectImportSource returned error: %v", err)
	}
	if source != sourceGitHub {
		t.Fatalf("expected sourceGitHub, got %v", source)
	}
	if normalized != "https://github.com/acme/skills-repo/tree/main/skills/code-review" {
		t.Fatalf("unexpected normalized URL: %s", normalized)
	}
}

func TestParseGitHubSkillURLTree(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/acme/skills-repo/contents/skills/code-review/SKILL.md" && r.URL.Query().Get("ref") == "main" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(githubContentEntry{
				Name:        "SKILL.md",
				Path:        "skills/code-review/SKILL.md",
				Type:        "file",
				DownloadURL: "http://example.invalid/skill",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	setGitHubAPIBaseURL(t, server.URL)

	loc, err := parseGitHubSkillURL(server.Client(), "https://github.com/acme/skills-repo/tree/main/skills/code-review")
	if err != nil {
		t.Fatalf("parseGitHubSkillURL returned error: %v", err)
	}
	if loc.Owner != "acme" || loc.Repo != "skills-repo" || loc.Ref != "main" || loc.SkillDir != "skills/code-review" {
		t.Fatalf("unexpected location: %+v", loc)
	}
}

func TestParseGitHubSkillURLBlobWithSlashRef(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/acme/skills-repo/contents/skills/code-review/SKILL.md" && r.URL.Query().Get("ref") == "feature/team" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(githubContentEntry{
				Name:        "SKILL.md",
				Path:        "skills/code-review/SKILL.md",
				Type:        "file",
				DownloadURL: "http://example.invalid/skill",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	setGitHubAPIBaseURL(t, server.URL)

	loc, err := parseGitHubSkillURL(server.Client(), "https://github.com/acme/skills-repo/blob/feature/team/skills/code-review/SKILL.md")
	if err != nil {
		t.Fatalf("parseGitHubSkillURL returned error: %v", err)
	}
	if loc.Ref != "feature/team" {
		t.Fatalf("expected ref feature/team, got %s", loc.Ref)
	}
	if loc.SkillDir != "skills/code-review" {
		t.Fatalf("expected skill dir skills/code-review, got %s", loc.SkillDir)
	}
}

func TestFetchFromGitHubURLImportsFiles(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/acme/skills-repo/contents/skills/code-review/SKILL.md" && r.URL.Query().Get("ref") == "main":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(githubContentEntry{
				Name:        "SKILL.md",
				Path:        "skills/code-review/SKILL.md",
				Type:        "file",
				DownloadURL: server.URL + "/raw/skill-md",
			})
		case r.URL.Path == "/repos/acme/skills-repo/contents/skills/code-review" && r.URL.Query().Get("ref") == "main":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]githubContentEntry{
				{Name: "SKILL.md", Path: "skills/code-review/SKILL.md", Type: "file", DownloadURL: server.URL + "/raw/skill-md"},
				{Name: "guide.md", Path: "skills/code-review/guide.md", Type: "file", DownloadURL: server.URL + "/raw/guide-md"},
				{Name: "templates", Path: "skills/code-review/templates", Type: "dir"},
				{Name: "LICENSE", Path: "skills/code-review/LICENSE", Type: "file", DownloadURL: server.URL + "/raw/license"},
			})
		case r.URL.Path == "/repos/acme/skills-repo/contents/skills/code-review/templates" && r.URL.Query().Get("ref") == "main":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]githubContentEntry{
				{Name: "example.go", Path: "skills/code-review/templates/example.go", Type: "file", DownloadURL: server.URL + "/raw/example-go"},
			})
		case r.URL.Path == "/raw/skill-md":
			_, _ = w.Write([]byte("---\nname: Code Review\ndescription: Review changes carefully.\n---\nUse this skill.\n"))
		case r.URL.Path == "/raw/guide-md":
			_, _ = w.Write([]byte("Guide content"))
		case r.URL.Path == "/raw/example-go":
			_, _ = w.Write([]byte("package main"))
		case r.URL.Path == "/raw/license":
			_, _ = w.Write([]byte("ignored"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	setGitHubAPIBaseURL(t, server.URL)

	imported, err := fetchFromGitHubURL(server.Client(), "https://github.com/acme/skills-repo/tree/main/skills/code-review")
	if err != nil {
		t.Fatalf("fetchFromGitHubURL returned error: %v", err)
	}
	if imported.name != "Code Review" {
		t.Fatalf("expected name Code Review, got %s", imported.name)
	}
	if imported.description != "Review changes carefully." {
		t.Fatalf("unexpected description: %s", imported.description)
	}
	if imported.content == "" {
		t.Fatal("expected imported content")
	}
	if len(imported.files) != 2 {
		t.Fatalf("expected 2 supporting files, got %d", len(imported.files))
	}
	got := map[string]string{}
	for _, file := range imported.files {
		got[file.path] = file.content
	}
	if got["guide.md"] != "Guide content" {
		t.Fatalf("unexpected guide.md content: %q", got["guide.md"])
	}
	if got["templates/example.go"] != "package main" {
		t.Fatalf("unexpected templates/example.go content: %q", got["templates/example.go"])
	}
}

func TestFetchFromSkillsShImportsUsingGitHubHelpers(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/acme/skills-repo/contents/skills/code-review/SKILL.md" && r.URL.Query().Get("ref") == "main":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(githubContentEntry{
				Name:        "SKILL.md",
				Path:        "skills/code-review/SKILL.md",
				Type:        "file",
				DownloadURL: server.URL + "/raw/skill-md",
			})
		case r.URL.Path == "/repos/acme/skills-repo/contents/skills/code-review" && r.URL.Query().Get("ref") == "main":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]githubContentEntry{
				{Name: "SKILL.md", Path: "skills/code-review/SKILL.md", Type: "file", DownloadURL: server.URL + "/raw/skill-md"},
				{Name: "guide.md", Path: "skills/code-review/guide.md", Type: "file", DownloadURL: server.URL + "/raw/guide-md"},
			})
		case r.URL.Path == "/raw/skill-md":
			_, _ = w.Write([]byte("---\nname: Code Review\n---\nUse this skill.\n"))
		case r.URL.Path == "/raw/guide-md":
			_, _ = w.Write([]byte("Guide content"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	setGitHubAPIBaseURL(t, server.URL)

	imported, err := fetchFromSkillsSh(server.Client(), "https://skills.sh/acme/skills-repo/code-review")
	if err != nil {
		t.Fatalf("fetchFromSkillsSh returned error: %v", err)
	}
	if imported.name != "Code Review" {
		t.Fatalf("expected name Code Review, got %s", imported.name)
	}
	if len(imported.files) != 1 || imported.files[0].path != "guide.md" {
		t.Fatalf("unexpected imported files: %+v", imported.files)
	}
}

func TestFetchFromGitHubURLRejectsRepoRootLink(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	setGitHubAPIBaseURL(t, server.URL)

	_, err := fetchFromGitHubURL(server.Client(), "https://github.com/acme/skills-repo")
	if err == nil {
		t.Fatal("expected error for repo root link")
	}
	var validationErr *importValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected importValidationError, got %T", err)
	}
}
