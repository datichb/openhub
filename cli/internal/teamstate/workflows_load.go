package teamstate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Integrity diagnostic codes of the published workflows (messages
// "teamstate.workflow.<code>"). All are warnings: the file is skipped.
const (
	DiagWorkflowUnlocked       = "workflow_unlocked"
	DiagWorkflowHashMismatch   = "workflow_hash_mismatch"
	DiagWorkflowPromptMismatch = "workflow_prompt_mismatch"
	DiagWorkflowLockOrphan     = "workflow_lock_orphan"
	DiagWorkflowLockInvalid    = "workflow_lock_invalid"
	DiagWorkflowReadFailed     = "workflow_read_failed"
)

// IntegrityCodes are the diagnostic codes of the integrity check.
var IntegrityCodes = []string{
	DiagWorkflowUnlocked, DiagWorkflowHashMismatch, DiagWorkflowPromptMismatch,
	DiagWorkflowLockOrphan, DiagWorkflowLockInvalid, DiagWorkflowReadFailed,
}

// IsIntegrityDiag reports whether d comes from the integrity check.
func IsIntegrityDiag(d workflow.Diagnostic) bool {
	for _, c := range IntegrityCodes {
		if d.Code == c {
			return true
		}
	}
	return false
}

func integrityDiag(code, source string, args ...any) workflow.Diagnostic {
	return workflow.Diagnostic{
		Severity: workflow.SeverityWarning,
		Code:     code,
		Source:   source,
		Message:  i18n.Tf("teamstate.workflow."+code, args...),
		Hint:     i18n.T("teamstate.workflow.integrity_hint"),
	}
}

// LoadWorkflowLayers adds to cat the published workflows of the team scope
// and, when project is not empty, of that project scope.
//
// Only files recorded in workflows.lock whose YAML (and prompt template)
// still match the recorded hashes are loaded; any other file is skipped
// with a warning (hand-edited, unpublished, lock entry without file).
// Parse errors are reported as by workflow.MemCatalog.LoadDir.
func (r *Repo) LoadWorkflowLayers(cat *workflow.MemCatalog, project string) workflow.Diagnostics {
	scopes := []WorkflowScope{TeamScope()}
	if project != "" {
		scopes = append(scopes, ProjectScope(project))
	}
	return r.LoadWorkflowScopes(cat, scopes...)
}

// LoadWorkflowScopes is LoadWorkflowLayers for explicit scopes.
func (r *Repo) LoadWorkflowScopes(cat *workflow.MemCatalog, scopes ...WorkflowScope) workflow.Diagnostics {
	r.mu.RLock()
	defer r.mu.RUnlock()
	lock, err := r.readWorkflowLock()
	if err != nil {
		return workflow.Diagnostics{integrityDiag(DiagWorkflowLockInvalid, filepath.Join(r.path, WorkflowsLockFile), err)}
	}
	var diags workflow.Diagnostics
	for _, s := range scopes {
		diags = append(diags, r.loadPublished(cat, s, lock)...)
	}
	return diags
}

func (r *Repo) loadPublished(cat *workflow.MemCatalog, scope WorkflowScope, lock *WorkflowLock) workflow.Diagnostics {
	wdir, err := r.WorkflowsDir(scope)
	if err != nil {
		return workflow.Diagnostics{integrityDiag(DiagWorkflowReadFailed, scope.String(), err)}
	}
	dir := filepath.Join(wdir, PublishedDirName)
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return workflow.Diagnostics{integrityDiag(DiagWorkflowReadFailed, dir, err)}
	}
	var diags workflow.Diagnostics
	seen := map[string]bool{}
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if e.IsDir() || (ext != ".yaml" && ext != ".yml") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ext)
		file := filepath.Join(dir, e.Name())
		seen[id] = true
		entry, ok := lock.Get(scope, id)
		if !ok || ext != ".yaml" || ValidWorkflowID(id) != nil {
			diags = append(diags, integrityDiag(DiagWorkflowUnlocked, file, scope.String()+":"+id))
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			diags = append(diags, integrityDiag(DiagWorkflowReadFailed, file, err))
			continue
		}
		if HashContent(data) != entry.Hash {
			diags = append(diags, integrityDiag(DiagWorkflowHashMismatch, file, scope.String()+":"+id, entry.Version))
			continue
		}
		if d, ok := r.checkPrompt(scope, id, data, entry, file); !ok {
			diags = append(diags, d)
			continue
		}
		doc, ds := workflow.Parse(data, workflow.Source{Layer: scope.Layer(), Path: file})
		diags = append(diags, ds...)
		if doc == nil || ds.HasErrors() {
			continue
		}
		if doc.Spec.ID != id {
			// Same rule as MemCatalog.LoadDir (id_filename_mismatch).
			d := workflow.Diagnostic{Severity: workflow.SeverityError, Code: "id_filename_mismatch", Path: "id", Source: file, Pos: doc.Pos("id"),
				Message: i18n.Tf("workflow.diag.id_filename_mismatch", doc.Spec.ID, e.Name())}
			diags = append(diags, d)
			continue
		}
		diags = append(diags, cat.Add(doc)...)
	}
	for _, id := range lock.IDs(scope) {
		if !seen[id] {
			diags = append(diags, integrityDiag(DiagWorkflowLockOrphan, filepath.Join(r.path, WorkflowsLockFile), scope.String()+":"+id))
		}
	}
	return diags
}

