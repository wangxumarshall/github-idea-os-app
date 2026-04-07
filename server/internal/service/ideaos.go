package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
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
	defaultGitHubAPIBase   = "https://api.github.com"
)

type IdeaOSConfig struct {
	RepoURL        string `json:"repo_url"`
	Branch         string `json:"branch"`
	Directory      string `json:"directory"`
	RepoVisibility string `json:"repo_visibility"`
	GitHubToken    string `json:"-"`
}

type PublicIdeaOSConfig struct {
	RepoURL        string `json:"repo_url"`
	Branch         string `json:"branch"`
	Directory      string `json:"directory"`
	RepoVisibility string `json:"repo_visibility"`
}

type IdeaSummary struct {
	Code               string   `json:"code"`
	Slug               string   `json:"slug"`
	Path               string   `json:"path"`
	Title              string   `json:"title"`
	Summary            string   `json:"summary"`
	Tags               []string `json:"tags"`
	ProjectRepoName    string   `json:"project_repo_name"`
	ProjectRepoURL     string   `json:"project_repo_url"`
	ProjectRepoStatus  string   `json:"project_repo_status"`
	ProvisioningError  string   `json:"provisioning_error,omitempty"`
	CreatedAt          string   `json:"created_at"`
	UpdatedAt          string   `json:"updated_at"`
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
	Tags              []string `json:"tags"`
	CreatedAt         string   `json:"created_at"`
	SHA               string   `json:"sha"`
	ProjectRepoName   string   `json:"project_repo_name"`
	ProjectRepoURL    string   `json:"project_repo_url"`
	ProjectRepoStatus string   `json:"project_repo_status"`
}

type IdeaOSConfigUpdate struct {
	RepoURL        string
	Branch         string
	Directory      string
	RepoVisibility string
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

func NewGitHubIdeaOSService() *GitHubIdeaOSService {
	return &GitHubIdeaOSService{
		BaseURL:    defaultGitHubAPIBase,
		HTTPClient: http.DefaultClient,
	}
}

func (s *GitHubIdeaOSService) PublicConfig(cfg IdeaOSConfig) PublicIdeaOSConfig {
	return PublicIdeaOSConfig{
		RepoURL:        strings.TrimSpace(cfg.RepoURL),
		Branch:         normalizeIdeaOSBranch(cfg.Branch),
		Directory:      normalizeIdeaOSDirectory(cfg.Directory),
		RepoVisibility: normalizeRepoVisibility(cfg.RepoVisibility),
	}
}

func (s *GitHubIdeaOSService) SanitizeSettings(raw []byte) any {
	settings := decodeSettings(raw)
	ideaCfg := IdeaOSConfigFromSettings(raw)
	if len(settings) == 0 && ideaCfg.RepoURL == "" {
		return map[string]any{}
	}

	idea := map[string]any{
		"repo_url":        strings.TrimSpace(ideaCfg.RepoURL),
		"branch":          normalizeIdeaOSBranch(ideaCfg.Branch),
		"directory":       normalizeIdeaOSDirectory(ideaCfg.Directory),
		"repo_visibility": normalizeRepoVisibility(ideaCfg.RepoVisibility),
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
	if strings.TrimSpace(merged.RepoURL) != "" {
		if _, _, err := parseGitHubRepoURL(merged.RepoURL); err != nil {
			return nil, PublicIdeaOSConfig{}, err
		}
	}

	ideaSettings := map[string]any{
		"repo_url":        strings.TrimSpace(merged.RepoURL),
		"branch":          normalizeIdeaOSBranch(merged.Branch),
		"directory":       normalizeIdeaOSDirectory(merged.Directory),
		"repo_visibility": normalizeRepoVisibility(merged.RepoVisibility),
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
		RepoURL:        stringFromAny(ideaRaw["repo_url"]),
		Branch:         stringFromAny(ideaRaw["branch"]),
		Directory:      stringFromAny(ideaRaw["directory"]),
		RepoVisibility: stringFromAny(ideaRaw["repo_visibility"]),
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
		if item.Type != "file" || !strings.HasSuffix(strings.ToLower(item.Name), ".md") {
			continue
		}

		file, err := s.getFileByPath(ctx, cfg, item.Path)
		if err != nil {
			return nil, err
		}

		doc, err := ParseIdeaDocument(file.Path, file.SHA, file.Content)
		if err != nil {
			return nil, err
		}
		ideas = append(ideas, doc.IdeaSummary)
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

	doc := IdeaDocument{
		IdeaSummary: IdeaSummary{
			Slug:      slug,
			Path:      ideaPath(cfg, slug),
			Title:     title,
			Tags:      normalizeTags(input.Tags),
			CreatedAt: normalizeIdeaDate(input.CreatedAt),
		},
		Content: input.Content,
		SHA:     strings.TrimSpace(input.SHA),
	}

	rendered, err := RenderIdeaDocument(doc)
	if err != nil {
		return IdeaDocument{}, err
	}

	putResp, err := s.putFile(ctx, cfg, doc.Path, rendered, doc.SHA, fmt.Sprintf("update idea: %s", slug))
	if err != nil {
		return IdeaDocument{}, err
	}

	return ParseIdeaDocument(putResp.Content.Path, putResp.Content.SHA, rendered)
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
			Code:      strings.TrimSpace(fm.Code),
			Slug:      slug,
			Path:      filePath,
			Title:     title,
			Summary:   summary,
			Tags:      tags,
			ProjectRepoURL:    strings.TrimSpace(fm.ProjectRepo),
			ProjectRepoStatus: defaultIfEmpty(strings.TrimSpace(fm.ProjectRepoStatus), "creating"),
			CreatedAt: created,
			UpdatedAt: updated,
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

func (s *GitHubIdeaOSService) CreatePrivateRepository(ctx context.Context, token, repoName string) (GitHubRepository, error) {
	reqBody := map[string]any{
		"name":         strings.TrimSpace(repoName),
		"private":      true,
		"auto_init":    true,
		"description":  "Project repository provisioned by GitHub IdeaOS",
	}
	var resp GitHubRepository
	if err := s.doJSON(ctx, http.MethodPost, strings.TrimRight(s.BaseURL, "/")+"/user/repos", token, reqBody, &resp); err != nil {
		return GitHubRepository{}, err
	}
	return resp, nil
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
		return fmt.Errorf("GitHub API error: %s", ghErr.Message)
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
		"ai":          {" ai", "llm", "model", "gpt", "anthropic", "openai"},
		"agent":       {"agent", "agents"},
		"github":      {"github", "git "},
		"devtools":    {"developer tool", "devtool", "pr review", "repo", "codebase"},
		"code-review": {"review", "pull request", "pr "},
		"architecture": {"architecture", "system design", "infra", "design"},
		"startup":     {"startup", "founder", "saas", "business", "market"},
		"product":     {"product", "roadmap", "ux", "feature"},
		"mobile":      {"ios", "android", "mobile", "flutter"},
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

func normalizeRepoVisibility(value string) string {
	if strings.TrimSpace(strings.ToLower(value)) == "public" {
		return "public"
	}
	return "private"
}
