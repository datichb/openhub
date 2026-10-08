package session

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionresults"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/worktree"
)

// Results (P3-T11): diff, branch, cost and tokens of a session, read live
// from the tool when its server runs (and saved), else from the last saved
// snapshot, else from the oh session row (usage only).

// Results is the outcome of a session.
type Results struct {
	sessionresults.Summary
	Patch string
	Live  bool // read from the running tool (else: snapshot or usage only)
	// Current: the changes were read now in the session directory.
	Current bool
	Session *domain.Session
}

// Results returns the results of a session: the tool (live) or the last
// snapshot give the tool view; the changes are those of the whole session in
// its directory (commits and changes since it started, subagents' work
// included) and the usage is the oh session total (v5 corrections, A23).
func (s *Service) Results(ctx context.Context, sessionID string) (*Results, error) {
	if s.Sessions == nil {
		return nil, errors.New("session: no session store")
	}
	sess, err := s.Sessions.Get(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("session %s: %w", sessionID, err)
	}
	r, err := s.toolResults(ctx, sess)
	if err != nil {
		return nil, err
	}
	withSessionUsage(r, sess)
	r.Current = withSessionDiff(r, sess)
	if r.Current || r.Live {
		if s.SessionsDir != "" {
			_ = sessionresults.SaveSummary(s.SessionsDir, r.Summary, r.Patch)
		}
	}
	return r, nil
}

// toolResults reads the results from the running tool, else the last
// snapshot, else the session row.
func (s *Service) toolResults(ctx context.Context, sess *domain.Session) (*Results, error) {
	if s.Adapter != nil {
		if h, err := s.handle(ctx, sess.ID); err == nil {
			if res, err := s.Adapter.Results(ctx, h, sess.ID); err == nil {
				return &Results{Summary: sessionresults.FromResult(res, s.now()), Patch: sessionresults.Patch(res.Changes), Live: true, Session: sess}, nil
			}
		}
	}
	if sum, patch, err := sessionresults.Load(s.SessionsDir, sess.ID); err == nil {
		return &Results{Summary: sum, Patch: patch, Session: sess}, nil
	} else if !errors.Is(err, sessionresults.ErrNoSnapshot) {
		return nil, err
	}
	sum := sessionresults.Summary{SessionID: sess.ID, Agent: sess.EntryAgent, CapturedAt: s.now()}
	if sess.Title != nil {
		sum.Title = *sess.Title
	}
	return &Results{Summary: sum, Session: sess}, nil
}

// withSessionUsage counts the subagent sessions: the session row holds the
// usage total of the oh session (what the budget counts).
func withSessionUsage(r *Results, sess *domain.Session) {
	r.Cost = math.Max(r.Cost, sess.Cost)
	r.TokensIn, r.TokensOut = max(r.TokensIn, sess.TokensIn), max(r.TokensOut, sess.TokensOut)
	r.TokensReasoning, r.TokensCacheRead = max(r.TokensReasoning, sess.TokensReasoning), max(r.TokensCacheRead, sess.TokensCacheRead)
}

// withSessionDiff replaces the changes by those of the whole session, read
// with git in its directory from its starting point (else from the start of
// its branch). It reports whether git could tell.
func withSessionDiff(r *Results, sess *domain.Session) bool {
	dir := sess.LaunchPath
	if dir == "" || sess.Runtime == string(sessionspec.RuntimeRemote) {
		return false
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return false
	}
	// The session branch may no longer be checked out in its directory
	// (base directory reused): its commits are read on the branch then.
	cur, _ := worktree.CurrentBranch(dir)
	other := r.Branch != "" && r.Branch != cur && worktree.BranchExists(dir, r.Branch)
	rev := ""
	if other {
		rev = r.Branch
	}
	ref := sess.StartRef
	if ref == "" && !sess.StartedAt.IsZero() {
		ref = gitutil.RefBefore(dir, rev, sess.StartedAt) // session recorded before v42
	}
	if ref == "" {
		ref = gitutil.BaseRef(dir)
	}
	if ref == "" {
		return false
	}
	read := func(ref string) (*gitutil.WorkDiff, error) {
		if other {
			return gitutil.DiffRange(dir, ref, r.Branch)
		}
		return gitutil.DiffSince(dir, ref)
	}
	d, err := read(ref)
	if errors.Is(err, gitutil.ErrUnknownRef) && sess.StartRef != "" {
		if base := gitutil.BaseRef(dir); base != "" {
			d, err = read(base)
		}
	}
	if err != nil {
		return false
	}
	r.Files, r.Additions, r.Deletions, r.Patch = nil, 0, 0, d.Patch
	for _, f := range d.Files {
		r.Files = append(r.Files, sessionresults.FileStat{File: f.File, Status: f.Status, Additions: f.Additions, Deletions: f.Deletions})
		r.Additions += f.Additions
		r.Deletions += f.Deletions
	}
	if r.Branch == "" && cur != "" && !strings.HasPrefix(cur, "(detached)") {
		r.Branch = cur
	}
	return true
}

// Recap is a one-line summary of the results (toast, CLI, recap at the end
// of a session).
func Recap(r *Results) string {
	return i18n.Tf("cmd.session.recap", len(r.Files), r.Additions, r.Deletions, r.TokensIn+r.TokensOut, r.Cost)
}

// MRDescription renders a merge request description (Markdown) from the
// results of a session.
func MRDescription(r *Results) string {
	var b strings.Builder
	title := r.Title
	if title == "" && r.Session != nil && r.Session.Title != nil {
		title = *r.Session.Title
	}
	if title != "" {
		b.WriteString("## " + title + "\n\n")
	}
	b.WriteString(Recap(r) + "\n")
	if len(r.Files) > 0 {
		b.WriteString("\n### " + i18n.T("cmd.session.mr.files") + "\n\n")
		for _, f := range r.Files {
			fmt.Fprintf(&b, "- `%s` (+%d −%d)\n", f.File, f.Additions, f.Deletions)
		}
	}
	agent := r.Agent
	if agent == "" && r.Session != nil {
		agent = r.Session.EntryAgent
	}
	b.WriteString("\n---\n")
	b.WriteString(i18n.Tf("cmd.session.mr.footer", r.SessionID, agent, orDash(r.Branch)) + "\n")
	return b.String()
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
