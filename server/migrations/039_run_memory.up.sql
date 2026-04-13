CREATE TABLE run_memory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    issue_id UUID NOT NULL REFERENCES issue(id) ON DELETE CASCADE,
    task_id UUID REFERENCES agent_task_queue(id) ON DELETE SET NULL,
    kind TEXT NOT NULL CHECK (kind IN ('run_summary', 'failure_pattern')),
    title TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_run_memory_issue_created_at
    ON run_memory(issue_id, created_at DESC);

CREATE INDEX idx_run_memory_task_kind
    ON run_memory(task_id, kind);
