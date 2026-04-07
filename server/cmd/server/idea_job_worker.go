package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const ideaJobSweepInterval = 10 * time.Second

func runIdeaJobWorker(ctx context.Context, pool db.DBTX, queries *db.Queries) {
	store := service.NewIdeaStore()
	oauth := service.NewGitHubOAuthService()
	gh := service.NewGitHubIdeaOSService()
	ticker := time.NewTicker(ideaJobSweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for {
				job, err := store.ClaimNextJob(ctx, pool)
				if err != nil {
					slog.Warn("idea job worker: claim failed", "error", err)
					break
				}
				if job == nil {
					break
				}
				processIdeaJob(ctx, pool, queries, store, oauth, gh, job)
			}
		}
	}
}

func processIdeaJob(ctx context.Context, pool db.DBTX, queries *db.Queries, store *service.IdeaStore, oauth *service.GitHubOAuthService, gh *service.GitHubIdeaOSService, job *service.IdeaJobRecord) {
	idea, err := store.GetIdeaByID(ctx, pool, job.IdeaID)
	if err != nil {
		_ = store.MarkJobFailed(ctx, pool, job.ID, "idea not found")
		return
	}

	account, err := oauth.GetAccountByID(ctx, pool, idea.GitHubAccountID)
	if err != nil {
		_ = store.MarkJobFailed(ctx, pool, job.ID, "GitHub account not found")
		_ = store.UpdateIdeaRepoState(ctx, pool, idea.ID, idea.ProjectRepoURL, "failed", "GitHub account not found")
		return
	}

	token, err := oauth.DecryptAccessToken(account)
	if err != nil {
		_ = store.MarkJobFailed(ctx, pool, job.ID, "failed to decrypt GitHub token")
		_ = store.UpdateIdeaRepoState(ctx, pool, idea.ID, idea.ProjectRepoURL, "failed", "failed to decrypt GitHub token")
		return
	}

	repo, err := gh.CreatePrivateRepository(ctx, token, idea.ProjectRepoName)
	if err != nil {
		message := err.Error()
		if strings.Contains(strings.ToLower(message), "name already exists") {
			repo = service.GitHubRepository{
				Name:          idea.ProjectRepoName,
				Private:       true,
				DefaultBranch: "main",
				HTMLURL:       fmt.Sprintf("https://github.com/%s/%s", account.Login, idea.ProjectRepoName),
			}
		} else {
			_ = store.MarkJobFailed(ctx, pool, job.ID, message)
			_ = store.UpdateIdeaRepoState(ctx, pool, idea.ID, idea.ProjectRepoURL, "failed", message)
			_ = syncIdeaRepoStatus(ctx, queries, gh, oauth, pool, idea, "failed", idea.ProjectRepoURL, message)
			return
		}
	}

	if repo.DefaultBranch != "" && repo.DefaultBranch != "main" {
		if err := gh.RenameDefaultBranchToMain(ctx, token, account.Login, repo.Name, repo.DefaultBranch); err != nil {
			slog.Warn("idea job worker: rename default branch failed", "repo", repo.Name, "error", err)
		}
	}

	if repo.HTMLURL == "" {
		repo.HTMLURL = idea.ProjectRepoURL
	}

	if err := store.UpdateIdeaRepoState(ctx, pool, idea.ID, repo.HTMLURL, "ready", ""); err != nil {
		slog.Warn("idea job worker: update idea repo state failed", "idea_id", idea.ID, "error", err)
	}
	if err := syncIdeaRepoStatus(ctx, queries, gh, oauth, pool, idea, "ready", repo.HTMLURL, ""); err != nil {
		slog.Warn("idea job worker: sync markdown status failed", "idea_id", idea.ID, "error", err)
	}
	if err := store.MarkJobCompleted(ctx, pool, job.ID); err != nil {
		slog.Warn("idea job worker: mark completed failed", "job_id", job.ID, "error", err)
	}
}

func syncIdeaRepoStatus(ctx context.Context, queries *db.Queries, gh *service.GitHubIdeaOSService, oauth *service.GitHubOAuthService, pool db.DBTX, idea *service.IdeaRecord, status, repoURL, reason string) error {
	workspace, err := queries.GetWorkspace(ctx, util.ParseUUID(idea.WorkspaceID))
	if err != nil {
		return err
	}
	account, err := oauth.GetAccountByID(ctx, pool, idea.GitHubAccountID)
	if err != nil {
		return err
	}
	token, err := oauth.DecryptAccessToken(account)
	if err != nil {
		return err
	}
	cfg := service.IdeaOSConfigFromSettings(workspace.Settings)
	cfg.GitHubToken = token

	content, sha, err := gh.GetMarkdownFile(ctx, cfg, idea.IdeaPath)
	if err != nil {
		return err
	}
	doc, err := service.ParseIdeaDocument(idea.IdeaPath, sha, content)
	if err != nil {
		return err
	}
	doc.Code = idea.Code
	doc.ProjectRepoName = idea.ProjectRepoName
	doc.ProjectRepoURL = repoURL
	doc.ProjectRepoStatus = status
	doc.ProvisioningError = reason

	rendered, err := service.RenderIdeaDocument(doc)
	if err != nil {
		return err
	}
	newSHA, err := gh.PutMarkdownFile(ctx, cfg, idea.IdeaPath, rendered, sha, "update idea: "+idea.SlugFull)
	if err != nil {
		return err
	}
	return service.NewIdeaStore().UpdateIdeaContent(ctx, pool, idea.ID, doc.Title, doc.Summary, newSHA, doc.Tags, status, reason)
}
