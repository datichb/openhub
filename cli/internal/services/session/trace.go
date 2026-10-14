package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Session trace (E11, P0.4): everything a session did, one event per line
// (JSONL), for evaluations and retrospectives. Sources: the session row, the
// decisions (permissions granted or refused, questions, checkpoints,
// budget), the checkpoint timeline kept by the oh daemon, the usage ledger,
// and the tool transcript (prompts, texts, tool calls, subagent sessions),
// read live from the tool server or from the last snapshot saved under
// SessionsDir. Secrets are masked in every string.

// Trace event kinds.
const (
	TraceSession    = "session"    // first event: what the session is
	TracePrompt     = "prompt"     // a message given to an agent
	TraceText       = "text"       // an agent wrote a message
	TraceTool       = "tool"       // a tool call
	TraceSkill      = "skill"      // a skill was loaded
	TraceSubagent   = "subagent"   // a delegation to a subagent
	TraceDecision   = "decision"   // a decision asked (permission, question, checkpoint…) and how it was answered
	TraceCheckpoint = "checkpoint" // an entry of the checkpoint timeline
	TraceUsage      = "usage"      // cost and tokens of one model turn
	TraceEnd        = "end"        // an agent loop ended
	TraceOutput     = "output"     // a typed output declared by the session (or recorded by oh)
	TraceNext       = "next"       // a workflow oh proposes after the session (R5, `next:`), Title = workflow, Text = inputs
	TraceSummary    = "summary"    // last event: totals
)

// TraceEvent is one line of a session trace.
type TraceEvent struct {
	Seq  int       `json:"seq"`
	Time time.Time `json:"time"`
	Kind string    `json:"kind"`
	// ToolSession is the tool session of the event (the oh session or one
	// of its subagent sessions); Parent is set for a subagent session.
	ToolSession string `json:"tool_session,omitempty"`
	Parent      string `json:"parent,omitempty"`
	// InSubagent: the event happened in a subagent session.
	InSubagent bool   `json:"in_subagent,omitempty"`
	Agent      string `json:"agent,omitempty"`
	Model      string `json:"model,omitempty"`
	Role       string `json:"role,omitempty"`
	Text       string `json:"text,omitempty"`

	// Tool calls.
	Tool         string         `json:"tool,omitempty"`
	CallID       string         `json:"call_id,omitempty"`
	Input        map[string]any `json:"input,omitempty"`
	Status       string         `json:"status,omitempty"`
	Error        string         `json:"error,omitempty"`
	Output       string         `json:"output,omitempty"`
	Skill        string         `json:"skill,omitempty"`
	Subagent     string         `json:"subagent,omitempty"`
	Child        string         `json:"child,omitempty"`
	Verification bool           `json:"verification,omitempty"`
	Denied       bool           `json:"denied,omitempty"`

	// Decisions.
	DecisionID   string         `json:"decision_id,omitempty"`
	DecisionKind string         `json:"decision_kind,omitempty"`
	Action       string         `json:"action,omitempty"`
	Resources    []string       `json:"resources,omitempty"`
	Title        string         `json:"title,omitempty"`
	Checkpoint   string         `json:"checkpoint,omitempty"`
	Resolution   string         `json:"resolution,omitempty"`
	Answer       map[string]any `json:"answer,omitempty"`
	ResolvedBy   string         `json:"resolved_by,omitempty"`
	ResolvedAt   *time.Time     `json:"resolved_at,omitempty"`
	Granted      *bool          `json:"granted,omitempty"`

	// Checkpoint timeline.
	Event string `json:"event,omitempty"`
	By    string `json:"by,omitempty"`

	// Usage and end of a loop.
	Cost      float64 `json:"cost,omitempty"`
	TokensIn  int64   `json:"tokens_in,omitempty"`
	TokensOut int64   `json:"tokens_out,omitempty"`
	Outcome   string  `json:"outcome,omitempty"`

	// Data: session and summary details, output values.
	Data map[string]any `json:"data,omitempty"`
}

