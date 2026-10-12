package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/buildinfo"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/selfupdate"
	"github.com/datichb/openhub/cli/internal/tui/progress"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
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

// check is one entry of the doctor registry: the label shown while it runs
// and the function that returns its result(s). A result without a name takes
// the label of its check.
type check struct {
	name string
	run  func() []views.DoctorCheck
}

// single adapts a check that returns a detail and a pass/fail flag.
func single(fn func() (string, bool)) func() []views.DoctorCheck {
	return func() []views.DoctorCheck {
		detail, ok := fn()
		return []views.DoctorCheck{{Detail: detail, OK: ok}}
	}
}

// one adapts a check that returns a full result (warnings).
func one(fn func() views.DoctorCheck) func() []views.DoctorCheck {
	return func() []views.DoctorCheck { return []views.DoctorCheck{fn()} }
}

// doctorRegistry lists every check of oh doctor, in display order. It is the
// single source of `oh doctor` and of the TUI Doctor view (A40).
func doctorRegistry() []check {
	return []check{
		{i18n.T("cmd.doctor.check.os"), single(checkOS)},
		{i18n.T("cmd.doctor.check.go"), single(checkGoRuntime)},
		{"git", single(checkBinary("git"))},
		{"bd (beads)", single(checkOptionalBinary("bd", "brew install datichb/tap/bd"))},
		{i18n.T("cmd.doctor.check.version"), one(checkOhUpdate)},
		{i18n.T("cmd.doctor.check.config"), single(checkConfig)},
		{i18n.T("cmd.doctor.check.credentials"), one(checkProviderCredentials)},
		{i18n.T("cmd.doctor.check.database"), one(checkDatabase)},
		{i18n.T("cmd.doctor.check.api_keys"), single(checkAPIKeys)},
		{i18n.T("cmd.doctor.check.beads"), single(checkBeadsSanity)},
		{i18n.T("cmd.doctor.check.beads_tracking"), beadsTrackingChecks},
		{i18n.T("cmd.doctor.check.runtime"), v5DoctorChecks},
	}
}

// collectDoctorChecks runs the whole registry (TUI Doctor view).
func collectDoctorChecks() []views.DoctorCheck {
	var out []views.DoctorCheck
	for _, c := range doctorRegistry() {
		out = append(out, c.results()...)
	}
	return out
}

func (c check) results() []views.DoctorCheck {
	res := c.run()
	for i := range res {
		if res[i].Name == "" {
			res[i].Name = c.name
		}
	}
	return res
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
		if err := fixOrphanServers(cmd); err != nil {
			return err
		}
	}
	return runDoctorChecks(a.IO.Out, doctorRegistry())
}

// runDoctorChecks prints each check; a failed check makes oh exit with 1
// (warnings are reported as passed checks).
func runDoctorChecks(w io.Writer, checks []check) error {
	allPassed := true
	for _, c := range checks {
		s := progress.NewSpinner(c.name)
		s.Start()
		res := c.results()
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
			fmt.Fprintf(w, "  %s %s — %s\n", icon, r.Name, r.Detail)
		}
	}

	fmt.Fprintln(w)
	if allPassed {
		fmt.Fprintln(w, theme.SuccessStyle.Render(i18n.T("cmd.doctor.all_passed")))
	} else {
		fmt.Fprintln(w, theme.WarningStyle.Render(i18n.T("cmd.doctor.some_failed")))
		return &ExitError{Code: 1}
	}
	return nil
}

func checkOS() (string, bool) {
	return fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH), true
}

func checkGoRuntime() (string, bool) {
	return runtime.Version(), true
}

func checkBinary(name string) func() (string, bool) {
	return func() (string, bool) {
		path, err := exec.LookPath(name)
		if err != nil {
			return i18n.T("cmd.doctor.not_found"), false
		}
		// Try to get version
		out, err := exec.Command(path, "--version").Output()
		if err != nil {
			return path, true
		}
		// First line only
		version := string(out)
		if i := indexOf(version, '\n'); i > 0 {
			version = version[:i]
		}
		return version, true
	}
}

