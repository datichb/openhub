package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/filelock"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/services/fanout"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// Fan-out (R6, E3, experimental, `[fanout] experimental = true`): a
// session launched with the fan-out input (`oh run implement-spec -i
// fanout=true`) gets one child session per ticket, each in its own
// worktree, merged by oh into an integration branch. The driver runs in an
// oh process: `oh run` starts it in the background (`oh fanout drive`), an
// evaluation pass in its own process.

func init() {
	fanoutCmd := &cobra.Command{Use: "fanout", Short: i18n.T("cmd.fanout.short")}
	drive := &cobra.Command{
		Use:   "drive <session-id>",
		Short: i18n.T("cmd.fanout.drive.short"),
		Args:  cobra.ExactArgs(1),
		RunE:  runFanoutDrive,
	}
	drive.Flags().Bool("relaunch", false, i18n.T("cmd.fanout.drive.flags.relaunch"))
	status := &cobra.Command{
		Use:   "status <session-id>",
		Short: i18n.T("cmd.fanout.status.short"),
		Args:  cobra.ExactArgs(1),
		RunE:  runFanoutStatus,
	}
	status.Flags().Bool("json", false, "Sortie JSON")
	clean := &cobra.Command{
		Use:   "clean <session-id>",
		Short: i18n.T("cmd.fanout.clean.short"),
		Args:  cobra.ExactArgs(1),
		RunE:  runFanoutClean,
	}
	clean.Flags().Bool("dry-run", false, i18n.T("cmd.fanout.clean.flags.dry_run"))
	clean.Flags().Bool("discard", false, i18n.T("cmd.fanout.clean.flags.discard"))
	clean.Flags().Bool("json", false, "Sortie JSON")
	fanoutCmd.AddCommand(drive, status, clean)
	rootCmd.AddCommand(fanoutCmd)
}

// fanoutEnabled reports whether the fan-out is enabled on this machine.
func fanoutEnabled(a *app.App) bool {
	return a != nil && a.Config != nil && a.Config.Fanout.Experimental
}

// fanoutParentRecord is the fan-out record of a parent at launch.
func fanoutParentRecord(a *app.App) *domain.SessionFanout {
	return &domain.SessionFanout{Role: domain.RoleParent, Experimental: true, Status: domain.FanoutWaiting,
		Parallel: a.Config.Fanout.EffectiveParallel(), BudgetUSD: a.Config.Fanout.EffectiveBudget()}
}

// checkFanoutLaunch refuses a fan-out launch the configuration or the
// runtime does not allow.
func checkFanoutLaunch(a *app.App, runtime sessionspec.RuntimeKind, perSession []string) error {
	switch {
	case !fanoutEnabled(a):
		return errors.New(i18n.T("cmd.fanout.disabled"))
	case runtime == sessionspec.RuntimeRemote || runtime == sessionspec.RuntimeContainer:
		return errors.New(i18n.Tf("cmd.fanout.local_only", string(runtime)))
	case len(perSession) > 0:
		return errors.New(i18n.T("cmd.fanout.one_session"))
	}
	return nil
}

func runFanoutDrive(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	a := MustApp()
	if !fanoutEnabled(a) {
		return errors.New(i18n.T("cmd.fanout.disabled"))
	}
	ssvc, err := sessionService(cmd)
	if err != nil {
		return err
	}
	sess, err := ssvc.Resolve(ctx, args[0])
	if err != nil {
		return sessionRefError(args[0], err)
	}
	d, err := newFanoutDriver(ctx, a, sess, cmd.ErrOrStderr(), nil)
	if err != nil {
		return err
	}
	unlock, err := filelock.TryLock(filepath.Join(ohSessionsDir(), sess.ID, "fanout.lock"))
	if err != nil {
		return errors.New(i18n.Tf("cmd.fanout.driven", sess.ID))
	}
	defer unlock()
	if relaunch, _ := cmd.Flags().GetBool("relaunch"); relaunch {
		if err := d.Relaunched(ctx, sess.ID); err != nil {
			return err
		}
	}
	if err := d.Run(ctx, sess.ID); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf("cmd.fanout.drive.done", sess.ID))
	return nil
}

func runFanoutStatus(cmd *cobra.Command, args []string) error {
	ssvc, err := sessionService(cmd)
	if err != nil {
		return err
	}
	sess, err := ssvc.Resolve(cmd.Context(), args[0])
	if err != nil {
		return sessionRefError(args[0], err)
	}
	f := sess.Fanout
	if f == nil {
		return errors.New(i18n.Tf("cmd.fanout.none", sess.ID))
	}
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		return printJSON(cmd, f)
	}
	printFanout(cmd.OutOrStdout(), sess.ID, f)
	return nil
}

