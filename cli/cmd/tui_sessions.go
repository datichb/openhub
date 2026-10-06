package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/launcher"
	"github.com/datichb/openhub/cli/internal/runsvc"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	"github.com/datichb/openhub/cli/internal/termlaunch"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// TUI wiring of the v5 sessions (P3-T16–T19): maps the SessionService and
// the RunService to the Sessions view, keeps the cached summary used by the
// landings and the mode bar badge, and toasts new decisions. No logic here
// beyond presentation: decisions, instructions and lifecycle are services.

const tuiSessionsSummaryLines = 3

// tuiSess is the sessions wiring of the running TUI.
var tuiSess *tuiSessions

type tuiSessions struct {
	a    *app.App
	view *views.SessionsView

	mu       sync.Mutex
	svc      *sessionsvc.Service
	cache    []sessionsvc.View
	teams    map[string]string // project id → team id
	seen     map[string]bool   // decision ids already toasted
	primed   bool
	sig      string // summary signature shown on the landings
	watchers map[chan struct{}]struct{}
	ends     map[string]string // session id → end signature (state, outputs), « Enchaîner avec… »
}

func newTUISessions(a *app.App) *tuiSessions {
	t := &tuiSessions{a: a, teams: map[string]string{}, seen: map[string]bool{}, watchers: map[chan struct{}]struct{}{},
		ends: map[string]string{}}
	t.view = views.NewSessionsView(views.SessionsViewConfig{Backend: t})
	return t
}

// service returns the SessionService (created on first use, off the loop).
func (t *tuiSessions) service(ctx context.Context) (*sessionsvc.Service, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.svc != nil {
		return t.svc, nil
	}
	svc, err := newSessionService(ctx, t.a)
	if err != nil {
		return nil, err
	}
	t.svc = svc
	return svc, nil
}

// sectionConfig is the Sessions section of the landings.
func (t *tuiSessions) sectionConfig() views.SessionsSectionConfig {
	return views.SessionsSectionConfig{Summary: t.summary, Open: t.open}
}

// open shows the Sessions view (focused on a session).
func (t *tuiSessions) open(sessionID string) {
	if tuiShell == nil {
		return
	}
	t.view.Focus(sessionID)
	tuiShell.NavigateTo(t.view.ID())
}

// start runs the change subscriber until ctx ends: cache, badge, toasts.
func (t *tuiSessions) start(ctx context.Context) {
	go func() {
		if !v5Available(ctx) {
			return
		}
		svc, err := t.service(ctx)
		if err != nil {
			slog.Debug("TUI sessions unavailable", "error", err)
			return
		}
		t.refresh(ctx)
		changes := svc.Subscribe(ctx)
		var timer *time.Timer
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-changes:
				if !ok {
					return
				}
				if timer != nil {
					timer.Stop()
				}
				timer = time.AfterFunc(300*time.Millisecond, func() { t.refresh(ctx) })
			}
		}
	}()
}

// refresh reloads the cache and pushes the badge, toasts and change signals.
func (t *tuiSessions) refresh(ctx context.Context) {
	svc, err := t.service(ctx)
	if err != nil {
		return
	}
	list, err := svc.List(ctx, sessionsvc.ListFilter{})
	if err != nil {
		return
	}
	running, decisions := 0, 0
	var fresh []domain.Decision
	t.mu.Lock()
	for _, v := range list {
		if _, ok := t.teams[v.Session.ProjectID]; !ok {
			t.teams[v.Session.ProjectID] = t.teamOf(ctx, v.Session.ProjectID)
		}
		if v.Session.State != domain.RunSleeping {
			running++
		}
		decisions += len(v.Decisions)
		for _, d := range v.Decisions {
			if !t.seen[d.ID] {
				t.seen[d.ID] = true
				if t.primed {
					fresh = append(fresh, d)
				}
			}
		}
	}
	ended := t.endedSessions(list)
	t.primed = true
	t.cache = list
	sig := summarySignature(list)
	changed := sig != t.sig
	t.sig = sig
	for ch := range t.watchers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	t.mu.Unlock()

	for _, s := range ended {
		go t.sessionEnded(ctx, s)
	}
	sh := tuiShell
	if sh == nil {
		return
	}
	sh.App().QueueUpdateDraw(func() {
		sh.SetSessionsBadge(views.SessionsBadge(running, decisions), decisions > 0)
		if changed {
			sh.RemountIf("home", "project.mode", "team.mode")
		}
		switch len(fresh) {
		case 0:
		case 1:
			sh.ShowToast(i18n.Tf("tui.notify.toast_one", sessionsvc.KindIcon(fresh[0].Kind), sessionsvc.DecisionSummary(fresh[0])), shell.ToastWarning)
		default:
			sh.ShowToast(i18n.Tf("tui.notify.toast_many", len(fresh)), shell.ToastWarning)
		}
	})
}