func indexOf(s string, c byte) int {
	for i := range s {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// checkOptionalBinary returns a check for a binary that is recommended but not required.
// If not found, it returns true (pass) with an install hint, since it's optional.
func checkOptionalBinary(name, installHint string) func() (string, bool) {
	return func() (string, bool) {
		path, err := exec.LookPath(name)
		if err != nil {
			return i18n.Tf("cmd.doctor.optional_missing", installHint), true
		}
		out, err := exec.Command(path, "--version").Output()
		if err != nil {
			return path, true
		}
		version := string(out)
		if i := indexOf(version, '\n'); i > 0 {
			version = version[:i]
		}
		return version, true
	}
}

// checkAPIKeys validates that configured MCP tokens are accessible.
// Reports per-service status so the user knows exactly which token is found,
// where it was found (env / keychain), and which is missing.
func checkAPIKeys() (string, bool) {
	a := TryApp()
	if a == nil {
		return i18n.T("cmd.doctor.app_unavailable"), false
	}

	type keyCheck struct {
		service string
		enabled bool
		envVar  string
		keyName string
	}

	keys := []keyCheck{
		{"figma", a.Config.MCP.Figma.Enabled, "FIGMA_TOKEN", a.Config.MCP.Figma.Token},
		{"gitlab", a.Config.MCP.Gitlab.Enabled, "GITLAB_TOKEN", a.Config.MCP.Gitlab.Token},
		{"jira", a.Config.MCP.Jira.Enabled, "JIRA_TOKEN", a.Config.MCP.Jira.Token},
		{"gslides", a.Config.MCP.Gslides.Enabled, "GOOGLE_ACCESS_TOKEN", a.Config.MCP.Gslides.Token},
	}

	var configured int
	var foundDetails []string
	var missingNames []string

	for _, k := range keys {
		if !k.enabled {
			continue
		}
		configured++

		// Check env var first
		if k.envVar != "" && os.Getenv(k.envVar) != "" {
			foundDetails = append(foundDetails, fmt.Sprintf("%s (env)", k.service))
			continue
		}

		// Check keychain
		if k.keyName != "" && a.Secrets != nil {
			if token, _ := a.Secrets.Get(context.Background(), k.keyName); token != "" {
				foundDetails = append(foundDetails, fmt.Sprintf("%s (keychain)", k.service))
				continue
			}
		}

		missingNames = append(missingNames, k.service)
	}

	if configured == 0 {
		return i18n.T("cmd.doctor.no_mcp_enabled"), true
	}

	found := len(foundDetails)

	if len(missingNames) > 0 {
		return i18n.Tf("cmd.doctor.tokens_missing", found, configured, strings.Join(missingNames, ", ")), false
	}

	return i18n.Tf("cmd.doctor.tokens_ok", found, configured, strings.Join(foundDetails, ", ")), true
}

// checkOhUpdate compares the running oh with the latest published release:
// a newer release is a warning (the check passes), never an older one (A1).
func checkOhUpdate() views.DoctorCheck {
	current := buildinfo.Version
	if compareOhVersion(current, "0.0.0") == versionUnknown {
		return views.DoctorCheck{OK: true, Detail: i18n.Tf("cmd.doctor.version.dev", current)}
	}
	if doctorOffline() {
		return views.DoctorCheck{OK: true, Detail: i18n.Tf("cmd.doctor.version.offline", current)}
	}
	release, err := selfupdate.LatestRelease()
	if err != nil {
		// Network failure is non-fatal — don't block doctor
		return views.DoctorCheck{OK: true, Warn: true, Detail: i18n.Tf("cmd.doctor.version.unreachable", current, err)}
	}
	return ohUpdateResult(current, release.Version())
}

func ohUpdateResult(current, latest string) views.DoctorCheck {
	switch compareOhVersion(current, latest) {
	case versionSame:
		return views.DoctorCheck{OK: true, Detail: i18n.Tf("cmd.doctor.version.up_to_date", current)}
	case versionAhead:
		return views.DoctorCheck{OK: true, Detail: i18n.Tf("cmd.doctor.version.ahead", current, latest)}
	case versionAvailable:
		return views.DoctorCheck{OK: true, Warn: true, Detail: i18n.Tf("cmd.doctor.version.available", current, latest)}
	default:
		return views.DoctorCheck{OK: true, Detail: i18n.Tf("cmd.doctor.version.dev", current)}
	}
}

// checkBeadsSanity verifies that beads-initialized projects have no side effects
// (hooks, .gitignore entries, agent files) that should have been prevented.
// Projects without .beads/ are silently skipped (pass).
func checkBeadsSanity() (string, bool) {
	a := TryApp()
	if a == nil || a.Projects == nil {
		return i18n.T("cmd.doctor.beads.no_project"), true
	}

	ctx := context.Background()
	projects, err := a.Projects.List(ctx, "")
	if err != nil {
		return i18n.T("cmd.doctor.beads.list_failed"), true
	}

	var totalIssues int
	var details []string

	for _, p := range projects {
		if p.Path == "" {
			continue
		}
		if !beads.IsInitialized(p.Path) {
			continue
		}
		issues := beads.DiagnoseBeadsImpact(p.Path)
		if len(issues) > 0 {
			totalIssues += len(issues)
			for _, issue := range issues {
				details = append(details, fmt.Sprintf("[%s] %s: %s", p.Name, issue.Kind, issue.Detail))
			}
		}
	}

	if totalIssues == 0 {
		checkedCount := 0
		for _, p := range projects {
			if p.Path != "" && beads.IsInitialized(p.Path) {
				checkedCount++
			}
		}
		if checkedCount == 0 {
			return i18n.T("cmd.doctor.beads.none"), true
		}
		return i18n.Tf("cmd.doctor.beads.ok", checkedCount), true
	}

	summary := i18n.Tf("cmd.doctor.beads.issues", totalIssues)
	if len(details) <= 3 {
		summary += ": " + strings.Join(details, "; ")
	} else {
		summary += ": " + strings.Join(details[:3], "; ") + " " + i18n.Tf("cmd.doctor.beads.more", len(details)-3)
	}
	return summary + " — " + i18n.T("cmd.doctor.beads.fix_hint"), false
}
