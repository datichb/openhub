package cmd

import (
	"context"
	"errors"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/wiki"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Metrics of the project wiki in the TUI (modernisation LD-I).

func newProjectWikiView(a *app.App) *views.ProjectWikiView {
	return views.NewProjectWikiView(views.ProjectWikiViewConfig{
		Project: func() (string, string) {
			p, err := resolveActiveProject(a)
			if err != nil || p == nil {
				return "", ""
			}
			return p.Name, p.Path
		},
		Metrics: func(ctx context.Context, path string) (views.ProjectWikiMetrics, error) {
			projectID := ""
			if a != nil && a.Projects != nil {
				if p, err := a.Projects.GetByPath(ctx, path); err == nil {
					projectID = p.ID
				}
			}
			svc := &wiki.Service{}
			if store != nil {
				svc = newWikiService()
			}
			m, err := svc.Metrics(ctx, path, projectID)
			if errors.Is(err, wiki.ErrNoWiki) {
				return views.ProjectWikiMetrics{NoWiki: true}, nil
			}
			if err != nil {
				return views.ProjectWikiMetrics{}, err
			}
			return projectWikiMetrics(m), nil
		},
		Proposals: func(ctx context.Context, path string) ([]views.WikiProposalView, error) {
			svc, projectID, err := projectWikiQueue(ctx, a, path)
			if err != nil {
				return nil, err
			}
			list, err := svc.List(ctx, projectID, "")
			if err != nil {
				return nil, err
			}
			return queueWiki(ctx, a, list), nil
		},
		Decide: func(ctx context.Context, path, id, choice, content string) error {
			svc, projectID, err := projectWikiQueue(ctx, a, path)
			if err != nil {
				return err
			}
			_, err = svc.Decide(ctx, projectID, id, choice, content, string(domain.ResolvedByOh))
			if err != nil {
				return errors.New(wikiDecideError(err))
			}
			return nil
		},
		Edit: tuiWikiEdit,
	})
}

// projectWikiQueue returns the wiki service and the oh project of a path
// (the queue of `oh wiki pending`).
func projectWikiQueue(ctx context.Context, a *app.App, path string) (*wiki.Service, string, error) {
	if store == nil || a == nil || a.Projects == nil {
		return nil, "", wiki.ErrNoStore
	}
	projectID, err := wikiProject(ctx, path, func(ctx context.Context, path string) (string, error) {
		p, err := a.Projects.GetByPath(ctx, path)
		if err != nil {
			return "", err
		}
		return p.ID, nil
	})
	if err != nil {
		return nil, "", err
	}
	return newWikiService(), projectID, nil
}

// queueWiki renders the queue of the project for the view: the proposals
// still to apply and the stale ones, with the workflow of their session.
func queueWiki(ctx context.Context, a *app.App, list []domain.WikiProposal) []views.WikiProposalView {
	workflows := map[string]string{}
	out := make([]views.WikiProposalView, 0, len(list))
	for _, p := range list {
		if !p.Open() && p.Status != domain.WikiStale {
			continue
		}
		wf, ok := workflows[p.SessionID]
		if !ok && p.SessionID != "" && a != nil && a.Sessions != nil {
			if s, err := a.Sessions.Get(ctx, p.SessionID); err == nil && s != nil {
				wf = s.WorkflowID
			}
			workflows[p.SessionID] = wf
		}
		v := wikiProposalView(p)
		v.Workflow = wf
		out = append(out, v)
	}
	return out
}

// wikiProposalView renders a proposal for the views.
func wikiProposalView(p domain.WikiProposal) views.WikiProposalView {
	return views.WikiProposalView{ID: p.ID(), Agent: p.Agent, Page: p.Page, Section: p.Section, Tag: p.Tag, Source: p.Source,
		Status: p.Status, Content: p.Content, Replaces: p.Replaces, SessionID: p.SessionID, Created: p.CreatedAt,
		Edited: p.Original != "", Error: p.Error, Reason: p.Reason}
}

// tuiWikiEdit edits the content of a proposal in $EDITOR, the TUI
// suspended (called on the event loop).
func tuiWikiEdit(name, content string) (string, error) {
	sh := tuiShell
	if sh == nil {
		return "", errors.New(i18n.T("tui.shell.suspend_failed"))
	}
	var out string
	err := sh.SuspendAndExec(func() (err error) {
		out, err = wikiEditText(name, content)
		return err
	})
	return out, err
}

func projectWikiMetrics(m wiki.Metrics) views.ProjectWikiMetrics {
	return views.ProjectWikiMetrics{
		Pages: m.Pages, Specs: m.ByKind[wiki.KindSpec], Decisions: m.ByKind[wiki.KindDecision], Postmortems: m.ByKind[wiki.KindPostmortem],
		Domains: m.Domains, DomainsCovered: m.DomainsCovered, Uncovered: m.Uncovered,
		MedianAgeDays: m.MedianAgeDays, Dated: m.Dated, Oldest: m.Oldest, OldestDays: m.OldestDays,
		Tagged: m.TaggedEnrichments(), Uncertain: m.Tags[wiki.TagUncertain], Confirmed: m.Tags[wiki.TagConfirmed], Deduced: m.Tags[wiki.TagDeduced],
		Pending: m.Pending, Accepted: m.Accepted, Synced: m.Synced,
	}
}
