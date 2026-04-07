package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type IdeaOSConfigResponse struct {
	service.PublicIdeaOSConfig
	GitHubConnected bool    `json:"github_connected"`
	GitHubLogin     *string `json:"github_login,omitempty"`
}

type UpdateIdeaOSConfigRequest struct {
	RepoURL        *string `json:"repo_url"`
	Branch         *string `json:"branch"`
	Directory      *string `json:"directory"`
	RepoVisibility *string `json:"repo_visibility"`
}

type RecommendIdeaNameRequest struct {
	RawInput string `json:"raw_input"`
}

type RecommendIdeaNameResponse struct {
	NextCode    string                      `json:"next_code"`
	Suggestions []service.IdeaNameSuggestion `json:"suggestions"`
}

type CreateIdeaRequest struct {
	RawInput     string `json:"raw_input"`
	SelectedName string `json:"selected_name"`
}

type UpdateIdeaRequest struct {
	Title     string   `json:"title"`
	Content   string   `json:"content"`
	Tags      []string `json:"tags"`
	CreatedAt string   `json:"created_at"`
	SHA       string   `json:"sha"`
}

func (h *Handler) loadWorkspaceByID(r *http.Request, workspaceID string) (db.Workspace, bool, error) {
	if workspaceID == "" {
		return db.Workspace{}, false, nil
	}
	workspace, err := h.Queries.GetWorkspace(r.Context(), parseUUID(workspaceID))
	if err != nil {
		return db.Workspace{}, false, err
	}
	return workspace, true, nil
}

func (h *Handler) currentGitHubAccount(r *http.Request) (*service.GitHubAccount, error) {
	userID, ok := requireUserID(httptestDiscardWriter{}, r)
	if !ok {
		return nil, fmt.Errorf("user not authenticated")
	}
	account, err := h.GitHubOAuth.GetAccountByUser(r.Context(), h.DB, userID)
	if err != nil {
		return nil, err
	}
	return account, nil
}

func (h *Handler) ideaOSWorkspaceConfig(r *http.Request) (db.Workspace, service.IdeaOSConfig, string, bool) {
	workspaceID := resolveWorkspaceID(r)
	workspace, ok, err := h.loadWorkspaceByID(r, workspaceID)
	if err != nil || !ok {
		return db.Workspace{}, service.IdeaOSConfig{}, "", false
	}
	return workspace, service.IdeaOSConfigFromSettings(workspace.Settings), workspaceID, true
}

func (h *Handler) ideaOSContext(w http.ResponseWriter, r *http.Request, requireGitHub bool) (db.Workspace, service.IdeaOSConfig, *service.GitHubAccount, string, bool) {
	workspace, cfg, workspaceID, ok := h.ideaOSWorkspaceConfig(r)
	if !ok {
		writeError(w, http.StatusNotFound, "workspace not found")
		return db.Workspace{}, service.IdeaOSConfig{}, nil, "", false
	}

	userID, ok := requireUserID(w, r)
	if !ok {
		return db.Workspace{}, service.IdeaOSConfig{}, nil, "", false
	}

	var account *service.GitHubAccount
	if requireGitHub {
		var err error
		account, err = h.GitHubOAuth.GetAccountByUser(r.Context(), h.DB, userID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "GitHub account is not connected")
			return db.Workspace{}, service.IdeaOSConfig{}, nil, "", false
		}
		token, err := h.GitHubOAuth.DecryptAccessToken(account)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load GitHub token")
			return db.Workspace{}, service.IdeaOSConfig{}, nil, "", false
		}
		cfg.GitHubToken = token
	}

	return workspace, cfg, account, workspaceID, true
}

