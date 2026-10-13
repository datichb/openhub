package wiki

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// Service keeps the proposals of a project (wiki_proposals) and applies
// them. The author of a proposal is the agent oh saw calling
// wiki_propose; the user decides on the card of the commit checkpoint
// (accept, edit, refuse), and oh applies the accepted proposals of the
// session when the checkpoint is passed (before the commit: they are
// committed with the code); the documentarian applies proposals with
// wiki_apply (doc-sync). Proposals of a workflow without a commit stay
// queued.
type Service struct {
	Store domain.WikiProposalStore
	// SessionsDir is ~/.oh/sessions: a proposal may not cite it.
	SessionsDir string
	Now         func() time.Time
}

// Limits of the queue.
const (
	MaxPerSession = 50
	MaxPerProject = 500
)

// AppliedByOh is the author of an application made by oh (proposals the
// user accepted on the card).
const AppliedByOh = "oh"

// Decision choices on a proposal.
const (
	ChoiceAccept = "accept"
	ChoiceEdit   = "edit"
	ChoiceRefuse = "refuse"
	ChoicePend   = "pending" // back to the queue
)

// Errors of the service.
var (
	ErrNoStore = errors.New("wiki: no proposal store")
	ErrBadID   = errors.New("a wiki proposal id reads wp-<number>")
	ErrClosed  = errors.New("the proposal is already applied or refused")
	ErrChoice  = errors.New("the choice is accept, edit, refuse or pending")
)

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Filed is a proposal to record, with what oh knows of its caller.
type Filed struct {
	ProjectID   string
	SessionID   string // the oh session (root of the delegations)
	ToolSession string
	Agent       string // seen by oh
	Proposal
}

// Propose checks, cleans and queues a proposal; the same open proposal of
// the session is returned instead of a copy.
func (s *Service) Propose(ctx context.Context, f Filed) (domain.WikiProposal, error) {
	if s.Store == nil {
		return domain.WikiProposal{}, ErrNoStore
	}
	clean, err := CleanProposal(f.Proposal, s.SessionsDir)
	if err != nil {
		return domain.WikiProposal{}, err
	}
	open, err := s.Store.ListWikiProposals(ctx, domain.WikiProposalFilter{ProjectID: f.ProjectID, SessionID: f.SessionID,
		Statuses: []string{domain.WikiPending, domain.WikiAccepted}})
	if err != nil {
		return domain.WikiProposal{}, err
	}
	for _, p := range open {
		if p.Page == clean.Page && strings.EqualFold(p.Section, clean.Section) && p.Content == clean.Content {
			return p, nil
		}
	}
	p := domain.WikiProposal{ProjectID: f.ProjectID, SessionID: f.SessionID, ToolSession: f.ToolSession, Agent: f.Agent,
		Page: clean.Page, Section: clean.Section, Content: clean.Content, Tag: clean.Tag, Source: clean.Source,
		Status: domain.WikiPending, CreatedAt: s.now()}
	if err := s.Store.AddWikiProposal(ctx, &p, MaxPerSession, MaxPerProject); err != nil {
		return domain.WikiProposal{}, err
	}
	return p, nil
}

// Open returns the proposals still to apply (pending and accepted) of a
// project, or of one of its sessions.
func (s *Service) Open(ctx context.Context, projectID, sessionID string) ([]domain.WikiProposal, error) {
	if s.Store == nil {
		return nil, ErrNoStore
	}
	return s.Store.ListWikiProposals(ctx, domain.WikiProposalFilter{ProjectID: projectID, SessionID: sessionID,
		Statuses: []string{domain.WikiPending, domain.WikiAccepted}})
}

// List returns every proposal of a project (or of one of its sessions).
func (s *Service) List(ctx context.Context, projectID, sessionID string) ([]domain.WikiProposal, error) {
	if s.Store == nil {
		return nil, ErrNoStore
	}
	return s.Store.ListWikiProposals(ctx, domain.WikiProposalFilter{ProjectID: projectID, SessionID: sessionID})
}

// Get returns a proposal of a project.
func (s *Service) Get(ctx context.Context, projectID, id string) (*domain.WikiProposal, error) {
	if s.Store == nil {
		return nil, ErrNoStore
	}
	seq, ok := domain.ParseWikiProposalID(id)
	if !ok {
		return nil, ErrBadID
	}
	return s.Store.GetWikiProposal(ctx, projectID, seq)
}

// Decide records the decision of the user on an open proposal: accept (oh
// applies it at the commit checkpoint), edit (accepted with the content
// given, cleaned like a proposal), refuse, or back to pending.
func (s *Service) Decide(ctx context.Context, projectID, id, choice, content, by string) (*domain.WikiProposal, error) {
	p, err := s.Get(ctx, projectID, id)
	if err != nil {
		return nil, err
	}
	if !p.Open() {
		return p, ErrClosed
	}
	now := s.now()
	switch choice {
	case ChoiceAccept:
		p.Status = domain.WikiAccepted
	case ChoiceEdit:
		clean, err := CleanProposal(Proposal{Page: p.Page, Section: p.Section, Content: content, Tag: p.Tag, Source: p.Source}, s.SessionsDir)
		if err != nil {
			return p, err
		}
		if clean.Content != p.Content {
			if p.Original == "" {
				p.Original = p.Content
			}
			p.Content = clean.Content
		}
		p.Status = domain.WikiAccepted
	case ChoiceRefuse:
		p.Status = domain.WikiRefused
	case ChoicePend:
		p.Status = domain.WikiPending
	default:
		return p, ErrChoice
	}
	p.DecidedBy, p.DecidedAt = by, &now
	return p, s.Store.UpdateWikiProposal(ctx, p)
}

