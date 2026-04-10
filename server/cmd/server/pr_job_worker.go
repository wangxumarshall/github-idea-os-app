package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const issuePRJobSweepInterval = 5 * time.Second

func runIssuePRJobWorker(ctx context.Context, pool db.DBTX, queries *db.Queries, bus *events.Bus, taskService *service.TaskService) {
	store := service.NewIssuePRStore()
	oauth := service.NewGitHubOAuthService()
	gh := service.NewGitHubIdeaOSService()
	ticker := time.NewTicker(issuePRJobSweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for {
				job, err := store.ClaimNext(ctx, pool)
				if err != nil {
					slog.Warn("issue PR job worker: claim failed", "error", err)
					break
				}
				if job == nil {
					break
				}
				processIssuePRJob(ctx, pool, queries, bus, taskService, store, oauth, gh, job)
			}
		}
	}
}

func registerPullRequestListeners(bus *events.Bus, queries *db.Queries, store *service.IssuePRStore, dbtx db.DBTX) {
	bus.Subscribe(protocol.EventIssueUpdated, func(e events.Event) {
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			return
		}
		statusChanged, _ := payload["status_changed"].(bool)
		prevStatus, _ := payload["prev_status"].(string)
		issue, ok := payload["issue"].(handler.IssueResponse)
		if !ok {
			return
		}
		if !statusChanged || issue.Status != "in_review" || prevStatus == "in_review" {
			return
		}
		if err := store.Enqueue(context.Background(), dbtx, issue.ID, map[string]any{"trigger": "issue_in_review"}); err != nil {
			slog.Warn("enqueue issue PR job failed", "issue_id", issue.ID, "error", err)
		}
	})

	bus.Subscribe(protocol.EventTaskCompleted, func(e events.Event) {
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			return
		}
		issueID, _ := payload["issue_id"].(string)
		if issueID == "" {
			return
		}
		resultPayload, _ := payload["result"].(map[string]any)
		if !hasReviewIntent(resultPayload) {
			return
		}
		if err := store.Enqueue(context.Background(), dbtx, issueID, map[string]any{"trigger": "task_completed"}); err != nil {
			slog.Warn("enqueue issue PR job on task completion failed", "issue_id", issueID, "error", err)
		}
	})
}