// Trace is the trace of a session.
type Trace struct {
	SessionID string
	Events    []TraceEvent
	// Transcript tells where the tool history came from: live | snapshot |
	// none (server stopped and no snapshot: decisions and timeline only).
	Transcript string
}

// Transcript sources.
const (
	TranscriptLive     = "live"
	TranscriptSnapshot = "snapshot"
	TranscriptNone     = "none"
)

// TraceOptions are the optional sources and settings of a trace.
type TraceOptions struct {
	Usage domain.UsageStore // per tool session totals (nil = none)
	// Secrets are literal values masked in addition to the known patterns
	// (the credentials of the caller).
	Secrets []string
}

// transcriptFile is the transcript snapshot of a session under SessionsDir.
const transcriptFile = "transcript.json"

// ErrNoSession is returned by Trace for an unknown session.
var ErrNoSession = errors.New("session not found")

// Trace gathers the trace of a session.
func (s *Service) Trace(ctx context.Context, sessionID string, o TraceOptions) (*Trace, error) {
	if s.Sessions == nil {
		return nil, errors.New("session: no session store")
	}
	sess, err := s.Sessions.Get(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrNoSession, sessionID, err)
	}
	t := &Trace{SessionID: sess.ID}
	var evs []TraceEvent
	evs = append(evs, sessionEvent(sess))

	entries, source := s.transcript(ctx, sess)
	t.Transcript = source
	verify := verifyPatterns(sess.LaunchPath)
	for _, e := range entries {
		evs = append(evs, entryEvent(e, sess.ID, verify))
	}
	if s.Decisions != nil {
		if list, err := s.Decisions.ListSince(ctx, time.Time{}); err == nil {
			for _, d := range list {
				if d.SessionID == sess.ID {
					evs = append(evs, decisionEvent(d))
				}
			}
		}
	}
	if s.Checkpoints != nil {
		if st, err := s.Checkpoints.State(ctx, sess.ID); err == nil {
			for _, c := range st.Timeline {
				evs = append(evs, TraceEvent{Time: c.At, Kind: TraceCheckpoint, Event: c.Kind, Checkpoint: c.ID, By: c.By, Text: c.Message})
			}
		}
	}
	var keys []string
	for k := range sess.Outputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		at := sess.StartedAt
		if sess.EndedAt != nil {
			at = *sess.EndedAt
		}
		evs = append(evs, TraceEvent{Time: at, Kind: TraceOutput, Title: k, Data: map[string]any{"value": sess.Outputs[k]}})
	}
	// Oldest first; the session event stays first.
	head, rest := evs[0], evs[1:]
	sort.SliceStable(rest, func(i, j int) bool { return rest[i].Time.Before(rest[j].Time) })
	evs = append([]TraceEvent{head}, rest...)
	fillDecisionAgents(evs, sess.EntryAgent)
	evs = append(evs, summaryEvent(ctx, sess, evs, source, o.Usage))
	m := newMasker(o.Secrets)
	for i := range evs {
		evs[i].Seq = i + 1
		evs[i].Time = evs[i].Time.UTC()
		m.event(&evs[i])
	}
	t.Events = evs
	return t, nil
}

// WriteJSONL writes the trace, one event per line.
func (t *Trace) WriteJSONL(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for i := range t.Events {
		if err := enc.Encode(&t.Events[i]); err != nil {
			return err
		}
	}
	return nil
}