// runFanoutClean removes the worktrees and branches oh/fanout/… of a
// fan-out merged or abandoned (--discard: the unmerged branches too, their
// commit traced).
func runFanoutClean(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	ssvc, err := sessionService(cmd)
	if err != nil {
		return err
	}
	sess, err := ssvc.Resolve(ctx, args[0])
	if err != nil {
		return sessionRefError(args[0], err)
	}
	if sess.Fanout == nil || sess.Fanout.Role != domain.RoleParent {
		return errors.New(i18n.Tf("cmd.fanout.none", sess.ID))
	}
	if !fanout.Cleanable(sess) {
		return errors.New(i18n.Tf("cmd.fanout.clean.not_done", sess.ID, string(sess.Fanout.Status)))
	}
	f := cmd.Flags()
	dry, _ := f.GetBool("dry-run")
	discard, _ := f.GetBool("discard")
	res, err := fanout.Clean(ctx, sqlite.NewSessionStore(store), sess, fanout.CleanOptions{DryRun: dry, Discard: discard})
	if err != nil {
		return err
	}
	if asJSON, _ := f.GetBool("json"); asJSON {
		return printJSON(cmd, res)
	}
	w := cmd.OutOrStdout()
	key := "cmd.fanout.clean.done"
	if dry {
		key = "cmd.fanout.clean.plan"
	}
	fmt.Fprintln(w, i18n.Tf(key, len(res.Worktrees), len(res.Branches)+len(res.Discarded)))
	for _, b := range res.Branches {
		fmt.Fprintln(w, "  - "+b)
	}
	for _, d := range res.Discarded {
		fmt.Fprintln(w, "  ✗ "+i18n.Tf("cmd.fanout.clean.discarded", d))
	}
	for _, k := range res.Kept {
		fmt.Fprintln(w, "  = "+k)
	}
	if len(res.Kept) > 0 && !discard {
		fmt.Fprintln(w, i18n.T("cmd.fanout.clean.kept_hint"))
	}
	return nil
}

// fanoutCleaner is the clean-up of the fan-outs for oh gc.
type fanoutCleaner struct{ store *sqlite.SessionStore }

func (c fanoutCleaner) Plan(ctx context.Context, s domain.Session) (what string, worktrees []string) {
	res, err := fanout.Clean(ctx, c.store, &s, fanout.CleanOptions{DryRun: true})
	if err != nil || res.Empty() {
		return "", nil
	}
	return i18n.Tf("cmd.fanout.clean.plan", len(res.Worktrees), len(res.Branches)), res.Worktrees
}

func (c fanoutCleaner) Clean(ctx context.Context, id string) error {
	s, err := c.store.Get(ctx, id)
	if err != nil {
		return err
	}
	_, err = fanout.Clean(ctx, c.store, s, fanout.CleanOptions{})
	return err
}

// cleanFanoutOnEnd cleans up the fan-out of a parent stopped for good (or
// merged), unless a session of it still runs: its driver does it then.
func cleanFanoutOnEnd(ctx context.Context, a *app.App, s domain.Session) {
	if store == nil || !fanout.Cleanable(&s) {
		return
	}
	for _, id := range fanoutChildren(ctx, a, s.ID) {
		if c, err := a.Sessions.Get(ctx, id); err == nil && !runsvcTerminal(c.State) {
			return
		}
	}
	if _, err := fanout.Clean(ctx, sqlite.NewSessionStore(store), &s, fanout.CleanOptions{}); err != nil {
		slog.Warn("fan-out: clean-up at the end of the parent", "session", s.ID, "error", err)
	}
}

func runsvcTerminal(st domain.RunState) bool {
	return st == domain.RunCompleted || st == domain.RunStopped || st == domain.RunFailed
}

