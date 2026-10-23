package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/launcher"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/services/launch"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// Workflow launches (oh run, aliases, TUI launch form): the resolution, the
// bundle, the plan of the sessions and their start are the services/launch
// package; what stays here is what the command line shows (recap, warnings),
// the opening of the client and the wiring of the service to the app.

// requestInput is the input -m fills (oh run -m).
const requestInput = launch.RequestInput

// runOptions are the choices of a workflow launch.
type runOptions = launch.Options

// preparedRun is a launch ready to start: its plan can be shown (recap).
type preparedRun = launch.Prepared

// prepareWorkflowRun resolves, compiles and plans a launch. What the
// preparation tells goes to opts.Progress, else to errOut.
func prepareWorkflowRun(ctx context.Context, a *app.App, opts runOptions, errOut io.Writer) (*preparedRun, error) {
	if opts.Progress == nil {
		opts.Progress = launchProgress(nil, errOut)
	}
	return launch.Prepare(ctx, launchDeps(a), opts)
}

// planDirty reports whether a writing session of the plan starts in a
// directory with uncommitted changes (whatever the choice made about it).
func planDirty(plan *runsvc.RunPlan) bool {
	for _, w := range plan.Warnings {
		switch w.Code {
		case runsvc.WarnDirty, runsvc.WarnDirtyWorktree, runsvc.WarnDirtyStash:
			return true
		}
	}
	return false
}

// warningText localizes a plan warning.
func warningText(w runsvc.Warning) string {
	return i18n.Tf("cmd.run.warn."+w.Code, w.Args...)
}

// startRun runs the prepared launch and opens the sessions (attach
// preference). Single-session browser / suspend attaches are handled here.
func startRun(ctx context.Context, a *app.App, p *preparedRun, ui launcher.LaunchUI) ([]*runsvc.StartResult, error) {
	return launch.Start(ctx, p, launch.StartOptions{
		Progress: uiProgress(ui),
		Attach: func(ctx context.Context, svc *runsvc.Service, attach sessionspec.AttachPref, res *runsvc.StartResult, first bool) error {
			return afterStart(ctx, a, svc, ui, attach, res, first)
		},
	})
}

// afterStart opens the client of a started session when the RunService did
// not (browser, suspension, no terminal). Only the first session of a run
// may take over the current terminal.
func afterStart(ctx context.Context, a *app.App, svc *runsvc.Service, ui launcher.LaunchUI, attach sessionspec.AttachPref, res *runsvc.StartResult, first bool) error {
	if first && inProcessDaemonRunning() {
		// Windows: the proxy lives in this process; leaving oh puts the
		// session to sleep.
		ui.Notify(i18n.T("cmd.daemon.inprocess_notice"), launcher.LevelWarning)
	}
	switch {
	case attach == sessionspec.AttachNone && res.Existing != "":
		ui.Notify(i18n.Tf("cmd.run.existing.continued."+string(res.Existing), res.SessionID), launcher.LevelSuccess)
	case attach == sessionspec.AttachNone:
		ui.Notify(i18n.Tf("cmd.run.started", res.SessionID), launcher.LevelSuccess)
	case attach == sessionspec.AttachBrowser:
		url, err := svc.PairURL(ctx, res.SessionID, pairURL)
		if err != nil {
			return err
		}
		ui.Notify(i18n.Tf("cmd.v5.opened_browser", url), launcher.LevelSuccess)
		return openURL(url)
	case attach == sessionspec.AttachSuspend || res.AttachErr != nil:
		if res.AttachErr != nil {
			ui.Notify(i18n.T("cmd.session.no_terminal"), launcher.LevelWarning)
		}
		if !first || (ui.SuspendAndExec() == nil && !stdinIsTerminal()) {
			// No terminal to take over (oh run from a script, an IDE…): the
			// client would block this process.
			ui.Notify(i18n.Tf("cmd.run.attach_later", res.SessionID), launcher.LevelInfo)
			return nil
		}
		err := runAttachInline(ctx, a, svc, ui, res.SessionID)
		if err == nil && first {
			// R5: the client is closed; propose what follows (never launched).
			printNextSteps(ctx, a, os.Stdout, res.SessionID)
		}
		return err
	default:
		ui.Notify(i18n.Tf("cmd.v5.opened_in", string(res.AttachMethod), res.SessionID), launcher.LevelSuccess)
	}
	return nil
}

// runRecap is what a launch will do: labelled rows and warnings (oh run
// --recap, TUI launch form).
func runRecap(p *preparedRun) (rows [][2]string, warnings []string) {
	rows, warnings = launchRecap(p)
	rows = append(rows, existingRecapRows(p.Plan)...)
	if p.Plan.NeedsChoice {
		warnings = append(warnings, launch.ExistingChoiceMessage(p.Plan.Existing))
	}
	return rows, warnings
}

