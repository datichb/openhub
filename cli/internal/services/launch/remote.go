package launch

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/remote"
	"github.com/datichb/openhub/cli/internal/runsvc"
	remotesvc "github.com/datichb/openhub/cli/internal/services/remote"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// Remote launches (`oh run … --runtime remote`): the workflow is resolved
// and its bundle built as for a local launch, then each session is sent to
// the oh-runner project of the project's GitLab group instead of a local
// server.

// RemotePrep is what a remote launch adds to a prepared run.
type RemotePrep struct {
	Target      config.RemoteTarget
	Checkpoints []remote.ManifestCheckpoint
	tickets     [][]string       // per session
	inputs      []map[string]any // per session
}

// ErrRemoteDisabled is the translated refusal of a remote entry point while
// the remote mode is not enabled.
func ErrRemoteDisabled() error {
	return errors.New(i18n.T("cmd.remote.experimental.disabled"))
}

// RequireRemoteExperimental refuses when the remote mode is not enabled.
func RequireRemoteExperimental(cfg *config.Config) error {
	if !RemoteExperimental(cfg) {
		return ErrRemoteDisabled()
	}
	return nil
}

// RemoteTargetFor returns the target of a project (explicit entry, else
// instance and group of its origin remote).
func RemoteTargetFor(cfg *config.Config, project *domain.Project) (*config.RemoteTarget, error) {
	if len(cfg.Remote.Targets) == 0 {
		return nil, errors.New(i18n.T("cmd.remote.send.no_target_configured"))
	}
	var gr config.GitRemote
	if out, err := exec.Command("git", "-C", project.Path, "remote", "get-url", "origin").Output(); err == nil {
		gr, _ = config.ParseGitRemote(string(out))
	}
	t := cfg.Remote.MatchTarget(project.ID, gr)
	if t == nil {
		return nil, errors.New(i18n.Tf("cmd.remote.send.no_target", project.Name, gr.Host+"/"+gr.Path))
	}
	return t, nil
}

// PrepareRemote checks that the workflow can run remotely (P5-T04) and
// chooses the target.
func PrepareRemote(ctx context.Context, d Deps, project *domain.Project, res *workflowsvc.Resolution) (*RemotePrep, error) {
	if err := RequireRemoteExperimental(d.Config); err != nil {
		return nil, err
	}
	cps, err := res.RemotePlan(i18n.Locale())
	var fe *workflowsvc.RemoteForbiddenError
	switch {
	case errors.Is(err, workflowsvc.ErrPublishRemote):
		return nil, errors.New(i18n.Tf("cmd.remote.send.publish", res.Spec.ID))
	case errors.Is(err, workflowsvc.ErrRemoteNotAllowed):
		return nil, errors.New(i18n.Tf("cmd.remote.send.not_allowed", res.Spec.ID))
	case errors.As(err, &fe):
		return nil, errors.New(i18n.Tf("cmd.remote.send.forbidden_checkpoint", fe.Checkpoint, res.Mode))
	case err != nil:
		return nil, err
	}
	t, err := RemoteTargetFor(d.Config, project)
	if err != nil {
		return nil, err
	}
	if d.Secrets != nil {
		if trig, _ := d.Secrets.Get(ctx, t.TriggerKeyOrDefault()); trig == "" {
			return nil, errors.New(i18n.Tf("cmd.remote.send.no_trigger", t.Name))
		}
	}
	rp := &RemotePrep{Target: *t}
	for _, c := range cps {
		rp.Checkpoints = append(rp.Checkpoints, remote.ManifestCheckpoint{ID: c.ID, Label: c.Label, Policy: string(c.Policy), Mandatory: c.Mandatory})
	}
	return rp, nil
}