// ReadTraceJSONL reads a trace written by WriteJSONL.
func ReadTraceJSONL(r io.Reader) ([]TraceEvent, error) {
	dec := json.NewDecoder(r)
	var out []TraceEvent
	for {
		var e TraceEvent
		if err := dec.Decode(&e); errors.Is(err, io.EOF) {
			return out, nil
		} else if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
}

// transcript reads the tool history live (and saves it), else the last
// snapshot.
func (s *Service) transcript(ctx context.Context, sess *domain.Session) (entries []adapters.TranscriptEntry, source string) {
	if tr, ok := s.Adapter.(adapters.Transcriber); ok {
		if h, err := s.traceHandle(ctx, sess); err == nil {
			if entries, err := tr.Transcript(ctx, h, sess.ID); err == nil {
				s.saveTranscript(sess.ID, entries)
				return entries, TranscriptLive
			}
		}
	}
	if s.SessionsDir != "" {
		if data, err := os.ReadFile(filepath.Join(s.SessionsDir, sess.ID, transcriptFile)); err == nil {
			var entries []adapters.TranscriptEntry
			if json.Unmarshal(data, &entries) == nil {
				return entries, TranscriptSnapshot
			}
		}
	}
	return nil, TranscriptNone
}

// SaveTranscript reads the tool history of a running session and saves it
// for later traces (the session is about to stop). Best effort.
func (s *Service) SaveTranscript(ctx context.Context, sessionID string) error {
	tr, ok := s.Adapter.(adapters.Transcriber)
	if !ok {
		return errors.New("session: the tool adapter cannot read transcripts")
	}
	sess, err := s.Sessions.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	h, err := s.traceHandle(ctx, sess)
	if err != nil {
		return err
	}
	entries, err := tr.Transcript(ctx, h, sessionID)
	if err != nil {
		return err
	}
	s.saveTranscript(sessionID, entries)
	return nil
}

// traceHandle returns the server of a session while it runs, whatever the
// state of the session (a workflow ended, its server still holds the
// history).
func (s *Service) traceHandle(ctx context.Context, sess *domain.Session) (adapters.ServerHandle, error) {
	if s.Servers == nil || sess.GroupKey == "" {
		return adapters.ServerHandle{}, ErrServerNotRunning
	}
	srv, err := s.Servers.Get(ctx, sess.GroupKey)
	if err != nil || srv.Status != domain.ServerReady || (s.Alive != nil && !s.Alive(srv.PID)) {
		return adapters.ServerHandle{}, ErrServerNotRunning
	}
	return adapters.ServerHandle{URL: srv.URL, Password: srv.Password, PID: srv.PID}, nil
}

func (s *Service) saveTranscript(sessionID string, entries []adapters.TranscriptEntry) {
	if s.SessionsDir == "" {
		return
	}
	dir := filepath.Join(s.SessionsDir, sessionID)
	if os.MkdirAll(dir, 0o700) != nil {
		return
	}
	// Masked before it is written: the snapshot outlives the session.
	m := newMasker(nil)
	masked := make([]adapters.TranscriptEntry, len(entries))
	for i, e := range entries {
		e.Text, e.Output = m.text(e.Text), m.text(e.Output)
		if e.Call != nil {
			c := *e.Call
			c.Input, c.Error = m.mapValue(c.Input), m.text(c.Error)
			e.Call = &c
		}
		masked[i] = e
	}
	data, err := json.Marshal(masked)
	if err != nil {
		return
	}
	tmp := filepath.Join(dir, transcriptFile+".tmp")
	if os.WriteFile(tmp, data, 0o600) == nil {
		_ = os.Rename(tmp, filepath.Join(dir, transcriptFile))
	}
}

// fillDecisionAgents names the agent of the decisions that do not tell it:
// the last agent seen in their tool session (the entry agent for the oh
// session itself).
func fillDecisionAgents(evs []TraceEvent, entry string) {
	last := map[string]string{}
	for i := range evs {
		e := &evs[i]
		switch {
		case e.Kind == TraceDecision && e.Agent == "":
			if a := last[e.ToolSession]; a != "" {
				e.Agent = a
			} else if !e.InSubagent {
				e.Agent = entry
			}
		case e.Agent != "" && e.ToolSession != "":
			last[e.ToolSession] = e.Agent
		}
	}
}

func sessionEvent(sess *domain.Session) TraceEvent {
	d := map[string]any{
		"id": sess.ID, "project_id": sess.ProjectID, "workflow": sess.WorkflowID, "entry_agent": sess.EntryAgent,
		"mode": sess.Mode, "runtime": sess.Runtime, "state": string(sess.State), "status": string(sess.Status),
		"provider": sess.Provider, "model": sess.Model, "launch_path": sess.LaunchPath, "started_at": sess.StartedAt,
	}
	if sess.EndedAt != nil {
		d["ended_at"] = *sess.EndedAt
	}
	if sess.Title != nil {
		d["title"] = *sess.Title
	}
	if sess.ParentSessionID != "" {
		d["parent_session"] = sess.ParentSessionID
	}
	if len(sess.Tickets) > 0 {
		d["tickets"] = sess.Tickets
	}
	if sess.WorkflowVersion > 0 {
		d["workflow_version"] = sess.WorkflowVersion
	}
	return TraceEvent{Time: sess.StartedAt, Kind: TraceSession, ToolSession: sess.ID, Agent: sess.EntryAgent, Model: sess.Model, Data: d}
}

// deniedError recognizes a tool call refused by the permission rules.
var deniedError = regexp.MustCompile(`(?i)(permission|denied|rejected|not allowed|refus|forbidden|prevents you)`)

func entryEvent(e adapters.TranscriptEntry, root string, verify []string) TraceEvent {
	ev := TraceEvent{Time: e.Time, ToolSession: e.SessionID, Parent: e.ParentID, InSubagent: e.SessionID != root,
		Agent: e.Agent, Model: e.Model, Role: e.Role, Text: e.Text}
	switch e.Kind {
	case adapters.EntryPrompt:
		ev.Kind = TracePrompt
	case adapters.EntryText:
		ev.Kind = TraceText
	case adapters.EntryUsage:
		ev.Kind, ev.Cost, ev.TokensIn, ev.TokensOut = TraceUsage, e.Cost, e.TokensIn, e.TokensOut
	case adapters.EntryEnd:
		ev.Kind, ev.Outcome = TraceEnd, e.Outcome
	case adapters.EntryTool:
		ev.Kind = TraceTool
		if c := e.Call; c != nil {
			ev.Tool, ev.CallID, ev.Input, ev.Status, ev.Error = c.Action, c.ID, c.Input, c.Status, c.Error
			switch c.Action {
			case sessionspec.ActionSkill:
				ev.Kind, ev.Skill = TraceSkill, c.Skill
			case sessionspec.ActionSubagent:
				ev.Kind, ev.Child = TraceSubagent, e.Child
				ev.Subagent, _ = c.Input["agent"].(string)
			case sessionspec.ActionShell:
				cmd, _ := c.Input["command"].(string)
				ev.Verification = matchesAny(cmd, verify)
			}
			ev.Denied = c.Status == adapters.CallFailed && deniedError.MatchString(c.Error)
		}
		ev.Output = clipText(e.Output, traceOutputMax)
	default:
		ev.Kind = e.Kind
	}
	return ev
}

// traceOutputMax bounds the tool output kept in a trace event.
const traceOutputMax = 4000

func clipText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && !utf8Start(s[cut]) {
		cut--
	}
	return s[:cut] + fmt.Sprintf("… [%d bytes truncated]", len(s)-cut)
}

