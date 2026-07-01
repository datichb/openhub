package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Affiche l'état du hub et du projet courant",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	a := GetApp()

	fmt.Fprintln(a.IO.Out, common.Title.Render("  oh status  "))
	fmt.Fprintln(a.IO.Out)

	// Hub info
	fmt.Fprintln(a.IO.Out, common.Bold.Render("Hub"))
	w := tabwriter.NewWriter(a.IO.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  Config:\t%s\n", config.ConfigPath())
	fmt.Fprintf(w, "  Langue:\t%s\n", a.Config.CLI.Language)
	fmt.Fprintf(w, "  Opencode:\t%s (%s)\n", a.Config.Opencode.Version, a.Config.Opencode.Channel)
	w.Flush()
	fmt.Fprintln(a.IO.Out)

	// Projects summary
	projects, err := a.Projects.List("")
	if err != nil {
		return err
	}

	active := 0
	for _, p := range projects {
		if p.Status == domain.ProjectStatusActive {
			active++
		}
	}

	fmt.Fprintln(a.IO.Out, common.Bold.Render("Projets"))
	fmt.Fprintf(a.IO.Out, "  Total: %d (actifs: %d)\n", len(projects), active)
	fmt.Fprintln(a.IO.Out)

	// Current directory project detection
	cwd, _ := os.Getwd()
	var currentProject *domain.Project
	for i, p := range projects {
		absPath, _ := filepath.Abs(p.Path)
		if absPath == cwd || isSubPath(cwd, absPath) {
			currentProject = &projects[i]
			break
		}
	}

	if currentProject != nil {
		fmt.Fprintln(a.IO.Out, common.Bold.Render("Projet courant"))
		fmt.Fprintf(a.IO.Out, "  %s %s (%s)\n",
			common.SuccessStyle.Render(common.IconSuccess),
			currentProject.Name, currentProject.Language)
		fmt.Fprintf(a.IO.Out, "  Chemin: %s\n", currentProject.Path)
		if currentProject.Tracker != "" {
			fmt.Fprintf(a.IO.Out, "  Tracker: %s\n", currentProject.Tracker)
		}
	} else {
		fmt.Fprintf(a.IO.Out, "  %s Pas dans un projet enregistré (%s)\n",
			common.WarningStyle.Render(common.IconWarning), cwd)
	}

	return nil
}

func isSubPath(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return !filepath.IsAbs(rel) && rel != ".." && !startsWith(rel, "..")
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