func processIssuePRJob(ctx context.Context, pool db.DBTX, queries *db.Queries, bus *events.Bus, taskService *service.TaskService, store *service.IssuePRStore, oauth *service.GitHubOAuthService, gh *service.GitHubIdeaOSService, job *service.IssuePRJobRecord) {
	issue, err := queries.GetIssue(ctx, util.ParseUUID(job.IssueID))
	if err != nil {
		_ = store.MarkSkipped(ctx, pool, job.ID, "issue not found")
		return
	}
	if issue.Status == "done" || issue.Status == "cancelled" {
		_ = store.MarkSkipped(ctx, pool, job.ID, "issue is terminal")
		return
	}
	if !issue.RepoUrl.Valid || strings.TrimSpace(issue.RepoUrl.String) == "" {
		_ = store.MarkSkipped(ctx, pool, job.ID, "issue has no repository")
		return
	}

	task, err := store.GetLatestCompletedTaskByIssue(ctx, queries, job.IssueID)
	if err != nil {
		if time.Since(job.CreatedAt) > 30*time.Minute {
			_ = store.MarkFailed(ctx, pool, job.ID, "latest completed task not found before timeout")
		} else {
			_ = store.Requeue(ctx, pool, job.ID, "waiting_for_completed_task", 5*time.Second)
		}
		return
	}

	payload := protocol.TaskCompletedPayload{}
	if len(task.Result) > 0 {
		_ = json.Unmarshal(task.Result, &payload)
	}
	enrichTaskPayload(issue.RepoUrl.String, &payload)
	if strings.TrimSpace(payload.PRURL) != "" {
		_ = store.MarkSkipped(ctx, pool, job.ID, "pull request already exists")
		return
	}
	if !taskRequestsReview(issue.Status, &payload, job.Payload) {
		_ = store.MarkSkipped(ctx, pool, job.ID, "task is not review-ready")
		return
	}
	if strings.TrimSpace(payload.BranchName) == "" {
		finalizeIssuePRHandoff(ctx, pool, queries, bus, taskService, store, job, issue, task, &payload, "Completed task did not record a branch name.")
		return
	}

	if issue.CreatorType != "member" {
		finalizeIssuePRHandoff(ctx, pool, queries, bus, taskService, store, job, issue, task, &payload, "Issue creator is not a member-backed GitHub account.")
		return
	}

	creatorID := util.UUIDToString(issue.CreatorID)
	account, err := oauth.GetAccountByUser(ctx, pool, creatorID)
	if err != nil {
		finalizeIssuePRHandoff(ctx, pool, queries, bus, taskService, store, job, issue, task, &payload, "Issue creator has not connected GitHub.")
		return
	}

	token, err := oauth.DecryptAccessToken(account)
	if err != nil {
		finalizeIssuePRHandoff(ctx, pool, queries, bus, taskService, store, job, issue, task, &payload, "Failed to decrypt issue creator GitHub token.")
		return
	}

	repo, err := gh.GetRepository(ctx, token, issue.RepoUrl.String)
	if err != nil {
		if shouldRetryPRCreation(err) && job.Attempts < 5 {
			_ = store.Requeue(ctx, pool, job.ID, err.Error(), 10*time.Second)
			return
		}
		finalizeIssuePRHandoff(ctx, pool, queries, bus, taskService, store, job, issue, task, &payload, handoffReasonForPRError(err))
		return
	}

	if strings.TrimSpace(payload.CompareURL) == "" {
		payload.CompareURL = compareURL(issue.RepoUrl.String, defaultBranch(repo), payload.BranchName)
	}

	headOwner := repo.Owner.Login
	if headOwner == "" {
		headOwner = repoOwner(issue.RepoUrl.String)
	}

	if existing, err := gh.FindOpenPullRequestByHead(ctx, token, issue.RepoUrl.String, headOwner, payload.BranchName); err == nil && existing != nil {
		payload.PRURL = existing.HTMLURL
		payload.DeliveryState = "delivered"
		payload.HandoffReason = ""
		payload.Summary = "Delivery ready. PR created."
		if err := finalizeIssuePRResult(ctx, pool, queries, bus, taskService, store, job, issue, task, &payload, "delivery_ready"); err != nil {
			slog.Warn("issue PR job: finalize existing PR failed", "issue_id", job.IssueID, "error", err)
			_ = store.MarkFailed(ctx, pool, job.ID, err.Error())
			return
		}
		_ = store.MarkCompleted(ctx, pool, job.ID)
		return
	} else if err != nil {
		if shouldRetryPRCreation(err) && job.Attempts < 5 {
			_ = store.Requeue(ctx, pool, job.ID, err.Error(), 10*time.Second)
			return
		}
		finalizeIssuePRHandoff(ctx, pool, queries, bus, taskService, store, job, issue, task, &payload, handoffReasonForPRError(err))
		return
	}

	pr, err := gh.CreatePullRequest(ctx, token, issue.RepoUrl.String, payload.BranchName, defaultBranch(repo), issuePRTitle(issue, queries), issuePRBody(issue, queries, &payload))
	if err != nil {
		if shouldRetryPRCreation(err) && job.Attempts < 5 {
			_ = store.Requeue(ctx, pool, job.ID, err.Error(), 10*time.Second)
			return
		}
		finalizeIssuePRHandoff(ctx, pool, queries, bus, taskService, store, job, issue, task, &payload, handoffReasonForPRError(err))
		return
	}

	payload.PRURL = pr.HTMLURL
	payload.DeliveryState = "delivered"
	payload.HandoffReason = ""
	payload.Summary = "Delivery ready. PR created."
	if err := finalizeIssuePRResult(ctx, pool, queries, bus, taskService, store, job, issue, task, &payload, "delivery_ready"); err != nil {
		slog.Warn("issue PR job: finalize PR creation failed", "issue_id", job.IssueID, "error", err)
		_ = store.MarkFailed(ctx, pool, job.ID, err.Error())
		return
	}
	_ = store.MarkCompleted(ctx, pool, job.ID)
}

