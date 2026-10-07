package runsvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

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
	WarnAutoWorktree   = "auto_worktree"   // O10: another writing session uses the directory
	WarnSharedLocation = "shared_location" // writers share a directory (no git: no worktree)
	WarnDirty          = "dirty"           // uncommitted changes where a writer starts
	WarnPending        = "pending"         // decisions already waiting in other sessions
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
	// ParentSessionID chains the sessions to a previous one (O7).
	ParentSessionID string
}

// PlannedSession is a session of a plan. Prompt is rendered by the caller
// once the location is known (it may use .oh.location).
type PlannedSession struct {
	PlannedInput
	Location Location `json:"location"`
	Prompt   string   `json:"-"`
}

// RunPlan is what Start will do.
type RunPlan struct {
	Request  RunRequest       `json:"-"`
	Sessions []PlannedSession `json:"sessions"`
	Warnings []Warning        `json:"warnings,omitempty"`
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
	locs, warns, err := s.planLocations(ctx, req.Base.ProjectID, req.ProjectPath, req.Location, writes, branches)
	if err != nil {
		return nil, err
	}
	plan := &RunPlan{Request: req, Warnings: warns}
	for i, in := range inputs {
		plan.Sessions = append(plan.Sessions, PlannedSession{PlannedInput: in, Location: locs[i]})
	}
	plan.Warnings = append(plan.Warnings, volumeMountPoints(req.Base, locs)...)
	if s.Decisions != nil {
		if open, err := s.Decisions.ListOpen(ctx, domain.DecisionFilter{}); err == nil && len(open) > 0 {
			plan.Warnings = append(plan.Warnings, Warning{Code: WarnPending, Args: []any{len(open)}})
		}
	}
	return plan, nil
}

// Start runs a plan: worktrees are created, then each session is started in
// the server group of the bundle. A launch lock per directory refuses a
// second launch while one is being prepared there (ErrLaunchInProgress).
// Sessions started before an error are returned with it.
func (s *Service) Start(ctx context.Context, plan *RunPlan) ([]*StartResult, error) {
	req := plan.Request
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
		if ps.Location.Create {
			p, err := s.createWorktree(req.ProjectPath, ps.Location.Branch)
			if err != nil {
				return out, fmt.Errorf("worktree %s: %w", ps.Location.Branch, err)
			}
			ps.Location.Path, ps.Location.Create = p, false
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
		res, err := s.StartSession(ctx, sr)
		if res != nil && res.SessionID != "" {
			out = append(out, res)
		}
		if err != nil {
			return out, err
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
