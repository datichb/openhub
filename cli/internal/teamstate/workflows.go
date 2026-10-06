package teamstate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/datichb/openhub/cli/internal/workflow"
)

// Workflow layout of the team-state (v5 phase 2, 03 §5):
//
//	workflows/{published,drafts/<member>,prompts,history/<id>}/…   team scope
//	projects/<p>/workflows/{published,drafts/<member>,prompts,history/<id>}/…
//	catalog/{agents,skills}/…                                         team bricks
//	workflows.lock                                                    integrity
//
// Prompt templates are referenced relative to the scope's workflows/ dir
// (`prompt.template: prompts/<id>.md.tmpl`), whatever the subfolder of the
// document (published, drafts, history).
const (
	WorkflowsDirName  = "workflows"
	PublishedDirName  = "published"
	DraftsDirName     = "drafts"
	PromptsDirName    = "prompts"
	HistoryDirName    = "history"
	CatalogDirName    = "catalog"
	WorkflowsLockFile = "workflows.lock"
)

// ErrInvalidWorkflowID is returned for an id that is not kebab-case.
var ErrInvalidWorkflowID = fmt.Errorf("invalid workflow id")

var validWorkflowID = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// ValidWorkflowID checks that id is a kebab-case workflow id (safe as a file
// name).
func ValidWorkflowID(id string) error {
	if !validWorkflowID.MatchString(id) {
		return fmt.Errorf("%w: %q", ErrInvalidWorkflowID, id)
	}
	return nil
}

// WorkflowScope is where a workflow lives in the team-state: the team itself
// (Project empty) or one of its projects.
type WorkflowScope struct {
	Project string
}

// TeamScope is the team-wide scope.
func TeamScope() WorkflowScope { return WorkflowScope{} }

// ProjectScope is the scope of project p.
func ProjectScope(p string) WorkflowScope { return WorkflowScope{Project: p} }

// IsProject reports whether the scope is a project.
func (s WorkflowScope) IsProject() bool { return s.Project != "" }

// Layer is the resolution layer of the scope's documents.
func (s WorkflowScope) Layer() workflow.Layer {
	if s.IsProject() {
		return workflow.LayerProject
	}
	return workflow.LayerTeam
}

func (s WorkflowScope) String() string {
	if s.IsProject() {
		return "project:" + s.Project
	}
	return "team"
}

func (s WorkflowScope) check() error {
	if s.IsProject() {
		if _, err := SafeName(s.Project); err != nil {
			return err
		}
	}
	return nil
}

// relDir is the scope's workflows dir, relative to the repo root.
func (s WorkflowScope) relDir() string {
	if s.IsProject() {
		return filepath.Join("projects", s.Project, WorkflowsDirName)
	}
	return WorkflowsDirName
}

// WorkflowsDir returns the absolute workflows/ dir of scope.
func (r *Repo) WorkflowsDir(scope WorkflowScope) (string, error) {
	if err := scope.check(); err != nil {
		return "", err
	}
	return filepath.Join(r.path, scope.relDir()), nil
}

// workflowRel builds a path relative to the repo root under the scope's
// workflows dir, after validating every name.
func workflowRel(scope WorkflowScope, id string, parts ...string) (string, error) {
	if err := scope.check(); err != nil {
		return "", err
	}
	if err := ValidWorkflowID(id); err != nil {
		return "", err
	}
	return filepath.Join(append([]string{scope.relDir()}, parts...)...), nil
}

// PublishedRel is published/<id>.yaml, relative to the repo root.
func PublishedRel(scope WorkflowScope, id string) (string, error) {
	return workflowRel(scope, id, PublishedDirName, id+".yaml")
}

// DraftRel is drafts/<member>/<id>.yaml, relative to the repo root.
func DraftRel(scope WorkflowScope, member, id string) (string, error) {
	if _, err := SafeName(member); err != nil {
		return "", err
	}
	return workflowRel(scope, id, DraftsDirName, member, id+".yaml")
}

// HistoryRel is history/<id>/<version>.yaml, relative to the repo root.
func HistoryRel(scope WorkflowScope, id string, version int) (string, error) {
	if version <= 0 {
		return "", fmt.Errorf("invalid workflow version %d", version)
	}
	return workflowRel(scope, id, HistoryDirName, id, strconv.Itoa(version)+".yaml")
}

// HistoryPromptRel is history/<id>/<version>.prompt.md.tmpl: the prompt
// template of a version, kept so that a restore is faithful.
func HistoryPromptRel(scope WorkflowScope, id string, version int) (string, error) {
	if version <= 0 {
		return "", fmt.Errorf("invalid workflow version %d", version)
	}
	return workflowRel(scope, id, HistoryDirName, id, strconv.Itoa(version)+".prompt.md.tmpl")
}

// PromptRel resolves a `prompt.template` path of the scope, relative to the
// repo root. The path must stay inside the scope's workflows dir.
func PromptRel(scope WorkflowScope, template string) (string, error) {
	if err := scope.check(); err != nil {
		return "", err
	}
	clean := filepath.Clean(filepath.FromSlash(template))
	if template == "" || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: prompt path %q", ErrUnsafeName, template)
	}
	return filepath.Join(scope.relDir(), clean), nil
}

// EnsureWorkflowLayout creates the workflow folders of scope (idempotent).
func (r *Repo) EnsureWorkflowLayout(scope WorkflowScope) error {
	dir, err := r.WorkflowsDir(scope)
	if err != nil {
		return err
	}
	for _, d := range []string{PublishedDirName, DraftsDirName, PromptsDirName, HistoryDirName} {
		if err := ensureKeptDir(filepath.Join(dir, d)); err != nil {
			return err
		}
	}
	return nil
}

// WorkflowProjects lists the projects that have a workflows/ folder.
func (r *Repo) WorkflowProjects() ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entries, err := os.ReadDir(filepath.Join(r.path, "projects"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := SafeName(e.Name()); err != nil {
			continue
		}
		if st, err := os.Stat(filepath.Join(r.path, "projects", e.Name(), WorkflowsDirName)); err == nil && st.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

// ensureKeptDir creates dir with a .gitkeep so that git tracks it empty.
func ensureKeptDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	keep := filepath.Join(dir, ".gitkeep")
	if _, err := os.Stat(keep); os.IsNotExist(err) {
		if err := os.WriteFile(keep, nil, 0o644); err != nil {
			return fmt.Errorf("creating .gitkeep in %s: %w", dir, err)
		}
	}
	return nil
}
