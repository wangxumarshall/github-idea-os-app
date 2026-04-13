package execenv

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

var defaultDeniedPathPatterns = []string{
	"~/.ssh/*",
	"~/.aws/*",
	"~/.kube/*",
	".env",
	".env.*",
}

// RuntimePolicyFilePath returns the standard path to the task policy file.
func RuntimePolicyFilePath(workDir string) string {
	return filepath.Join(workDir, ".agent_context", "policy.json")
}

type runtimePolicy struct {
	Version                int      `json:"version"`
	Mode                   string   `json:"mode"`
	ReadOnly               bool     `json:"read_only"`
	RepoCheckoutRestricted bool     `json:"repo_checkout_restricted"`
	PreferredRepoURL       string   `json:"preferred_repo_url,omitempty"`
	AvailableRepoURLs      []string `json:"available_repo_urls,omitempty"`
	DeniedPathPatterns     []string `json:"denied_path_patterns"`
	NetworkAccess          string   `json:"network_access"`
	AllowedNetworkDomains  []string `json:"allowed_network_domains"`
}

func writePolicyFile(workDir string, ctx TaskContextForEnv) error {
	availableRepos := make([]string, 0, len(ctx.Repos))
	for _, repo := range ctx.Repos {
		if url := strings.TrimSpace(repo.URL); url != "" {
			availableRepos = append(availableRepos, url)
		}
	}

	policy := runtimePolicy{
		Version:                1,
		Mode:                   strings.TrimSpace(ctx.Mode),
		ReadOnly:               strings.TrimSpace(ctx.Mode) == "plan",
		RepoCheckoutRestricted: strings.TrimSpace(ctx.SelectedRepoURL) != "",
		PreferredRepoURL:       strings.TrimSpace(ctx.SelectedRepoURL),
		AvailableRepoURLs:      availableRepos,
		DeniedPathPatterns:     append([]string(nil), defaultDeniedPathPatterns...),
		NetworkAccess:          "deny",
		AllowedNetworkDomains:  []string{},
	}

	data, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(RuntimePolicyFilePath(workDir), data, 0o644)
}
