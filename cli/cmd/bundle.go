package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/runsvc"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// oh bundle build|show: the session bundle of a workflow (replaces oh deploy
// to inspect what a session sees, and oh skill budget).

var bundleCmd = &cobra.Command{
	Use:   "bundle",
	Short: "Paquets de session (contenu compilé d'un workflow)",
}

func init() {
	rootCmd.AddCommand(bundleCmd)
	bundleCmd.AddCommand(bundleBuildCmd(), bundleShowCmd())
}

func bundleFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("project", "p", "", "Projet (instructions, modèles, MCP) ; détecté depuis le dossier courant")
	cmd.Flags().StringP("provider", "P", "", "Fournisseur LLM (normalisation des modèles)")
	cmd.Flags().Bool("json", false, "Sortie JSON")
}

func bundleBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build <workflow>",
		Short: "Compile le paquet de session d'un workflow",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := buildBundleFor(cmd, args[0])
			if err != nil {
				return err
			}
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				return printJSON(cmd, map[string]string{"hash": b.Spec.Hash, "dir": b.Dir})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess),
				i18n.Tf("cmd.bundle.build.done", args[0], shortHash(b.Spec.Hash), b.Dir))
			return nil
		},
	}
	bundleFlags(cmd)
	return cmd
}

func bundleShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <workflow>|<hash>",
		Short: "Affiche le contenu d'un paquet de session (agents, skills, permissions, budget)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := bundle.Load(ohBundlesDir(), args[0])
			if err != nil {
				if b, err = buildBundleFor(cmd, args[0]); err != nil {
					return err
				}
			}
			r := bundle.Show(b)
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				return printJSON(cmd, r)
			}
			budget, _ := cmd.Flags().GetBool("budget")
			printBundleReport(cmd.OutOrStdout(), args[0], r, budget)
			return nil
		},
	}
	bundleFlags(cmd)
	cmd.Flags().Bool("budget", false, "Détaille le budget estimé (tokens par agent et par skill)")
	return cmd
}

// buildBundleFor resolves a workflow and compiles its bundle.
func buildBundleFor(cmd *cobra.Command, id string) (*bundle.Bundle, error) {
	a := MustApp()
	ctx := cmd.Context()
	projectID, _ := cmd.Flags().GetString("project")
	project, err := bundleProject(ctx, a, projectID)
	if err != nil {
		return nil, err
	}
	c := workflowsvc.Context{}
	if project != nil {
		c.ProjectID = project.ID // the workflow of the project's layers (A6)
	}
	res, err := newWorkflowService(ctx).Resolve(ctx, c, id, workflowsvc.ResolveOpts{})
	if err != nil {
		return nil, workflowError(cmd.ErrOrStderr(), id, err)
	}
	provFlag, _ := cmd.Flags().GetString("provider")
	prov := provFlag
	if project != nil {
		prov = provider.ResolveProvider(provFlag, project.Provider, a.Config.LLM.DefaultProvider)
	} else if prov == "" {
		prov = a.Config.LLM.DefaultProvider
	}
	b, missing, err := buildWorkflowBundle(a, project, res, prov)
	if err != nil {
		return nil, fmt.Errorf("building session bundle: %w", err)
	}
	for _, m := range missing {
		fmt.Fprintf(cmd.ErrOrStderr(), "%s %s\n", theme.WarningStyle.Render(theme.IconWarning), warningText(runsvc.Warning{Code: "mcp_missing", Args: []any{m}}))
	}
	return b, nil
}

// workflowError explains a resolution failure (findings printed to w).
func workflowError(w io.Writer, id string, err error) error {
	if errors.Is(err, workflowsvc.ErrUnknownWorkflow) {
		return errors.New(i18n.Tf("cmd.workflow.show.unknown", id))
	}
	var invalid *workflowsvc.InvalidError
	if errors.As(err, &invalid) {
		printDiagnostics(w, invalid.Diagnostics)
		return errors.New(i18n.Tf("cmd.workflow.show.invalid", len(invalid.Diagnostics.Errors())))
	}
	return err
}

