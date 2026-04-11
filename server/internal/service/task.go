package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/mention"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/multica-ai/multica/server/pkg/redact"
)

type TaskService struct {
	Queries *db.Queries
	Hub     *realtime.Hub
	Bus     *events.Bus
}

func NewTaskService(q *db.Queries, hub *realtime.Hub, bus *events.Bus) *TaskService {
	return &TaskService{Queries: q, Hub: hub, Bus: bus}
}

// EnqueueTaskForIssue creates a queued task for an agent-assigned issue.
// No context snapshot is stored — the agent fetches all data it needs at
// runtime via the multica CLI.
func (s *TaskService) EnqueueTaskForIssue(ctx context.Context, issue db.Issue, triggerCommentID ...pgtype.UUID) (db.AgentTaskQueue, error) {
	return s.EnqueueTaskForIssueInModeWithSource(ctx, issue, PreferredTaskMode(issue), TaskTriggerSourceEvent, triggerCommentID...)
}

// EnqueueTaskForIssueInMode creates a queued task for an agent-assigned issue
// in the explicitly requested execution mode.
func (s *TaskService) EnqueueTaskForIssueInMode(ctx context.Context, issue db.Issue, mode string, triggerCommentID ...pgtype.UUID) (db.AgentTaskQueue, error) {
	return s.EnqueueTaskForIssueInModeWithSource(ctx, issue, mode, TaskTriggerSourceEvent, triggerCommentID...)
}

// EnqueueTaskForIssueInModeWithSource creates a queued task for an
// agent-assigned issue in the explicitly requested execution mode and stores
// the originating trigger source for downstream dedupe logic.
func (s *TaskService) EnqueueTaskForIssueInModeWithSource(ctx context.Context, issue db.Issue, mode, triggerSource string, triggerCommentID ...pgtype.UUID) (db.AgentTaskQueue, error) {
	if !issue.AssigneeID.Valid {
		slog.Error("task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "error", "issue has no assignee")
		return db.AgentTaskQueue{}, fmt.Errorf("issue has no assignee")
	}
	mode = NormalizeTaskMode(mode)
	triggerSource = strings.TrimSpace(triggerSource)
	if triggerSource == "" {
		triggerSource = TaskTriggerSourceEvent
	}

	agent, err := s.Queries.GetAgent(ctx, issue.AssigneeID)
	if err != nil {
		slog.Error("task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("load agent: %w", err)
	}
	if agent.ArchivedAt.Valid {
		slog.Debug("task enqueue skipped: agent is archived", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agent.ID))
		return db.AgentTaskQueue{}, fmt.Errorf("agent is archived")
	}
	if !agent.RuntimeID.Valid {
		slog.Error("task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "error", "agent has no runtime")
		return db.AgentTaskQueue{}, fmt.Errorf("agent has no runtime")
	}
	if err := ValidateRuntimeExecutionModeSupport(ctx, s.Queries, agent.RuntimeID, mode); err != nil {
		slog.Warn("task enqueue blocked by runtime capability", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agent.ID), "mode", mode, "error", err)
		return db.AgentTaskQueue{}, err
	}

	var commentID pgtype.UUID
	if len(triggerCommentID) > 0 {
		commentID = triggerCommentID[0]
	}

	if _, _, err := s.setIssueExecutionStage(ctx, issue, QueueStageForMode(mode)); err != nil {
		slog.Warn("task enqueue: failed to update execution stage", "issue_id", util.UUIDToString(issue.ID), "mode", mode, "error", err)
	}

	task, err := s.Queries.CreateAgentTask(ctx, db.CreateAgentTaskParams{
		AgentID:          issue.AssigneeID,
		RuntimeID:        agent.RuntimeID,
		IssueID:          issue.ID,
		Priority:         priorityToInt(issue.Priority),
		Mode:             mode,
		TriggerCommentID: commentID,
		TriggerSource:    triggerSource,
	})
	if err != nil {
		slog.Error("task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("create task: %w", err)
	}

	slog.Info("task enqueued", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(issue.AssigneeID))
	return task, nil
}

// EnqueueTaskForMention creates a queued task for a mentioned agent on an issue.
// Unlike EnqueueTaskForIssue, this takes an explicit agent ID rather than
// deriving it from the issue assignee.
func (s *TaskService) EnqueueTaskForMention(ctx context.Context, issue db.Issue, agentID pgtype.UUID, triggerCommentID pgtype.UUID) (db.AgentTaskQueue, error) {
	return s.EnqueueTaskForMentionInModeWithSource(ctx, issue, agentID, PreferredTaskMode(issue), TaskTriggerSourceEvent, triggerCommentID)
}

// EnqueueTaskForMentionInMode creates a queued task for a mentioned agent in
// the explicitly requested execution mode.
func (s *TaskService) EnqueueTaskForMentionInMode(ctx context.Context, issue db.Issue, agentID pgtype.UUID, mode string, triggerCommentID pgtype.UUID) (db.AgentTaskQueue, error) {
	return s.EnqueueTaskForMentionInModeWithSource(ctx, issue, agentID, mode, TaskTriggerSourceEvent, triggerCommentID)
}

// EnqueueTaskForMentionInModeWithSource creates a queued task for a mentioned
// agent in the explicitly requested execution mode and records the trigger
// source for downstream dedupe logic.
func (s *TaskService) EnqueueTaskForMentionInModeWithSource(ctx context.Context, issue db.Issue, agentID pgtype.UUID, mode, triggerSource string, triggerCommentID pgtype.UUID) (db.AgentTaskQueue, error) {
	mode = NormalizeTaskMode(mode)
	triggerSource = strings.TrimSpace(triggerSource)
	if triggerSource == "" {
		triggerSource = TaskTriggerSourceEvent
	}
	agent, err := s.Queries.GetAgent(ctx, agentID)
	if err != nil {
		slog.Error("mention task enqueue failed: agent not found", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("load agent: %w", err)
	}
	if agent.ArchivedAt.Valid {
		slog.Debug("mention task enqueue skipped: agent is archived", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID))
		return db.AgentTaskQueue{}, fmt.Errorf("agent is archived")
	}
	if !agent.RuntimeID.Valid {
		slog.Error("mention task enqueue failed: agent has no runtime", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID))
		return db.AgentTaskQueue{}, fmt.Errorf("agent has no runtime")
	}
	if err := ValidateRuntimeExecutionModeSupport(ctx, s.Queries, agent.RuntimeID, mode); err != nil {
		slog.Warn("mention task enqueue blocked by runtime capability", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID), "mode", mode, "error", err)
		return db.AgentTaskQueue{}, err
	}

	if _, _, err := s.setIssueExecutionStage(ctx, issue, QueueStageForMode(mode)); err != nil {
		slog.Warn("mention task enqueue: failed to update execution stage", "issue_id", util.UUIDToString(issue.ID), "mode", mode, "error", err)
	}

	task, err := s.Queries.CreateAgentTask(ctx, db.CreateAgentTaskParams{
		AgentID:          agentID,
		RuntimeID:        agent.RuntimeID,
		IssueID:          issue.ID,
		Priority:         priorityToInt(issue.Priority),
		Mode:             mode,
		TriggerCommentID: triggerCommentID,
		TriggerSource:    triggerSource,
	})
	if err != nil {
		slog.Error("mention task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("create task: %w", err)
	}

	slog.Info("mention task enqueued", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID))
	return task, nil
}

