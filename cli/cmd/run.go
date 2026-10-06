package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/launcher"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

var runCmd = &cobra.Command{
	Use:   "run <workflow>",
	Short: "Lance un workflow (une session par ticket)",
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkflowCmd,
}

func init() {
	rootCmd.AddCommand(runCmd)
	addRunFlags(runCmd)
}

// addRunFlags registers the flags of oh run.
func addRunFlags(c *cobra.Command) {
	f := c.Flags()
	f.StringArrayP("input", "i", nil, "Entrée du workflow (clé=valeur, répétable)")
	f.String("mode", "", "Mode du workflow (manuel, semi-auto, auto)")
	f.String("runtime", "", "Environnement d'exécution (local, container)")
	f.String("location", "", "Emplacement : base, new (nouveau worktree) ou chemin d'un worktree existant")
	f.StringSlice("tickets", nil, "Tickets Beads (une session par ticket si le workflow le permet)")
	f.String("attach", "", "Ouverture : auto, iterm, terminal, tmux, browser, suspend, none")
	f.Bool("draft", false, "Utiliser mon brouillon du workflow (phase 2)")
	f.Bool("recap", false, "Afficher le récapitulatif et demander confirmation avant le lancement")
	f.StringP("project", "p", "", "Projet (détecté depuis le dossier courant)")
	f.StringP("provider", "P", "", "Fournisseur LLM")
	f.String("parent", "", "Session précédente (enchaînement)")
	f.Bool("one-session", false, "Tous les tickets dans une seule session (au lieu d'une session par ticket)")
	f.Bool("headless", false, "Sans interface : attendre la fin du tour, afficher la réponse, arrêter la session")
	f.String("output", "", "Avec --headless : écrire la réponse dans ce fichier (défaut : sortie standard)")
	f.Duration("timeout", 30*time.Minute, "Avec --headless : durée maximale d'attente (0 = aucune)")
}

func runWorkflowCmd(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()
	if draft, _ := cmd.Flags().GetBool("draft"); draft {
		return errors.New(i18n.T("cmd.run.draft_unsupported"))
	}
	opts, err := runOptionsFromFlags(cmd, args[0])
	if err != nil {
		return err
	}
	projectID, _ := cmd.Flags().GetString("project")
	if opts.Project, err = resolveProject(ctx, a, projectID); err != nil {
		return err
	}
	recap, _ := cmd.Flags().GetBool("recap")
	if headless, _ := cmd.Flags().GetBool("headless"); headless {
		return runWorkflowHeadlessCLI(cmd, opts)
	}
	return runWorkflowCLI(cmd, opts, recap)
}

// runWorkflowHeadlessCLI runs `oh run --headless`: the answer of each session
// goes to stdout or to --output (one file per session: <output>.<label>).
func runWorkflowHeadlessCLI(cmd *cobra.Command, opts runOptions) error {
	a := MustApp()
	ctx := cmd.Context()
	errOut := cmd.ErrOrStderr()
	opts.Attach = string(sessionspec.AttachNone)
	opts.Progress = func(line string) { fmt.Fprintln(errOut, theme.Subtitle.Render("  "+line)) }
	fmt.Fprintf(errOut, "%s %s\n", theme.SuccessStyle.Render(theme.IconArrow), i18n.Tf("cmd.run.preparing", opts.Workflow))
	p, err := prepareWorkflowRun(ctx, a, opts, errOut)
	if err != nil {
		return err
	}
	printRunWarnings(errOut, p.plan.Warnings)
	timeout, _ := cmd.Flags().GetDuration("timeout")
	results, err := runHeadless(ctx, a, p, launcher.NewCLIUI(errOut), timeout)
	output, _ := cmd.Flags().GetString("output")
	for i, r := range results {
		if output == "" {
			fmt.Fprintln(cmd.OutOrStdout(), r.Text)
			continue
		}
		path := output
		if len(results) > 1 {
			path = fmt.Sprintf("%s.%d", output, i+1)
			if i < len(p.plan.Sessions) && p.plan.Sessions[i].Label != "" {
				path = output + "." + p.plan.Sessions[i].Label
			}
		}
		if werr := os.WriteFile(path, []byte(r.Text), 0o600); werr != nil {
			return werr
		}
		fmt.Fprintf(errOut, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.run.headless_written", path, r.Result.Cost))
	}
	return err
}

// runOptionsFromFlags reads the launch flags shared by oh run and its aliases.
func runOptionsFromFlags(cmd *cobra.Command, workflowID string) (runOptions, error) {
	opts := runOptions{Workflow: workflowID, Inputs: map[string]string{}}
	f := cmd.Flags()
	raw, _ := f.GetStringArray("input")
	for _, kv := range raw {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			return opts, errors.New(i18n.Tf("cmd.run.bad_input", kv))
		}
		opts.Inputs[strings.TrimSpace(k)] = v
	}
	opts.Mode, _ = f.GetString("mode")
	opts.Runtime, _ = f.GetString("runtime")
	opts.Location, _ = f.GetString("location")
	opts.Attach, _ = f.GetString("attach")
	opts.Provider, _ = f.GetString("provider")
	opts.ParentSessionID, _ = f.GetString("parent")
	opts.OneSession, _ = f.GetBool("one-session")
	tickets, _ := f.GetStringSlice("tickets")
	for _, t := range tickets {
		if t = strings.TrimSpace(t); t != "" {
			opts.Tickets = append(opts.Tickets, t)
		}
	}
	return opts, nil
}

