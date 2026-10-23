package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/buildinfo"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/selfupdate"
	"github.com/datichb/openhub/cli/internal/services/doctor"
	"github.com/datichb/openhub/cli/internal/tui/progress"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Vérifie l'état du système et les dépendances",
	RunE:  runDoctor,
}

func init() {
	rootCmd.AddCommand(withInitLevel(doctorCmd, levelDegraded))
	doctorCmd.Flags().Bool("fix", false, "Répare ce qui peut l'être (Beads zéro impact) après un résumé et une confirmation")
	doctorCmd.Flags().BoolP("yes", "y", false, "Avec --fix : appliquer sans confirmation (obligatoire sans terminal)")
	doctorCmd.Flags().Bool("offline", false, "Aucun appel réseau (version publiée, validation de la clé du provider) ; aussi OH_DOCTOR_OFFLINE=1")
}

// doctorOffline reports whether oh doctor must not use the network
// (--offline, OH_DOCTOR_OFFLINE).
var doctorOffline = func() bool {
	v := os.Getenv("OH_DOCTOR_OFFLINE")
	return v != "" && v != "0" && v != "false"
}

// Identifiers of the results that have a repair (their check is the runtime
// one, which gathers the registered checks).
const (
	doctorIDGC            = "gc"
	doctorIDDeployCleanup = "deploy-cleanup"
	doctorIDOrphans       = "orphans"
)

// single adapts a check that returns a detail and a pass/fail flag.
func single(fn func() (string, bool)) func(context.Context) []doctor.Result {
	return func(context.Context) []doctor.Result {
		detail, ok := fn()
		return []doctor.Result{{Detail: detail, OK: ok}}
	}
}

// one adapts a check that returns a full result (warnings).
func one(fn func() doctor.Result) func(context.Context) []doctor.Result {
	return func(context.Context) []doctor.Result { return []doctor.Result{fn()} }
}

// many adapts a check that returns several results.
func many(fn func() []doctor.Result) func(context.Context) []doctor.Result {
	return func(context.Context) []doctor.Result { return fn() }
}

// doctorRegistry lists every check of oh doctor, in display order. It is the
// single source of `oh doctor` and of the TUI Doctor view (A40). The checks
// of the machine (os, go, git, bd) live in services/doctor; the others still
// read the state of cmd (configuration, database, keychain, projects).
func doctorRegistry() []doctor.Check {
	return []doctor.Check{
		doctor.OS(),
		doctor.GoRuntime(),
		doctor.Binary("git", "git"),
		doctor.OptionalBinary("bd", "bd", "bd (beads)", "brew install datichb/tap/bd"),
		{ID: "version", Name: i18n.T("cmd.doctor.check.version"), Run: one(checkOhUpdate)},
		{ID: "config", Name: i18n.T("cmd.doctor.check.config"), Run: single(checkConfig), Fix: configFix},
		{ID: "setup", Name: i18n.T("cmd.doctor.check.setup"), Run: setupDoctorChecks},
		{ID: doctorIDTrackerTokens, Name: i18n.T("cmd.doctor.check.tracker_tokens"), Run: trackerTokensChecks},
		{ID: doctorIDTrackerConnection, Name: i18n.T("cmd.doctor.check.tracker_connection"), Run: trackerConnectionChecks},
		{ID: doctorIDTrackerUsername, Name: i18n.T("cmd.doctor.check.tracker_username"), Run: trackerUsernameChecks},
		{ID: doctorIDTrackerIdentity, Name: i18n.T("cmd.doctor.check.tracker_identity"), Run: trackerIdentityChecks},
		{ID: "credentials", Name: i18n.T("cmd.doctor.check.credentials"), Run: one(checkProviderCredentials)},
		{ID: "database", Name: i18n.T("cmd.doctor.check.database"), Run: one(checkDatabase)},
		{ID: "api_keys", Name: i18n.T("cmd.doctor.check.api_keys"), Run: single(checkAPIKeys)},
		{ID: "beads", Name: i18n.T("cmd.doctor.check.beads"), Run: many(checkBeadsSanity), Fix: beadsFix},
		{ID: "beads_tracking", Name: i18n.T("cmd.doctor.check.beads_tracking"), Run: many(beadsTrackingChecks)},
		{ID: "runtime", Name: i18n.T("cmd.doctor.check.runtime"), Run: v5DoctorChecks, Fix: runtimeFix},
	}
}

// collectDoctorChecks runs the whole registry (TUI Doctor view), the repairs
// attached to the results.
func collectDoctorChecks() []doctor.Result {
	reg, err := doctor.NewRegistry(doctorRegistry()...)
	if err != nil {
		panic(err) // a duplicated identifier: caught by TestDoctorRegistryIDs
	}
	return reg.Run(context.Background())
}

func runDoctor(cmd *cobra.Command, args []string) error {
	a := MustApp()

	fmt.Fprintln(a.IO.Out, theme.Title.Render("  oh doctor  "))
	fmt.Fprintln(a.IO.Out)
	printStartupHints(a.IO.Out)
	if off, _ := cmd.Flags().GetBool("offline"); off {
		_ = os.Setenv("OH_DOCTOR_OFFLINE", "1")
	}
	if fix, _ := cmd.Flags().GetBool("fix"); fix {
		if err := fixConfig(cmd); err != nil {
			return err
		}
		if err := runDoctorFix(cmd, a); err != nil {
			return err
		}
		if err := fixClaimsProject(cmd); err != nil {
			return err
		}
		if err := fixOrphanServers(cmd); err != nil {
			return err
		}
	}
	return runDoctorChecks(a.IO.Out, doctorRegistry())
}

