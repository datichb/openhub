package runsvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/filelock"
	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Workflow launches (03 §2.4): Plan computes what a launch will do (fiche de
// lancement, `oh run --recap`), Start runs it: N sessions of one bundle in
// the same server group (I5), one location each.

// Warning is a finding of a launch plan; Code is stable and localized by the
// caller ("cmd.run.warn.<code>").
type Warning struct {
	Code string `json:"code"`
	Args []any  `json:"args,omitempty"`
}

// Warning codes.
const (
	WarnAutoWorktree = "auto_worktree" // O10: another writing session uses the directory (new worktree)
	// WarnAutoWorktreeExisting: O10 with a worktree that already exists.
	WarnAutoWorktreeExisting = "auto_worktree_existing"
	// WarnWorktreeBusy: the worktree of the session is used by another
	// writing session too.
	WarnWorktreeBusy = "worktree_busy"
	// WarnTicketWorktree: the session works in the project directory while
	// an open session of its ticket works in a worktree.
	WarnTicketWorktree = "ticket_worktree"
	// WarnExisting: an open session (another workflow) works on a ticket of
	// the launch; WarnParallel: the new session starts beside it.
	WarnExisting = "existing"
	WarnParallel = "parallel"
	// WarnTransferBundle: the transferred session continues with another
	// bundle than the one it started with.
	WarnTransferBundle = "transfer_bundle"
	WarnSharedLocation = "shared_location" // writers share a directory (no git: no worktree)
	WarnDirty          = "dirty"           // uncommitted changes where a writer starts (allowed)
	WarnDirtyWorktree  = "dirty_worktree"  // uncommitted changes: the writer works in a worktree
	WarnDirtyStash     = "dirty_stash"     // uncommitted changes: stashed when the session starts
	// WarnPending: decisions already waiting (args: on the tickets of the
	// launch, in the project, in total).
	WarnPending = "pending"
	// WarnVolumeMountPoint: a relative cache volume of the container is
	// absent from a location; the engine creates it there, empty, as the
	// mount point (its content stays in the container volume).
	WarnVolumeMountPoint = "volume_mount_point"
)

// WorkflowRef identifies the launched workflow in the session records.
type WorkflowRef struct {
	ID      string
	Layer   string
	Version int
	Risk    string // read | write | publish
}

// PlannedInput describes one session of a run (one per ticket).
type PlannedInput struct {
	Title string
	// Branch is the branch of the session worktree, if one is needed.
	Branch string
	// Label identifies the session in the plan (ticket id…).
	Label string
	// Tickets are the Beads tickets the session works on (ticket input).
	Tickets []string
}

// RunRequest describes a workflow launch.
type RunRequest struct {
	// Base holds the settings shared by the sessions (bundle, provider, team,
	// attach, runtime, mode). Location, Title and Prompt are set per session.
	Base        StartRequest
	Workflow    WorkflowRef
	ProjectPath string
	Location    LocationChoice
	Sessions    []PlannedInput // empty: one session
	// Dirty is what a writer does in a directory with uncommitted changes.
	Dirty DirtyPolicy
	// ParentSessionID chains the sessions to a previous one (O7).
	ParentSessionID string
	// OnExisting is what is done with the open sessions of the tickets of
	// the launch (RunPlan.Existing).
	OnExisting OnExisting
	// DiscardBaseChanges allows OnExistingDelete to discard the uncommitted
	// changes of a session that worked in the project directory.
	DiscardBaseChanges bool
}

// PlannedSession is a session of a plan. Prompt is rendered by the caller
// once the location is known (it may use .oh.location).
type PlannedSession struct {
	PlannedInput
	Location Location `json:"location"`
	Prompt   string   `json:"-"`
	// Existing is the open session of its tickets the choice applies to
	// ("" = none); Action the choice applied (OnExistingUnset = none).
	Existing string     `json:"existing,omitempty"`
	Action   OnExisting `json:"action,omitempty"`
}

// RunPlan is what Start will do.
type RunPlan struct {
	Request  RunRequest       `json:"-"`
	Sessions []PlannedSession `json:"sessions"`
	Warnings []Warning        `json:"warnings,omitempty"`
	// Existing are the open sessions of the project working on a ticket of
	// the launch, most recent first.
	Existing []ExistingSession `json:"existing,omitempty"`
	// NeedsChoice: one of them runs the same workflow and no choice was
	// made (RunRequest.OnExisting); Start refuses the plan.
	NeedsChoice bool `json:"needs_choice,omitempty"`
}

// Worktrees counts the worktrees of the plan (created or reused).
func (p *RunPlan) Worktrees() int {
	n := 0
	for _, s := range p.Sessions {
		if s.Location.Kind == LocationWorktree {
			n++
		}
	}
	return n
}

// ErrLaunchInProgress is returned when another launch prepares a session in
// the same directory (double launch protection).
var ErrLaunchInProgress = errors.New("a launch is already in progress in this directory")

// Plan resolves the locations of a run and gathers its warnings. It has no
// side effect.
func (s *Service) Plan(ctx context.Context, req RunRequest) (*RunPlan, error) {
	if req.Base.Bundle == nil {
		return nil, errors.New("runsvc: the run has no bundle")
	}
	if req.ProjectPath == "" {
		return nil, errors.New("runsvc: the run has no project directory")
	}
	inputs := req.Sessions
	if len(inputs) == 0 {
		inputs = []PlannedInput{{Title: req.Base.Title}}
	}
	branches := make([]string, len(inputs))
	for i, in := range inputs {
		branches[i] = in.Branch
	}
	writes := RiskWrites(req.Workflow.Risk)
	existing, projectSessions := s.findExisting(ctx, req.Base.ProjectID, inputs)
	for i := range existing {
		existing[i].SameWorkflow = existing[i].WorkflowID == req.Workflow.ID && existing[i].ID != req.ParentSessionID
	}
	targets := existingTargets(existing, len(inputs))
	lo, actions, err := s.existingLocations(req, existing, targets)
	if err != nil {
		return nil, err
	}
	locs, warns, err := s.planLocations(ctx, req.Base.ProjectID, req.ProjectPath, req.Location, writes, branches, req.Dirty, lo)
	if err != nil {
		return nil, err
	}
	plan := &RunPlan{Request: req, Warnings: warns, Existing: existing}
	for i, in := range inputs {
		plan.Sessions = append(plan.Sessions, PlannedSession{PlannedInput: in, Location: locs[i], Existing: targets[i], Action: actions[i]})
	}
	for i := range plan.Existing {
		e := &plan.Existing[i]
		compareExisting(e, req, plan.Sessions[e.Planned])
		e.Work = workInfo(e.LaunchPath, req.ProjectPath)
		if e.SameWorkflow && req.OnExisting == OnExistingUnset {
			plan.NeedsChoice = true
		}
		switch {
		case e.ID != targets[e.Planned] || req.OnExisting == OnExistingParallel:
			plan.Warnings = append(plan.Warnings, Warning{Code: WarnParallel, Args: []any{e.ID, strings.Join(e.Shared, ", ")}})
		case req.OnExisting == OnExistingUnset && !e.SameWorkflow:
			plan.Warnings = append(plan.Warnings, Warning{Code: WarnExisting, Args: []any{e.ID, e.WorkflowID, strings.Join(e.Shared, ", ")}})
		case req.OnExisting == OnExistingTransfer && req.Base.Bundle != nil && e.BundleHash != req.Base.Bundle.Spec.Hash:
			plan.Warnings = append(plan.Warnings, Warning{Code: WarnTransferBundle, Args: []any{e.ID}})
		}
	}
	plan.Warnings = append(plan.Warnings, volumeMountPoints(req.Base, locs)...)
	if s.Decisions != nil {
		if open, err := s.Decisions.ListOpen(ctx, domain.DecisionFilter{}); err == nil && len(open) > 0 {
			plan.Warnings = append(plan.Warnings, pendingWarning(open, projectSessions, plan.Existing))
		}
	}
	return plan, nil
}

// existingLocations applies the choice made for the existing sessions to
// the locations of a run: the action of each planned session and the
// location changes it implies.
func (s *Service) existingLocations(req RunRequest, existing []ExistingSession, targets []string) (locOptions, []OnExisting, error) {
	lo := locOptions{free: map[string]bool{}, fixed: map[int]Location{}, recreate: map[string]bool{}, clean: map[string]bool{},
		ticketWorktree: map[int]ExistingSession{}}
	actions := make([]OnExisting, len(targets))
	project := filepath.Clean(req.ProjectPath)
	for i, id := range targets {
		if id == "" {
			continue
		}
		e, _ := existingByID(existing, id)
		if e.LaunchPath != "" && filepath.Clean(e.LaunchPath) != project {
			lo.ticketWorktree[i] = e
		}
		c := req.OnExisting
		if c == OnExistingUnset {
			continue
		}
		if !e.Allows(c) {
			return lo, nil, fmt.Errorf("%w: %s (%s)", ErrChoiceUnavailable, c, e.ID)
		}
		actions[i] = c
		if e.LaunchPath == "" {
			continue
		}
		path := filepath.Clean(e.LaunchPath)
		here := Location{Path: path, Kind: LocationWorktree, Branch: e.Branch}
		if path == project {
			here.Kind = LocationBase
		}
		switch c {
		case OnExistingResume, OnExistingTransfer, OnExistingKeep:
			lo.fixed[i] = here
			lo.free[path] = true
		case OnExistingDelete:
			lo.free[path] = true
			if path == project {
				if req.DiscardBaseChanges {
					lo.clean[path] = true
				}
			} else {
				lo.recreate[path] = true
			}
		}
	}
	return lo, actions, nil
}

// pendingWarning counts the open decisions on the tickets of a launch (its
// existing sessions, whose OpenDecisions it sets), in its project and in
// total.
func pendingWarning(open []domain.Decision, project []domain.Session, existing []ExistingSession) Warning {
	inProject := map[string]bool{}
	for _, o := range project {
		inProject[o.ID] = true
	}
	onTicket := map[string]int{}
	for _, e := range existing {
		onTicket[e.ID] = 0
	}
	nProject, nTicket := 0, 0
	for _, d := range open {
		if inProject[d.SessionID] {
			nProject++
		}
		if _, ok := onTicket[d.SessionID]; ok {
			onTicket[d.SessionID]++
			nTicket++
		}
	}
	for i := range existing {
		existing[i].OpenDecisions = onTicket[existing[i].ID]
	}
	return Warning{Code: WarnPending, Args: []any{nTicket, nProject, len(open)}}
}

// Start runs a plan: worktrees are created, then each session is started in
// the server group of the bundle. A launch lock per directory refuses a
// second launch while one is being prepared there (ErrLaunchInProgress).
// Sessions started before an error are returned with it.
func (s *Service) Start(ctx context.Context, plan *RunPlan) ([]*StartResult, error) {
	req := plan.Request
	if plan.NeedsChoice && req.OnExisting == OnExistingUnset {
		return nil, &ExistingChoiceError{Existing: plan.Existing}
	}
	// Before anything is created: worktrees, stashes, stopped sessions.
	if err := checkRequestModels(req.Base); err != nil {
		return nil, err
	}
	var unlocks []func()
	defer func() {
		for _, u := range unlocks {
			u()
		}
	}()
	for _, ps := range plan.Sessions {
		u, err := filelock.TryLock(s.launchLockPath(req.Base.ProjectID, ps.Location.Path))
		if errors.Is(err, filelock.ErrLocked) {
			return nil, fmt.Errorf("%w: %s", ErrLaunchInProgress, ps.Location.Path)
		}
		if err != nil {
			return nil, err
		}
		unlocks = append(unlocks, u)
	}

	var out []*StartResult
	for i := range plan.Sessions {
		ps := &plan.Sessions[i]
		var (
			handoff string
			deleted *DeleteReport
			kept    *ExistingSession
		)
		if ps.Existing != "" {
			e, _ := existingByID(plan.Existing, ps.Existing)
			switch ps.Action {
			case OnExistingResume, OnExistingTransfer:
				res, err := s.continueExisting(ctx, req.Base, e, ps.Action)
				if res != nil {
					out = append(out, res)
				}
				if err != nil {
					return out, err
				}
				continue
			case OnExistingKeep:
				if err := s.StopSession(ctx, e.ID); err != nil {
					return out, fmt.Errorf("stopping session %s: %w", e.ID, err)
				}
				handoff, kept = s.HandoffBrief(ctx, e.ID, req.ProjectPath), &e
			case OnExistingDelete:
				rep, err := s.DeleteSession(ctx, e.ID, DeleteOptions{ProjectPath: req.ProjectPath, DiscardBaseChanges: req.DiscardBaseChanges})
				if err != nil {
					return out, fmt.Errorf("deleting session %s: %w", e.ID, err)
				}
				deleted = rep
			}
		}
		created := false
		if ps.Location.Create {
			p, err := s.createWorktree(req.ProjectPath, ps.Location.Branch)
			if err != nil {
				return out, fmt.Errorf("worktree %s: %w", ps.Location.Branch, err)
			}
			ps.Location.Path, ps.Location.Create = p, false
			created = true
		}
		var stashed string
		if ps.Location.Stash {
			ref, err := s.stash(ps.Location.Path, "oh: "+req.Workflow.ID+" "+time.Now().Format("2006-01-02 15:04"))
			if err != nil {
				return out, err
			}
			stashed, ps.Location.Stash = ref, false
		}
		sr := req.Base
		sr.Location, sr.Prompt = ps.Location.Path, ps.Prompt
		if ps.Title != "" {
			sr.Title = ps.Title
		}
		if sr.ProjectDir == "" {
			sr.ProjectDir = req.ProjectPath
		}
		sr.WorkflowID = req.Workflow.ID
		sr.WorkflowLayer, sr.WorkflowVersion, sr.WorkflowRisk = req.Workflow.Layer, req.Workflow.Version, req.Workflow.Risk
		sr.LocationKind = string(ps.Location.Kind)
		sr.ParentSessionID = req.ParentSessionID
		sr.Tickets = ps.Tickets
		if created {
			sr.WorktreeAuto, sr.WorktreeBranch = true, ps.Location.Branch
		}
		if kept != nil {
			// The new session continues the work of the stopped one.
			sr.ParentSessionID, sr.Handoff = kept.ID, handoff
			if prev, err := s.Session(ctx, kept.ID); err == nil {
				sr.WorktreeAuto, sr.WorktreeBranch = prev.WorktreeAuto, prev.WorktreeBranch
			}
		}
		res, err := s.StartSession(ctx, sr)
		if res != nil && res.SessionID != "" {
			res.Stashed = stashed
			res.Deleted = deleted
			if kept != nil {
				res.Replaced = kept.ID
			}
			out = append(out, res)
		}
		if err != nil {
			return out, err
		}
		if s.OnTicketsStarted != nil && len(ps.Tickets) > 0 && res != nil {
			res.Notes = append(res.Notes, s.OnTicketsStarted(WithStartedSession(ctx, res.SessionID), req.Base.ProjectID, ps.Tickets)...)
		}
	}
	return out, nil
}

// launchLockPath is the launch lock of a directory of a project.
func (s *Service) launchLockPath(projectID, location string) string {
	sum := sha256.Sum256([]byte(projectID + "\x00" + filepath.Clean(location)))
	dir := s.LaunchLocksDir
	if dir == "" {
		dir = filepath.Join(filepath.Dir(s.ServersDir), "run", "launch")
	}
	return filepath.Join(dir, hex.EncodeToString(sum[:8])+".lock")
}

// RuntimeAvailability reports whether a runtime can host sessions now
// (local always can; an unknown runtime cannot).
func (s *Service) RuntimeAvailability(ctx context.Context, kind sessionspec.RuntimeKind) (ohruntime.Availability, error) {
	kind = runtimeKind(kind)
	if kind == sessionspec.RuntimeLocal {
		return ohruntime.Availability{OK: true}, nil
	}
	rt, ok := s.Runtimes[kind]
	if !ok || rt == nil {
		return ohruntime.Availability{}, fmt.Errorf("runsvc: runtime %q is not available", kind)
	}
	return rt.Available(ctx)
}

// volumeMountPoints warns about the relative cache volumes of a container
// run that are absent from a location (v5 finalisation, Q3-4).
func volumeMountPoints(base StartRequest, locs []Location) []Warning {
	if base.Runtime != sessionspec.RuntimeContainer {
		return nil
	}
	var out []Warning
	seen := map[string]bool{}
	for _, l := range locs {
		for _, v := range base.Volumes {
			if v == "" || filepath.IsAbs(v) {
				continue
			}
			p := filepath.Join(l.Path, v)
			if seen[p] {
				continue
			}
			seen[p] = true
			if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
				out = append(out, Warning{Code: WarnVolumeMountPoint, Args: []any{v, p}})
			}
		}
	}
	return out
}
