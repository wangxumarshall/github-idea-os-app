CREATE TABLE github_account (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    github_user_id BIGINT NOT NULL,
    login TEXT NOT NULL,
    avatar_url TEXT,
    profile_url TEXT,
    access_token_encrypted TEXT NOT NULL,
    token_type TEXT NOT NULL DEFAULT 'bearer',
    scope TEXT NOT NULL DEFAULT '',
    next_idea_seq INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id),
    UNIQUE(github_user_id)
);

CREATE TABLE idea (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    owner_user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    github_account_id UUID NOT NULL REFERENCES github_account(id) ON DELETE CASCADE,
    seq_no INTEGER NOT NULL,
    code TEXT NOT NULL,
    slug_suffix TEXT NOT NULL,
    slug_full TEXT NOT NULL,
    title TEXT NOT NULL,
    raw_input TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    tags JSONB NOT NULL DEFAULT '[]',
    idea_path TEXT NOT NULL,
    markdown_sha TEXT,
    project_repo_name TEXT NOT NULL,
    project_repo_url TEXT NOT NULL,
    project_repo_status TEXT NOT NULL DEFAULT 'creating'
        CHECK (project_repo_status IN ('creating', 'ready', 'failed')),
    provisioning_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(github_account_id, seq_no),
    UNIQUE(workspace_id, slug_full),
    UNIQUE(project_repo_name)
);

CREATE TABLE idea_job (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idea_id UUID NOT NULL REFERENCES idea(id) ON DELETE CASCADE,
    job_type TEXT NOT NULL CHECK (job_type IN ('create_project_repo')),
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'completed', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0,
    payload JSONB NOT NULL DEFAULT '{}',
    last_error TEXT,
    run_after TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_github_account_user_id ON github_account(user_id);
CREATE INDEX idx_idea_workspace_id ON idea(workspace_id, updated_at DESC);
CREATE INDEX idx_idea_github_account_seq ON idea(github_account_id, seq_no);
CREATE INDEX idx_idea_job_queue ON idea_job(status, run_after, created_at);
