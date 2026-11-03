package cmd

import (
	"context"
	"strings"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
)

// Doctor › the former max_auto_plan_per_member of the team. It bounded the
// claims a sync created per member; the setting is now the limit of tickets a
// member has in progress (max_in_progress_per_member). oh still reads the
// former key when the new one is absent. A warning with the command that
// writes the new key (and drops the old one) for each team clone whose
// config.toml still sets it; nothing otherwise. The hub.toml of the member is
// reported by the former keys check (legacyConfigCheck).

const doctorIDTrackerLimit = "tracker_wip_key"

func init() { registerDoctorCheck(trackerLimitDoctorChecks) }

func trackerLimitDoctorChecks(ctx context.Context) []doctor.Result {
	a := TryApp()
	if a == nil || a.Config == nil {
		return nil
	}
	return trackerLimitResults(ctx, a)
}

func trackerLimitResults(ctx context.Context, a *app.App) []doctor.Result {
	var out []doctor.Result
	for _, t := range claimTeams(ctx, a, a.Config) {
		cfg, err := t.repo.LoadConfig(ctx)
		if err != nil || cfg == nil || !cfg.Tracker.UsesFormerInProgressKey() {
			continue
		}
		out = append(out, doctor.Result{
			ID: doctorIDTrackerLimit, Name: i18n.T("cmd.doctor.tracker_wip_key.name"), OK: true, Warn: true,
			Detail: i18n.Tf("cmd.doctor.tracker_wip_key.found", cfg.Tracker.MaxAutoPlanPerMember, t.repo.Path(), cfg.Tracker.InProgressLimit()),
		})
	}
	return out
}

// formerAutoPlanHint is the sentence the former keys check adds when
// hub.toml still sets max_auto_plan_per_member: the key changed meaning, so
// the command names the new key.
func formerAutoPlanHint(found []string) string {
	for _, f := range found {
		if strings.Contains(f, "tracker.max_auto_plan_per_member") {
			return i18n.T("cmd.doctor.legacy.in_progress_hint")
		}
	}
	return ""
}
