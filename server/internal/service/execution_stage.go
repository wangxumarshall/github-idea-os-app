package service

import (
	"strings"

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
