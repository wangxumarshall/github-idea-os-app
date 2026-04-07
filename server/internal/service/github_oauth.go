package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const githubOAuthStateAudience = "github-oauth"

type GitHubOAuthService struct {
	HTTPClient *http.Client
}

type GitHubAccount struct {
	ID                   string
	UserID               string
	GitHubUserID         int64
	Login                string
	AvatarURL            string
	ProfileURL           string
	AccessTokenEncrypted string
	TokenType            string
	Scope                string
	NextIdeaSeq          int
}

type GitHubAccountPublic struct {
	ID         string  `json:"id"`
	Login      string  `json:"login"`
	AvatarURL  *string `json:"avatar_url"`
	ProfileURL *string `json:"profile_url"`
}

type GitHubOAuthStartResponse struct {
	AuthorizeURL string `json:"authorize_url"`
}

type githubOAuthStateClaims struct {
	UserID       string `json:"user_id"`
	RedirectPath string `json:"redirect_path"`
	jwt.RegisteredClaims
}

type githubAccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
}

type githubUserResponse struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
	HTMLURL   string `json:"html_url"`
}

func NewGitHubOAuthService() *GitHubOAuthService {
	return &GitHubOAuthService{HTTPClient: http.DefaultClient}
}

func (s *GitHubOAuthService) BuildAuthorizeURL(userID, redirectPath string) (string, error) {
	clientID := strings.TrimSpace(os.Getenv("GITHUB_CLIENT_ID"))
	if clientID == "" {
		return "", fmt.Errorf("GitHub OAuth client is not configured")
	}

	callbackURL := githubOAuthCallbackURL()
	state, err := s.signState(userID, redirectPath)
	if err != nil {
		return "", err
	}

	query := url.Values{}
	query.Set("client_id", clientID)
	query.Set("redirect_uri", callbackURL)
	query.Set("scope", "repo read:user user:email")
	query.Set("state", state)

	return "https://github.com/login/oauth/authorize?" + query.Encode(), nil
}

func (s *GitHubOAuthService) ExchangeCode(ctx context.Context, code string) (githubAccessTokenResponse, error) {
	clientID := strings.TrimSpace(os.Getenv("GITHUB_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("GITHUB_CLIENT_SECRET"))
	if clientID == "" || clientSecret == "" {
		return githubAccessTokenResponse{}, fmt.Errorf("GitHub OAuth client is not configured")
	}

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", githubOAuthCallbackURL())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return githubAccessTokenResponse{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := s.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return githubAccessTokenResponse{}, err
	}
	defer resp.Body.Close()

	var tokenResp githubAccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return githubAccessTokenResponse{}, err
	}
	if tokenResp.Error != "" || strings.TrimSpace(tokenResp.AccessToken) == "" {
		return githubAccessTokenResponse{}, fmt.Errorf("GitHub token exchange failed")
	}
	return tokenResp, nil
}

func (s *GitHubOAuthService) FetchUser(ctx context.Context, accessToken string) (githubUserResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return githubUserResponse{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := s.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return githubUserResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return githubUserResponse{}, fmt.Errorf("GitHub user lookup failed")
	}

	var user githubUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return githubUserResponse{}, err
	}
	if user.ID == 0 || strings.TrimSpace(user.Login) == "" {
		return githubUserResponse{}, fmt.Errorf("GitHub user lookup returned empty profile")
	}
	return user, nil
}

func (s *GitHubOAuthService) CompleteOAuth(ctx context.Context, dbtx db.DBTX, code, state string) (string, string, error) {
	claims, err := s.verifyState(state)
	if err != nil {
		return "", "", err
	}

	tokenResp, err := s.ExchangeCode(ctx, code)
	if err != nil {
		return "", "", err
	}

	ghUser, err := s.FetchUser(ctx, tokenResp.AccessToken)
	if err != nil {
		return "", "", err
	}

	encryptedToken, err := auth.EncryptString(tokenResp.AccessToken)
	if err != nil {
		return "", "", err
	}

	if _, err := dbtx.Exec(ctx, `
		INSERT INTO github_account (
			user_id, github_user_id, login, avatar_url, profile_url, access_token_encrypted, token_type, scope
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id) DO UPDATE SET
			github_user_id = EXCLUDED.github_user_id,
			login = EXCLUDED.login,
			avatar_url = EXCLUDED.avatar_url,
			profile_url = EXCLUDED.profile_url,
			access_token_encrypted = EXCLUDED.access_token_encrypted,
			token_type = EXCLUDED.token_type,
			scope = EXCLUDED.scope,
			updated_at = now()
	`, claims.UserID, ghUser.ID, ghUser.Login, nullableString(ghUser.AvatarURL), nullableString(ghUser.HTMLURL), encryptedToken, defaultIfEmpty(tokenResp.TokenType, "bearer"), tokenResp.Scope); err != nil {
		return "", "", err
	}

	return claims.UserID, claims.RedirectPath, nil
}

