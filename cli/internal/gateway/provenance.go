package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ProvenanceSource is the producer of the provenance events oh records.
const ProvenanceSource = "oh"

// Provenance kinds and reference kinds of bd (bd provenance record).
const (
	ProvClaim   = "claim"
	ProvCommit  = "commit"
	ProvHandoff = "handoff"

	RefWorkID = "work-id"
	RefGitSHA = "git-sha"
	RefPR     = "pr"
)

// Provenance is an event bound to a ticket: what oh saw happen to it
// (claimed by a session, closed on a commit, handed off in a merge
// request). bd keeps an append-only log and recording the same event twice
// is a no-op (its id derives from source, issue, kind and ref).
type Provenance struct {
	Issue   string
	Kind    string
	RefKind string
	Ref     string
	Actor   string
	Payload map[string]string
}

// ProvenanceArgs is the bd command recording ev.
func ProvenanceArgs(ev Provenance) []string {
	argv := []string{"provenance", "record", "--issue=" + ev.Issue, "--kind=" + ev.Kind, "--source=" + ProvenanceSource}
	if ev.Ref != "" {
		argv = append(argv, "--ref="+ev.Ref)
	}
	if ev.RefKind != "" {
		argv = append(argv, "--ref-kind="+ev.RefKind)
	}
	if ev.Actor != "" {
		argv = append(argv, "--actor="+ev.Actor)
	}
	if len(ev.Payload) > 0 {
		if data, err := json.Marshal(ev.Payload); err == nil {
			argv = append(argv, "--payload="+string(data))
		}
	}
	return argv
}

// RecordProvenance records ev in the Beads workspace of a grant, with the
// real bd (never from an agent: `provenance record` is a write no
// allow-list of oh grants by default). Nothing is recorded for a remote
// job, whose bd is a journal: the machine records the events of the job
// when it fetches the session.
func (b *Beads) RecordProvenance(ctx context.Context, g Grant, ev Provenance) error {
	if ev.Issue == "" || ev.Kind == "" || ev.Ref == "" {
		return errors.New("provenance: issue, kind and ref are required")
	}
	return b.runInternal(ctx, g, ProvenanceArgs(ev)...)
}

// Heartbeat refreshes the Beads lease of a ticket the session claimed (bd
// leases expire after a few minutes: `bd reclaim` would give the ticket
// back to the pool while the session still works on it). Only the owner of
// the lease may refresh it: the person of the grant, actor of its claims.
func (b *Beads) Heartbeat(ctx context.Context, g Grant, id string) error {
	argv := []string{"heartbeat", id}
	if h := cleanActorPart(g.Human); h != "" {
		argv = append([]string{"--actor=" + h}, argv...)
	}
	return b.runInternal(ctx, g, argv...)
}

// runInternal runs a command of oh itself (not of an agent) with the real
// bd, in the Beads workspace of a grant. Nothing runs for a remote job,
// whose bd is a journal.
func (b *Beads) runInternal(ctx context.Context, g Grant, argv ...string) error {
	if journalMode() || (g.Base != nil && g.Base.Dir == journalBase) {
		return nil
	}
	base := b.base(ctx, g)
	if base == nil || base.Dir == journalBase {
		return errors.New("no Beads workspace")
	}
	bin, err := b.binary()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, argv...)
	cmd.Dir = g.Location
	cmd.Env = beadsEnv(os.Environ(), base.Dir, false)
	if out, err := cmd.CombinedOutput(); err != nil {
		name := argv[0]
		for _, a := range argv {
			if !strings.HasPrefix(a, "-") {
				name = a
				break
			}
		}
		return fmt.Errorf("bd %s: %s: %w", name, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// ActorOf is the Beads actor of a grant's writes (see Actor).
func (b *Beads) ActorOf(ctx context.Context, g Grant) string { return b.actor(ctx, g, false) }
