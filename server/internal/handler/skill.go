package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// --- Response structs ---

type SkillResponse struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Content     string  `json:"content"`
	Config      any     `json:"config"`
	CreatedBy   *string `json:"created_by"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type SkillFileResponse struct {
	ID        string `json:"id"`
	SkillID   string `json:"skill_id"`
	Path      string `json:"path"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type SkillWithFilesResponse struct {
	SkillResponse
	Files []SkillFileResponse `json:"files"`
}

func skillToResponse(s db.Skill) SkillResponse {
	var config any
	if s.Config != nil {
		json.Unmarshal(s.Config, &config)
	}
	if config == nil {
		config = map[string]any{}
	}

	return SkillResponse{
		ID:          uuidToString(s.ID),
		WorkspaceID: uuidToString(s.WorkspaceID),
		Name:        s.Name,
		Description: s.Description,
		Content:     s.Content,
		Config:      config,
		CreatedBy:   uuidToPtr(s.CreatedBy),
		CreatedAt:   timestampToString(s.CreatedAt),
		UpdatedAt:   timestampToString(s.UpdatedAt),
	}
}

func skillFileToResponse(f db.SkillFile) SkillFileResponse {
	return SkillFileResponse{
		ID:        uuidToString(f.ID),
		SkillID:   uuidToString(f.SkillID),
		Path:      f.Path,
		Content:   f.Content,
		CreatedAt: timestampToString(f.CreatedAt),
		UpdatedAt: timestampToString(f.UpdatedAt),
	}
}

// --- Request structs ---

type CreateSkillRequest struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Content     string                   `json:"content"`
	Config      any                      `json:"config"`
	Files       []CreateSkillFileRequest `json:"files,omitempty"`
}

type CreateSkillFileRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type UpdateSkillRequest struct {
	Name        *string                  `json:"name"`
	Description *string                  `json:"description"`
	Content     *string                  `json:"content"`
	Config      any                      `json:"config"`
	Files       []CreateSkillFileRequest `json:"files,omitempty"`
}

type SetAgentSkillsRequest struct {
	SkillIDs []string `json:"skill_ids"`
}

// --- Helpers ---

// validateFilePath checks that a file path is safe (no traversal, no absolute paths).
func validateFilePath(p string) bool {
	if p == "" {
		return false
	}
	if filepath.IsAbs(p) {
		return false
	}
	cleaned := filepath.Clean(p)
	if strings.HasPrefix(cleaned, "..") {
		return false
	}
	return true
}

func (h *Handler) loadSkillForUser(w http.ResponseWriter, r *http.Request, id string) (db.Skill, bool) {
	workspaceID := resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return db.Skill{}, false
	}

	skill, err := h.Queries.GetSkillInWorkspace(r.Context(), db.GetSkillInWorkspaceParams{
		ID:          parseUUID(id),
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "skill not found")
		return skill, false
	}
	return skill, true
}

// --- Skill CRUD ---

func (h *Handler) ListSkills(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)

	skills, err := h.Queries.ListSkillsByWorkspace(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list skills")
		return
	}

	resp := make([]SkillResponse, len(skills))
	for i, s := range skills {
		resp[i] = skillToResponse(s)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) GetSkill(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	skill, ok := h.loadSkillForUser(w, r, id)
	if !ok {
		return
	}

	files, err := h.Queries.ListSkillFiles(r.Context(), skill.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list skill files")
		return
	}

	fileResps := make([]SkillFileResponse, len(files))
	for i, f := range files {
		fileResps[i] = skillFileToResponse(f)
	}

	writeJSON(w, http.StatusOK, SkillWithFilesResponse{
		SkillResponse: skillToResponse(skill),
		Files:         fileResps,
	})
}

