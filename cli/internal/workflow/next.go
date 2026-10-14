package workflow

import (
	"fmt"
	"sort"
)

// Catalogue side of the composition (R5, schema_next.go): the spec of
// another workflow (checks of `next:` and `from: session.output`) and the
// aliases.

// WorkflowSpecs gives the resolved spec of a workflow of the catalogue,
// most specific layer first (an alias stands for its default target).
type WorkflowSpecs interface {
	WorkflowSpec(id string) (*Spec, bool)
}

// Top returns the most specific document reference of id.
func (c *MemCatalog) Top(id string) (Ref, bool) {
	for i := len(DocumentLayers) - 1; i >= 0; i-- {
		ref := Ref{Layer: DocumentLayers[i], ID: id}
		if _, ok := c.docs[ref]; ok {
			return ref, true
		}
	}
	return Ref{}, false
}

// WorkflowSpec implements WorkflowSpecs.
func (c *MemCatalog) WorkflowSpec(id string) (*Spec, bool) {
	ref, ok := c.Top(id)
	if !ok {
		if ref, ok = c.ResolveAlias(id, nil); !ok {
			return nil, false
		}
	}
	r, _ := ResolveSpec(c, ref, nil)
	if r == nil {
		return nil, false
	}
	return r.Spec, true
}

// AliasTarget is a workflow that declares an alias.
type AliasTarget struct {
	Ref  Ref
	With string
}

// AliasTargets returns the workflows declaring alias id (most specific
// document of each workflow), by workflow id.
func (c *MemCatalog) AliasTargets(id string) []AliasTarget {
	ids := map[string]bool{}
	for r := range c.docs {
		ids[r.ID] = true
	}
	var out []AliasTarget
	for wid := range ids {
		ref, _ := c.Top(wid)
		r, _ := ResolveSpec(c, ref, nil)
		if r == nil {
			continue
		}
		for _, a := range r.Spec.Aliases {
			if a.ID == id {
				out = append(out, AliasTarget{Ref: ref, With: a.With})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Ref.ID < out[j].Ref.ID })
	return out
}

// ResolveAlias returns the workflow alias id launches when no document has
// that id: a target whose `with` input is given (given may be nil: none
// is), else the target without `with`.
func (c *MemCatalog) ResolveAlias(id string, given func(input string) bool) (Ref, bool) {
	if _, exists := c.Top(id); exists {
		return Ref{}, false
	}
	var fallback *AliasTarget
	targets := c.AliasTargets(id)
	for i, t := range targets {
		switch {
		case t.With == "" && fallback == nil:
			fallback = &targets[i]
		case t.With != "" && given != nil && given(t.With):
			return t.Ref, true
		}
	}
	if fallback != nil {
		return fallback.Ref, true
	}
	return Ref{}, false
}

// CheckAliases reports the aliases of the catalogue that cannot be
// resolved: an alias declared without `with` by two workflows, an alias
// with only `with` routes (nothing to launch without the input).
func CheckAliases(c *MemCatalog) Diagnostics {
	seen := map[string]bool{}
	var diags Diagnostics
	for _, ref := range c.Refs() {
		r, _ := ResolveSpec(c, ref, nil)
		if r == nil {
			continue
		}
		for _, a := range r.Spec.Aliases {
			if seen[a.ID] {
				continue
			}
			seen[a.ID] = true
			if _, exists := c.Top(a.ID); exists {
				continue // a document wins (validated with the workflow)
			}
			var plain []string
			for _, t := range c.AliasTargets(a.ID) {
				if t.With == "" {
					plain = append(plain, t.Ref.ID)
				}
			}
			switch {
			case len(plain) > 1:
				d := errDiag("alias_ambiguous", "aliases", a.ID, fmt.Sprint(plain))
				d.Source = ref.String()
				diags = append(diags, d)
			case len(plain) == 0:
				d := errDiag("alias_without_default", "aliases", a.ID)
				d.Source = ref.String()
				diags = append(diags, d)
			}
		}
	}
	return diags
}
