package workflow

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	wf "github.com/datichb/openhub/cli/internal/workflow"
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
}

// Chain suggests the workflows to chain after a session of fromID. outputs
// are the values declared by the session, by output id; fallback gives
// values known otherwise (e.g. the branch of the session worktree), by type,
// used when the session declared nothing of that type.
func (s *Service) Chain(ctx context.Context, c Context, fromID string, outputs map[string]any, fallback map[wf.OutputType]any) ([]ChainSuggestion, error) {
	byType := map[wf.OutputType]any{}
	if fromID != "" {
		if from, err := s.Resolve(ctx, c, fromID, ResolveOpts{}); from != nil {
			for _, o := range from.Spec.Outputs {
				if v, ok := outputs[o.ID]; ok && v != nil {
					byType[o.Type] = v
				}
			}
		} else if err != nil && !isUnknown(err) {
			return nil, err
		}
	}
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
	for _, sum := range s.catalogFrom(cat) {
		if !sum.Valid || sum.ID == fromID {
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
