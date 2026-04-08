DROP INDEX IF EXISTS idx_idea_root_issue_id;
DROP INDEX IF EXISTS idx_issue_parent_issue_id;
DROP INDEX IF EXISTS idx_issue_idea_id;
DROP INDEX IF EXISTS idx_issue_repo_url;

ALTER TABLE idea
    DROP COLUMN IF EXISTS root_issue_id;

ALTER TABLE issue
    DROP COLUMN IF EXISTS idea_id,
    DROP COLUMN IF EXISTS repo_url;
