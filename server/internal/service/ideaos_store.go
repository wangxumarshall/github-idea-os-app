package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type IdeaRecord struct {
	ID                   string
	WorkspaceID          string
	OwnerUserID          string
	GitHubAccountID      string
	SeqNo                int
	Code                 string
	SlugSuffix           string
	SlugFull             string
	Title                string
	RawInput             string
	Summary              string
	Tags                 []string
	IdeaPath             string
	MarkdownSHA          string
	RootIssueID          string
	ProjectSpecSHA       string
	ProjectSpecSyncError string
	ProjectRepoName      string
	ProjectRepoURL       string
	ProjectRepoStatus    string
	ProvisioningError    string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type IdeaJobRecord struct {
	ID        string
	IdeaID    string
	JobType   string
	Status    string
	Attempts  int
	Payload   map[string]any
	LastError string
	RunAfter  time.Time
}

type IdeaNameSuggestion struct {
	Name       string `json:"name"`
	SlugSuffix string `json:"slug_suffix"`
	FullName   string `json:"full_name"`
}

type IdeaStore struct{}

func NewIdeaStore() *IdeaStore {
	return &IdeaStore{}
}

func (s *IdeaStore) NextIdeaSequence(ctx context.Context, dbtx db.DBTX, githubAccountID string) (int, string, error) {
	var seq int
	var login string
	err := dbtx.QueryRow(ctx, `
		UPDATE github_account
		SET next_idea_seq = next_idea_seq + 1,
		    updated_at = now()
		WHERE id = $1
		RETURNING next_idea_seq, login
	`, githubAccountID).Scan(&seq, &login)
	if err != nil {
		return 0, "", err
	}
	return seq, login, nil
}

func (s *IdeaStore) PeekNextIdeaSequence(ctx context.Context, dbtx db.DBTX, githubAccountID string) (int, error) {
	var next int
	err := dbtx.QueryRow(ctx, `
		SELECT next_idea_seq + 1
		FROM github_account
		WHERE id = $1
	`, githubAccountID).Scan(&next)
	if err != nil {
		return 0, err
	}
	return next, nil
}

func (s *IdeaStore) InsertIdea(ctx context.Context, dbtx db.DBTX, record IdeaRecord) (IdeaRecord, error) {
	tagsJSON, err := json.Marshal(record.Tags)
	if err != nil {
		return IdeaRecord{}, err
	}

	var created IdeaRecord
	err = dbtx.QueryRow(ctx, `
		INSERT INTO idea (
			workspace_id, owner_user_id, github_account_id, seq_no, code, slug_suffix, slug_full,
			title, raw_input, summary, tags, idea_path, markdown_sha, root_issue_id, project_spec_sha, project_spec_sync_error,
			project_repo_name, project_repo_url, project_repo_status, provisioning_error
		)
		VALUES ($1, $2, NULLIF($3, '')::uuid, $4, $5, $6, $7, $8, $9, $10, $11::jsonb, $12, NULLIF($13, ''), NULLIF($14, '')::uuid, NULLIF($15, ''), NULLIF($16, ''), $17, $18, $19, NULLIF($20, ''))
		RETURNING id::text, workspace_id::text, owner_user_id::text, COALESCE(github_account_id::text, ''), seq_no, code, slug_suffix, slug_full,
		          title, raw_input, summary, tags, idea_path, COALESCE(markdown_sha, ''), COALESCE(root_issue_id::text, ''), COALESCE(project_spec_sha, ''),
		          COALESCE(project_spec_sync_error, ''), project_repo_name, project_repo_url, project_repo_status, COALESCE(provisioning_error, ''), created_at, updated_at
	`, record.WorkspaceID, record.OwnerUserID, record.GitHubAccountID, record.SeqNo, record.Code, record.SlugSuffix, record.SlugFull,
		record.Title, record.RawInput, record.Summary, string(tagsJSON), record.IdeaPath, record.MarkdownSHA, record.RootIssueID, record.ProjectSpecSHA, record.ProjectSpecSyncError,
		record.ProjectRepoName, record.ProjectRepoURL, record.ProjectRepoStatus, record.ProvisioningError).Scan(
		&created.ID,
		&created.WorkspaceID,
		&created.OwnerUserID,
		&created.GitHubAccountID,
		&created.SeqNo,
		&created.Code,
		&created.SlugSuffix,
		&created.SlugFull,
		&created.Title,
		&created.RawInput,
		&created.Summary,
		&tagsJSON,
		&created.IdeaPath,
		&created.MarkdownSHA,
		&created.RootIssueID,
		&created.ProjectSpecSHA,
		&created.ProjectSpecSyncError,
		&created.ProjectRepoName,
		&created.ProjectRepoURL,
		&created.ProjectRepoStatus,
		&created.ProvisioningError,
		&created.CreatedAt,
		&created.UpdatedAt,
	)
	if err != nil {
		return IdeaRecord{}, err
	}
	_ = json.Unmarshal(tagsJSON, &created.Tags)
	return created, nil
}

func (s *IdeaStore) UpdateIdeaProjectSpecState(ctx context.Context, dbtx db.DBTX, ideaID, projectSpecSHA, syncError string) error {
	_, err := dbtx.Exec(ctx, `
		UPDATE idea
		SET project_spec_sha = NULLIF($2, ''),
		    project_spec_sync_error = NULLIF($3, ''),
		    updated_at = now()
		WHERE id = $1
	`, ideaID, projectSpecSHA, syncError)
	return err
}

func (s *IdeaStore) UpdateIdeaContent(ctx context.Context, dbtx db.DBTX, ideaID, title, summary, markdownSHA string, tags []string, provisioningStatus, provisioningError string) error {
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return err
	}
	_, err = dbtx.Exec(ctx, `
		UPDATE idea
		SET title = $2,
		    summary = $3,
		    tags = $4::jsonb,
		    markdown_sha = NULLIF($5, ''),
		    project_repo_status = COALESCE(NULLIF($6, ''), project_repo_status),
		    provisioning_error = NULLIF($7, ''),
		    updated_at = now()
		WHERE id = $1
	`, ideaID, title, summary, string(tagsJSON), markdownSHA, nullableOrEmpty(provisioningStatus), provisioningError)
	return err
}