// printFanout prints the state of a fan-out (parent) or the parent of a
// child.
func printFanout(w io.Writer, id string, f *domain.SessionFanout) {
	if f.Role != domain.RoleParent {
		fmt.Fprintln(w, i18n.Tf("cmd.fanout.status.child", id, string(f.Role), f.Parent, f.Ticket, f.Branch))
		return
	}
	fmt.Fprintln(w, i18n.Tf("cmd.fanout.status.head", id, string(f.Status)))
	if ig := f.Integration; ig != nil {
		fmt.Fprintln(w, i18n.Tf("cmd.fanout.status.integration", ig.Branch, ig.Path, shortHash(ig.Head)))
	}
	if f.BudgetUSD > 0 {
		fmt.Fprintln(w, i18n.Tf("cmd.fanout.status.budget", f.SpentUSD, f.BudgetUSD))
	}
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, i18n.T("cmd.fanout.status.header"))
	for _, c := range f.Children {
		detail := c.Error
		if detail == "" && len(c.Conflicts) > 0 {
			detail = strings.Join(c.Conflicts, ", ")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", fanout.ChildLabel(c), c.State, c.Session, shortHash(c.Merge), detail)
	}
	_ = tw.Flush()
	if n := len(f.Events); n > 0 {
		fmt.Fprintln(w, i18n.T("cmd.fanout.status.events"))
		for _, e := range f.Events[max(0, n-15):] {
			fmt.Fprintf(w, "  %s  %-11s %s %s %s\n", e.At.Local().Format("15:04:05"), e.Kind, e.Ticket, e.Session, e.Message)
		}
	}
}

// spawnFanoutDriver starts `oh fanout drive <id>` in the background (its
// output in ~/.oh/sessions/<id>/fanout.log): the children are launched
// once the parent passes cp-plan, whatever the client of the parent does.
func spawnFanoutDriver(sessionID string, extra ...string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(ohSessionsDir(), sessionID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	logPath := filepath.Join(dir, "fanout.log")
	logf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return "", err
	}
	defer logf.Close()
	c := exec.Command(exe, append([]string{"fanout", "drive", sessionID}, extra...)...)
	c.Stdout, c.Stderr, c.Stdin = logf, logf, nil
	c.Env = os.Environ()
	daemon.SetDetached(c)
	if err := c.Start(); err != nil {
		return "", err
	}
	_ = c.Process.Release()
	return logPath, nil
}

// fanoutLauncher launches the children and mergers of a fan-out as
// headless sessions of their workflow (fanout.Launcher).
type fanoutLauncher struct {
	a       *app.App
	project *domain.Project
	ssvc    *sessionsvc.Service
	rsvc    *runsvc.Service
	log     io.Writer
	// adjust changes a launch before it starts (evaluation: token cap).
	adjust func(*preparedRun)
}

func (l *fanoutLauncher) Start(ctx context.Context, req fanout.LaunchRequest) (string, error) {
	opts := runOptions{Workflow: req.Workflow, Project: l.project, Mode: req.Parent.Mode, Location: req.Dir,
		Attach: string(sessionspec.AttachNone), ParentSessionID: req.Parent.ID, Dirty: runsvc.DirtyAllow, OneSession: true,
		OnExisting: runsvc.OnExistingParallel, LooseInputs: req.Inputs}
	if req.Record.Role == domain.RoleChild && req.Ticket != "" && req.Task == nil {
		opts.Tickets = []string{req.Ticket} // a task (sweep-fanout) has no ticket
	}
	if req.Record.Role == domain.RoleMerger {
		opts.Mode = "" // the mode of its workflow (auto: nobody to answer)
	}
	p, err := prepareWorkflowRun(ctx, l.a, opts, l.log)
	if err != nil {
		return "", err
	}
	base := &p.plan.Request.Base
	base.Attach, base.Headless, base.Fanout = sessionspec.AttachNone, true, req.Record
	note := ""
	switch req.Record.Role {
	case domain.RoleChild:
		note = i18n.Tf("cmd.fanout.child_note", req.Parent.ID, req.Record.Branch)
		if req.Task != nil {
			note = i18n.Tf("cmd.fanout.child_note_task", req.Parent.ID, req.Record.Branch, req.Task.ID, strings.Join(req.Task.Files, ", "))
		}
	case domain.RoleFixer:
		note = i18n.Tf("cmd.fanout.fixer_note", req.Parent.ID, req.Record.Branch)
	}
	if note != "" {
		// What changes for a child or a fixer, told by oh after its prompt.
		for i := range p.plan.Sessions {
			p.plan.Sessions[i].Prompt += "\n\n" + note
		}
	}
	if req.BudgetUSD > 0 && (base.Limits.SessionBudgetUSD == 0 || base.Limits.SessionBudgetUSD > req.BudgetUSD) {
		base.Limits.SessionBudgetUSD = req.BudgetUSD // F-04: what remains of the global budget
	}
	if l.adjust != nil {
		l.adjust(p)
	}
	results, err := p.svc.Start(ctx, p.plan)
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return "", errors.New("no session started")
	}
	return results[0].SessionID, nil
}

