package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/remote"
	"github.com/datichb/openhub/cli/internal/remote/ciconfig"
	"github.com/datichb/openhub/cli/internal/remote/gitlab"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// Errors of a send.
var (
	ErrNoTarget         = errors.New("no remote target for this project")
	ErrNoTrigger        = errors.New("no trigger token on this machine")
	ErrPipelineOutdated = errors.New("the oh-runner pipeline is missing or from another oh version")
	ErrDetached         = errors.New("the project is not on a branch")
	ErrUnpushed         = errors.New("the branch has commits that are not pushed")
	ErrForeignRemote    = errors.New("the project remote is not on the instance of the target")
	ErrNoBinary         = errors.New("development build: no oh binary uploaded for this version")
	ErrTicketTaken      = errors.New("ticket already claimed by another member")
)

// TeamClaims are the team-state claims of the project (nil: no team).
type TeamClaims interface {
	// Claim reserves ticket for member; created is false when member
	// already held it. Another member's claim is ErrTicketTaken.
	Claim(ctx context.Context, project, ticket, member string) (created bool, err error)
	Release(ctx context.Context, project, ticket string) error
	SetRemote(ctx context.Context, project, ticket string, r teamstate.ClaimRemote) error
}

// SendRequest is a remote session to start.
type SendRequest struct {
	Target     config.RemoteTarget
	ProjectID  string // oh project
	Project    string // oh project name
	ProjectDir string // base directory of the project on the machine
	Dockerfile string // configured dev Dockerfile ("" = detected)
	MemberID   *string

	Bundle     *bundle.Bundle
	Workflow   remote.ManifestWorkflow
	Mode       string
	Title      string
	EntryAgent string
	// Prompt is rendered for remote.WorkDir(Project).
	Prompt      string
	Inputs      map[string]any
	Branch      string // branch the job pushes
	Checkpoints []remote.ManifestCheckpoint

	Provider      string
	AllowedModels []string
	MaxTokens     int64
	BeadsAllow    []string
	Tickets       []string

	Team     TeamClaims
	TeamInfo *remote.ManifestTeam
}

// SendResult is a started remote session.
type SendResult struct {
	SessionID string
	Ref       domain.RemoteRef
	// Warnings are stable codes (cmd.remote.send.warn.<code>).
	Warnings []string
}

// Send warnings.
const (
	WarnDirty      = "dirty"       // uncommitted changes are not sent
	WarnImageBuild = "image_build" // the project image is built first
	WarnClaimLost  = "claim_progress"
)

// WorkDir is the job location of a project (prompt rendering).
func WorkDir(project string) string {
	name := imageSlug(project)
	if name == "" {
		name = "project"
	}
	return path.Join(remote.WorkRoot, name)
}

var schemaRe = regexp.MustCompile(`pipeline schema (\d+)`)

