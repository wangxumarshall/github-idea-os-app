package main

import (
	"context"
	"strings"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func isIdeaRootIssueReadyForAutomation(ctx context.Context, pool db.DBTX, store *service.IdeaStore, issue db.Issue) (bool, error) {
	if !issue.IdeaID.Valid {
		return true, nil
	}

	idea, err := store.GetIdeaByID(ctx, pool, util.UUIDToString(issue.IdeaID))
	if err != nil {
		return false, err
	}

	if strings.TrimSpace(idea.RootIssueID) != util.UUIDToString(issue.ID) {
		return true, nil
	}

	if strings.TrimSpace(idea.ProjectRepoStatus) != "ready" {
		return false, nil
	}
	if strings.TrimSpace(idea.ProjectSpecSyncError) != "" {
		return false, nil
	}

	return true, nil
}