func (h *Handler) CreateSkill(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)

	creatorID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var req CreateSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	for _, f := range req.Files {
		if !validateFilePath(f.Path) {
			writeError(w, http.StatusBadRequest, "invalid file path: "+f.Path)
			return
		}
	}

	config, _ := json.Marshal(req.Config)
	if req.Config == nil {
		config = []byte("{}")
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())

	qtx := h.Queries.WithTx(tx)

	skill, err := qtx.CreateSkill(r.Context(), db.CreateSkillParams{
		WorkspaceID: parseUUID(workspaceID),
		Name:        req.Name,
		Description: req.Description,
		Content:     req.Content,
		Config:      config,
		CreatedBy:   parseUUID(creatorID),
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "a skill with this name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create skill: "+err.Error())
		return
	}

	fileResps := make([]SkillFileResponse, 0, len(req.Files))
	for _, f := range req.Files {
		sf, err := qtx.UpsertSkillFile(r.Context(), db.UpsertSkillFileParams{
			SkillID: skill.ID,
			Path:    f.Path,
			Content: f.Content,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create skill file: "+err.Error())
			return
		}
		fileResps = append(fileResps, skillFileToResponse(sf))
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit")
		return
	}

	resp := SkillWithFilesResponse{
		SkillResponse: skillToResponse(skill),
		Files:         fileResps,
	}
	actorType, actorID := h.resolveActor(r, creatorID, workspaceID)
	h.publish(protocol.EventSkillCreated, workspaceID, actorType, actorID, map[string]any{"skill": resp})
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) UpdateSkill(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	skill, ok := h.loadSkillForUser(w, r, id)
	if !ok {
		return
	}
	if _, ok := h.requireWorkspaceRole(w, r, uuidToString(skill.WorkspaceID), "skill not found", "owner", "admin"); !ok {
		return
	}

	var req UpdateSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	for _, f := range req.Files {
		if !validateFilePath(f.Path) {
			writeError(w, http.StatusBadRequest, "invalid file path: "+f.Path)
			return
		}
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())

	qtx := h.Queries.WithTx(tx)

	params := db.UpdateSkillParams{
		ID: parseUUID(id),
	}
	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}
	if req.Content != nil {
		params.Content = pgtype.Text{String: *req.Content, Valid: true}
	}
	if req.Config != nil {
		config, _ := json.Marshal(req.Config)
		params.Config = config
	}

	skill, err = qtx.UpdateSkill(r.Context(), params)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "a skill with this name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update skill: "+err.Error())
		return
	}

	// If files are provided, replace all files.
	var fileResps []SkillFileResponse
	if req.Files != nil {
		if err := qtx.DeleteSkillFilesBySkill(r.Context(), skill.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete old skill files")
			return
		}
		fileResps = make([]SkillFileResponse, 0, len(req.Files))
		for _, f := range req.Files {
			sf, err := qtx.UpsertSkillFile(r.Context(), db.UpsertSkillFileParams{
				SkillID: skill.ID,
				Path:    f.Path,
				Content: f.Content,
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to upsert skill file: "+err.Error())
				return
			}
			fileResps = append(fileResps, skillFileToResponse(sf))
		}
	} else {
		files, _ := qtx.ListSkillFiles(r.Context(), skill.ID)
		fileResps = make([]SkillFileResponse, len(files))
		for i, f := range files {
			fileResps[i] = skillFileToResponse(f)
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit")
		return
	}

	resp := SkillWithFilesResponse{
		SkillResponse: skillToResponse(skill),
		Files:         fileResps,
	}
	wsID := resolveWorkspaceID(r)
	actorType, actorID := h.resolveActor(r, requestUserID(r), wsID)
	h.publish(protocol.EventSkillUpdated, wsID, actorType, actorID, map[string]any{"skill": resp})
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) DeleteSkill(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	skill, ok := h.loadSkillForUser(w, r, id)
	if !ok {
		return
	}
	if _, ok := h.requireWorkspaceRole(w, r, uuidToString(skill.WorkspaceID), "skill not found", "owner", "admin"); !ok {
		return
	}

	if err := h.Queries.DeleteSkill(r.Context(), parseUUID(id)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete skill")
		return
	}
	actorType, actorID := h.resolveActor(r, requestUserID(r), uuidToString(skill.WorkspaceID))
	h.publish(protocol.EventSkillDeleted, uuidToString(skill.WorkspaceID), actorType, actorID, map[string]any{"skill_id": id})
	w.WriteHeader(http.StatusNoContent)
}

// --- Skill import ---

type ImportSkillRequest struct {
	URL string `json:"url"`
}

// importedSkill holds the data extracted from an external source.
type importedSkill struct {
	name        string
	description string
	content     string // SKILL.md body
	files       []importedFile
}

type importedFile struct {
	path    string
	content string
}

type importValidationError struct {
	message string
}

func (e *importValidationError) Error() string {
	return e.message
}

func newImportValidationError(format string, args ...any) error {
	return &importValidationError{message: fmt.Sprintf(format, args...)}
}

// --- ClawHub types ---

type clawhubGetSkillResponse struct {
	Skill         clawhubSkill          `json:"skill"`
	LatestVersion *clawhubLatestVersion `json:"latestVersion"`
}

type clawhubSkill struct {
	Slug        string            `json:"slug"`
	DisplayName string            `json:"displayName"`
	Summary     string            `json:"summary"`
	Tags        map[string]string `json:"tags"`
}

type clawhubLatestVersion struct {
	Version string `json:"version"`
}

type clawhubVersionDetailResponse struct {
	Version clawhubVersionDetail `json:"version"`
}

type clawhubVersionDetail struct {
	Version string             `json:"version"`
	Files   []clawhubFileEntry `json:"files"`
}

type clawhubFileEntry struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// --- GitHub types ---

type githubContentEntry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"` // "file" or "dir"
	DownloadURL string `json:"download_url"`
}