// CancelTasksForIssue cancels all active tasks for an issue.
func (s *TaskService) CancelTasksForIssue(ctx context.Context, issueID pgtype.UUID) error {
	return s.Queries.CancelAgentTasksByIssue(ctx, issueID)
}

// CancelTask cancels a single task by ID. It broadcasts a task:cancelled event
// so frontends can update immediately.
func (s *TaskService) CancelTask(ctx context.Context, taskID pgtype.UUID) (*db.AgentTaskQueue, error) {
	task, err := s.Queries.CancelAgentTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("cancel task: %w", err)
	}

	slog.Info("task cancelled", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID))

	// Reconcile agent status
	s.ReconcileAgentStatus(ctx, task.AgentID)
	s.syncIssueExecutionStageForTask(ctx, task, FailedStageForMode(task.Mode))

	// Broadcast cancellation as a task:failed event so frontends clear the live card
	s.broadcastTaskEvent(ctx, protocol.EventTaskCancelled, task)

	return &task, nil
}

// ClaimTask atomically claims the next queued task for an agent,
// respecting max_concurrent_tasks.
func (s *TaskService) ClaimTask(ctx context.Context, agentID pgtype.UUID) (*db.AgentTaskQueue, error) {
	agent, err := s.Queries.GetAgent(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	running, err := s.Queries.CountRunningTasks(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("count running tasks: %w", err)
	}
	if running >= int64(agent.MaxConcurrentTasks) {
		slog.Debug("task claim: no capacity", "agent_id", util.UUIDToString(agentID), "running", running, "max", agent.MaxConcurrentTasks)
		return nil, nil // No capacity
	}

	task, err := s.Queries.ClaimAgentTask(ctx, agentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Debug("task claim: no tasks available", "agent_id", util.UUIDToString(agentID))
			return nil, nil // No tasks available
		}
		return nil, fmt.Errorf("claim task: %w", err)
	}

	slog.Info("task claimed", "task_id", util.UUIDToString(task.ID), "agent_id", util.UUIDToString(agentID))

	// Update agent status to working
	s.updateAgentStatus(ctx, agentID, "working")

	// Broadcast task:dispatch
	s.broadcastTaskDispatch(ctx, task)

	return &task, nil
}