// Result is the outcome of the application of a proposal.
type Result struct {
	ID   string `json:"id"`
	Page string `json:"page,omitempty"`
	Err  string `json:"error,omitempty"`
}

// ApplyIDs applies proposals of a project to the wiki at root (wiki_apply
// of the documentarian): pending or accepted ones, by the agent by.
func (s *Service) ApplyIDs(ctx context.Context, root, projectID string, ids []string, by string) []Result {
	var out []Result
	seen := map[string]bool{}
	for _, id := range ids {
		p, err := s.Get(ctx, projectID, id)
		if err != nil {
			out = append(out, Result{ID: strings.TrimSpace(id), Err: err.Error()})
			continue
		}
		if seen[p.ID()] {
			continue
		}
		seen[p.ID()] = true
		if !p.Open() {
			out = append(out, Result{ID: p.ID(), Page: p.Page, Err: ErrClosed.Error()})
			continue
		}
		out = append(out, s.apply(ctx, root, p, by))
	}
	return out
}

// ApplyAccepted applies the accepted proposals of a session to the wiki at
// root (the commit checkpoint is passed: oh applies what the user
// validated on the card).
func (s *Service) ApplyAccepted(ctx context.Context, root, projectID, sessionID string) ([]Result, error) {
	if s.Store == nil {
		return nil, ErrNoStore
	}
	list, err := s.Store.ListWikiProposals(ctx, domain.WikiProposalFilter{ProjectID: projectID, SessionID: sessionID, Statuses: []string{domain.WikiAccepted}})
	if err != nil {
		return nil, err
	}
	out := make([]Result, 0, len(list))
	for i := range list {
		out = append(out, s.apply(ctx, root, &list[i], AppliedByOh))
	}
	return out, nil
}

func (s *Service) apply(ctx context.Context, root string, p *domain.WikiProposal, by string) Result {
	now := s.now()
	err := Apply(root, Entry{Page: p.Page, Section: p.Section, Content: p.Content, Tag: p.Tag, Source: p.Source, Agent: p.Agent, Date: now})
	res := Result{ID: p.ID(), Page: p.Page}
	if err != nil && !errors.Is(err, ErrSymlink) && fileWritten(root, p) {
		// The entry is in the page; only the god nodes refresh failed.
		err = nil
	}
	if err != nil {
		p.Error = err.Error()
		res.Err = p.Error
	} else {
		p.Status, p.AppliedBy, p.AppliedAt, p.Error = domain.WikiApplied, by, &now, ""
	}
	if uerr := s.Store.UpdateWikiProposal(ctx, p); uerr != nil && res.Err == "" {
		res.Err = fmt.Sprintf("applied, not recorded: %v", uerr)
	}
	return res
}

// fileWritten reports whether the page holds the content of p.
func fileWritten(root string, p *domain.WikiProposal) bool {
	text, err := ReadPage(root, p.Page, "")
	if err != nil {
		return false
	}
	first, _, _ := strings.Cut(p.Content, "\n")
	return strings.Contains(text, strings.TrimPrefix(first, "- "))
}

// Summary returns the pages an application changed and the proposals that
// failed, with their error.
func Summary(results []Result) (applied, failed []string) {
	for _, r := range results {
		if r.Err != "" {
			failed = append(failed, r.ID+" ("+r.Err+")")
			continue
		}
		if !slices.Contains(applied, r.Page) {
			applied = append(applied, r.Page)
		}
	}
	return applied, failed
}

// CommitPassed applies the accepted proposals of a session when its commit
// checkpoint is passed (checkpoint.Service.CommitPassed): the line for the
// agent and the proposals applied.
func (s *Service) CommitPassed(ctx context.Context, sess *domain.Session, checkpoint string) (message string, applied []string) {
	if s.Store == nil || sess == nil || sess.LaunchPath == "" {
		return "", nil
	}
	results, err := s.ApplyAccepted(ctx, sess.LaunchPath, sess.ProjectID, sess.ID)
	if err != nil {
		slog.Warn("wiki: accepted proposals not applied", "session", sess.ID, "checkpoint", checkpoint, "error", err)
		return "", nil
	}
	if len(results) == 0 {
		return "", nil
	}
	var ids []string
	for _, r := range results {
		if r.Err == "" {
			ids = append(ids, r.ID)
		}
	}
	pages, failed := Summary(results)
	slog.Info("wiki: accepted proposals applied", "session", sess.ID, "checkpoint", checkpoint, "applied", ids, "failed", failed)
	var lines []string
	if len(ids) > 0 {
		files := make([]string, len(pages))
		for i, p := range pages {
			files[i] = Dir + "/" + p
		}
		lines = append(lines, i18n.Tf("wiki.commit.applied", strings.Join(ids, ", "), strings.Join(files, ", ")))
	}
	if len(failed) > 0 {
		lines = append(lines, i18n.Tf("wiki.commit.failed", strings.Join(failed, " ; ")))
	}
	return strings.Join(lines, "\n"), ids
}