// Send reserves the tickets, uploads the bundle and the session envelope
// (with the Beads snapshot) and triggers the oh-runner pipeline (P5-T05 →
// T08). The reservations are undone when the pipeline cannot be started.
func (s *Service) Send(ctx context.Context, req SendRequest) (*SendResult, error) {
	t := req.Target
	if err := t.Validate(); err != nil {
		return nil, err
	}
	if req.Bundle == nil || req.Bundle.Spec.Hash == "" {
		return nil, errors.New("remote: the session has no bundle")
	}
	if s.Secrets == nil || s.Sessions == nil || s.Remote == nil {
		return nil, errors.New("remote: service not wired")
	}
	token, _ := s.Secrets.Get(ctx, t.TokenKeyOrDefault())
	if token == "" {
		return nil, ErrNoToken
	}
	trigger, _ := s.Secrets.Get(ctx, t.TriggerKeyOrDefault())
	if trigger == "" {
		return nil, ErrNoTrigger
	}
	c := s.client(t, token)
	res := &SendResult{}

	// oh-runner and its pipeline.
	runner, err := c.Project(ctx, runnerRef(t))
	if err != nil {
		return nil, fmt.Errorf("oh-runner project: %w", err)
	}
	branch := runner.DefaultBranch
	if branch == "" {
		branch = "main"
	}
	ci, err := c.File(ctx, fmt.Sprint(runner.ID), CIFile, branch)
	if err != nil || !bytes.HasPrefix(ci, []byte(ciconfig.Marker)) {
		return nil, ErrPipelineOutdated
	}
	if m := schemaRe.FindSubmatch(ci); len(m) < 2 || string(m[1]) != fmt.Sprint(remote.PipelineSchema) {
		return nil, ErrPipelineOutdated
	}
	rid := fmt.Sprint(runner.ID)

	// Target project and the commit the job starts from.
	g := s.git()
	rawRemote, err := g.RemoteURL(ctx, req.ProjectDir)
	if err != nil {
		return nil, err
	}
	gr, err := config.ParseGitRemote(rawRemote)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(stripPort(gr.Host), stripPort(t.Host())) {
		return nil, fmt.Errorf("%w (%s, %s)", ErrForeignRemote, gr.Host, t.Host())
	}
	proj, err := c.Project(ctx, gr.Path)
	if err != nil {
		return nil, fmt.Errorf("project %s: %w", gr.Path, err)
	}
	ref, err := g.CurrentBranch(ctx, req.ProjectDir)
	if err != nil {
		return nil, err
	}
	if ref == "" {
		return nil, ErrDetached
	}
	commit, err := g.FetchHead(ctx, req.ProjectDir, ref)
	if err != nil {
		return nil, fmt.Errorf("%w: %s (%v)", ErrUnpushed, ref, err)
	}
	head, err := g.Head(ctx, req.ProjectDir)
	if err != nil {
		return nil, err
	}
	if ok, err := g.IsAncestor(ctx, req.ProjectDir, head, commit); err != nil || !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnpushed, ref)
	}
	if dirty, _ := g.Dirty(ctx, req.ProjectDir); dirty {
		res.Warnings = append(res.Warnings, WarnDirty)
	}

	// oh binary of the job and image.
	vars := map[string]string{}
	identity := ""
	switch {
	case IsRelease(s.OhVersion):
		v := strings.TrimPrefix(s.OhVersion, "v")
		vars[remote.VarCLIVersion], identity = v, "release:"+v
	case t.Binaries[BinaryKey(s.OhVersion, archOf(t))] != "":
		sum := t.Binaries[BinaryKey(s.OhVersion, archOf(t))]
		u, err := c.GenericPackageURL(rid, remote.PackageCLI, sum, remote.CLIFile(archOf(t)))
		if err != nil {
			return nil, err
		}
		vars[remote.VarCLIURL], vars[remote.VarCLISHA256], identity = u, sum, "binary:"+sum
	default:
		return nil, ErrNoBinary
	}
	if s.ToolVersion == "" {
		return nil, errors.New("remote: opencode version unknown (adapter not detected)")
	}
	vars[remote.VarToolVersion] = s.ToolVersion
	identity += "+opencode:" + s.ToolVersion
	img, err := planImage(ctx, g, req.ProjectDir, req.Dockerfile, commit, runner.ContainerRegistryImagePrefix, proj.PathWithNamespace, identity, archOf(t))
	if err != nil {
		return nil, err
	}
	have, err := c.RegistryTagExists(ctx, rid, img.ImageRepo, img.Tag)
	if err != nil {
		return nil, fmt.Errorf("container registry of oh-runner: %w", err)
	}
	if !have {
		vars[remote.VarImageBuild] = "true"
		res.Warnings = append(res.Warnings, WarnImageBuild)
	}

	// Reservation (P5-T05), then the snapshot, which records the claimed
	// state of the tickets.
	sid := s.newSessionID()
	undo, err := s.reserve(ctx, req)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			undo()
		}
	}()
	snap, err := TakeSnapshot(ctx, s.beads(), req.ProjectDir, req.Tickets, s.now())
	if err != nil {
		return nil, fmt.Errorf("Beads snapshot: %w", err)
	}

	// Packages (P5-T07): the bundle by its hash, the envelope by its own.
	now := s.now().UTC()
	man := remote.Manifest{
		Schema: remote.ManifestSchema, SessionID: sid, OhVersion: s.OhVersion, SentAt: now, Target: t.Name,
		BundleHash: req.Bundle.Spec.Hash, Workflow: req.Workflow, Mode: req.Mode, Title: req.Title,
		EntryAgent: req.EntryAgent, Prompt: req.Prompt, Inputs: req.Inputs,
		Project: remote.ManifestProject{ID: proj.ID, Path: proj.PathWithNamespace, Name: req.Project, OhID: req.ProjectID, CloneURL: proj.HTTPURLToRepo},
		WorkDir: WorkDir(req.Project), Ref: ref, Commit: commit, Branch: req.Branch,
		Provider: req.Provider, AllowedModels: req.AllowedModels, MaxTokens: req.MaxTokens,
		BeadsAllow: req.BeadsAllow, Tickets: req.Tickets, Checkpoints: req.Checkpoints, Team: req.TeamInfo,
	}
	bundleURL, err := s.uploadBundle(ctx, c, rid, req.Bundle)
	if err != nil {
		return nil, err
	}
	sessionURL, err := s.uploadEnvelope(ctx, c, rid, man, snap)
	if err != nil {
		return nil, err
	}
	if err := s.saveEnvelope(sid, man, snap); err != nil {
		return nil, err
	}

	// Trigger (P5-T08).
	vars[remote.VarSessionID] = sid
	vars[remote.VarProjectID] = fmt.Sprint(proj.ID)
	vars[remote.VarProjectPath] = proj.PathWithNamespace
	vars[remote.VarRef] = ref
	vars[remote.VarCommit] = commit
	vars[remote.VarWorkflow] = req.Workflow.ID
	vars[remote.VarBundleURL] = bundleURL
	vars[remote.VarSessionURL] = sessionURL
	vars[remote.VarImage] = img.Image
	vars[remote.VarImageBase] = img.Base
	vars[remote.VarDockerfile] = img.Dockerfile
	pl, err := c.TriggerPipeline(ctx, rid, branch, trigger, vars)
	if err != nil {
		return nil, fmt.Errorf("triggering the oh-runner pipeline: %w", err)
	}
	ok = true

	rref := domain.RemoteRef{
		Target: t.Name, URL: t.URL, RunnerID: runner.ID, ProjectPath: proj.PathWithNamespace, ProjectID: proj.ID, ProjectDir: req.ProjectDir,
		Ref: ref, Commit: commit, Branch: req.Branch, Pipeline: pl.ID, PipelineURL: pl.WebURL,
		BundleURL: bundleURL, SessionURL: sessionURL, Image: img.Image, Tickets: req.Tickets,
		Status: domain.RemoteSent, SentAt: now, UpdatedAt: now,
	}
	if req.TeamInfo != nil {
		rref.TeamID = req.TeamInfo.ID
	}
	res.SessionID, res.Ref = sid, rref
	if err := s.persist(ctx, req, sid, rref); err != nil {
		return res, err
	}
	if req.Team != nil {
		for _, tk := range req.Tickets {
			if err := req.Team.SetRemote(ctx, req.ProjectID, tk, teamstate.ClaimRemote{Session: sid, Target: t.Name,
				Pipeline: pl.ID, PipelineURL: pl.WebURL, Status: teamstate.RemoteSent}); err != nil {
				slog.Warn("remote: claim progress not published", "ticket", tk, "err", err)
				res.Warnings = append(res.Warnings, WarnClaimLost)
			}
		}
	}
	return res, nil
}