type githubSkillLocation struct {
	Owner    string
	Repo     string
	Ref      string
	SkillDir string
}

// --- URL detection ---

// importSource identifies where a URL points.
type importSource int

const (
	sourceClawHub importSource = iota
	sourceSkillsSh
	sourceGitHub
)

var githubAPIBaseURL = "https://api.github.com"

// detectImportSource determines the source from a URL.
// Returns the source and a normalized URL (with scheme).
func detectImportSource(raw string) (importSource, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, "", fmt.Errorf("empty URL")
	}

	normalized := raw
	if !strings.HasPrefix(normalized, "http://") && !strings.HasPrefix(normalized, "https://") {
		normalized = "https://" + normalized
	}

	parsed, err := url.Parse(normalized)
	if err != nil {
		return 0, "", fmt.Errorf("invalid URL: %w", err)
	}

	host := strings.ToLower(parsed.Hostname())
	switch {
	case host == "skills.sh" || host == "www.skills.sh":
		return sourceSkillsSh, normalized, nil
	case host == "clawhub.ai" || host == "www.clawhub.ai":
		return sourceClawHub, normalized, nil
	case host == "github.com" || host == "www.github.com":
		return sourceGitHub, normalized, nil
	default:
		// If no host (bare slug), default to clawhub
		if !strings.Contains(raw, "/") || !strings.Contains(raw, ".") {
			return sourceClawHub, raw, nil
		}
		return 0, "", fmt.Errorf("unsupported source: %s (supported: clawhub.ai, skills.sh, github.com)", host)
	}
}

// --- ClawHub import ---

// parseClawHubSlug extracts the skill slug from a clawhub.ai URL.
func parseClawHubSlug(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", newImportValidationError("invalid URL: %v", err)
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	// /{owner}/{slug} — take the last segment as the slug
	if len(parts) == 2 {
		return parts[1], nil
	}
	if len(parts) == 1 && parts[0] != "" {
		return parts[0], nil
	}
	// Bare slug (no path)
	if raw == parsed.Host || parsed.Path == "" || parsed.Path == "/" {
		return "", newImportValidationError("missing skill slug in URL")
	}
	return "", newImportValidationError("could not extract skill slug from URL: %s", raw)
}

