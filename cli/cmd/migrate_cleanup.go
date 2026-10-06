package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/deploycleanup"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/prefsvc"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// oh migrate deploy-cleanup (P3-T28): removes what the former per-project
// deployment left in the registered projects (internal/deploycleanup),
// after showing it (diff of opencode.json) and asking for confirmation.

var migrateCleanupCmd = &cobra.Command{
	Use:   "deploy-cleanup",
	Short: i18n.T("cmd.migrate.cleanup.short"),
	Long:  i18n.T("cmd.migrate.cleanup.long"),
	Args:  cobra.NoArgs,
	RunE:  runMigrateCleanup,
}

func init() {
	f := migrateCleanupCmd.Flags()
	f.StringP("project", "p", "", i18n.T("cmd.migrate.cleanup.flags.project"))
	f.Bool("dry-run", false, i18n.T("cmd.migrate.cleanup.flags.dry_run"))
	f.BoolP("yes", "y", false, i18n.T("cmd.migrate.cleanup.flags.yes"))
	f.Bool("diff", false, i18n.T("cmd.migrate.cleanup.flags.diff"))
	f.Bool("json", false, i18n.T("cmd.migrate.cleanup.flags.json"))
	migrateCmd.AddCommand(migrateCleanupCmd)
}

// projectCleanup is the cleanup plan of a registered project.
type projectCleanup struct {
	Project domain.Project      `json:"-"`
	Name    string              `json:"project"`
	Plan    *deploycleanup.Plan `json:"plan"`
}

// cleanupOptions are the hub agents and instruction files oh deployed.
func cleanupOptions(a *app.App) deploycleanup.Options {
	opts := deploycleanup.Options{InstructionFiles: a.Config.Deploy.InstructionFiles}
	if hub := findHubDir(); hub != "" {
		if files, err := bricks.FindAgentFiles(hub); err == nil {
			for id := range files {
				opts.AgentIDs = append(opts.AgentIDs, id)
			}
		}
	}
	return opts
}

// scanDeployLeftovers returns the projects with former deployment files
// (projectID "" = every active project).
func scanDeployLeftovers(ctx context.Context, a *app.App, projectID string) ([]projectCleanup, error) {
	var projects []domain.Project
	if projectID != "" {
		p, err := resolveProject(ctx, a, projectID)
		if err != nil {
			return nil, err
		}
		projects = []domain.Project{*p}
	} else if a.Projects != nil {
		list, err := a.Projects.List(ctx, domain.ProjectStatusActive)
		if err != nil {
			return nil, err
		}
		projects = list
	}
	opts := cleanupOptions(a)
	var out []projectCleanup
	for _, p := range projects {
		if p.Path == "" {
			continue
		}
		if _, err := os.Stat(p.Path); err != nil {
			continue
		}
		plan, err := deploycleanup.Scan(p.Path, opts)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p.Name, err)
		}
		if plan.Leftovers() {
			out = append(out, projectCleanup{Project: p, Name: p.Name, Plan: plan})
		}
	}
	return out, nil
}

func runMigrateCleanup(cmd *cobra.Command, _ []string) error {
	a := MustApp()
	ctx := cmd.Context()
	f := cmd.Flags()
	projectID, _ := f.GetString("project")
	dry, _ := f.GetBool("dry-run")
	yes, _ := f.GetBool("yes")
	showDiff, _ := f.GetBool("diff")
	asJSON, _ := f.GetBool("json")
	out := cmd.OutOrStdout()

	list, err := scanDeployLeftovers(ctx, a, projectID)
	if err != nil {
		return err
	}
	markCleanupOffered(ctx, a)
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if list == nil {
			list = []projectCleanup{}
		}
		return enc.Encode(list)
	}
	if len(list) == 0 {
		fmt.Fprintf(out, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.T("cmd.migrate.cleanup.none"))
		return nil
	}
	for _, pc := range list {
		printCleanupPlan(out, pc, showDiff || dry)
	}
	if dry {
		fmt.Fprintln(out, i18n.T("cmd.migrate.cleanup.dry_run"))
		return nil
	}
	if !yes {
		if !isTerminal() {
			return errors.New(i18n.T("cmd.migrate.cleanup.needs_yes"))
		}
		ok := false
		form := theme.NewForm(huh.NewGroup(huh.NewConfirm().Title(i18n.Tf("cmd.migrate.cleanup.confirm", len(list))).Value(&ok)))
		if err := form.Run(); err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(out, i18n.T("cmd.migrate.cleanup.cancelled"))
			return nil
		}
	}
	return applyCleanups(out, list)
}

