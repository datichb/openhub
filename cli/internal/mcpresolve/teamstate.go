package mcpresolve

import "github.com/datichb/openhub/cli/internal/teamstate"

// TeamState is what the team says about an MCP service: the three values of
// `mcp.<service>.enabled` in the config.toml of the team-state.
//
//	enabled = true   → TeamEnabled   (the team uses the service)
//	enabled = false  → TeamDisabled  (the team does not)
//	key absent       → TeamInherited (the team says nothing: each member decides)
//
// The file format is the one every version of oh reads: the key is a boolean
// that is written only when the team has an opinion. Going back to
// "inherited" removes the key.
//
// Where the opinion counts (ResolveFull): an enforced one beats everything,
// otherwise it is only the fallback of a member (or project) with no explicit
// choice of their own; "inherited" never changes what a member has chosen.
type TeamState int

const (
	// TeamInherited: the team has no opinion (the key is absent).
	TeamInherited TeamState = iota
	// TeamEnabled: the team uses the service (`enabled = true`).
	TeamEnabled
	// TeamDisabled: the team does not use the service (`enabled = false`).
	TeamDisabled
)

// TeamStateOf reads the state of a service in the config of the team.
func TeamStateOf(s teamstate.SharedMCPConfig) TeamState {
	switch {
	case s.Enabled == nil:
		return TeamInherited
	case *s.Enabled:
		return TeamEnabled
	default:
		return TeamDisabled
	}
}

// Next is the state after this one when the user cycles through them:
// inherited → enabled → disabled → inherited.
func (s TeamState) Next() TeamState {
	switch s {
	case TeamInherited:
		return TeamEnabled
	case TeamEnabled:
		return TeamDisabled
	default:
		return TeamInherited
	}
}

// Value is the state as the screens exchange it: "" for inherited, "true" and
// "false" otherwise (the same text as the former boolean).
func (s TeamState) Value() string {
	switch s {
	case TeamEnabled:
		return "true"
	case TeamDisabled:
		return "false"
	default:
		return ""
	}
}

// ParseTeamState is the state a Value designates; any other text is the
// inherited state (a value that says nothing).
func ParseTeamState(v string) TeamState {
	switch v {
	case "true":
		return TeamEnabled
	case "false":
		return TeamDisabled
	default:
		return TeamInherited
	}
}

// SetTeamState puts a service of the team config in a state. Inherited removes
// the key (and the enforcement that went with it, which means nothing on its
// own); an entry left with nothing in it is removed too, so that no empty
// `[mcp.<service>]` table is written. The other settings of the entry (URL,
// write recommendation) are kept.
func SetTeamState(cfg *teamstate.TeamConfig, service string, st TeamState) {
	if cfg == nil {
		return
	}
	entry := cfg.MCP[service]
	switch st {
	case TeamEnabled:
		entry.Enabled = flag(true)
	case TeamDisabled:
		entry.Enabled = flag(false)
	default:
		entry.Enabled, entry.EnabledEnforced = nil, nil
		if entry == (teamstate.SharedMCPConfig{}) {
			delete(cfg.MCP, service)
			return
		}
	}
	if cfg.MCP == nil {
		cfg.MCP = map[string]teamstate.SharedMCPConfig{}
	}
	cfg.MCP[service] = entry
}

func flag(b bool) *bool { return &b }
