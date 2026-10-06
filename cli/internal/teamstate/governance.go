package teamstate

import (
	"errors"
	"fmt"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// Publication policies of `[governance] publish` (D7). Only any_member is
// implemented; finer rules are BL-2.
const (
	GovernancePublishAnyMember = "any_member"
)

// GovernanceConfig is the `[governance]` section of the team config.toml.
type GovernanceConfig struct {
	// Publish says who may publish workflows; empty = any_member.
	Publish string `toml:"publish,omitempty"`
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
func (r *Repo) CheckPublish(memberID string) error {
	cfg, err := r.LoadConfig()
	if err != nil {
		return err
	}
	return cfg.Governance.CanPublish(memberID, r.HasMember)
}