func finalizeIssuePRHandoff(ctx context.Context, pool db.DBTX, queries *db.Queries, bus *events.Bus, taskService *service.TaskService, store *service.IssuePRStore, job *service.IssuePRJobRecord, issue db.Issue, task *db.AgentTaskQueue, payload *protocol.TaskCompletedPayload, reason string) {
	payload.DeliveryState = "handoff_required"
	payload.HandoffReason = strings.TrimSpace(reason)
	payload.Summary = "Delivery ready, but PR creation requires handoff."
	if err := finalizeIssuePRResult(ctx, pool, queries, bus, taskService, store, job, issue, task, payload, "delivery_handoff_required"); err != nil {
		slog.Warn("issue PR job: finalize handoff failed", "issue_id", job.IssueID, "error", err)
		_ = store.MarkFailed(ctx, pool, job.ID, err.Error())
		return
	}
	_ = store.MarkSkipped(ctx, pool, job.ID, reason)
}

func finalizeIssuePRResult(ctx context.Context, pool db.DBTX, queries *db.Queries, bus *events.Bus, taskService *service.TaskService, store *service.IssuePRStore, job *service.IssuePRJobRecord, issue db.Issue, task *db.AgentTaskQueue, payload *protocol.TaskCompletedPayload, activityAction string) error {
	commentID, err := taskService.UpsertTaskDeliveryComment(ctx, *task, payload)
	if err != nil {
		return err
	}
	if commentID != "" {
		payload.DeliveryCommentID = commentID
	}

	resultJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if err := store.UpdateTaskResult(ctx, pool, util.UUIDToString(task.ID), resultJSON); err != nil {
		return err
	}

	record, err := queries.CreateActivity(ctx, db.CreateActivityParams{
		WorkspaceID: issue.WorkspaceID,
		IssueID:     issue.ID,
		ActorType:   util.StrToText("agent"),
		ActorID:     task.AgentID,
		Action:      activityAction,
		Details:     resultJSON,
	})
	if err != nil {
		return err
	}

	publishActivityEvent(bus, events.Event{
		WorkspaceID: util.UUIDToString(issue.WorkspaceID),
		ActorType:   "agent",
		ActorID:     util.UUIDToString(task.AgentID),
	}, record)

	bus.Publish(events.Event{
		Type:        protocol.EventTaskUpdated,
		WorkspaceID: util.UUIDToString(issue.WorkspaceID),
		ActorType:   "system",
		ActorID:     "",
		Payload: map[string]any{
			"task_id":  util.UUIDToString(task.ID),
			"agent_id": util.UUIDToString(task.AgentID),
			"issue_id": util.UUIDToString(task.IssueID),
			"status":   task.Status,
			"result": map[string]any{
				"summary":             payload.Summary,
				"pr_url":              payload.PRURL,
				"compare_url":         payload.CompareURL,
				"branch_name":         payload.BranchName,
				"delivery_state":      payload.DeliveryState,
				"handoff_reason":      payload.HandoffReason,
				"delivery_comment_id": payload.DeliveryCommentID,
			},
		},
	})

	return nil
}

func issuePRTitle(issue db.Issue, queries *db.Queries) string {
	identifier := issue.Number
	prefix := "ISSUE"
	if workspace, err := queries.GetWorkspace(context.Background(), issue.WorkspaceID); err == nil && workspace.IssuePrefix != "" {
		prefix = workspace.IssuePrefix
	}
	return fmt.Sprintf("%s-%d %s", prefix, identifier, issue.Title)
}

func issuePRBody(issue db.Issue, queries *db.Queries, payload *protocol.TaskCompletedPayload) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Auto-created by Multica for issue `%s`.", issuePRTitle(issue, queries)))
	if appURL := strings.TrimRight(strings.TrimSpace(os.Getenv("MULTICA_APP_URL")), "/"); appURL != "" {
		lines = append(lines, "", fmt.Sprintf("Issue: %s/issues/%s", appURL, util.UUIDToString(issue.ID)))
	}
	if payload != nil && strings.TrimSpace(payload.CompareURL) != "" {
		lines = append(lines, "", "Compare: "+strings.TrimSpace(payload.CompareURL))
	}
	return strings.Join(lines, "\n")
}

func defaultBranch(repo service.GitHubRepository) string {
	if strings.TrimSpace(repo.DefaultBranch) != "" {
		return strings.TrimSpace(repo.DefaultBranch)
	}
	return "main"
}

func repoOwner(repoURL string) string {
	owner, _, err := service.ParseGitHubRepoURL(repoURL)
	if err != nil {
		return ""
	}
	return owner
}