// ClaimTaskForRuntime claims the next runnable task for a runtime while
// still respecting each agent's max_concurrent_tasks limit.
func (s *TaskService) ClaimTaskForRuntime(ctx context.Context, runtimeID pgtype.UUID) (*db.AgentTaskQueue, error) {
	tasks, err := s.Queries.ListPendingTasksByRuntime(ctx, runtimeID)
	if err != nil {
		return nil, fmt.Errorf("list pending tasks: %w", err)
	}

	triedAgents := map[string]struct{}{}
	for _, candidate := range tasks {
		agentKey := util.UUIDToString(candidate.AgentID)
		if _, seen := triedAgents[agentKey]; seen {
			continue
		}
		triedAgents[agentKey] = struct{}{}

		task, err := s.ClaimTask(ctx, candidate.AgentID)
		if err != nil {
			return nil, err
		}
		if task != nil && task.RuntimeID == runtimeID {
			return task, nil
		}
	}

	return nil, nil
}

// StartTask transitions a dispatched task to running.
// Issue status is NOT changed here — the agent manages it via the CLI.
func (s *TaskService) StartTask(ctx context.Context, taskID pgtype.UUID) (*db.AgentTaskQueue, error) {
	task, err := s.Queries.StartAgentTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("start task: %w", err)
	}

	s.syncIssueExecutionStageForTask(ctx, task, RunningStageForMode(task.Mode))
	slog.Info("task started", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID))
	return &task, nil
}

// CompleteTask marks a task as completed.
// Issue status is NOT changed here — the agent manages it via the CLI.
func (s *TaskService) CompleteTask(ctx context.Context, taskID pgtype.UUID, result []byte, sessionID, workDir string) (*db.AgentTaskQueue, error) {
	task, err := s.Queries.CompleteAgentTask(ctx, db.CompleteAgentTaskParams{
		ID:        taskID,
		Result:    result,
		SessionID: pgtype.Text{String: sessionID, Valid: sessionID != ""},
		WorkDir:   pgtype.Text{String: workDir, Valid: workDir != ""},
	})
	if err != nil {
		// Log the current task state to help debug why the update matched no rows.
		if existing, lookupErr := s.Queries.GetAgentTask(ctx, taskID); lookupErr == nil {
			slog.Warn("complete task failed: task not in running state",
				"task_id", util.UUIDToString(taskID),
				"current_status", existing.Status,
				"issue_id", util.UUIDToString(existing.IssueID),
				"agent_id", util.UUIDToString(existing.AgentID),
			)
		} else {
			slog.Warn("complete task failed: task not found",
				"task_id", util.UUIDToString(taskID),
				"lookup_error", lookupErr,
			)
		}
		return nil, fmt.Errorf("complete task: %w", err)
	}

	slog.Info("task completed", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID))

	var payload protocol.TaskCompletedPayload
	if len(result) > 0 {
		_ = json.Unmarshal(result, &payload)
	}

	if NormalizeTaskMode(task.Mode) == TaskModePlan {
		latestPlan := s.latestPlanArtifact(ctx, task.IssueID, task.ID)
		revision, revErr := s.nextPlanRevision(ctx, task.IssueID, task.ID)
		if revErr == nil {
			payload.PlanRevision = revision
		}
		if payload.PlanThreadRootID == "" && latestPlan != nil {
			payload.PlanThreadRootID = strings.TrimSpace(latestPlan.PlanThreadRootID)
		}
		if payload.PlanStatus == "" {
			payload.PlanStatus = "ready"
		}
		if len(payload.PlanQuestions) > 0 {
			payload.PlanRequiresDecision = true
		}
		if payload.PlanRequiresDecision {
			payload.PlanStatus = "draft"
		}
		if strings.TrimSpace(payload.PlanStatus) == "draft" {
			payload.PlanRequiresDecision = true
		}

		commentID, rootID, commentErr := s.CreateTaskPlanRevisionComment(ctx, task, &payload)
		if commentErr != nil {
			slog.Warn("upsert task plan comment failed", "task_id", util.UUIDToString(task.ID), "error", commentErr)
		} else {
			payload.PlanCommentID = commentID
			payload.PlanThreadRootID = rootID
		}

		if resultBytes, marshalErr := json.Marshal(payload); marshalErr == nil {
			if updateErr := s.updateTaskResult(ctx, task.ID, resultBytes); updateErr != nil {
				slog.Warn("update task result after plan completion failed", "task_id", util.UUIDToString(task.ID), "error", updateErr)
			} else {
				task.Result = resultBytes
			}
		}
	}

	s.syncIssueExecutionStageForTask(ctx, task, CompletedStageForMode(task.Mode))

	// Reconcile agent status
	s.ReconcileAgentStatus(ctx, task.AgentID)

	// Broadcast
	s.broadcastTaskEvent(ctx, protocol.EventTaskCompleted, task)

	return &task, nil
}