func (s *IdeaStore) UpdateIdeaRootIssue(ctx context.Context, dbtx db.DBTX, ideaID, rootIssueID string) error {
	_, err := dbtx.Exec(ctx, `
		UPDATE idea
		SET root_issue_id = NULLIF($2, '')::uuid,
		    updated_at = now()
		WHERE id = $1
	`, ideaID, rootIssueID)
	return err
}

func (s *IdeaStore) UpdateIdeaRepoState(ctx context.Context, dbtx db.DBTX, ideaID, repoURL, status, provisioningError string) error {
	_, err := dbtx.Exec(ctx, `
		UPDATE idea
		SET project_repo_url = $2,
		    project_repo_status = $3,
		    provisioning_error = NULLIF($4, ''),
		    updated_at = now()
		WHERE id = $1
	`, ideaID, repoURL, status, provisioningError)
	return err
}

func (s *IdeaStore) GetIdeaBySlug(ctx context.Context, dbtx db.DBTX, workspaceID, slugFull string) (*IdeaRecord, error) {
	rows, err := dbtx.Query(ctx, `
		SELECT id::text, workspace_id::text, owner_user_id::text, COALESCE(github_account_id::text, ''), seq_no, code, slug_suffix, slug_full,
		       title, raw_input, summary, tags, idea_path, COALESCE(markdown_sha, ''), COALESCE(root_issue_id::text, ''), COALESCE(project_spec_sha, ''),
		       COALESCE(project_spec_sync_error, ''), project_repo_name, project_repo_url, project_repo_status, COALESCE(provisioning_error, ''), created_at, updated_at
		FROM idea
		WHERE workspace_id = $1 AND slug_full = $2
		LIMIT 1
	`, workspaceID, slugFull)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, fmt.Errorf("idea not found")
	}

	record, err := scanIdea(rows)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *IdeaStore) ListIdeasByWorkspace(ctx context.Context, dbtx db.DBTX, workspaceID string) ([]IdeaRecord, error) {
	rows, err := dbtx.Query(ctx, `
		SELECT id::text, workspace_id::text, owner_user_id::text, COALESCE(github_account_id::text, ''), seq_no, code, slug_suffix, slug_full,
		       title, raw_input, summary, tags, idea_path, COALESCE(markdown_sha, ''), COALESCE(root_issue_id::text, ''), COALESCE(project_spec_sha, ''),
		       COALESCE(project_spec_sync_error, ''), project_repo_name, project_repo_url, project_repo_status, COALESCE(provisioning_error, ''), created_at, updated_at
		FROM idea
		WHERE workspace_id = $1
		ORDER BY updated_at DESC, seq_no DESC
	`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ideas []IdeaRecord
	for rows.Next() {
		record, err := scanIdea(rows)
		if err != nil {
			return nil, err
		}
		ideas = append(ideas, record)
	}
	return ideas, rows.Err()
}

func (s *IdeaStore) GetIdeaByID(ctx context.Context, dbtx db.DBTX, ideaID string) (*IdeaRecord, error) {
	rows, err := dbtx.Query(ctx, `
		SELECT id::text, workspace_id::text, owner_user_id::text, COALESCE(github_account_id::text, ''), seq_no, code, slug_suffix, slug_full,
		       title, raw_input, summary, tags, idea_path, COALESCE(markdown_sha, ''), COALESCE(root_issue_id::text, ''), COALESCE(project_spec_sha, ''),
		       COALESCE(project_spec_sync_error, ''), project_repo_name, project_repo_url, project_repo_status, COALESCE(provisioning_error, ''), created_at, updated_at
		FROM idea
		WHERE id = $1
		LIMIT 1
	`, ideaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, fmt.Errorf("idea not found")
	}
	record, err := scanIdea(rows)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *IdeaStore) DeleteIdea(ctx context.Context, dbtx db.DBTX, ideaID string) error {
	_, err := dbtx.Exec(ctx, `
		DELETE FROM idea
		WHERE id = $1
	`, ideaID)
	return err
}

func (s *IdeaStore) EnqueueRepoProvisionJob(ctx context.Context, dbtx db.DBTX, ideaID string) error {
	_, err := dbtx.Exec(ctx, `
		INSERT INTO idea_job (idea_id, job_type, status, payload)
		VALUES ($1, 'create_project_repo', 'queued', '{}'::jsonb)
	`, ideaID)
	return err
}

func (s *IdeaStore) RetryRepoProvisionJob(ctx context.Context, dbtx db.DBTX, ideaID string) error {
	_, err := dbtx.Exec(ctx, `
		UPDATE idea
		SET project_repo_status = 'creating',
		    provisioning_error = NULL,
		    updated_at = now()
		WHERE id = $1
	`, ideaID)
	if err != nil {
		return err
	}
	_, err = dbtx.Exec(ctx, `
		INSERT INTO idea_job (idea_id, job_type, status, payload)
		VALUES ($1, 'create_project_repo', 'queued', '{}'::jsonb)
	`, ideaID)
	return err
}

func (s *IdeaStore) GetLegacyIdeaByWorkspace(ctx context.Context, dbtx db.DBTX, workspaceID string) (*IdeaRecord, error) {
	rows, err := dbtx.Query(ctx, `
		SELECT id::text, workspace_id::text, owner_user_id::text, COALESCE(github_account_id::text, ''), seq_no, code, slug_suffix, slug_full,
		       title, raw_input, summary, tags, idea_path, COALESCE(markdown_sha, ''), COALESCE(root_issue_id::text, ''), COALESCE(project_spec_sha, ''),
		       COALESCE(project_spec_sync_error, ''), project_repo_name, project_repo_url, project_repo_status, COALESCE(provisioning_error, ''), created_at, updated_at
		FROM idea
		WHERE workspace_id = $1 AND code = $2 AND slug_suffix = $3
		LIMIT 1
	`, workspaceID, LegacyIdeaCode, LegacyIdeaSlugSuffix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, fmt.Errorf("legacy idea not found")
	}

	record, err := scanIdea(rows)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *IdeaStore) EnsureLegacyIdea(ctx context.Context, dbtx db.DBTX, workspaceID string) (*IdeaRecord, error) {
	if existing, err := s.GetLegacyIdeaByWorkspace(ctx, dbtx, workspaceID); err == nil {
		return existing, nil
	}

	var ownerUserID string
	var githubAccountID string
	err := dbtx.QueryRow(ctx, `
		SELECT m.user_id::text, COALESCE(ga.id::text, '')
		FROM member m
		LEFT JOIN github_account ga ON ga.user_id = m.user_id
		WHERE m.workspace_id = $1
		ORDER BY CASE m.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END, m.created_at ASC
		LIMIT 1
	`, workspaceID).Scan(&ownerUserID, &githubAccountID)
	if err != nil {
		return nil, err
	}

	record, err := s.InsertIdea(ctx, dbtx, IdeaRecord{
		WorkspaceID:       workspaceID,
		OwnerUserID:       ownerUserID,
		GitHubAccountID:   githubAccountID,
		SeqNo:             0,
		Code:              LegacyIdeaCode,
		SlugSuffix:        LegacyIdeaSlugSuffix,
		SlugFull:          LegacyIdeaCode + "-" + LegacyIdeaSlugSuffix,
		Title:             "Legacy Issues",
		RawInput:          "System-generated placeholder idea for issues created before idea binding was required.",
		Summary:           "System-generated placeholder idea for legacy issues.",
		Tags:              []string{"legacy", "system"},
		IdeaPath:          path.Join("ideas", LegacyIdeaCode+"-"+LegacyIdeaSlugSuffix, LegacyIdeaCode+"-"+LegacyIdeaSlugSuffix+".md"),
		ProjectRepoName:   "legacy-backfill-" + strings.ReplaceAll(workspaceID, "-", ""),
		ProjectRepoURL:    "",
		ProjectRepoStatus: "ready",
	})
	if err == nil {
		return &record, nil
	}

	return s.GetLegacyIdeaByWorkspace(ctx, dbtx, workspaceID)
}

func (s *IdeaStore) ClaimNextJob(ctx context.Context, dbtx db.DBTX) (*IdeaJobRecord, error) {
	rows, err := dbtx.Query(ctx, `
		UPDATE idea_job
		SET status = 'running',
		    attempts = attempts + 1,
		    locked_at = now(),
		    updated_at = now()
		WHERE id = (
			SELECT id
			FROM idea_job
			WHERE status = 'queued' AND run_after <= now()
			ORDER BY created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING id::text, idea_id::text, job_type, status, attempts, payload, COALESCE(last_error, ''), run_after
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	var job IdeaJobRecord
	var payloadJSON []byte
	if err := rows.Scan(&job.ID, &job.IdeaID, &job.JobType, &job.Status, &job.Attempts, &payloadJSON, &job.LastError, &job.RunAfter); err != nil {
		return nil, err
	}
	_ = json.Unmarshal(payloadJSON, &job.Payload)
	return &job, nil
}

func (s *IdeaStore) MarkJobCompleted(ctx context.Context, dbtx db.DBTX, jobID string) error {
	_, err := dbtx.Exec(ctx, `
		UPDATE idea_job
		SET status = 'completed',
		    completed_at = now(),
		    updated_at = now()
		WHERE id = $1
	`, jobID)
	return err
}

func (s *IdeaStore) MarkJobFailed(ctx context.Context, dbtx db.DBTX, jobID, reason string) error {
	_, err := dbtx.Exec(ctx, `
		UPDATE idea_job
		SET status = 'failed',
		    last_error = $2,
		    updated_at = now()
		WHERE id = $1
	`, jobID, reason)
	return err
}

func scanIdea(rows interface{ Scan(dest ...any) error }) (IdeaRecord, error) {
	var record IdeaRecord
	var tagsJSON []byte
	if err := rows.Scan(
		&record.ID,
		&record.WorkspaceID,
		&record.OwnerUserID,
		&record.GitHubAccountID,
		&record.SeqNo,
		&record.Code,
		&record.SlugSuffix,
		&record.SlugFull,
		&record.Title,
		&record.RawInput,
		&record.Summary,
		&tagsJSON,
		&record.IdeaPath,
		&record.MarkdownSHA,
		&record.RootIssueID,
		&record.ProjectSpecSHA,
		&record.ProjectSpecSyncError,
		&record.ProjectRepoName,
		&record.ProjectRepoURL,
		&record.ProjectRepoStatus,
		&record.ProvisioningError,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return IdeaRecord{}, err
	}
	_ = json.Unmarshal(tagsJSON, &record.Tags)
	return record, nil
}

func nullableOrEmpty(value string) string {
	return strings.TrimSpace(value)
}

func RecommendIdeaNames(rawInput string, nextCode int) []IdeaNameSuggestion {
	code := fmt.Sprintf("idea%04d", nextCode)
	text := strings.TrimSpace(rawInput)
	if text == "" {
		return nil
	}

	parts := strings.Fields(strings.ToLower(text))
	stopwords := map[string]struct{}{
		"the": {}, "a": {}, "an": {}, "to": {}, "for": {}, "of": {}, "and": {}, "or": {},
		"with": {}, "build": {}, "make": {}, "create": {}, "doing": {}, "idea": {}, "app": {},
	}

	keywords := make([]string, 0, 6)
	seen := map[string]struct{}{}
	for _, part := range parts {
		part = NormalizeIdeaSlug(part)
		if part == "" {
			continue
		}
		if _, ok := stopwords[part]; ok {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		keywords = append(keywords, part)
		if len(keywords) == 6 {
			break
		}
	}

	makeName := func(items ...string) IdeaNameSuggestion {
		suffix := NormalizeIdeaSlug(strings.Join(items, "-"))
		if suffix == "" {
			suffix = "new-idea"
		}
		name := prettifyIdeaSlug(suffix)
		return IdeaNameSuggestion{
			Name:       name,
			SlugSuffix: suffix,
			FullName:   code + "-" + suffix,
		}
	}

	var candidates []IdeaNameSuggestion
	switch {
	case len(keywords) >= 3:
		candidates = []IdeaNameSuggestion{
			makeName(keywords[0], keywords[1], keywords[2]),
			makeName(keywords[0], keywords[1], "system"),
			makeName(keywords[0], keywords[1], "studio"),
		}
	case len(keywords) == 2:
		candidates = []IdeaNameSuggestion{
			makeName(keywords[0], keywords[1]),
			makeName(keywords[0], keywords[1], "studio"),
			makeName(keywords[0], keywords[1], "engine"),
		}
	case len(keywords) == 1:
		candidates = []IdeaNameSuggestion{
			makeName(keywords[0], "studio"),
			makeName(keywords[0], "engine"),
			makeName(keywords[0], "workspace"),
		}
	default:
		candidates = []IdeaNameSuggestion{
			makeName("new", "idea"),
			makeName("product", "concept"),
			makeName("technical", "idea"),
		}
	}

	unique := make([]IdeaNameSuggestion, 0, len(candidates))
	seenNames := map[string]struct{}{}
	for _, candidate := range candidates {
		if _, ok := seenNames[candidate.SlugSuffix]; ok {
			continue
		}
		seenNames[candidate.SlugSuffix] = struct{}{}
		unique = append(unique, candidate)
	}
	return unique
}
