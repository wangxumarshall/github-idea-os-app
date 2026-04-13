-- name: CreateRunMemory :one
INSERT INTO run_memory (workspace_id, issue_id, task_id, kind, title, content, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListRecentRunMemoryByIssue :many
SELECT * FROM run_memory
WHERE issue_id = $1
ORDER BY created_at DESC
LIMIT $2;
