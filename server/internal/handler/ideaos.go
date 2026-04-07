package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/go-chi/chi/v5"
)

type IdeaOSConfigResponse = service.PublicIdeaOSConfig

type UpdateIdeaOSConfigRequest struct {
	RepoURL          *string `json:"repo_url"`
	Branch           *string `json:"branch"`
	Directory        *string `json:"directory"`
	GitHubToken      *string `json:"github_token"`
	ClearGitHubToken bool    `json:"clear_github_token"`
}

type CreateIdeaRequest struct {
	Title string `json:"title"`
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

func (h *Handler) ideaOSConfigForWorkspace(r *http.Request) (service.IdeaOSConfig, string, bool) {
	workspaceID := resolveWorkspaceID(r)
	workspace, ok, err := h.loadWorkspaceByID(r, workspaceID)
	if err != nil || !ok {
		return service.IdeaOSConfig{}, "", false
	}
	return service.IdeaOSConfigFromSettings(workspace.Settings), workspaceID, true
}

func (h *Handler) GetIdeaOSConfig(w http.ResponseWriter, r *http.Request) {
	cfg, _, ok := h.ideaOSConfigForWorkspace(r)
	if !ok {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}

	writeJSON(w, http.StatusOK, h.IdeaOS.PublicConfig(cfg))
}

func (h *Handler) UpdateIdeaOSConfig(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	workspace, ok, err := h.loadWorkspaceByID(r, workspaceID)
	if err != nil || !ok {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}

	var req UpdateIdeaOSConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	update := service.IdeaOSConfigUpdate{
		ClearStoredToken: req.ClearGitHubToken,
	}
	if req.RepoURL != nil {
		update.RepoURL = strings.TrimSpace(*req.RepoURL)
	}
	if req.Branch != nil {
		update.Branch = strings.TrimSpace(*req.Branch)
	}
	if req.Directory != nil {
		update.Directory = strings.TrimSpace(*req.Directory)
	}
	if req.GitHubToken != nil {
		update.GitHubToken = strings.TrimSpace(*req.GitHubToken)
		update.ReplaceToken = true
	}

	settingsJSON, publicConfig, err := h.IdeaOS.UpdateSettings(workspace.Settings, update)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to update IdeaOS config")
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
	writeJSON(w, http.StatusOK, publicConfig)
}

func (h *Handler) ListIdeas(w http.ResponseWriter, r *http.Request) {
	cfg, _, ok := h.ideaOSConfigForWorkspace(r)
	if !ok {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}

	ideas, err := h.IdeaOS.ListIdeas(r.Context(), cfg)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, ideas)
}

func (h *Handler) CreateIdea(w http.ResponseWriter, r *http.Request) {
	cfg, _, ok := h.ideaOSConfigForWorkspace(r)
	if !ok {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}

	var req CreateIdeaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	idea, err := h.IdeaOS.CreateIdea(r.Context(), cfg, req.Title)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, idea)
}

func (h *Handler) GetIdea(w http.ResponseWriter, r *http.Request) {
	cfg, _, ok := h.ideaOSConfigForWorkspace(r)
	if !ok {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}

	idea, err := h.IdeaOS.GetIdea(r.Context(), cfg, chi.URLParam(r, "slug"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, idea)
}

func (h *Handler) UpdateIdea(w http.ResponseWriter, r *http.Request) {
	cfg, _, ok := h.ideaOSConfigForWorkspace(r)
	if !ok {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}

	var req UpdateIdeaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	idea, err := h.IdeaOS.UpdateIdea(r.Context(), cfg, service.UpdateIdeaInput{
		Slug:      chi.URLParam(r, "slug"),
		Title:     req.Title,
		Content:   req.Content,
		Tags:      req.Tags,
		CreatedAt: req.CreatedAt,
		SHA:       req.SHA,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, idea)
}
