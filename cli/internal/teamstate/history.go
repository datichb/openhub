package teamstate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// History of a workflow: history/<id>/<v>.yaml (the document of version v),
// <v>.prompt.md.tmpl (its prompt template, when it had one) and
// <v>.lock.toml (its lock entry: author, date, message).

// HistoryMetaRel is history/<id>/<version>.lock.toml.
func HistoryMetaRel(scope WorkflowScope, id string, version int) (string, error) {
	rel, err := HistoryRel(scope, id, version)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(rel, ".yaml") + ".lock.toml", nil
}

// HistoryVersion is a previous version of a workflow.
type HistoryVersion struct {
	Version int
	Entry   LockEntry // zero when the metadata file is missing
	Path    string    // absolute path of the YAML file
	Prompt  string    // absolute path of the prompt copy ("" = none)
}

// ListHistory returns the previous versions of id in scope, oldest first.
// It only reads files: it may be called inside a transaction.
func (r *Repo) ListHistory(scope WorkflowScope, id string) ([]HistoryVersion, error) {
	if err := ValidWorkflowID(id); err != nil {
		return nil, err
	}
	wdir, err := r.WorkflowsDir(scope)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(wdir, HistoryDirName, id)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []HistoryVersion
	for _, e := range entries {
		v, err := strconv.Atoi(strings.TrimSuffix(e.Name(), ".yaml"))
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") || err != nil || v <= 0 {
			continue
		}
		h := HistoryVersion{Version: v, Path: filepath.Join(dir, e.Name())}
		h.Prompt = ownPromptOf(h.Path)
		if data, err := os.ReadFile(filepath.Join(dir, strconv.Itoa(v)+".lock.toml")); err == nil {
			_ = toml.Unmarshal(data, &h.Entry)
		}
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// MarshalLockEntry encodes a lock entry for <v>.lock.toml.
func MarshalLockEntry(e LockEntry) ([]byte, error) {
	data, err := toml.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("marshaling lock entry: %w", err)
	}
	return data, nil
}

// PromptOfFile returns the prompt template a workflow file of scope
// references, relative to the repo root ("" when none).
func (r *Repo) PromptOfFile(scope WorkflowScope, data []byte) (string, error) {
	return r.promptOf(scope, data)
}
