package workflow

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// Summary describes a workflow of the catalogue: its most specific
// document, resolved through `extends`.
type Summary struct {
	ID          string       `json:"id"`
	Ref         string       `json:"ref"` // layer:id of the most specific document
	Layer       wf.Layer     `json:"layer"`
	Version     int          `json:"version,omitempty"`
	Category    wf.Category  `json:"category,omitempty"`
	Label       string       `json:"label"`
	Description string       `json:"description,omitempty"`
	Chain       []string     `json:"chain"` // documents applied, root first
	Source      string       `json:"source,omitempty"`
	ReadOnly    bool         `json:"read_only"` // hub workflows are built in
	Risk        wf.Risk      `json:"risk,omitempty"`
	EntryAgent  string       `json:"entry_agent"`
	Runtimes    []wf.Runtime `json:"runtimes"`
	Modes       []string     `json:"modes"`
	DefaultMode string       `json:"default_mode"`
	// TicketInput is the first beads-id / beads-ids input ("" when the
	// workflow does not take tickets); MultiTickets when it accepts several.
	TicketInput  string `json:"ticket_input,omitempty"`
	MultiTickets bool   `json:"multi_tickets,omitempty"`
	Valid        bool   `json:"valid"`
	Errors       int    `json:"errors"`
	Warnings     int    `json:"warnings"`
	// Diagnostics are the findings of the validation (load errors included).
	Diagnostics wf.Diagnostics `json:"diagnostics,omitempty"`
}

// Catalog lists the workflows of every available layer, one entry per id
// (its most specific document), ordered by category then id. A file that
// cannot be loaded is listed as invalid.
func (s *Service) Catalog(ctx context.Context, c Context) ([]Summary, error) {
	cat, err := s.load(ctx, c)
	if err != nil {
		return nil, err
	}
	top := map[string]wf.Ref{}
	for _, ref := range cat.docs.Refs() { // by layer rank: the last one wins
		top[ref.ID] = ref
	}
	var out []Summary
	for _, ref := range top {
		out = append(out, s.summarize(cat, ref))
	}
	for _, d := range cat.diags {
		if d.Severity != wf.SeverityError || d.Source == "" {
			continue
		}
		id := strings.TrimSuffix(filepath.Base(d.Source), filepath.Ext(d.Source))
		if _, ok := top[id]; ok || id == "" {
			continue
		}
		top[id] = wf.Ref{Layer: wf.LayerHub, ID: id}
		sum := Summary{ID: id, Ref: top[id].String(), Layer: wf.LayerHub, Label: id, Source: d.Source, ReadOnly: true}
		for _, dd := range cat.diags {
			if dd.Source == d.Source {
				sum.Diagnostics = append(sum.Diagnostics, dd)
			}
		}
		sum.count()
		out = append(out, sum)
	}
	sort.Slice(out, func(i, j int) bool {
		ci, cj := categoryRank(out[i].Category), categoryRank(out[j].Category)
		if ci != cj {
			return ci < cj
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// IDs returns the identifiers of the catalogue (preferences, omnibar).
func IDs(list []Summary) []string {
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = s.ID
	}
	return out
}

// Find returns the summary of id.
func Find(list []Summary, id string) (Summary, bool) {
	for _, s := range list {
		if s.ID == id {
			return s, true
		}
	}
	return Summary{}, false
}

func (s *Service) summarize(cat *catalog, ref wf.Ref) Summary {
	doc, _ := cat.docs.Lookup(ref)
	sum := Summary{ID: ref.ID, Ref: ref.String(), Layer: ref.Layer, Label: ref.ID,
		Source: doc.Source.Path, ReadOnly: ref.Layer == wf.LayerHub}
	r, diags := wf.Check(cat.docs, ref, nil, cat.env)
	sum.Diagnostics = diags
	if r != nil {
		sp := r.Spec
		for _, c := range r.Chain {
			sum.Chain = append(sum.Chain, c.String())
		}
		sum.Version, sum.Category, sum.Risk = sp.Version, sp.Category, sp.Risk
		if l := sp.Label.Text(s.lang()); l != "" {
			sum.Label = l
		}
		sum.Description = sp.Description.Text(s.lang())
		sum.EntryAgent = sp.EntryAgent()
		sum.Runtimes, sum.Modes, sum.DefaultMode = sp.AllowedRuntimes(), sp.AllowedModes(), sp.DefaultMode()
		sum.TicketInput, sum.MultiTickets = TicketInput(sp)
	}
	sum.count()
	return sum
}

func (s *Summary) count() {
	s.Errors, s.Warnings = 0, 0
	for _, d := range s.Diagnostics {
		if d.Severity == wf.SeverityError {
			s.Errors++
		} else {
			s.Warnings++
		}
	}
	s.Valid = s.Errors == 0 && s.Chain != nil
}

// TicketInput returns the first input taking Beads tickets and whether it
// accepts several (one session per ticket).
func TicketInput(sp *wf.Spec) (string, bool) {
	for _, k := range sp.Inputs.Keys() {
		in, _ := sp.Inputs.Get(k)
		switch in.Type {
		case wf.InputBeadsID:
			return k, in.Picker != nil && in.Picker.Multi
		case wf.InputBeadsIDs:
			return k, false
		}
	}
	return "", false
}

// CategoryOrder is the display order of the categories.
var CategoryOrder = []wf.Category{wf.CategoryDevelop, wf.CategoryFrame, wf.CategoryQuality, wf.CategoryKnowledge, wf.CategoryOther}

func categoryRank(c wf.Category) int {
	for i, k := range CategoryOrder {
		if k == c {
			return i
		}
	}
	return len(CategoryOrder)
}
