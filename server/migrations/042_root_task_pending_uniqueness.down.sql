DROP INDEX IF EXISTS idx_one_pending_root_task_per_issue;

CREATE UNIQUE INDEX idx_one_pending_task_per_issue
    ON agent_task_queue (issue_id)
    WHERE status IN ('queued', 'dispatched');
