package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type IdeaOSConfigResponse struct {
	service.PublicIdeaOSConfig
	GitHubConnected bool    `json:"github_connected"`
	GitHubLogin     *string `json:"github_login,omitempty"`
}

type UpdateIdeaOSConfigRequest struct {
	RepoURL         *string  `json:"repo_url"`
	Branch          *string  `json:"branch"`
	Directory       *string  `json:"directory"`
	RepoVisibility  *string  `json:"repo_visibility"`
	DefaultAgentIDs []string `json:"default_agent_ids"`
}

type RecommendIdeaNameRequest struct {
	RawInput string `json:"raw_input"`
}

type RecommendIdeaNameResponse struct {
	NextCode    string                       `json:"next_code"`
	Suggestions []service.IdeaNameSuggestion `json:"suggestions"`
}

type CreateIdeaRequest struct {
	RawInput     string `json:"raw_input"`
	SelectedName string `json:"selected_name"`
}

type IdeaIssuesResponse struct {
	RootIssue   *IssueResponse  `json:"root_issue"`
	ChildIssues []IssueResponse `json:"child_issues"`
}

type UpdateIdeaRequest struct {
	Title         string   `json:"title"`
	Content       string   `json:"content"`
	BaseTitle     string   `json:"base_title"`
	BaseContent   string   `json:"base_content"`
	Tags          []string `json:"tags"`
	BaseTags      []string `json:"base_tags"`
	CreatedAt     string   `json:"created_at"`
	BaseCreatedAt string   `json:"base_created_at"`
	SHA           string   `json:"sha"`
}

func stringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func buildIdeaRootIssueDescription(idea service.IdeaRecord) string {
	var b strings.Builder

	b.WriteString("# Idea Delivery\n\n")
	b.WriteString("Implement the project described by this idea.\n\n")
	b.WriteString("- Idea code: `" + idea.Code + "`\n")
	b.WriteString("- Idea title: " + idea.Title + "\n")
	b.WriteString("- Idea slug: `" + idea.SlugFull + "`\n")
	b.WriteString("- Idea markdown: `" + idea.IdeaPath + "`\n")
	if strings.TrimSpace(idea.ProjectRepoURL) != "" {
		b.WriteString("- Project repository: " + strings.TrimSpace(idea.ProjectRepoURL) + "\n")
		b.WriteString("- Project instructions path: `AGENTS.md`\n")
	}
	b.WriteString("\n## Workflow\n\n")
	b.WriteString("1. Run `multica idea get " + idea.SlugFull + " --output json`\n")
	b.WriteString("2. Treat the full idea markdown as the product and technical source of truth\n")
	b.WriteString("3. Implement the project in the repository bound to this issue\n")

	if strings.TrimSpace(idea.Summary) != "" {
		b.WriteString("\n## Summary\n\n")
		b.WriteString(idea.Summary)
		b.WriteString("\n")
	}

	return b.String()
}

func (h *Handler) syncIdeaProjectSpec(ctx context.Context, cfg service.IdeaOSConfig, record *service.IdeaRecord, doc service.IdeaDocument, dbtx db.DBTX) error {
	if record == nil {
		return nil
	}
	if strings.TrimSpace(record.ProjectRepoURL) == "" || record.ProjectRepoStatus != "ready" {
		return nil
	}

	rendered, err := service.RenderIdeaDocument(doc)
	if err != nil {
		return err
	}

	newSHA, err := h.IdeaOS.SyncProjectRepoSpec(ctx, cfg.GitHubToken, record.ProjectRepoURL, rendered, record.ProjectSpecSHA, "sync idea AGENTS: "+record.SlugFull)
	if err != nil {
		record.ProjectSpecSyncError = err.Error()
		_ = h.IdeaStore.UpdateIdeaProjectSpecState(ctx, dbtx, record.ID, record.ProjectSpecSHA, record.ProjectSpecSyncError)
		return err
	}

	record.ProjectSpecSHA = newSHA
	record.ProjectSpecSyncError = ""
	return h.IdeaStore.UpdateIdeaProjectSpecState(ctx, dbtx, record.ID, newSHA, "")
}

func (h *Handler) validateDefaultIdeaAgentIDs(r *http.Request, workspaceID string, ids []string) ([]string, error) {
	normalized := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, agentID := range ids {
		agentID = strings.TrimSpace(agentID)
		if agentID == "" {
			continue
		}
		if _, ok := seen[agentID]; ok {
			continue
		}
		seen[agentID] = struct{}{}
		normalized = append(normalized, agentID)
	}
	if len(normalized) == 0 {
		return nil, nil
	}

	agents, err := h.Queries.ListAllAgents(r.Context(), parseUUID(workspaceID))
	if err != nil {
		return nil, err
	}

	valid := make(map[string]struct{}, len(agents))
	for _, agent := range agents {
		if agent.ArchivedAt.Valid {
			continue
		}
		valid[uuidToString(agent.ID)] = struct{}{}
	}

	for _, agentID := range normalized {
		if _, ok := valid[agentID]; !ok {
			return nil, fmt.Errorf("invalid default idea agent: %s", agentID)
		}
	}

	return normalized, nil
}

