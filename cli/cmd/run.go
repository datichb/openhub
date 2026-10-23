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
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/services/launch"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

var runCmd = &cobra.Command{
	Use:   "run [workflow]",
	Short: "Lance un workflow (une session par ticket ; sans argument : workflow par défaut du projet)",
	Args:  cobra.MaximumNArgs(1),
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
	f.StringP("message", "m", "", "Demande (raccourci de -i request=…, si le workflow a cette entrée)")
	f.String("mode", "", "Mode du workflow (manuel, semi-auto, auto)")
	f.StringP("agent", "a", "", i18n.T("cmd.run.flags.agent"))
	f.String("runtime", "", i18n.T("cmd.run.flags.runtime"))
	f.String("location", "", "Emplacement : base, new (nouveau worktree) ou chemin d'un worktree existant")
	f.Bool("allow-dirty", false, "Lancer quand même dans un dossier modifié (sans worktree ni stash)")
	f.Bool("stash", false, "Mettre de côté (git stash) les modifications du dossier avant le lancement")
	f.StringSlice("tickets", nil, "Tickets Beads (une session par ticket si le workflow le permet)")
	f.String("attach", "", "Ouverture : auto, iterm, terminal, tmux, browser, suspend, none")
	f.Bool("draft", false, "Utiliser mon brouillon du workflow (local uniquement, sans assouplir la sécurité publiée)")
	f.Bool("recap", false, "Afficher le récapitulatif et demander confirmation avant le lancement")
	f.BoolP("yes", "y", false, i18n.T("cmd.confirm.flags.yes"))
	f.StringP("project", "p", "", "Projet (détecté depuis le dossier courant)")
	f.StringP("provider", "P", "", "Fournisseur LLM")
	f.String("parent", "", "Session précédente (enchaînement)")
	f.Bool("one-session", false, "Tous les tickets dans une seule session (au lieu d'une session par ticket)")
	f.Bool("dry-run", false, i18n.T("cmd.run.dry_run.flag"))
	f.Bool("headless", false, "Sans interface : attendre la fin du tour, afficher la réponse, arrêter la session")
	f.String("output", "", "Avec --headless : écrire la réponse dans ce fichier (défaut : sortie standard)")
	f.Duration("timeout", 30*time.Minute, "Avec --headless : durée maximale d'attente (0 = aucune)")
	f.String("on-existing", "", i18n.T("cmd.run.flags.on_existing"))
	f.Bool("discard-base-changes", false, i18n.T("cmd.run.flags.discard_base"))
}

func runWorkflowCmd(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()
	workflowID := ""
	if len(args) > 0 {
		workflowID = args[0]
	}
	opts, err := runOptionsFromFlags(cmd, workflowID)
	if err != nil {
		return err
	}
	// The tool first: no project question for a launch that cannot start.
	if err := requireV2(ctx); err != nil {
		return err
	}
	projectID, _ := cmd.Flags().GetString("project")
	if opts.Project, err = resolveProject(ctx, a, projectID, projectWrite); err != nil {
		return err
	}
	if opts.Workflow == "" {
		if opts.Workflow = projectExec(opts.Project).DefaultWorkflow; opts.Workflow == "" {
			return errors.New(i18n.Tf("cmd.run.no_default_workflow", opts.Project.Name))
		}
	}
	recap, _ := cmd.Flags().GetBool("recap")
	headless, _ := cmd.Flags().GetBool("headless")
	if dry, _ := cmd.Flags().GetBool("dry-run"); dry {
		return runWorkflowDryRun(cmd, opts, headless)
	}
	if headless {
		return runWorkflowHeadlessCLI(cmd, opts)
	}
	return runWorkflowCLI(cmd, opts, recap)
}

// runWorkflowDryRun runs `oh run --dry-run`: the launch is prepared (workflow
// resolved, bundle compiled, sessions planned) and shown (recap with the
// pauses, warnings), then nothing is started. Nothing is asked either: no
// choice about existing sessions, no suggested workflow, no import of the
// Bedrock key. With --headless it also tells whether the launch could run
// without interface.
func runWorkflowDryRun(cmd *cobra.Command, opts launch.Options, headless bool) error {
	a := MustApp()
	out := cmd.OutOrStdout()
	opts.DryRun = true
	if headless {
		if opts.Runtime == string(sessionspec.RuntimeRemote) {
			return errors.New(i18n.T("cmd.remote.send.headless"))
		}
		opts.Attach = string(sessionspec.AttachNone)
	}
	opts.Progress = launchProgress(out, cmd.ErrOrStderr())
	fmt.Fprintf(out, "%s %s\n", theme.SuccessStyle.Render(theme.IconArrow), i18n.Tf("cmd.run.preparing", opts.Workflow))
	p, err := launch.Prepare(cmd.Context(), launchDeps(a), opts)
	if err != nil {
		return err
	}
	if headless {
		if err := headlessCheck(p); err != nil {
			return err
		}
	}
	printRunRecap(out, p)
	fmt.Fprintf(out, "%s %s\n", theme.Subtitle.Render(theme.IconArrow), i18n.T("cmd.run.dry_run.done"))
	return nil
}

