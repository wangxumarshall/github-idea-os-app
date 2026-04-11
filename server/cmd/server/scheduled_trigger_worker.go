package main

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const scheduledTriggerSweepInterval = 30 * time.Second

func runScheduledTriggerWorker(ctx context.Context, pool db.DBTX, queries *db.Queries, taskService *service.TaskService) {
	store := service.NewIdeaStore()
	ticker := time.NewTicker(scheduledTriggerSweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sweepScheduledTriggers(ctx, pool, queries, taskService, store, time.Now().UTC())
		}
	}
}

func sweepScheduledTriggers(ctx context.Context, pool db.DBTX, queries *db.Queries, taskService *service.TaskService, store *service.IdeaStore, now time.Time) {
	agents, err := queries.ListRunnableAgents(ctx)
	if err != nil {
		slog.Warn("scheduled trigger worker: list runnable agents failed", "error", err)
		return
	}

	for _, agent := range agents {
		scheduledTriggers := service.EnabledScheduledTriggers(agent.Triggers)
		if len(scheduledTriggers) == 0 {
			continue
		}

		issues, err := queries.ListIssuesAssignedToAgent(ctx, agent.ID)
		if err != nil {
			slog.Warn("scheduled trigger worker: list assigned issues failed", "agent_id", util.UUIDToString(agent.ID), "error", err)
			continue
		}

		for _, issue := range issues {
			eligible, err := isScheduledIssueEligible(ctx, pool, store, issue)
			if err != nil {
				slog.Warn("scheduled trigger worker: eligibility check failed", "issue_id", util.UUIDToString(issue.ID), "error", err)
				continue
			}
			if !eligible {
				continue
			}

			active, err := queries.HasActiveTaskForIssue(ctx, issue.ID)
			if err != nil {
				slog.Warn("scheduled trigger worker: active task check failed", "issue_id", util.UUIDToString(issue.ID), "error", err)
				continue
			}
			if active {
				continue
			}

			for _, trigger := range scheduledTriggers {
				dueAt, due, err := scheduledTriggerDueAt(now, trigger.Config)
				if err != nil {
					slog.Warn("scheduled trigger worker: invalid schedule config", "agent_id", util.UUIDToString(agent.ID), "issue_id", util.UUIDToString(issue.ID), "error", err)
					continue
				}
				if !due {
					continue
				}

				if alreadyQueuedForSchedule(ctx, queries, issue.ID, agent.ID, dueAt) {
					continue
				}

				if _, err := taskService.EnqueueTaskForIssueInModeWithSource(ctx, issue, service.PreferredTaskMode(issue), service.TaskTriggerSourceScheduled); err != nil {
					slog.Warn("scheduled trigger worker: enqueue failed", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agent.ID), "error", err)
				}
				break
			}
		}
	}
}

func isScheduledIssueEligible(ctx context.Context, pool db.DBTX, store *service.IdeaStore, issue db.Issue) (bool, error) {
	if issue.Status == "done" || issue.Status == "cancelled" {
		return false, nil
	}
	if !issue.AssigneeType.Valid || issue.AssigneeType.String != "agent" || !issue.AssigneeID.Valid {
		return false, nil
	}
	return isIdeaRootIssueReadyForAutomation(ctx, pool, store, issue)
}

func alreadyQueuedForSchedule(ctx context.Context, queries *db.Queries, issueID, agentID pgtype.UUID, dueAt time.Time) bool {
	createdAt, err := queries.GetLatestTaskCreatedAtByIssueAgentSource(ctx, db.GetLatestTaskCreatedAtByIssueAgentSourceParams{
		IssueID:       issueID,
		AgentID:       agentID,
		TriggerSource: service.TaskTriggerSourceScheduled,
	})
	if err != nil {
		return !errors.Is(err, pgx.ErrNoRows)
	}
	return !createdAt.Time.Before(dueAt)
}

func scheduledTriggerDueAt(now time.Time, config map[string]any) (time.Time, bool, error) {
	cronExpr := service.DefaultScheduledCron
	if raw, ok := config["cron"].(string); ok && strings.TrimSpace(raw) != "" {
		cronExpr = strings.TrimSpace(raw)
	}

	location := time.UTC
	if raw, ok := config["timezone"].(string); ok && strings.TrimSpace(raw) != "" {
		loc, err := time.LoadLocation(strings.TrimSpace(raw))
		if err != nil {
			return time.Time{}, false, err
		}
		location = loc
	}

	localNow := now.In(location).Truncate(time.Minute)
	match, err := cronMatchesTime(localNow, cronExpr)
	if err != nil {
		return time.Time{}, false, err
	}
	if !match {
		return time.Time{}, false, nil
	}

	return time.Date(
		localNow.Year(),
		localNow.Month(),
		localNow.Day(),
		localNow.Hour(),
		localNow.Minute(),
		0,
		0,
		location,
	).UTC(), true, nil
}