func utf8Start(b byte) bool { return b&0xC0 != 0x80 }

func decisionEvent(d domain.Decision) TraceEvent {
	ev := TraceEvent{Time: d.CreatedAt, Kind: TraceDecision, DecisionID: d.ID, DecisionKind: string(d.Kind),
		Action: d.Payload.Action, Resources: d.Payload.Resources, Title: d.Payload.Title, Text: d.Payload.Message,
		ResolvedAt: d.ResolvedAt, ResolvedBy: string(d.ResolvedBy)}
	if ts := d.ToolSessionID(); ts != d.SessionID {
		ev.ToolSession, ev.InSubagent = ts, true
	} else {
		ev.ToolSession = d.SessionID
	}
	if cp, ok := d.Payload.Data["checkpoint"].(string); ok {
		ev.Checkpoint = cp
	}
	if ag, ok := d.Payload.Data["agent"].(string); ok {
		ev.Agent = ag
	}
	if len(d.Payload.Fields) > 0 {
		var fields []any
		for _, f := range d.Payload.Fields {
			var opts []any
			for _, o := range f.Options {
				opts = append(opts, o.Label+" "+o.Description)
			}
			fields = append(fields, map[string]any{"title": f.Title, "description": f.Description, "options": opts})
		}
		ev.Data = map[string]any{"fields": fields}
	}
	if r := d.Resolution; r != nil {
		ev.Resolution, ev.Answer = r.Decision, r.Answer
		if r.Message != "" {
			if ev.Data == nil {
				ev.Data = map[string]any{}
			}
			ev.Data["message"] = r.Message
		}
	}
	if d.Kind == domain.DecisionPermission && d.Resolution != nil {
		granted := d.Resolution.Decision == "once" || d.Resolution.Decision == "always"
		ev.Granted = &granted
	}
	return ev
}

