package daemon

// AgentEntry describes a single available agent CLI.
type AgentEntry struct {
	Path  string // path to CLI binary
	Model string // model override (optional)
}

// Runtime represents a registered daemon runtime.
type Runtime struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Status   string `json:"status"`
}

// RepoData holds repository information from the workspace.
type RepoData struct {
	URL         string `json:"url"`
	Description string `json:"description"`
}

// Task represents a claimed task from the server.
// Agent data (name, skills) is populated by the claim endpoint.
type Task struct {
	ID                      string     `json:"id"`
	AgentID                 string     `json:"agent_id"`
	RuntimeID               string     `json:"runtime_id"`
	IssueID                 string     `json:"issue_id"`
	WorkspaceID             string     `json:"workspace_id"`
	Mode                    string     `json:"mode"`
	Agent                   *AgentData `json:"agent,omitempty"`
	Repos                   []RepoData `json:"repos,omitempty"`
	SelectedRepoURL         string     `json:"selected_repo_url,omitempty"`
	SelectedRepoDescription string     `json:"selected_repo_description,omitempty"`
	IdeaSlug                string     `json:"idea_slug,omitempty"`
	IdeaCode                string     `json:"idea_code,omitempty"`
	IdeaTitle               string     `json:"idea_title,omitempty"`
	PriorSessionID          string     `json:"prior_session_id,omitempty"`   // Claude session ID from a previous task on this issue
	PriorWorkDir            string     `json:"prior_work_dir,omitempty"`     // work_dir from a previous task on this issue
	TriggerCommentID        string     `json:"trigger_comment_id,omitempty"` // comment that triggered this task
}

// AgentData holds agent details returned by the claim endpoint.
type AgentData struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Instructions string      `json:"instructions"`
	Skills       []SkillData `json:"skills"`
}

// SkillData represents a structured skill for task execution.
type SkillData struct {
	Name    string          `json:"name"`
	Content string          `json:"content"`
	Files   []SkillFileData `json:"files,omitempty"`
}

// SkillFileData represents a supporting file within a skill.
type SkillFileData struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// TaskResult is the outcome of executing a task.
type TaskResult struct {
	Status               string   `json:"status"`
	Comment              string   `json:"comment"`
	Summary              string   `json:"summary,omitempty"`
	PRURL                string   `json:"pr_url,omitempty"`
	CompareURL           string   `json:"compare_url,omitempty"`
	BranchName           string   `json:"branch_name,omitempty"`
	DeliveryState        string   `json:"delivery_state,omitempty"`
	HandoffReason        string   `json:"handoff_reason,omitempty"`
	PlanStatus           string   `json:"plan_status,omitempty"`
	PlanRequiresDecision bool     `json:"plan_requires_decision,omitempty"`
	PlanQuestions        []string `json:"plan_questions,omitempty"`
	EnvType              string   `json:"env_type,omitempty"`
	SessionID            string   `json:"session_id,omitempty"` // Claude session ID for future resumption
	WorkDir              string   `json:"work_dir,omitempty"`   // working directory used during execution
}