// checkPrompt compares the prompt template referenced by a published file
// with its lock entry.
func (r *Repo) checkPrompt(scope WorkflowScope, id string, data []byte, entry LockEntry, file string) (workflow.Diagnostic, bool) {
	promptRel, err := r.promptOf(scope, data)
	if err != nil {
		return integrityDiag(DiagWorkflowPromptMismatch, file, scope.String()+":"+id, err), false
	}
	if promptRel == "" {
		if entry.PromptHash != "" {
			return integrityDiag(DiagWorkflowPromptMismatch, file, scope.String()+":"+id, "-"), false
		}
		return workflow.Diagnostic{}, true
	}
	pdata, err := os.ReadFile(filepath.Join(r.path, promptRel))
	if err != nil || HashContent(pdata) != entry.PromptHash {
		return integrityDiag(DiagWorkflowPromptMismatch, file, scope.String()+":"+id, promptRel), false
	}
	return workflow.Diagnostic{}, true
}

// PromptSource reads the prompt templates of team-state documents relative
// to their scope's workflows/ dir (published, drafts and history alike);
// other documents (hub) go to Fallback.
type PromptSource struct {
	Repo     *Repo
	Fallback workflow.PromptSource
}

// ReadPrompt implements workflow.PromptSource.
func (p PromptSource) ReadPrompt(origin workflow.Origin, path string) ([]byte, error) {
	if p.Repo != nil && (origin.Layer == workflow.LayerTeam || origin.Layer == workflow.LayerProject) && origin.Source != "" {
		if dir, ok := p.Repo.workflowsDirOf(origin.Source); ok {
			if own := ownPromptOf(origin.Source); own != "" {
				return os.ReadFile(own)
			}
			full := filepath.Join(dir, filepath.FromSlash(path))
			if r, err := filepath.Rel(dir, full); err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
				return nil, fmt.Errorf("%w: prompt path %q", ErrUnsafeName, path)
			}
			return os.ReadFile(full)
		}
	}
	if p.Fallback == nil {
		return nil, os.ErrNotExist
	}
	return p.Fallback.ReadPrompt(origin, path)
}

// workflowsDirOf returns the scope workflows/ dir that contains file.
func (r *Repo) workflowsDirOf(file string) (string, bool) {
	root := filepath.Clean(r.path)
	for dir := filepath.Dir(filepath.Clean(file)); dir != root && dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if filepath.Base(dir) != WorkflowsDirName {
			continue
		}
		parent := filepath.Dir(dir)
		if parent == root || filepath.Base(filepath.Dir(parent)) == "projects" && filepath.Dir(filepath.Dir(parent)) == root {
			return dir, true
		}
	}
	return "", false
}
