package main

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func setTestAgentTriggers(t *testing.T, agentID string, triggers any) {
	t.Helper()
	payload, err := json.Marshal(triggers)
	if err != nil {
		t.Fatalf("marshal triggers: %v", err)
	}
	if _, err := testPool.Exec(context.Background(), `UPDATE agent SET triggers = $2::jsonb WHERE id = $1`, agentID, string(payload)); err != nil {
		t.Fatalf("update agent triggers: %v", err)
	}
}

func createScheduledIssueFixture(t *testing.T, agentID, title, executionStage string) string {
	t.Helper()
	var issueID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO issue (
			workspace_id, title, status, priority, creator_type, creator_id,
			assignee_type, assignee_id, position, execution_stage
		)
		VALUES ($1, $2, 'todo', 'medium', 'member', $3, 'agent', $4, 0, $5)
		RETURNING id::text
	`, testWorkspaceID, title, testUserID, agentID, executionStage).Scan(&issueID); err != nil {
		t.Fatalf("insert scheduled issue: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE issue_id = $1`, issueID)
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
	})
	return issueID
}

func createIdeaRootIssueFixture(t *testing.T, agentID, title, repoStatus string) string {
	t.Helper()
	store := service.NewIdeaStore()
	seq := int(time.Now().UnixNano() % 1000000)
	code := "idea" + strconv.Itoa(seq)
	slug := code + "-scheduled-root"
	record, err := store.InsertIdea(context.Background(), testPool, service.IdeaRecord{
		WorkspaceID:       testWorkspaceID,
		OwnerUserID:       testUserID,
		SeqNo:             seq,
		Code:              code,
		SlugSuffix:        "scheduled-root",
		SlugFull:          slug,
		Title:             "Scheduled Root",
		RawInput:          "scheduled root",
		Summary:           "scheduled root",
		Tags:              []string{"scheduled"},
		IdeaPath:          "ideas/" + slug + "/" + slug + ".md",
		ProjectRepoName:   slug,
		ProjectRepoURL:    "https://github.com/example/" + slug,
		ProjectRepoStatus: repoStatus,
	})
	if err != nil {
		t.Fatalf("insert idea: %v", err)
	}

	var issueID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO issue (
			workspace_id, title, status, priority, creator_type, creator_id,
			assignee_type, assignee_id, position, idea_id
		)
		VALUES ($1, $2, 'todo', 'medium', 'member', $3, 'agent', $4, 0, $5::uuid)
		RETURNING id::text
	`, testWorkspaceID, title, testUserID, agentID, record.ID).Scan(&issueID); err != nil {
		t.Fatalf("insert root issue: %v", err)
	}

	if err := store.UpdateIdeaRootIssue(context.Background(), testPool, record.ID, issueID); err != nil {
		t.Fatalf("update root issue: %v", err)
	}

	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE issue_id = $1`, issueID)
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
		testPool.Exec(context.Background(), `DELETE FROM idea WHERE id = $1`, record.ID)
	})
	return issueID
}

func countTasksForIssue(t *testing.T, issueID string) int {
	t.Helper()
	var count int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, issueID).Scan(&count); err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	return count
}

func TestScheduledTriggerDueAtMatchesCurrentMinute(t *testing.T) {
	now := time.Date(2026, time.April, 13, 9, 0, 30, 0, time.UTC)

	dueAt, due, err := scheduledTriggerDueAt(now, map[string]any{
		"cron":     "0 9 * * 1-5",
		"timezone": "UTC",
	})
	if err != nil {
		t.Fatalf("scheduledTriggerDueAt: %v", err)
	}
	if !due {
		t.Fatal("expected schedule to be due")
	}
	if dueAt != time.Date(2026, time.April, 13, 9, 0, 0, 0, time.UTC) {
		t.Fatalf("unexpected dueAt: %s", dueAt)
	}
}