func (h *Handler) resolveDefaultIdeaAgent(r *http.Request, workspaceID string, cfg service.IdeaOSConfig) *db.Agent {
	for _, agentID := range cfg.DefaultAgentIDs {
		agent, err := h.Queries.GetAgentInWorkspace(r.Context(), db.GetAgentInWorkspaceParams{
			ID:          parseUUID(agentID),
			WorkspaceID: parseUUID(workspaceID),
		})
		if err != nil || agent.ArchivedAt.Valid || !agent.RuntimeID.Valid {
			continue
		}
		return &agent
	}
	return nil
}

func (h *Handler) ideaIssueResponse(ctx context.Context, issue db.Issue) IssueResponse {
	prefix := h.getIssuePrefix(ctx, issue.WorkspaceID)
	return h.issueResponse(ctx, issue, prefix)
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
	if req.DefaultAgentIDs != nil {
		validatedIDs, err := h.validateDefaultIdeaAgentIDs(r, workspaceID, req.DefaultAgentIDs)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		update.DefaultAgentIDs = &validatedIDs
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
		if service.IsLegacyIdea(idea.Code, idea.SlugFull) {
			continue
		}
		resp = append(resp, service.IdeaSummary{
			ID:                   idea.ID,
			Code:                 idea.Code,
			Slug:                 idea.SlugFull,
			Path:                 idea.IdeaPath,
			Title:                idea.Title,
			Summary:              idea.Summary,
			Tags:                 idea.Tags,
			ProjectRepoName:      idea.ProjectRepoName,
			ProjectRepoURL:       idea.ProjectRepoURL,
			ProjectRepoStatus:    idea.ProjectRepoStatus,
			ProvisioningError:    idea.ProvisioningError,
			RootIssueID:          idea.RootIssueID,
			ProjectSpecSyncError: idea.ProjectSpecSyncError,
			CreatedAt:            idea.CreatedAt.Format("2006-01-02"),
			UpdatedAt:            idea.UpdatedAt.Format("2006-01-02"),
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

	var rootIssue db.Issue
	defaultAgent := h.resolveDefaultIdeaAgent(r, workspaceID, cfg)
	rootIssueReq := CreateIssueRequest{
		Title:       fmt.Sprintf("Build %s", parsed.Title),
		Description: stringPtr(buildIdeaRootIssueDescription(record)),
		Status:      "todo",
		Priority:    "medium",
		RepoURL:     stringPtr(projectRepoURL),
	}
	if defaultAgent != nil {
		agentID := uuidToString(defaultAgent.ID)
		rootIssueReq.AssigneeType = stringPtr("agent")
		rootIssueReq.AssigneeID = &agentID
	}

	rootIssueNumber, err := h.Queries.WithTx(tx).IncrementIssueCounter(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to allocate root issue sequence")
		return
	}

	rootIssue, err = h.createIssueRecord(r.Context(), h.Queries.WithTx(tx), workspaceID, "member", userID, rootIssueReq, rootIssueNumber, &record.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create root issue")
		return
	}

	if err := h.IdeaStore.UpdateIdeaRootIssue(r.Context(), tx, record.ID, uuidToString(rootIssue.ID)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to link root issue")
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
	parsed.ID = record.ID
	parsed.Code = code
	parsed.Slug = slugFull
	parsed.Path = doc.Path
	parsed.ProjectRepoName = slugFull
	parsed.ProjectRepoURL = projectRepoURL
	parsed.ProjectRepoStatus = "creating"
	parsed.RootIssueID = uuidToString(rootIssue.ID)
	parsed.ProjectSpecSyncError = record.ProjectSpecSyncError

	rootResp := h.ideaIssueResponse(r.Context(), rootIssue)
	h.publish(protocol.EventIssueCreated, workspaceID, "member", userID, map[string]any{"issue": rootResp})
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
	doc.ID = record.ID
	doc.Code = record.Code
	doc.ProjectRepoName = record.ProjectRepoName
	doc.ProjectRepoURL = record.ProjectRepoURL
	doc.ProjectRepoStatus = record.ProjectRepoStatus
	doc.ProvisioningError = record.ProvisioningError
	doc.RootIssueID = record.RootIssueID
	doc.ProjectSpecSyncError = record.ProjectSpecSyncError
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

	parsed, err := h.IdeaOS.UpdateIdea(r.Context(), cfg, service.UpdateIdeaInput{
		Slug:              record.SlugFull,
		Code:              record.Code,
		Title:             req.Title,
		Content:           req.Content,
		BaseTitle:         req.BaseTitle,
		BaseContent:       req.BaseContent,
		Tags:              req.Tags,
		BaseTags:          req.BaseTags,
		CreatedAt:         req.CreatedAt,
		BaseCreatedAt:     req.BaseCreatedAt,
		SHA:               strings.TrimSpace(req.SHA),
		ProjectRepoName:   record.ProjectRepoName,
		ProjectRepoURL:    record.ProjectRepoURL,
		ProjectRepoStatus: record.ProjectRepoStatus,
	})
	if err != nil {
		var conflictErr *service.IdeaConflictError
		if errors.As(err, &conflictErr) {
			writeError(w, http.StatusConflict, conflictErr.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.IdeaStore.UpdateIdeaContent(r.Context(), h.DB, record.ID, parsed.Title, parsed.Summary, parsed.SHA, parsed.Tags, "", ""); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update idea metadata")
		return
	}
	if err := h.syncIdeaProjectSpec(r.Context(), cfg, record, parsed, h.DB); err != nil {
		parsed.ProjectSpecSyncError = err.Error()
	}

	parsed.ID = record.ID
	parsed.Code = record.Code
	parsed.Slug = record.SlugFull
	parsed.Path = record.IdeaPath
	parsed.ProjectRepoName = record.ProjectRepoName
	parsed.ProjectRepoURL = record.ProjectRepoURL
	parsed.ProjectRepoStatus = record.ProjectRepoStatus
	parsed.ProvisioningError = record.ProvisioningError
	parsed.RootIssueID = record.RootIssueID
	if parsed.ProjectSpecSyncError == "" {
		parsed.ProjectSpecSyncError = record.ProjectSpecSyncError
	}
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

func (h *Handler) ListIdeaIssues(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)

	record, err := h.IdeaStore.GetIdeaBySlug(r.Context(), h.DB, workspaceID, chi.URLParam(r, "slug"))
	if err != nil {
		writeError(w, http.StatusNotFound, "idea not found")
		return
	}

	if record.RootIssueID == "" {
		writeJSON(w, http.StatusOK, IdeaIssuesResponse{ChildIssues: []IssueResponse{}})
		return
	}

	issues, err := h.Queries.ListIssuesByIdeaID(r.Context(), parseUUID(record.ID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list idea issues")
		return
	}

	resp := IdeaIssuesResponse{ChildIssues: make([]IssueResponse, 0, len(issues))}
	for _, issue := range issues {
		issueResp := h.ideaIssueResponse(r.Context(), issue)
		if uuidToString(issue.ID) == record.RootIssueID {
			copyResp := issueResp
			resp.RootIssue = &copyResp
			continue
		}
		resp.ChildIssues = append(resp.ChildIssues, issueResp)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) CreateIdeaIssue(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	record, err := h.IdeaStore.GetIdeaBySlug(r.Context(), h.DB, workspaceID, chi.URLParam(r, "slug"))
	if err != nil {
		writeError(w, http.StatusNotFound, "idea not found")
		return
	}
	if record.RootIssueID == "" {
		writeError(w, http.StatusConflict, "idea root issue is not initialized")
		return
	}

	var req CreateIssueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if _, err := parseIssueDueDate(req.DueDate); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.IdeaID = stringPtr(record.ID)
	req.ParentIssueID = stringPtr(record.RootIssueID)
	if err := bindCreateIssueToIdea(&req, record); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.AssigneeType != nil && *req.AssigneeType == "agent" && req.AssigneeID != nil {
		if ok, msg := h.canAssignAgent(r.Context(), r, *req.AssigneeID, workspaceID); !ok {
			writeError(w, http.StatusForbidden, msg)
			return
		}
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create issue")
		return
	}
	defer tx.Rollback(r.Context())

	qtx := h.Queries.WithTx(tx)
	issueNumber, err := qtx.IncrementIssueCounter(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create issue")
		return
	}

	creatorType, actualCreatorID := h.resolveActor(r, userID, workspaceID)
	issue, err := h.createIssueRecord(r.Context(), qtx, workspaceID, creatorType, actualCreatorID, req, issueNumber, &record.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create issue: "+err.Error())
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create issue")
		return
	}

	if len(req.AttachmentIDs) > 0 {
		h.linkAttachmentsByIssueIDs(r.Context(), issue.ID, issue.WorkspaceID, req.AttachmentIDs)
	}

	resp := h.ideaIssueResponse(r.Context(), issue)
	h.publish(protocol.EventIssueCreated, workspaceID, creatorType, actualCreatorID, map[string]any{"issue": resp})
	if issue.AssigneeType.Valid && issue.AssigneeID.Valid && h.shouldEnqueueAgentTask(r.Context(), issue) {
		_ = h.enqueueIssueTaskWithWarning(r.Context(), issue)
	}

	writeJSON(w, http.StatusCreated, resp)
}

type httptestDiscardWriter struct{}

func (httptestDiscardWriter) Header() http.Header        { return http.Header{} }
func (httptestDiscardWriter) Write([]byte) (int, error)  { return 0, nil }
func (httptestDiscardWriter) WriteHeader(statusCode int) {}
