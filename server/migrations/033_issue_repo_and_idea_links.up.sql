ALTER TABLE issue
    ADD COLUMN repo_url TEXT,
    ADD COLUMN idea_id UUID REFERENCES idea(id) ON DELETE SET NULL;

ALTER TABLE idea
    ADD COLUMN root_issue_id UUID REFERENCES issue(id) ON DELETE SET NULL;

CREATE INDEX idx_issue_repo_url ON issue(repo_url);
CREATE INDEX idx_issue_idea_id ON issue(idea_id);
CREATE INDEX idx_issue_parent_issue_id ON issue(parent_issue_id);
CREATE UNIQUE INDEX idx_idea_root_issue_id ON idea(root_issue_id) WHERE root_issue_id IS NOT NULL;
