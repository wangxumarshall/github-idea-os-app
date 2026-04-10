ALTER TABLE issue
    ADD COLUMN execution_stage TEXT NOT NULL DEFAULT 'idle'
        CHECK (execution_stage IN ('idle', 'planning', 'plan_ready', 'build_ready', 'building'));

ALTER TABLE agent_task_queue
    ADD COLUMN mode TEXT NOT NULL DEFAULT 'build'
        CHECK (mode IN ('plan', 'build'));