// FailTask marks a task as failed.
// Issue status is NOT changed here — the agent manages it via the CLI.
func (s *TaskService) FailTask(ctx context.Context, taskID pgtype.UUID, errMsg string) (*db.AgentTaskQueue, error) {
	task, err := s.Queries.FailAgentTask(ctx, db.FailAgentTaskParams{
		ID:    taskID,
		Error: pgtype.Text{String: errMsg, Valid: true},
	})
	if err != nil {
		if existing, lookupErr := s.Queries.GetAgentTask(ctx, taskID); lookupErr == nil {
			slog.Warn("fail task failed: task not in dispatched/running state",
				"task_id", util.UUIDToString(taskID),
				"current_status", existing.Status,
				"issue_id", util.UUIDToString(existing.IssueID),
				"agent_id", util.UUIDToString(existing.AgentID),
			)
		} else {
			slog.Warn("fail task failed: task not found",
				"task_id", util.UUIDToString(taskID),
				"lookup_error", lookupErr,
			)
		}
		return nil, fmt.Errorf("fail task: %w", err)
	}

	slog.Warn("task failed", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID), "error", errMsg)

	if errMsg != "" {
		s.createAgentComment(ctx, task.IssueID, task.AgentID, redact.Text(errMsg), "system", task.TriggerCommentID)
	}
	// Reconcile agent status
	s.ReconcileAgentStatus(ctx, task.AgentID)
	s.syncIssueExecutionStageForTask(ctx, task, FailedStageForMode(task.Mode))

	// Broadcast
	s.broadcastTaskEvent(ctx, protocol.EventTaskFailed, task)

	return &task, nil
}

// ReportProgress broadcasts a progress update via the event bus.
func (s *TaskService) ReportProgress(ctx context.Context, taskID string, workspaceID string, summary string, step, total int) {
	s.Bus.Publish(events.Event{
		Type:        protocol.EventTaskProgress,
		WorkspaceID: workspaceID,
		ActorType:   "system",
		ActorID:     "",
		Payload: protocol.TaskProgressPayload{
			TaskID:  taskID,
			Summary: summary,
			Step:    step,
			Total:   total,
		},
	})
}

// ReconcileAgentStatus checks running task count and sets agent status accordingly.
func (s *TaskService) ReconcileAgentStatus(ctx context.Context, agentID pgtype.UUID) {
	running, err := s.Queries.CountRunningTasks(ctx, agentID)
	if err != nil {
		return
	}
	newStatus := "idle"
	if running > 0 {
		newStatus = "working"
	}
	slog.Debug("agent status reconciled", "agent_id", util.UUIDToString(agentID), "status", newStatus, "running_tasks", running)
	s.updateAgentStatus(ctx, agentID, newStatus)
}

func (s *TaskService) updateAgentStatus(ctx context.Context, agentID pgtype.UUID, status string) {
	agent, err := s.Queries.UpdateAgentStatus(ctx, db.UpdateAgentStatusParams{
		ID:     agentID,
		Status: status,
	})
	if err != nil {
		return
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventAgentStatus,
		WorkspaceID: util.UUIDToString(agent.WorkspaceID),
		ActorType:   "system",
		ActorID:     "",
		Payload:     map[string]any{"agent": agentToMap(agent)},
	})
}