func TestScheduledTriggerWorkerQueuesPlanTaskForPlanReadyIssue(t *testing.T) {
	queries := db.New(testPool)
	taskService := service.NewTaskService(queries, nil, events.New())
	agentID := testAgentIDForWorkspace(t, testWorkspaceID)

	setTestAgentTriggers(t, agentID, []service.AgentTriggerSnapshot{
		{Type: service.TriggerOnScheduled, Enabled: true, Config: map[string]any{"cron": "* * * * *", "timezone": "UTC"}},
	})
	t.Cleanup(func() { setTestAgentTriggers(t, agentID, []service.AgentTriggerSnapshot{}) })

	issueID := createScheduledIssueFixture(t, agentID, "Scheduled plan issue", "plan_ready")

	sweepScheduledTriggers(context.Background(), testPool, queries, taskService, service.NewIdeaStore(), time.Date(2026, time.April, 12, 9, 0, 0, 0, time.UTC))

	var mode, source string
	if err := testPool.QueryRow(context.Background(), `
		SELECT mode, trigger_source
		FROM agent_task_queue
		WHERE issue_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, issueID).Scan(&mode, &source); err != nil {
		t.Fatalf("load scheduled task: %v", err)
	}
	if mode != service.TaskModePlan {
		t.Fatalf("expected scheduled task mode %q, got %q", service.TaskModePlan, mode)
	}
	if source != service.TaskTriggerSourceScheduled {
		t.Fatalf("expected trigger_source %q, got %q", service.TaskTriggerSourceScheduled, source)
	}
}

func TestScheduledTriggerWorkerSkipsIdeaRootUntilReady(t *testing.T) {
	queries := db.New(testPool)
	taskService := service.NewTaskService(queries, nil, events.New())
	agentID := testAgentIDForWorkspace(t, testWorkspaceID)

	setTestAgentTriggers(t, agentID, []service.AgentTriggerSnapshot{
		{Type: service.TriggerOnScheduled, Enabled: true, Config: map[string]any{"cron": "* * * * *", "timezone": "UTC"}},
	})
	t.Cleanup(func() { setTestAgentTriggers(t, agentID, []service.AgentTriggerSnapshot{}) })

	issueID := createIdeaRootIssueFixture(t, agentID, "Idea root not ready", "creating")

	sweepScheduledTriggers(context.Background(), testPool, queries, taskService, service.NewIdeaStore(), time.Date(2026, time.April, 12, 9, 0, 0, 0, time.UTC))

	if got := countTasksForIssue(t, issueID); got != 0 {
		t.Fatalf("expected no scheduled task for unready root issue, got %d", got)
	}
}

func TestScheduledTriggerWorkerQueuesReadyIdeaRootIssue(t *testing.T) {
	queries := db.New(testPool)
	taskService := service.NewTaskService(queries, nil, events.New())
	agentID := testAgentIDForWorkspace(t, testWorkspaceID)

	setTestAgentTriggers(t, agentID, []service.AgentTriggerSnapshot{
		{Type: service.TriggerOnScheduled, Enabled: true, Config: map[string]any{"cron": "* * * * *", "timezone": "UTC"}},
	})
	t.Cleanup(func() { setTestAgentTriggers(t, agentID, []service.AgentTriggerSnapshot{}) })

	issueID := createIdeaRootIssueFixture(t, agentID, "Idea root ready", "ready")

	sweepScheduledTriggers(context.Background(), testPool, queries, taskService, service.NewIdeaStore(), time.Date(2026, time.April, 12, 9, 0, 0, 0, time.UTC))

	var source string
	if err := testPool.QueryRow(context.Background(), `
		SELECT trigger_source
		FROM agent_task_queue
		WHERE issue_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, issueID).Scan(&source); err != nil {
		t.Fatalf("load scheduled root task: %v", err)
	}
	if source != service.TaskTriggerSourceScheduled {
		t.Fatalf("expected trigger_source %q, got %q", service.TaskTriggerSourceScheduled, source)
	}
}

