package runner

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/datichb/openhub/cli/internal/remote"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// OpenTeamState clones the team-state of the claims with the team-state
// token into dir (whose parent must be private to the runner). The token is
// written in the clone's own git config (never in a command line), so that
// the team-state pulls and pushes authenticate.
func OpenTeamState(ctx context.Context, team *remote.ManifestTeam, token, dir string, secrets Secrets) (*teamstate.Repo, error) {
	if team == nil || team.Repo == "" {
		return nil, errors.New("no team-state")
	}
	if token == "" {
		return nil, fmt.Errorf("CI variable %s is missing: the claims are not updated", remote.VarTeamStateToken)
	}
	g := Git{Token: token, Secrets: secrets}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if _, err := g.run(ctx, dir, "clone", "--quiet", team.Repo, "."); err != nil {
		return nil, err
	}
	cfg := filepath.Join(dir, ".git", "config")
	f, err := os.OpenFile(cfg, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	auth := base64.StdEncoding.EncodeToString([]byte("oauth2:" + token))
	_, werr := fmt.Fprintf(f, "[http]\n\textraHeader = Authorization: Basic %s\n[user]\n\tname = oh runner\n\temail = oh-runner@users.noreply.oh\n", auth)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return nil, werr
	}
	_ = os.Chmod(cfg, 0o600)
	return teamstate.NewRepo(team.Repo, dir), nil
}

// claimSetter is the part of teamstate.Repo used for progress.
type claimSetter interface {
	SetClaimRemote(ctx context.Context, project, ticketID string, rem teamstate.ClaimRemote) error
}

// TeamProgress publishes the progress of a remote session in the claims of
// its tickets (P5-T13). Failures are logged, never fatal.
type TeamProgress struct {
	Repo        claimSetter
	Project     string // oh project ID
	Tickets     []string
	Session     string
	Target      string
	Pipeline    int64
	PipelineURL string
	Now         func() time.Time
}

// Set implements Progress.
func (p *TeamProgress) Set(ctx context.Context, status, step, mrURL string) {
	now := time.Now
	if p.Now != nil {
		now = p.Now
	}
	for _, t := range p.Tickets {
		err := p.Repo.SetClaimRemote(ctx, p.Project, t, teamstate.ClaimRemote{Session: p.Session, Target: p.Target,
			Pipeline: p.Pipeline, PipelineURL: p.PipelineURL, Status: status, Step: step, MRURL: mrURL, UpdatedAt: now().UTC()})
		if err != nil {
			slog.Warn("runner: claim progress not published", "ticket", t, "err", err)
		}
	}
}
