package handler

import (
	"encoding/json"
	"testing"
)

func TestMergeRuntimeMetadataPreservesSSHConfig(t *testing.T) {
	existing := []byte(`{"ssh_enabled":true,"ssh_host":"127.0.0.1","ssh_port":22,"ssh_user":"ubuntu","version":"old","cli_version":"old-cli"}`)

	merged := mergeRuntimeMetadata(existing, "new-version", "new-cli")

	var payload map[string]any
	if err := json.Unmarshal(merged, &payload); err != nil {
		t.Fatalf("unmarshal merged metadata: %v", err)
	}

	if payload["ssh_host"] != "127.0.0.1" {
		t.Fatalf("expected ssh_host to be preserved, got %#v", payload["ssh_host"])
	}
	if payload["ssh_user"] != "ubuntu" {
		t.Fatalf("expected ssh_user to be preserved, got %#v", payload["ssh_user"])
	}
	if payload["version"] != "new-version" {
		t.Fatalf("expected version to be updated, got %#v", payload["version"])
	}
	if payload["cli_version"] != "new-cli" {
		t.Fatalf("expected cli_version to be updated, got %#v", payload["cli_version"])
	}
}
