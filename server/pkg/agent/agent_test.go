package agent

import (
	"context"
	"testing"
)

func TestNewReturnsClaudeBackend(t *testing.T) {
	t.Parallel()
	b, err := New("claude", Config{ExecutablePath: "/nonexistent/claude"})
	if err != nil {
		t.Fatalf("New(claude) error: %v", err)
	}
	if _, ok := b.(*claudeBackend); !ok {
		t.Fatalf("expected *claudeBackend, got %T", b)
	}
}

func TestNewReturnsCodexBackend(t *testing.T) {
	t.Parallel()
	b, err := New("codex", Config{ExecutablePath: "/nonexistent/codex"})
	if err != nil {
		t.Fatalf("New(codex) error: %v", err)
	}
	if _, ok := b.(*codexBackend); !ok {
		t.Fatalf("expected *codexBackend, got %T", b)
	}
}

func TestNewReturnsTraeBackend(t *testing.T) {
	t.Parallel()
	b, err := New("trae", Config{ExecutablePath: "/nonexistent/trae-cli"})
	if err != nil {
		t.Fatalf("New(trae) error: %v", err)
	}
	if _, ok := b.(*traeBackend); !ok {
		t.Fatalf("expected *traeBackend, got %T", b)
	}
}

func TestNewReturnsHermesBackend(t *testing.T) {
	t.Parallel()
	b, err := New("hermes", Config{ExecutablePath: "/nonexistent/hermes"})
	if err != nil {
		t.Fatalf("New(hermes) error: %v", err)
	}
	if _, ok := b.(*hermesBackend); !ok {
		t.Fatalf("expected *hermesBackend, got %T", b)
	}
}

func TestNewRejectsUnknownType(t *testing.T) {
	t.Parallel()
	_, err := New("gpt", Config{})
	if err == nil {
		t.Fatal("expected error for unknown agent type")
	}
}

func TestNewDefaultsLogger(t *testing.T) {
	t.Parallel()
	b, _ := New("claude", Config{})
	cb := b.(*claudeBackend)
	if cb.cfg.Logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestDetectVersionFailsForMissingBinary(t *testing.T) {
	t.Parallel()
	_, err := DetectVersion(context.Background(), "/nonexistent/binary")
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}

func TestCapabilitiesForSupportedTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		agentType string
		check     func(t *testing.T, got Capabilities)
	}{
		{
			name:      "claude",
			agentType: "claude",
			check: func(t *testing.T, got Capabilities) {
				if !got.StructuredStream || !got.NativeResume || !got.NativePlanMode || !got.ApprovalCallback {
					t.Fatalf("unexpected claude capabilities: %#v", got)
				}
			},
		},
		{
			name:      "codex",
			agentType: "codex",
			check: func(t *testing.T, got Capabilities) {
				if !got.StructuredStream || !got.NativeResume || !got.ApprovalCallback || got.NativePlanMode {
					t.Fatalf("unexpected codex capabilities: %#v", got)
				}
			},
		},
		{
			name:      "opencode",
			agentType: "opencode",
			check: func(t *testing.T, got Capabilities) {
				if !got.StructuredStream || !got.NativeResume || !got.NativePlanMode {
					t.Fatalf("unexpected opencode capabilities: %#v", got)
				}
			},
		},
		{
			name:      "trae",
			agentType: "trae",
			check: func(t *testing.T, got Capabilities) {
				if !got.NativeResume || !got.NativePlanMode || !got.TrajectoryFile || got.StructuredStream {
					t.Fatalf("unexpected trae capabilities: %#v", got)
				}
			},
		},
		{
			name:      "hermes",
			agentType: "hermes",
			check: func(t *testing.T, got Capabilities) {
				if got.NativeResume || got.NativePlanMode || got.StructuredStream {
					t.Fatalf("unexpected hermes capabilities: %#v", got)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := CapabilitiesFor(tt.agentType)
			if err != nil {
				t.Fatalf("CapabilitiesFor(%q) error: %v", tt.agentType, err)
			}
			tt.check(t, got)
		})
	}
}

func TestCapabilitiesForRejectsUnknownType(t *testing.T) {
	t.Parallel()
	if _, err := CapabilitiesFor("gpt"); err == nil {
		t.Fatal("expected error for unknown type")
	}
}
