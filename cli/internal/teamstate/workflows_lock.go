package teamstate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/datichb/openhub/cli/internal/workflow"
)

// WorkflowLock is workflows.lock: the published version and content hash of
// every workflow of the team-state. Only published files whose hashes match
// their entry are loaded.
//
//	[team.ticket-hotfix]
//	version = 2
//	hash = "sha256:…"          # YAML file
//	prompt_hash = "sha256:…"   # prompt template, when the file references one
//	published_by = "alice"
//	published_at = 2026-10-06T10:00:00Z
//	message = "…"
//
//	[projects.<p>.<id>]
//	…
type WorkflowLock struct {
	Team     map[string]LockEntry            `toml:"team,omitempty"`
	Projects map[string]map[string]LockEntry `toml:"projects,omitempty"`
}

// LockEntry is the lock of one published workflow.
type LockEntry struct {
	Version     int       `toml:"version"`
	Hash        string    `toml:"hash"`
	PromptHash  string    `toml:"prompt_hash,omitempty"`
	PublishedBy string    `toml:"published_by"`
	PublishedAt time.Time `toml:"published_at"`
	Message     string    `toml:"message,omitempty"`
}

// HashContent is the lock hash of a file content ("sha256:<hex>").
func HashContent(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// Entries returns the entries of scope (nil when none).
func (l *WorkflowLock) Entries(scope WorkflowScope) map[string]LockEntry {
	if l == nil {
		return nil
	}
	if scope.IsProject() {
		return l.Projects[scope.Project]
	}
	return l.Team
}

// Get returns the entry of id in scope.
func (l *WorkflowLock) Get(scope WorkflowScope, id string) (LockEntry, bool) {
	e, ok := l.Entries(scope)[id]
	return e, ok
}

// Set records the entry of id in scope.
func (l *WorkflowLock) Set(scope WorkflowScope, id string, e LockEntry) {
	if scope.IsProject() {
		if l.Projects == nil {
			l.Projects = map[string]map[string]LockEntry{}
		}
		if l.Projects[scope.Project] == nil {
			l.Projects[scope.Project] = map[string]LockEntry{}
		}
		l.Projects[scope.Project][id] = e
		return
	}
	if l.Team == nil {
		l.Team = map[string]LockEntry{}
	}
	l.Team[id] = e
}

// Delete removes the entry of id in scope.
func (l *WorkflowLock) Delete(scope WorkflowScope, id string) {
	if scope.IsProject() {
		delete(l.Projects[scope.Project], id)
		if len(l.Projects[scope.Project]) == 0 {
			delete(l.Projects, scope.Project)
		}
		return
	}
	delete(l.Team, id)
}

// IDs returns the ids locked in scope, sorted.
func (l *WorkflowLock) IDs(scope WorkflowScope) []string {
	m := l.Entries(scope)
	out := make([]string, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// ReadWorkflowLock reads workflows.lock; a missing file is an empty lock.
func (r *Repo) ReadWorkflowLock() (*WorkflowLock, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.readWorkflowLock()
}

func (r *Repo) readWorkflowLock() (*WorkflowLock, error) {
	data, err := os.ReadFile(filepath.Join(r.path, WorkflowsLockFile))
	if err != nil {
		if os.IsNotExist(err) {
			return &WorkflowLock{}, nil
		}
		return nil, fmt.Errorf("reading %s: %w", WorkflowsLockFile, err)
	}
	var l WorkflowLock
	if err := toml.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", WorkflowsLockFile, err)
	}
	return &l, nil
}

// WriteWorkflowLockLocal writes workflows.lock without committing. Call it
// inside WithWriteLock (or on a local-only repo).
func (r *Repo) WriteWorkflowLockLocal(l *WorkflowLock) error {
	data, err := toml.Marshal(l)
	if err != nil {
		return fmt.Errorf("marshaling %s: %w", WorkflowsLockFile, err)
	}
	return os.WriteFile(filepath.Join(r.path, WorkflowsLockFile), data, 0o644)
}

// LockMeta describes a publication.
type LockMeta struct {
	Version     int
	PublishedBy string
	PublishedAt time.Time
	Message     string
}

// SealWorkflowLocal records the published file of id in workflows.lock with
// the hashes of its current content (YAML and prompt template). It does not
// commit: call it inside WithWriteLock. It returns the files to commit.
func (r *Repo) SealWorkflowLocal(scope WorkflowScope, id string, meta LockMeta) (LockEntry, []string, error) {
	rel, err := PublishedRel(scope, id)
	if err != nil {
		return LockEntry{}, nil, err
	}
	data, err := os.ReadFile(filepath.Join(r.path, rel))
	if err != nil {
		return LockEntry{}, nil, fmt.Errorf("reading published workflow: %w", err)
	}
	entry := LockEntry{
		Version:     meta.Version,
		Hash:        HashContent(data),
		PublishedBy: meta.PublishedBy,
		PublishedAt: meta.PublishedAt.UTC().Truncate(time.Second),
		Message:     meta.Message,
	}
	files := []string{rel, WorkflowsLockFile}
	promptRel, err := r.promptOf(scope, data)
	if err != nil {
		return LockEntry{}, nil, err
	}
	if promptRel != "" {
		pdata, err := os.ReadFile(filepath.Join(r.path, promptRel))
		if err != nil {
			return LockEntry{}, nil, fmt.Errorf("reading prompt template: %w", err)
		}
		entry.PromptHash = HashContent(pdata)
		files = append(files, promptRel)
	}
	l, err := r.readWorkflowLock()
	if err != nil {
		return LockEntry{}, nil, err
	}
	l.Set(scope, id, entry)
	if err := r.WriteWorkflowLockLocal(l); err != nil {
		return LockEntry{}, nil, err
	}
	return entry, files, nil
}

// promptOf returns the prompt template referenced by a workflow file of
// scope (relative to the repo root), or "" when it has none.
func (r *Repo) promptOf(scope WorkflowScope, data []byte) (string, error) {
	doc, _ := workflow.Parse(data, workflow.Source{Layer: scope.Layer()})
	if doc == nil || doc.Spec.Prompt == nil || doc.Spec.Prompt.Template == "" {
		return "", nil
	}
	return PromptRel(scope, doc.Spec.Prompt.Template)
}