func runnerRef(t config.RemoteTarget) string {
	if t.RunnerProjectID != 0 {
		return fmt.Sprint(t.RunnerProjectID)
	}
	return t.RunnerProjectPath()
}

func stripPort(h string) string {
	if i := strings.LastIndexByte(h, ':'); i >= 0 {
		return h[:i]
	}
	return h
}

// reserve claims the tickets in Beads and in the team-state. The returned
// function undoes what this call did (not the reservations that existed).
func (s *Service) reserve(ctx context.Context, req SendRequest) (func(), error) {
	var undos []func()
	undo := func() {
		for i := len(undos) - 1; i >= 0; i-- {
			undos[i]()
		}
	}
	b := s.beads()
	member := ""
	if req.MemberID != nil {
		member = *req.MemberID
	}
	for _, tk := range req.Tickets {
		before := ""
		if recs, err := b.Show(ctx, req.ProjectDir, tk); err == nil && len(recs) == 1 {
			if h, err := headOf(recs[0]); err == nil {
				before = h.Status
			}
		}
		if err := b.Claim(ctx, req.ProjectDir, tk); err != nil {
			undo()
			return nil, fmt.Errorf("%s: %w", tk, err)
		}
		if before != "in_progress" {
			tk := tk
			undos = append(undos, func() {
				if err := b.Unclaim(context.WithoutCancel(ctx), req.ProjectDir, tk); err != nil {
					slog.Warn("remote: bd unclaim failed", "ticket", tk, "err", err)
				}
			})
		}
		if req.Team == nil {
			continue
		}
		created, err := req.Team.Claim(ctx, req.ProjectID, tk, member)
		if err != nil {
			undo()
			return nil, fmt.Errorf("%s: %w", tk, err)
		}
		if created {
			tk := tk
			undos = append(undos, func() {
				if err := req.Team.Release(context.WithoutCancel(ctx), req.ProjectID, tk); err != nil {
					slog.Warn("remote: claim release failed", "ticket", tk, "err", err)
				}
			})
		}
	}
	return undo, nil
}

