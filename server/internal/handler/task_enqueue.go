package handler

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func (h *Handler) enqueueIssueTaskWithWarning(ctx context.Context, issue db.Issue, triggerCommentID ...pgtype.UUID) error {
	_, err := h.TaskService.EnqueueTaskForIssue(ctx, issue, triggerCommentID...)
	if err != nil {
		h.handleAutoEnqueueError(ctx, issue, issue.AssigneeID, err, triggerCommentID...)
	}
	return err
}

func (h *Handler) enqueueIssueTaskInModeWithWarning(ctx context.Context, issue db.Issue, mode string, triggerCommentID ...pgtype.UUID) error {
	_, err := h.TaskService.EnqueueTaskForIssueInMode(ctx, issue, mode, triggerCommentID...)
	if err != nil {
		h.handleAutoEnqueueError(ctx, issue, issue.AssigneeID, err, triggerCommentID...)
	}
	return err
}

func (h *Handler) enqueueMentionTaskWithWarning(ctx context.Context, issue db.Issue, agentID pgtype.UUID, triggerCommentID pgtype.UUID) error {
	_, err := h.TaskService.EnqueueTaskForMention(ctx, issue, agentID, triggerCommentID)
	if err != nil {
		h.handleAutoEnqueueError(ctx, issue, agentID, err, triggerCommentID)
	}
	return err
}

func (h *Handler) handleAutoEnqueueError(ctx context.Context, issue db.Issue, agentID pgtype.UUID, err error, parentID ...pgtype.UUID) {
	if err == nil {
		return
	}
	slog.Warn("auto-enqueue failed", "issue_id", uuidToString(issue.ID), "agent_id", uuidToString(agentID), "error", err)

	var unsupported *service.UnsupportedExecutionModeError
	if !errors.As(err, &unsupported) || !agentID.Valid {
		return
	}

	var replyTo pgtype.UUID
	if len(parentID) > 0 {
		replyTo = parentID[0]
	}
	h.createAgentSystemWarningComment(ctx, issue, agentID, unsupported.UserMessage(), replyTo)
}

func (h *Handler) createAgentSystemWarningComment(ctx context.Context, issue db.Issue, agentID pgtype.UUID, content string, parentID pgtype.UUID) {
	content = strings.TrimSpace(content)
	if content == "" || !agentID.Valid {
		return
	}

	existing, err := h.Queries.ListCommentsPaginated(ctx, db.ListCommentsPaginatedParams{
		IssueID:     issue.ID,
		WorkspaceID: issue.WorkspaceID,
		Limit:       20,
		Offset:      0,
	})
	if err == nil {
		for _, comment := range existing {
			if comment.AuthorType == "agent" &&
				uuidToString(comment.AuthorID) == uuidToString(agentID) &&
				comment.Type == "system" &&
				strings.TrimSpace(comment.Content) == content {
				return
			}
		}
	}

	comment, err := h.Queries.CreateComment(ctx, db.CreateCommentParams{
		IssueID:     issue.ID,
		WorkspaceID: issue.WorkspaceID,
		AuthorType:  "agent",
		AuthorID:    agentID,
		Content:     content,
		Type:        "system",
		ParentID:    parentID,
	})
	if err != nil {
		slog.Warn("create system warning comment failed", "issue_id", uuidToString(issue.ID), "error", err)
		return
	}

	h.publish(protocol.EventCommentCreated, uuidToString(issue.WorkspaceID), "agent", uuidToString(agentID), map[string]any{
		"comment":      commentToResponse(comment, nil, nil),
		"issue_title":  issue.Title,
		"issue_status": issue.Status,
	})
}
