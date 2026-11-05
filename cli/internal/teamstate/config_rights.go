package teamstate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
)

type actorKey struct{}

// WithActor says which member changes the team (the member id of the team in
// hub.toml): UpdateConfig signs the config_changed event with it and checks
// its role for the settings that bind the others.
func WithActor(ctx context.Context, memberID string) context.Context {
	return context.WithValue(ctx, actorKey{}, memberID)
}

// ActorOf is the member WithActor gave ("" when none did).
func ActorOf(ctx context.Context) string {
	id, _ := ctx.Value(actorKey{}).(string)
	return id
}

// ErrNotLead is returned when a member who is not a lead changes a setting
// that binds the others while the team reserves them to its leads
// ([governance] config = "lead", the default). Nothing is written.
type ErrNotLead struct {
	Member string
	Keys   []string
}

func (e *ErrNotLead) Error() string {
	return i18n.Tf("v54c.governance.not_lead", e.Member, strings.Join(e.Keys, ", "))
}

// IsNotLead reports whether err is an ErrNotLead and returns it.
func IsNotLead(err error) (*ErrNotLead, bool) {
	var e *ErrNotLead
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// ChangeNeedsLead reports whether changing the setting key binds the other
// members: an *_enforced flag, the enforced ceilings of [limits.enforced] and
// the [governance] section itself.
func ChangeNeedsLead(key string) bool {
	last := key
	if i := strings.LastIndex(key, "."); i >= 0 {
		last = key[i+1:]
	}
	return strings.HasSuffix(last, "_enforced") ||
		key == "limits.enforced" || strings.HasPrefix(key, "limits.enforced.") ||
		key == "governance" || strings.HasPrefix(key, "governance.")
}

// mayEnforce reports whether member may change the settings that bind the
// others under gov: always when the team lets any member, a lead always, and
// anyone when the team has no lead at all (it is not locked out of its own
// settings).
func mayEnforce(gov GovernanceConfig, members map[string]Member, member string) bool {
	if gov.ConfigPolicy() == GovernanceConfigAnyMember {
		return true
	}
	hasLead := false
	for id, m := range members {
		if m.Role != RoleLead {
			continue
		}
		hasLead = true
		if id == member {
			return true
		}
	}
	return !hasLead
}

// checkConfigRights refuses the change of the settings that need a lead to a
// member who is not one (see mayEnforce).
func (tx *Tx) checkConfigRights(gov GovernanceConfig, by string, keys []string) error {
	var restricted []string
	for _, k := range keys {
		if ChangeNeedsLead(k) {
			restricted = append(restricted, k)
		}
	}
	if len(restricted) == 0 {
		return nil
	}
	mf, err := tx.r.readMembersFile()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var members map[string]Member
	if mf != nil {
		members = mf.Members
	}
	if mayEnforce(gov, members, by) {
		return nil
	}
	return &ErrNotLead{Member: by, Keys: restricted}
}

// CanEnforce reports whether member may change the settings that bind the
// others in the config of the clone (a screen locks them when not).
func (r *Repo) CanEnforce(ctx context.Context, member string) bool {
	cfg, err := r.LoadConfig(ctx)
	if err != nil {
		return false
	}
	members, err := r.ListMembers(ctx)
	if err != nil && !os.IsNotExist(err) {
		return false
	}
	byID := make(map[string]Member, len(members))
	for _, m := range members {
		byID[m.ID] = m
	}
	return mayEnforce(cfg.Governance, byID, member)
}

// IsLead reports whether the member has the role lead in members.toml.
func (r *Repo) IsLead(ctx context.Context, memberID string) bool {
	m, err := r.GetMember(ctx, memberID)
	return err == nil && m != nil && m.Role == RoleLead
}

// Bounds of the numeric settings of config.toml, checked where the setting
// is changed (the TUI, oh team config set, UpdateConfig) and reported by oh
// doctor for what a file holds.
const (
	MinStaleDays          = 1
	MaxStaleDays          = 30
	MinDoneRetentionDays  = 1
	MaxDoneRetentionDays  = 90
	MaxInProgressLimit    = 100
	MaxSyncIntervalMinute = 60
	MinUnassignedIssues   = 1
	MaxUnassignedIssues   = 200
)

// NotifierTypes are the values of notification.type.
var NotifierTypes = []string{"mattermost", "slack", "discord", "teams"}

func validNotifierType(t string) bool {
	if t == "" {
		return true
	}
	for _, n := range NotifierTypes {
		if t == n {
			return true
		}
	}
	return false
}

func touches(keys []string, prefixes ...string) bool {
	for _, k := range keys {
		for _, p := range prefixes {
			if k == p || strings.HasPrefix(k, p+".") {
				return true
			}
		}
	}
	return false
}

func outOfRange(key string, v, lo, hi int) error {
	if v < lo || v > hi {
		return fmt.Errorf("%s: %s", key, i18n.Tf("v54c.config.bounds", lo, hi, v))
	}
	return nil
}

// ValidateTouched checks the settings keys names (the result of
// ConfigPatch.Keys): the lease settings, the board and the mappings of the
// tracker, the numeric bounds, the closed lists. What the change does not
// touch is not checked.
func (cfg *TeamConfig) ValidateTouched(keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	if touches(keys, "claim") {
		if err := cfg.Claim.Validate(); err != nil {
			return err
		}
	}
	if touches(keys, "board") {
		if err := cfg.Board.Validate(); err != nil {
			return err
		}
	}
	if touches(keys, "board", "tracker.status_mapping", "tracker.label_status_mapping") {
		if err := cfg.Tracker.ValidateMappings(cfg.Board); err != nil {
			return err
		}
	}
	checks := []struct {
		key    string
		v      int
		lo, hi int
	}{
		{"takeover.stale_days", cfg.Takeover.StaleDays, MinStaleDays, MaxStaleDays},
		{"claim.done_retention_days", cfg.Claim.DoneRetentionDays, MinDoneRetentionDays, MaxDoneRetentionDays},
		{"tracker.sync_interval_minutes", cfg.Tracker.SyncIntervalMinutes, 0, MaxSyncIntervalMinute},
		{"tracker.max_unassigned_issues", cfg.Tracker.MaxUnassignedIssues, MinUnassignedIssues, MaxUnassignedIssues},
	}
	for _, c := range checks {
		if touches(keys, c.key) {
			if err := outOfRange(c.key, c.v, c.lo, c.hi); err != nil {
				return err
			}
		}
	}
	if touches(keys, "tracker.max_in_progress_per_member") && cfg.Tracker.MaxInProgressPerMember != nil {
		if err := outOfRange("tracker.max_in_progress_per_member", *cfg.Tracker.MaxInProgressPerMember, 0, MaxInProgressLimit); err != nil {
			return err
		}
	}
	if touches(keys, "notification") {
		if !validNotifierType(cfg.Notification.Type) {
			return fmt.Errorf("notification.type: %s", i18n.Tf("v54c.config.one_of", strings.Join(NotifierTypes, ", "), cfg.Notification.Type))
		}
		for i, d := range cfg.Notification.Destinations {
			if !validNotifierType(d.Type) {
				return fmt.Errorf("notification.destinations[%d].type: %s", i, i18n.Tf("v54c.config.one_of", strings.Join(NotifierTypes, ", "), d.Type))
			}
		}
	}
	if touches(keys, "governance") {
		if p := cfg.Governance.Config; p != "" && p != GovernanceConfigLead && p != GovernanceConfigAnyMember {
			return fmt.Errorf("governance.config: %s", i18n.Tf("v54c.config.one_of", GovernanceConfigLead+", "+GovernanceConfigAnyMember, p))
		}
	}
	return nil
}
