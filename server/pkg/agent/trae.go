package agent

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const traeSessionPrefix = "trae:"

type traeBackend struct {
	cfg Config
}

type traeSyntheticSession struct {
	Version        int    `json:"version"`
	TrajectoryFile string `json:"trajectory_file"`
}

type traeTrajectory struct {
	Task            string               `json:"task"`
	StartTime       string               `json:"start_time"`
	EndTime         string               `json:"end_time"`
	Provider        string               `json:"provider"`
	Model           string               `json:"model"`
	LLMInteractions []traeLLMInteraction `json:"llm_interactions"`
	AgentSteps      []traeAgentStep      `json:"agent_steps"`
	Success         bool                 `json:"success"`
	FinalResult     string               `json:"final_result"`
}

type traeLLMInteraction struct {
	Timestamp string          `json:"timestamp"`
	Provider  string          `json:"provider"`
	Model     string          `json:"model"`
	Response  traeLLMResponse `json:"response"`
}

type traeAgentStep struct {
	StepNumber  int              `json:"step_number"`
	Timestamp   string           `json:"timestamp"`
	State       string           `json:"state"`
	LLMResponse *traeLLMResponse `json:"llm_response"`
	ToolCalls   []traeToolCall   `json:"tool_calls"`
	ToolResults []traeToolResult `json:"tool_results"`
	Reflection  string           `json:"reflection"`
	Error       string           `json:"error"`
}

type traeLLMResponse struct {
	Content   string         `json:"content"`
	Model     string         `json:"model"`
	Usage     *traeUsage     `json:"usage"`
	ToolCalls []traeToolCall `json:"tool_calls"`
}

type traeUsage struct {
	InputTokens              int64  `json:"input_tokens"`
	OutputTokens             int64  `json:"output_tokens"`
	CacheCreationInputTokens *int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     *int64 `json:"cache_read_input_tokens"`
	ReasoningTokens          *int64 `json:"reasoning_tokens"`
}