func (l *fanoutLauncher) Status(ctx context.Context, id string) (fanout.SessionStatus, error) {
	sess, err := l.ssvc.Sessions.Get(ctx, id)
	if err != nil {
		return fanout.SessionStatus{}, err
	}
	switch sess.State {
	case domain.RunCompleted, domain.RunStopped, domain.RunFailed:
		return fanout.SessionStatus{Ended: true}, nil
	case domain.RunIdle:
		items, err := l.ssvc.Inbox(ctx, domain.DecisionFilter{SessionID: id})
		return fanout.SessionStatus{Idle: err == nil && len(items) == 0}, nil
	}
	return fanout.SessionStatus{}, nil
}

func (l *fanoutLauncher) Stop(ctx context.Context, id string) error {
	// The transcript of a child stays readable in the trace once stopped.
	if err := l.ssvc.SaveTranscript(ctx, id); err != nil {
		fmt.Fprintf(l.log, "fan-out: transcript of %s not saved: %v\n", id, err)
	}
	return l.rsvc.StopSession(ctx, id)
}

func (l *fanoutLauncher) Tell(ctx context.Context, id, text string) error {
	err := l.ssvc.Send(ctx, id, text, sessionsvc.SendOptions{})
	if err == nil {
		return nil
	}
	// The server of the parent slept while the children ran: wake it.
	if werr := resumeV5Session(ctx, l.a, l.rsvc, id); werr != nil {
		return errors.Join(err, werr)
	}
	return l.ssvc.Send(ctx, id, text, sessionsvc.SendOptions{})
}

// newFanoutDriver wires the driver of the fan-out of sess.
func newFanoutDriver(ctx context.Context, a *app.App, sess *domain.Session, log io.Writer, adjust func(*preparedRun)) (*fanout.Driver, error) {
	project, err := a.Projects.Get(ctx, sess.ProjectID)
	if err != nil {
		return nil, err
	}
	ssvc, err := newSessionService(ctx, a)
	if err != nil {
		return nil, err
	}
	rsvc, err := newRunService(ctx, a)
	if err != nil {
		return nil, err
	}
	cps := newCheckpointService(a)
	l := &fanoutLauncher{a: a, project: project, ssvc: ssvc, rsvc: rsvc, log: log, adjust: adjust}
	return &fanout.Driver{
		Sessions: sqlite.NewSessionStore(store),
		Launcher: l,
		Parallel: a.Config.Fanout.EffectiveParallel(),
		Workflow: func(ctx context.Context, id string) (*sessionspec.WorkflowRuntime, domain.CheckpointState, error) {
			wf, _, err := cps.Workflow(ctx, id)
			if err != nil {
				return nil, domain.CheckpointState{}, err
			}
			st, err := cps.State(ctx, id)
			return wf, st, err
		},
		Log: func(format string, args ...any) {
			fmt.Fprintf(log, time.Now().Format("15:04:05 ")+format+"\n", args...)
		},
	}, nil
}

// fanoutLabel marks the sessions of a fan-out in the lists (CLI, TUI):
// "↳ " before a child or a merger, with its ticket; "⑂" after a parent.
func fanoutLabel(s *domain.Session, label string) string {
	if s == nil || s.Fanout == nil {
		return label
	}
	switch s.Fanout.Role {
	case domain.RoleParent:
		return label + " ⑂"
	case domain.RoleMerger:
		return "↳ " + label + " (merge " + s.Fanout.Ticket + ")"
	case domain.RoleFixer:
		return "↳ " + label + " (" + i18n.T("cmd.fanout.fixer_label") + ")"
	default:
		return "↳ " + label + " (" + s.Fanout.Ticket + ")"
	}
}

// fanoutRunning reports whether the fan-out of a session still has
// children to run (an evaluation does not settle meanwhile).
func fanoutRunning(ctx context.Context, a *app.App, id string) bool {
	sess, err := a.Sessions.Get(ctx, id)
	if err != nil || sess.Fanout == nil || sess.Fanout.Role != domain.RoleParent {
		return false
	}
	// Waiting: the children start once the parent passes cp-plan, which
	// only the parent does; an idle parent then is done.
	return sess.Fanout.Status == domain.FanoutRunning || sess.Fanout.Status == domain.FanoutFixing
}

// fanoutChildren returns the sessions of the fan-out of a parent (children,
// mergers and fixers).
func fanoutChildren(ctx context.Context, a *app.App, id string) []string {
	sess, err := a.Sessions.Get(ctx, id)
	if err != nil || sess.Fanout == nil {
		return nil
	}
	return fanout.Sessions(sess.Fanout)
}