func fetchFromClawHub(httpClient *http.Client, rawURL string) (*importedSkill, error) {
	slug, err := parseClawHubSlug(rawURL)
	if err != nil {
		return nil, err
	}

	apiBase := "https://clawhub.ai/api/v1"

	// 1. Fetch skill metadata
	skillResp, err := httpClient.Get(apiBase + "/skills/" + url.PathEscape(slug))
	if err != nil {
		return nil, fmt.Errorf("failed to reach ClawHub: %w", err)
	}
	defer skillResp.Body.Close()

	if skillResp.StatusCode == http.StatusNotFound {
		return nil, newImportValidationError("skill not found on ClawHub: %s", slug)
	}
	if skillResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ClawHub returned status %d", skillResp.StatusCode)
	}

	var chResp clawhubGetSkillResponse
	if err := json.NewDecoder(skillResp.Body).Decode(&chResp); err != nil {
		return nil, fmt.Errorf("failed to parse ClawHub response")
	}
	chSkill := chResp.Skill

	// 2. Determine latest version and fetch file list
	latestVersion := ""
	if v, ok := chSkill.Tags["latest"]; ok {
		latestVersion = v
	} else if chResp.LatestVersion != nil {
		latestVersion = chResp.LatestVersion.Version
	}

	var filePaths []string
	if latestVersion != "" {
		vURL := fmt.Sprintf("%s/skills/%s/versions/%s", apiBase, url.PathEscape(slug), url.PathEscape(latestVersion))
		vResp, err := httpClient.Get(vURL)
		if err == nil {
			defer vResp.Body.Close()
			if vResp.StatusCode == http.StatusOK {
				var vDetail clawhubVersionDetailResponse
				if err := json.NewDecoder(vResp.Body).Decode(&vDetail); err == nil {
					for _, f := range vDetail.Version.Files {
						filePaths = append(filePaths, f.Path)
					}
				}
			}
		}
	}

	// 3. Download each file
	result := &importedSkill{
		name:        chSkill.DisplayName,
		description: chSkill.Summary,
	}
	if result.name == "" {
		result.name = slug
	}

	for _, fp := range filePaths {
		fileURL := fmt.Sprintf("%s/skills/%s/file?path=%s", apiBase, url.PathEscape(slug), url.QueryEscape(fp))
		if latestVersion != "" {
			fileURL += "&version=" + url.QueryEscape(latestVersion)
		}
		body, err := fetchRawFile(httpClient, fileURL)
		if err != nil {
			slog.Warn("clawhub import: file download failed", "path", fp, "error", err)
			continue
		}
		if fp == "SKILL.md" {
			result.content = string(body)
		} else {
			result.files = append(result.files, importedFile{path: fp, content: string(body)})
		}
	}

	return result, nil
}

// --- skills.sh import ---

// parseSkillsShParts extracts owner, repo, skill-name from a skills.sh URL.
// URL format: https://skills.sh/{owner}/{repo}/{skill-name}
func parseSkillsShParts(raw string) (owner, repo, skillName string, err error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", "", "", newImportValidationError("invalid URL: %v", err)
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 3 {
		return "", "", "", newImportValidationError("expected URL format: skills.sh/{owner}/{repo}/{skill-name}, got: %s", parsed.Path)
	}
	return parts[0], parts[1], parts[2], nil
}

func splitURLPathSegments(u *url.URL) ([]string, error) {
	rawPath := strings.Trim(u.EscapedPath(), "/")
	if rawPath == "" {
		return nil, nil
	}
	parts := strings.Split(rawPath, "/")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		decoded, err := url.PathUnescape(part)
		if err != nil {
			return nil, err
		}
		segments = append(segments, decoded)
	}
	return segments, nil
}

func escapeGitHubContentPath(p string) string {
	if p == "" {
		return ""
	}
	parts := strings.Split(p, "/")
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		escaped = append(escaped, url.PathEscape(part))
	}
	return strings.Join(escaped, "/")
}

