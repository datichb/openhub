package teamstate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/datichb/openhub/cli/internal/workflow"
)

// Drafts: drafts/<member>/<id>.yaml in each scope, pushed with the
// team-state (visible to other members) but only ever loaded for their
// author. A draft may carry its own copy of the prompt template next to it
// (drafts/<member>/<id>.prompt.md.tmpl, OwnPromptSuffix): it replaces the
// template the document references, so that editing a draft never touches
// the published prompts. History versions use the same convention.

// OwnPromptSuffix names the prompt copy kept next to a draft or a history
// version (<base>.prompt.md.tmpl).
const OwnPromptSuffix = ".prompt.md.tmpl"

// DraftPromptRel is drafts/<member>/<id>.prompt.md.tmpl.
func DraftPromptRel(scope WorkflowScope, member, id string) (string, error) {
	rel, err := DraftRel(scope, member, id)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(rel, ".yaml") + OwnPromptSuffix, nil
}

// ownPromptOf returns the prompt copy next to a document of the drafts or
// history folders ("" for published documents or when absent).
func ownPromptOf(file string) string {
	if !strings.HasSuffix(file, ".yaml") {
		return ""
	}
	dir := filepath.Dir(file)
	if filepath.Base(filepath.Dir(dir)) != DraftsDirName && filepath.Base(filepath.Dir(dir)) != HistoryDirName {
		return ""
	}
	p := strings.TrimSuffix(file, ".yaml") + OwnPromptSuffix
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		return p
	}
	return ""
}

// DraftInfo is a draft found on disk.
type DraftInfo struct {
	Scope  WorkflowScope
	Member string
	ID     string
	// Path is the absolute path of the YAML file.
	Path string
	// Prompt is the absolute path of the draft's own prompt template ("").
	Prompt string
}

// Ref is the reference of the draft's document.
func (d DraftInfo) Ref() workflow.Ref { return workflow.Ref{Layer: d.Scope.Layer(), ID: d.ID} }

// ListDrafts returns the drafts of member in scopes.
func (r *Repo) ListDrafts(member string, scopes ...WorkflowScope) ([]DraftInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.listDrafts(member, scopes...)
}

func (r *Repo) listDrafts(member string, scopes ...WorkflowScope) ([]DraftInfo, error) {
	if _, err := SafeName(member); err != nil {
		return nil, err
	}
	var out []DraftInfo
	for _, s := range scopes {
		wdir, err := r.WorkflowsDir(s)
		if err != nil {
			return nil, err
		}
		dir := filepath.Join(wdir, DraftsDirName, member)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".yaml") {
				continue
			}
			id := strings.TrimSuffix(name, ".yaml")
			if ValidWorkflowID(id) != nil {
				continue
			}
			d := DraftInfo{Scope: s, Member: member, ID: id, Path: filepath.Join(dir, name)}
			d.Prompt = ownPromptOf(d.Path)
			out = append(out, d)
		}
	}
	return out, nil
}

// LoadDrafts puts the drafts of member in scopes into cat, replacing the
// published documents of the same reference (Source.Draft is set). It
// returns the references loaded.
func (r *Repo) LoadDrafts(cat *workflow.MemCatalog, member string, scopes ...WorkflowScope) ([]workflow.Ref, workflow.Diagnostics) {
	drafts, err := r.ListDrafts(member, scopes...)
	if err != nil {
		return nil, workflow.Diagnostics{integrityDiag(DiagWorkflowReadFailed, r.path, err)}
	}
	var refs []workflow.Ref
	var diags workflow.Diagnostics
	for _, d := range drafts {
		doc, ds := workflow.ParseFile(d.Path, d.Scope.Layer())
		diags = append(diags, ds...)
		if doc == nil || ds.HasErrors() {
			continue
		}
		doc.Source.Draft = true
		if doc.Spec.ID != d.ID {
			diags = append(diags, workflow.Diagnostic{Severity: workflow.SeverityError, Code: "id_filename_mismatch", Path: "id",
				Source: d.Path, Pos: doc.Pos("id"), Message: fmt.Sprintf("id %q ≠ %s", doc.Spec.ID, filepath.Base(d.Path))})
			continue
		}
		cat.Put(doc)
		refs = append(refs, doc.Ref())
	}
	return refs, diags
}

// WriteDraftLocal writes the draft of member (YAML and, when prompt is not
// nil, its own prompt template; an empty prompt removes it). It returns the
// files to commit. Call it inside WithWriteLock (or on a solo repo).
func (r *Repo) WriteDraftLocal(scope WorkflowScope, member, id string, data, prompt []byte) ([]string, error) {
	rel, err := DraftRel(scope, member, id)
	if err != nil {
		return nil, err
	}
	if err := writeRel(r.path, rel, data); err != nil {
		return nil, err
	}
	files := []string{rel}
	if prompt != nil {
		prel, _ := DraftPromptRel(scope, member, id)
		if len(prompt) == 0 {
			if err := os.Remove(filepath.Join(r.path, prel)); err != nil && !os.IsNotExist(err) {
				return nil, err
			}
		} else if err := writeRel(r.path, prel, prompt); err != nil {
			return nil, err
		}
		files = append(files, prel)
	}
	return files, nil
}

// DeleteDraftLocal removes the draft of member (and its prompt copy). It
// returns the paths removed, relative to the repo root.
func (r *Repo) DeleteDraftLocal(scope WorkflowScope, member, id string) ([]string, error) {
	rel, err := DraftRel(scope, member, id)
	if err != nil {
		return nil, err
	}
	prel, _ := DraftPromptRel(scope, member, id)
	var removed []string
	for _, p := range []string{rel, prel} {
		if err := os.Remove(filepath.Join(r.path, p)); err == nil {
			removed = append(removed, p)
		} else if !os.IsNotExist(err) {
			return removed, err
		}
	}
	return removed, nil
}

// writeRel writes data at root/rel, creating the parent folders.
func writeRel(root, rel string, data []byte) error {
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, data, 0o644)
}