func applyCleanups(out io.Writer, list []projectCleanup) error {
	var errs []error
	for _, pc := range list {
		if err := pc.Plan.Apply(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", pc.Name, err))
			fmt.Fprintf(out, "%s %s\n", theme.ErrorStyle.Render(theme.IconError), i18n.Tf("cmd.migrate.cleanup.failed", pc.Name, err))
			continue
		}
		fmt.Fprintf(out, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.migrate.cleanup.done", pc.Name))
	}
	return errors.Join(errs...)
}

// cleanupLines describes a plan (shared by the CLI and the TUI screen).
func cleanupLines(p *deploycleanup.Plan) []string {
	var lines []string
	for _, it := range p.Items {
		l := "− " + it.Rel
		if it.Dir {
			l += " " + i18n.Tf("cmd.migrate.cleanup.files", it.Count)
		}
		lines = append(lines, l)
	}
	if len(p.Removed) > 0 {
		lines = append(lines, i18n.Tf("cmd.migrate.cleanup.config_removed", strings.Join(p.Removed, ", ")))
	}
	if p.DeleteConfig {
		lines = append(lines, i18n.T("cmd.migrate.cleanup.config_deleted"))
	}
	if len(p.Kept) > 0 {
		lines = append(lines, i18n.Tf("cmd.migrate.cleanup.config_kept", strings.Join(p.Kept, ", ")))
	}
	if p.ConfigUntouched != "" {
		lines = append(lines, i18n.T("cmd.migrate.cleanup.untouched."+p.ConfigUntouched))
	}
	return lines
}

func printCleanupPlan(out io.Writer, pc projectCleanup, diff bool) {
	fmt.Fprintf(out, "%s %s\n", theme.Bold.Render(pc.Name), theme.Subtitle.Render(pc.Project.Path))
	for _, l := range cleanupLines(pc.Plan) {
		fmt.Fprintln(out, "  "+l)
	}
	if d := pc.Plan.Diff(); diff && d != "" {
		fmt.Fprintln(out)
		fmt.Fprintln(out, d)
	}
	fmt.Fprintln(out)
}

// The cleanup is offered once after the update to v5 (notice at startup,
// screen in the TUI); the preference remembers it was offered.
const cleanupOfferedKey = "migrate.deploy_cleanup.offered"

func cleanupOffered(ctx context.Context, a *app.App) bool {
	if a.Preferences == nil {
		return true
	}
	var done bool
	ok, err := prefsvc.New(a.Preferences, a.WorkflowUsage).Get(ctx, domain.PreferenceScopeGlobal, cleanupOfferedKey, &done)
	return err != nil || (ok && done)
}

func markCleanupOffered(ctx context.Context, a *app.App) {
	if a.Preferences != nil {
		_ = prefsvc.New(a.Preferences, a.WorkflowUsage).Set(ctx, domain.PreferenceScopeGlobal, cleanupOfferedKey, true)
	}
}

// offerDeployCleanupAtStartup prints, once, where former deployment files
// were found (CLI commands; the TUI shows its cleanup screen instead).
func offerDeployCleanupAtStartup(ctx context.Context, w io.Writer, cmdName string) {
	a := TryApp()
	if a == nil || cmdName == "migrate" || cmdName == "deploy-cleanup" || cleanupOffered(ctx, a) {
		return
	}
	list, err := scanDeployLeftovers(ctx, a, "")
	if err != nil {
		return
	}
	markCleanupOffered(ctx, a)
	if len(list) > 0 {
		fmt.Fprintln(w, "oh: "+i18n.Tf("cmd.migrate.cleanup.offer", len(list)))
	}
}

// offerCleanupFor reports whether a command may print the cleanup offer:
// user commands only (not the TUI, which shows a screen, nor the processes
// started by sessions and the daemon).
func offerCleanupFor(cmd *cobra.Command) bool {
	if !cmd.HasParent() {
		return false
	}
	for c := cmd; c != nil; c = c.Parent() {
		switch c.Name() {
		case "init", "purge", "migrate", "mcp", "daemon", "runner", "gateway", "completion", "__complete", "help", "version", "serve", "hook":
			return false
		}
	}
	return true
}