func joinSlashPath(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(part, "/")
		if part == "" {
			continue
		}
		filtered = append(filtered, part)
	}
	return strings.Join(filtered, "/")
}

func githubContentsURL(owner, repo, contentPath string) string {
	base := strings.TrimRight(githubAPIBaseURL, "/")
	urlPath := base + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/contents"
	if contentPath == "" {
		return urlPath
	}
	return urlPath + "/" + escapeGitHubContentPath(contentPath)
}

func fetchGitHubContentsJSON(httpClient *http.Client, owner, repo, ref, contentPath string) ([]byte, int, error) {
	endpoint := githubContentsURL(owner, repo, contentPath)
	if ref != "" {
		endpoint += "?ref=" + url.QueryEscape(ref)
	}
	resp, err := httpClient.Get(endpoint)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func getGitHubFileMetadata(httpClient *http.Client, owner, repo, ref, filePath string) (githubContentEntry, error) {
	body, status, err := fetchGitHubContentsJSON(httpClient, owner, repo, ref, filePath)
	if err != nil {
		return githubContentEntry{}, err
	}
	if status != http.StatusOK {
		return githubContentEntry{}, fmt.Errorf("HTTP %d", status)
	}
	var entry githubContentEntry
	if err := json.Unmarshal(body, &entry); err != nil {
		return githubContentEntry{}, fmt.Errorf("failed to parse GitHub file response")
	}
	if entry.Type != "file" {
		return githubContentEntry{}, fmt.Errorf("expected file, got %s", entry.Type)
	}
	return entry, nil
}

func listGitHubDirectory(httpClient *http.Client, owner, repo, ref, dirPath string) ([]githubContentEntry, error) {
	body, status, err := fetchGitHubContentsJSON(httpClient, owner, repo, ref, dirPath)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", status)
	}
	var entries []githubContentEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub directory response")
	}
	return entries, nil
}

func skillFilePath(skillDir string) string {
	return joinSlashPath(skillDir, "SKILL.md")
}

func githubSkillExistsAtDir(httpClient *http.Client, owner, repo, ref, skillDir string) (bool, error) {
	entry, err := getGitHubFileMetadata(httpClient, owner, repo, ref, skillFilePath(skillDir))
	if err != nil {
		if strings.Contains(err.Error(), "HTTP 404") {
			return false, nil
		}
		return false, err
	}
	return strings.EqualFold(entry.Name, "SKILL.md"), nil
}

func resolveGitHubTreeReference(httpClient *http.Client, owner, repo string, tail []string) (string, string, error) {
	for split := len(tail) - 1; split >= 1; split-- {
		ref := strings.Join(tail[:split], "/")
		skillDir := strings.Join(tail[split:], "/")
		ok, err := githubSkillExistsAtDir(httpClient, owner, repo, ref, skillDir)
		if err != nil {
			return "", "", err
		}
		if ok {
			return ref, skillDir, nil
		}
	}
	return "", "", newImportValidationError("GitHub directory link must point to a skill directory containing SKILL.md")
}

func resolveGitHubBlobReference(httpClient *http.Client, owner, repo string, tail []string) (string, string, error) {
	if len(tail) < 2 {
		return "", "", newImportValidationError("GitHub file link must point to a SKILL.md page")
	}
	if !strings.EqualFold(tail[len(tail)-1], "SKILL.md") {
		return "", "", newImportValidationError("GitHub file link must point to a SKILL.md page")
	}

	pathTail := tail[:len(tail)-1]
	for split := len(pathTail); split >= 1; split-- {
		ref := strings.Join(pathTail[:split], "/")
		skillDir := strings.Join(pathTail[split:], "/")
		entry, err := getGitHubFileMetadata(httpClient, owner, repo, ref, skillFilePath(skillDir))
		if err != nil {
			if strings.Contains(err.Error(), "HTTP 404") {
				continue
			}
			return "", "", err
		}
		if strings.EqualFold(entry.Name, "SKILL.md") {
			return ref, skillDir, nil
		}
	}

	return "", "", newImportValidationError("GitHub file link must point to a SKILL.md page")
}

