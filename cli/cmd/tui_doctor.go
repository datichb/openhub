package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Doctor view repairs (modernisation tui-launch): the repairs of oh doctor
// --fix on the checks that have one (configuration restored from a
// backup, Beads zero impact, orphan servers), each planned, confirmed in
// the view, then applied like `oh doctor --fix --yes`; the clean-up checks
// open their section of the clean-up view. The offline mode is oh doctor --offline.

func init() {
	views.DoctorFixFor = doctorFixFor
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

// doctorFixFor is the repair of a check of the registry (nil: none).
func doctorFixFor(c views.DoctorCheck) *views.DoctorFix {
	switch {
	case c.Name == i18n.T("cmd.gc.doctor.name") && c.Warn:
		return &views.DoctorFix{Open: actionCleanup}
	case c.Name == i18n.T("cmd.migrate.cleanup.doctor_name") && !c.OK:
		return &views.DoctorFix{Open: actionCleanupDeploy}
	case c.OK:
		return nil
	case c.Name == i18n.T("cmd.doctor.check.config"):
		backup := latestConfigBackup()
		if backup == "" {
			return nil
		}
		return &views.DoctorFix{
			Plan: func(context.Context) (string, error) {
				return i18n.Tf("cmd.doctor.db.config_restore_plan", backup, config.ConfigPath()), nil
			},
			Apply: doctorFixRun(fixConfig),
		}
	case c.Name == i18n.T("cmd.doctor.check.beads"):
		return &views.DoctorFix{Plan: planBeadsFix, Apply: doctorFixRun(func(cmd *cobra.Command) error { return runDoctorFix(cmd, MustApp()) })}
	case c.Name == i18n.Tf("cmd.doctor.orphans.name", toolName()):
		return &views.DoctorFix{Plan: planOrphansFix, Apply: doctorFixRun(fixOrphanServers)}
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

// doctorFixRun runs a repair of oh doctor --fix as with --yes (the view
// confirmed it), its output as the report.
func doctorFixRun(fix func(cmd *cobra.Command) error) func(ctx context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		var out bytes.Buffer
		err := fix(doctorFixCommand(ctx, &out))
		return stripANSI(out.String()), err
	}
}

func doctorFixCommand(ctx context.Context, w io.Writer) *cobra.Command {
	c := &cobra.Command{Use: "doctor"}
	c.Flags().BoolP("yes", "y", false, "")
	_ = c.Flags().Set("yes", "true")
	c.SetOut(w)
	c.SetErr(w)
	c.SetContext(ctx)
	return c
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

// stripANSI removes the terminal styles of a CLI output (shown in a
// window of the TUI).
func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }
