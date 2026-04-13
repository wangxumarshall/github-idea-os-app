package agent

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"
)

// hermesBackend implements Backend by spawning Hermes Agent in one-shot query mode.
// Hermes does not expose a structured event stream here, so stdout is treated as
// plain assistant text and stderr is forwarded as log events.
type hermesBackend struct {
	cfg Config
}

func (b *hermesBackend) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	execPath := b.cfg.ExecutablePath
	if execPath == "" {
		execPath = "hermes"
	}
	if err := validateExecutable(execPath, b.cfg.Sandbox); err != nil {
		return nil, fmt.Errorf("hermes executable not available: %w", err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 20 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)

	args := []string{"chat", "--quiet", "-q", prompt}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	if opts.ResumeSessionID != "" {
		args = append(args, "--resume", opts.ResumeSessionID)
	}

	cmd, err := buildCommand(runCtx, execPath, args, opts.Cwd, b.cfg.Env, b.cfg.Sandbox)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("build hermes command: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("hermes stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("hermes stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start hermes: %w", err)
	}

	b.cfg.Logger.Info("hermes started", "pid", cmd.Process.Pid, "cwd", opts.Cwd, "model", opts.Model)

	msgCh := make(chan Message, 256)
	resCh := make(chan Result, 1)

	go func() {
		defer cancel()
		defer close(msgCh)
		defer close(resCh)

		startTime := time.Now()
		var output strings.Builder
		finalStatus := "completed"
		var finalError string
		sessionID := opts.ResumeSessionID

		trySend(msgCh, Message{Type: MessageStatus, Status: "running"})

		stderrDone := make(chan struct{})
		go func() {
			defer close(stderrDone)
			scanner := bufio.NewScanner(stderr)
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" {
					continue
				}
				trySend(msgCh, Message{Type: MessageLog, Level: "info", Content: line})
			}
		}()

		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			if output.Len() > 0 {
				output.WriteByte('\n')
			}
			output.WriteString(line)
			trySend(msgCh, Message{Type: MessageText, Content: line})
		}

		<-stderrDone
		exitErr := cmd.Wait()
		duration := time.Since(startTime)

		if runCtx.Err() == context.DeadlineExceeded {
			finalStatus = "timeout"
			finalError = fmt.Sprintf("hermes timed out after %s", timeout)
		} else if runCtx.Err() == context.Canceled {
			finalStatus = "aborted"
			finalError = "execution cancelled"
		} else if scanErr := scanner.Err(); scanErr != nil {
			finalStatus = "failed"
			finalError = fmt.Sprintf("stdout read error: %v", scanErr)
		} else if exitErr != nil {
			finalStatus = "failed"
			finalError = fmt.Sprintf("hermes exited with error: %v", exitErr)
		}

		resCh <- Result{
			Status:     finalStatus,
			Output:     output.String(),
			Error:      finalError,
			DurationMs: duration.Milliseconds(),
			SessionID:  sessionID,
		}
	}()

	return &Session{Messages: msgCh, Result: resCh}, nil
}
