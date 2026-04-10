package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultIdeaOSBranch    = "main"
	DefaultIdeaOSDirectory = "ideas"
	DefaultProjectSpecPath = "AGENTS.md"
	LegacyIdeaCode         = "idea0000"
	LegacyIdeaSlugSuffix   = "legacy-backfill"
	defaultGitHubAPIBase   = "https://api.github.com"
)

func IsLegacyIdea(code, slug string) bool {
	return strings.TrimSpace(code) == LegacyIdeaCode &&
		strings.HasSuffix(strings.TrimSpace(slug), LegacyIdeaSlugSuffix)
}

type IdeaOSConfig struct {
	RepoURL         string   `json:"repo_url"`
	Branch          string   `json:"branch"`
	Directory       string   `json:"directory"`
	RepoVisibility  string   `json:"repo_visibility"`
	DefaultAgentIDs []string `json:"default_agent_ids"`
	GitHubToken     string   `json:"-"`
}

type PublicIdeaOSConfig struct {
	RepoURL         string   `json:"repo_url"`
	Branch          string   `json:"branch"`
	Directory       string   `json:"directory"`
	RepoVisibility  string   `json:"repo_visibility"`
	DefaultAgentIDs []string `json:"default_agent_ids"`
}

type IdeaSummary struct {
	ID                   string   `json:"id"`
	Code                 string   `json:"code"`
	Slug                 string   `json:"slug"`
	Path                 string   `json:"path"`
	Title                string   `json:"title"`
	Summary              string   `json:"summary"`
	Tags                 []string `json:"tags"`
	ProjectRepoName      string   `json:"project_repo_name"`
	ProjectRepoURL       string   `json:"project_repo_url"`
	ProjectRepoStatus    string   `json:"project_repo_status"`
	ProvisioningError    string   `json:"provisioning_error,omitempty"`
	RootIssueID          string   `json:"root_issue_id,omitempty"`
	ProjectSpecSyncError string   `json:"project_spec_sync_error,omitempty"`
	CreatedAt            string   `json:"created_at"`
	UpdatedAt            string   `json:"updated_at"`
}

type IdeaDocument struct {
	IdeaSummary
	Content string `json:"content"`
	SHA     string `json:"sha"`
}

type UpdateIdeaInput struct {
	Slug              string   `json:"slug"`
	Code              string   `json:"code"`
	Title             string   `json:"title"`
	Content           string   `json:"content"`
	BaseTitle         string   `json:"base_title"`
	BaseContent       string   `json:"base_content"`
	Tags              []string `json:"tags"`
	BaseTags          []string `json:"base_tags"`
	CreatedAt         string   `json:"created_at"`
	BaseCreatedAt     string   `json:"base_created_at"`
	SHA               string   `json:"sha"`
	ProjectRepoName   string   `json:"project_repo_name"`
	ProjectRepoURL    string   `json:"project_repo_url"`
	ProjectRepoStatus string   `json:"project_repo_status"`
}

type IdeaOSConfigUpdate struct {
	RepoURL         string
	Branch          string
	Directory       string
	RepoVisibility  string
	DefaultAgentIDs *[]string
}

type GitHubIdeaOSService struct {
	BaseURL    string
	HTTPClient *http.Client
}

type ideaFrontmatter struct {
	Code              string   `yaml:"code,omitempty"`
	Title             string   `yaml:"title"`
	Created           string   `yaml:"created"`
	Updated           string   `yaml:"updated"`
	Tags              []string `yaml:"tags"`
	Summary           string   `yaml:"summary,omitempty"`
	ProjectRepo       string   `yaml:"project_repo,omitempty"`
	ProjectRepoStatus string   `yaml:"project_repo_status,omitempty"`
}