func summaryEvent(ctx context.Context, sess *domain.Session, evs []TraceEvent, source string, usage domain.UsageStore) TraceEvent {
	var (
		cost                          float64
		in, out                       int64
		prompts, tools, denied, skill int
		subagents, decisions, verify  int
		perAgent                      = map[string]map[string]any{}
	)
	last := sess.StartedAt
	for _, e := range evs {
		if e.Time.After(last) {
			last = e.Time
		}
		switch e.Kind {
		case TraceUsage:
			cost += e.Cost
			in += e.TokensIn
			out += e.TokensOut
			a := perAgent[e.Agent]
			if a == nil {
				a = map[string]any{"cost": 0.0, "tokens_in": int64(0), "tokens_out": int64(0), "turns": 0}
				perAgent[e.Agent] = a
			}
			a["cost"] = a["cost"].(float64) + e.Cost
			a["tokens_in"] = a["tokens_in"].(int64) + e.TokensIn
			a["tokens_out"] = a["tokens_out"].(int64) + e.TokensOut
			a["turns"] = a["turns"].(int) + 1
		case TracePrompt:
			prompts++
		case TraceTool:
			tools++
			if e.Verification {
				verify++
			}
		case TraceSkill:
			skill++
		case TraceSubagent:
			subagents++
		case TraceDecision:
			decisions++
		}
		if e.Denied || (e.Granted != nil && !*e.Granted) {
			denied++
		}
	}
	d := map[string]any{
		"transcript": source, "cost_usd": max(cost, sess.Cost), "transcript_cost_usd": cost,
		"tokens_in": max(in, sess.TokensIn), "tokens_out": max(out, sess.TokensOut),
		"session_cost_usd": sess.Cost, "prompts": prompts, "tool_calls": tools, "skills_loaded": skill,
		"subagent_calls": subagents, "decisions": decisions, "denied": denied, "verifications": verify,
		"per_agent": perAgent, "state": string(sess.State),
	}
	if usage != nil {
		if u, err := usage.SessionTotal(ctx, sess.ID); err == nil {
			d["ledger_cost_usd"], d["ledger_tokens_in"], d["ledger_tokens_out"] = u.CostUSD, u.TokensIn, u.TokensOut
		}
	}
	return TraceEvent{Time: last, Kind: TraceSummary, ToolSession: sess.ID, Data: d}
}

// verifyPatterns are the shell patterns of the verification commands of the
// project (build, tests, lint).
func verifyPatterns(dir string) []string {
	if dir == "" {
		return nil
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return nil
	}
	return bricks.VerifyCommands(bricks.DetectSignals(dir))
}

// chainSep splits chained shell commands (cd x && go test; a | b).
var chainSep = regexp.MustCompile(`&&|\|\||;|\||\n`)

