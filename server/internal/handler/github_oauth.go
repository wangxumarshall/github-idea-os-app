package handler

import (
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/multica-ai/multica/server/internal/service"
)

func (h *Handler) StartGitHubOAuth(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	redirectPath := r.URL.Query().Get("redirect_path")
	if strings.TrimSpace(redirectPath) == "" {
		redirectPath = "/settings?tab=ideas&github=connected"
	}

	authorizeURL, err := h.GitHubOAuth.BuildAuthorizeURL(userID, redirectPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, service.GitHubOAuthStartResponse{AuthorizeURL: authorizeURL})
}

func (h *Handler) GitHubOAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	if code == "" || state == "" {
		writeError(w, http.StatusBadRequest, "missing GitHub OAuth callback parameters")
		return
	}

	_, redirectPath, err := h.GitHubOAuth.CompleteOAuth(r.Context(), h.DB, code, state)
	if err != nil {
		redirectURL := frontendRedirectURL("/settings?tab=ideas&github=error")
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	http.Redirect(w, r, frontendRedirectURL(redirectPath), http.StatusFound)
}

func (h *Handler) GetGitHubAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	account, err := h.GitHubOAuth.GetAccountByUser(r.Context(), h.DB, userID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"connected": false, "account": nil})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"connected": true,
		"account":   h.GitHubOAuth.PublicAccount(account),
	})
}

func (h *Handler) DeleteGitHubAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	if err := h.GitHubOAuth.DeleteAccountByUser(r.Context(), h.DB, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to disconnect GitHub account")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "GitHub account disconnected"})
}

func frontendRedirectURL(redirectPath string) string {
	redirectPath = strings.TrimSpace(redirectPath)
	if redirectPath == "" {
		redirectPath = "/settings?tab=ideas"
	}
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("MULTICA_APP_URL")), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("FRONTEND_ORIGIN")), "/")
	}
	if base == "" {
		base = "http://localhost:3000"
	}
	if parsed, err := url.Parse(base); err == nil {
		parsed.Path = ""
		parsed.RawQuery = ""
		parsed.Fragment = ""
		base = strings.TrimRight(parsed.String(), "/")
	}
	if strings.HasPrefix(redirectPath, "http://") || strings.HasPrefix(redirectPath, "https://") {
		return redirectPath
	}
	return base + redirectPath
}
