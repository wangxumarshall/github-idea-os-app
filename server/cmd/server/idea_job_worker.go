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

	workspace, err := queries.GetWorkspace(ctx, util.ParseUUID(idea.WorkspaceID))
	if err != nil {
		_ = store.MarkJobFailed(ctx, pool, job.ID, "workspace not found")
		_ = store.UpdateIdeaRepoState(ctx, pool, idea.ID, idea.ProjectRepoURL, "failed", "workspace not found")
		return
	}
	cfg := service.IdeaOSConfigFromSettings(workspace.Settings)
	cfg.GitHubToken = token

	repo, err := gh.CreateRepository(ctx, token, idea.ProjectRepoName, cfg.RepoVisibility)
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
	if err := syncIdeaProjectSpec(ctx, store, gh, cfg, pool, idea); err != nil {
		slog.Warn("idea job worker: sync project spec failed", "idea_id", idea.ID, "error", err)
	}
	if err := enqueueIdeaRootIssueIfReady(ctx, queries, idea); err != nil {
		slog.Warn("idea job worker: enqueue root issue failed", "idea_id", idea.ID, "error", err)
	}
	if err := store.MarkJobCompleted(ctx, pool, job.ID); err != nil {
		slog.Warn("idea job worker: mark completed failed", "job_id", job.ID, "error", err)
	}
}

func enqueueIdeaRootIssueIfReady(ctx context.Context, queries *db.Queries, idea *service.IdeaRecord) error {
	if idea.RootIssueID == "" {
		return nil
	}
	if idea.ProjectSpecSyncError != "" {
		return nil
	}

	issue, err := queries.GetIssue(ctx, util.ParseUUID(idea.RootIssueID))
	if err != nil {
		return err
	}
	if issue.Status == "done" || issue.Status == "cancelled" {
		return nil
	}
	if !issue.AssigneeType.Valid || issue.AssigneeType.String != "agent" || !issue.AssigneeID.Valid {
		return nil
	}

	agent, err := queries.GetAgent(ctx, issue.AssigneeID)
	if err != nil {
		return err
	}
	if agent.ArchivedAt.Valid || !agent.RuntimeID.Valid {
		return nil
	}

	existingTasks, err := queries.ListTasksByIssue(ctx, issue.ID)
	if err != nil {
		return err
	}
	if len(existingTasks) > 0 {
		return nil
	}

	if _, err := queries.UpdateIssueExecutionStage(ctx, db.UpdateIssueExecutionStageParams{
		ID:             issue.ID,
		ExecutionStage: service.QueueStageForMode(service.PreferredTaskMode(issue)),
	}); err != nil {
		return err
	}

	_, err = queries.CreateAgentTask(ctx, db.CreateAgentTaskParams{
		AgentID:   issue.AssigneeID,
		RuntimeID: agent.RuntimeID,
		IssueID:   issue.ID,
		Priority:  priorityToQueueValue(issue.Priority),
		Mode:      service.PreferredTaskMode(issue),
	})
	return err
}

func syncIdeaProjectSpec(ctx context.Context, store *service.IdeaStore, gh *service.GitHubIdeaOSService, cfg service.IdeaOSConfig, pool db.DBTX, idea *service.IdeaRecord) error {
	content, _, err := gh.GetMarkdownFile(ctx, cfg, idea.IdeaPath)
	if err != nil {
		return err
	}

	newSHA, err := gh.SyncProjectRepoSpec(ctx, cfg.GitHubToken, idea.ProjectRepoURL, content, idea.ProjectSpecSHA, "sync idea AGENTS: "+idea.SlugFull)
	if err != nil {
		idea.ProjectSpecSyncError = err.Error()
		_ = store.UpdateIdeaProjectSpecState(ctx, pool, idea.ID, idea.ProjectSpecSHA, idea.ProjectSpecSyncError)
		return err
	}

	idea.ProjectSpecSHA = newSHA
	idea.ProjectSpecSyncError = ""
	return store.UpdateIdeaProjectSpecState(ctx, pool, idea.ID, newSHA, "")
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

func priorityToQueueValue(priority string) int32 {
	switch priority {
	case "urgent":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}
