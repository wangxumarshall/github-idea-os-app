package daemon

import "github.com/multica-ai/multica/server/pkg/agent"

func canonicalMessageMetadata(provider string, msg agent.Message) map[string]any {
	metadata := map[string]any{
		"provider": provider,
	}
	if msg.CallID != "" {
		metadata["call_id"] = msg.CallID
	}
	if msg.Status != "" {
		metadata["status"] = msg.Status
	}
	if msg.Level != "" {
		metadata["level"] = msg.Level
	}
	return metadata
}
