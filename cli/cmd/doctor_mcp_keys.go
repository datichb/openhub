package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/mcpresolve"
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
	var keys []keyCheck
	for _, svc := range mcpresolve.Services() {
		if hub := cfg.MCPServer(svc.Name); hub != nil {
			keys = append(keys, keyCheck{svc.Name, hub.Enabled, svc.TokenEnv, hub.Token})
		}
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
