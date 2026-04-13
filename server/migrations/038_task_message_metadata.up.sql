ALTER TABLE task_message
    ADD COLUMN metadata JSONB NOT NULL DEFAULT '{}'::jsonb;
