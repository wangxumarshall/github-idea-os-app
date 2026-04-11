package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncodeDecodeTraeSessionID(t *testing.T) {
	t.Parallel()

	sessionID := encodeTraeSessionID("/tmp/trae-home/trajectory.json")
	got, err := decodeTraeSessionID(sessionID)
	if err != nil {
		t.Fatalf("decodeTraeSessionID error: %v", err)
	}
	if got.TrajectoryFile != "/tmp/trae-home/trajectory.json" {
		t.Fatalf("trajectory_file = %q", got.TrajectoryFile)
	}
}

func TestBuildTraePromptIncludesResumeBlock(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	trajectoryPath := filepath.Join(dir, "trajectory.json")
	data := `{
  "task": "Previous task",
  "final_result": "Previous result",
  "agent_steps": [
    {
      "step_number": 3,
      "state": "completed",
      "llm_response": {
        "content": "Last step output"
      }
    }
  ]
}`
	if err := os.WriteFile(trajectoryPath, []byte(data), 0o644); err != nil {
		t.Fatalf("write trajectory: %v", err)
	}

	prompt := buildTraePrompt("New task", "System rule", encodeTraeSessionID(trajectoryPath))
	for _, want := range []string{
		"System instructions:",
		"Previous Trae execution context:",
		"Previous final result: Previous result",
		"Task:\nNew task",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestResolveTraeTrajectoryFilePrefersEnv(t *testing.T) {
	t.Parallel()

	got := resolveTraeTrajectoryFile("/tmp/workdir", map[string]string{
		"MULTICA_TRAE_TRAJECTORY_FILE": "/tmp/custom.json",
	})
	if got != "/tmp/custom.json" {
		t.Fatalf("resolveTraeTrajectoryFile = %q", got)
	}
}

func TestExtractTraeFinalOutputFallsBackToLastStep(t *testing.T) {
	t.Parallel()

	traj := &traeTrajectory{
		AgentSteps: []traeAgentStep{
			{
				LLMResponse: &traeLLMResponse{Content: "step output"},
			},
		},
	}
	if got := extractTraeFinalOutput(traj); got != "step output" {
		t.Fatalf("extractTraeFinalOutput = %q", got)
	}
}