// RemoteTickets are the Beads tickets of a session: the ticket input value.
func RemoteTickets(values map[string]any, ticketInput string) []string {
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

// TeamClaims adapts the project team-state to remotesvc.TeamClaims.
type TeamClaims struct{ Repo *teamstate.Repo }

func (t TeamClaims) Claim(ctx context.Context, project, ticket, member string) (bool, error) {
	existing, err := t.Repo.CreateClaim(ctx, teamstate.Claim{TicketID: ticket, Project: project, ClaimedBy: member})
	if errors.Is(err, teamstate.ErrClaimExists) && existing != nil {
		if existing.ClaimedBy != member {
			return false, fmt.Errorf("%w (%s)", remotesvc.ErrTicketTaken, existing.ClaimedBy)
		}
		return false, nil
	}
	return err == nil, err
}

func (t TeamClaims) Release(ctx context.Context, project, ticket string) error {
	return t.Repo.ReleaseClaim(ctx, project, ticket)
}

func (t TeamClaims) SetRemote(ctx context.Context, project, ticket string, r teamstate.ClaimRemote) error {
	return t.Repo.SetClaimRemote(ctx, project, ticket, r)
}

// HTTPSRepoURL turns a git remote into its HTTPS clone URL (the runner
// authenticates with a token).
func HTTPSRepoURL(raw string) string {
	gr, err := config.ParseGitRemote(raw)
	if err != nil {
		return ""
	}
	return "https://" + gr.Host + "/" + gr.Path + ".git"
}

// sendRemote sends each session of the plan (P5-T05 → T08).
func sendRemote(ctx context.Context, p *Prepared, progress func(Event)) ([]*runsvc.StartResult, error) {
	d, rp := p.deps, p.Remote
	if err := RequireRemoteExperimental(d.Config); err != nil {
		return nil, err
	}
	emit(progress, Event{Kind: EventWarning, Text: i18n.T("cmd.remote.experimental.warning")})
	svc := d.RemoteSender()
	base := p.Plan.Request.Base

	req := remotesvc.SendRequest{
		Target: rp.Target, ProjectID: p.Project.ID, Project: p.Project.Name, ProjectDir: p.Project.Path,
		MemberID: base.MemberID, Bundle: base.Bundle, Mode: base.Mode, EntryAgent: base.Bundle.Spec.EntryAgent,
		Workflow: remote.ManifestWorkflow{ID: p.Plan.Request.Workflow.ID, Layer: p.Plan.Request.Workflow.Layer,
			Version: p.Plan.Request.Workflow.Version, Risk: p.Plan.Request.Workflow.Risk},
		Checkpoints: rp.Checkpoints, Provider: base.Provider, AllowedModels: base.AllowedModels, MaxTokens: base.MaxTokens,
		BeadsAllow: base.BeadsAllow,
	}
	if ec := p.Project.ExecConfig; ec != nil {
		req.Dockerfile = ec.Dockerfile // build args are not sent (commit-addressed image)
	}
	if team := config.ResolveTeamForProject(d.Config, p.Project); team.Enabled {
		if repo, err := d.TeamRepo(ctx, p.Project); err == nil {
			req.Team = TeamClaims{Repo: repo}
			req.TeamInfo = &remote.ManifestTeam{ID: team.TeamID, Repo: HTTPSRepoURL(team.StateRepo), MemberID: team.MemberID}
		}
	}

	var out []*runsvc.StartResult
	for i, ps := range p.Plan.Sessions {
		r := req
		r.Title, r.Prompt, r.Branch = ps.Title, ps.Prompt, ps.Branch
		if r.Branch == "" {
			r.Branch = ps.Location.Branch
		}
		r.Tickets, r.Inputs = rp.tickets[i], rp.inputs[i]
		res, err := svc.Send(ctx, r)
		if err != nil {
			return out, RemoteSendError(err)
		}
		for _, w := range res.Warnings {
			emit(progress, Event{Kind: EventWarning, Text: i18n.T("cmd.remote.send.warn." + w)})
		}
		emit(progress, Event{Kind: EventSuccess, Text: i18n.Tf("cmd.remote.send.sent", res.SessionID, res.Ref.Pipeline, res.Ref.PipelineURL)})
		out = append(out, &runsvc.StartResult{SessionID: res.SessionID})
	}
	if len(out) > 0 && d.Daemon != nil {
		// The daemon follows the pipelines while oh is closed (BL-17).
		if _, err := d.Daemon(ctx); err != nil {
			slog.Debug("oh daemon not started for remote tracking", "error", err)
		}
	}
	return out, nil
}

// RemoteSendError localizes the errors of a send.
func RemoteSendError(err error) error {
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
		{remotesvc.ErrNoJournalKey, "cmd.remote.send.err.no_journal_key"},
	} {
		if errors.Is(err, e.err) {
			return fmt.Errorf("%s (%w)", i18n.T(e.key), err)
		}
	}
	return err
}
