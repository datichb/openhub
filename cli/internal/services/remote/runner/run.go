package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/remote"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Session is the session control of the job (RunService and SessionService
// of the job, wired by the command).
type Session interface {
	Start(ctx context.Context, req runsvc.StartRequest) (string, error)
	AwaitTurn(ctx context.Context, sessionID string) (*runsvc.HeadlessResult, error)
	Interrupt(ctx context.Context, sessionID string) error
	Stop(ctx context.Context, sessionID string) error
	Results(ctx context.Context, sessionID string) (adapters.SessionResult, error)
	// Export returns the transcripts of the session and its sub-agent
	// sessions, parents first, with their ids.
	Export(ctx context.Context, sessionID string) ([]json.RawMessage, []string, error)
	Outputs(ctx context.Context, sessionID string) map[string]any
	// Failed reports whether the tool ended the session's last run in
	// failure (LLM error…).
	Failed(ctx context.Context, sessionID string) bool
}

// Repo is the work tree of the job (Git).
type Repo interface {
	Clone(ctx context.Context, cloneURL, ref, commit, branch, dir string) (string, error)
	Finish(ctx context.Context, dir, base, ref, branch, title, description string) (*PushResult, error)
}

// Progress publishes the progress of the session (team-state claims).
type Progress interface {
	Set(ctx context.Context, status, step, mrURL string)
}

// Input is a remote session to run.
type Input struct {
	Job       *Job
	Fetched   *Fetched
	OutDir    string // artifacts (journal.jsonl is written there by the fake bd)
	Git       Repo
	Session   Session
	Responder *Responder
	Progress  Progress // nil: no team-state
	Now       func() time.Time
	// TurnPoll is how often the turn is awaited again after a decision.
	TurnPoll time.Duration
}

// JournalPath is the Beads journal of a job.
func JournalPath(outDir string) string { return filepath.Join(outDir, remote.JournalFile) }

