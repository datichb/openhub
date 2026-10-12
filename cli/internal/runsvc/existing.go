package runsvc

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/worktree"
)

// Existing sessions of a ticket (launch on a ticket already worked on).
//
// API for the launch UIs (CLI `oh run`, TUI recap):
//
//   - Plan fills RunPlan.Existing with the open sessions of the project that
//     share a ticket with the launch (any workflow), each compared with the
//     planned session (ExistingSession.Diffs, Work, Actions). When one of
//     them runs the same workflow and RunRequest.OnExisting is not set,
//     RunPlan.NeedsChoice is true and Start refuses the plan
//     (ExistingChoiceError): the UI asks for a choice and plans again with
//     RunRequest.OnExisting (and DiscardBaseChanges for a delete).
//   - Start applies the choice to the matched session of each planned
//     session (PlannedSession.Existing, PlannedSession.Action): resume and
//     transfer return the existing session (StartResult.Existing) instead of
//     a new one; keep stops it and starts the new one in its directory with
//     a handoff brief; delete removes it (DeleteSession) before the launch;
//     parallel starts the new one beside it.
//   - A failed transfer leaves the session as it was and returns a
//     *TransferError: the UI may plan again with OnExistingKeep.
//   - PlanDelete lists what a deletion loses (confirmation), DeleteSession
//     does it (also `oh session delete`).

// OnExisting is what a launch does with a session already working on one
// of its tickets.
type OnExisting string

// Choices for an existing session.
const (
	// OnExistingUnset: no choice (informational for another workflow; the
	// same workflow needs one, RunPlan.NeedsChoice).
	OnExistingUnset OnExisting = ""
	// OnExistingResume: no new session, the existing one is opened again.
	OnExistingResume OnExisting = "resume"
	// OnExistingTransfer: the existing session (its conversation) moves to
	// the runtime and model of the launch.
	OnExistingTransfer OnExisting = "transfer"
	// OnExistingKeep: the existing session stops; the new one continues its
	// work in the same directory and branch, with a handoff brief.
	OnExistingKeep OnExisting = "keep"
	// OnExistingDelete: the existing session and its work are deleted, then
	// the launch starts afresh.
	OnExistingDelete OnExisting = "delete"
	// OnExistingParallel: the new session starts beside the existing one
	// (another worktree).
	OnExistingParallel OnExisting = "parallel"
)

// OnExistingChoices lists the choices, in the order offered to the user.
var OnExistingChoices = []OnExisting{OnExistingResume, OnExistingTransfer, OnExistingKeep, OnExistingDelete, OnExistingParallel}

// ParseOnExisting reads a --on-existing value.
func ParseOnExisting(v string) (OnExisting, error) {
	if v == "" {
		return OnExistingUnset, nil
	}
	for _, c := range OnExistingChoices {
		if string(c) == v {
			return c, nil
		}
	}
	return "", fmt.Errorf("runsvc: unknown choice %q for an existing session (resume, transfer, keep, delete, parallel)", v)
}

// WorkInfo is the git work of a session in its directory.
type WorkInfo struct {
	// Known is false outside git (the counts are then zero).
	Known bool `json:"known"`
	// CommitsAhead are the commits of its branch not on the base branch.
	CommitsAhead int `json:"commits_ahead"`
	// CommitsUnpushed are the commits found on no remote branch.
	CommitsUnpushed int `json:"commits_unpushed"`
	// ChangedFiles are the files with uncommitted changes.
	ChangedFiles int `json:"changed_files"`
}

// Fields of FieldDiff.
const (
	DiffWorkflow = "workflow"
	DiffVersion  = "version"
	DiffBundle   = "bundle"
	DiffRuntime  = "runtime"
	DiffMode     = "mode"
	DiffProvider = "provider"
	DiffModel    = "model"
	DiffLocation = "location"
	DiffBranch   = "branch"
)

// FieldDiff is a setting of an existing session that differs from the
// planned launch.
type FieldDiff struct {
	Field    string `json:"field"`
	Existing string `json:"existing"`
	Planned  string `json:"planned"`
}

