package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/remote"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
)

// CISecrets is the secret store of the job: the provider key of the jobs
// (OH_LLM_KEY) for every provider key name, nothing else. Only the daemon's
// credential proxy reads it.
type CISecrets struct{ LLMKey string }

var _ domain.SecretStore = CISecrets{}

// Get implements domain.SecretStore.
func (s CISecrets) Get(_ context.Context, key string) (string, error) {
	for _, n := range provider.AllProviders() {
		if base := provider.KeychainKey(n, ""); base != "" && (key == base || strings.HasPrefix(key, base+".")) {
			return s.LLMKey, nil
		}
	}
	return "", nil
}

// Set implements domain.SecretStore (read-only).
func (CISecrets) Set(context.Context, string, string) error {
	return errors.New("the secret store of the oh runner is read-only")
}

// Delete implements domain.SecretStore.
func (CISecrets) Delete(context.Context, string) error { return nil }

// List implements domain.SecretStore.
func (CISecrets) List(context.Context) ([]string, error) { return nil, nil }

// Decisions is what the responder needs from the SessionService.
type Decisions interface {
	ListOpen(ctx context.Context, f domain.DecisionFilter) ([]domain.Decision, error)
}

// Responder answers the decisions of a remote session by the workflow
// policy (S5), never by approving everything (--auto):
//   - checkpoint waiting in the mode, `remote: auto` → approved;
//   - checkpoint `remote: defer` → the session stops cleanly (MR ready, the
//     user continues locally after fetching);
//   - any other permission → rejected with a message;
//   - question → no default answer exists: the session stops ("question
//     pending"), answered locally;
//   - error, budget, circuit breaker → the session stops (failed).
type Responder struct {
	Decisions Decisions
	Decide    func(ctx context.Context, r sessionsvc.Reply) error
	// Policies are the checkpoints waiting in the mode, by id (lowercase).
	Policies map[string]remote.ManifestCheckpoint
	Now      func() time.Time
	Poll     time.Duration

	mu      sync.Mutex
	answers []remote.SummaryDecision
	stop    *Stop
}

// Stop is why the responder ended the session.
type Stop struct {
	Outcome  string // remote.OutcomeDeferred | OutcomeQuestion | OutcomeFailed
	Decision remote.SummaryDecision
	Reason   string
}

// NewResponder builds a responder for a manifest.
func NewResponder(d Decisions, decide func(context.Context, sessionsvc.Reply) error, m *remote.Manifest) *Responder {
	pol := map[string]remote.ManifestCheckpoint{}
	for _, c := range m.Checkpoints {
		pol[strings.ToLower(c.ID)] = c
	}
	return &Responder{Decisions: d, Decide: decide, Policies: pol}
}

// RejectMessage is sent to the agent with a refused permission.
const RejectMessage = "Refused by the remote policy of oh: this action needs a human and nobody can approve it on the runner. Continue without it, or stop and say what you need."

func (r *Responder) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}

// Answers returns the decisions answered so far.
func (r *Responder) Answers() []remote.SummaryDecision {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]remote.SummaryDecision{}, r.answers...)
}

// Stopped returns why the responder stopped the session (nil: it did not).
func (r *Responder) Stopped() *Stop {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stop
}

func (r *Responder) record(d remote.SummaryDecision) {
	r.mu.Lock()
	r.answers = append(r.answers, d)
	r.mu.Unlock()
}

// Run answers the open decisions of sessionID until ctx ends or a decision
// stops the session; onStop is called once, then Run returns.
func (r *Responder) Run(ctx context.Context, sessionID string, onStop func(Stop)) {
	poll := r.Poll
	if poll == 0 {
		poll = 2 * time.Second
	}
	t := time.NewTicker(poll)
	defer t.Stop()
	for {
		if st := r.Step(ctx, sessionID); st != nil {
			onStop(*st)
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Step answers the open decisions once; it returns a Stop when the session
// must end.
func (r *Responder) Step(ctx context.Context, sessionID string) *Stop {
	list, err := r.Decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: sessionID})
	if err != nil {
		return nil
	}
	for _, d := range list {
		if st := r.answer(ctx, d); st != nil {
			r.mu.Lock()
			if r.stop == nil {
				r.stop = st
			}
			r.mu.Unlock()
			return st
		}
	}
	return nil
}

func (r *Responder) answer(ctx context.Context, d domain.Decision) *Stop {
	sd := remote.SummaryDecision{ID: d.ID, Kind: string(d.Kind), Label: d.Payload.Title, At: r.now()}
	switch d.Kind {
	case domain.DecisionCheckpoint:
		id, _ := d.Payload.Data["checkpoint"].(string)
		sd.ID = id
		pol, ok := r.Policies[strings.ToLower(id)]
		if ok && pol.Policy == "auto" {
			if err := r.Decide(ctx, sessionsvc.Reply{DecisionID: d.ID, Decision: "approve",
				Message: "Approved by the remote policy of the workflow (remote: auto)."}); err != nil && !isResolved(err) {
				sd.Answer = "pending"
				return &Stop{Outcome: remote.OutcomeFailed, Decision: sd, Reason: fmt.Sprintf("checkpoint %s: %v", id, err)}
			}
			sd.Answer = "approved"
			r.record(sd)
			return nil
		}
		// defer, or a checkpoint unknown to the manifest: never approved here.
		sd.Answer = "deferred"
		r.record(sd)
		return &Stop{Outcome: remote.OutcomeDeferred, Decision: sd, Reason: "checkpoint " + id + " is deferred to the machine (remote: defer)"}
	case domain.DecisionPermission:
		sd.Label, sd.Resources = d.Payload.Action, d.Payload.Resources
		if err := r.Decide(ctx, sessionsvc.Reply{DecisionID: d.ID, Decision: "reject", Message: RejectMessage}); err != nil && !isResolved(err) {
			sd.Answer = "pending"
			return &Stop{Outcome: remote.OutcomeFailed, Decision: sd, Reason: fmt.Sprintf("permission %s: %v", d.Payload.Action, err)}
		}
		sd.Answer = "rejected"
		r.record(sd)
		return nil
	case domain.DecisionQuestion:
		sd.Answer = "pending"
		r.record(sd)
		return &Stop{Outcome: remote.OutcomeQuestion, Decision: sd, Reason: "the agent asked a question: " + d.Payload.Title}
	default: // error, budget, circuit breaker
		sd.Answer = "stopped"
		if sd.Label == "" {
			sd.Label = d.Payload.Message
		}
		r.record(sd)
		return &Stop{Outcome: remote.OutcomeFailed, Decision: sd, Reason: string(d.Kind) + ": " + d.Payload.Message}
	}
}

func isResolved(err error) bool {
	var re *sessionsvc.ResolvedError
	return errors.As(err, &re)
}