// runDoctorChecks prints each check; a failed mandatory check makes oh exit
// with ExitFailure. Warnings, including the failures of optional checks
// (container engine, bd…), are reported as passed checks (U-05).
func runDoctorChecks(w io.Writer, checks []doctor.Check) error {
	allPassed := true
	for _, c := range checks {
		s := progress.NewSpinner(c.Name)
		s.Start()
		res := c.Results(context.Background())
		s.Stop()

		for _, r := range res {
			icon := theme.SuccessStyle.Render(theme.IconSuccess)
			switch {
			case !r.OK:
				icon = theme.ErrorStyle.Render(theme.IconError)
				allPassed = false
			case r.Warn:
				icon = theme.WarningStyle.Render(theme.IconWarning)
			}
			fmt.Fprintf(w, "  %s %s — %s\n", icon, r.Name, strings.ReplaceAll(r.Detail, "\n", "\n      "))
		}
	}

	fmt.Fprintln(w)
	if allPassed {
		fmt.Fprintln(w, theme.SuccessStyle.Render(i18n.T("cmd.doctor.all_passed")))
	} else {
		fmt.Fprintln(w, theme.WarningStyle.Render(i18n.T("cmd.doctor.some_failed")))
		return &ExitError{Code: ExitFailure}
	}
	return nil
}

// checkAPIKeys validates that configured MCP tokens are accessible.
// Reports per-service status so the user knows exactly which token is found,
// where it was found (env / keychain), and which is missing.
func checkAPIKeys() (string, bool) {
	a := TryApp()
	if a == nil {
		return i18n.T("cmd.doctor.app_unavailable"), false
	}
	return mcpTokensReport(a.Config, a.Secrets)
}

// checkOhUpdate compares the running oh with the latest published release:
// a newer release is a warning (the check passes), never an older one (A1).
func checkOhUpdate() doctor.Result {
	current := buildinfo.Version
	if compareOhVersion(current, "0.0.0") == versionUnknown {
		return doctor.Result{OK: true, Detail: i18n.Tf("cmd.doctor.version.dev", current)}
	}
	if doctorOffline() {
		return doctor.Result{OK: true, Detail: i18n.Tf("cmd.doctor.version.offline", current)}
	}
	release, err := selfupdate.LatestRelease()
	if err != nil {
		// Network failure is non-fatal — don't block doctor
		return doctor.Result{OK: true, Warn: true, Detail: i18n.Tf("cmd.doctor.version.unreachable", current, err)}
	}
	return ohUpdateResult(current, release.Version())
}

func ohUpdateResult(current, latest string) doctor.Result {
	switch compareOhVersion(current, latest) {
	case versionSame:
		return doctor.Result{OK: true, Detail: i18n.Tf("cmd.doctor.version.up_to_date", current)}
	case versionAhead:
		return doctor.Result{OK: true, Detail: i18n.Tf("cmd.doctor.version.ahead", current, latest)}
	case versionAvailable:
		return doctor.Result{OK: true, Warn: true, Detail: i18n.Tf("cmd.doctor.version.available", current, latest)}
	default:
		return doctor.Result{OK: true, Detail: i18n.Tf("cmd.doctor.version.dev", current)}
	}
}

// checkBeadsSanity verifies that beads-initialized projects have no side effects
// (hooks, .gitignore entries, agent files, missing exclude entries) that
// should have been prevented. Projects without .beads/ are silently skipped
// (pass). A project sharing .beads/ through git whose .gitignore does not
// cover /.beads.gate.lock is a warning: the entry is for the team to add.
func checkBeadsSanity() []doctor.Result {
	a := TryApp()
	if a == nil || a.Projects == nil {
		return []doctor.Result{{OK: true, Detail: i18n.T("cmd.doctor.beads.no_project")}}
	}

	ctx := context.Background()
	projects, err := a.Projects.List(ctx, "")
	if err != nil {
		return []doctor.Result{{OK: true, Detail: i18n.T("cmd.doctor.beads.list_failed")}}
	}
	return beadsSanityResults(projects)
}

// beadsSanityResults checks the Beads projects among projects.
func beadsSanityResults(projects []domain.Project) []doctor.Result {
	var totalIssues, checkedCount int
	var details []string
	var warnings []doctor.Result

	for _, p := range projects {
		if p.Path == "" || !beads.IsInitialized(p.Path) {
			continue
		}
		checkedCount++
		issues := beads.DiagnoseBeadsImpact(p.Path)
		totalIssues += len(issues)
		for _, issue := range issues {
			details = append(details, fmt.Sprintf("[%s] %s: %s", p.Name, issue.Kind, issue.Detail))
		}
		if beads.SharedGateLockUnignored(p.Path) {
			warnings = append(warnings, doctor.Result{OK: true, Warn: true, Detail: i18n.Tf("cmd.doctor.beads.shared_gate_lock", p.Name)})
		}
	}

	var res doctor.Result
	switch {
	case totalIssues > 0:
		summary := i18n.Tf("cmd.doctor.beads.issues", totalIssues)
		if len(details) <= 3 {
			summary += ": " + strings.Join(details, "; ")
		} else {
			summary += ": " + strings.Join(details[:3], "; ") + " " + i18n.Tf("cmd.doctor.beads.more", len(details)-3)
		}
		res = doctor.Result{Detail: summary + " — " + i18n.T("cmd.doctor.beads.fix_hint")}
	case checkedCount == 0:
		res = doctor.Result{OK: true, Detail: i18n.T("cmd.doctor.beads.none")}
	default:
		res = doctor.Result{OK: true, Detail: i18n.Tf("cmd.doctor.beads.ok", checkedCount)}
	}
	return append([]doctor.Result{res}, warnings...)
}
