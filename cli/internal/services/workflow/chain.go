package workflow

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	wf "github.com/datichb/openhub/cli/internal/workflow"
	"github.com/datichb/openhub/cli/internal/worktree"
)

// « Enchaîner avec… » (O7, P1-T27): after a session, the workflows whose
// inputs take one of its typed outputs, with those inputs prefilled.

// ChainSuggestion is a workflow to chain after a session.
type ChainSuggestion struct {
	WorkflowID string            `json:"workflow_id"`
	Label      string            `json:"label"`
	Prefill    map[string]string `json:"prefill,omitempty"`
	// Tickets fill the ticket input (several: one session per ticket when
	// the workflow allows it).
	Tickets []string `json:"tickets,omitempty"`
	// Matched lists the output types used (« branch », « beads-ids »).
	Matched []wf.OutputType `json:"matched"`
	// Declared: the suggestion is a `next:` step of the workflow that ended
	// (R5, E4), listed first.
	Declared bool `json:"declared,omitempty"`
}

// Chain suggests the workflows to chain after a session of fromID. outputs
// are the values declared by the session, by output id; fallback gives
// values known otherwise (e.g. the branch of the session worktree), by type,
// used when the session declared nothing of that type.
func (s *Service) Chain(ctx context.Context, c Context, fromID string, outputs map[string]any, fallback map[wf.OutputType]any) ([]ChainSuggestion, error) {
	byType := map[wf.OutputType]any{}
	var declared []ChainSuggestion
	if fromID != "" {
		if from, err := s.Resolve(ctx, c, fromID, ResolveOpts{}); from != nil {
			for _, o := range from.Spec.Outputs {
				if v, ok := outputs[o.ID]; ok && v != nil {
					byType[o.Type] = v
				}
			}
			declared = s.nextSteps(ctx, c, from.Spec, outputs)
		} else if err != nil && !isUnknown(err) {
			return nil, err
		}
	}
	rest, err := s.chainByType(ctx, c, fromID, byType, fallback, declared)
	if err != nil {
		return nil, err
	}
	return append(declared, rest...), nil
}

// NextSteps returns the `next:` steps of workflow fromID whose conditions
// hold for the outputs of a session, with their inputs (R5, E4): what oh
// proposes at the end of the session, never launched by itself.
func (s *Service) NextSteps(ctx context.Context, c Context, fromID string, outputs map[string]any) ([]ChainSuggestion, error) {
	from, err := s.Resolve(ctx, c, fromID, ResolveOpts{})
	if from == nil {
		if isUnknown(err) {
			return nil, nil
		}
		return nil, err
	}
	return s.nextSteps(ctx, c, from.Spec, outputs), nil
}

// nextSteps evaluates the `next:` of sp: a step is kept when every output
// of its `when` was recorded; its inputs take the outputs named by
// session.output(<id>).
func (s *Service) nextSteps(ctx context.Context, c Context, sp *wf.Spec, outputs map[string]any) []ChainSuggestion {
	var out []ChainSuggestion
	for _, n := range sp.Next {
		if n.When != nil && !allGiven(n.When.Outputs, outputs) {
			continue
		}
		target, _ := s.Resolve(ctx, c, n.Workflow, ResolveOpts{})
		if target == nil {
			continue // unknown or unreadable: validation reports it
		}
		sg := ChainSuggestion{WorkflowID: target.Spec.ID, Label: firstText(n.Label, target.Spec.Label, s.lang()), Prefill: map[string]string{}, Declared: true}
		keys := make([]string, 0, len(n.Inputs))
		for k := range n.Inputs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			f, ok := wf.ParseSessionFrom(n.Inputs[k])
			if !ok || f.Source != wf.SourceSessionOutput {
				continue
			}
			v, ok := outputs[f.Output]
			if !ok || v == nil || fmt.Sprint(v) == "" {
				continue
			}
			o, _ := sp.Output(f.Output)
			sg.Matched = append(sg.Matched, o.Type)
			in, _ := target.Spec.Inputs.Get(k)
			if ticket, _ := TicketInput(target.Spec); ticket == k && (in.Type == wf.InputBeadsID || in.Type == wf.InputBeadsIDs) {
				sg.Tickets = ticketList(v)
				continue
			}
			sg.Prefill[k] = valueText(v)
		}
		out = append(out, sg)
	}
	return out
}

