package execenv

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

var hermesCopiedFiles = []string{
	"config.yaml",
	"SOUL.md",
}

func prepareHermesHome(hermesHome string, logger *slog.Logger) error {
	sharedHome := resolveSharedHermesHome()
	if err := os.MkdirAll(hermesHome, 0o755); err != nil {
		return fmt.Errorf("create hermes-home dir: %w", err)
	}

	for _, name := range hermesCopiedFiles {
		src := filepath.Join(sharedHome, name)
		dst := filepath.Join(hermesHome, name)
		if err := copyFileIfExists(src, dst); err != nil {
			logger.Warn("execenv: hermes-home copy failed", "file", name, "error", err)
		}
	}

	return nil
}

func resolveSharedHermesHome() string {
	if v := os.Getenv("HERMES_HOME"); v != "" {
		if abs, err := filepath.Abs(v); err == nil {
			return abs
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("/tmp", ".hermes")
	}
	return filepath.Join(home, ".hermes")
}
