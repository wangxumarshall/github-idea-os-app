package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigDetectsTraeCLI(t *testing.T) {
	tmp := t.TempDir()
	traePath := filepath.Join(tmp, "trae-cli")
	script := "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then echo 'trae-cli 0.1.0'; exit 0; fi\nexit 0\n"
	if err := os.WriteFile(traePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake trae-cli: %v", err)
	}

	t.Setenv("PATH", tmp)
	t.Setenv("MULTICA_WORKSPACES_ROOT", filepath.Join(tmp, "workspaces"))

	cfg, err := LoadConfig(Overrides{})
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	entry, ok := cfg.Agents["trae"]
	if !ok {
		t.Fatal("expected trae agent to be detected")
	}
	if entry.Path != "trae-cli" {
		t.Fatalf("entry.Path = %q, want trae-cli", entry.Path)
	}
}

func TestLoadConfigDetectsHermesCLIAndSandboxSettings(t *testing.T) {
	tmp := t.TempDir()
	hermesPath := filepath.Join(tmp, "hermes")
	script := "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then echo 'hermes 1.0.0'; exit 0; fi\nexit 0\n"
	if err := os.WriteFile(hermesPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake hermes: %v", err)
	}

	t.Setenv("PATH", tmp)
	t.Setenv("MULTICA_WORKSPACES_ROOT", filepath.Join(tmp, "workspaces"))
	t.Setenv("MULTICA_SANDBOX_DRIVER", "docker")
	t.Setenv("MULTICA_SANDBOX_IMAGE", "ghcr.io/example/multica-agent:latest")
	t.Setenv("MULTICA_SANDBOX_NETWORK_MODE", "none")

	cfg, err := LoadConfig(Overrides{})
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	entry, ok := cfg.Agents["hermes"]
	if !ok {
		t.Fatal("expected hermes agent to be detected")
	}
	if entry.Path != "hermes" {
		t.Fatalf("entry.Path = %q, want hermes", entry.Path)
	}
	if cfg.SandboxDriver != "docker" {
		t.Fatalf("SandboxDriver = %q", cfg.SandboxDriver)
	}
	if cfg.SandboxImage != "ghcr.io/example/multica-agent:latest" {
		t.Fatalf("SandboxImage = %q", cfg.SandboxImage)
	}
	if cfg.SandboxNetworkMode != "none" {
		t.Fatalf("SandboxNetworkMode = %q", cfg.SandboxNetworkMode)
	}
}
