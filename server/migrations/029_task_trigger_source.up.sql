ALTER TABLE agent_task_queue
ADD COLUMN trigger_source TEXT NOT NULL DEFAULT 'event'
    CHECK (trigger_source IN ('event', 'scheduled'));