// summarySignature changes when what the landings show changes.
func summarySignature(list []sessionsvc.View) string {
	var b strings.Builder
	for _, v := range list {
		fmt.Fprintf(&b, "%s:%s:%d;", v.Session.ID, v.Session.State, len(v.Decisions))
	}
	return b.String()
}

func (t *tuiSessions) teamOf(ctx context.Context, projectID string) string {
	if t.a.Projects == nil || projectID == "" {
		return ""
	}
	p, err := t.a.Projects.Get(ctx, projectID)
	if err != nil {
		return ""
	}
	if r := config.ResolveTeamForProject(t.a.Config, p); r.Enabled {
		return r.TeamID
	}
	return ""
}

// summary returns the cached summary of a scope (no I/O: event loop safe).
func (t *tuiSessions) summary(scope views.SessionsScope) views.SessionsSummary {
	t.mu.Lock()
	defer t.mu.Unlock()
	var sum views.SessionsSummary
	var keep []sessionsvc.View
	for _, v := range t.cache {
		if scope.ProjectID != "" && v.Session.ProjectID != scope.ProjectID {
			continue
		}
		if scope.TeamID != "" && t.teams[v.Session.ProjectID] != scope.TeamID {
			continue
		}
		if v.Session.State != domain.RunSleeping {
			sum.Running++
		}
		sum.Decisions += len(v.Decisions)
		keep = append(keep, v)
	}
	sort.SliceStable(keep, func(i, j int) bool { return statePriority(keep[i]) < statePriority(keep[j]) })
	for i, v := range keep {
		if i == tuiSessionsSummaryLines {
			break
		}
		desc := stateLabel(v.Session.State)
		if len(v.Decisions) > 0 {
			desc = sessionsvc.KindIcon(v.Decisions[0].Kind) + " " + sessionsvc.DecisionSummary(v.Decisions[0])
		}
		label := firstNonEmpty(v.Session.WorkflowID, v.Session.EntryAgent)
		if v.ProjectName != "" && scope.ProjectID == "" {
			label += " · " + v.ProjectName
		}
		sum.Lines = append(sum.Lines, views.SessionLine{ID: v.Session.ID, Icon: sessionsvc.StateIcon(v.Session.State), Label: label, Desc: desc})
	}
	return sum
}