// runWorkflowHeadlessCLI runs `oh run --headless`: the answer of each session
// goes to stdout or to --output (one file per session: <output>.<label>).
func runWorkflowHeadlessCLI(cmd *cobra.Command, opts launch.Options) error {
	a := MustApp()
	ctx := cmd.Context()
	errOut := cmd.ErrOrStderr()
	if opts.Runtime == string(sessionspec.RuntimeRemote) {
		return errors.New(i18n.T("cmd.remote.send.headless"))
	}
	opts.Attach = string(sessionspec.AttachNone)
	opts.Progress = launchProgress(errOut, errOut)
	fmt.Fprintf(errOut, "%s %s\n", theme.SuccessStyle.Render(theme.IconArrow), i18n.Tf("cmd.run.preparing", opts.Workflow))
	p, err := launch.Prepare(ctx, launchDeps(a), opts)
	if err != nil {
		return err
	}
	printRunWarnings(errOut, p.Plan.Warnings)
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
			if i < len(p.Plan.Sessions) && p.Plan.Sessions[i].Label != "" {
				path = output + "." + p.Plan.Sessions[i].Label
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
func runOptionsFromFlags(cmd *cobra.Command, workflowID string) (launch.Options, error) {
	opts := launch.Options{Workflow: workflowID, Inputs: map[string]string{}}
	f := cmd.Flags()
	raw, _ := f.GetStringArray("input")
	for _, kv := range raw {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			return opts, errors.New(i18n.Tf("cmd.run.bad_input", kv))
		}
		opts.Inputs[strings.TrimSpace(k)] = v
	}
	if f.Changed("message") {
		msg, _ := f.GetString("message")
		if _, set := opts.Inputs[launch.RequestInput]; set {
			return opts, errors.New(i18n.T("cmd.run.message_and_request"))
		}
		if strings.TrimSpace(msg) == "" {
			return opts, errors.New(i18n.T("cmd.run.message_empty"))
		}
		opts.Inputs[launch.RequestInput] = msg
		opts.Message = true
	}
	opts.Mode, _ = f.GetString("mode")
	opts.Runtime, _ = f.GetString("runtime")
	opts.Location, _ = f.GetString("location")
	opts.Attach, _ = f.GetString("attach")
	opts.Provider, _ = f.GetString("provider")
	opts.ParentSessionID, _ = f.GetString("parent")
	opts.OneSession, _ = f.GetBool("one-session")
	opts.Draft, _ = f.GetBool("draft")
	opts.Agent, _ = f.GetString("agent")
	allowDirty, _ := f.GetBool("allow-dirty")
	stash, _ := f.GetBool("stash")
	switch {
	case allowDirty && stash:
		return opts, errors.New(i18n.T("cmd.run.dirty_conflict"))
	case allowDirty:
		opts.Dirty = runsvc.DirtyAllow
	case stash:
		opts.Dirty = runsvc.DirtyStash
	}
	if f.Lookup("on-existing") != nil {
		v, _ := f.GetString("on-existing")
		choice, err := runsvc.ParseOnExisting(v)
		if err != nil {
			return opts, errors.New(i18n.Tf("cmd.run.existing.bad_flag", v))
		}
		opts.OnExisting = choice
		opts.DiscardBaseChanges, _ = f.GetBool("discard-base-changes")
	}
	tickets, _ := f.GetStringSlice("tickets")
	for _, t := range tickets {
		if t = strings.TrimSpace(t); t != "" {
			opts.Tickets = append(opts.Tickets, t)
		}
	}
	return opts, nil
}

// runWorkflowCLI prepares, optionally confirms, and starts a launch.
func runWorkflowCLI(cmd *cobra.Command, opts launch.Options, recap bool) error {
	a := MustApp()
	ctx := cmd.Context()
	out := cmd.OutOrStdout()
	opts.Progress = launchProgress(out, cmd.ErrOrStderr())
	fmt.Fprintf(out, "%s %s\n", theme.SuccessStyle.Render(theme.IconArrow), i18n.Tf("cmd.run.preparing", opts.Workflow))
	p, err := launch.Prepare(ctx, launchDeps(a), opts)
	if err != nil && offerBedrockEnvImport(ctx, a, opts.Project, opts.Provider, err, cmd.ErrOrStderr()) {
		p, err = launch.Prepare(ctx, launchDeps(a), opts)
	}
	if err != nil {
		return err
	}
	if len(p.Suggestions) > 0 && isTerminal() {
		handled, err := askPreconditionChoice(cmd, a, opts, p)
		if handled || err != nil {
			return err
		}
	}
	if ok, err := resolveExistingChoice(ctx, out, p, isTerminal()); err != nil || !ok {
		if err == nil {
			fmt.Fprintln(out, i18n.T("cmd.run.cancelled"))
		}
		return err
	}
	if recap {
		printRunRecap(out, p)
		ok, err := confirmAction(cmd, i18n.T("cmd.run.confirm"), true)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(out, i18n.T("cmd.run.cancelled"))
			return nil
		}
	} else {
		printRunPauses(out, p)
		printRunWarnings(out, p.Plan.Warnings)
	}
	_, err = startRun(ctx, a, p, launcher.NewCLIUI(out))
	var te *runsvc.TransferError
	if errors.As(err, &te) {
		if !isTerminal() {
			return fmt.Errorf("%w\n%s", err, i18n.T("cmd.run.existing.transfer_hint"))
		}
		ok, aerr := askTransferFallback(out, te)
		if aerr != nil || !ok {
			return errors.Join(err, aerr)
		}
		if err := launch.Replan(ctx, p, runsvc.OnExistingKeep, false); err != nil {
			return err
		}
		_, err = startRun(ctx, a, p, launcher.NewCLIUI(out))
	}
	return err
}

// askPreconditionChoice offers to run the suggested workflow first (failed
// `suggest` precondition). handled is true when the original launch must not
// start now (suggested workflow started, or cancelled).
func askPreconditionChoice(cmd *cobra.Command, a *app.App, opts launch.Options, p *launch.Prepared) (bool, error) {
	s := p.Suggestions[0]
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