type traeToolCall struct {
	CallID    string         `json:"call_id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type traeToolResult struct {
	CallID  string `json:"call_id"`
	Success bool   `json:"success"`
	Result  string `json:"result"`
	Error   string `json:"error"`
}

type traeTrajectoryWatcher struct {
	path          string
	onMessage     func(Message)
	lastStepCount int
	callIDToTool  map[string]string
}

func (b *traeBackend) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	execPath := b.cfg.ExecutablePath
	if execPath == "" {
		execPath = "trae-cli"
	}
	if _, err := exec.LookPath(execPath); err != nil {
		return nil, fmt.Errorf("trae executable not found at %q: %w", execPath, err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 20 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)

	workingDir := strings.TrimSpace(opts.Cwd)
	if workingDir == "" {
		workingDir = os.TempDir()
	}
	absWorkingDir, err := filepath.Abs(workingDir)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("resolve trae working directory: %w", err)
	}

	trajectoryFile := resolveTraeTrajectoryFile(absWorkingDir, b.cfg.Env)
	args := []string{
		"run",
		buildTraePrompt(prompt, opts.SystemPrompt, opts.ResumeSessionID),
		"--console-type", "simple",
		"--working-dir", absWorkingDir,
		"--trajectory-file", trajectoryFile,
	}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	if opts.MaxTurns > 0 {
		args = append(args, "--max-steps", fmt.Sprintf("%d", opts.MaxTurns))
	}

	cmd := exec.CommandContext(runCtx, execPath, args...)
	cmd.Dir = absWorkingDir
	cmd.Env = buildEnv(b.cfg.Env)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("trae stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("trae stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start trae: %w", err)
	}

	b.cfg.Logger.Info("trae started", "pid", cmd.Process.Pid, "cwd", absWorkingDir, "model", opts.Model, "trajectory", trajectoryFile)

	msgCh := make(chan Message, 256)
	resCh := make(chan Result, 1)

	var stdoutBuf strings.Builder
	var stderrBuf strings.Builder
	var outMu sync.Mutex

	drainPipe := func(scanner *bufio.Scanner, level string, builder *strings.Builder) {
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			outMu.Lock()
			if builder.Len() > 0 {
				builder.WriteByte('\n')
			}
			builder.WriteString(line)
			outMu.Unlock()
			trySend(msgCh, Message{Type: MessageLog, Level: level, Content: line})
		}
	}

	stdoutDone := make(chan struct{})
	go func() {
		defer close(stdoutDone)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		drainPipe(scanner, "info", &stdoutBuf)
	}()

	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		drainPipe(scanner, "error", &stderrBuf)
	}()

	go func() {
		defer cancel()
		defer close(msgCh)
		defer close(resCh)

		startTime := time.Now()
		finalStatus := "completed"
		var finalError string

		trySend(msgCh, Message{Type: MessageStatus, Status: "running"})

		doneWatching := make(chan struct{})
		watcher := traeTrajectoryWatcher{
			path:         trajectoryFile,
			onMessage:    func(msg Message) { trySend(msgCh, msg) },
			callIDToTool: make(map[string]string),
		}

		go func() {
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-doneWatching:
					return
				case <-ticker.C:
					watcher.poll()
				}
			}
		}()

		exitErr := cmd.Wait()
		close(doneWatching)
		watcher.poll()

		<-stdoutDone
		<-stderrDone

		duration := time.Since(startTime)
		outMu.Lock()
		stdoutText := stdoutBuf.String()
		stderrText := stderrBuf.String()
		outMu.Unlock()

		traj, trajErr := loadTraeTrajectory(trajectoryFile)
		if runCtx.Err() == context.DeadlineExceeded {
			finalStatus = "timeout"
			finalError = fmt.Sprintf("trae timed out after %s", timeout)
		} else if runCtx.Err() == context.Canceled {
			finalStatus = "aborted"
			finalError = "execution cancelled"
		} else if exitErr != nil {
			finalStatus = "failed"
			finalError = strings.TrimSpace(stderrText)
			if finalError == "" {
				finalError = fmt.Sprintf("trae exited with error: %v", exitErr)
			}
		} else if trajErr == nil && !traj.Success {
			finalStatus = "failed"
			finalError = strings.TrimSpace(extractTraeError(*traj))
			if finalError == "" {
				finalError = "trae execution failed"
			}
		}

		output := strings.TrimSpace(extractTraeFinalOutput(traj))
		if output == "" {
			output = strings.TrimSpace(stdoutText)
		}

		sessionID := encodeTraeSessionID(trajectoryFile)
		if trajErr != nil {
			trySend(msgCh, Message{Type: MessageLog, Level: "warn", Content: fmt.Sprintf("failed to parse Trae trajectory: %v", trajErr)})
		}

		b.cfg.Logger.Info("trae finished", "pid", cmd.Process.Pid, "status", finalStatus, "duration", duration.Round(time.Millisecond).String())

		resCh <- Result{
			Status:     finalStatus,
			Output:     output,
			Error:      finalError,
			DurationMs: duration.Milliseconds(),
			SessionID:  sessionID,
		}
	}()

	return &Session{Messages: msgCh, Result: resCh}, nil
}

func resolveTraeTrajectoryFile(cwd string, env map[string]string) string {
	if v := strings.TrimSpace(env["MULTICA_TRAE_TRAJECTORY_FILE"]); v != "" {
		return v
	}
	return filepath.Join(cwd, ".agent_context", "trae-trajectory.json")
}

func buildTraePrompt(prompt, systemPrompt, resumeSessionID string) string {
	var sections []string
	systemPrompt = strings.TrimSpace(systemPrompt)
	if systemPrompt != "" {
		sections = append(sections, "System instructions:\n"+systemPrompt)
	}
	if resumeBlock := buildTraeResumeBlock(resumeSessionID); resumeBlock != "" {
		sections = append(sections, resumeBlock)
	}
	sections = append(sections, "Task:\n"+strings.TrimSpace(prompt))
	return strings.Join(sections, "\n\n")
}

func buildTraeResumeBlock(resumeSessionID string) string {
	session, err := decodeTraeSessionID(resumeSessionID)
	if err != nil || strings.TrimSpace(session.TrajectoryFile) == "" {
		return ""
	}
	traj, err := loadTraeTrajectory(session.TrajectoryFile)
	if err != nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("Previous Trae execution context:\n")
	if strings.TrimSpace(traj.Task) != "" {
		b.WriteString("- Previous task: " + truncateTraeText(traj.Task, 400) + "\n")
	}
	if result := strings.TrimSpace(traj.FinalResult); result != "" {
		b.WriteString("- Previous final result: " + truncateTraeText(result, 1200) + "\n")
	}
	if len(traj.AgentSteps) > 0 {
		b.WriteString("- Most recent steps:\n")
		start := len(traj.AgentSteps) - 3
		if start < 0 {
			start = 0
		}
		for _, step := range traj.AgentSteps[start:] {
			summary := traeStepSummary(step)
			if summary != "" {
				b.WriteString("  - " + summary + "\n")
			}
		}
	}
	b.WriteString("Continue from this context when it is relevant, but treat the new task instructions as authoritative.")
	return b.String()
}

func traeStepSummary(step traeAgentStep) string {
	var parts []string
	if step.StepNumber > 0 {
		parts = append(parts, fmt.Sprintf("step %d", step.StepNumber))
	}
	if step.State != "" {
		parts = append(parts, step.State)
	}
	if step.Error != "" {
		parts = append(parts, "error="+truncateTraeText(step.Error, 240))
	}
	if step.Reflection != "" {
		parts = append(parts, truncateTraeText(step.Reflection, 240))
	}
	if step.LLMResponse != nil && step.LLMResponse.Content != "" {
		parts = append(parts, truncateTraeText(step.LLMResponse.Content, 240))
	}
	return strings.Join(parts, " | ")
}

func truncateTraeText(s string, limit int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}

func encodeTraeSessionID(trajectoryFile string) string {
	payload, _ := json.Marshal(traeSyntheticSession{
		Version:        1,
		TrajectoryFile: trajectoryFile,
	})
	return traeSessionPrefix + base64.RawURLEncoding.EncodeToString(payload)
}

func decodeTraeSessionID(sessionID string) (traeSyntheticSession, error) {
	var session traeSyntheticSession
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return session, fmt.Errorf("empty session id")
	}
	if !strings.HasPrefix(sessionID, traeSessionPrefix) {
		return session, fmt.Errorf("not a Trae synthetic session id")
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(sessionID, traeSessionPrefix))
	if err != nil {
		return session, err
	}
	if err := json.Unmarshal(payload, &session); err != nil {
		return session, err
	}
	return session, nil
}

func (w *traeTrajectoryWatcher) poll() {
	traj, err := loadTraeTrajectory(w.path)
	if err != nil {
		return
	}
	if len(traj.AgentSteps) <= w.lastStepCount {
		return
	}
	for _, step := range traj.AgentSteps[w.lastStepCount:] {
		if step.LLMResponse != nil && strings.TrimSpace(step.LLMResponse.Content) != "" {
			w.onMessage(Message{Type: MessageThinking, Content: step.LLMResponse.Content})
		}
		for _, call := range step.ToolCalls {
			w.callIDToTool[call.CallID] = call.Name
			w.onMessage(Message{
				Type:   MessageToolUse,
				Tool:   call.Name,
				CallID: call.CallID,
				Input:  call.Arguments,
			})
		}
		for _, result := range step.ToolResults {
			output := strings.TrimSpace(result.Result)
			if output == "" {
				output = strings.TrimSpace(result.Error)
			}
			w.onMessage(Message{
				Type:   MessageToolResult,
				Tool:   w.callIDToTool[result.CallID],
				CallID: result.CallID,
				Output: output,
			})
		}
		if step.Error != "" {
			w.onMessage(Message{Type: MessageError, Content: step.Error})
		}
	}
	w.lastStepCount = len(traj.AgentSteps)
}

func loadTraeTrajectory(path string) (*traeTrajectory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, fmt.Errorf("empty trajectory")
	}
	var traj traeTrajectory
	if err := json.Unmarshal(data, &traj); err != nil {
		return nil, err
	}
	return &traj, nil
}

func extractTraeFinalOutput(traj *traeTrajectory) string {
	if traj == nil {
		return ""
	}
	if strings.TrimSpace(traj.FinalResult) != "" {
		return traj.FinalResult
	}
	for i := len(traj.AgentSteps) - 1; i >= 0; i-- {
		step := traj.AgentSteps[i]
		if step.LLMResponse != nil && strings.TrimSpace(step.LLMResponse.Content) != "" {
			return step.LLMResponse.Content
		}
	}
	return ""
}

func extractTraeError(traj traeTrajectory) string {
	for i := len(traj.AgentSteps) - 1; i >= 0; i-- {
		step := traj.AgentSteps[i]
		if strings.TrimSpace(step.Error) != "" {
			return step.Error
		}
	}
	return ""
}
