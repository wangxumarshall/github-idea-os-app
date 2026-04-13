package agent

import "fmt"

// Capabilities describes the runtime features exposed by an agent adapter.
// The daemon publishes these as runtime metadata so orchestration can make
// capability-aware decisions without hard-coding provider checks.
type Capabilities struct {
	StructuredStream bool `json:"structured_stream"`
	NativeResume     bool `json:"native_resume"`
	NativePlanMode   bool `json:"native_plan_mode"`
	ApprovalCallback bool `json:"approval_callback"`
	TrajectoryFile   bool `json:"trajectory_file"`
	PtyOnly          bool `json:"pty_only"`
}

var capabilitiesByType = map[string]Capabilities{
	"claude": {
		StructuredStream: true,
		NativeResume:     true,
		NativePlanMode:   true,
		ApprovalCallback: true,
	},
	"codex": {
		StructuredStream: true,
		NativeResume:     true,
		ApprovalCallback: true,
	},
	"opencode": {
		StructuredStream: true,
		NativeResume:     true,
		NativePlanMode:   true,
	},
	"trae": {
		NativeResume:   true,
		NativePlanMode: true,
		TrajectoryFile: true,
	},
	"hermes": {},
}

// CapabilitiesFor returns the capability set for a supported provider.
func CapabilitiesFor(agentType string) (Capabilities, error) {
	caps, ok := capabilitiesByType[agentType]
	if !ok {
		return Capabilities{}, fmt.Errorf("unknown agent type: %q", agentType)
	}
	return caps, nil
}