func cronMatchesTime(ts time.Time, expr string) (bool, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return false, errors.New("cron expression must have 5 fields")
	}

	minuteField, hourField, domField, monthField, dowField := fields[0], fields[1], fields[2], fields[3], fields[4]

	minuteMatch, err := cronFieldMatches(minuteField, ts.Minute(), 0, 59, false)
	if err != nil || !minuteMatch {
		return false, err
	}
	hourMatch, err := cronFieldMatches(hourField, ts.Hour(), 0, 23, false)
	if err != nil || !hourMatch {
		return false, err
	}
	monthMatch, err := cronFieldMatches(monthField, int(ts.Month()), 1, 12, false)
	if err != nil || !monthMatch {
		return false, err
	}

	domMatch, domWildcard, err := cronFieldMatchesWithWildcard(domField, ts.Day(), 1, 31, false)
	if err != nil {
		return false, err
	}
	dowMatch, dowWildcard, err := cronFieldMatchesWithWildcard(dowField, int(ts.Weekday()), 0, 7, true)
	if err != nil {
		return false, err
	}

	dayMatches := false
	switch {
	case domWildcard && dowWildcard:
		dayMatches = true
	case domWildcard:
		dayMatches = dowMatch
	case dowWildcard:
		dayMatches = domMatch
	default:
		dayMatches = domMatch || dowMatch
	}

	return dayMatches, nil
}

func cronFieldMatches(field string, value, min, max int, dayOfWeek bool) (bool, error) {
	match, _, err := cronFieldMatchesWithWildcard(field, value, min, max, dayOfWeek)
	return match, err
}

func cronFieldMatchesWithWildcard(field string, value, min, max int, dayOfWeek bool) (bool, bool, error) {
	field = strings.TrimSpace(field)
	if field == "*" {
		return true, true, nil
	}

	for _, part := range strings.Split(field, ",") {
		ok, err := cronPartMatches(strings.TrimSpace(part), value, min, max, dayOfWeek)
		if err != nil {
			return false, false, err
		}
		if ok {
			return true, false, nil
		}
	}

	return false, false, nil
}

func cronPartMatches(part string, value, min, max int, dayOfWeek bool) (bool, error) {
	step := 1
	base := part
	if strings.Contains(part, "/") {
		chunks := strings.Split(part, "/")
		if len(chunks) != 2 {
			return false, errors.New("invalid cron step syntax")
		}
		base = chunks[0]
		parsedStep, err := strconv.Atoi(chunks[1])
		if err != nil || parsedStep <= 0 {
			return false, errors.New("invalid cron step value")
		}
		step = parsedStep
	}

	if base == "*" {
		return ((value - min) % step) == 0, nil
	}

	start, end, err := cronPartRange(base, min, max, dayOfWeek)
	if err != nil {
		return false, err
	}
	if value < start || value > end {
		return false, nil
	}
	return ((value - start) % step) == 0, nil
}

func cronPartRange(base string, min, max int, dayOfWeek bool) (int, int, error) {
	if strings.Contains(base, "-") {
		chunks := strings.Split(base, "-")
		if len(chunks) != 2 {
			return 0, 0, errors.New("invalid cron range syntax")
		}
		start, err := cronFieldNumber(chunks[0], min, max, dayOfWeek)
		if err != nil {
			return 0, 0, err
		}
		end, err := cronFieldNumber(chunks[1], min, max, dayOfWeek)
		if err != nil {
			return 0, 0, err
		}
		if end < start {
			return 0, 0, errors.New("cron range end before start")
		}
		return start, end, nil
	}

	number, err := cronFieldNumber(base, min, max, dayOfWeek)
	if err != nil {
		return 0, 0, err
	}
	return number, number, nil
}

func cronFieldNumber(raw string, min, max int, dayOfWeek bool) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, errors.New("invalid cron value")
	}
	if dayOfWeek && value == 7 {
		value = 0
	}
	if value < min || value > max {
		return 0, errors.New("cron value out of range")
	}
	return value, nil
}
