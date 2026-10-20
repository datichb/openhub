package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// mcpTokensReport checks the token of every enabled hub MCP service: the
// environment variable, then the keychain under the configured key (current
// name, legacy name as a fallback). A key whose value is empty or not found
// is reported with its name and the command that creates it again.
func mcpTokensReport(cfg *config.Config, secrets domain.SecretStore) (string, bool) {
	type keyCheck struct {
		service string
		enabled bool
		envVar  string
		keyName string
	}
	keys := []keyCheck{
		{"figma", cfg.MCP.Figma.Enabled, "FIGMA_TOKEN", cfg.MCP.Figma.Token},
		{"gitlab", cfg.MCP.Gitlab.Enabled, "GITLAB_TOKEN", cfg.MCP.Gitlab.Token},
		{"jira", cfg.MCP.Jira.Enabled, "JIRA_TOKEN", cfg.MCP.Jira.Token},
		{"gslides", cfg.MCP.Gslides.Enabled, "GOOGLE_ACCESS_TOKEN", cfg.MCP.Gslides.Token},
	}

	var configured int
	var foundDetails, missing []string
	for _, k := range keys {
		if !k.enabled {
			continue
		}
		configured++

		if k.envVar != "" && os.Getenv(k.envVar) != "" {
			foundDetails = append(foundDetails, fmt.Sprintf("%s (env)", k.service))
			continue
		}
		if k.keyName != "" && secrets != nil {
			if token, _ := config.ReadMCPToken(context.Background(), secrets, k.service, k.keyName); token != "" {
				foundDetails = append(foundDetails, fmt.Sprintf("%s (keychain)", k.service))
				continue
			}
		}
		key := k.keyName
		if key == "" {
			key = config.MCPTokenKey(k.service)
		}
		missing = append(missing, i18n.Tf("cmd.doctor.token_key_item", k.service, key, k.service))
	}

	if configured == 0 {
		return i18n.T("cmd.doctor.no_mcp_enabled"), true
	}
	if len(missing) > 0 {
		return i18n.Tf("cmd.doctor.tokens_missing", len(foundDetails), configured, strings.Join(missing, "; ")), false
	}
	return i18n.Tf("cmd.doctor.tokens_ok", len(foundDetails), configured, strings.Join(foundDetails, ", ")), true
}
