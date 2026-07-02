package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

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
		{"Compatibilité oh ↔ opencode", checkCompatibility},
		{"Configuration hub.toml", checkConfig},
		{"Base de données", checkDatabase},
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
		fmt.Fprintln(a.IO.Out, common.SuccessStyle.Render("Toutes les vérifications sont passées."))
	} else {
		fmt.Fprintln(a.IO.Out, common.WarningStyle.Render("Certaines vérifications ont échoué."))
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
			return "non trouvé dans PATH", false
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
		return "non chargée", false
	}
	return fmt.Sprintf("langue=%s, opencode=%s", a.Config.CLI.Language, a.Config.Opencode.Version), true
}

func checkDatabase() (string, bool) {
	a := TryApp()
	if a == nil || a.Projects == nil {
		return "non connectée", false
	}
	projects, err := a.Projects.List("")
	if err != nil {
		return fmt.Sprintf("erreur: %v", err), false
	}
	return fmt.Sprintf("OK (%d projets)", len(projects)), true
}

func checkCompatibility() (string, bool) {
	ocVersion, err := opencode.Version()
	if err != nil {
		return "opencode non disponible", false
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
