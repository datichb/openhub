package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"

	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// ResolveOpts are the choices of a resolution.
type ResolveOpts struct {
	// Layer forces the document layer ("" = the most specific one).
	Layer wf.Layer
	// Session holds the launch choices (mode, runtime, inputs); nil for
	// catalogue views.
	Session *wf.SessionOptions
}

// Resolution is a resolved and validated workflow.
type Resolution struct {
	*wf.Resolved
	Diagnostics wf.Diagnostics
	env         wf.Env
}

// ErrUnknownWorkflow is returned for an id absent from every layer.
var ErrUnknownWorkflow = errors.New("unknown workflow")

// InvalidError is returned when the resolved workflow has errors.
type InvalidError struct {
	Ref         string
	Diagnostics wf.Diagnostics
}

func (e *InvalidError) Error() string {
	return fmt.Sprintf("workflow %s is invalid:\n%s", e.Ref, e.Diagnostics.Err())
}

// Resolve resolves id ("<id>" or "<layer>:<id>") through `extends` and the
// session options, then validates it against the brick catalogue. The
// resolution is returned with an *InvalidError when it has errors, so that
// callers can show every finding.
func (s *Service) Resolve(ctx context.Context, c Context, id string, opts ResolveOpts) (*Resolution, error) {
	cat, err := s.load(ctx, c)
	if err != nil {
		return nil, err
	}
	ref, ok := s.lookup(cat, id, opts.Layer)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownWorkflow, id)
	}
	r, diags := wf.Check(cat.docs, ref, opts.Session, cat.env)
	if r == nil {
		return nil, &InvalidError{Ref: ref.String(), Diagnostics: diags}
	}
	diags.Sort()
	res := &Resolution{Resolved: r, Diagnostics: diags, env: cat.env}
	if diags.HasErrors() {
		return res, &InvalidError{Ref: ref.String(), Diagnostics: diags}
	}
	return res, nil
}

// Has reports whether id exists in a layer (aliases check their target).
func (s *Service) Has(ctx context.Context, c Context, id string) bool {
	cat, err := s.load(ctx, c)
	if err != nil {
		return false
	}
	_, ok := s.lookup(cat, id, "")
	return ok
}

func (s *Service) lookup(cat *catalog, id string, layer wf.Layer) (wf.Ref, bool) {
	if ref, err := wf.ParseRef(id); err == nil {
		_, ok := cat.docs.Lookup(ref)
		return ref, ok
	}
	if layer != "" {
		ref := wf.Ref{Layer: layer, ID: id}
		_, ok := cat.docs.Lookup(ref)
		return ref, ok
	}
	for i := len(wf.DocumentLayers) - 1; i >= 0; i-- {
		ref := wf.Ref{Layer: wf.DocumentLayers[i], ID: id}
		if _, ok := cat.docs.Lookup(ref); ok {
			return ref, true
		}
	}
	return wf.Ref{}, false
}

// MCPSelection returns the oh MCP servers the workflow selects (`mcp:`)
// and whether a document writes the field: absent, the session keeps the
// MCP servers of the project; present (even empty), only those listed.
func (r *Resolution) MCPSelection() (ids []string, set bool) {
	for p := range r.Origins {
		if p == "mcp" || strings.HasPrefix(p, "mcp[") {
			set = true
			break
		}
	}
	return r.Spec.MCP, set
}
