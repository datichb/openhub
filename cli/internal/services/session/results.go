package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionresults"
)

// Results (P3-T11): diff, branch, cost and tokens of a session, read live
// from the tool when its server runs (and saved), else from the last saved
// snapshot, else from the oh session row (usage only).

// Results is the outcome of a session.
type Results struct {
	sessionresults.Summary
	Patch   string
	Live    bool // read from the running tool (else: snapshot or usage only)
	Session *domain.Session
}

// Results returns the results of a session.
func (s *Service) Results(ctx context.Context, sessionID string) (*Results, error) {
	if s.Sessions == nil {
		return nil, errors.New("session: no session store")
	}
	sess, err := s.Sessions.Get(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("session %s: %w", sessionID, err)
	}
	if s.Adapter != nil {
		if h, err := s.handle(ctx, sessionID); err == nil {
			res, err := s.Adapter.Results(ctx, h, sessionID)
			if err == nil {
				if s.SessionsDir != "" {
					_ = sessionresults.Save(s.SessionsDir, res, s.now())
				}
				return &Results{Summary: sessionresults.FromResult(res, s.now()), Patch: sessionresults.Patch(res.Changes), Live: true, Session: sess}, nil
			}
		}
	}
	if sum, patch, err := sessionresults.Load(s.SessionsDir, sessionID); err == nil {
		return &Results{Summary: sum, Patch: patch, Session: sess}, nil
	} else if !errors.Is(err, sessionresults.ErrNoSnapshot) {
		return nil, err
	}
	sum := sessionresults.Summary{SessionID: sess.ID, Agent: sess.EntryAgent, Cost: sess.Cost,
		TokensIn: sess.TokensIn, TokensOut: sess.TokensOut, TokensReasoning: sess.TokensReasoning, TokensCacheRead: sess.TokensCacheRead}
	if sess.Title != nil {
		sum.Title = *sess.Title
	}
	return &Results{Summary: sum, Session: sess}, nil
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
