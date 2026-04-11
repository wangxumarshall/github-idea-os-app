package execenv

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// ResolveSharedTraeConfigFile returns the user's shared Trae config file path.
// It prefers TRAE_CONFIG_FILE and falls back to common user-level locations.
func ResolveSharedTraeConfigFile() string {
	if raw := strings.TrimSpace(os.Getenv("TRAE_CONFIG_FILE")); raw != "" {
		if abs, err := filepath.Abs(raw); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs
			}
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	var candidates []string
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, "trae_config.yaml"),
			filepath.Join(cwd, "trae_config.yml"),
			filepath.Join(cwd, "trae_config.json"),
		)
	}
	candidates = append(candidates,
		filepath.Join(home, ".trae", "trae_config.yaml"),
		filepath.Join(home, ".trae", "trae_config.yml"),
		filepath.Join(home, ".trae", "trae_config.json"),
		filepath.Join(home, "trae_config.yaml"),
		filepath.Join(home, "trae_config.yml"),
		filepath.Join(home, "trae_config.json"),
	)

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func prepareTraeHome(traeHome string, logger *slog.Logger) (string, string, error) {
	if err := os.MkdirAll(traeHome, 0o755); err != nil {
		return "", "", fmt.Errorf("create trae-home dir: %w", err)
	}

	trajectoryFile := filepath.Join(traeHome, "trajectory.json")
	sharedConfig := ResolveSharedTraeConfigFile()
	if sharedConfig == "" {
		return "", trajectoryFile, fmt.Errorf("shared Trae config not found: set TRAE_CONFIG_FILE or create ~/.trae/trae_config.yaml")
	}

	configFile := filepath.Join(traeHome, filepath.Base(sharedConfig))
	if err := copyFile(sharedConfig, configFile); err != nil {
		logger.Warn("execenv: trae config copy failed", "source", sharedConfig, "destination", configFile, "error", err)
		return "", trajectoryFile, err
	}

	return configFile, trajectoryFile, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create parent dir for %s: %w", dst, err)
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s → %s: %w", src, dst, err)
	}
	return nil
}
