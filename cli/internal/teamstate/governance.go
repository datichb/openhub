package teamstate

import (
	"context"
	"errors"
	"fmt"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// Publication policies of `[governance] publish`. Only any_member is
// implemented; finer rules are not implemented.
const (
	GovernancePublishAnyMember = "any_member"
)

// Policies of `[governance] config`: who may change the settings that bind
// the other members (the *_enforced settings, [limits.enforced] and
// [governance] itself).
const (
	// GovernanceConfigLead reserves them to the members of role lead.
	GovernanceConfigLead = "lead"
	// GovernanceConfigAnyMember lets every member change them.
	GovernanceConfigAnyMember = "any_member"
	// RoleLead is the role of the members who may change them.
	RoleLead = "lead"
)

// GovernanceConfig is the `[governance]` section of the team config.toml.
type GovernanceConfig struct {
	// Publish says who may publish workflows; empty = any_member.
	Publish string `toml:"publish,omitempty"`
	// Config says who may change the enforced settings; empty = lead.
	Config string `toml:"config,omitempty"`
}

// ConfigPolicy returns the effective policy of the enforced settings.
func (g GovernanceConfig) ConfigPolicy() string {
	if g.Config == "" {
		return GovernanceConfigLead
	}
	return g.Config
}

// DefaultGovernance is the governance written by `oh team init`.
func DefaultGovernance() GovernanceConfig {
	return GovernanceConfig{Publish: GovernancePublishAnyMember}
}

// PublishPolicy returns the effective publication policy.
func (g GovernanceConfig) PublishPolicy() string {
	if g.Publish == "" {
		return GovernancePublishAnyMember
	}
	return g.Publish
}

var (
	// ErrGovernanceUnsupported is returned when config.toml names a
	// publication policy this version of oh does not implement: publishing
	// is refused rather than allowed.
	ErrGovernanceUnsupported = errors.New("unsupported governance policy")
	// ErrNotMember is returned when the publisher is not in members.toml.
	ErrNotMember = errors.New("not a team member")
)

// CanPublish checks that memberID may publish workflows under g.
func (g GovernanceConfig) CanPublish(memberID string, isMember func(string) bool) error {
	switch g.PublishPolicy() {
	case GovernancePublishAnyMember:
		if memberID == "" || isMember == nil || !isMember(memberID) {
			return fmt.Errorf("%w: %s", ErrNotMember, i18n.Tf("teamstate.workflow.governance.not_member", memberID))
		}
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrGovernanceUnsupported, i18n.Tf("teamstate.workflow.governance.unsupported", g.Publish))
	}
}

// CheckPublish reads the team governance and checks that memberID may
// publish workflows.
func (r *Repo) CheckPublish(ctx context.Context, memberID string) error {
	cfg, err := r.LoadConfig(ctx)
	if err != nil {
		return err
	}
	return cfg.Governance.CanPublish(memberID, func(id string) bool { return r.HasMember(ctx, id) })
}
