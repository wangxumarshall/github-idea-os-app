ALTER TABLE idea
    DROP COLUMN IF EXISTS project_spec_sync_error,
    DROP COLUMN IF EXISTS project_spec_sha;
