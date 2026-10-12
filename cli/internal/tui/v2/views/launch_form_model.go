package views

import (
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Launch form model (P1-T22): the state of the generated form, without
// widgets (tests, golden rendering).

// LaunchRuntime is an execution environment offered by the form.
type LaunchRuntime struct {
	Kind      string // local | container | remote
	Label     string
	Available bool
	Reason    string // why it is unavailable
}

// LaunchChoices are the choices of the form.
type LaunchChoices struct {
	Inputs   map[string]string
	Tickets  []string // ticket input (one session per ticket when multi)
	Mode     string
	Runtime  string
	Location string // base | new | <worktree path>
	Attach   string
	// OneSession gives every ticket to a single session.
	OneSession bool
	// Dirty is what a writing session does in a directory with uncommitted
	// changes: LaunchDirtyWorktree (default), LaunchDirtyStash or
	// LaunchDirtyAllow.
	Dirty string
	// OnExisting is what is done with the open session already working on a
	// ticket of the launch (LaunchExisting*; "" = no choice);
	// DiscardBaseChanges lets a deletion discard the uncommitted changes of
	// a session that worked in the project directory.
	OnExisting         string
	DiscardBaseChanges bool
}

// Choices for an open session already working on a ticket of the launch
// (runsvc.OnExisting values).
const (
	LaunchExistingResume   = "resume"
	LaunchExistingTransfer = "transfer"
	LaunchExistingKeep     = "keep"
	LaunchExistingDelete   = "delete"
	LaunchExistingParallel = "parallel"
)

// Fields compared between an existing session and the launch
// (runsvc.Diff* values).
const (
	LaunchDiffWorkflow = "workflow"
	LaunchDiffVersion  = "version"
	LaunchDiffRuntime  = "runtime"
	LaunchDiffMode     = "mode"
	LaunchDiffModel    = "model"
	LaunchDiffLocation = "location"
	LaunchDiffBranch   = "branch"
)

// LaunchExisting is an open session working on a ticket of the launch,
// next to the planned session (recap comparison block).
type LaunchExisting struct {
	ID     string
	Title  string
	Shared []string // tickets worked on by both
	// SameWorkflow: it runs the workflow of the launch (a choice is needed).
	SameWorkflow bool
	// Target: the choice buttons apply to it.
	Target bool

	State, StateLabel string
	OpenDecisions     int

	Workflow, PlannedWorkflow string
	Version, PlannedVersion   int
	Runtime, PlannedRuntime   string
	Mode, PlannedMode         string
	Model, PlannedModel       string
	Path, PlannedPath         string
	Branch, PlannedBranch     string

	// Work of the existing session (WorkKnown false outside git).
	WorkKnown       bool
	CommitsUnpushed int
	ChangedFiles    int
	Cost            float64

	// Diffs are the differing fields (LaunchDiff*); Actions the possible
	// choices (LaunchExisting*).
	Diffs   []string
	Actions []string
}

// Differs reports whether a field differs from the launch.
func (e LaunchExisting) Differs(fields ...string) bool {
	for _, d := range e.Diffs {
		for _, f := range fields {
			if d == f {
				return true
			}
		}
	}
	return false
}

// Allows reports whether a choice is possible.
func (e LaunchExisting) Allows(choice string) bool {
	for _, a := range e.Actions {
		if a == choice {
			return true
		}
	}
	return false
}

// DeleteSummary is what deleting a session loses (confirmation).
type DeleteSummary struct {
	SessionID string
	// Items are the lines of the confirmation: removed, kept, lost.
	Items []string
	// Losses: commits or uncommitted changes are lost.
	Losses bool
	// BaseChanges are the uncommitted changes of the project directory the
	// session worked in: kept unless the user also discards them (second
	// confirmation).
	BaseChanges int
}

// Choices offered when the directory of a writing session has uncommitted
// changes (Récap: [w] worktree · [s] stash · [b] launch anyway).
const (
	LaunchDirtyWorktree = ""
	LaunchDirtyStash    = "stash"
	LaunchDirtyAllow    = "allow"
)

// LaunchRecap is the recap step content (computed by the wiring layer).
type LaunchRecap struct {
	Rows     []InfoField
	Warnings []string
	// Dirty: a writing session starts in a directory with uncommitted
	// changes; the recap offers the choices (LaunchChoices.Dirty).
	Dirty bool
	// Suggestions are workflows to run first (failed preconditions).
	Suggestions []LaunchSuggestion
	// Existing are the open sessions working on a ticket of the launch;
	// NeedsChoice: one runs the same workflow, « Lancer » gives way to the
	// choices (LaunchChoices.OnExisting).
	Existing    []LaunchExisting
	NeedsChoice bool
}

// existingTarget is the existing session the choices apply to.
func (r *LaunchRecap) existingTarget() (LaunchExisting, bool) {
	if r == nil {
		return LaunchExisting{}, false
	}
	for _, e := range r.Existing {
		if e.Target {
			return e, true
		}
	}
	return LaunchExisting{}, false
}

// LaunchSuggestion offers to run another workflow first.
type LaunchSuggestion struct {
	WorkflowID string
	Label      string // button label (« Lancer onboarding d'abord, puis revenir »)
}

// launchModel holds the values of the form.
type launchModel struct {
	spec        *workflow.Spec
	lang        string
	values      map[string]string
	tickets     []string
	ticketInput string
	multi       bool // several tickets may be chosen
	perSession  bool // one session per ticket (beads-id with picker.multi)
	mode        string
	runtime     string
	location    string
	attach      string
	runtimes    []LaunchRuntime
	oneSession  bool
	dirty       string
	onExisting  string
	discardBase bool
}

func newLaunchModel(cfg LaunchFormConfig) *launchModel {
	m := &launchModel{spec: cfg.Spec, lang: cfg.Lang, values: map[string]string{}, attach: cfg.DefaultAttach,
		runtimes: cfg.Runtimes, mode: cfg.DefaultMode, runtime: cfg.DefaultRuntime}
	if m.lang == "" {
		m.lang = i18n.Locale()
	}
	for _, k := range cfg.Spec.Inputs.Keys() {
		in, _ := cfg.Spec.Inputs.Get(k)
		switch in.Type {
		case workflow.InputBeadsID, workflow.InputBeadsIDs:
			if m.ticketInput == "" {
				m.ticketInput = k
				m.perSession = in.Type == workflow.InputBeadsID && in.Picker != nil && in.Picker.Multi
				m.multi = in.Type == workflow.InputBeadsIDs || m.perSession
				continue
			}
		}
		if s, ok := in.Default.(string); ok && !strings.Contains(s, "{{") {
			m.values[k] = s
		} else if b, ok := in.Default.(bool); ok && b {
			m.values[k] = "true"
		}
	}
	for k, v := range cfg.Prefill {
		if k == m.ticketInput {
			m.tickets = splitTickets(v)
			continue
		}
		m.values[k] = v
	}
	if len(cfg.Tickets) > 0 {
		m.tickets = append([]string(nil), cfg.Tickets...)
	}
	if m.mode == "" {
		m.mode = cfg.Spec.DefaultMode()
	}
	if m.runtime == "" {
		m.runtime = string(cfg.Spec.DefaultRuntime())
	}
	if len(cfg.Locations) > 0 {
		m.location = cfg.Locations[0].Value
	}
	return m
}

func splitTickets(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// inputLabel is the label of an input (localized label, else its id).
func (m *launchModel) inputLabel(k string) string {
	in, _ := m.spec.Inputs.Get(k)
	l := in.Label.Text(m.lang)
	if l == "" {
		l = k
	}
	if in.Required && in.Default == nil && in.From == "" {
		l += " *"
	}
	return l
}

// validate returns the problems blocking the launch (localized).
func (m *launchModel) validate() []string {
	var out []string
	for _, k := range m.spec.Inputs.Keys() {
		in, _ := m.spec.Inputs.Get(k)
		if !in.Required || in.Default != nil || in.From != "" {
			continue // optional, defaulted, or computed by oh at launch
		}
		if k == m.ticketInput {
			if len(m.tickets) == 0 {
				out = append(out, i18n.Tf("tui.launch.required", m.inputLabel(k)))
			}
			continue
		}
		if strings.TrimSpace(m.values[k]) == "" {
			out = append(out, i18n.Tf("tui.launch.required", m.inputLabel(k)))
		}
	}
	if !m.multi && len(m.tickets) > 1 {
		out = append(out, i18n.T("tui.launch.single_ticket"))
	}
	for _, r := range m.runtimes {
		if r.Kind == m.runtime && !r.Available {
			out = append(out, i18n.Tf("tui.launch.runtime_unavailable", r.Label, r.Reason))
		}
	}
	return out
}

// choices returns the launch choices.
func (m *launchModel) choices() LaunchChoices {
	c := LaunchChoices{Inputs: map[string]string{}, Mode: m.mode, Runtime: m.runtime, Location: m.location, Attach: m.attach,
		OneSession: m.oneSession && m.perSession, Dirty: m.dirty, OnExisting: m.onExisting, DiscardBaseChanges: m.discardBase}
	keys := make([]string, 0, len(m.values))
	for k := range m.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if v := m.values[k]; v != "" {
			c.Inputs[k] = v
		}
	}
	c.Tickets = append([]string(nil), m.tickets...)
	return c
}

// sessionsLine summarizes a multi-ticket launch (« 3 sessions · 1 serveur »).
func (m *launchModel) sessionsLine() string {
	if !m.perSession || len(m.tickets) < 2 || m.oneSession {
		return ""
	}
	return i18n.Tf("tui.launch.sessions_line", len(m.tickets))
}
