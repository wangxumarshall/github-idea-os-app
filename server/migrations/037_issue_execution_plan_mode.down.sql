ALTER TABLE agent_task_queue
    DROP COLUMN IF EXISTS mode;

ALTER TABLE issue
    DROP COLUMN IF EXISTS execution_stage;
