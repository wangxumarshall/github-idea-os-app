package usage

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestParseTraeFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "ws-1", "task-1", "trae-home")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `{
  "start_time": "2026-04-12T10:00:00",
  "end_time": "2026-04-12T10:05:00",
  "llm_interactions": [
    {
      "model": "claude-sonnet-4-20250514",
      "response": {
        "usage": {
          "input_tokens": 100,
          "output_tokens": 40,
          "cache_creation_input_tokens": 5,
          "cache_read_input_tokens": 20
        }
      }
    },
    {
      "model": "claude-sonnet-4-20250514",
      "response": {
        "usage": {
          "input_tokens": 50,
          "output_tokens": 10,
          "cache_creation_input_tokens": 0,
          "cache_read_input_tokens": 15
        }
      }
    },
    {
      "model": "gpt-4o",
      "response": {
        "usage": {
          "input_tokens": 70,
          "output_tokens": 30,
          "cache_creation_input_tokens": 0,
          "cache_read_input_tokens": 0
        }
      }
    }
  ]
}`
	filePath := filepath.Join(path, "trajectory.json")
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewScanner(slog.Default(), root)
	records := s.parseTraeFile(filePath)
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	found := map[string]Record{}
	for _, record := range records {
		found[record.Model] = record
	}

	claude := found["claude-sonnet-4-20250514"]
	if claude.Provider != "trae" {
		t.Fatalf("claude provider = %q", claude.Provider)
	}
	if claude.Date != "2026-04-12" {
		t.Fatalf("claude date = %q", claude.Date)
	}
	if claude.InputTokens != 150 || claude.OutputTokens != 50 {
		t.Fatalf("claude tokens = %#v", claude)
	}
	if claude.CacheReadTokens != 35 || claude.CacheWriteTokens != 5 {
		t.Fatalf("claude cache tokens = %#v", claude)
	}

	gpt := found["gpt-4o"]
	if gpt.InputTokens != 70 || gpt.OutputTokens != 30 {
		t.Fatalf("gpt tokens = %#v", gpt)
	}
}

func TestScanTraeWalksWorkspaceRoots(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	traeHome := filepath.Join(root, "ws-1", "task-1", "trae-home")
	if err := os.MkdirAll(traeHome, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `{
  "start_time": "2026-04-12T10:00:00",
  "llm_interactions": [
    {
      "model": "gpt-4o",
      "response": {
        "usage": {
          "input_tokens": 100,
          "output_tokens": 50,
          "cache_creation_input_tokens": 0,
          "cache_read_input_tokens": 0
        }
      }
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(traeHome, "trajectory.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewScanner(slog.Default(), root)
	records := s.scanTrae()
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Provider != "trae" {
		t.Fatalf("provider = %q", records[0].Provider)
	}
}
