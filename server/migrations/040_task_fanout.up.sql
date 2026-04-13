ALTER TABLE agent_task_queue
    ADD COLUMN parent_task_id UUID REFERENCES agent_task_queue(id) ON DELETE SET NULL,
    ADD COLUMN swarm_role TEXT NOT NULL DEFAULT '';

CREATE INDEX idx_agent_task_queue_parent_task
    ON agent_task_queue(parent_task_id, created_at ASC);
