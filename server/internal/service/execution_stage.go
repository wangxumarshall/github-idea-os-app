package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	ExecutionStageIdle       = "idle"
	ExecutionStagePlanning   = "planning"
	ExecutionStagePlanReady  = "plan_ready"
	ExecutionStageBuildReady = "build_ready"
	ExecutionStageBuilding   = "building"

	TaskModePlan  = "plan"
	TaskModeBuild = "build"
)

func NormalizeExecutionStage(stage string) string {
	switch strings.TrimSpace(stage) {
	case ExecutionStagePlanning:
		return ExecutionStagePlanning
	case ExecutionStagePlanReady:
		return ExecutionStagePlanReady
	case ExecutionStageBuildReady:
		return ExecutionStageBuildReady
	case ExecutionStageBuilding:
		return ExecutionStageBuilding
	default:
		return ExecutionStageIdle
	}
}

func NormalizeTaskMode(mode string) string {
	if strings.TrimSpace(mode) == TaskModePlan {
		return TaskModePlan
	}
	return TaskModeBuild
}

func PreferredTaskMode(issue db.Issue) string {
	switch NormalizeExecutionStage(issue.ExecutionStage) {
	case ExecutionStageBuildReady, ExecutionStageBuilding:
		return TaskModeBuild
	default:
		return TaskModePlan
	}
}

func QueueStageForMode(mode string) string {
	switch NormalizeTaskMode(mode) {
	case TaskModePlan:
		return ExecutionStagePlanning
	default:
		return ExecutionStageBuildReady
	}
}

func RunningStageForMode(mode string) string {
	switch NormalizeTaskMode(mode) {
	case TaskModePlan:
		return ExecutionStagePlanning
	default:
		return ExecutionStageBuilding
	}
}

func CompletedStageForMode(mode string) string {
	switch NormalizeTaskMode(mode) {
	case TaskModePlan:
		return ExecutionStagePlanReady
	default:
		return ExecutionStageBuildReady
	}
}

func FailedStageForMode(mode string) string {
	switch NormalizeTaskMode(mode) {
	case TaskModePlan:
		return ExecutionStageIdle
	default:
		return ExecutionStageBuildReady
	}
}

type UnsupportedExecutionModeError struct {
	Provider string
	Mode     string
}

func (e *UnsupportedExecutionModeError) Error() string {
	provider := strings.TrimSpace(strings.ToLower(e.Provider))
	mode := NormalizeTaskMode(e.Mode)
	switch provider {
	case "codex":
		return fmt.Sprintf("Codex native %s mode is paused while the daemon still uses the app-server adapter", mode)
	default:
		return fmt.Sprintf("%s native %s mode is not supported", provider, mode)
	}
}

func (e *UnsupportedExecutionModeError) UserMessage() string {
	provider := strings.TrimSpace(strings.ToLower(e.Provider))
	switch provider {
	case "codex":
		return "Auto-execution did not start. Codex staged execution is paused because the current daemon still uses the app-server adapter and does not expose native plan/build mode."
	default:
		return e.Error()
	}
}

func ValidateRuntimeExecutionModeSupport(ctx context.Context, queries *db.Queries, runtimeID pgtype.UUID, mode string) error {
	runtime, err := queries.GetAgentRuntime(ctx, runtimeID)
	if err != nil {
		return fmt.Errorf("load runtime: %w", err)
	}

	_ = mode
	_ = runtime
	return nil
}
