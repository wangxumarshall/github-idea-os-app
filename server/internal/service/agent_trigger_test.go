package service

import (
	"encoding/json"
	"testing"
)

func TestParseAgentTriggersNormalizesScheduledConfig(t *testing.T) {
	raw, err := json.Marshal([]AgentTriggerSnapshot{
		{Type: TriggerOnScheduled, Enabled: true, Config: nil},
		{Type: TriggerOnMention, Enabled: true, Config: nil},
	})
	if err != nil {
		t.Fatalf("marshal triggers: %v", err)
	}

	triggers, err := ParseAgentTriggers(raw)
	if err != nil {
		t.Fatalf("ParseAgentTriggers: %v", err)
	}
	if len(triggers) != 2 {
		t.Fatalf("expected 2 triggers, got %d", len(triggers))
	}
	if triggers[0].Config == nil {
		t.Fatal("expected scheduled config to be normalized")
	}
	if triggers[0].Config["cron"] != DefaultScheduledCron {
		t.Fatalf("cron = %#v, want %q", triggers[0].Config["cron"], DefaultScheduledCron)
	}
	if triggers[0].Config["timezone"] != DefaultScheduledTimezone {
		t.Fatalf("timezone = %#v, want %q", triggers[0].Config["timezone"], DefaultScheduledTimezone)
	}
}

func TestEnabledScheduledTriggers(t *testing.T) {
	raw, err := json.Marshal([]AgentTriggerSnapshot{
		{Type: TriggerOnScheduled, Enabled: false, Config: DefaultScheduledTriggerConfig()},
		{Type: TriggerOnScheduled, Enabled: true, Config: map[string]any{"cron": "*/15 * * * *", "timezone": "UTC"}},
		{Type: TriggerOnComment, Enabled: true, Config: map[string]any{}},
	})
	if err != nil {
		t.Fatalf("marshal triggers: %v", err)
	}

	triggers := EnabledScheduledTriggers(raw)
	if len(triggers) != 1 {
		t.Fatalf("expected 1 enabled scheduled trigger, got %d", len(triggers))
	}
	if triggers[0].Config["cron"] != "*/15 * * * *" {
		t.Fatalf("cron = %#v", triggers[0].Config["cron"])
	}
}
