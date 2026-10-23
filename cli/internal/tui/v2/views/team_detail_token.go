package views

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// Keys of the fields of team detail that are edited by a flow of their own.
const (
	fieldTrackerToken  = "tracker_token"
	fieldTeamGitLabURL = "team_gitlab_url"
)

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// serverCheck is the rule of the fields that hold a server (a URL or a host):
// empty means « not declared ».
func serverCheck(value string) error {
	if err := tracker.ValidateServer(strings.TrimSpace(value)); err != nil {
		return errors.New(i18n.Tf("tui.team.detail.server_invalid", value))
	}
	return nil
}

// effectiveTracker is the tracker configuration in force: the team config as
// edited merged with the local overrides as edited.
func (v *TeamDetailView) effectiveTracker() tracker.EffectiveTrackerConfig {
	return tracker.ResolveTrackerConfig(&v.teamCfg.Tracker, v.localTrk)
}

// tokenFields are the fields that say where the token of the tracker is:
// what the team recommends (key and server, in config.toml), what the member
// declares (key, server and the token itself, in hub.toml and the keychain)
// and the GitLab of the team-state repository.
func (v *TeamDetailView) tokenFields() []configField {
	urlRule := &FieldValidator{Check: serverCheck, AllowEmpty: true}
	return []configField{
		{
			Key: "tracker_token_key", Kind: CfgFieldString, Label: i18n.T("tui.team.detail.token_key.label"),
			Description: i18n.T("tui.team.detail.token_key.desc"), Scope: ScopeTeamShared,
			Placeholder: i18n.T("tui.team.detail.token_key.placeholder"),
			Get:         func() string { return v.teamCfg.Tracker.TrackerTokenKey },
			Set: func(val string) {
				v.teamCfg.Tracker.TrackerTokenKey = strings.TrimSpace(val)
				if v.teamCfg.Tracker.TrackerTokenKey == "" {
					// a server without a key means nothing
					v.teamCfg.Tracker.TrackerTokenURL = ""
				}
				v.dirtyTeam = true
			},
		},
		{
			Key: "tracker_token_url", Kind: CfgFieldString, Label: i18n.T("tui.team.detail.token_url.label"),
			Description: i18n.T("tui.team.detail.token_url.desc"), Scope: ScopeTeamShared,
			Placeholder: i18n.T("tui.team.detail.token_url.placeholder"),
			Validator:   urlRule,
			Get:         func() string { return v.teamCfg.Tracker.TrackerTokenURL },
			Set: func(val string) {
				v.teamCfg.Tracker.TrackerTokenURL = strings.TrimSpace(val)
				v.dirtyTeam = true
			},
		},
		{
			Key: fieldTrackerToken, Kind: CfgFieldString, Label: i18n.T("tui.team.detail.token.label"),
			Description: i18n.T("tui.team.detail.token.desc"), Scope: ScopeTeamLocal,
			Get:    v.tokenValue,
			Source: v.tokenSource,
			// the flow of editSelected asks the key, the server and the token
			Set: func(string) {},
		},
		{
			Key: fieldTeamGitLabURL, Kind: CfgFieldString, Label: i18n.T("tui.team.detail.team_gitlab.label"),
			Description: i18n.T("tui.team.detail.team_gitlab.desc"), Scope: ScopeTeamLocal,
			Placeholder: i18n.T("tui.team.detail.team_gitlab.placeholder"),
			Validator:   urlRule,
			Get:         func() string { return v.cfg.ResolveTeam().GitLabURL },
			Set:         v.setTeamGitLabURL,
		},
	}
}

// tokenValue is what the field of the token shows: the key in force and the
// server it is valid for, or nothing when no key is declared (the field then
// reads « non configuré » — the default key openhub.tracker.<type>.token is
// read for GitLab without being declared, it is not shown as if it were).
func (v *TeamDetailView) tokenValue() string {
	eff := v.effectiveTracker()
	if eff.TrackerTokenKey == "" {
		return ""
	}
	if eff.Type == string(tracker.TypeJira) {
		return eff.TrackerTokenKey
	}
	server, implicit := eff.TokenServer()
	if implicit {
		return i18n.Tf("tui.team.detail.token_value_implicit", eff.TrackerTokenKey)
	}
	return i18n.Tf("tui.team.detail.token_value", eff.TrackerTokenKey, firstNonEmpty(tracker.ServerOf(server), server))
}

