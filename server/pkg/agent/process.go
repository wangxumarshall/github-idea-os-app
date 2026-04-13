package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

var defaultContainerPATH = []string{
	"/usr/local/sbin",
	"/usr/local/bin",
	"/usr/sbin",
	"/usr/bin",
	"/sbin",
	"/bin",
	"/app",
}

func validateExecutable(execPath string, sandbox SandboxConfig) error {
	if strings.TrimSpace(sandbox.Driver) == "docker" {
		if _, err := exec.LookPath("docker"); err != nil {
			return fmt.Errorf("docker executable not found: %w", err)
		}
		return nil
	}
	if _, err := exec.LookPath(execPath); err != nil {
		return fmt.Errorf("executable not found at %q: %w", execPath, err)
	}
	return nil
}

func buildCommand(ctx context.Context, execPath string, args []string, cwd string, extraEnv map[string]string, sandbox SandboxConfig) (*exec.Cmd, error) {
	switch strings.TrimSpace(sandbox.Driver) {
	case "", "host":
		cmd := exec.CommandContext(ctx, execPath, args...)
		if cwd != "" {
			cmd.Dir = cwd
		}
		cmd.Env = buildEnv(extraEnv)
		return cmd, nil
	case "docker":
		image := strings.TrimSpace(sandbox.Image)
		if image == "" {
			return nil, fmt.Errorf("docker sandbox requires an image")
		}
		dockerEnv := sanitizeDockerEnv(extraEnv)

		dockerArgs := []string{
			"run", "--rm", "-i",
			"--cap-drop=ALL",
			"--security-opt=no-new-privileges",
			"--tmpfs", "/tmp:rw,noexec,nosuid,size=512m",
		}
		networkMode := strings.TrimSpace(sandbox.NetworkMode)
		if networkMode == "" {
			networkMode = "host"
		}
		if strings.TrimSpace(dockerEnv["MULTICA_DAEMON_PORT"]) != "" {
			networkMode = "host"
		}
		dockerArgs = append(dockerArgs, "--network", networkMode)

		for _, mount := range collectMountPaths(cwd, dockerEnv) {
			dockerArgs = append(dockerArgs, "-v", mount+":"+mount)
		}
		if cwd != "" {
			dockerArgs = append(dockerArgs, "-w", cwd)
		}
		for _, envEntry := range dockerEnvArgs(dockerEnv) {
			dockerArgs = append(dockerArgs, "-e", envEntry)
		}
		dockerArgs = append(dockerArgs, image, execPath)
		dockerArgs = append(dockerArgs, args...)

		cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
		cmd.Env = buildEnv(nil)
		return cmd, nil
	default:
		return nil, fmt.Errorf("unsupported sandbox driver %q", sandbox.Driver)
	}
}

func collectMountPaths(cwd string, extraEnv map[string]string) []string {
	paths := map[string]struct{}{}
	addPath := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" || !filepath.IsAbs(raw) {
			return
		}
		info, err := os.Stat(raw)
		if err != nil {
			return
		}
		mountPath := raw
		if !info.IsDir() {
			mountPath = filepath.Dir(raw)
		}
		if mountPath != "" {
			paths[mountPath] = struct{}{}
		}
	}

	addPath(cwd)
	for _, value := range extraEnv {
		if strings.Contains(value, string(os.PathListSeparator)) {
			for _, part := range filepath.SplitList(value) {
				if isDefaultContainerPath(part) {
					continue
				}
				addPath(part)
			}
			continue
		}
		addPath(value)
	}

	result := make([]string, 0, len(paths))
	for path := range paths {
		result = append(result, path)
	}
	sort.Strings(result)
	return result
}

func isDefaultContainerPath(path string) bool {
	path = strings.TrimSpace(path)
	for _, entry := range defaultContainerPATH {
		if path == entry {
			return true
		}
	}
	return false
}

func dockerEnvArgs(extraEnv map[string]string) []string {
	if len(extraEnv) == 0 {
		return nil
	}
	keys := make([]string, 0, len(extraEnv))
	for key := range extraEnv {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+extraEnv[key])
	}
	return result
}

func sanitizeDockerEnv(extraEnv map[string]string) map[string]string {
	env := map[string]string{}
	for key, value := range extraEnv {
		env[key] = value
	}

	pathEntries := append([]string{}, defaultContainerPATH...)
	if rawPath := strings.TrimSpace(env["PATH"]); rawPath != "" {
		for _, part := range filepath.SplitList(rawPath) {
			part = strings.TrimSpace(part)
			if part == "" || !filepath.IsAbs(part) {
				continue
			}
			pathEntries = append(pathEntries, part)
		}
	}
	env["PATH"] = joinUniquePaths(pathEntries)
	return env
}

func joinUniquePaths(entries []string) string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if _, exists := seen[entry]; exists {
			continue
		}
		seen[entry] = struct{}{}
		result = append(result, entry)
	}
	return strings.Join(result, string(os.PathListSeparator))
}