// runWorkflowCLI prepares, optionally confirms, and starts a launch.
func runWorkflowCLI(cmd *cobra.Command, opts runOptions, recap bool) error {
	a := MustApp()
	ctx := cmd.Context()
	out := cmd.OutOrStdout()
	opts.Progress = func(line string) { fmt.Fprintln(out, theme.Subtitle.Render("  "+line)) }
	fmt.Fprintf(out, "%s %s\n", theme.SuccessStyle.Render(theme.IconArrow), i18n.Tf("cmd.run.preparing", opts.Workflow))
	p, err := prepareWorkflowRun(ctx, a, opts, cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	if len(p.suggestions) > 0 && isTerminal() {
		handled, err := askPreconditionChoice(cmd, a, opts, p)
		if handled || err != nil {
			return err
		}
	}
	if recap {
		printRunRecap(out, p)
		ok := true
		form := theme.NewForm(huh.NewGroup(huh.NewConfirm().Title(i18n.T("cmd.run.confirm")).Value(&ok)))
		if err := form.Run(); err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(out, i18n.T("cmd.run.cancelled"))
			return nil
		}
	} else {
		printRunWarnings(out, p.plan.Warnings)
	}
	_, err = p.start(ctx, a, launcher.NewCLIUI(out))
	return err
}

// askPreconditionChoice offers to run the suggested workflow first (failed
// `suggest` precondition). handled is true when the original launch must not
// start now (suggested workflow started, or cancelled).
func askPreconditionChoice(cmd *cobra.Command, a *app.App, opts runOptions, p *preparedRun) (bool, error) {
	s := p.suggestions[0]
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s %s\n", theme.WarningStyle.Render(theme.IconWarning), i18n.Tf("cmd.run.warn.precondition", s.Label, s.Workflow))
	choice := "first"
	first := i18n.Tf("cmd.run.precondition_first", s.Workflow)
	if s.Resume {
		first = i18n.Tf("cmd.run.precondition_first_resume", s.Workflow, opts.Workflow)
	}
	form := theme.NewForm(huh.NewGroup(huh.NewSelect[string]().Title(i18n.T("cmd.run.precondition_title")).
		Options(huh.NewOption(first, "first"), huh.NewOption(i18n.T("cmd.run.precondition_continue"), "continue"),
			huh.NewOption(i18n.T("cmd.run.precondition_cancel"), "cancel")).Value(&choice)))
	if err := form.Run(); err != nil {
		return true, err
	}
	switch choice {
	case "continue":
		return false, nil
	case "cancel":
		fmt.Fprintln(out, i18n.T("cmd.run.cancelled"))
		return true, nil
	}
	if _, err := runSuggestedFirst(cmd.Context(), a, opts, s, launcher.NewCLIUI(out)); err != nil {
		return true, err
	}
	if s.Resume {
		fmt.Fprintf(out, "%s %s\n", theme.Subtitle.Render(theme.IconArrow), i18n.Tf("cmd.run.precondition_resume_hint", resumeCommand(resumeIntent{
			Workflow: opts.Workflow, Inputs: opts.Inputs, Tickets: opts.Tickets})))
	}
	return true, nil
}