func TestScheduledTriggerWorkerSkipsSecondEnqueueWithinSameMinute(t *testing.T) {
	queries := db.New(testPool)
	taskService := service.NewTaskService(queries, nil, events.New())
	agentID := testAgentIDForWorkspace(t, testWorkspaceID)

	setTestAgentTriggers(t, agentID, []service.AgentTriggerSnapshot{
		{Type: service.TriggerOnScheduled, Enabled: true, Config: map[string]any{"cron": "* * * * *", "timezone": "UTC"}},
	})
	t.Cleanup(func() { setTestAgentTriggers(t, agentID, []service.AgentTriggerSnapshot{}) })

	issueID := createScheduledIssueFixture(t, agentID, "Scheduled dedupe issue", "idle")
	now := time.Date(2026, time.April, 12, 9, 0, 0, 0, time.UTC)

	sweepScheduledTriggers(context.Background(), testPool, queries, taskService, service.NewIdeaStore(), now)

	if _, err := testPool.Exec(context.Background(), `
		UPDATE agent_task_queue SET status = 'completed', completed_at = now()
		WHERE issue_id = $1
	`, issueID); err != nil {
		t.Fatalf("complete first scheduled task: %v", err)
	}

	sweepScheduledTriggers(context.Background(), testPool, queries, taskService, service.NewIdeaStore(), now.Add(30*time.Second))

	if got := countTasksForIssue(t, issueID); got != 1 {
		t.Fatalf("expected one scheduled task within the same due minute, got %d", got)
	}

	var source string
	if err := testPool.QueryRow(context.Background(), `
		SELECT trigger_source
		FROM agent_task_queue
		WHERE issue_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, issueID).Scan(&source); err != nil {
		t.Fatalf("load latest scheduled task: %v", err)
	}
	if source != service.TaskTriggerSourceScheduled {
		t.Fatalf("expected trigger_source %q, got %q", service.TaskTriggerSourceScheduled, source)
	}
}

func TestScheduledTriggerWorkerLeavesCommentTriggeringUnchanged(t *testing.T) {
	queries := db.New(testPool)
	taskService := service.NewTaskService(queries, nil, events.New())
	agentID := testAgentIDForWorkspace(t, testWorkspaceID)

	setTestAgentTriggers(t, agentID, []service.AgentTriggerSnapshot{
		{Type: service.TriggerOnComment, Enabled: true, Config: map[string]any{}},
		{Type: service.TriggerOnScheduled, Enabled: true, Config: map[string]any{"cron": "* * * * *", "timezone": "UTC"}},
	})
	t.Cleanup(func() { setTestAgentTriggers(t, agentID, []service.AgentTriggerSnapshot{}) })

	issueID := createScheduledIssueFixture(t, agentID, "Scheduled comment coexistence", "planning")
	postComment(t, issueID, "Please revise the current plan", nil)

	if got := countTasksForIssue(t, issueID); got == 0 {
		t.Fatal("expected comment-triggered task to be created")
	}

	if _, err := testPool.Exec(context.Background(), `
		DELETE FROM agent_task_queue WHERE issue_id = $1 AND trigger_source = $2
	`, issueID, service.TaskTriggerSourceEvent); err != nil {
		t.Fatalf("clear event task: %v", err)
	}

	sweepScheduledTriggers(context.Background(), testPool, queries, taskService, service.NewIdeaStore(), time.Date(2026, time.April, 12, 9, 0, 0, 0, time.UTC))

	var source string
	if err := testPool.QueryRow(context.Background(), `
		SELECT trigger_source
		FROM agent_task_queue
		WHERE issue_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, issueID).Scan(&source); err != nil {
		t.Fatalf("load scheduled follow-up task: %v", err)
	}
	if source != service.TaskTriggerSourceScheduled {
		t.Fatalf("expected scheduled source after sweep, got %q", source)
	}
}

func TestIsIdeaRootIssueReadyForAutomation(t *testing.T) {
	store := service.NewIdeaStore()
	agentID := testAgentIDForWorkspace(t, testWorkspaceID)
	issueID := createIdeaRootIssueFixture(t, agentID, "Root readiness helper", "ready")

	issue, err := db.New(testPool).GetIssue(context.Background(), util.ParseUUID(issueID))
	if err != nil {
		t.Fatalf("load issue: %v", err)
	}

	ready, err := isIdeaRootIssueReadyForAutomation(context.Background(), testPool, store, issue)
	if err != nil {
		t.Fatalf("isIdeaRootIssueReadyForAutomation: %v", err)
	}
	if !ready {
		t.Fatal("expected ready root issue to be eligible")
	}
}
