package keychain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// secretsIndex persists the list of known keychain key names to disk so that
// List() can enumerate them across process restarts.
//
// The index only stores key names, NEVER values. It lives at
// ~/.oh/secrets-index.json and is updated on every Set/Delete.
type secretsIndex struct {
	mu       sync.Mutex
	path     string
	entries  []indexEntry
	loaded   bool
}

type indexEntry struct {
	Key   string `json:"key"`
	Scope string `json:"scope"` // "global" or "<project-id>"
}

type indexFile struct {
	Entries []indexEntry `json:"entries"`
}

func newSecretsIndex(hubDir string) *secretsIndex {
	return &secretsIndex{
		path: filepath.Join(hubDir, "secrets-index.json"),
	}
}

func (idx *secretsIndex) load() {
	if idx.loaded {
		return
	}
	idx.loaded = true
	data, err := os.ReadFile(idx.path)
	if err != nil {
		return // file not found is OK — start empty
	}
	var f indexFile
	if err := json.Unmarshal(data, &f); err != nil {
		return // corrupt index — start fresh
	}
	idx.entries = f.Entries
}

func (idx *secretsIndex) save() {
	f := indexFile{Entries: idx.entries}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(idx.path), 0o755)
	_ = os.WriteFile(idx.path, data, 0o600)
}

func (idx *secretsIndex) add(key, scope string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.load()
	for _, e := range idx.entries {
		if e.Key == key && e.Scope == scope {
			return // already present
		}
	}
	idx.entries = append(idx.entries, indexEntry{Key: key, Scope: scope})
	sort.Slice(idx.entries, func(i, j int) bool {
		if idx.entries[i].Scope != idx.entries[j].Scope {
			return idx.entries[i].Scope < idx.entries[j].Scope
		}
		return idx.entries[i].Key < idx.entries[j].Key
	})
	idx.save()
}

func (idx *secretsIndex) remove(key, scope string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.load()
	filtered := idx.entries[:0]
	for _, e := range idx.entries {
		if e.Key == key && e.Scope == scope {
			continue
		}
		filtered = append(filtered, e)
	}
	idx.entries = filtered
	idx.save()
}

func (idx *secretsIndex) list() []indexEntry {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.load()
	result := make([]indexEntry, len(idx.entries))
	copy(result, idx.entries)
	return result
}
