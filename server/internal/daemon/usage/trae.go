package usage

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type traeTrajectory struct {
	StartTime       string               `json:"start_time"`
	EndTime         string               `json:"end_time"`
	LLMInteractions []traeLLMInteraction `json:"llm_interactions"`
}

type traeLLMInteraction struct {
	Model    string          `json:"model"`
	Response traeLLMResponse `json:"response"`
}

type traeLLMResponse struct {
	Usage *traeUsage `json:"usage"`
}

type traeUsage struct {
	InputTokens              int64  `json:"input_tokens"`
	OutputTokens             int64  `json:"output_tokens"`
	CacheCreationInputTokens *int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     *int64 `json:"cache_read_input_tokens"`
}

func (s *Scanner) scanTrae() []Record {
	if len(s.workspaceRoots) == 0 {
		return nil
	}

	var allRecords []Record
	for _, root := range s.workspaceRoots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		if info, err := os.Stat(root); err != nil || !info.IsDir() {
			continue
		}

		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if d.Name() == ".repos" {
					return filepath.SkipDir
				}
				return nil
			}
			if d.Name() != "trajectory.json" {
				return nil
			}
			if !strings.Contains(filepath.ToSlash(path), "/trae-home/") {
				return nil
			}

			allRecords = append(allRecords, s.parseTraeFile(path)...)
			return nil
		})
	}

	return mergeRecords(allRecords)
}

func (s *Scanner) parseTraeFile(path string) []Record {
	data, err := os.ReadFile(path)
	if err != nil || len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}

	var traj traeTrajectory
	if err := json.Unmarshal(data, &traj); err != nil {
		return nil
	}

	date := traeTrajectoryDate(traj)
	if date == "" {
		return nil
	}

	byModel := make(map[string]*Record)
	for _, interaction := range traj.LLMInteractions {
		if interaction.Response.Usage == nil {
			continue
		}
		model := strings.TrimSpace(interaction.Model)
		if model == "" {
			model = "unknown"
		}
		record := byModel[model]
		if record == nil {
			record = &Record{
				Date:     date,
				Provider: "trae",
				Model:    model,
			}
			byModel[model] = record
		}
		record.InputTokens += interaction.Response.Usage.InputTokens
		record.OutputTokens += interaction.Response.Usage.OutputTokens
		record.CacheReadTokens += traeInt64(interaction.Response.Usage.CacheReadInputTokens)
		record.CacheWriteTokens += traeInt64(interaction.Response.Usage.CacheCreationInputTokens)
	}

	records := make([]Record, 0, len(byModel))
	for _, record := range byModel {
		records = append(records, *record)
	}
	return records
}

func traeTrajectoryDate(traj traeTrajectory) string {
	for _, raw := range []string{traj.EndTime, traj.StartTime} {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		for _, layout := range []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02T15:04:05.999999",
			"2006-01-02T15:04:05",
		} {
			if ts, err := time.Parse(layout, raw); err == nil {
				return ts.Local().Format("2006-01-02")
			}
		}
	}
	return ""
}

func traeInt64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}
