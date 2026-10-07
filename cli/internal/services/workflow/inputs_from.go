package workflow

import (
	"context"
	"fmt"
	"strings"

	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// InputSource computes the value of an input declared with `from:` from the
// value of its argument input (QB2: GitLab discussions of a merge request).
type InputSource func(ctx context.Context, c Context, arg string) (string, error)

// ComputeInputs fills, in values, the inputs declared with `from:` that are
// not given: a value given at launch wins, then the computed one, then the
// default. A source that fails is an error for an input without default
// (the workflow cannot run without it), otherwise the default applies.
func (s *Service) ComputeInputs(ctx context.Context, c Context, sp *wf.Spec, values map[string]any) error {
	for _, k := range sp.Inputs.Keys() {
		in, _ := sp.Inputs.Get(k)
		if in.From == "" || given(values[k]) {
			continue
		}
		src, argName, ok := wf.ParseFrom(in.From)
		if !ok {
			continue // reported by validation
		}
		arg, _ := values[argName].(string)
		if strings.TrimSpace(arg) == "" {
			continue // nothing to compute from: required-input checks apply
		}
		fn := s.InputSources[src]
		if fn == nil {
			if in.Default == nil {
				return fmt.Errorf("input %s: no %s source on this machine", k, src)
			}
			continue
		}
		v, err := fn(ctx, c, arg)
		if err != nil {
			if in.Default == nil {
				return fmt.Errorf("input %s (%s): %w", k, src, err)
			}
			continue
		}
		if v != "" {
			values[k] = v
		}
	}
	return nil
}

func given(v any) bool {
	if v == nil {
		return false
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s) != ""
	}
	return true
}
