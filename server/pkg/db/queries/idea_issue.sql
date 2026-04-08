-- name: ListIssuesByIdeaID :many
SELECT * FROM issue
WHERE idea_id = $1
ORDER BY
  CASE WHEN parent_issue_id IS NULL THEN 0 ELSE 1 END,
  position ASC,
  created_at DESC;
