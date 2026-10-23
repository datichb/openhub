package checkpoint

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/remote"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/termtext"
	"github.com/datichb/openhub/cli/internal/textfmt"
)

// Verifications in a remote session (R3-remote): the job runs them where
// the work is, as the account of the agents (Account), and keeps the last
// result of each checkpoint in summary.json; the machine imports them at
// fetch time, cleaned and bounded again, for the card and the timeline.
// They never count as a verification of the work on the machine: a
// checkpoint asked again locally is verified again.

// Bounds of an imported verification.
const (
	remoteVerifyMax   = 50  // checkpoints
	remoteCommandsMax = 20  // commands per verification
	remoteFieldMax    = 500 // reason, command
)

// VerifyOf is the verification of a checkpoint.
type VerifyOf struct {
	ID     string
	Result domain.CheckpointVerify
}

// Verifications returns the verifications of a session, by checkpoint id.
func (s *Service) Verifications(ctx context.Context, sessionID string) ([]VerifyOf, error) {
	st, err := s.state(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	out := make([]VerifyOf, 0, len(st.Verify))
	for id, r := range st.Verify {
		out = append(out, VerifyOf{ID: id, Result: r})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// ImportRemoteVerify records the verifications run in the remote job of a
// session (fetch): kept apart from the local ones (RemoteVerify), traced in
// the timeline (author "remote"). Unknown checkpoints and statuses are
// dropped; it returns how many were recorded.
func (s *Service) ImportRemoteVerify(ctx context.Context, sessionID string, list []VerifyOf) (int, error) {
	if len(list) == 0 || s.States == nil {
		return 0, nil
	}
	known := func(string) bool { return true }
	if _, wf, err := s.session(ctx, sessionID); err == nil {
		known = func(id string) bool { _, ok := wf.Checkpoint(id); return ok }
	}
	n := 0
	_, err := s.update(ctx, sessionID, func(st *domain.CheckpointState) error {
		for i, v := range list {
			if i >= remoteVerifyMax {
				break
			}
			id := strings.TrimSpace(v.ID)
			r, ok := cleanRemoteVerify(v.Result)
			if id == "" || !ok || !known(id) {
				continue
			}
			if cur, ok := st.RemoteVerify[id]; ok && cur.Started.Equal(r.Started) && cur.Status == r.Status {
				continue // fetched again
			}
			if st.RemoteVerify == nil {
				st.RemoteVerify = map[string]domain.CheckpointVerify{}
			}
			st.RemoteVerify[id] = r
			kind := domain.CheckpointVerified
			if !r.OK() {
				kind = domain.CheckpointVerifyFail
			}
			at := r.Started
			if r.Finished != nil {
				at = *r.Finished
			}
			st.Timeline = append(st.Timeline, domain.CheckpointEvent{At: at, Kind: kind, ID: id, By: RemoteAuthor, Message: VerifySummary(r)})
			n++
		}
		if n == 0 {
			return errUnchanged
		}
		sort.SliceStable(st.Timeline, func(i, j int) bool { return st.Timeline[i].At.Before(st.Timeline[j].At) })
		if k := len(st.Timeline); k > timelineMax {
			st.Timeline = st.Timeline[k-timelineMax:]
		}
		return nil
	})
	if errors.Is(err, errUnchanged) {
		err = nil
	}
	return n, err
}

// RemoteAuthor is the author of the timeline entries imported from a
// remote job.
const RemoteAuthor = "remote"

// cleanRemoteVerify neutralizes and bounds a verification read from the
// artifacts of a job (untrusted text).
func cleanRemoteVerify(r domain.CheckpointVerify) (domain.CheckpointVerify, bool) {
	switch r.Status {
	case domain.VerifyPassed, domain.VerifyFailed, domain.VerifyNone, domain.VerifyUnavailable, domain.VerifyRunning:
	default:
		return domain.CheckpointVerify{}, false
	}
	out := domain.CheckpointVerify{Status: r.Status, Started: r.Started, Finished: r.Finished,
		Reason: textfmt.ClipBytes(termtext.Clean(r.Reason), remoteFieldMax)}
	if r.Expect == sessionspec.VerifyPass || r.Expect == sessionspec.VerifyFail {
		out.Expect = r.Expect
	}
	if r.Excerpt != "" {
		lines := strings.Split(r.Excerpt, "\n")
		for i, l := range lines {
			lines[i] = termtext.Clean(l)
		}
		out.Excerpt = excerpt([]byte(strings.Join(lines, "\n")))
	}
	for i, c := range r.Commands {
		if i >= remoteCommandsMax {
			break
		}
		out.Commands = append(out.Commands, domain.VerifyCommandResult{Kind: textfmt.ClipBytes(termtext.Clean(c.Kind), 32),
			Cmd: textfmt.ClipBytes(termtext.Clean(c.Cmd), remoteFieldMax), Exit: c.Exit, TimedOut: c.TimedOut, Millis: c.Millis})
	}
	return out, true
}

// SummaryVerifications converts verifications for summary.json.
func SummaryVerifications(list []VerifyOf) []remote.SummaryVerification {
	out := make([]remote.SummaryVerification, 0, len(list))
	for _, v := range list {
		sv := remote.SummaryVerification{Checkpoint: v.ID, Status: v.Result.Status, Expect: v.Result.Expect, Reason: v.Result.Reason,
			Excerpt: v.Result.Excerpt, Started: v.Result.Started, Finished: v.Result.Finished}
		for _, c := range v.Result.Commands {
			sv.Commands = append(sv.Commands, remote.SummaryVerifyCommand{Kind: c.Kind, Cmd: c.Cmd, Exit: c.Exit, TimedOut: c.TimedOut, Millis: c.Millis})
		}
		out = append(out, sv)
	}
	return out
}

// FromSummary converts the verifications of a summary.json (not cleaned:
// ImportRemoteVerify does it).
func FromSummary(list []remote.SummaryVerification) []VerifyOf {
	out := make([]VerifyOf, 0, len(list))
	for _, sv := range list {
		r := domain.CheckpointVerify{Status: sv.Status, Expect: sv.Expect, Reason: sv.Reason, Excerpt: sv.Excerpt, Started: sv.Started, Finished: sv.Finished}
		for _, c := range sv.Commands {
			r.Commands = append(r.Commands, domain.VerifyCommandResult{Kind: c.Kind, Cmd: c.Cmd, Exit: c.Exit, TimedOut: c.TimedOut, Millis: c.Millis})
		}
		out = append(out, VerifyOf{ID: sv.Checkpoint, Result: r})
	}
	return out
}
