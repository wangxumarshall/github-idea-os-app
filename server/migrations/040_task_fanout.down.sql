DROP INDEX IF EXISTS idx_agent_task_queue_parent_task;

ALTER TABLE agent_task_queue
    DROP COLUMN IF EXISTS swarm_role,
    DROP COLUMN IF EXISTS parent_task_id;
