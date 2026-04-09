ALTER TABLE idea
    ALTER COLUMN github_account_id DROP NOT NULL;

WITH workspaces_needing_backfill AS (
    SELECT DISTINCT issue.workspace_id
    FROM issue
    WHERE issue.idea_id IS NULL
),
workspace_defaults AS (
    SELECT
        ws.workspace_id,
        (
            SELECT member.user_id
            FROM member
            WHERE member.workspace_id = ws.workspace_id
            ORDER BY
                CASE member.role
                    WHEN 'owner' THEN 0
                    WHEN 'admin' THEN 1
                    ELSE 2
                END,
                member.created_at ASC
            LIMIT 1
        ) AS owner_user_id,
        (
            SELECT github_account.id
            FROM member
            LEFT JOIN github_account ON github_account.user_id = member.user_id
            WHERE member.workspace_id = ws.workspace_id
            ORDER BY
                CASE member.role
                    WHEN 'owner' THEN 0
                    WHEN 'admin' THEN 1
                    ELSE 2
                END,
                member.created_at ASC
            LIMIT 1
        ) AS github_account_id
    FROM workspaces_needing_backfill ws
),
inserted_ideas AS (
    INSERT INTO idea (
        workspace_id,
        owner_user_id,
        github_account_id,
        seq_no,
        code,
        slug_suffix,
        slug_full,
        title,
        raw_input,
        summary,
        tags,
        idea_path,
        project_repo_name,
        project_repo_url,
        project_repo_status,
        provisioning_error
    )
    SELECT
        workspace_id,
        owner_user_id,
        github_account_id,
        0,
        'idea0000',
        'legacy-backfill',
        'idea0000-legacy-backfill',
        'Legacy Issues',
        'System-generated placeholder idea for issues created before idea binding was required.',
        'System-generated placeholder idea for legacy issues.',
        '["legacy","system"]'::jsonb,
        'ideas/idea0000-legacy-backfill/idea0000-legacy-backfill.md',
        'legacy-backfill-' || replace(workspace_id::text, '-', ''),
        '',
        'ready',
        NULL
    FROM workspace_defaults
    WHERE owner_user_id IS NOT NULL
      AND NOT EXISTS (
          SELECT 1
          FROM idea
          WHERE idea.workspace_id = workspace_defaults.workspace_id
            AND idea.code = 'idea0000'
            AND idea.slug_suffix = 'legacy-backfill'
      )
    RETURNING id, workspace_id
)
UPDATE issue
SET idea_id = idea.id
FROM idea
WHERE issue.workspace_id = idea.workspace_id
  AND issue.idea_id IS NULL
  AND idea.code = 'idea0000'
  AND idea.slug_suffix = 'legacy-backfill';