func parseGitHubSkillURL(httpClient *http.Client, rawURL string) (githubSkillLocation, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return githubSkillLocation{}, newImportValidationError("invalid URL: %v", err)
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "github.com" && host != "www.github.com" {
		return githubSkillLocation{}, newImportValidationError("unsupported GitHub host: %s", parsed.Hostname())
	}

	segments, err := splitURLPathSegments(parsed)
	if err != nil {
		return githubSkillLocation{}, newImportValidationError("invalid GitHub path: %v", err)
	}
	if len(segments) < 3 {
		return githubSkillLocation{}, newImportValidationError("expected a GitHub link to a skill directory or SKILL.md page")
	}

	loc := githubSkillLocation{
		Owner: segments[0],
		Repo:  segments[1],
	}
	tail := segments[3:]
	switch segments[2] {
	case "tree":
		if len(tail) < 2 {
			return githubSkillLocation{}, newImportValidationError("GitHub tree link must include a skill directory path")
		}
		loc.Ref, loc.SkillDir, err = resolveGitHubTreeReference(httpClient, loc.Owner, loc.Repo, tail)
	case "blob":
		loc.Ref, loc.SkillDir, err = resolveGitHubBlobReference(httpClient, loc.Owner, loc.Repo, tail)
	default:
		return githubSkillLocation{}, newImportValidationError("expected a GitHub /tree/... directory link or /blob/.../SKILL.md file link")
	}
	if err != nil {
		return githubSkillLocation{}, err
	}
	return loc, nil
}

func collectGitHubFiles(httpClient *http.Client, owner, repo, ref, dirPath string, out *[]githubContentEntry) {
	entries, err := listGitHubDirectory(httpClient, owner, repo, ref, dirPath)
	if err != nil {
		return
	}
	for _, entry := range entries {
		lower := strings.ToLower(entry.Name)
		if lower == "skill.md" || lower == "license" || lower == "license.txt" || lower == "license.md" {
			continue
		}
		if entry.Type == "file" {
			*out = append(*out, entry)
			continue
		}
		if entry.Type == "dir" {
			collectGitHubFiles(httpClient, owner, repo, ref, entry.Path, out)
		}
	}
}

func fetchGitHubSkillLocation(httpClient *http.Client, loc githubSkillLocation) (*importedSkill, error) {
	skillFile, err := getGitHubFileMetadata(httpClient, loc.Owner, loc.Repo, loc.Ref, skillFilePath(loc.SkillDir))
	if err != nil {
		if strings.Contains(err.Error(), "HTTP 404") {
			return nil, newImportValidationError("SKILL.md not found in repository %s/%s at %s", loc.Owner, loc.Repo, skillFilePath(loc.SkillDir))
		}
		return nil, fmt.Errorf("fetch GitHub skill metadata: %w", err)
	}
	if skillFile.DownloadURL == "" {
		return nil, fmt.Errorf("GitHub did not provide a download URL for %s", skillFile.Path)
	}

	skillMdBody, err := fetchRawFile(httpClient, skillFile.DownloadURL)
	if err != nil {
		return nil, fmt.Errorf("download GitHub SKILL.md: %w", err)
	}

	name, description := parseSkillFrontmatter(string(skillMdBody))
	if name == "" {
		name = path.Base(loc.SkillDir)
		if name == "." || name == "/" || name == "" {
			name = loc.Repo
		}
	}

	result := &importedSkill{
		name:        name,
		description: description,
		content:     string(skillMdBody),
	}

	var allFiles []githubContentEntry
	collectGitHubFiles(httpClient, loc.Owner, loc.Repo, loc.Ref, loc.SkillDir, &allFiles)

	basePath := loc.SkillDir
	if basePath != "" {
		basePath += "/"
	}
	for _, entry := range allFiles {
		if entry.DownloadURL == "" {
			continue
		}
		body, err := fetchRawFile(httpClient, entry.DownloadURL)
		if err != nil {
			slog.Warn("github import: file download failed", "path", entry.Path, "error", err)
			continue
		}
		relPath := entry.Path
		if basePath != "" {
			relPath = strings.TrimPrefix(entry.Path, basePath)
		}
		result.files = append(result.files, importedFile{path: relPath, content: string(body)})
	}

	return result, nil
}