// launchRecap is the recap without the existing sessions of the tickets
// (the TUI shows them as a comparison block).
func launchRecap(p *preparedRun) (rows [][2]string, warnings []string) {
	r := bundle.Show(p.Bundle)
	add := func(label, value string) { rows = append(rows, [2]string{label, value}) }
	add(i18n.T("cmd.run.recap.mode"), p.Resolution.Mode)
	add(i18n.T("cmd.run.recap.runtime"), string(p.Resolution.Runtime))
	agents := make([]string, len(r.Agents))
	for i, ag := range r.Agents {
		agents[i] = ag.ID
		if ag.Entry {
			agents[i] += " (" + i18n.T("cmd.bundle.show.entry") + ")"
		}
	}
	add(i18n.Tf("cmd.bundle.show.agents", len(r.Agents)), strings.Join(agents, " · "))
	add(i18n.Tf("cmd.bundle.show.skills", len(r.Skills)), i18n.Tf("cmd.bundle.show.initial_value", r.Budget.Initial, r.Budget.EntryAgent, r.Budget.SkillCatalog))
	if len(r.MCP) > 0 {
		add("MCP", strings.Join(r.MCP, ", "))
	}
	add(i18n.T("cmd.run.recap.code_mode"), onOff(r.CodeMode))
	add(i18n.T("cmd.run.recap.beads"), recapBeads(p.Plan.Request.Base.BeadsAllow))
	if p.Bundle.Spec.Workflow != nil && len(p.Bundle.Spec.Workflow.Checkpoints) > 0 {
		add(i18n.T("cmd.run.recap.checkpoints"), recapCheckpoints(p.Bundle.Spec.Workflow.Checkpoints, p.Resolution.Mode))
		add(i18n.T("cmd.run.recap.pauses"), recapPauses(p.Bundle.Spec.Workflow, p.Resolution.Mode))
	}
	isolation := string(r.Isolation)
	if r.StrictIsolation {
		isolation += " · strict"
	}
	add(i18n.T("cmd.bundle.show.isolation"), isolation)
	for _, e := range p.Opts.Epics {
		add(i18n.T("cmd.run.recap.epic"), e.Epic+" → "+strings.Join(e.Children, " · "))
	}
	add(i18n.T("cmd.run.recap.sessions"), i18n.Tf("cmd.run.recap.sessions_value", len(p.Plan.Sessions), p.Plan.Worktrees()))
	for _, s := range p.Plan.Sessions {
		loc := s.Location.Path
		switch {
		case s.Location.Create:
			loc += " (" + i18n.Tf("cmd.run.recap.new_worktree", s.Location.Branch) + ")"
		case s.Location.Kind == runsvc.LocationBase:
			loc += " (" + i18n.T("cmd.run.recap.base") + ")"
		}
		label := s.Label
		if label == "" {
			label = theme.IconArrow
		}
		if s.Action != runsvc.OnExistingUnset {
			loc += " · " + i18n.Tf("cmd.run.recap.on_existing", i18n.T("cmd.run.existing.choice."+string(s.Action)), s.Existing)
		}
		add("  "+label, loc)
	}
	warnings = append(warnings, remoteRecapWarnings(p)...)
	for _, w := range p.Plan.Warnings {
		warnings = append(warnings, warningText(w))
	}
	return rows, warnings
}

// recapBeads describes the Beads commands a session may run (beads.allow;
// not declared = the read-only default).
func recapBeads(allow []string) string {
	switch {
	case allow == nil:
		return i18n.T("cmd.run.recap.beads_default")
	case len(allow) == 0:
		return i18n.T("cmd.run.recap.beads_none")
	}
	return strings.Join(allow, " · ")
}

// recapCheckpoints describes the checkpoints in the session mode: what each
// one does, and which ones are mandatory.
func recapCheckpoints(cps []sessionspec.CheckpointDef, mode string) string {
	parts := make([]string, len(cps))
	for i, c := range cps {
		s := c.ID + " " + i18n.T("cmd.run.recap.cp."+c.Behavior(mode))
		if c.Mandatory {
			s += " (" + i18n.T("cmd.run.recap.cp.mandatory") + ")"
		}
		parts[i] = s
	}
	return strings.Join(parts, " · ")
}

// printRunRecap prints what a launch will do (oh run --recap).
func printRunRecap(w io.Writer, p *preparedRun) {
	fmt.Fprintln(w, theme.Title.Render(i18n.Tf("cmd.run.recap.title", p.Resolution.Spec.ID, p.Resolution.Ref.String(), p.Project.Name)))
	rows, warns := runRecap(p)
	for _, r := range rows {
		fmt.Fprintf(w, "  %-14s %s\n", r[0], r[1])
	}
	for _, wr := range warns {
		fmt.Fprintf(w, "  %s %s\n", theme.WarningStyle.Render(theme.IconWarning), wr)
	}
}

func printRunWarnings(w io.Writer, warns []runsvc.Warning) {
	for _, wr := range warns {
		fmt.Fprintf(w, "  %s %s\n", theme.WarningStyle.Render(theme.IconWarning), warningText(wr))
	}
}

// ProviderCredentialError is a launch refused for want of credentials of
// its provider (the TUI shows it in the recap of the launch form).
type ProviderCredentialError struct{ Detail string }

func (e *ProviderCredentialError) Error() string {
	return i18n.Tf("cmd.run.provider.missing", e.Detail)
}

// launchCredentials refuses a launch whose provider has no credentials
// (project, team, hub, AWS profile for Bedrock).
var launchCredentials = func(ctx context.Context, a *app.App, project *domain.Project, providerFlag string) error {
	if detail, ok := credentialStatus(ctx, a, project, providerFlag); !ok {
		return &ProviderCredentialError{Detail: detail}
	}
	return nil
}