func compareURL(repoURL, baseBranch, headBranch string) string {
	repoURL = strings.TrimSuffix(strings.TrimSpace(repoURL), ".git")
	repoURL = strings.TrimSuffix(repoURL, "/")
	if repoURL == "" || strings.TrimSpace(headBranch) == "" {
		return ""
	}
	if strings.TrimSpace(baseBranch) == "" {
		baseBranch = "main"
	}
	return fmt.Sprintf("%s/compare/%s...%s?expand=1", repoURL, baseBranch, headBranch)
}

func branchNameFromCompareURL(compare string) string {
	compare = strings.TrimSpace(compare)
	if compare == "" {
		return ""
	}
	parts := strings.Split(compare, "/compare/")
	if len(parts) != 2 {
		return ""
	}
	rest := parts[1]
	rest = strings.Split(rest, "?")[0]
	rangeParts := strings.Split(rest, "...")
	if len(rangeParts) != 2 {
		return ""
	}
	return strings.TrimSpace(rangeParts[1])
}

func branchNameFromOutput(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`/pull/new/([A-Za-z0-9._/\-]+)`),
		regexp.MustCompile(`pushed branch\s+` + "`?" + `([A-Za-z0-9._/\-]+)` + "`?"),
		regexp.MustCompile(`branch\s+` + "`" + `([A-Za-z0-9._/\-]+)` + "`"),
	}
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(output)
		if len(match) > 1 {
			return strings.TrimSpace(match[1])
		}
	}
	return ""
}

func hasReviewIntent(result map[string]any) bool {
	if len(result) == 0 {
		return false
	}
	for _, key := range []string{"pr_url", "compare_url", "branch_name"} {
		if value, ok := result[key].(string); ok && strings.TrimSpace(value) != "" {
			return true
		}
	}
	if state, ok := result["delivery_state"].(string); ok && strings.TrimSpace(state) != "" {
		return true
	}
	if output, ok := result["output"].(string); ok && strings.TrimSpace(branchNameFromOutput(output)) != "" {
		return true
	}
	return false
}

func enrichTaskPayload(repoURL string, payload *protocol.TaskCompletedPayload) {
	if payload == nil {
		return
	}
	derivedBranch := branchNameFromOutput(payload.Output)
	if strings.TrimSpace(derivedBranch) == "" {
		derivedBranch = branchNameFromCompareURL(payload.CompareURL)
	}
	if strings.TrimSpace(derivedBranch) != "" {
		payload.BranchName = derivedBranch
	}
	if strings.TrimSpace(payload.CompareURL) == "" && strings.TrimSpace(payload.BranchName) != "" {
		payload.CompareURL = compareURL(repoURL, "main", payload.BranchName)
	} else if strings.TrimSpace(payload.CompareURL) != "" && strings.TrimSpace(payload.BranchName) != "" {
		expected := branchNameFromCompareURL(payload.CompareURL)
		if expected != payload.BranchName {
			payload.CompareURL = compareURL(repoURL, "main", payload.BranchName)
		}
	}
}

func taskRequestsReview(issueStatus string, payload *protocol.TaskCompletedPayload, jobPayload map[string]any) bool {
	if payload == nil {
		return false
	}
	if strings.TrimSpace(payload.PRURL) != "" || strings.TrimSpace(payload.CompareURL) != "" || strings.TrimSpace(payload.BranchName) != "" {
		return true
	}
	if issueStatus == "in_review" {
		return true
	}
	trigger, _ := jobPayload["trigger"].(string)
	return trigger == "issue_in_review"
}

func shouldRetryPRCreation(err error) bool {
	var ghErr *service.GitHubAPIError
	if !errors.As(err, &ghErr) {
		return false
	}
	return ghErr.StatusCode == 429 || ghErr.StatusCode >= 500
}

func handoffReasonForPRError(err error) string {
	var ghErr *service.GitHubAPIError
	if !errors.As(err, &ghErr) {
		return "PR creation is unavailable from the server environment."
	}
	switch ghErr.StatusCode {
	case 401:
		return "Issue creator GitHub token is invalid or expired."
	case 403:
		return "Issue creator does not have permission to create a pull request in this repository."
	case 404:
		return "Repository not found or issue creator cannot access it."
	default:
		return strings.TrimSpace(ghErr.Message)
	}
}
