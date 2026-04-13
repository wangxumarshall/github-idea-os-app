package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildCommandDockerWrapsExecutable(t *testing.T) {
	t.Parallel()

	workdir := t.TempDir()
	stateDir := filepath.Join(workdir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir state: %v", err)
	}

	cmd, err := buildCommand(context.Background(), "hermes", []string{"chat", "--quiet"}, workdir, map[string]string{
		"HERMES_HOME":         stateDir,
		"PATH":                "/home/ubuntu/multica/server/bin:/usr/local/bin",
		"MULTICA_DAEMON_PORT": "19514",
	}, SandboxConfig{
		Driver:      "docker",
		Image:       "ghcr.io/example/multica-agent:latest",
		NetworkMode: "none",
	})
	if err != nil {
		t.Fatalf("buildCommand error: %v", err)
	}

	if got := cmd.Args[0]; got != "docker" {
		t.Fatalf("cmd.Args[0] = %q, want docker", got)
	}
	args := strings.Join(cmd.Args, " ")
	for _, want := range []string{
		"--network host",
		"ghcr.io/example/multica-agent:latest hermes chat --quiet",
		workdir + ":" + workdir,
		stateDir + ":" + stateDir,
		"/home/ubuntu/multica/server/bin:/home/ubuntu/multica/server/bin",
		"-e PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/app:/home/ubuntu/multica/server/bin",
	} {
		if !strings.Contains(args, want) {
			t.Fatalf("docker args missing %q: %s", want, args)
		}
	}
}
