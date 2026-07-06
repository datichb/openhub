package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Vérifie l'état du système et les dépendances",
	RunE:  runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

type check struct {
	name string
	test func() (string, bool)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	a := MustApp()

	fmt.Fprintln(a.IO.Out, common.Title.Render("  oh doctor  "))
	fmt.Fprintln(a.IO.Out)

	checks := []check{
		{"OS / Architecture", checkOS},
		{"Go runtime", checkGoRuntime},
		{"git", checkBinary("git")},
		{"opencode", checkBinary("opencode")},
		{"bd (ticket tracker)", checkOptionalBinary("bd", "brew install datichb/tap/bd")},
		{"fzf (fuzzy finder)", checkOptionalBinary("fzf", "brew install fzf")},
		{"Compatibilité oh ↔ opencode", checkCompatibility},
		{"Configuration hub.toml", checkConfig},
		{"Base de données", checkDatabase},
		{"Clés API (keychain)", checkAPIKeys},
	}

	allPassed := true
	for _, c := range checks {
		detail, ok := c.test()
		if ok {
			fmt.Fprintf(a.IO.Out, "  %s %s — %s\n",
				common.SuccessStyle.Render(common.IconSuccess),
				c.name, detail)
		} else {
			fmt.Fprintf(a.IO.Out, "  %s %s — %s\n",
				common.ErrorStyle.Render(common.IconError),
				c.name, detail)
			allPassed = false
		}
	}

	fmt.Fprintln(a.IO.Out)
	if allPassed {
		fmt.Fprintln(a.IO.Out, common.SuccessStyle.Render(i18n.T("cmd.doctor.all_passed")))
	} else {
		fmt.Fprintln(a.IO.Out, common.WarningStyle.Render(i18n.T("cmd.doctor.some_failed")))
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

func checkConfig() (string, bool) {
	a := TryApp()
	if a == nil || a.Config == nil {
		return i18n.T("cmd.doctor.config_not_loaded"), false
	}
	return fmt.Sprintf("langue=%s, opencode=%s", a.Config.CLI.Language, a.Config.Opencode.Version), true
}

func checkDatabase() (string, bool) {
	a := TryApp()
	if a == nil || a.Projects == nil {
		return i18n.T("cmd.doctor.db_not_connected"), false
	}
	ctx := context.Background()
	projects, err := a.Projects.List(ctx, "")
	if err != nil {
		return fmt.Sprintf("erreur: %v", err), false
	}
	return i18n.Tf("cmd.doctor.db_ok", len(projects)), true
}

func checkCompatibility() (string, bool) {
	ocVersion, err := opencode.Version()
	if err != nil {
		return i18n.T("cmd.doctor.opencode_unavailable"), false
	}
	result := opencode.CheckCompatibility(Version, ocVersion)
	if result.Compatible {
		return fmt.Sprintf("oh %s ↔ opencode %s — OK", Version, ocVersion), true
	}
	return result.Warning, false
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
func checkAPIKeys() (string, bool) {
	a := TryApp()
	if a == nil {
		return "app non disponible", false
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
		{"gslides", a.Config.MCP.Gslides.Enabled, "GOOGLE_ACCESS_TOKEN", a.Config.MCP.Gslides.Token},
	}

	var configured, found, missing int
	for _, k := range keys {
		if !k.enabled {
			continue
		}
		configured++

		// Check env var first
		if k.envVar != "" && os.Getenv(k.envVar) != "" {
			found++
			continue
		}

		// Check keychain
		if k.keyName != "" && a.Secrets != nil {
			if token, _ := a.Secrets.Get(context.Background(), k.keyName); token != "" {
				found++
				continue
			}
		}

		missing++
	}

	if configured == 0 {
		return i18n.T("cmd.doctor.no_mcp_enabled"), true
	}

	if missing > 0 {
		return i18n.Tf("cmd.doctor.tokens_missing", found, configured, missing), false
	}

	return i18n.Tf("cmd.doctor.tokens_ok", found, configured), true
}