// LoadAgentSkills loads an agent's skills with their files for task execution.
func (s *TaskService) LoadAgentSkills(ctx context.Context, agentID pgtype.UUID) []AgentSkillData {
	skills, err := s.Queries.ListAgentSkills(ctx, agentID)
	if err != nil || len(skills) == 0 {
		return nil
	}

	result := make([]AgentSkillData, 0, len(skills))
	for _, sk := range skills {
		data := AgentSkillData{Name: sk.Name, Content: sk.Content}
		files, _ := s.Queries.ListSkillFiles(ctx, sk.ID)
		for _, f := range files {
			data.Files = append(data.Files, AgentSkillFileData{Path: f.Path, Content: f.Content})
		}
		result = append(result, data)
	}
	return result
}

// AgentSkillData represents a skill for task execution responses.
type AgentSkillData struct {
	Name    string               `json:"name"`
	Content string               `json:"content"`
	Files   []AgentSkillFileData `json:"files,omitempty"`
}

// AgentSkillFileData represents a supporting file within a skill.
type AgentSkillFileData struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func priorityToInt(p string) int32 {
	switch p {
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

func (s *TaskService) updateTaskResult(ctx context.Context, taskID pgtype.UUID, result []byte) error {
	return s.Queries.UpdateAgentTaskResult(ctx, db.UpdateAgentTaskResultParams{
		ID:     taskID,
		Result: result,
	})
}

func (s *TaskService) nextPlanRevision(ctx context.Context, issueID pgtype.UUID, excludeTaskID pgtype.UUID) (int, error) {
	latest := s.latestPlanArtifact(ctx, issueID, excludeTaskID)
	if latest != nil && latest.PlanRevision > 0 {
		return latest.PlanRevision + 1, nil
	}
	return 1, nil
}

func (s *TaskService) latestPlanArtifact(ctx context.Context, issueID pgtype.UUID, excludeTaskID ...pgtype.UUID) *protocol.TaskCompletedPayload {
	tasks, err := s.Queries.ListTasksByIssue(ctx, issueID)
	if err != nil {
		return nil
	}
	excluded := ""
	if len(excludeTaskID) > 0 {
		excluded = util.UUIDToString(excludeTaskID[0])
	}
	for _, task := range tasks {
		if excluded != "" && util.UUIDToString(task.ID) == excluded {
			continue
		}
		if task.Status != "completed" || NormalizeTaskMode(task.Mode) != TaskModePlan || len(task.Result) == 0 {
			continue
		}
		var payload protocol.TaskCompletedPayload
		if err := json.Unmarshal(task.Result, &payload); err != nil {
			continue
		}
		return &payload
	}
	return nil
}

func (s *TaskService) setIssueExecutionStage(ctx context.Context, issue db.Issue, stage string) (db.Issue, bool, error) {
	stage = NormalizeExecutionStage(stage)
	if NormalizeExecutionStage(issue.ExecutionStage) == stage {
		return issue, false, nil
	}
	updated, err := s.Queries.UpdateIssueExecutionStage(ctx, db.UpdateIssueExecutionStageParams{
		ID:             issue.ID,
		ExecutionStage: stage,
	})
	if err != nil {
		return db.Issue{}, false, err
	}
	s.broadcastIssueUpdated(updated)
	return updated, true, nil
}

func (s *TaskService) syncIssueExecutionStageForTask(ctx context.Context, task db.AgentTaskQueue, stage string) {
	issue, err := s.Queries.GetIssue(ctx, task.IssueID)
	if err != nil {
		return
	}
	if _, _, err := s.setIssueExecutionStage(ctx, issue, stage); err != nil {
		slog.Warn("task stage sync failed", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID), "stage", stage, "error", err)
	}
}

func (s *TaskService) broadcastTaskDispatch(ctx context.Context, task db.AgentTaskQueue) {
	var payload map[string]any
	if task.Context != nil {
		json.Unmarshal(task.Context, &payload)
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payload["task_id"] = util.UUIDToString(task.ID)
	payload["runtime_id"] = util.UUIDToString(task.RuntimeID)

	workspaceID := ""
	if issue, err := s.Queries.GetIssue(ctx, task.IssueID); err == nil {
		workspaceID = util.UUIDToString(issue.WorkspaceID)
	}
	if workspaceID == "" {
		return // Issue deleted; skip broadcast to avoid global leak
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventTaskDispatch,
		WorkspaceID: workspaceID,
		ActorType:   "system",
		ActorID:     "",
		Payload:     payload,
	})
}

func (s *TaskService) broadcastTaskEvent(ctx context.Context, eventType string, task db.AgentTaskQueue) {
	workspaceID := ""
	if issue, err := s.Queries.GetIssue(ctx, task.IssueID); err == nil {
		workspaceID = util.UUIDToString(issue.WorkspaceID)
	}
	if workspaceID == "" {
		return // Issue deleted; skip broadcast to avoid global leak
	}
	var result any
	if len(task.Result) > 0 {
		_ = json.Unmarshal(task.Result, &result)
	}
	s.Bus.Publish(events.Event{
		Type:        eventType,
		WorkspaceID: workspaceID,
		ActorType:   "system",
		ActorID:     "",
		Payload: map[string]any{
			"task_id":  util.UUIDToString(task.ID),
			"agent_id": util.UUIDToString(task.AgentID),
			"issue_id": util.UUIDToString(task.IssueID),
			"status":   task.Status,
			"result":   result,
		},
	})
}

func (s *TaskService) broadcastIssueUpdated(issue db.Issue) {
	prefix := s.getIssuePrefix(issue.WorkspaceID)
	s.Bus.Publish(events.Event{
		Type:        protocol.EventIssueUpdated,
		WorkspaceID: util.UUIDToString(issue.WorkspaceID),
		ActorType:   "system",
		ActorID:     "",
		Payload:     map[string]any{"issue": issueToMap(issue, prefix)},
	})
}

func (s *TaskService) getIssuePrefix(workspaceID pgtype.UUID) string {
	ws, err := s.Queries.GetWorkspace(context.Background(), workspaceID)
	if err != nil {
		return ""
	}
	return ws.IssuePrefix
}

func (s *TaskService) createAgentComment(ctx context.Context, issueID, agentID pgtype.UUID, content, commentType string, parentID pgtype.UUID) {
	if content == "" {
		return
	}
	// Look up issue to get workspace ID for mention expansion and broadcasting.
	issue, err := s.Queries.GetIssue(ctx, issueID)
	if err != nil {
		return
	}
	// Expand bare issue identifiers (e.g. MUL-117) into mention links.
	content = mention.ExpandIssueIdentifiers(ctx, s.Queries, issue.WorkspaceID, content)
	comment, err := s.Queries.CreateComment(ctx, db.CreateCommentParams{
		IssueID:    issueID,
		AuthorType: "agent",
		AuthorID:   agentID,
		Content:    content,
		Type:       commentType,
		ParentID:   parentID,
	})
	if err != nil {
		return
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventCommentCreated,
		WorkspaceID: util.UUIDToString(issue.WorkspaceID),
		ActorType:   "agent",
		ActorID:     util.UUIDToString(agentID),
		Payload: map[string]any{
			"comment": map[string]any{
				"id":          util.UUIDToString(comment.ID),
				"issue_id":    util.UUIDToString(comment.IssueID),
				"author_type": comment.AuthorType,
				"author_id":   util.UUIDToString(comment.AuthorID),
				"content":     comment.Content,
				"type":        comment.Type,
				"parent_id":   util.UUIDToPtr(comment.ParentID),
				"created_at":  comment.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
			},
			"issue_title":  issue.Title,
			"issue_status": issue.Status,
		},
	})
}