// tokenSource says which configuration declares the key in force.
func (v *TeamDetailView) tokenSource() string {
	switch {
	case v.localTrk.TrackerTokenKey != "":
		return i18n.T("tui.team.detail.source_local")
	case v.teamCfg.Tracker.TrackerTokenKey != "":
		return i18n.T("tui.team.detail.source_team")
	}
	return ""
}

// claimFields are the lease and the heartbeat of the claims ([claim] of the
// team config), checked together with ClaimConfig.Validate.
func (v *TeamDetailView) claimFields() []configField {
	check := func(set func(*teamstate.ClaimConfig, int)) func(string) error {
		return func(val string) error {
			// a value that is not a number reads 0, which Validate accepts: the
			// number rule of the field is the one that reports it
			n, _ := strconv.Atoi(val)
			c := v.teamCfg.Claim
			set(&c, n)
			if err := c.Validate(); err != nil {
				return errors.New(i18n.Tf("tui.team.detail.claim_invalid", err.Error()))
			}
			return nil
		}
	}
	return []configField{
		{
			Key: "lease_hours", Kind: CfgFieldInt, Label: i18n.T("tui.team.detail.lease.label"),
			Description: i18n.T("tui.team.detail.lease.desc"), Scope: ScopeTeamShared,
			Placeholder: i18n.T("tui.team.detail.lease.placeholder"),
			Validator: &FieldValidator{Numeric: true, MinInt: intPtr(teamstate.MinLeaseHours), MaxInt: intPtr(teamstate.MaxLeaseHours),
				Check: check(func(c *teamstate.ClaimConfig, n int) { c.LeaseHours = n })},
			Get: func() string { return strconv.Itoa(v.teamCfg.Claim.LeaseHours) },
			Set: func(val string) {
				if n, err := strconv.Atoi(val); err == nil {
					v.teamCfg.Claim.LeaseHours = n
					v.dirtyTeam = true
				}
			},
		},
		{
			Key: "heartbeat_minutes", Kind: CfgFieldInt, Label: i18n.T("tui.team.detail.heartbeat.label"),
			Description: i18n.T("tui.team.detail.heartbeat.desc"), Scope: ScopeTeamShared,
			Placeholder: i18n.T("tui.team.detail.heartbeat.placeholder"),
			Validator: &FieldValidator{Numeric: true, MinInt: intPtr(0), MaxInt: intPtr(teamstate.MaxHeartbeatMinutes),
				Check: check(func(c *teamstate.ClaimConfig, n int) { c.HeartbeatMinutes = n })},
			Get: func() string { return strconv.Itoa(v.teamCfg.Claim.HeartbeatMinutes) },
			Set: func(val string) {
				if n, err := strconv.Atoi(val); err == nil {
					v.teamCfg.Claim.HeartbeatMinutes = n
					v.dirtyTeam = true
				}
			},
		},
	}
}

// setTeamGitLabURL declares the GitLab of the team-state repository in the
// team of hub.toml. It is written at once: it is a setting of the machine
// (the alias it replaces is a name of ~/.ssh/config), not part of the team
// config that « w » publishes.
func (v *TeamDetailView) setTeamGitLabURL(val string) {
	if v.shell == nil || v.cfg.UpdateHubConfig == nil {
		return
	}
	val = strings.TrimSpace(val)
	id := v.cfg.ResolveTeam().TeamID
	err := v.cfg.UpdateHubConfig(func(c *config.Config) error {
		t := c.FindTeam(id)
		if t == nil {
			return errors.New(i18n.Tf("tui.team.detail.team_not_in_hub", id))
		}
		t.GitLabURL = val
		return nil
	})
	if err != nil {
		v.shell.ShowToastMsg(i18n.Tf("tui.team.detail.save_local_error", err.Error()), false)
		return
	}
	v.shell.ShowToastMsg(i18n.Tf("tui.team.detail.saved", i18n.T("tui.team.detail.saved_local")), true)
}

