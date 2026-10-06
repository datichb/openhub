package cmd

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/launcher"
	"github.com/datichb/openhub/cli/internal/remote"
	"github.com/datichb/openhub/cli/internal/runsvc"
	remotesvc "github.com/datichb/openhub/cli/internal/services/remote"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// Remote launches (`oh run … --runtime remote`): the workflow is resolved
// and its bundle built as for a local launch, then each session is sent to
// the oh-runner project of the project's GitLab group instead of a local
// server.

// remotePrep is what a remote launch adds to a prepared run.
type remotePrep struct {
	target      config.RemoteTarget
	checkpoints []remote.ManifestCheckpoint
	tickets     [][]string       // per session
	inputs      []map[string]any // per session
}

// remoteTargetFor returns the target of a project (explicit entry, else
// instance and group of its origin remote).
func remoteTargetFor(a *app.App, project *domain.Project) (*config.RemoteTarget, error) {
	if len(a.Config.Remote.Targets) == 0 {
		return nil, errors.New(i18n.T("cmd.remote.send.no_target_configured"))
	}
	var gr config.GitRemote
	if out, err := exec.Command("git", "-C", project.Path, "remote", "get-url", "origin").Output(); err == nil {
		gr, _ = config.ParseGitRemote(string(out))
	}
	t := a.Config.Remote.MatchTarget(project.ID, gr)
	if t == nil {
		return nil, errors.New(i18n.Tf("cmd.remote.send.no_target", project.Name, gr.Host+"/"+gr.Path))
	}
	return t, nil
}

// prepareRemote checks that the workflow can run remotely (P5-T04) and
// chooses the target.
func prepareRemote(ctx context.Context, a *app.App, project *domain.Project, res *workflowsvc.Resolution) (*remotePrep, error) {
	cps, err := res.RemotePlan(i18n.Locale())
	var fe *workflowsvc.RemoteForbiddenError
	switch {
	case errors.Is(err, workflowsvc.ErrRemoteNotAllowed):
		return nil, errors.New(i18n.Tf("cmd.remote.send.not_allowed", res.Spec.ID))
	case errors.As(err, &fe):
		return nil, errors.New(i18n.Tf("cmd.remote.send.forbidden_checkpoint", fe.Checkpoint, res.Mode))
	case err != nil:
		return nil, err
	}
	t, err := remoteTargetFor(a, project)
	if err != nil {
		return nil, err
	}
	if a.Secrets != nil {
		if trig, _ := a.Secrets.Get(ctx, t.TriggerKeyOrDefault()); trig == "" {
			return nil, errors.New(i18n.Tf("cmd.remote.send.no_trigger", t.Name))
		}
	}
	rp := &remotePrep{target: *t}
	for _, c := range cps {
		rp.checkpoints = append(rp.checkpoints, remote.ManifestCheckpoint{ID: c.ID, Label: c.Label, Policy: string(c.Policy), Mandatory: c.Mandatory})
	}
	return rp, nil
}