func buildTaskDeliverySummaryComment(payload protocol.TaskCompletedPayload) string {
	summary := strings.TrimSpace(payload.Summary)
	if summary == "" {
		switch strings.TrimSpace(payload.DeliveryState) {
		case "delivered":
			summary = "Delivery ready."
		case "handoff_required":
			summary = "Delivery ready, but PR creation requires handoff."
		default:
			summary = "Run completed."
		}
	}

	lines := []string{summary}
	if branch := strings.TrimSpace(payload.BranchName); branch != "" {
		lines = append(lines, "Branch: `"+branch+"`")
	}
	if prURL := strings.TrimSpace(payload.PRURL); prURL != "" {
		lines = append(lines, "PR: "+prURL)
	} else if compareURL := strings.TrimSpace(payload.CompareURL); compareURL != "" {
		lines = append(lines, "Compare: "+compareURL)
	}
	if reason := strings.TrimSpace(payload.HandoffReason); reason != "" {
		lines = append(lines, "Handoff: "+reason)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func buildTaskPlanComment(payload protocol.TaskCompletedPayload) string {
	output := strings.TrimSpace(payload.Output)
	summary := strings.TrimSpace(payload.Summary)
	if output == "" && summary == "" {
		return ""
	}

	lines := []string{}
	if payload.PlanRevision > 0 {
		lines = append(lines, fmt.Sprintf("Plan revision %d", payload.PlanRevision))
	}
	if summary != "" {
		lines = append(lines, summary)
	}
	if payload.PlanRequiresDecision && len(payload.PlanQuestions) > 0 {
		lines = append(lines, "Open Decisions:\n- "+strings.Join(payload.PlanQuestions, "\n- "))
	}
	if output != "" {
		if summary == "" || !strings.Contains(output, summary) {
			lines = append(lines, output)
		}
	}
	if payload.PlanRequiresDecision {
		lines = append(lines, "Reply in this thread with decisions or requested changes. Confirm Plan will stay disabled until the open decisions are resolved.")
	} else {
		lines = append(lines, "Reply in this thread with decisions or requested changes. Click Confirm Plan when the plan is final.")
	}
	return strings.TrimSpace(strings.Join(lines, "\n\n"))
}

func (s *TaskService) CreateTaskPlanRevisionComment(ctx context.Context, task db.AgentTaskQueue, payload *protocol.TaskCompletedPayload) (string, string, error) {
	if payload == nil {
		return "", "", nil
	}
	content := buildTaskPlanComment(*payload)
	if content == "" {
		return "", "", nil
	}

	issue, err := s.Queries.GetIssue(ctx, task.IssueID)
	if err != nil {
		return "", "", err
	}
	content = mention.ExpandIssueIdentifiers(ctx, s.Queries, issue.WorkspaceID, content)

	rootID := strings.TrimSpace(payload.PlanThreadRootID)
	if rootID == "" {
		if latest := s.latestPlanArtifact(ctx, task.IssueID); latest != nil {
			rootID = strings.TrimSpace(latest.PlanThreadRootID)
			if rootID == "" {
				rootID = strings.TrimSpace(latest.PlanCommentID)
			}
		}
	}

	parentID := task.TriggerCommentID
	if rootID != "" {
		parentID = util.ParseUUID(rootID)
	}

	comment, err := s.Queries.CreateComment(ctx, db.CreateCommentParams{
		IssueID:     task.IssueID,
		WorkspaceID: issue.WorkspaceID,
		AuthorType:  "agent",
		AuthorID:    task.AgentID,
		Content:     content,
		Type:        "system",
		ParentID:    parentID,
	})
	if err != nil {
		return "", "", err
	}
	if rootID == "" {
		rootID = util.UUIDToString(comment.ID)
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventCommentCreated,
		WorkspaceID: util.UUIDToString(issue.WorkspaceID),
		ActorType:   "agent",
		ActorID:     util.UUIDToString(task.AgentID),
		Payload: map[string]any{
			"comment": map[string]any{
				"id":          util.UUIDToString(comment.ID),
				"issue_id":    util.UUIDToString(comment.IssueID),
				"author_type": comment.AuthorType,
				"author_id":   util.UUIDToString(comment.AuthorID),
				"content":     comment.Content,
				"type":        comment.Type,
				"parent_id":   util.UUIDToPtr(comment.ParentID),
				"created_at":  comment.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
				"updated_at":  comment.UpdatedAt.Time.Format("2006-01-02T15:04:05Z"),
				"reactions":   []any{},
				"attachments": []any{},
			},
			"issue_title":  issue.Title,
			"issue_status": issue.Status,
		},
	})
	return util.UUIDToString(comment.ID), rootID, nil
}

func (s *TaskService) UpsertTaskDeliveryComment(ctx context.Context, task db.AgentTaskQueue, payload *protocol.TaskCompletedPayload) (string, error) {
	if payload == nil {
		return "", nil
	}
	content := buildTaskDeliverySummaryComment(*payload)
	if content == "" {
		return "", nil
	}

	issue, err := s.Queries.GetIssue(ctx, task.IssueID)
	if err != nil {
		return "", err
	}
	content = mention.ExpandIssueIdentifiers(ctx, s.Queries, issue.WorkspaceID, content)

	if commentID := strings.TrimSpace(payload.DeliveryCommentID); commentID != "" {
		comment, err := s.Queries.UpdateComment(ctx, db.UpdateCommentParams{
			ID:      util.ParseUUID(commentID),
			Content: content,
		})
		if err == nil {
			s.Bus.Publish(events.Event{
				Type:        protocol.EventCommentUpdated,
				WorkspaceID: util.UUIDToString(issue.WorkspaceID),
				ActorType:   "agent",
				ActorID:     util.UUIDToString(task.AgentID),
				Payload: map[string]any{
					"comment": map[string]any{
						"id":          util.UUIDToString(comment.ID),
						"issue_id":    util.UUIDToString(comment.IssueID),
						"author_type": comment.AuthorType,
						"author_id":   util.UUIDToString(comment.AuthorID),
						"content":     comment.Content,
						"type":        comment.Type,
						"parent_id":   util.UUIDToPtr(comment.ParentID),
						"created_at":  comment.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
						"updated_at":  comment.UpdatedAt.Time.Format("2006-01-02T15:04:05Z"),
						"reactions":   []any{},
						"attachments": []any{},
					},
				},
			})
			return util.UUIDToString(comment.ID), nil
		}
	}

	comment, err := s.Queries.CreateComment(ctx, db.CreateCommentParams{
		IssueID:     task.IssueID,
		WorkspaceID: issue.WorkspaceID,
		AuthorType:  "agent",
		AuthorID:    task.AgentID,
		Content:     content,
		Type:        "system",
		ParentID:    task.TriggerCommentID,
	})
	if err != nil {
		return "", err
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventCommentCreated,
		WorkspaceID: util.UUIDToString(issue.WorkspaceID),
		ActorType:   "agent",
		ActorID:     util.UUIDToString(task.AgentID),
		Payload: map[string]any{
			"comment": map[string]any{
				"id":          util.UUIDToString(comment.ID),
				"issue_id":    util.UUIDToString(comment.IssueID),
				"author_type": comment.AuthorType,
				"author_id":   util.UUIDToString(comment.AuthorID),
				"content":     comment.Content,
				"type":        comment.Type,
				"parent_id":   util.UUIDToPtr(comment.ParentID),
				"created_at":  comment.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
				"updated_at":  comment.UpdatedAt.Time.Format("2006-01-02T15:04:05Z"),
				"reactions":   []any{},
				"attachments": []any{},
			},
			"issue_title":  issue.Title,
			"issue_status": issue.Status,
		},
	})
	return util.UUIDToString(comment.ID), nil
}

