package fanout

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// Package fanout runs the fan-out of a session (R6, E3, experimental): the
// driver launches the children, oh merges them into the integration branch
// (merger on a conflict) and, once the merge checkpoint of the parent is
// validated and verified, merges the integration branch into the branch
// of the parent and closes the tickets merged (Integrate, in the daemon).

// Store is what the fan-out reads and writes of the sessions.
type Store interface {
	domain.SessionStore
	domain.SessionFanoutStore
}

// Integrator merges the integration branch of a fan-out (merge checkpoint).
type Integrator struct {
	Sessions Store
	// Record records the tickets closed by oh in the outputs of the parent
	// (`record: closed`, nil = none).
	Record func(ctx context.Context, sessionID string, tickets []string)
	// Close closes a ticket in dir (nil = bd close).
	Close func(ctx context.Context, dir, ticket, reason string) error
}

// Integrate merges the integration branch of the fan-out of sess into the
// branch of sess (fast forward when it can) and closes the ticket of each
// child merged, as oh. It refuses a branch of the parent with changes not
// committed, or an integration branch moved by someone else than oh. The
// fan-out record is marked merged, with its events.
func (in *Integrator) Integrate(ctx context.Context, sess *domain.Session, checkpoint string) (string, error) {
	f := sess.Fanout
	if f == nil || f.Role != domain.RoleParent || f.Integration == nil {
		return "", errors.New(i18n.T("cmd.fanout.not_started"))
	}
	ig := f.Integration
	head, err := Head(ctx, ig.Path)
	if err != nil {
		return "", err
	}
	if head != ig.Head {
		return "", errors.New(i18n.Tf("cmd.fanout.integration_moved", ig.Branch, short(ig.Head), short(head)))
	}
	if dirty, err := Dirty(ctx, sess.LaunchPath); err != nil {
		return "", err
	} else if dirty {
		return "", errors.New(i18n.T("cmd.fanout.parent_dirty"))
	}
	if err := FastForward(ctx, sess.LaunchPath, ig.Branch, fmt.Sprintf("Merge %s (fan-out %s)", ig.Branch, sess.ID)); err != nil {
		return "", err
	}
	merged, err := Head(ctx, sess.LaunchPath)
	if err != nil {
		return "", err
	}
	rec := *f
	rec.Status, rec.Merged = domain.FanoutMerged, merged
	rec.AddEvent(domain.FanoutEventIntegrated, "", sess.ID, checkpoint+" "+short(merged))
	closeTicket := in.Close
	if closeTicket == nil {
		closeTicket = bdClose
	}
	var closed, failed []string
	for _, c := range rec.Children {
		if c.State != domain.ChildMerged {
			continue
		}
		reason := fmt.Sprintf("Merged by oh in %s (fan-out %s, %s)", short(c.Merge), sess.ID, short(merged))
		if err := closeTicket(ctx, sess.LaunchPath, c.Ticket, reason); err != nil {
			failed = append(failed, c.Ticket)
			rec.AddEvent(domain.FanoutEventFailed, c.Ticket, "", "bd close: "+err.Error())
			continue
		}
		closed = append(closed, c.Ticket)
		rec.AddEvent(domain.FanoutEventClosed, c.Ticket, "", reason)
	}
	if err := in.Sessions.SetSessionFanout(ctx, sess.ID, &rec); err != nil {
		return "", err
	}
	if in.Record != nil && len(closed) > 0 {
		in.Record(ctx, sess.ID, closed)
	}
	// Everything is on the branch of the parent now: the worktrees and the
	// branches of the fan-out go (an unmerged child keeps its branch).
	done := *sess
	done.Fanout = &rec
	if _, err := Clean(ctx, in.Sessions, &done, CleanOptions{}); err != nil {
		slog.Warn("fan-out: clean-up after the merge", "session", sess.ID, "error", err)
	}
	msg := i18n.Tf("cmd.fanout.integrated", ig.Branch, short(merged), strings.Join(closed, ", "))
	if len(failed) > 0 {
		msg += " " + i18n.Tf("cmd.fanout.close_failed", strings.Join(failed, ", "))
	}
	return msg, nil
}

func bdClose(ctx context.Context, dir, ticket, reason string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	out, err := beads.BdCommand(ctx, "-C", dir, "close", ticket, "--reason", reason).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func short(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}