// remoteTickets are the Beads tickets of a session: the ticket input value.
func remoteTickets(values map[string]any, ticketInput string) []string {
	if ticketInput == "" {
		return nil
	}
	var out []string
	switch v := values[ticketInput].(type) {
	case string:
		for _, t := range strings.Split(v, ",") {
			if t = strings.TrimSpace(t); t != "" {
				out = append(out, t)
			}
		}
	case []string:
		out = append(out, v...)
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok && s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

// teamClaims adapts the project team-state to remotesvc.TeamClaims.
type teamClaims struct{ repo *teamstate.Repo }

func (t teamClaims) Claim(ctx context.Context, project, ticket, member string) (bool, error) {
	existing, err := t.repo.CreateClaim(ctx, teamstate.Claim{TicketID: ticket, Project: project, ClaimedBy: member})
	if errors.Is(err, teamstate.ErrClaimExists) && existing != nil {
		if existing.ClaimedBy != member {
			return false, fmt.Errorf("%w (%s)", remotesvc.ErrTicketTaken, existing.ClaimedBy)
		}
		return false, nil
	}
	return err == nil, err
}

func (t teamClaims) Release(ctx context.Context, project, ticket string) error {
	return t.repo.ReleaseClaim(ctx, project, ticket)
}

func (t teamClaims) SetRemote(ctx context.Context, project, ticket string, r teamstate.ClaimRemote) error {
	return t.repo.SetClaimRemote(ctx, project, ticket, r)
}

// httpsRepoURL turns a git remote into its HTTPS clone URL (the runner
// authenticates with a token).
func httpsRepoURL(raw string) string {
	gr, err := config.ParseGitRemote(raw)
	if err != nil {
		return ""
	}
	return "https://" + gr.Host + "/" + gr.Path + ".git"
}

// newRemoteSendService wires the remote service for sending.
func newRemoteSendService(a *app.App) *remotesvc.Service {
	svc := newRemoteService(a)
	svc.Sessions = a.Sessions
	svc.Remote = sqlite.NewRemoteStore(store)
	if v5Adapter != nil {
		svc.ToolVersion = v5Adapter.Ver
	}
	return svc
}

// sendRemote sends each session of the plan (P5-T05 → T08).
func (p *preparedRun) sendRemote(ctx context.Context, a *app.App, ui launcher.LaunchUI) ([]*runsvc.StartResult, error) {
	rp := p.remote
	svc := newRemoteSendService(a)
	base := p.plan.Request.Base

	req := remotesvc.SendRequest{
		Target: rp.target, ProjectID: p.project.ID, Project: p.project.Name, ProjectDir: p.project.Path,
		MemberID: base.MemberID, Bundle: base.Bundle, Mode: base.Mode, EntryAgent: base.Bundle.Spec.EntryAgent,
		Workflow: remote.ManifestWorkflow{ID: p.plan.Request.Workflow.ID, Layer: p.plan.Request.Workflow.Layer,
			Version: p.plan.Request.Workflow.Version, Risk: p.plan.Request.Workflow.Risk},
		Checkpoints: rp.checkpoints, Provider: base.Provider, AllowedModels: base.AllowedModels, MaxTokens: base.MaxTokens,
		BeadsAllow: base.BeadsAllow,
	}
	if team := config.ResolveTeamForProject(a.Config, p.project); team.Enabled {
		if repo, err := ensureTeamRepoForProject(ctx, a, p.project); err == nil {
			req.Team = teamClaims{repo: repo}
			req.TeamInfo = &remote.ManifestTeam{ID: team.TeamID, Repo: httpsRepoURL(team.StateRepo), MemberID: team.MemberID}
		}
	}

	var out []*runsvc.StartResult
	for i, ps := range p.plan.Sessions {
		r := req
		r.Title, r.Prompt, r.Branch = ps.Title, ps.Prompt, ps.Branch
		if r.Branch == "" {
			r.Branch = ps.Location.Branch
		}
		r.Tickets, r.Inputs = rp.tickets[i], rp.inputs[i]
		res, err := svc.Send(ctx, r)
		if err != nil {
			return out, remoteSendError(err)
		}
		for _, w := range res.Warnings {
			ui.Notify(i18n.T("cmd.remote.send.warn."+w), launcher.LevelWarning)
		}
		ui.Notify(i18n.Tf("cmd.remote.send.sent", res.SessionID, res.Ref.Pipeline, res.Ref.PipelineURL), launcher.LevelSuccess)
		out = append(out, &runsvc.StartResult{SessionID: res.SessionID})
	}
	return out, nil
}

// remoteSendError localizes the errors of a send.
func remoteSendError(err error) error {
	for _, e := range []struct {
		err error
		key string
	}{
		{remotesvc.ErrNoToken, "cmd.remote.send.err.no_token"},
		{remotesvc.ErrNoTrigger, "cmd.remote.send.err.no_trigger"},
		{remotesvc.ErrPipelineOutdated, "cmd.remote.send.err.pipeline_outdated"},
		{remotesvc.ErrDetached, "cmd.remote.send.err.detached"},
		{remotesvc.ErrUnpushed, "cmd.remote.send.err.unpushed"},
		{remotesvc.ErrForeignRemote, "cmd.remote.send.err.foreign_remote"},
		{remotesvc.ErrNoBinary, "cmd.remote.send.err.no_binary"},
		{remotesvc.ErrTicketTaken, "cmd.remote.send.err.ticket_taken"},
		{remotesvc.ErrDockerfileNotCommitted, "cmd.remote.send.err.dockerfile"},
	} {
		if errors.Is(err, e.err) {
			return fmt.Errorf("%s (%w)", i18n.T(e.key), err)
		}
	}
	return err
}
