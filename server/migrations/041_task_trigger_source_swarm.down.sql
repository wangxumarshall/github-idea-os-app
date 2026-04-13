ALTER TABLE agent_task_queue
    DROP CONSTRAINT IF EXISTS agent_task_queue_trigger_source_check;

ALTER TABLE agent_task_queue
    ADD CONSTRAINT agent_task_queue_trigger_source_check
    CHECK (trigger_source IN ('event', 'scheduled'));
