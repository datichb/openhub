package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// ValidateInput selects what Validate checks: a file, a catalogue
// reference, or every workflow.
type ValidateInput struct {
	// File is a workflow file validated as a document of Layer.
	File  string
	Layer wf.Layer
	// Target is "<id>" or "<layer>:<id>" of the catalogue (bare: hub).
	Target string
	// All validates every workflow of the catalogue.
	All bool
}

// ValidateReport is the result of Validate (`oh workflow validate`).
type ValidateReport struct {
	Workflows   []ValidateItem `json:"workflows"`
	Diagnostics wf.Diagnostics `json:"diagnostics"`
}

// ValidateItem is one validated workflow.
type ValidateItem struct {
	Ref   string `json:"ref"`
	Valid bool   `json:"valid"`
}

// Counts returns the number of errors and warnings.
func (r *ValidateReport) Counts() (errs, warns int) {
	for _, d := range r.Diagnostics {
		if d.Severity == wf.SeverityError {
			errs++
		} else {
			warns++
		}
	}
	return errs, warns
}

// Validate checks a file, a catalogue workflow or every workflow against
// the catalogue of c: hub and team-state layers, and the brick catalogue
// merged with the team bricks (as `oh run` and the publication do).
func (s *Service) Validate(ctx context.Context, c Context, in ValidateInput) (*ValidateReport, error) {
	if in.Layer == "" {
		in.Layer = wf.LayerHub
	}
	if !in.Layer.IsDocumentLayer() {
		return nil, fmt.Errorf("%s", i18n.Tf("cmd.workflow.validate.bad_layer", string(in.Layer)))
	}
	cat, err := s.load(ctx, c)
	if err != nil {
		return nil, err
	}
	report := &ValidateReport{Diagnostics: wf.Diagnostics{}}
	var refs []wf.Ref
	switch {
	case in.All:
		report.Diagnostics = append(report.Diagnostics, cat.diags...)
		refs = cat.docs.Refs()
		// Files that could not be loaded are listed as invalid.
		seen := map[string]bool{}
		for _, d := range cat.diags {
			name := strings.TrimSuffix(filepath.Base(d.Source), filepath.Ext(d.Source))
			if d.Severity == wf.SeverityError && name != "" && !seen[name] {
				seen[name] = true
				report.Workflows = append(report.Workflows, ValidateItem{Ref: wf.Ref{Layer: cat.sourceLayer(d.Source), ID: name}.String()})
			}
		}
	case in.File != "":
		doc, diags := wf.ParseFile(in.File, in.Layer)
		report.Diagnostics = append(report.Diagnostics, diags...)
		if doc == nil || diags.HasErrors() {
			name := strings.TrimSuffix(filepath.Base(in.File), filepath.Ext(in.File))
			report.Workflows = append(report.Workflows, ValidateItem{Ref: string(in.Layer) + ":" + name})
			return report, nil
		}
		cat.docs.Put(doc)
		refs = []wf.Ref{doc.Ref()}
	default:
		ref, err := wf.ParseRef(in.Target)
		if err != nil {
			ref = wf.Ref{Layer: wf.LayerHub, ID: in.Target}
		}
		// Load errors of the requested file explain an unknown workflow.
		for _, d := range cat.diags {
			if base := filepath.Base(d.Source); base == ref.ID+".yaml" || base == ref.ID+".yml" {
				report.Diagnostics = append(report.Diagnostics, d)
			}
		}
		refs = []wf.Ref{ref}
	}

	for _, ref := range refs {
		_, diags := wf.Check(cat.docs, ref, nil, cat.env)
		report.Diagnostics = append(report.Diagnostics, diags...)
		report.Workflows = append(report.Workflows, ValidateItem{Ref: ref.String(), Valid: !diags.HasErrors()})
	}
	if !in.All && report.Diagnostics.HasErrors() {
		report.Workflows[0].Valid = false
	}
	report.Diagnostics.Sort()
	return report, nil
}

// IsWorkflowFile reports whether target names a workflow file rather than
// a catalogue reference.
func IsWorkflowFile(target string) bool {
	if ext := filepath.Ext(target); ext == ".yaml" || ext == ".yml" {
		return true
	}
	st, err := os.Stat(target)
	return err == nil && !st.IsDir()
}