func allGiven(ids []string, outputs map[string]any) bool {
	for _, id := range ids {
		v, ok := outputs[id]
		if !ok || v == nil || valueText(v) == "" {
			return false
		}
	}
	return true
}

// valueText is an output value as an input value (lists comma-joined).
func valueText(v any) string {
	switch x := v.(type) {
	case []string:
		return strings.Join(x, ",")
	case []any:
		return strings.Join(ticketList(x), ",")
	}
	return fmt.Sprint(v)
}

func firstText(a, b wf.LocalizedText, lang string) string {
	if t := a.Text(lang); t != "" {
		return t
	}
	return b.Text(lang)
}

// chainByType suggests the workflows whose inputs take an output type of
// the session (O7), other than the declared steps.
func (s *Service) chainByType(ctx context.Context, c Context, fromID string, byType, fallback map[wf.OutputType]any, declared []ChainSuggestion) ([]ChainSuggestion, error) {
	for t, v := range fallback {
		if _, ok := byType[t]; !ok && v != nil && fmt.Sprint(v) != "" {
			byType[t] = v
		}
	}
	if len(byType) == 0 {
		return nil, nil
	}
	cat, err := s.load(ctx, c)
	if err != nil {
		return nil, err
	}
	var out []ChainSuggestion
	skip := map[string]bool{fromID: true}
	for _, d := range declared {
		skip[d.WorkflowID] = true
	}
	for _, sum := range s.catalogFrom(cat) {
		if !sum.Valid || skip[sum.ID] {
			continue
		}
		r, _ := wf.ResolveSpec(cat.docs, mustRef(sum.Ref), nil)
		if r == nil {
			continue
		}
		sg := ChainSuggestion{WorkflowID: sum.ID, Label: sum.Label, Prefill: map[string]string{}}
		used := map[string]bool{}
		for _, k := range r.Spec.Inputs.Keys() {
			in, _ := r.Spec.Inputs.Get(k)
			for _, t := range outputTypesFor(in.Type) {
				v, ok := byType[t]
				if !ok || used[string(t)] {
					continue
				}
				used[string(t)] = true
				sg.Matched = append(sg.Matched, t)
				if in.Type == wf.InputBeadsID || in.Type == wf.InputBeadsIDs {
					sg.Tickets = ticketList(v)
				} else {
					sg.Prefill[k] = fmt.Sprint(v)
				}
				break
			}
		}
		if len(sg.Matched) > 0 {
			out = append(out, sg)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return len(out[i].Matched) > len(out[j].Matched) })
	return out, nil
}

// outputTypesFor lists the output types an input type accepts.
func outputTypesFor(t wf.InputType) []wf.OutputType {
	switch t {
	case wf.InputBranch:
		return []wf.OutputType{wf.OutputBranch}
	case wf.InputBeadsID, wf.InputBeadsIDs:
		return []wf.OutputType{wf.OutputBeadsIDs}
	case wf.InputPath:
		return []wf.OutputType{wf.OutputPath}
	}
	return nil
}

// catalogFrom summarizes the most specific documents of a loaded catalogue.
func (s *Service) catalogFrom(cat *catalog) []Summary {
	top := map[string]wf.Ref{}
	for _, ref := range cat.docs.Refs() {
		top[ref.ID] = ref
	}
	out := make([]Summary, 0, len(top))
	for _, ref := range top {
		out = append(out, s.summarize(cat, ref))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func mustRef(s string) wf.Ref {
	r, _ := wf.ParseRef(s)
	return r
}

func isUnknown(err error) bool { return errors.Is(err, ErrUnknownWorkflow) }

// ticketList accepts a list or a comma-separated string of tickets.
func ticketList(v any) []string {
	var raw []string
	switch x := v.(type) {
	case []string:
		raw = x
	case []any:
		for _, e := range x {
			raw = append(raw, fmt.Sprint(e))
		}
	default:
		raw = strings.Split(fmt.Sprint(v), ",")
	}
	var out []string
	for _, e := range raw {
		if e = strings.TrimSpace(e); e != "" {
			out = append(out, e)
		}
	}
	return out
}

// SessionFallback returns the outputs known from a session directory: its
// branch, unless it is the main branch (a session that declared nothing).
func SessionFallback(location string) map[wf.OutputType]any {
	out := map[wf.OutputType]any{}
	if location == "" {
		return out
	}
	if b, err := worktree.CurrentBranch(location); err == nil && b != "" && b != "main" && b != "master" {
		out[wf.OutputBranch] = b
	}
	return out
}