func (s *Service) uploadBundle(ctx context.Context, c *gitlab.Client, rid string, b *bundle.Bundle) (string, error) {
	const file = "bundle.tar.gz"
	hash := b.Spec.Hash
	u, err := c.GenericPackageURL(rid, remote.PackageBundle, hash, file)
	if err != nil {
		return "", err
	}
	exists, err := c.GenericPackageExists(ctx, rid, remote.PackageBundle, hash, file)
	if err != nil {
		return "", err
	}
	if exists {
		return u, nil
	}
	data, err := remote.PackDir(b.Dir)
	if err != nil {
		return "", fmt.Errorf("packing the bundle: %w", err)
	}
	if _, err := c.UploadGenericPackage(ctx, rid, remote.PackageBundle, hash, file, bytes.NewReader(data), int64(len(data))); err != nil {
		return "", fmt.Errorf("uploading the bundle: %w", err)
	}
	return u, nil
}

func (s *Service) uploadEnvelope(ctx context.Context, c *gitlab.Client, rid string, man remote.Manifest, snap *remote.Snapshot) (string, error) {
	const file = "session.tar.gz"
	mdata, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return "", err
	}
	sdata, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return "", err
	}
	data, err := remote.PackFiles(map[string][]byte{remote.ManifestFile: mdata, remote.SnapshotFile: sdata})
	if err != nil {
		return "", err
	}
	sum := remote.SHA256(data)
	if _, err := c.UploadGenericPackage(ctx, rid, remote.PackageSession, sum, file, bytes.NewReader(data), int64(len(data))); err != nil {
		return "", fmt.Errorf("uploading the session envelope: %w", err)
	}
	return c.GenericPackageURL(rid, remote.PackageSession, sum, file)
}

// persist records the session (state active, runtime remote) and its
// remote reference.
func (s *Service) persist(ctx context.Context, req SendRequest, sid string, ref domain.RemoteRef) error {
	now := s.now()
	title := req.Title
	ext := sid
	sess := &domain.Session{
		ID: sid, ProjectID: req.ProjectID, StartedAt: now, Status: domain.SessionStatusRunning,
		Provider: req.Provider, LaunchPath: req.ProjectDir, MemberID: req.MemberID, Platform: "remote",
		ExternalSessionID: &ext, Title: &title, Type: domain.SessionTypeHeadless,
		WorkflowID: req.Workflow.ID, EntryAgent: req.EntryAgent, BundleHash: req.Bundle.Spec.Hash,
		Runtime: string(sessionspec.RuntimeRemote), Mode: req.Mode, State: domain.RunActive, StateChangedAt: &now,
		WorkflowLayer: req.Workflow.Layer, WorkflowVersion: req.Workflow.Version, WorkflowRisk: req.Workflow.Risk,
		Location: "remote",
	}
	if err := s.Sessions.Create(ctx, sess); err != nil {
		return fmt.Errorf("recording the remote session: %w", err)
	}
	return s.Remote.SetRemoteRef(ctx, sid, ref)
}

// RemoteDir is where a remote session keeps its envelope and artifacts.
func (s *Service) RemoteDir(sessionID string) string {
	return filepath.Join(s.SessionsDir, sessionID, "remote")
}

// saveEnvelope keeps the manifest and the snapshot sent (replay, P5-T17).
func (s *Service) saveEnvelope(sid string, man remote.Manifest, snap *remote.Snapshot) error {
	if s.SessionsDir == "" {
		return nil
	}
	dir := s.RemoteDir(sid)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	for name, v := range map[string]any{remote.ManifestFile: man, remote.SnapshotFile: snap} {
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) git() Git {
	if s.Git != nil {
		return s.Git
	}
	return GitCLI{}
}

func (s *Service) beads() Beads {
	if s.Beads != nil {
		return s.Beads
	}
	return BdCLI{}
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) newSessionID() string {
	if s.NewSessionID != nil {
		return s.NewSessionID()
	}
	return sessionspec.NewSessionID()
}