// matchesAny reports whether a command, or one of its chained parts,
// matches one of the shell patterns (`*` matches any text).
func matchesAny(cmd string, patterns []string) bool {
	parts := append([]string{cmd}, chainSep.Split(cmd, -1)...)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		for _, p := range patterns {
			if wildcard(p, part) {
				return true
			}
		}
	}
	return false
}

func wildcard(pattern, s string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == s
	}
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	s = s[len(parts[0]):]
	for i := 1; i < len(parts)-1; i++ {
		k := strings.Index(s, parts[i])
		if k < 0 {
			return false
		}
		s = s[k+len(parts[i]):]
	}
	return strings.HasSuffix(s, parts[len(parts)-1])
}

// ── Secret masking ────────────────────────────────────────────────────────

// Masked replaces a secret in a trace.
const Masked = "[masked]"

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`),
	regexp.MustCompile(`\bohs_[A-Za-z0-9]{16,}`),
	regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`),
	regexp.MustCompile(`\bABSK[A-Za-z0-9+/=]{20,}`),
	regexp.MustCompile(`\b(?:gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,})`),
	regexp.MustCompile(`\bglpat-[A-Za-z0-9_\-]{20,}`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9_\-]{20,}`),
	regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9\-]{10,}`),
	regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}`),
}

// secretAssign keeps the name of an assigned secret and masks its value
// (TOKEN=…, "password": "…", Authorization: Bearer …).
var secretAssign = regexp.MustCompile(`(?i)((?:authorization|proxy-authorization)\s*[:=]\s*(?:bearer|basic|token)?\s*|(?:[a-z0-9_\-]*(?:password|passwd|secret|token|api[_\-]?key|access[_\-]?key|private[_\-]?key))["']?\s*[:=]\s*["']?)([^\s"'&,;]{6,})`)

// urlCredentials masks the password of a URL (scheme://user:pass@host).
var urlCredentials = regexp.MustCompile(`(://[^/\s:@]+:)([^@\s/]+)(@)`)

// secretKey tells a map key holding a secret.
var secretKey = regexp.MustCompile(`(?i)(password|passwd|secret|token|api[_\-]?key|access[_\-]?key|private[_\-]?key|authorization|credential)`)

type masker struct{ literals []string }

func newMasker(literals []string) *masker {
	var keep []string
	for _, l := range literals {
		if len(l) >= 6 {
			keep = append(keep, l)
		}
	}
	return &masker{literals: keep}
}

func (m *masker) text(s string) string {
	if s == "" {
		return s
	}
	for _, l := range m.literals {
		s = strings.ReplaceAll(s, l, Masked)
	}
	for _, re := range secretPatterns {
		s = re.ReplaceAllString(s, Masked)
	}
	s = secretAssign.ReplaceAllStringFunc(s, func(match string) string {
		sub := secretAssign.FindStringSubmatch(match)
		if sub[2] == Masked || strings.HasPrefix(sub[2], "$") || strings.HasPrefix(sub[2], "{env:") {
			return match // already masked, or a reference to a variable
		}
		return sub[1] + Masked
	})
	return urlCredentials.ReplaceAllString(s, "${1}"+Masked+"${3}")
}

func (m *masker) value(key string, v any) any {
	switch x := v.(type) {
	case string:
		if key != "" && secretKey.MatchString(key) && x != "" {
			return Masked
		}
		return m.text(x)
	case map[string]any:
		return m.mapValue(x)
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = m.value(key, e)
		}
		return out
	case []string:
		out := make([]string, len(x))
		for i, e := range x {
			out[i] = m.text(e)
		}
		return out
	}
	return v
}

func (m *masker) mapValue(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = m.value(k, v)
	}
	return out
}

func (m *masker) event(e *TraceEvent) {
	e.Text, e.Output, e.Error, e.Title = m.text(e.Text), m.text(e.Output), m.text(e.Error), m.text(e.Title)
	e.Input, e.Answer, e.Data = m.mapValue(e.Input), m.mapValue(e.Answer), m.mapValue(e.Data)
	for i, r := range e.Resources {
		e.Resources[i] = m.text(r)
	}
}