func issueToMap(issue db.Issue, issuePrefix string) map[string]any {
	return map[string]any{
		"id":              util.UUIDToString(issue.ID),
		"workspace_id":    util.UUIDToString(issue.WorkspaceID),
		"number":          issue.Number,
		"identifier":      issuePrefix + "-" + strconv.Itoa(int(issue.Number)),
		"title":           issue.Title,
		"description":     util.TextToPtr(issue.Description),
		"status":          issue.Status,
		"priority":        issue.Priority,
		"assignee_type":   util.TextToPtr(issue.AssigneeType),
		"assignee_id":     util.UUIDToPtr(issue.AssigneeID),
		"creator_type":    issue.CreatorType,
		"creator_id":      util.UUIDToString(issue.CreatorID),
		"parent_issue_id": util.UUIDToPtr(issue.ParentIssueID),
		"position":        issue.Position,
		"due_date":        util.TimestampToPtr(issue.DueDate),
		"execution_stage": NormalizeExecutionStage(issue.ExecutionStage),
		"created_at":      util.TimestampToString(issue.CreatedAt),
		"updated_at":      util.TimestampToString(issue.UpdatedAt),
	}
}

// agentToMap builds a simple map for broadcasting agent status updates.
func agentToMap(a db.Agent) map[string]any {
	var rc any
	if a.RuntimeConfig != nil {
		json.Unmarshal(a.RuntimeConfig, &rc)
	}
	var tools any
	if a.Tools != nil {
		json.Unmarshal(a.Tools, &tools)
	}
	var triggers any
	if a.Triggers != nil {
		json.Unmarshal(a.Triggers, &triggers)
	}
	return map[string]any{
		"id":                   util.UUIDToString(a.ID),
		"workspace_id":         util.UUIDToString(a.WorkspaceID),
		"runtime_id":           util.UUIDToString(a.RuntimeID),
		"name":                 a.Name,
		"description":          a.Description,
		"avatar_url":           util.TextToPtr(a.AvatarUrl),
		"runtime_mode":         a.RuntimeMode,
		"runtime_config":       rc,
		"visibility":           a.Visibility,
		"status":               a.Status,
		"max_concurrent_tasks": a.MaxConcurrentTasks,
		"owner_id":             util.UUIDToPtr(a.OwnerID),
		"skills":               []any{},
		"tools":                tools,
		"triggers":             triggers,
		"created_at":           util.TimestampToString(a.CreatedAt),
		"updated_at":           util.TimestampToString(a.UpdatedAt),
		"archived_at":          util.TimestampToPtr(a.ArchivedAt),
		"archived_by":          util.UUIDToPtr(a.ArchivedBy),
	}
}