func (h *Handler) GetIdeaOSConfig(w http.ResponseWriter, r *http.Request) {
	workspace, cfg, _, _, ok := h.ideaOSContext(w, r, false)
	if !ok {
		return
	}

	resp := IdeaOSConfigResponse{
		PublicIdeaOSConfig: h.IdeaOS.PublicConfig(cfg),
	}
	userID := requestUserID(r)
	if account, err := h.GitHubOAuth.GetAccountByUser(r.Context(), h.DB, userID); err == nil {
		resp.GitHubConnected = true
		login := account.Login
		resp.GitHubLogin = &login
	}

	_ = workspace
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) UpdateIdeaOSConfig(w http.ResponseWriter, r *http.Request) {
	workspace, _, _, workspaceID, ok := h.ideaOSContext(w, r, false)
	if !ok {
		return
	}

	var req UpdateIdeaOSConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	update := service.IdeaOSConfigUpdate{}
	if req.RepoURL != nil {
		update.RepoURL = strings.TrimSpace(*req.RepoURL)
	}
	if req.Branch != nil {
		update.Branch = strings.TrimSpace(*req.Branch)
	}
	if req.Directory != nil {
		update.Directory = strings.TrimSpace(*req.Directory)
	}
	if req.RepoVisibility != nil {
		update.RepoVisibility = strings.TrimSpace(*req.RepoVisibility)
	}

	settingsJSON, publicConfig, err := h.IdeaOS.UpdateSettings(workspace.Settings, update)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := h.Queries.UpdateWorkspace(r.Context(), db.UpdateWorkspaceParams{
		ID:       parseUUID(workspaceID),
		Settings: settingsJSON,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save IdeaOS config")
		return
	}

	userID := requestUserID(r)
	h.publish("workspace:updated", workspaceID, "member", userID, map[string]any{"workspace": workspaceToResponse(updated)})

	resp := IdeaOSConfigResponse{PublicIdeaOSConfig: publicConfig}
	if account, err := h.GitHubOAuth.GetAccountByUser(r.Context(), h.DB, userID); err == nil {
		resp.GitHubConnected = true
		login := account.Login
		resp.GitHubLogin = &login
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) RecommendIdeaNames(w http.ResponseWriter, r *http.Request) {
	_, _, account, _, ok := h.ideaOSContext(w, r, true)
	if !ok {
		return
	}

	var req RecommendIdeaNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	rawInput := strings.TrimSpace(req.RawInput)
	if rawInput == "" {
		writeError(w, http.StatusBadRequest, "raw_input is required")
		return
	}

	nextSeq, err := h.IdeaStore.PeekNextIdeaSequence(r.Context(), h.DB, account.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to allocate idea sequence")
		return
	}

	writeJSON(w, http.StatusOK, RecommendIdeaNameResponse{
		NextCode:    fmt.Sprintf("idea%04d", nextSeq),
		Suggestions: service.RecommendIdeaNames(rawInput, nextSeq),
	})
}

func (h *Handler) ListIdeas(w http.ResponseWriter, r *http.Request) {
	_, _, _, workspaceID, ok := h.ideaOSContext(w, r, false)
	if !ok {
		return
	}

	ideas, err := h.IdeaStore.ListIdeasByWorkspace(r.Context(), h.DB, workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list ideas")
		return
	}

	resp := make([]service.IdeaSummary, 0, len(ideas))
	for _, idea := range ideas {
		resp = append(resp, service.IdeaSummary{
			Code:              idea.Code,
			Slug:              idea.SlugFull,
			Path:              idea.IdeaPath,
			Title:             idea.Title,
			Summary:           idea.Summary,
			Tags:              idea.Tags,
			ProjectRepoName:   idea.ProjectRepoName,
			ProjectRepoURL:    idea.ProjectRepoURL,
			ProjectRepoStatus: idea.ProjectRepoStatus,
			ProvisioningError: idea.ProvisioningError,
			CreatedAt:         idea.CreatedAt.Format("2006-01-02"),
			UpdatedAt:         idea.UpdatedAt.Format("2006-01-02"),
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) CreateIdea(w http.ResponseWriter, r *http.Request) {
	_, cfg, account, workspaceID, ok := h.ideaOSContext(w, r, true)
	if !ok {
		return
	}
	if strings.TrimSpace(cfg.RepoURL) == "" {
		writeError(w, http.StatusBadRequest, "IdeaOS repository is not configured")
		return
	}

	var req CreateIdeaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rawInput := strings.TrimSpace(req.RawInput)
	name := strings.TrimSpace(req.SelectedName)
	if rawInput == "" || name == "" {
		writeError(w, http.StatusBadRequest, "raw_input and selected_name are required")
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create idea")
		return
	}
	defer tx.Rollback(r.Context())

	seqNo, login, err := h.IdeaStore.NextIdeaSequence(r.Context(), tx, account.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to allocate idea sequence")
		return
	}

	code := fmt.Sprintf("idea%04d", seqNo)
	slugSuffix := service.NormalizeIdeaSlug(name)
	if slugSuffix == "" {
		writeError(w, http.StatusBadRequest, "selected_name must contain letters or numbers")
		return
	}
	slugFull := code + "-" + slugSuffix
	projectRepoURL := fmt.Sprintf("https://github.com/%s/%s", login, slugFull)

	doc := service.IdeaDocument{
		IdeaSummary: service.IdeaSummary{
			Code:              code,
			Slug:              slugFull,
			Path:              service.IdeaPath(cfg, slugFull),
			Title:             name,
			ProjectRepoName:   slugFull,
			ProjectRepoURL:    projectRepoURL,
			ProjectRepoStatus: "creating",
		},
		Content: "# Notes\n\n" + rawInput + "\n",
	}

	rendered, err := service.RenderIdeaDocument(doc)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to render idea markdown")
		return
	}

	parsed, err := service.ParseIdeaDocument(doc.Path, "", rendered)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse rendered idea")
		return
	}

	sha, err := h.IdeaOS.PutMarkdownFile(r.Context(), cfg, doc.Path, rendered, "", fmt.Sprintf("create idea: %s", slugFull))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	userID := requestUserID(r)
	record, err := h.IdeaStore.InsertIdea(r.Context(), tx, service.IdeaRecord{
		WorkspaceID:       workspaceID,
		OwnerUserID:       userID,
		GitHubAccountID:   account.ID,
		SeqNo:             seqNo,
		Code:              code,
		SlugSuffix:        slugSuffix,
		SlugFull:          slugFull,
		Title:             parsed.Title,
		RawInput:          rawInput,
		Summary:           parsed.Summary,
		Tags:              parsed.Tags,
		IdeaPath:          doc.Path,
		MarkdownSHA:       sha,
		ProjectRepoName:   slugFull,
		ProjectRepoURL:    projectRepoURL,
		ProjectRepoStatus: "creating",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to persist idea")
		return
	}

	if err := h.IdeaStore.EnqueueRepoProvisionJob(r.Context(), tx, record.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue repository provisioning")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create idea")
		return
	}

	parsed.SHA = sha
	parsed.Code = code
	parsed.Slug = slugFull
	parsed.Path = doc.Path
	parsed.ProjectRepoName = slugFull
	parsed.ProjectRepoURL = projectRepoURL
	parsed.ProjectRepoStatus = "creating"
	writeJSON(w, http.StatusCreated, parsed)
}

func (h *Handler) GetIdea(w http.ResponseWriter, r *http.Request) {
	_, cfg, _, workspaceID, ok := h.ideaOSContext(w, r, true)
	if !ok {
		return
	}

	record, err := h.IdeaStore.GetIdeaBySlug(r.Context(), h.DB, workspaceID, chi.URLParam(r, "slug"))
	if err != nil {
		writeError(w, http.StatusNotFound, "idea not found")
		return
	}

	content, sha, err := h.IdeaOS.GetMarkdownFile(r.Context(), cfg, record.IdeaPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	doc, err := service.ParseIdeaDocument(record.IdeaPath, sha, content)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse idea markdown")
		return
	}
	doc.Code = record.Code
	doc.ProjectRepoName = record.ProjectRepoName
	doc.ProjectRepoURL = record.ProjectRepoURL
	doc.ProjectRepoStatus = record.ProjectRepoStatus
	doc.ProvisioningError = record.ProvisioningError
	writeJSON(w, http.StatusOK, doc)
}

func (h *Handler) UpdateIdea(w http.ResponseWriter, r *http.Request) {
	_, cfg, _, workspaceID, ok := h.ideaOSContext(w, r, true)
	if !ok {
		return
	}

	record, err := h.IdeaStore.GetIdeaBySlug(r.Context(), h.DB, workspaceID, chi.URLParam(r, "slug"))
	if err != nil {
		writeError(w, http.StatusNotFound, "idea not found")
		return
	}

	var req UpdateIdeaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	doc := service.IdeaDocument{
		IdeaSummary: service.IdeaSummary{
			Code:              record.Code,
			Slug:              record.SlugFull,
			Path:              record.IdeaPath,
			Title:             req.Title,
			Tags:              req.Tags,
			CreatedAt:         req.CreatedAt,
			ProjectRepoName:   record.ProjectRepoName,
			ProjectRepoURL:    record.ProjectRepoURL,
			ProjectRepoStatus: record.ProjectRepoStatus,
		},
		Content: req.Content,
		SHA:     req.SHA,
	}

	rendered, err := service.RenderIdeaDocument(doc)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to render idea markdown")
		return
	}

	parsed, err := service.ParseIdeaDocument(record.IdeaPath, "", rendered)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse idea markdown")
		return
	}

	sha, err := h.IdeaOS.PutMarkdownFile(r.Context(), cfg, record.IdeaPath, rendered, strings.TrimSpace(req.SHA), fmt.Sprintf("update idea: %s", record.SlugFull))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.IdeaStore.UpdateIdeaContent(r.Context(), h.DB, record.ID, parsed.Title, parsed.Summary, sha, parsed.Tags, "", ""); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update idea metadata")
		return
	}

	parsed.SHA = sha
	parsed.Code = record.Code
	parsed.Slug = record.SlugFull
	parsed.Path = record.IdeaPath
	parsed.ProjectRepoName = record.ProjectRepoName
	parsed.ProjectRepoURL = record.ProjectRepoURL
	parsed.ProjectRepoStatus = record.ProjectRepoStatus
	parsed.ProvisioningError = record.ProvisioningError
	writeJSON(w, http.StatusOK, parsed)
}

func (h *Handler) RetryIdeaRepoProvision(w http.ResponseWriter, r *http.Request) {
	_, _, account, workspaceID, ok := h.ideaOSContext(w, r, true)
	if !ok {
		return
	}

	record, err := h.IdeaStore.GetIdeaBySlug(r.Context(), h.DB, workspaceID, chi.URLParam(r, "slug"))
	if err != nil {
		writeError(w, http.StatusNotFound, "idea not found")
		return
	}
	if record.GitHubAccountID != account.ID {
		writeError(w, http.StatusForbidden, "only the connected idea owner can retry repository provisioning")
		return
	}

	if err := h.IdeaStore.RetryRepoProvisionJob(r.Context(), h.DB, record.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retry repository provisioning")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "repository provisioning retried"})
}

type httptestDiscardWriter struct{}

func (httptestDiscardWriter) Header() http.Header       { return http.Header{} }
func (httptestDiscardWriter) Write([]byte) (int, error) { return 0, nil }
func (httptestDiscardWriter) WriteHeader(statusCode int) {}