func fetchFromGitHubURL(httpClient *http.Client, rawURL string) (*importedSkill, error) {
	loc, err := parseGitHubSkillURL(httpClient, rawURL)
	if err != nil {
		return nil, err
	}
	return fetchGitHubSkillLocation(httpClient, loc)
}

func fetchFromSkillsSh(httpClient *http.Client, rawURL string) (*importedSkill, error) {
	owner, repo, skillName, err := parseSkillsShParts(rawURL)
	if err != nil {
		return nil, err
	}

	// Skills can be at different paths depending on the repo structure:
	//   skills/{name}/SKILL.md          (most common)
	//   plugin/skills/{name}/SKILL.md   (e.g. microsoft repos)
	//   {name}/SKILL.md                 (skill at repo root level)
	candidatePaths := []string{
		"skills/" + skillName,
		"plugin/skills/" + skillName,
		skillName,
	}

	for _, dir := range candidatePaths {
		ok, err := githubSkillExistsAtDir(httpClient, owner, repo, "main", dir)
		if err == nil {
			if ok {
				return fetchGitHubSkillLocation(httpClient, githubSkillLocation{
					Owner:    owner,
					Repo:     repo,
					Ref:      "main",
					SkillDir: dir,
				})
			}
			continue
		}
		return nil, fmt.Errorf("resolve skills.sh import path: %w", err)
	}
	return nil, newImportValidationError("SKILL.md not found in repository %s/%s for skill %s", owner, repo, skillName)
}

// parseSkillFrontmatter extracts name and description from YAML frontmatter in SKILL.md.
func parseSkillFrontmatter(content string) (name, description string) {
	if !strings.HasPrefix(content, "---") {
		return "", ""
	}
	end := strings.Index(content[3:], "---")
	if end < 0 {
		return "", ""
	}
	frontmatter := content[3 : 3+end]
	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name:") {
			name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
			name = strings.Trim(name, "\"'")
		} else if strings.HasPrefix(line, "description:") {
			description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
			description = strings.Trim(description, "\"'")
		}
	}
	return name, description
}

// --- Shared helpers ---

