package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Doctor view repairs (modernisation tui-launch): the repairs of oh doctor
// --fix on the checks that have one (configuration restored from a
// backup, Beads zero impact, orphan servers), each planned, confirmed in
// the view, then applied like `oh doctor --fix --yes`; the clean-up checks
// open their section of the clean-up view. The offline mode is oh doctor --offline.

func init() {
	views.DoctorOffline = &views.DoctorOfflineSwitch{
		On: func() bool { return doctorOffline() },
		Set: func(on bool) {
			if on {
				_ = os.Setenv("OH_DOCTOR_OFFLINE", "1")
			} else {
				_ = os.Unsetenv("OH_DOCTOR_OFFLINE")
			}
		},
	}
}

// The repairs of the registry (Check.Fix): each one is planned, confirmed in
// the view, then applied as `oh doctor --fix --yes` does, its output as the
// report; the clean-up results open their section of the clean-up view.

// configFix restores the latest backup of a hub.toml that does not load.
func configFix(r doctor.Result) *doctor.Fix {
	if r.OK {
		return nil
	}
	backup := latestConfigBackup()
	if backup == "" {
		return nil
	}
	return &doctor.Fix{
		Plan: func(context.Context) (string, error) {
			return i18n.Tf("cmd.doctor.db.config_restore_plan", backup, config.ConfigPath()), nil
		},
		Apply: doctorFixRun(func(_ context.Context, w io.Writer) error { return repairConfig(w, alwaysConfirm) }),
	}
}

// beadsFix applies the zero-impact clean-up of the Beads projects.
func beadsFix(r doctor.Result) *doctor.Fix {
	if r.OK {
		return nil
	}
	return &doctor.Fix{Plan: planBeadsFix, Apply: doctorFixRun(func(ctx context.Context, w io.Writer) error {
		return repairBeads(ctx, w, MustApp(), true, false, alwaysConfirm)
	})}
}

// runtimeFix is the repair of the results gathered by the runtime check.
func runtimeFix(r doctor.Result) *doctor.Fix {
	switch {
	case r.ID == doctorIDClaimsProject:
		return claimsProjectFix(r)
	case r.ID == doctorIDClaimsQueue:
		return claimsQueueFix(r)
	case r.ID == doctorIDGC && r.Warn:
		return &doctor.Fix{Open: actionCleanup}
	case r.ID == doctorIDDeployCleanup && !r.OK:
		return &doctor.Fix{Open: actionCleanupDeploy}
	case r.ID == doctorIDOrphans && !r.OK:
		return &doctor.Fix{Plan: planOrphansFix, Apply: doctorFixRun(func(ctx context.Context, w io.Writer) error {
			return stopOrphanServers(ctx, w, alwaysConfirm)
		})}
	}
	return nil
}

func planBeadsFix(ctx context.Context) (string, error) {
	plans, err := beadsFixPlans(ctx, MustApp(), true)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	for _, p := range plans {
		b.WriteString(p.project.Name + " (" + p.project.Path + ")\n")
		printCleanActions(&b, p.report)
	}
	if b.Len() == 0 {
		return i18n.T("cmd.doctor.fix.nothing"), nil
	}
	return stripANSI(b.String()), nil
}

func planOrphansFix(ctx context.Context) (string, error) {
	orphans, _, err := findOrphanServers(ctx)
	if err != nil {
		return "", err
	}
	lines := make([]string, 0, len(orphans)+1)
	lines = append(lines, i18n.Tf("cmd.doctor.orphans.plan", toolName()))
	for _, o := range orphans {
		lines = append(lines, "  "+describeOrphans([]orphanServer{o}, 1))
	}
	return strings.Join(lines, "\n"), nil
}

// doctorFixRun runs a repair of oh doctor --fix (the view confirmed it), its
// output as the report.
func doctorFixRun(fix func(ctx context.Context, w io.Writer) error) func(ctx context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		var out bytes.Buffer
		err := fix(ctx, &out)
		return stripANSI(out.String()), err
	}
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

// stripANSI removes the terminal styles of a CLI output (shown in a
// window of the TUI).
func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }
