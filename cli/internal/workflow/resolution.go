package workflow

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Origin says which document set a resolved value.
type Origin struct {
	Layer   Layer  `json:"layer"`
	ID      string `json:"id"`
	Version int    `json:"version,omitempty"`
	// Source is the file of the document (empty for in-memory documents).
	Source string `json:"source,omitempty"`
	Pos    Pos    `json:"pos,omitzero"`
	Draft  bool   `json:"draft,omitempty"`
}

// Ref returns the reference of the document that set the value.
func (o Origin) Ref() Ref { return Ref{Layer: o.Layer, ID: o.ID} }

func (o Origin) String() string {
	s := o.Ref().String()
	if o.Version > 0 {
		s += "@v" + strconv.Itoa(o.Version)
	}
	if o.Draft {
		s += " (draft)"
	}
	return s
}

// Origins maps a field path of the resolved Spec to the document that set it.
type Origins map[string]Origin

// Of returns the origin of path, or of its closest recorded parent.
func (o Origins) Of(path string) (Origin, bool) {
	for p := path; p != ""; p = parentPath(p) {
		if v, ok := o[p]; ok {
			return v, true
		}
	}
	return Origin{}, false
}

func (o Origins) deleteUnder(prefix string) {
	for p := range o {
		if strings.HasPrefix(p, prefix+".") || strings.HasPrefix(p, prefix+"[") {
			delete(o, p)
		}
	}
}

func originOf(doc *Document) Origin {
	return Origin{
		Layer:   doc.Source.Layer,
		ID:      doc.Spec.ID,
		Version: doc.Spec.Version,
		Source:  doc.Source.Path,
		Draft:   doc.Source.Draft,
	}
}

// SessionOptions are the launch choices (CLI flags, launch form). They form
// the last, never persisted layer.
type SessionOptions struct {
	Mode    string
	Runtime Runtime
	// Inputs holds the launch inputs; values may be strings as typed on the
	// command line ("true", "3", "a,b").
	Inputs map[string]any
}

// Resolved is a workflow after `extends` resolution and session options.
type Resolved struct {
	Ref Ref
	// Spec is the merged definition (Extends cleared).
	Spec *Spec
	// Chain lists the documents applied, root definition first.
	Chain []Ref
	// Origins gives the document that set each written path of Spec.
	Origins Origins
	// Mode and Runtime are the effective session choices.
	Mode    string
	Runtime Runtime
	// Inputs are the session inputs given (not defaults).
	Inputs map[string]any
}

// Locate fills the source and position of d from the origin of its path.
func (r *Resolved) Locate(d *Diagnostic) {
	if strings.HasPrefix(d.Path, "session.") {
		d.Source = string(LayerSession)
		return
	}
	o, ok := r.Origins.Of(d.Path)
	if !ok {
		o, ok = r.Origins.Of("id")
	}
	if !ok {
		d.Source = r.Ref.String()
		return
	}
	d.Source = o.Source
	if d.Source == "" {
		d.Source = o.Ref().String()
	}
	d.Pos = o.Pos
}

// ResolveSpec resolves ref against cat: it follows `extends` up to the root
// definition, applies each patch from the root down (security may only be
// hardened), records the origin of every value, then applies opts (nil for
// catalogue views: default mode and runtime, no input checks).
//
// The result is nil when the chain itself cannot be built.
func ResolveSpec(cat Catalog, ref Ref, opts *SessionOptions) (*Resolved, Diagnostics) {
	chain, diags := buildChain(cat, ref)
	if chain == nil {
		return nil, diags
	}

	root := chain[len(chain)-1]
	spec := root.Spec.Clone()
	spec.Extends = ""
	var removed []string
	for _, k := range spec.Agents.Keys() {
		if a, _ := spec.Agents.Get(k); a.Role == RoleDisabled {
			spec.Agents.Delete(k)
			removed = append(removed, "agents."+k)
		}
	}
	for _, k := range spec.Checkpoints.Keys() {
		if c, _ := spec.Checkpoints.Get(k); c.Disabled {
			spec.Checkpoints.Delete(k)
			removed = append(removed, "checkpoints."+k)
		}
	}
	origins := Origins{}
	recordOrigins(origins, root, originOf(root), removed)

	refs := []Ref{root.Ref()}
	for i := len(chain) - 2; i >= 0; i-- {
		doc := chain[i]
		p := &patcher{dst: spec.Clone(), doc: doc, parent: chain[i+1].Ref().String()}
		p.apply()
		spec = p.dst
		diags = append(diags, p.diags...)
		recordOrigins(origins, doc, originOf(doc), p.removed)
		refs = append(refs, doc.Ref())
	}

	r := &Resolved{Ref: ref, Spec: spec, Chain: refs, Origins: origins,
		Mode: spec.DefaultMode(), Runtime: spec.DefaultRuntime()}
	if opts != nil {
		diags = append(diags, r.applySession(opts)...)
	}
	return r, diags
}

func buildChain(cat Catalog, ref Ref) ([]*Document, Diagnostics) {
	var chain []*Document
	at := func(doc *Document, d Diagnostic) Diagnostics {
		d.Source = doc.Source.String()
		d.Pos = doc.Pos(d.Path)
		return Diagnostics{d}
	}
	seen := map[Ref]bool{}
	cur := ref
	for {
		doc, ok := cat.Lookup(cur)
		if !ok {
			if len(chain) == 0 {
				return nil, Diagnostics{errDiag("unknown_workflow", "", cur.String())}
			}
			return nil, at(chain[len(chain)-1], errDiag("extends_not_found", "extends", cur.String()))
		}
		seen[cur] = true
		chain = append(chain, doc)

		// A document must extend the nearest less specific document with the
		// same id, so that no layer can drop the rules of the one below it.
		nearest, hasNearest := nearestBelow(cat, cur)
		if doc.Spec.Extends == "" {
			if hasNearest {
				return nil, at(doc, errDiag("shadow_without_extends", "id", cur.String(), nearest.String()))
			}
			return chain, nil
		}
		parent, err := ParseRef(doc.Spec.Extends)
		if err != nil {
			return nil, at(doc, errDiag("invalid_extends", "extends", doc.Spec.Extends))
		}
		if parent.Layer.Rank() > cur.Layer.Rank() {
			return nil, at(doc, errDiag("extends_more_specific", "extends", parent.String(), string(cur.Layer)))
		}
		if hasNearest && parent != nearest {
			return nil, at(doc, errDiag("must_extend_nearest", "extends", cur.String(), nearest.String()))
		}
		if seen[parent] {
			return nil, at(doc, errDiag("extends_cycle", "extends", parent.String()))
		}
		cur = parent
	}
}

// nearestBelow returns the closest less specific document with ref's id.
func nearestBelow(cat Catalog, ref Ref) (Ref, bool) {
	for i := len(DocumentLayers) - 1; i >= 0; i-- {
		l := DocumentLayers[i]
		if l.Rank() >= ref.Layer.Rank() {
			continue
		}
		cand := Ref{Layer: l, ID: ref.ID}
		if _, ok := cat.Lookup(cand); ok {
			return cand, true
		}
	}
	return Ref{}, false
}

func (r *Resolved) applySession(opts *SessionOptions) Diagnostics {
	var diags Diagnostics
	add := func(d Diagnostic) {
		d.Source = string(LayerSession)
		diags = append(diags, d)
	}
	s := r.Spec
	if opts.Mode != "" {
		if !containsStr(s.AllowedModes(), opts.Mode) {
			add(errDiag("session_mode_not_allowed", "session.mode", opts.Mode, strings.Join(s.AllowedModes(), ", ")))
		} else {
			r.Mode = opts.Mode
		}
	}
	if opts.Runtime != "" {
		if !s.AllowsRuntime(opts.Runtime) {
			add(errDiag("session_runtime_not_allowed", "session.runtime", string(opts.Runtime), joinRuntimes(s.AllowedRuntimes())))
		} else {
			r.Runtime = opts.Runtime
		}
	}
	r.Inputs = map[string]any{}
	for _, k := range sortedKeys(opts.Inputs) {
		v := opts.Inputs[k]
		in, ok := s.Inputs.Get(k)
		if !ok {
			add(errDiag("session_input_unknown", "session.inputs."+k, k))
			continue
		}
		if want, ok := checkInputValue(in, v); !ok {
			add(errDiag("session_input_invalid", "session.inputs."+k, fmt.Sprint(v), k, want))
			continue
		}
		r.Inputs[k] = v
	}
	for _, k := range s.Inputs.Keys() {
		in, _ := s.Inputs.Get(k)
		if _, given := r.Inputs[k]; !given && in.Required && in.Default == nil {
			add(errDiag("session_input_missing", "session.inputs."+k, k))
		}
	}
	return diags
}

// checkInputValue reports whether v suits the input type. Strings as typed
// on a command line are accepted for every type ("true", "12", "a,b").
func checkInputValue(in Input, v any) (string, bool) {
	want := string(in.Type)
	switch in.Type {
	case InputBool:
		switch x := v.(type) {
		case bool:
			return want, true
		case string:
			_, err := strconv.ParseBool(x)
			return want, err == nil
		}
		return want, false
	case InputInt:
		switch x := v.(type) {
		case int, int64:
			return want, true
		case string:
			_, err := strconv.Atoi(x)
			return want, err == nil
		}
		return want, false
	case InputEnum:
		s, ok := v.(string)
		return strings.Join(in.Values, "|"), ok && containsStr(in.Values, s)
	case InputBeadsIDs:
		return want, isStringOrList(v)
	case InputBeadsID:
		if in.Picker != nil && in.Picker.Multi {
			return want, isStringOrList(v)
		}
		_, ok := v.(string)
		return want, ok
	}
	_, ok := v.(string)
	return want, ok
}

func isStringOrList(v any) bool {
	switch x := v.(type) {
	case string:
		return true
	case []string:
		return true
	case []any:
		for _, e := range x {
			if _, ok := e.(string); !ok {
				return false
			}
		}
		return true
	}
	return false
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
