package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Affiche l'état du hub et du projet courant",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().Bool("json", false, "Output in JSON format")
}

func runStatus(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	// Projects summary
	projects, err := a.Projects.List(ctx, "")
	if err != nil {
		return err
	}

	active := 0
	for _, p := range projects {
		if p.Status == domain.ProjectStatusActive {
			active++
		}
	}

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

	jsonOut, _ := cmd.Flags().GetBool("json")
	if jsonOut {
		type statusJSON struct {
			ConfigPath     string          `json:"config_path"`
			Language       string          `json:"language"`
			OpencodeVer    string          `json:"opencode_version"`
			Channel        string          `json:"channel"`
			TotalProjects  int             `json:"total_projects"`
			ActiveProjects int             `json:"active_projects"`
			CurrentProject *domain.Project `json:"current_project,omitempty"`
			Cwd            string          `json:"cwd"`
		}
		out := statusJSON{
			ConfigPath:     config.ConfigPath(),
			Language:       a.Config.CLI.Language,
			OpencodeVer:    a.Config.Opencode.Version,
			Channel:        a.Config.Opencode.Channel,
			TotalProjects:  len(projects),
			ActiveProjects: active,
			CurrentProject: currentProject,
			Cwd:            cwd,
		}
		return json.NewEncoder(os.Stdout).Encode(out)
	}

	fmt.Fprintln(a.IO.Out, common.Title.Render("  oh status  "))
	fmt.Fprintln(a.IO.Out)

	// Hub info
	fmt.Fprintln(a.IO.Out, common.Bold.Render(i18n.T("cmd.status.title_hub")))
	w := tabwriter.NewWriter(a.IO.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  %s\t%s\n", i18n.T("cmd.status.config"), config.ConfigPath())
	fmt.Fprintf(w, "  %s\t%s\n", i18n.T("cmd.status.language"), a.Config.CLI.Language)
	fmt.Fprintf(w, "  %s\t%s (%s)\n", i18n.T("cmd.status.opencode"), a.Config.Opencode.Version, a.Config.Opencode.Channel)
	w.Flush()
	fmt.Fprintln(a.IO.Out)

	fmt.Fprintln(a.IO.Out, common.Bold.Render(i18n.T("cmd.status.title_projects")))
	fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.status.total_active", len(projects), active))
	fmt.Fprintln(a.IO.Out)

	if currentProject != nil {
		fmt.Fprintln(a.IO.Out, common.Bold.Render(i18n.T("cmd.status.title_current")))
		fmt.Fprintf(a.IO.Out, "  %s %s (%s)\n",
			common.SuccessStyle.Render(common.IconSuccess),
			currentProject.Name, currentProject.Language)
		fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.status.path", currentProject.Path))
	} else {
		fmt.Fprintf(a.IO.Out, "  %s %s\n",
			common.WarningStyle.Render(common.IconWarning), i18n.Tf("cmd.status.not_in_project", cwd))
	}

	return nil
}

func isSubPath(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