// Run runs a remote session end to end (P5-T09 → T14) and writes the
// summary. The returned error is the job failure (the summary is written
// whenever possible).
func Run(ctx context.Context, in Input) (*remote.Summary, error) {
	now := in.Now
	if now == nil {
		now = time.Now
	}
	m := &in.Fetched.Manifest
	sum := &remote.Summary{Schema: remote.SummarySchema, SessionID: m.SessionID, StartedAt: now().UTC(), Branch: m.Branch}
	progress := func(status, step, mr string) {
		if in.Progress != nil {
			in.Progress.Set(ctx, status, step, mr)
		}
	}
	finish := func(err error) (*remote.Summary, error) {
		sum.EndedAt = now().UTC()
		if err != nil {
			sum.Outcome = remote.OutcomeFailed
			sum.Error = in.Job.Secrets.Redact(err.Error())
		}
		sum.JournalEntries = countLines(JournalPath(in.OutDir))
		if werr := writeJSON(filepath.Join(in.OutDir, remote.SummaryFile), sum); werr != nil && err == nil {
			err = werr
		}
		switch {
		case sum.Outcome == remote.OutcomeFailed:
			progress("failed", sum.Error, sum.MRURL)
		case sum.MRURL != "":
			progress("mr_ready", sum.Outcome, sum.MRURL)
		default:
			progress("ready", sum.Outcome, "")
		}
		return sum, err
	}
	if err := os.MkdirAll(in.OutDir, 0o755); err != nil {
		return finish(err)
	}
	progress("running", "clone", "")

	// T09: clone of the target project.
	base, err := in.Git.Clone(ctx, m.Project.CloneURL, m.Ref, m.Commit, m.Branch, m.WorkDir)
	if err != nil {
		return finish(fmt.Errorf("cloning %s: %w", m.Project.Path, err))
	}
	sum.BaseCommit = base

	// T10: the session, with the same RunService as on the machine.
	b := in.Fetched.Bundle
	entry := m.EntryAgent
	if entry == "" {
		entry = b.Spec.EntryAgent
	}
	sid, err := in.Session.Start(ctx, runsvc.StartRequest{
		SessionID: m.SessionID, ProjectID: m.Project.OhID, Location: m.WorkDir, ProjectDir: m.WorkDir, Bundle: b,
		EntryAgent: entry, Title: m.Title, Prompt: m.Prompt, WorkflowID: m.Workflow.ID, Mode: m.Mode,
		WorkflowLayer: m.Workflow.Layer, WorkflowVersion: m.Workflow.Version, WorkflowRisk: m.Workflow.Risk,
		LocationKind: "base", Headless: true, Provider: m.Provider, AllowedModels: m.AllowedModels, MaxTokens: m.MaxTokens,
		Attach: sessionspec.AttachNone, Runtime: sessionspec.RuntimeRemote, BeadsAllow: m.BeadsAllow,
	})
	if err != nil {
		return finish(fmt.Errorf("starting the session: %w", err))
	}
	progress("running", "session", "")

	// T11: the policy responder answers while the turn is awaited.
	turnCtx, stopTurn := context.WithCancel(ctx)
	defer stopTurn()
	respDone := make(chan struct{})
	go func() {
		defer close(respDone)
		in.Responder.Run(turnCtx, sid, func(Stop) { stopTurn() })
	}()
	poll := in.TurnPoll
	if poll == 0 {
		poll = 3 * time.Second
	}
	var turnErr error
	for {
		res, err := in.Session.AwaitTurn(turnCtx, sid)
		if err == nil {
			sum.Text = res.Text
			break
		}
		if errors.Is(err, runsvc.ErrDecisionPending) {
			select {
			case <-turnCtx.Done():
			case <-time.After(poll):
				continue
			}
		}
		if turnCtx.Err() != nil {
			break
		}
		turnErr = err
		break
	}
	stopTurn()
	<-respDone
	if turnErr == nil && in.Responder.Stopped() == nil && ctx.Err() == nil {
		// Decisions raised as the turn ended (error of the last step).
		in.Responder.Step(ctx, sid)
		if in.Responder.Stopped() == nil && in.Session.Failed(ctx, sid) {
			turnErr = errors.New("the session ended in failure (see session.export)")
		}
	}
	sum.Decisions = in.Responder.Answers()

	switch st := in.Responder.Stopped(); {
	case st != nil:
		sum.Outcome = st.Outcome
		d := st.Decision
		switch st.Outcome {
		case remote.OutcomeDeferred:
			sum.Deferred = &d
		case remote.OutcomeQuestion:
			sum.Question = &d
		default:
			sum.Error = st.Reason
		}
		if err := in.Session.Interrupt(ctx, sid); err != nil {
			slog.Warn("runner: interrupt failed", "err", err)
		}
	case ctx.Err() != nil:
		sum.Outcome, sum.Error = remote.OutcomeFailed, "job canceled or timed out"
		ctx = context.WithoutCancel(ctx)
		_ = in.Session.Interrupt(ctx, sid)
	case turnErr != nil:
		sum.Outcome, sum.Error = remote.OutcomeFailed, in.Job.Secrets.Redact(turnErr.Error())
	default:
		sum.Outcome = remote.OutcomeCompleted
	}

	// T14 (part): results and export before the server stops.
	if r, err := in.Session.Results(ctx, sid); err == nil {
		sum.Cost, sum.TokensIn, sum.TokensOut = r.Cost, r.TokensIn, r.TokensOut
		sum.TokensReasoning, sum.TokensCacheRead, sum.Model = r.TokensReasoning, r.TokensCacheRead, r.Model
	}
	sum.Outputs = in.Session.Outputs(ctx, sid)
	var exportErr error
	if trs, ids, err := in.Session.Export(ctx, sid); err == nil {
		sum.Sessions = ids
		exportErr = writeJSON(filepath.Join(in.OutDir, remote.ExportFile), remote.Export{Schema: remote.ExportSchema, Sessions: trs})
	} else {
		exportErr = err
	}
	if err := in.Session.Stop(ctx, sid); err != nil {
		slog.Warn("runner: stopping the session", "err", err)
	}

	// T13: branch and draft merge request.
	title := "oh: " + m.Title
	pr, err := in.Git.Finish(ctx, m.WorkDir, base, m.Ref, m.Branch, title, mrDescription(m, sum))
	if err != nil {
		if sum.Outcome != remote.OutcomeFailed {
			sum.Outcome = remote.OutcomeFailed
		}
		return finish(fmt.Errorf("pushing %s: %w", m.Branch, err))
	}
	sum.Commit, sum.MRURL = pr.Commit, pr.MRURL
	if sum.Outputs == nil {
		sum.Outputs = map[string]any{}
	}
	if pr.Commit != "" {
		sum.Outputs["branch"] = m.Branch
	}
	if pr.MRURL != "" {
		sum.Outputs["merge_request"] = pr.MRURL
	}
	if exportErr != nil {
		return finish(fmt.Errorf("exporting the session: %w", exportErr))
	}
	if sum.Outcome == remote.OutcomeFailed && sum.Error != "" {
		return finish(errors.New(sum.Error))
	}
	return finish(nil)
}

func mrDescription(m *remote.Manifest, s *remote.Summary) string {
	parts := []string{fmt.Sprintf("Remote oh session %s (workflow %s, mode %s): %s.", m.SessionID, m.Workflow.ID, m.Mode, s.Outcome)}
	if len(m.Tickets) > 0 {
		parts = append(parts, "Tickets: "+strings.Join(m.Tickets, ", ")+".")
	}
	if s.Deferred != nil {
		parts = append(parts, "Checkpoint "+s.Deferred.ID+" waits for you: fetch the session (oh session fetch "+m.SessionID+").")
	}
	if s.Question != nil {
		parts = append(parts, "A question waits for you: fetch the session (oh session fetch "+m.SessionID+").")
	}
	return strings.Join(parts, " ")
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func countLines(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return strings.Count(string(data), "\n")
}