// ExistingSession is an open session sharing a ticket with a launch.
type ExistingSession struct {
	ID              string          `json:"id"`
	Title           string          `json:"title,omitempty"`
	State           domain.RunState `json:"state"`
	WorkflowID      string          `json:"workflow"`
	WorkflowVersion int             `json:"workflow_version,omitempty"`
	BundleHash      string          `json:"bundle,omitempty"`
	Runtime         string          `json:"runtime"`
	Mode            string          `json:"mode,omitempty"`
	Provider        string          `json:"provider,omitempty"`
	Model           string          `json:"model,omitempty"`
	// LocationKind is base or worktree; LaunchPath its directory.
	LocationKind string   `json:"location_kind,omitempty"`
	LaunchPath   string   `json:"path"`
	Branch       string   `json:"branch,omitempty"`
	Tickets      []string `json:"tickets"`
	// Shared are its tickets that the planned session works on too.
	Shared []string `json:"shared"`
	// Planned is the index of the planned session it matches (RunPlan.Sessions).
	Planned       int       `json:"planned"`
	OpenDecisions int       `json:"open_decisions"`
	Cost          float64   `json:"cost"`
	StartedAt     time.Time `json:"started_at"`
	LastActivity  time.Time `json:"last_activity"`
	// SameWorkflow: it runs the workflow of the launch (a choice is needed).
	SameWorkflow bool        `json:"same_workflow"`
	Work         WorkInfo    `json:"work"`
	Diffs        []FieldDiff `json:"diffs,omitempty"`
	// Actions are the choices possible for it.
	Actions []OnExisting `json:"actions"`
}

// Differs reports whether a field differs from the planned launch.
func (e ExistingSession) Differs(field string) bool {
	for _, d := range e.Diffs {
		if d.Field == field {
			return true
		}
	}
	return false
}

// Allows reports whether a choice is possible for the session.
func (e ExistingSession) Allows(c OnExisting) bool { return slices.Contains(e.Actions, c) }

// ErrNeedsChoice is returned by Start when a session of the same workflow
// already works on a ticket of the launch and no choice was made.
var ErrNeedsChoice = errors.New("a session of this workflow already works on this ticket: a choice is needed")

// ErrChoiceUnavailable is returned by Plan when the choice is not possible
// for the existing session (e.g. resuming a remote session).
var ErrChoiceUnavailable = errors.New("this choice is not possible for the existing session")

// ExistingChoiceError carries the sessions that need a choice.
type ExistingChoiceError struct{ Existing []ExistingSession }

func (e *ExistingChoiceError) Error() string {
	ids := make([]string, 0, len(e.Existing))
	for _, x := range e.Existing {
		if x.SameWorkflow {
			ids = append(ids, x.ID)
		}
	}
	return fmt.Sprintf("%v (%s)", ErrNeedsChoice, strings.Join(ids, ", "))
}

func (e *ExistingChoiceError) Unwrap() error { return ErrNeedsChoice }

// openState reports a session that still holds its work ("non-stopped"):
// failed launches and legacy rows are left out.
func openState(st domain.RunState) bool {
	return st != "" && st != domain.RunStopped && st != domain.RunFailed
}

// existingActions lists the choices possible for a session.
func existingActions(o domain.Session) []OnExisting {
	if o.Runtime == string(sessionspec.RuntimeRemote) {
		// Runs elsewhere: fetched back with `oh session fetch`.
		return []OnExisting{OnExistingParallel}
	}
	out := []OnExisting{OnExistingResume}
	if o.GroupKey != "" && o.BundleHash != "" {
		out = append(out, OnExistingTransfer)
	}
	return append(out, OnExistingKeep, OnExistingDelete, OnExistingParallel)
}

// findExisting returns the open sessions of the project sharing a ticket
// with a planned session, most recent first, and the project sessions.
func (s *Service) findExisting(ctx context.Context, projectID string, inputs []PlannedInput) ([]ExistingSession, []domain.Session) {
	if s.Sessions == nil || projectID == "" {
		return nil, nil
	}
	list, err := s.Sessions.List(ctx, projectID)
	if err != nil {
		return nil, nil
	}
	var out []ExistingSession
	for _, o := range list {
		if !openState(o.State) || len(o.Tickets) == 0 {
			continue
		}
		for i, in := range inputs {
			shared := sharedTickets(o.Tickets, in.Tickets)
			if len(shared) == 0 {
				continue
			}
			e := ExistingSession{ID: o.ID, State: o.State, WorkflowID: o.WorkflowID, WorkflowVersion: o.WorkflowVersion,
				BundleHash: o.BundleHash, Runtime: string(runtimeKind(sessionspec.RuntimeKind(o.Runtime))), Mode: o.Mode,
				Provider: o.Provider, Model: o.Model, LocationKind: o.Location, LaunchPath: o.LaunchPath,
				Branch: o.WorktreeBranch, Tickets: o.Tickets, Shared: shared, Planned: i, Cost: o.Cost,
				StartedAt: o.StartedAt, LastActivity: o.StartedAt, Actions: existingActions(o)}
			if o.Title != nil {
				e.Title = *o.Title
			}
			if o.StateChangedAt != nil {
				e.LastActivity = *o.StateChangedAt
			}
			if e.Branch == "" && o.LaunchPath != "" {
				if b, err := worktree.CurrentBranch(o.LaunchPath); err == nil && !strings.HasPrefix(b, "(detached)") {
					e.Branch = b
				}
			}
			out = append(out, e)
			break
		}
	}
	return out, list
}

