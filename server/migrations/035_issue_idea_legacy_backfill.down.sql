UPDATE issue
SET idea_id = NULL
WHERE idea_id IN (
    SELECT id
    FROM idea
    WHERE code = 'idea0000'
      AND slug_suffix = 'legacy-backfill'
);

DELETE FROM idea
WHERE code = 'idea0000'
  AND slug_suffix = 'legacy-backfill';

ALTER TABLE idea
    ALTER COLUMN github_account_id SET NOT NULL;
