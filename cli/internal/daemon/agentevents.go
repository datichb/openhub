package daemon

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
)

// Agent telemetry (QB2): one agent_events row per tool session, i.e. the
// entry agent of an oh session and every subagent it delegates to, kept
// up to date at the end of each step (status, duration, tokens, cost of the
// tool session, skills loaded). It feeds the per-agent table of oh metrics,
// oh serve and the Metrics view.

type agentRun struct {
	started time.Time
	skills  []string
	agent   string
}

// onAgentActivity follows the agent of toolSession (root itself, or one of
// its subagent sessions) for the agent telemetry.
func (w *watcher) onAgentActivity(ctx context.Context, root, toolSession string, ev adapters.ToolEvent) {
	if w.d.opts.AgentEvents == nil {
		return
	}
	w.mu.Lock()
	if w.runs == nil {
		w.runs = map[string]*agentRun{}
	}
	r := w.runs[toolSession]
	if r == nil {
		r = &agentRun{started: eventTime(ev)}
		w.runs[toolSession] = r
	}
	if ev.Feed != nil && ev.Feed.Kind == domain.FeedAgent && ev.Feed.Agent != "" {
		r.agent = ev.Feed.Agent
	}
	if c := ev.Call; c != nil && c.Skill != "" && c.Status == adapters.CallCalled && !slices.Contains(r.skills, c.Skill) {
		r.skills = append(r.skills, c.Skill)
	}
	run := *r
	run.skills = slices.Clone(r.skills)
	w.mu.Unlock()
	if ev.Kind == adapters.EventExecEnded {
		w.saveAgentRun(ctx, root, toolSession, run, ev)
	}
}

func eventTime(ev adapters.ToolEvent) time.Time {
	if !ev.Time.IsZero() {
		return ev.Time
	}
	return time.Now()
}

// saveAgentRun writes the agent_events row of a tool session (ID = the tool
// session; created at its first step end, updated at the next ones).
func (w *watcher) saveAgentRun(ctx context.Context, root, toolSession string, run agentRun, ev adapters.ToolEvent) {
	store := w.d.opts.AgentEvents
	sess, err := w.d.opts.Sessions.Get(ctx, root)
	if err != nil {
		return
	}
	agent := run.agent
	if agent == "" && toolSession == root {
		agent = sess.EntryAgent
	}
	if agent == "" {
		return // unknown agent: nothing to aggregate on
	}
	end := eventTime(ev)
	e := &domain.AgentEvent{
		ID: toolSession, SessionID: root, ProjectID: sess.ProjectID, AgentName: agent,
		SkillsLoaded: run.skills, StartedAt: run.started, CompletedAt: &end,
		Status: agentStatus(ev.Outcome), MemberID: sess.MemberID,
	}
	if e.SkillsLoaded == nil {
		e.SkillsLoaded = []string{}
	}
	if res, err := w.usage(ctx, toolSession); err == nil {
		e.TokensIn, e.TokensOut, e.CostUSD = res.TokensIn, res.TokensOut, res.Cost
	}
	if e.Status == domain.AgentEventFailed {
		e.ErrorMessage = ev.Outcome
	}
	err = store.Update(ctx, e)
	if errors.Is(err, domain.ErrNotFound) {
		err = store.Create(ctx, e)
	}
	if err != nil {
		slog.Debug("ohd: agent event not saved", "session", root, "tool_session", toolSession, "error", err)
	}
}

func agentStatus(outcome string) domain.AgentEventStatus {
	switch outcome {
	case "succeeded":
		return domain.AgentEventSuccess
	case "failed":
		return domain.AgentEventFailed
	}
	return domain.AgentEventCancelled
}