func sharedTickets(a, b []string) []string {
	var out []string
	for _, t := range a {
		if slices.Contains(b, t) && !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out
}

// target returns, for each planned session, the existing session a choice
// applies to: the most recent one of the same workflow, else the most
// recent one ("" = none).
func existingTargets(existing []ExistingSession, n int) []string {
	out := make([]string, n)
	same := make([]bool, n)
	for _, e := range existing {
		i := e.Planned
		if i < 0 || i >= n {
			continue
		}
		if out[i] == "" || (e.SameWorkflow && !same[i]) {
			out[i], same[i] = e.ID, e.SameWorkflow
		}
	}
	return out
}

func existingByID(existing []ExistingSession, id string) (ExistingSession, bool) {
	for _, e := range existing {
		if e.ID == id {
			return e, true
		}
	}
	return ExistingSession{}, false
}

// compareExisting fills the differences between an existing session and
// its planned session.
func compareExisting(e *ExistingSession, req RunRequest, ps PlannedSession) {
	add := func(field, existing, planned string) {
		if existing != planned && planned != "" {
			e.Diffs = append(e.Diffs, FieldDiff{Field: field, Existing: existing, Planned: planned})
		}
	}
	add(DiffWorkflow, e.WorkflowID, req.Workflow.ID)
	if e.SameWorkflow && req.Workflow.Version != 0 {
		add(DiffVersion, strconv.Itoa(e.WorkflowVersion), strconv.Itoa(req.Workflow.Version))
	}
	if b := req.Base.Bundle; b != nil {
		add(DiffBundle, e.BundleHash, b.Spec.Hash)
	}
	add(DiffRuntime, e.Runtime, string(runtimeKind(req.Base.Runtime)))
	mode := req.Base.Mode
	if mode == "" && req.Base.Bundle != nil && req.Base.Bundle.Spec.Workflow != nil {
		mode = req.Base.Bundle.Spec.Workflow.DefaultMode
	}
	add(DiffMode, e.Mode, mode)
	add(DiffProvider, e.Provider, req.Base.Provider)
	if e.Model != "" {
		add(DiffModel, e.Model, plannedModel(req.Base))
	}
	if e.LaunchPath != "" && ps.Location.Path != "" && filepath.Clean(e.LaunchPath) != filepath.Clean(ps.Location.Path) {
		e.Diffs = append(e.Diffs, FieldDiff{Field: DiffLocation, Existing: e.LaunchPath, Planned: ps.Location.Path})
	}
	add(DiffBranch, e.Branch, ps.Location.Branch)
}

// plannedModel is the model of the entry agent of a launch.
func plannedModel(base StartRequest) string {
	if base.Bundle == nil {
		return ""
	}
	entry := base.EntryAgent
	if entry == "" {
		entry = base.Bundle.Spec.EntryAgent
	}
	if m := entryModel(base.Bundle.Spec, entry); m != nil {
		return m.String()
	}
	return ""
}

// workInfo reads the git work of a directory cheaply (no network).
func workInfo(dir, projectPath string) WorkInfo {
	if dir == "" || !worktree.IsGitRepo(dir) {
		return WorkInfo{}
	}
	w := WorkInfo{Known: true}
	if out, err := gitLines(dir, "status", "--porcelain"); err == nil {
		w.ChangedFiles = len(out)
	}
	base := worktree.DetectBaseBranch(projectPath)
	if n, err := gitCount(dir, "rev-list", "--count", base+"..HEAD"); err == nil {
		w.CommitsAhead = n
	}
	if remotes, err := gitLines(dir, "remote"); err == nil && len(remotes) == 0 {
		w.CommitsUnpushed = w.CommitsAhead
	} else if n, err := gitCount(dir, "rev-list", "--count", "HEAD", "--not", "--remotes"); err == nil {
		w.CommitsUnpushed = n
	}
	return w
}

func gitOutput(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := gitutil.Command(ctx, dir, args...).Output()
	return strings.TrimRight(string(out), "\n"), err
}

func gitLines(dir string, args ...string) ([]string, error) {
	out, err := gitOutput(dir, args...)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	return lines, nil
}

func gitCount(dir string, args ...string) (int, error) {
	out, err := gitOutput(dir, args...)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(out))
}
