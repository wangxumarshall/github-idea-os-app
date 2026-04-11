package service

import (
	"encoding/json"
	"strings"
)

const (
	TriggerOnAssign    = "on_assign"
	TriggerOnComment   = "on_comment"
	TriggerOnMention   = "on_mention"
	TriggerOnScheduled = "on_scheduled"

	DefaultScheduledCron     = "0 9 * * 1-5"
	DefaultScheduledTimezone = "UTC"
)

type AgentTriggerSnapshot struct {
	ID      string         `json:"id,omitempty"`
	Type    string         `json:"type"`
	Enabled bool           `json:"enabled"`
	Config  map[string]any `json:"config"`
}

func DefaultScheduledTriggerConfig() map[string]any {
	return map[string]any{
		"cron":     DefaultScheduledCron,
		"timezone": DefaultScheduledTimezone,
	}
}

func DefaultTriggerEnabled(triggerType string) bool {
	switch strings.TrimSpace(triggerType) {
	case TriggerOnScheduled:
		return false
	case TriggerOnAssign, TriggerOnComment, TriggerOnMention:
		return true
	default:
		return false
	}
}

func NormalizeTriggerConfig(triggerType string, config map[string]any) map[string]any {
	if config == nil {
		config = map[string]any{}
	}
	switch strings.TrimSpace(triggerType) {
	case TriggerOnScheduled:
		normalized := map[string]any{}
		for key, value := range config {
			normalized[key] = value
		}
		if _, ok := normalized["cron"]; !ok {
			normalized["cron"] = DefaultScheduledCron
		}
		if _, ok := normalized["timezone"]; !ok {
			normalized["timezone"] = DefaultScheduledTimezone
		}
		return normalized
	default:
		return config
	}
}

func NormalizeTrigger(raw AgentTriggerSnapshot) AgentTriggerSnapshot {
	raw.Type = strings.TrimSpace(raw.Type)
	raw.Config = NormalizeTriggerConfig(raw.Type, raw.Config)
	return raw
}

func ParseAgentTriggers(raw []byte) ([]AgentTriggerSnapshot, error) {
	if raw == nil || len(raw) == 0 {
		return nil, nil
	}

	var triggers []AgentTriggerSnapshot
	if err := json.Unmarshal(raw, &triggers); err != nil {
		return nil, err
	}
	for i := range triggers {
		triggers[i] = NormalizeTrigger(triggers[i])
	}
	return triggers, nil
}

func DefaultAgentTriggers() []byte {
	payload, _ := json.Marshal([]AgentTriggerSnapshot{
		{Type: TriggerOnAssign, Enabled: true, Config: map[string]any{}},
		{Type: TriggerOnComment, Enabled: true, Config: map[string]any{}},
		{Type: TriggerOnMention, Enabled: true, Config: map[string]any{}},
		{Type: TriggerOnScheduled, Enabled: false, Config: DefaultScheduledTriggerConfig()},
	})
	return payload
}

func AgentHasTriggerEnabled(raw []byte, triggerType string) bool {
	triggers, err := ParseAgentTriggers(raw)
	if err != nil {
		return false
	}
	if len(triggers) == 0 {
		return DefaultTriggerEnabled(triggerType)
	}
	found := false
	enabled := DefaultTriggerEnabled(triggerType)
	for _, trigger := range triggers {
		if trigger.Type == triggerType {
			found = true
			enabled = trigger.Enabled
		}
	}
	if found {
		return enabled
	}
	return DefaultTriggerEnabled(triggerType)
}

func EnabledScheduledTriggers(raw []byte) []AgentTriggerSnapshot {
	triggers, err := ParseAgentTriggers(raw)
	if err != nil {
		return nil
	}
	out := make([]AgentTriggerSnapshot, 0, len(triggers))
	for _, trigger := range triggers {
		if trigger.Type == TriggerOnScheduled && trigger.Enabled {
			out = append(out, trigger)
		}
	}
	return out
}