// fetchRawFile downloads a URL and returns the body bytes. Limit 1MB.
func fetchRawFile(httpClient *http.Client, fileURL string) ([]byte, error) {
	resp, err := httpClient.Get(fileURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// --- Import handler ---

func (h *Handler) ImportSkill(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)

	creatorID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var req ImportSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	source, normalized, err := detectImportSource(req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}

	var imported *importedSkill
	switch source {
	case sourceClawHub:
		imported, err = fetchFromClawHub(httpClient, normalized)
	case sourceSkillsSh:
		imported, err = fetchFromSkillsSh(httpClient, normalized)
	case sourceGitHub:
		imported, err = fetchFromGitHubURL(httpClient, normalized)
	}
	if err != nil {
		var validationErr *importValidationError
		if errors.As(err, &validationErr) {
			writeError(w, http.StatusBadRequest, validationErr.Error())
			return
		}
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	// Create skill in database
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())

	qtx := h.Queries.WithTx(tx)

	skill, err := qtx.CreateSkill(r.Context(), db.CreateSkillParams{
		WorkspaceID: parseUUID(workspaceID),
		Name:        imported.name,
		Description: imported.description,
		Content:     imported.content,
		Config:      []byte("{}"),
		CreatedBy:   parseUUID(creatorID),
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "a skill with this name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create skill: "+err.Error())
		return
	}

	fileResps := make([]SkillFileResponse, 0, len(imported.files))
	for _, f := range imported.files {
		if !validateFilePath(f.path) {
			continue
		}
		sf, err := qtx.UpsertSkillFile(r.Context(), db.UpsertSkillFileParams{
			SkillID: skill.ID,
			Path:    f.path,
			Content: f.content,
		})
		if err != nil {
			continue
		}
		fileResps = append(fileResps, skillFileToResponse(sf))
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit")
		return
	}

	resp := SkillWithFilesResponse{
		SkillResponse: skillToResponse(skill),
		Files:         fileResps,
	}
	actorType, actorID := h.resolveActor(r, creatorID, workspaceID)
	h.publish(protocol.EventSkillCreated, workspaceID, actorType, actorID, map[string]any{"skill": resp})
	writeJSON(w, http.StatusCreated, resp)
}

// --- Skill File endpoints ---

func (h *Handler) ListSkillFiles(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	skill, ok := h.loadSkillForUser(w, r, id)
	if !ok {
		return
	}

	files, err := h.Queries.ListSkillFiles(r.Context(), skill.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list skill files")
		return
	}

	resp := make([]SkillFileResponse, len(files))
	for i, f := range files {
		resp[i] = skillFileToResponse(f)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) UpsertSkillFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	skill, ok := h.loadSkillForUser(w, r, id)
	if !ok {
		return
	}
	if _, ok := h.requireWorkspaceRole(w, r, uuidToString(skill.WorkspaceID), "skill not found", "owner", "admin"); !ok {
		return
	}

	var req CreateSkillFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !validateFilePath(req.Path) {
		writeError(w, http.StatusBadRequest, "invalid file path")
		return
	}

	sf, err := h.Queries.UpsertSkillFile(r.Context(), db.UpsertSkillFileParams{
		SkillID: skill.ID,
		Path:    req.Path,
		Content: req.Content,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to upsert skill file: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, skillFileToResponse(sf))
}

func (h *Handler) DeleteSkillFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	skill, ok := h.loadSkillForUser(w, r, id)
	if !ok {
		return
	}
	if _, ok := h.requireWorkspaceRole(w, r, uuidToString(skill.WorkspaceID), "skill not found", "owner", "admin"); !ok {
		return
	}

	fileID := chi.URLParam(r, "fileId")
	if err := h.Queries.DeleteSkillFile(r.Context(), parseUUID(fileID)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete skill file")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Agent-Skill junction ---

func (h *Handler) ListAgentSkills(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	agent, ok := h.loadAgentForUser(w, r, id)
	if !ok {
		return
	}

	skills, err := h.Queries.ListAgentSkills(r.Context(), agent.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list agent skills")
		return
	}

	resp := make([]SkillResponse, len(skills))
	for i, s := range skills {
		resp[i] = skillToResponse(s)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) SetAgentSkills(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	agent, ok := h.loadAgentForUser(w, r, id)
	if !ok {
		return
	}
	if !h.canManageAgent(w, r, agent) {
		return
	}

	var req SetAgentSkillsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())

	qtx := h.Queries.WithTx(tx)

	if err := qtx.RemoveAllAgentSkills(r.Context(), agent.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear agent skills")
		return
	}

	for _, skillID := range req.SkillIDs {
		if err := qtx.AddAgentSkill(r.Context(), db.AddAgentSkillParams{
			AgentID: agent.ID,
			SkillID: parseUUID(skillID),
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to add agent skill: "+err.Error())
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit")
		return
	}

	// Return the updated skills list.
	skills, err := h.Queries.ListAgentSkills(r.Context(), agent.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list agent skills")
		return
	}

	resp := make([]SkillResponse, len(skills))
	for i, s := range skills {
		resp[i] = skillToResponse(s)
	}
	actorType, actorID := h.resolveActor(r, requestUserID(r), uuidToString(agent.WorkspaceID))
	h.publish(protocol.EventAgentStatus, uuidToString(agent.WorkspaceID), actorType, actorID, map[string]any{"agent_id": uuidToString(agent.ID), "skills": resp})
	writeJSON(w, http.StatusOK, resp)
}