// promptTokenSetup asks where the token of the tracker is — its key in the
// keychain, then the server it is valid for (GitLab) — then the token, and
// writes the declaration to hub.toml ([tracker] tracker_token_key and
// tracker_token_url) and the token to the keychain under that key. The key
// proposed is the one in force, else the default one, said as a proposal.
func (v *TeamDetailView) promptTokenSetup() {
	if v.shell == nil || v.teamCfg == nil {
		return
	}
	trackerType := v.teamCfg.Tracker.Type
	if trackerType == "" {
		trackerType = string(tracker.TypeGitLab)
	}
	eff := v.effectiveTracker()
	eff.Type = trackerType
	proposed := firstNonEmpty(eff.TrackerTokenKey, tracker.DefaultTokenKey(trackerType))

	v.shell.ShowInputModal(i18n.Tf("tui.team.detail.token_key_prompt", trackerType), proposed, func(key string) {
		key = firstNonEmpty(strings.TrimSpace(key), proposed)
		if trackerType == string(tracker.TypeJira) {
			v.promptToken(eff, key, "")
			return
		}
		v.shell.ShowInputModal(i18n.T("tui.team.detail.token_server_prompt"), eff.TrackerTokenURL, func(server string) {
			server = strings.TrimSpace(server)
			if err := serverCheck(server); err != nil {
				v.shell.ShowToastMsg(err.Error(), false)
				return
			}
			v.promptToken(eff, key, server)
		})
	})
}

func (v *TeamDetailView) promptToken(eff tracker.EffectiveTrackerConfig, key, server string) {
	v.shell.ShowPasswordModal(i18n.Tf("tui.team.detail.token_prompt", eff.Type, key), func(token string) {
		v.applyTokenDecl(eff, key, server, token)
	})
}

// applyTokenDecl stores the token (when one is typed) under key, then declares
// key and server in hub.toml when the configuration in force does not already
// say so (tracker.EffectiveTrackerConfig.NeedsTokenDecl). The token goes first:
// a key is not declared for a token the keychain refused.
func (v *TeamDetailView) applyTokenDecl(eff tracker.EffectiveTrackerConfig, key, server, token string) {
	if token != "" {
		if err := v.storeSecret(key, token); err != nil {
			v.shell.ShowToastMsg(i18n.Tf("tui.team.detail.token_save_error", err.Error()), false)
			return
		}
	}
	declared := false
	if eff.NeedsTokenDecl(key, server) {
		if v.cfg.UpdateHubConfig == nil {
			return
		}
		if err := v.cfg.UpdateHubConfig(func(c *config.Config) error {
			c.Tracker.SetTokenDecl(key, server)
			return nil
		}); err != nil {
			v.shell.ShowToastMsg(i18n.Tf("tui.team.detail.save_local_error", err.Error()), false)
			return
		}
		// what hub.toml now holds is the base of the next save, not an edit
		v.localTrk.SetTokenDecl(key, server)
		v.baseTrk.SetTokenDecl(key, server)
		declared = true
	}
	switch {
	case token != "":
		v.shell.ShowToastMsg(i18n.T("tui.team.detail.token_saved"), true)
	case declared:
		v.shell.ShowToastMsg(i18n.Tf("tui.team.detail.token_declared", key), true)
	default:
		v.shell.ShowToastMsg(i18n.T("tui.team.detail.nothing_to_save"), false)
	}
	v.buildFields()
	v.renderFields()
}

// storeSecret writes a secret to the keychain.
func (v *TeamDetailView) storeSecret(key, value string) error {
	ctx := context.Background()
	if v.shell != nil {
		ctx = v.shell.Context()
	}
	if v.cfg.SetSecret != nil {
		return v.cfg.SetSecret(ctx, key, value)
	}
	if v.cfg.GetSecrets != nil {
		if setter, ok := v.cfg.GetSecrets().(interface {
			Set(ctx context.Context, key, value string) error
		}); ok {
			return setter.Set(ctx, key, value)
		}
	}
	return errors.New(i18n.T("tui.views.keychain_unavailable"))
}
