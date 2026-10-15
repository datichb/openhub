package workflow

import (
	"context"
	"fmt"
	"net/url"
	"sort"
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
		if wf.IsSessionSource(in.From) {
			if err := s.computeFromSession(ctx, c, k, in, values); err != nil {
				return err
			}
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

// FromEqualsSuffix encodes the `from_equals` of an input after the
// argument of its session source: "?k=v&k2=v2" (sorted, "" without).
func FromEqualsSuffix(eq map[string]string) string {
	if len(eq) == 0 {
		return ""
	}
	keys := make([]string, 0, len(eq))
	for k := range eq {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	q := url.Values{}
	for _, k := range keys {
		q.Set(k, eq[k])
	}
	return "?" + q.Encode()
}

// ParseFromArg splits the argument of a session source into the argument
// and its `from_equals`.
func ParseFromArg(arg string) (base string, equals map[string]string) {
	b, query, ok := strings.Cut(arg, "?")
	if !ok {
		return arg, nil
	}
	q, err := url.ParseQuery(query)
	if err != nil {
		return b, map[string]string{"": "\x00"} // matches no session
	}
	equals = map[string]string{}
	for k := range q {
		equals[k] = q.Get(k)
	}
	return b, equals
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

// computeFromSession fills an input read from a previous session (R5):
// session.output(<workflow>.<id>) and session.trace([<workflow>]). The
// source gets the argument "<workflow>.<id>" or "<workflow>"; nothing found
// leaves the input empty (the required-input check applies).
func (s *Service) computeFromSession(ctx context.Context, c Context, k string, in wf.Input, values map[string]any) error {
	f, ok := wf.ParseSessionFrom(in.From)
	if !ok {
		return nil // reported by validation
	}
	fn := s.InputSources[f.Source]
	if fn == nil {
		return nil
	}
	arg := f.Workflow
	if f.Output != "" {
		arg += "." + f.Output
	}
	arg += FromEqualsSuffix(in.FromEquals)
	v, err := fn(ctx, c, arg)
	if err != nil {
		if in.Required && in.Default == nil {
			return fmt.Errorf("input %s (%s): %w", k, in.From, err)
		}
		return nil
	}
	if v != "" {
		values[k] = v
	}
	return nil
}
