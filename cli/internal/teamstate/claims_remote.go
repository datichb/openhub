package teamstate

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// ClaimRemote is the progress of a remote session on a claimed ticket.
type ClaimRemote struct {
	Session     string    `toml:"session"`
	Target      string    `toml:"target,omitempty"`   // remote target (oh-runner project)
	Pipeline    int64     `toml:"pipeline,omitempty"` // pipeline ID
	PipelineURL string    `toml:"pipeline_url,omitempty"`
	Status      string    `toml:"status"`         // sent | running | mr_ready | ready | failed | fetched
	Step        string    `toml:"step,omitempty"` // free text: current agent, checkpoint…
	MRURL       string    `toml:"mr_url,omitempty"`
	UpdatedAt   time.Time `toml:"updated_at"`
}

// Remote progress statuses.
const (
	RemoteSent    = "sent"
	RemoteRunning = "running"
	RemoteMRReady = "mr_ready"
	RemoteReady   = "ready"
	RemoteFailed  = "failed"
	RemoteFetched = "fetched"
)

// SetClaimRemote records the remote progress of a claim (commit + push).
// Returns ErrClaimNotFound if the claim does not exist.
func (r *Repo) SetClaimRemote(ctx context.Context, project, ticketID string, rem ClaimRemote) error {
	if _, err := SafeName(project); err != nil {
		return fmt.Errorf("invalid project name: %w", err)
	}
	if _, err := SafeName(ticketID); err != nil {
		return fmt.Errorf("invalid ticket ID: %w", err)
	}
	if rem.UpdatedAt.IsZero() {
		rem.UpdatedAt = time.Now().UTC()
	}
	return r.withWriteLock(ctx, func(ctx context.Context) error {
		rel, err := r.updateClaimFieldsLocal(project, ticketID, func(c *Claim) error {
			if rem.MRURL != "" {
				c.MRURL = rem.MRURL
			}
			c.Remote = &rem
			return nil
		})
		if err != nil {
			return err
		}
		slog.Info("teamstate.claim.remote", "project", project, "ticket", ticketID, "status", rem.Status)
		return r.commitAndPush(ctx, fmt.Sprintf("remote: %s/%s %s", project, ticketID, rem.Status), rel)
	})
}
