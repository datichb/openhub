package cmd

import (
	"context"
	"errors"
	"slices"
	"strconv"

	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// TUI wiring of the publication and history screens (P2-T15): service calls
// mapped to the view models.

// openPublishView pushes the publication screen of the draft ref.
func openPublishView(ref string) {
	if tuiShell == nil {
		return
	}
	c := catalogContext()
	tuiShell.PushView(views.NewPublishView(views.PublishViewConfig{
		Ref:  ref,
		Load: func(ctx context.Context) (*views.PublishPreview, error) { return loadPublishPreview(ctx, c, ref) },
		Publish: func(ctx context.Context, message string) (*views.PublishResult, error) {
			p, err := newWorkflowService(ctx).Publish(ctx, c, ref, message)
			if err != nil {
				return nil, errors.New(tuiWorkflowError(err))
			}
			return publishResult(p), nil
		},
		OnDone: func(*views.PublishResult) { refreshCatalog() },
	}))
}

func publishResult(p *workflowsvc.Publication) *views.PublishResult {
	return &views.PublishResult{Version: p.Version, Queued: p.Queued, Warnings: diagLines(p.Diagnostics)}
}

// loadPublishPreview computes the preview of a publication of the draft ref.
// An invalid draft gives a preview (not valid) with its findings.
func loadPublishPreview(ctx context.Context, c workflowsvc.Context, ref string) (*views.PublishPreview, error) {
	svc := newWorkflowService(ctx)
	p, err := svc.Preview(ctx, c, ref)
	var invalid *workflowsvc.InvalidError
	if err != nil && (p == nil || !errors.As(err, &invalid)) {
		return nil, errors.New(tuiWorkflowError(err))
	}
	out := &views.PublishPreview{Ref: p.Ref.String(), Next: p.NextVersion, Valid: err == nil,
		Findings: diagLines(p.Diagnostics), New: p.Impact.New, NewBricks: p.Impact.NewBricks}
	if p.Published != nil && p.NextVersion > 1 {
		out.Current = p.NextVersion - 1
	}
	for _, it := range p.Impact.Items {
		out.Impact = append(out.Impact, views.PublishImpact{Widen: it.Level == workflowsvc.ImpactWiden, Text: it.Message})
	}
	out.Diff = textDiff(p.Published, p.Draft, out.Ref+" ("+i18n.T("tui.publish.diff_published")+")", out.Ref+" ("+i18n.T("tui.publish.diff_draft")+")")
	if policy, err := svc.Governance(ctx, c); err == nil {
		out.Governance = views.GovernanceLabel(policy)
	}
	if ops, err := svc.QueuedOps(ctx, c); err == nil {
		out.Queued = slices.ContainsFunc(ops, func(op teamstate.QueuedOp) bool {
			scope, _ := teamstate.ParseScope(op.Scope)
			return workflow.Ref{Layer: scope.Layer(), ID: op.ID}.String() == out.Ref
		})
	}
	return out, nil
}

// textDiff is the unified diff of two versions: document, then prompt
// template.
func textDiff(from, to *workflowsvc.Text, fromLabel, toLabel string) string {
	a, b := workflowsvc.TextString(from, false), workflowsvc.TextString(to, false)
	diff := workflowsvc.UnifiedDiff(a, b, fromLabel, toLabel)
	if to != nil && to.Prompt != nil {
		pa, pb := workflowsvc.TextString(from, true), workflowsvc.TextString(to, true)
		diff += workflowsvc.UnifiedDiff(pa, pb, "prompt · "+fromLabel, "prompt · "+toLabel)
	}
	return diff
}

// openHistoryView pushes the history screen of ref.
func openHistoryView(ref string) {
	if tuiShell == nil {
		return
	}
	c := catalogContext()
	tuiShell.PushView(views.NewHistoryView(views.HistoryViewConfig{
		Ref: ref,
		Load: func(ctx context.Context) ([]views.HistoryVersion, error) {
			vs, err := newWorkflowService(ctx).History(ctx, c, ref)
			if err != nil {
				return nil, errors.New(tuiWorkflowError(err))
			}
			out := make([]views.HistoryVersion, len(vs))
			for i, v := range vs {
				out[i] = views.HistoryVersion{Version: v.Version, By: v.Entry.PublishedBy, At: v.Entry.PublishedAt,
					Message: v.Entry.Message, Current: v.Current}
			}
			return out, nil
		},
		Diff: func(ctx context.Context, version int) (string, error) {
			svc := newWorkflowService(ctx)
			old, err := svc.VersionText(ctx, c, ref, version)
			if err != nil {
				return "", errors.New(tuiWorkflowError(err))
			}
			cur, err := svc.DocumentText(ctx, c, ref)
			if err != nil {
				cur = nil // archived: the version alone
			}
			if old.Prompt == nil && cur != nil {
				old.Prompt = cur.Prompt // the current version uses the referenced template
			}
			return textDiff(old, cur, ref+" v"+strconv.Itoa(version), ref+" ("+i18n.T("tui.history.current")+")"), nil
		},
		Restore: func(ctx context.Context, version int) (*views.PublishResult, error) {
			p, err := newWorkflowService(ctx).Restore(ctx, c, ref, version)
			if err != nil {
				return nil, errors.New(tuiWorkflowError(err))
			}
			return publishResult(p), nil
		},
		OnRestored: func(*views.PublishResult) { refreshCatalog() },
	}))
}
