package opencodev2

import (
	"encoding/json"
	"strings"

	"github.com/datichb/openhub/cli/internal/adapters"
)

// cliConfigEnv carries a CLI configuration that opencode merges over the
// user's own cli.json.
const cliConfigEnv = "OPENCODE_CLI_CONFIG_CONTENT"

var _ adapters.SingleSessionClient = (*Adapter)(nil)

// SingleSessionAttachCommand implements adapters.SingleSessionClient: the
// session tabs of the TUI are turned off. They are remembered per directory
// whatever the server, so opening another session of the same project added
// a tab to the window already open (A43). A configuration the user already
// passes in the variable is kept.
func (a *Adapter) SingleSessionAttachCommand(h adapters.ServerHandle, sessionID string, base []string) (argv, env []string) {
	argv, env = a.AttachCommand(h, sessionID)
	return argv, append(env, cliConfigEnv+"="+tabsOff(lookupEnv(base, cliConfigEnv)))
}

// tabsOff sets tabs.mode = "off" in a CLI configuration (JSON; an unreadable
// one is replaced).
func tabsOff(current string) string {
	cfg := map[string]any{}
	if strings.TrimSpace(current) != "" {
		if err := json.Unmarshal([]byte(current), &cfg); err != nil || cfg == nil {
			cfg = map[string]any{}
		}
	}
	tabs, _ := cfg["tabs"].(map[string]any)
	if tabs == nil {
		tabs = map[string]any{}
	}
	delete(tabs, "enabled") // older form, would be migrated to a mode
	tabs["mode"] = "off"
	cfg["tabs"] = tabs
	out, _ := json.Marshal(cfg)
	return string(out)
}

func lookupEnv(env []string, name string) string {
	v := ""
	for _, kv := range env {
		if k, val, ok := strings.Cut(kv, "="); ok && k == name {
			v = val // the last one wins, as for exec
		}
	}
	return v
}