func (s *GitHubOAuthService) GetAccountByUser(ctx context.Context, dbtx db.DBTX, userID string) (*GitHubAccount, error) {
	row := dbtx.QueryRow(ctx, `
		SELECT id::text, user_id::text, github_user_id, login, COALESCE(avatar_url, ''), COALESCE(profile_url, ''), access_token_encrypted, token_type, scope, next_idea_seq
		FROM github_account
		WHERE user_id = $1
	`, userID)

	var account GitHubAccount
	if err := row.Scan(
		&account.ID,
		&account.UserID,
		&account.GitHubUserID,
		&account.Login,
		&account.AvatarURL,
		&account.ProfileURL,
		&account.AccessTokenEncrypted,
		&account.TokenType,
		&account.Scope,
		&account.NextIdeaSeq,
	); err != nil {
		return nil, err
	}
	return &account, nil
}

func (s *GitHubOAuthService) GetAccountByID(ctx context.Context, dbtx db.DBTX, accountID string) (*GitHubAccount, error) {
	row := dbtx.QueryRow(ctx, `
		SELECT id::text, user_id::text, github_user_id, login, COALESCE(avatar_url, ''), COALESCE(profile_url, ''), access_token_encrypted, token_type, scope, next_idea_seq
		FROM github_account
		WHERE id = $1
	`, accountID)

	var account GitHubAccount
	if err := row.Scan(
		&account.ID,
		&account.UserID,
		&account.GitHubUserID,
		&account.Login,
		&account.AvatarURL,
		&account.ProfileURL,
		&account.AccessTokenEncrypted,
		&account.TokenType,
		&account.Scope,
		&account.NextIdeaSeq,
	); err != nil {
		return nil, err
	}
	return &account, nil
}

func (s *GitHubOAuthService) DeleteAccountByUser(ctx context.Context, dbtx db.DBTX, userID string) error {
	_, err := dbtx.Exec(ctx, `DELETE FROM github_account WHERE user_id = $1`, userID)
	return err
}

func (s *GitHubOAuthService) PublicAccount(account *GitHubAccount) *GitHubAccountPublic {
	if account == nil {
		return nil
	}
	resp := &GitHubAccountPublic{
		ID:    account.ID,
		Login: account.Login,
	}
	if strings.TrimSpace(account.AvatarURL) != "" {
		resp.AvatarURL = &account.AvatarURL
	}
	if strings.TrimSpace(account.ProfileURL) != "" {
		resp.ProfileURL = &account.ProfileURL
	}
	return resp
}

func (s *GitHubOAuthService) DecryptAccessToken(account *GitHubAccount) (string, error) {
	if account == nil {
		return "", fmt.Errorf("GitHub account is required")
	}
	return auth.DecryptString(account.AccessTokenEncrypted)
}

func (s *GitHubOAuthService) signState(userID, redirectPath string) (string, error) {
	if redirectPath == "" {
		redirectPath = "/settings?tab=ideas&github=connected"
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, githubOAuthStateClaims{
		UserID:       userID,
		RedirectPath: redirectPath,
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  []string{githubOAuthStateAudience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})
	return token.SignedString(auth.JWTSecret())
}

func (s *GitHubOAuthService) verifyState(state string) (*githubOAuthStateClaims, error) {
	parsed, err := jwt.ParseWithClaims(state, &githubOAuthStateClaims{}, func(token *jwt.Token) (any, error) {
		return auth.JWTSecret(), nil
	}, jwt.WithAudience(githubOAuthStateAudience))
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*githubOAuthStateClaims)
	if !ok || !parsed.Valid || claims.UserID == "" {
		return nil, fmt.Errorf("invalid GitHub OAuth state")
	}
	return claims, nil
}

func githubOAuthCallbackURL() string {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("GITHUB_OAUTH_CALLBACK_URL")), "/")
	if base != "" {
		return base
	}
	for _, key := range []string{"MULTICA_APP_URL", "FRONTEND_ORIGIN"} {
		if value := strings.TrimRight(strings.TrimSpace(os.Getenv(key)), "/"); value != "" {
			return value + "/auth/github/callback"
		}
	}
	return "http://localhost:3000/auth/github/callback"
}

func nullableString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func defaultIfEmpty(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
