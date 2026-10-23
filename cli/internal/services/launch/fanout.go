package launch

import (
	"errors"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// FanoutEnabled reports whether the fan-out is enabled on this machine.
func FanoutEnabled(cfg *config.Config) bool {
	return cfg != nil && cfg.Fanout.Experimental
}

// FanoutParentRecord is the fan-out record of a parent at launch.
func FanoutParentRecord(cfg *config.Config) *domain.SessionFanout {
	return &domain.SessionFanout{Role: domain.RoleParent, Experimental: true, Status: domain.FanoutWaiting,
		Parallel: cfg.Fanout.EffectiveParallel(), BudgetUSD: cfg.Fanout.EffectiveBudget()}
}

// CheckFanoutLaunch refuses a fan-out launch the configuration or the
// runtime does not allow: local or container (fanout-container), never
// remote (backlog: the children, the merges by oh and the verification on
// the integration branch run on the machine of the parent).
func CheckFanoutLaunch(cfg *config.Config, runtime sessionspec.RuntimeKind, perSession []string) error {
	switch {
	case !FanoutEnabled(cfg):
		return errors.New(i18n.T("cmd.fanout.disabled"))
	case runtime == sessionspec.RuntimeRemote:
		return errors.New(i18n.Tf("cmd.fanout.remote_refused", string(runtime)))
	case len(perSession) > 0:
		return errors.New(i18n.T("cmd.fanout.one_session"))
	}
	return nil
}