type GitHubRepository struct {
	Name          string `json:"name"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	HTMLURL       string `json:"html_url"`
	Owner         struct {
		Login string `json:"login"`
	} `json:"owner"`
}

type GitHubPullRequest struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	State   string `json:"state"`
	Title   string `json:"title"`
}

type githubContentItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
}

type githubFileContent struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	SHA      string `json:"sha"`
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
}

type githubPutContentRequest struct {
	Message string `json:"message"`
	Content string `json:"content"`
	Branch  string `json:"branch,omitempty"`
	SHA     string `json:"sha,omitempty"`
}

type githubPutContentResponse struct {
	Content githubFileContent `json:"content"`
}

type githubErrorResponse struct {
	Message string `json:"message"`
}

type IdeaProjectSpecConflictError struct {
	Message string
}

func (e *IdeaProjectSpecConflictError) Error() string {
	return e.Message
}

type GitHubAPIError struct {
	StatusCode int
	Message    string
}

func (e *GitHubAPIError) Error() string {
	return fmt.Sprintf("GitHub API error: %s", e.Message)
}

type IdeaConflictError struct {
	Message string
}

func (e *IdeaConflictError) Error() string {
	return e.Message
}

type ideaEditableState struct {
	Title     string
	Content   string
	Tags      []string
	CreatedAt string
}

func NewGitHubIdeaOSService() *GitHubIdeaOSService {
	return &GitHubIdeaOSService{
		BaseURL:    defaultGitHubAPIBase,
		HTTPClient: http.DefaultClient,
	}
}

func (s *GitHubIdeaOSService) PublicConfig(cfg IdeaOSConfig) PublicIdeaOSConfig {
	return PublicIdeaOSConfig{
		RepoURL:         strings.TrimSpace(cfg.RepoURL),
		Branch:          normalizeIdeaOSBranch(cfg.Branch),
		Directory:       normalizeIdeaOSDirectory(cfg.Directory),
		RepoVisibility:  normalizeRepoVisibility(cfg.RepoVisibility),
		DefaultAgentIDs: normalizeAgentIDs(cfg.DefaultAgentIDs),
	}
}

func (s *GitHubIdeaOSService) SanitizeSettings(raw []byte) any {
	settings := decodeSettings(raw)
	ideaCfg := IdeaOSConfigFromSettings(raw)
	if len(settings) == 0 && ideaCfg.RepoURL == "" {
		return map[string]any{}
	}

	idea := map[string]any{
		"repo_url":          strings.TrimSpace(ideaCfg.RepoURL),
		"branch":            normalizeIdeaOSBranch(ideaCfg.Branch),
		"directory":         normalizeIdeaOSDirectory(ideaCfg.Directory),
		"repo_visibility":   normalizeRepoVisibility(ideaCfg.RepoVisibility),
		"default_agent_ids": normalizeAgentIDs(ideaCfg.DefaultAgentIDs),
	}

	if ideaCfg.RepoURL == "" {
		delete(settings, "ideaos")
	} else {
		settings["ideaos"] = idea
	}

	return settings
}

func (s *GitHubIdeaOSService) UpdateSettings(raw []byte, update IdeaOSConfigUpdate) ([]byte, PublicIdeaOSConfig, error) {
	settings := decodeSettings(raw)
	merged := IdeaOSConfigFromSettings(raw)

	if strings.TrimSpace(update.RepoURL) != "" {
		merged.RepoURL = strings.TrimSpace(update.RepoURL)
	}
	if strings.TrimSpace(update.Branch) != "" {
		merged.Branch = strings.TrimSpace(update.Branch)
	}
	if strings.TrimSpace(update.Directory) != "" {
		merged.Directory = strings.TrimSpace(update.Directory)
	}
	if strings.TrimSpace(update.RepoVisibility) != "" {
		merged.RepoVisibility = strings.TrimSpace(update.RepoVisibility)
	}
	if update.DefaultAgentIDs != nil {
		merged.DefaultAgentIDs = normalizeAgentIDs(*update.DefaultAgentIDs)
	}
	if strings.TrimSpace(merged.RepoURL) != "" {
		if _, _, err := parseGitHubRepoURL(merged.RepoURL); err != nil {
			return nil, PublicIdeaOSConfig{}, err
		}
	}

	ideaSettings := map[string]any{
		"repo_url":          strings.TrimSpace(merged.RepoURL),
		"branch":            normalizeIdeaOSBranch(merged.Branch),
		"directory":         normalizeIdeaOSDirectory(merged.Directory),
		"repo_visibility":   normalizeRepoVisibility(merged.RepoVisibility),
		"default_agent_ids": normalizeAgentIDs(merged.DefaultAgentIDs),
	}

	if ideaSettings["repo_url"] == "" {
		delete(settings, "ideaos")
	} else {
		settings["ideaos"] = ideaSettings
	}

	nextRaw, err := json.Marshal(settings)
	if err != nil {
		return nil, PublicIdeaOSConfig{}, err
	}

	return nextRaw, s.PublicConfig(merged), nil
}

func IdeaOSConfigFromSettings(raw []byte) IdeaOSConfig {
	settings := decodeSettings(raw)
	ideaRaw, ok := settings["ideaos"].(map[string]any)
	if !ok {
		return IdeaOSConfig{}
	}

	return IdeaOSConfig{
		RepoURL:         stringFromAny(ideaRaw["repo_url"]),
		Branch:          stringFromAny(ideaRaw["branch"]),
		Directory:       stringFromAny(ideaRaw["directory"]),
		RepoVisibility:  stringFromAny(ideaRaw["repo_visibility"]),
		DefaultAgentIDs: stringSliceFromAny(ideaRaw["default_agent_ids"]),
	}
}

func (s *GitHubIdeaOSService) ListIdeas(ctx context.Context, cfg IdeaOSConfig) ([]IdeaSummary, error) {
	if err := validateIdeaOSConfig(cfg); err != nil {
		return nil, err
	}

	items, err := s.listDirectory(ctx, cfg, normalizeIdeaOSDirectory(cfg.Directory))
	if err != nil {
		return nil, err
	}

	ideas := make([]IdeaSummary, 0, len(items))
	for _, item := range items {
		switch {
		case item.Type == "file" && strings.HasSuffix(strings.ToLower(item.Name), ".md"):
			file, err := s.getFileByPath(ctx, cfg, item.Path)
			if err != nil {
				return nil, err
			}

			doc, err := ParseIdeaDocument(file.Path, file.SHA, file.Content)
			if err != nil {
				return nil, err
			}
			ideas = append(ideas, doc.IdeaSummary)
		case item.Type == "dir":
			nested, err := s.listDirectory(ctx, cfg, item.Path)
			if err != nil {
				return nil, err
			}
			for _, nestedItem := range nested {
				if nestedItem.Type != "file" || !strings.HasSuffix(strings.ToLower(nestedItem.Name), ".md") {
					continue
				}
				file, err := s.getFileByPath(ctx, cfg, nestedItem.Path)
				if err != nil {
					return nil, err
				}
				doc, err := ParseIdeaDocument(file.Path, file.SHA, file.Content)
				if err != nil {
					return nil, err
				}
				ideas = append(ideas, doc.IdeaSummary)
			}
		}
	}

	sort.SliceStable(ideas, func(i, j int) bool {
		if ideas[i].UpdatedAt == ideas[j].UpdatedAt {
			return ideas[i].Title < ideas[j].Title
		}
		return ideas[i].UpdatedAt > ideas[j].UpdatedAt
	})

	return ideas, nil
}

func (s *GitHubIdeaOSService) GetIdea(ctx context.Context, cfg IdeaOSConfig, slug string) (IdeaDocument, error) {
	if err := validateIdeaOSConfig(cfg); err != nil {
		return IdeaDocument{}, err
	}
	slug = normalizeIdeaSlug(slug)
	if slug == "" {
		return IdeaDocument{}, fmt.Errorf("idea slug is required")
	}

	file, err := s.getFileByPath(ctx, cfg, ideaPath(cfg, slug))
	if err != nil {
		return IdeaDocument{}, err
	}

	return ParseIdeaDocument(file.Path, file.SHA, file.Content)
}

func (s *GitHubIdeaOSService) CreateIdea(ctx context.Context, cfg IdeaOSConfig, title string) (IdeaDocument, error) {
	if err := validateIdeaOSConfig(cfg); err != nil {
		return IdeaDocument{}, err
	}

	title = strings.TrimSpace(title)
	if title == "" {
		return IdeaDocument{}, fmt.Errorf("title is required")
	}

	slug := normalizeIdeaSlug(title)
	if slug == "" {
		return IdeaDocument{}, fmt.Errorf("title must contain at least one letter or number")
	}

	now := time.Now().Format("2006-01-02")
	doc := IdeaDocument{
		IdeaSummary: IdeaSummary{
			Slug:      slug,
			Path:      ideaPath(cfg, slug),
			Title:     title,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Content: "# Notes\n\n",
	}

	rendered, err := RenderIdeaDocument(doc)
	if err != nil {
		return IdeaDocument{}, err
	}

	putResp, err := s.putFile(ctx, cfg, doc.Path, rendered, "", fmt.Sprintf("create idea: %s", slug))
	if err != nil {
		return IdeaDocument{}, err
	}

	return ParseIdeaDocument(putResp.Content.Path, putResp.Content.SHA, rendered)
}

func (s *GitHubIdeaOSService) UpdateIdea(ctx context.Context, cfg IdeaOSConfig, input UpdateIdeaInput) (IdeaDocument, error) {
	if err := validateIdeaOSConfig(cfg); err != nil {
		return IdeaDocument{}, err
	}

	slug := normalizeIdeaSlug(input.Slug)
	if slug == "" {
		return IdeaDocument{}, fmt.Errorf("idea slug is required")
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = prettifyIdeaSlug(slug)
	}

	baseTitle := strings.TrimSpace(input.BaseTitle)
	if baseTitle == "" {
		baseTitle = title
	}
	baseCreatedAt := normalizeIdeaDate(input.BaseCreatedAt)
	if baseCreatedAt == "" {
		baseCreatedAt = normalizeIdeaDate(input.CreatedAt)
	}
	baseTags := normalizeTags(input.BaseTags)
	if len(baseTags) == 0 && len(input.BaseTags) == 0 {
		baseTags = normalizeTags(input.Tags)
	}
	baseContent := input.BaseContent
	if baseContent == "" {
		baseContent = input.Content
	}

	desired := IdeaDocument{
		IdeaSummary: IdeaSummary{
			Code:              strings.TrimSpace(input.Code),
			Slug:              slug,
			Path:              ideaPath(cfg, slug),
			Title:             title,
			Tags:              normalizeTags(input.Tags),
			ProjectRepoName:   strings.TrimSpace(input.ProjectRepoName),
			ProjectRepoURL:    strings.TrimSpace(input.ProjectRepoURL),
			ProjectRepoStatus: defaultIfEmpty(strings.TrimSpace(input.ProjectRepoStatus), "creating"),
			CreatedAt:         normalizeIdeaDate(input.CreatedAt),
		},
		Content: stripManagedProjectRepoSection(input.Content),
		SHA:     strings.TrimSpace(input.SHA),
	}

	base := IdeaDocument{
		IdeaSummary: IdeaSummary{
			Code:              desired.Code,
			Slug:              slug,
			Path:              desired.Path,
			Title:             baseTitle,
			Tags:              baseTags,
			ProjectRepoName:   desired.ProjectRepoName,
			ProjectRepoURL:    desired.ProjectRepoURL,
			ProjectRepoStatus: desired.ProjectRepoStatus,
			CreatedAt:         baseCreatedAt,
		},
		Content: stripManagedProjectRepoSection(baseContent),
		SHA:     strings.TrimSpace(input.SHA),
	}

	return s.updateIdeaDocument(ctx, cfg, desired, base, fmt.Sprintf("update idea: %s", slug))
}

func (s *GitHubIdeaOSService) updateIdeaDocument(ctx context.Context, cfg IdeaOSConfig, desired, base IdeaDocument, message string) (IdeaDocument, error) {
	rendered, err := RenderIdeaDocument(desired)
	if err != nil {
		return IdeaDocument{}, err
	}

	sha := strings.TrimSpace(desired.SHA)
	for attempt := 0; attempt < 3; attempt++ {
		putResp, err := s.putFile(ctx, cfg, desired.Path, rendered, sha, message)
		if err == nil {
			return ParseIdeaDocument(putResp.Content.Path, putResp.Content.SHA, rendered)
		}
		if !isGitHubSHAMismatch(err) {
			return IdeaDocument{}, err
		}

		latestContent, latestSHA, latestErr := s.GetMarkdownFile(ctx, cfg, desired.Path)
		if latestErr != nil {
			return IdeaDocument{}, latestErr
		}
		latest, latestErr := ParseIdeaDocument(desired.Path, latestSHA, latestContent)
		if latestErr != nil {
			return IdeaDocument{}, latestErr
		}
		if !sameIdeaEditableState(ideaEditableStateFromDoc(latest), ideaEditableStateFromDoc(base)) {
			return IdeaDocument{}, &IdeaConflictError{
				Message: "idea changed on GitHub; reload the page and merge the latest version before saving again",
			}
		}

		desired.ProjectRepoURL = latest.ProjectRepoURL
		desired.ProjectRepoStatus = latest.ProjectRepoStatus
		desired.UpdatedAt = latest.UpdatedAt
		rendered, err = RenderIdeaDocument(desired)
		if err != nil {
			return IdeaDocument{}, err
		}
		sha = latestSHA
	}

	return IdeaDocument{}, &IdeaConflictError{
		Message: "idea changed on GitHub too frequently; reload the page and try again",
	}
}

func ParseIdeaDocument(filePath, sha, markdown string) (IdeaDocument, error) {
	slug := normalizeIdeaSlug(strings.TrimSuffix(path.Base(filePath), ".md"))
	if slug == "" {
		return IdeaDocument{}, fmt.Errorf("invalid idea path")
	}

	fm, body, err := splitIdeaFrontmatter(markdown)
	if err != nil {
		return IdeaDocument{}, err
	}

	title := strings.TrimSpace(fm.Title)
	if title == "" {
		title = prettifyIdeaSlug(slug)
	}

	created := normalizeIdeaDate(fm.Created)
	if created == "" {
		created = time.Now().Format("2006-01-02")
	}

	updated := normalizeIdeaDate(fm.Updated)
	if updated == "" {
		updated = created
	}

	tags := normalizeTags(fm.Tags)
	if len(tags) == 0 {
		tags = inferIdeaTags(title, body)
	}

	summary := strings.TrimSpace(fm.Summary)
	if summary == "" {
		summary = inferIdeaSummary(body)
	}

	return IdeaDocument{
		IdeaSummary: IdeaSummary{
			Code:              strings.TrimSpace(fm.Code),
			Slug:              slug,
			Path:              filePath,
			Title:             title,
			Summary:           summary,
			Tags:              tags,
			ProjectRepoURL:    strings.TrimSpace(fm.ProjectRepo),
			ProjectRepoStatus: defaultIfEmpty(strings.TrimSpace(fm.ProjectRepoStatus), "creating"),
			CreatedAt:         created,
			UpdatedAt:         updated,
		},
		Content: normalizeIdeaBody(body),
		SHA:     sha,
	}, nil
}

func RenderIdeaDocument(doc IdeaDocument) (string, error) {
	title := strings.TrimSpace(doc.Title)
	if title == "" {
		title = prettifyIdeaSlug(doc.Slug)
	}

	body := normalizeIdeaBody(doc.Content)
	created := normalizeIdeaDate(doc.CreatedAt)
	if created == "" {
		created = time.Now().Format("2006-01-02")
	}

	updated := time.Now().Format("2006-01-02")
	tags := normalizeTags(doc.Tags)
	if len(tags) == 0 {
		tags = inferIdeaTags(title, body)
	}

	fm := ideaFrontmatter{
		Code:              strings.TrimSpace(doc.Code),
		Title:             title,
		Created:           created,
		Updated:           updated,
		Tags:              tags,
		Summary:           inferIdeaSummary(body),
		ProjectRepo:       strings.TrimSpace(doc.ProjectRepoURL),
		ProjectRepoStatus: defaultIfEmpty(doc.ProjectRepoStatus, "creating"),
	}

	yamlBytes, err := yaml.Marshal(fm)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("---\n")
	b.Write(yamlBytes)
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimRight(body, "\n"))
	if !strings.Contains(body, "# Project Repo") {
		b.WriteString("\n\n# Project Repo\n\n")
		if strings.TrimSpace(doc.ProjectRepoURL) != "" {
			b.WriteString("- Repository: " + strings.TrimSpace(doc.ProjectRepoURL) + "\n")
		}
		b.WriteString("- Status: " + defaultIfEmpty(doc.ProjectRepoStatus, "creating") + "\n")
	}
	b.WriteString("\n")
	return b.String(), nil
}

func (s *GitHubIdeaOSService) listDirectory(ctx context.Context, cfg IdeaOSConfig, directory string) ([]githubContentItem, error) {
	var items []githubContentItem
	if err := s.doJSON(ctx, http.MethodGet, s.contentsURL(cfg, directory), cfg.GitHubToken, nil, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *GitHubIdeaOSService) getFileByPath(ctx context.Context, cfg IdeaOSConfig, filePath string) (githubFileContent, error) {
	var file githubFileContent
	if err := s.doJSON(ctx, http.MethodGet, s.contentsURL(cfg, filePath), cfg.GitHubToken, nil, &file); err != nil {
		return githubFileContent{}, err
	}
	if file.Encoding != "base64" {
		return githubFileContent{}, fmt.Errorf("unsupported GitHub content encoding: %s", file.Encoding)
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(file.Content, "\n", ""))
	if err != nil {
		return githubFileContent{}, fmt.Errorf("decode GitHub file: %w", err)
	}
	file.Content = string(decoded)
	return file, nil
}

func (s *GitHubIdeaOSService) GetMarkdownFile(ctx context.Context, cfg IdeaOSConfig, filePath string) (string, string, error) {
	file, err := s.getFileByPath(ctx, cfg, filePath)
	if err != nil {
		return "", "", err
	}
	return file.Content, file.SHA, nil
}

func (s *GitHubIdeaOSService) putFile(ctx context.Context, cfg IdeaOSConfig, filePath, markdown, sha, message string) (githubPutContentResponse, error) {
	reqBody := githubPutContentRequest{
		Message: message,
		Content: base64.StdEncoding.EncodeToString([]byte(markdown)),
		Branch:  normalizeIdeaOSBranch(cfg.Branch),
		SHA:     strings.TrimSpace(sha),
	}

	var resp githubPutContentResponse
	if err := s.doJSON(ctx, http.MethodPut, s.contentsURL(cfg, filePath), cfg.GitHubToken, reqBody, &resp); err != nil {
		return githubPutContentResponse{}, err
	}
	return resp, nil
}

func (s *GitHubIdeaOSService) PutMarkdownFile(ctx context.Context, cfg IdeaOSConfig, filePath, markdown, sha, message string) (string, error) {
	resp, err := s.putFile(ctx, cfg, filePath, markdown, sha, message)
	if err != nil {
		return "", err
	}
	return resp.Content.SHA, nil
}

func (s *GitHubIdeaOSService) SyncProjectRepoSpec(ctx context.Context, token, repoURL, markdown, sha, message string) (string, error) {
	cfg := IdeaOSConfig{
		RepoURL:     strings.TrimSpace(repoURL),
		Branch:      DefaultIdeaOSBranch,
		GitHubToken: strings.TrimSpace(token),
	}
	if err := validateIdeaOSConfig(cfg); err != nil {
		return "", err
	}

	filePath := DefaultProjectSpecPath
	desired := normalizeMarkdownForComparison(markdown)
	currentSHA := strings.TrimSpace(sha)

	if currentSHA == "" {
		currentContent, latestSHA, err := s.GetMarkdownFile(ctx, cfg, filePath)
		switch {
		case err == nil:
			if normalizeMarkdownForComparison(currentContent) == desired {
				return latestSHA, nil
			}
			currentSHA = latestSHA
		case !isGitHubNotFound(err):
			return "", err
		}
	}

	nextSHA, err := s.PutMarkdownFile(ctx, cfg, filePath, markdown, currentSHA, message)
	if err == nil {
		return nextSHA, nil
	}
	if !isGitHubSHAMismatch(err) {
		return "", err
	}

	currentContent, latestSHA, latestErr := s.GetMarkdownFile(ctx, cfg, filePath)
	if latestErr != nil {
		return "", latestErr
	}
	if normalizeMarkdownForComparison(currentContent) == desired {
		return latestSHA, nil
	}
	if currentSHA == "" {
		return s.PutMarkdownFile(ctx, cfg, filePath, markdown, latestSHA, message)
	}

	return "", &IdeaProjectSpecConflictError{
		Message: "AGENTS.md changed in the project repository; sync blocked until reconciled",
	}
}

func (s *GitHubIdeaOSService) CreateRepository(ctx context.Context, token, repoName, visibility string) (GitHubRepository, error) {
	reqBody := map[string]any{
		"name":        strings.TrimSpace(repoName),
		"private":     normalizeRepoVisibility(visibility) != "public",
		"auto_init":   true,
		"description": "Project repository provisioned by GitHub IdeaOS",
	}
	var resp GitHubRepository
	if err := s.doJSON(ctx, http.MethodPost, strings.TrimRight(s.BaseURL, "/")+"/user/repos", token, reqBody, &resp); err != nil {
		return GitHubRepository{}, err
	}
	return resp, nil
}

func (s *GitHubIdeaOSService) GetRepository(ctx context.Context, token, repoURL string) (GitHubRepository, error) {
	cfg := IdeaOSConfig{
		RepoURL:     strings.TrimSpace(repoURL),
		Branch:      DefaultIdeaOSBranch,
		GitHubToken: strings.TrimSpace(token),
	}
	if err := validateIdeaOSConfig(cfg); err != nil {
		return GitHubRepository{}, err
	}

	owner, repo, err := parseGitHubRepoURL(cfg.RepoURL)
	if err != nil {
		return GitHubRepository{}, err
	}

	var result GitHubRepository
	if err := s.doJSON(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s", strings.TrimRight(s.BaseURL, "/"), owner, repo), cfg.GitHubToken, nil, &result); err != nil {
		return GitHubRepository{}, err
	}
	return result, nil
}

func (s *GitHubIdeaOSService) FindOpenPullRequestByHead(ctx context.Context, token, repoURL, headOwner, headBranch string) (*GitHubPullRequest, error) {
	repoURL = strings.TrimSpace(repoURL)
	headOwner = strings.TrimSpace(headOwner)
	headBranch = strings.TrimSpace(headBranch)
	if repoURL == "" || headOwner == "" || headBranch == "" {
		return nil, nil
	}

	owner, repo, err := parseGitHubRepoURL(repoURL)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("state", "open")
	query.Set("head", headOwner+":"+headBranch)

	var pulls []GitHubPullRequest
	if err := s.doJSON(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s/pulls?%s", strings.TrimRight(s.BaseURL, "/"), owner, repo, query.Encode()), strings.TrimSpace(token), nil, &pulls); err != nil {
		return nil, err
	}
	if len(pulls) == 0 {
		return nil, nil
	}
	return &pulls[0], nil
}

func (s *GitHubIdeaOSService) CreatePullRequest(ctx context.Context, token, repoURL, headBranch, baseBranch, title, body string) (GitHubPullRequest, error) {
	repoURL = strings.TrimSpace(repoURL)
	headBranch = strings.TrimSpace(headBranch)
	baseBranch = strings.TrimSpace(baseBranch)
	if repoURL == "" || headBranch == "" || baseBranch == "" {
		return GitHubPullRequest{}, fmt.Errorf("repo URL, head branch, and base branch are required")
	}

	owner, repo, err := parseGitHubRepoURL(repoURL)
	if err != nil {
		return GitHubPullRequest{}, err
	}

	reqBody := map[string]any{
		"title": strings.TrimSpace(title),
		"head":  headBranch,
		"base":  baseBranch,
		"body":  strings.TrimSpace(body),
	}
	var pr GitHubPullRequest
	if err := s.doJSON(ctx, http.MethodPost, fmt.Sprintf("%s/repos/%s/%s/pulls", strings.TrimRight(s.BaseURL, "/"), owner, repo), strings.TrimSpace(token), reqBody, &pr); err != nil {
		return GitHubPullRequest{}, err
	}
	return pr, nil
}

func (s *GitHubIdeaOSService) RenameDefaultBranchToMain(ctx context.Context, token, owner, repo, currentBranch string) error {
	currentBranch = strings.TrimSpace(currentBranch)
	if currentBranch == "" || currentBranch == "main" {
		return nil
	}
	body := map[string]any{"new_name": "main"}
	return s.doJSON(ctx, http.MethodPost, fmt.Sprintf("%s/repos/%s/%s/branches/%s/rename", strings.TrimRight(s.BaseURL, "/"), owner, repo, currentBranch), token, body, nil)
}

func (s *GitHubIdeaOSService) doJSON(ctx context.Context, method, rawURL, token string, reqBody any, out any) error {
	var body io.Reader
	if reqBody != nil {
		payload, err := json.Marshal(reqBody)
		if err != nil {
			return err
		}
		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}

	client := s.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var ghErr githubErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&ghErr)
		if ghErr.Message == "" {
			ghErr.Message = resp.Status
		}
		return &GitHubAPIError{
			StatusCode: resp.StatusCode,
			Message:    ghErr.Message,
		}
	}

	if out == nil {
		io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func validateIdeaOSConfig(cfg IdeaOSConfig) error {
	if strings.TrimSpace(cfg.RepoURL) == "" {
		return fmt.Errorf("IdeaOS repository is not configured")
	}
	if strings.TrimSpace(cfg.GitHubToken) == "" {
		return fmt.Errorf("IdeaOS GitHub token is not configured")
	}
	_, _, err := parseGitHubRepoURL(cfg.RepoURL)
	return err
}

func parseGitHubRepoURL(raw string) (string, string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", "", err
	}
	if u.Host != "github.com" {
		return "", "", fmt.Errorf("unsupported repository host: %s", u.Host)
	}
	parts := strings.Split(strings.Trim(strings.TrimSuffix(u.Path, ".git"), "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid GitHub repository URL")
	}
	return parts[0], parts[1], nil
}

func ParseGitHubRepoURL(raw string) (string, string, error) {
	return parseGitHubRepoURL(raw)
}

func (s *GitHubIdeaOSService) contentsURL(cfg IdeaOSConfig, contentPath string) string {
	owner, repo, _ := parseGitHubRepoURL(cfg.RepoURL)
	base := strings.TrimRight(s.BaseURL, "/")
	if base == "" {
		base = defaultGitHubAPIBase
	}
	ref := normalizeIdeaOSBranch(cfg.Branch)
	return fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s", base, owner, repo, strings.Trim(contentPath, "/"), url.QueryEscape(ref))
}

func splitIdeaFrontmatter(markdown string) (ideaFrontmatter, string, error) {
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	if !strings.HasPrefix(markdown, "---\n") {
		return ideaFrontmatter{}, markdown, nil
	}

	rest := strings.TrimPrefix(markdown, "---\n")
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return ideaFrontmatter{}, markdown, nil
	}

	var fm ideaFrontmatter
	if err := yaml.Unmarshal([]byte(rest[:idx]), &fm); err != nil {
		return ideaFrontmatter{}, "", err
	}

	return fm, rest[idx+5:], nil
}

func normalizeIdeaBody(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.TrimSpace(body)
	if body == "" {
		return "# Notes\n"
	}
	return body + "\n"
}

func stripManagedProjectRepoSection(body string) string {
	normalized := normalizeIdeaBody(body)
	trimmed := strings.TrimSuffix(normalized, "\n")

	switch {
	case strings.HasPrefix(trimmed, "# Project Repo\n\n"):
		if isManagedProjectRepoSection(trimmed) {
			return "# Notes\n"
		}
	default:
		idx := strings.LastIndex(trimmed, "\n\n# Project Repo\n\n")
		if idx == -1 {
			return normalized
		}
		section := trimmed[idx+2:]
		if !isManagedProjectRepoSection(section) {
			return normalized
		}
		return normalizeIdeaBody(trimmed[:idx])
	}

	return normalized
}

func isManagedProjectRepoSection(section string) bool {
	lines := strings.Split(section, "\n")
	if len(lines) < 3 {
		return false
	}
	if lines[0] != "# Project Repo" || lines[1] != "" {
		return false
	}

	idx := 2
	if strings.HasPrefix(lines[idx], "- Repository: ") {
		if strings.TrimSpace(strings.TrimPrefix(lines[idx], "- Repository: ")) == "" {
			return false
		}
		idx++
	}
	if idx >= len(lines) {
		return false
	}
	if !strings.HasPrefix(lines[idx], "- Status: ") {
		return false
	}
	if strings.TrimSpace(strings.TrimPrefix(lines[idx], "- Status: ")) == "" {
		return false
	}
	return idx == len(lines)-1
}

func ideaEditableStateFromDoc(doc IdeaDocument) ideaEditableState {
	return ideaEditableState{
		Title:     strings.TrimSpace(doc.Title),
		Content:   stripManagedProjectRepoSection(doc.Content),
		Tags:      normalizeTags(doc.Tags),
		CreatedAt: normalizeIdeaDate(doc.CreatedAt),
	}
}

func sameIdeaEditableState(left, right ideaEditableState) bool {
	if left.Title != right.Title || left.Content != right.Content || left.CreatedAt != right.CreatedAt {
		return false
	}
	if len(left.Tags) != len(right.Tags) {
		return false
	}
	for idx := range left.Tags {
		if left.Tags[idx] != right.Tags[idx] {
			return false
		}
	}
	return true
}

func isGitHubSHAMismatch(err error) bool {
	var ghErr *GitHubAPIError
	if !errors.As(err, &ghErr) {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(ghErr.Message))
	if strings.Contains(message, "does not match") {
		return true
	}
	return ghErr.StatusCode == http.StatusConflict && strings.Contains(message, "sha")
}

func isGitHubNotFound(err error) bool {
	var ghErr *GitHubAPIError
	if !errors.As(err, &ghErr) {
		return false
	}
	return ghErr.StatusCode == http.StatusNotFound
}

func normalizeMarkdownForComparison(markdown string) string {
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	return strings.TrimSpace(markdown)
}

func normalizeIdeaDate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) >= 10 {
		return value[:10]
	}
	return value
}

func inferIdeaSummary(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.TrimPrefix(line, "#")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(line) > 140 {
			return line[:140]
		}
		return line
	}
	return "Idea note"
}

func inferIdeaTags(title, body string) []string {
	text := strings.ToLower(title + "\n" + body)
	tagSet := map[string]struct{}{}
	add := func(tag string) {
		if len(tagSet) < 6 {
			tagSet[tag] = struct{}{}
		}
	}

	keywords := map[string][]string{
		"ai":           {" ai", "llm", "model", "gpt", "anthropic", "openai"},
		"agent":        {"agent", "agents"},
		"github":       {"github", "git "},
		"devtools":     {"developer tool", "devtool", "pr review", "repo", "codebase"},
		"code-review":  {"review", "pull request", "pr "},
		"architecture": {"architecture", "system design", "infra", "design"},
		"startup":      {"startup", "founder", "saas", "business", "market"},
		"product":      {"product", "roadmap", "ux", "feature"},
		"mobile":       {"ios", "android", "mobile", "flutter"},
	}

	for tag, terms := range keywords {
		for _, term := range terms {
			if strings.Contains(text, term) {
				add(tag)
				break
			}
		}
	}

	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(tags))
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		tag = strings.ReplaceAll(tag, " ", "-")
		tag = strings.Trim(tag, "-")
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		normalized = append(normalized, tag)
	}
	sort.Strings(normalized)
	return normalized
}

func normalizeIdeaOSBranch(branch string) string {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return DefaultIdeaOSBranch
	}
	return branch
}

func normalizeIdeaOSDirectory(directory string) string {
	directory = strings.Trim(strings.TrimSpace(directory), "/")
	if directory == "" {
		return DefaultIdeaOSDirectory
	}
	return directory
}

func ideaPath(cfg IdeaOSConfig, slug string) string {
	return path.Join(normalizeIdeaOSDirectory(cfg.Directory), slug, slug+".md")
}

func IdeaPath(cfg IdeaOSConfig, slug string) string {
	return ideaPath(cfg, slug)
}

func normalizeIdeaSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case !lastDash && b.Len() > 0:
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func NormalizeIdeaSlug(value string) string {
	return normalizeIdeaSlug(value)
}

func prettifyIdeaSlug(slug string) string {
	parts := strings.Split(strings.TrimSpace(slug), "-")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func decodeSettings(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}

	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil || settings == nil {
		return map[string]any{}
	}
	return settings
}

func stringFromAny(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func stringSliceFromAny(v any) []string {
	items, ok := v.([]any)
	if !ok {
		return []string{}
	}

	values := make([]string, 0, len(items))
	for _, item := range items {
		value := stringFromAny(item)
		if value == "" {
			continue
		}
		values = append(values, value)
	}
	return normalizeAgentIDs(values)
}

func normalizeAgentIDs(ids []string) []string {
	if len(ids) == 0 {
		return []string{}
	}

	seen := make(map[string]struct{}, len(ids))
	normalized := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}

	if len(normalized) == 0 {
		return []string{}
	}
	return normalized
}

func normalizeRepoVisibility(value string) string {
	if strings.TrimSpace(strings.ToLower(value)) == "public" {
		return "public"
	}
	return "private"
}