// bundleProject returns the -p project, else the project of the current
// directory, else nil (hub-only bundle). It never prompts.
func bundleProject(ctx context.Context, a *app.App, projectID string) (*domain.Project, error) {
	if projectID != "" {
		return resolveProject(ctx, a, projectID)
	}
	if a.Projects == nil {
		return nil, nil
	}
	cwd, _ := os.Getwd()
	if resolved, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = resolved
	}
	projects, err := a.Projects.List(ctx, domain.ProjectStatusActive)
	if err != nil {
		return nil, err
	}
	for i, p := range projects {
		abs, _ := filepath.Abs(p.Path)
		if resolved, err := filepath.EvalSymlinks(abs); err == nil {
			abs = resolved // symbolic links (/var → /private/var on macOS)
		}
		if abs == cwd || isSubPath(cwd, abs) {
			return &projects[i], nil
		}
	}
	return nil, nil
}

func printBundleReport(w io.Writer, name string, r *bundle.Report, budget bool) {
	fmt.Fprintln(w, theme.Title.Render(i18n.Tf("cmd.bundle.show.title", name, shortHash(r.Hash))))
	fmt.Fprintln(w, theme.Subtitle.Render(r.Dir))
	row := func(label, value string) {
		if value == "" {
			value = "—"
		}
		fmt.Fprintf(w, "  %-14s %s\n", label, value)
	}
	agents := make([]string, len(r.Agents))
	for i, ag := range r.Agents {
		agents[i] = ag.ID
		if ag.Entry {
			agents[i] += " (" + i18n.T("cmd.bundle.show.entry") + ")"
		}
	}
	row(i18n.Tf("cmd.bundle.show.agents", len(r.Agents)), strings.Join(agents, " · "))
	row(i18n.Tf("cmd.bundle.show.skills", len(r.Skills)), i18n.Tf("cmd.bundle.show.tokens", r.Budget.Skills))
	row("MCP", strings.Join(r.MCP, ", "))
	row(i18n.T("cmd.bundle.show.plugins"), strings.Join(r.Plugins, ", "))
	row(i18n.T("cmd.bundle.show.model"), r.DefaultModel)
	row(i18n.T("cmd.bundle.show.depth"), fmt.Sprint(r.MaxDepth))
	codeMode := "off"
	if r.CodeMode {
		codeMode = "on"
	}
	row("Code Mode", codeMode)
	isolation := string(r.Isolation)
	if r.StrictIsolation {
		isolation += " · strict"
	}
	row(i18n.T("cmd.bundle.show.isolation"), isolation)
	var delegations []string
	for _, ag := range r.Agents {
		if len(ag.Calls) > 0 {
			delegations = append(delegations, ag.ID+" → "+strings.Join(ag.Calls, ", "))
		}
	}
	row(i18n.T("cmd.bundle.show.delegations"), strings.Join(delegations, " ; "))
	var perms []string
	for _, p := range r.Permissions {
		perms = append(perms, fmt.Sprintf("%s %s %s", p.Action, p.Resource, p.Effect))
	}
	row(i18n.T("cmd.bundle.show.permissions"), strings.Join(perms, " ; "))
	row(i18n.T("cmd.bundle.show.initial"), i18n.Tf("cmd.bundle.show.initial_value", r.Budget.Initial, r.Budget.EntryAgent, r.Budget.SkillCatalog))

	if !budget {
		return
	}
	fmt.Fprintf(w, "\n%s\n", theme.Title.Render(i18n.T("cmd.bundle.show.budget_title")))
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "  %s\t%s\t\n", i18n.T("cmd.bundle.show.col_agent"), i18n.T("cmd.bundle.show.col_tokens"))
	for _, ag := range r.Agents {
		fmt.Fprintf(tw, "  %s\t%d\t\n", ag.ID, ag.Tokens)
	}
	fmt.Fprintf(tw, "  %s\t%d\t\n", i18n.T("cmd.bundle.show.total"), r.Budget.Agents)
	_ = tw.Flush()
	fmt.Fprintln(w)
	tw = tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "  %s\t%s\t\n", i18n.T("cmd.bundle.show.col_skill"), i18n.T("cmd.bundle.show.col_tokens"))
	for _, sk := range r.Skills {
		fmt.Fprintf(tw, "  %s\t%d\t\n", sk.ID, sk.Tokens)
	}
	fmt.Fprintf(tw, "  %s\t%d\t\n", i18n.T("cmd.bundle.show.total"), r.Budget.Skills)
	_ = tw.Flush()
	fmt.Fprintln(w, theme.Subtitle.Render(i18n.T("cmd.bundle.show.estimate_note")))
}

func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}
