package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type IssuePRJobRecord struct {
	ID          string
	IssueID     string
	Status      string
	Attempts    int
	Payload     map[string]any
	LastError   string
	RunAfter    time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt time.Time
}

type IssuePRStore struct{}

func NewIssuePRStore() *IssuePRStore {
	return &IssuePRStore{}
}

func (s *IssuePRStore) Enqueue(ctx context.Context, dbtx db.DBTX, issueID string, payload map[string]any) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = dbtx.Exec(ctx, `
		INSERT INTO issue_pr_job (issue_id, status, payload)
		VALUES ($1, 'queued', $2::jsonb)
		ON CONFLICT DO NOTHING
	`, issueID, string(payloadJSON))
	return err
}

func (s *IssuePRStore) ClaimNext(ctx context.Context, dbtx db.DBTX) (*IssuePRJobRecord, error) {
	rows, err := dbtx.Query(ctx, `
		UPDATE issue_pr_job
		SET status = 'running',
		    attempts = attempts + 1,
		    locked_at = now(),
		    updated_at = now()
		WHERE id = (
			SELECT id
			FROM issue_pr_job
			WHERE status = 'queued' AND run_after <= now()
			ORDER BY created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING id::text, issue_id::text, status, attempts, payload, COALESCE(last_error, ''),
		          run_after, created_at, updated_at, COALESCE(completed_at, 'epoch'::timestamptz)
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}

	var job IssuePRJobRecord
	var payloadJSON []byte
	if err := rows.Scan(
		&job.ID,
		&job.IssueID,
		&job.Status,
		&job.Attempts,
		&payloadJSON,
		&job.LastError,
		&job.RunAfter,
		&job.CreatedAt,
		&job.UpdatedAt,
		&job.CompletedAt,
	); err != nil {
		return nil, err
	}
	if len(payloadJSON) > 0 {
		_ = json.Unmarshal(payloadJSON, &job.Payload)
	}
	return &job, nil
}

func (s *IssuePRStore) MarkCompleted(ctx context.Context, dbtx db.DBTX, jobID string) error {
	_, err := dbtx.Exec(ctx, `
		UPDATE issue_pr_job
		SET status = 'completed',
		    last_error = NULL,
		    completed_at = now(),
		    updated_at = now()
		WHERE id = $1
	`, jobID)
	return err
}

func (s *IssuePRStore) MarkSkipped(ctx context.Context, dbtx db.DBTX, jobID, reason string) error {
	_, err := dbtx.Exec(ctx, `
		UPDATE issue_pr_job
		SET status = 'skipped',
		    last_error = NULLIF($2, ''),
		    completed_at = now(),
		    updated_at = now()
		WHERE id = $1
	`, jobID, reason)
	return err
}

func (s *IssuePRStore) MarkFailed(ctx context.Context, dbtx db.DBTX, jobID, reason string) error {
	_, err := dbtx.Exec(ctx, `
		UPDATE issue_pr_job
		SET status = 'failed',
		    last_error = NULLIF($2, ''),
		    completed_at = now(),
		    updated_at = now()
		WHERE id = $1
	`, jobID, reason)
	return err
}

func (s *IssuePRStore) Requeue(ctx context.Context, dbtx db.DBTX, jobID, reason string, delay time.Duration) error {
	_, err := dbtx.Exec(ctx, `
		UPDATE issue_pr_job
		SET status = 'queued',
		    last_error = NULLIF($2, ''),
		    run_after = $3,
		    locked_at = NULL,
		    updated_at = now()
		WHERE id = $1
	`, jobID, reason, time.Now().Add(delay))
	return err
}

func (s *IssuePRStore) GetLatestCompletedTaskByIssue(ctx context.Context, queries *db.Queries, issueID string) (*db.AgentTaskQueue, error) {
	tasks, err := queries.ListTasksByIssue(ctx, util.ParseUUID(issueID))
	if err != nil {
		return nil, err
	}
	for _, task := range tasks {
		if task.Status == "completed" {
			copyTask := task
			return &copyTask, nil
		}
	}
	return nil, fmt.Errorf("completed task not found")
}

func (s *IssuePRStore) UpdateTaskResult(ctx context.Context, dbtx db.DBTX, taskID string, result []byte) error {
	_, err := dbtx.Exec(ctx, `
		UPDATE agent_task_queue
		SET result = $2
		WHERE id = $1
	`, taskID, result)
	return err
}