func statePriority(v sessionsvc.View) int {
	switch {
	case len(v.Decisions) > 0:
		return 0
	case v.Session.State == domain.RunActive:
		return 1
	case v.Session.State == domain.RunSleeping:
		return 3
	}
	return 2
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

// ── views.SessionsBackend ─────────────────────────────────────────────────

var _ views.SessionsBackend = (*tuiSessions)(nil)

func (t *tuiSessions) List(ctx context.Context, projectID string, all bool) ([]views.SessionRow, error) {
	svc, err := t.service(ctx)
	if err != nil {
		return nil, err
	}
	list, err := svc.List(ctx, sessionsvc.ListFilter{ProjectID: projectID, All: all})
	if err != nil {
		return nil, err
	}
	rows := make([]views.SessionRow, 0, len(list))
	for _, v := range list {
		s := v.Session
		r := views.SessionRow{
			ID: s.ID, ProjectID: s.ProjectID, Project: v.ProjectName, Workflow: s.WorkflowID, Agent: s.EntryAgent,
			Mode: s.Mode, Runtime: s.Runtime, Location: s.LaunchPath, Bundle: s.BundleHash, State: string(s.State),
			StateLabel: stateLabel(s.State), StateIcon: sessionsvc.StateIcon(s.State), Cost: s.Cost, Started: s.StartedAt,
			Finished: !v.Open(), Next: chainNext(s.ID),
		}
		if s.Title != nil {
			r.Title = *s.Title
		}
		if s.StateChangedAt != nil {
			r.Changed = *s.StateChangedAt
		} else if s.EndedAt != nil {
			r.Changed = *s.EndedAt
		}
		for _, d := range v.Decisions {
			r.Decisions = append(r.Decisions, toViewDecision(d))
		}
		r.Timeline = sessionTimeline(ctx, svc, s.ID)
		rows = append(rows, r)
	}
	return t.decorateRemote(ctx, rows), nil
}

func toViewDecision(d domain.Decision) views.SessionDecision {
	out := views.SessionDecision{ID: d.ID, SessionID: d.SessionID, Kind: string(d.Kind), Icon: sessionsvc.KindIcon(d.Kind),
		Summary: sessionsvc.DecisionSummary(d), Message: d.Payload.Message, Created: d.CreatedAt}
	for _, f := range d.Payload.Fields {
		vf := views.SessionDecisionField{Key: f.Key, Title: f.Title, Description: f.Description, Type: f.Type, Custom: f.Custom, Required: f.Required}
		for _, o := range f.Options {
			vf.Options = append(vf.Options, views.SelectOption{Label: firstNonEmpty(o.Label, o.Value), Value: o.Value})
		}
		out.Fields = append(out.Fields, vf)
	}
	return out
}

func (t *tuiSessions) Changes(ctx context.Context) <-chan struct{} {
	ch := make(chan struct{}, 1)
	t.mu.Lock()
	t.watchers[ch] = struct{}{}
	t.mu.Unlock()
	go func() {
		<-ctx.Done()
		t.mu.Lock()
		delete(t.watchers, ch)
		t.mu.Unlock()
	}()
	return ch
}

func (t *tuiSessions) Follow(ctx context.Context, sessionID string) (<-chan views.SessionFeedLine, error) {
	svc, err := t.service(ctx)
	if err != nil {
		return nil, err
	}
	feed, err := svc.Follow(ctx, sessionID)
	if errors.Is(err, sessionsvc.ErrNoLive) {
		return nil, errors.New(i18n.T("tui.sessions.feed_unavailable"))
	}
	if err != nil {
		return nil, err
	}
	out := make(chan views.SessionFeedLine, 64)
	go func() {
		defer close(out)
		for it := range feed {
			line := views.SessionFeedLine{Time: it.Time, Text: sessionsvc.FeedLine(it)}
			if it.Kind == domain.FeedUsage {
				line.Cost = it.Cost
			}
			select {
			case out <- line:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

func (t *tuiSessions) Decide(ctx context.Context, decisionID, choice, message string, answers map[string]string) error {
	svc, err := t.service(ctx)
	if err != nil {
		return err
	}
	reply := sessionsvc.Reply{DecisionID: decisionID, Decision: choice, Message: message}
	if answers != nil {
		d, err := svc.Decisions.Get(ctx, decisionID)
		if err != nil {
			return err
		}
		if reply.Answer, err = sessionsvc.ParseAnswers(d.Payload.Fields, answers); err != nil {
			return err
		}
	}
	return decisionError(svc.Decide(ctx, reply))
}

func (t *tuiSessions) Send(ctx context.Context, sessionID, text string) error {
	svc, err := t.service(ctx)
	if err != nil {
		return err
	}
	return decisionError(svc.Send(ctx, sessionID, text, sessionsvc.SendOptions{}))
}

func (t *tuiSessions) Interrupt(ctx context.Context, sessionID string) error {
	svc, err := t.service(ctx)
	if err != nil {
		return err
	}
	return decisionError(svc.Interrupt(ctx, sessionID))
}

func (t *tuiSessions) SwitchModel(ctx context.Context, sessionID, model string) error {
	svc, err := t.service(ctx)
	if err != nil {
		return err
	}
	return decisionError(svc.SwitchModel(ctx, sessionID, model))
}

func (t *tuiSessions) Stop(ctx context.Context, sessionID string) error {
	rs, err := newRunService(ctx, t.a)
	if err != nil {
		return err
	}
	return rs.StopSession(ctx, sessionID)
}

func (t *tuiSessions) Resume(ctx context.Context, sessionID string) error {
	rs, err := newRunService(ctx, t.a)
	if err != nil {
		return err
	}
	return resumeV5Session(ctx, t.a, rs, sessionID)
}

// Attach opens the session client off the event loop: a new terminal tab or
// window (or tmux), else the current terminal (the TUI is suspended).
func (t *tuiSessions) Attach(sessionID, how string) {
	sh := tuiShell
	if sh == nil {
		return
	}
	toast := func(msg string, ok bool) { sh.App().QueueUpdateDraw(func() { sh.ShowToastMsg(msg, ok) }) }
	go func() {
		ctx := context.Background()
		rs, err := newRunService(ctx, t.a)
		if err != nil {
			toast(err.Error(), false)
			return
		}
		if how == "" {
			how = attachPreference(t.a)
		}
		if _, _, err := rs.AttachCommand(ctx, sessionID); errors.Is(err, runsvc.ErrServerNotRunning) {
			toast(i18n.T("cmd.session.resuming"), true)
			if err := resumeV5Session(ctx, t.a, rs, sessionID); err != nil {
				toast(err.Error(), false)
				return
			}
		}
		ui := launcher.NewTUIUI(func(fn func() error) error {
			done := make(chan error, 1)
			sh.App().QueueUpdateDraw(func() { done <- sh.SuspendAndExec(fn) })
			return <-done
		}, toast)
		switch how {
		case "browser":
			url, err := rs.PairURL(ctx, sessionID, pairURL)
			if err == nil {
				err = openURL(url)
			}
			if err != nil {
				toast(err.Error(), false)
			}
			return
		case "suspend":
			if err := runAttachInline(ctx, t.a, rs, ui, sessionID); err != nil {
				toast(err.Error(), false)
			}
			return
		}
		dir := ""
		if sess, err := rs.Session(ctx, sessionID); err == nil {
			dir = sess.LaunchPath
		}
		m, err := rs.Attach(ctx, sessionID, dir, termlaunch.Pref(how), termlaunch.ITermStyle(t.a.Config.Session.ITermStyle), "")
		if errors.Is(err, termlaunch.ErrNoTerminal) {
			err = runAttachInline(ctx, t.a, rs, ui, sessionID)
		} else if err == nil {
			toast(i18n.Tf("cmd.session.opened_in", string(m)), true)
		}
		if err != nil {
			toast(err.Error(), false)
		}
	}()
}

func (t *tuiSessions) OpenBrowser(ctx context.Context, sessionID string) (string, error) {
	rs, err := newRunService(ctx, t.a)
	if err != nil {
		return "", err
	}
	if _, _, err := rs.AttachCommand(ctx, sessionID); errors.Is(err, runsvc.ErrServerNotRunning) {
		if err := resumeV5Session(ctx, t.a, rs, sessionID); err != nil {
			return "", err
		}
	}
	url, err := rs.PairURL(ctx, sessionID, pairURL)
	if err != nil {
		return "", err
	}
	return url, openURL(url)
}

func (t *tuiSessions) MRDescription(ctx context.Context, sessionID string) (string, error) {
	svc, err := t.service(ctx)
	if err != nil {
		return "", err
	}
	r, err := svc.Results(ctx, sessionID)
	if err != nil {
		return "", err
	}
	md := sessionsvc.MRDescription(r)
	if !r.Live {
		md += "\n" + i18n.T("cmd.session.results_snapshot")
	}
	return strings.TrimRight(md, "\n"), nil
}
