// Package sessionctx keeps the evolving state of a session known to its
// entry agent (S8, option 2, QB8): checkpoints passed and current, remaining
// budget, resume instruction. Each entry goes through the adapter capability
// (adapters.SessionContextSetter) and is written only when it changes: the
// tool announces every change in the history, which invalidates the prompt
// cache from there. Without the capability, an entry with a fallback text is
// sent as a synthetic message; the others stay readable through the workflow
// MCP tools. Entries are not passed to subagents (tool limit).
package sessionctx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/datichb/openhub/cli/internal/adapters"
)

// Neutral and stable keys.
const (
	KeyCheckpoints = "oh.checkpoints"
	KeyBudget      = "oh.budget"
	KeyResume      = "oh.resume"
)

// Entry is a state entry to set.
type Entry struct {
	Key   string
	Value any
	// Fallback is sent as a synthetic message when the tool has no session
	// context ("" = no fallback: the state stays readable another way).
	Fallback string
}

// Writer writes the entries of the sessions.
type Writer struct {
	Adapter adapters.ToolAdapter
	// Dir keeps, per session, the last value written (~/.oh/sessions; ""
	// = memory only).
	Dir string

	mu          sync.Mutex
	last        map[string]map[string]json.RawMessage // session → key → value
	unsupported map[string]bool                       // server URL → capability gone
}

func (w *Writer) file(sessionID string) string {
	if w.Dir == "" {
		return ""
	}
	return filepath.Join(w.Dir, sessionID, "context.json")
}

// load returns the last values of a session (lock held). With a Dir, the
// file is read again each time: the CLI (resume) and the daemon both write.
func (w *Writer) load(sessionID string) map[string]json.RawMessage {
	if w.last == nil {
		w.last = map[string]map[string]json.RawMessage{}
	}
	f := w.file(sessionID)
	if m, ok := w.last[sessionID]; ok && f == "" {
		return m
	}
	m := map[string]json.RawMessage{}
	if f != "" {
		if data, err := os.ReadFile(f); err == nil {
			_ = json.Unmarshal(data, &m)
		}
	}
	w.last[sessionID] = m
	return m
}

// save persists the last values of a session (lock held).
func (w *Writer) save(sessionID string, m map[string]json.RawMessage) {
	f := w.file(sessionID)
	if f == "" {
		return
	}
	data, err := json.Marshal(m)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(f), 0o700); err == nil {
		_ = os.WriteFile(f, data, 0o600)
	}
}

// Last returns the last value written for key ("" when none).
func (w *Writer) Last(sessionID, key string) json.RawMessage {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.load(sessionID)[key]
}

// supported reports whether the tool keeps a session context on this server.
func (w *Writer) supported(h adapters.ServerHandle) (adapters.SessionContextSetter, bool) {
	setter, ok := w.Adapter.(adapters.SessionContextSetter)
	if !ok || w.Adapter == nil || !w.Adapter.Capabilities().SessionContext {
		return nil, false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return setter, !w.unsupported[h.URL]
}

func (w *Writer) markUnsupported(h adapters.ServerHandle) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.unsupported == nil {
		w.unsupported = map[string]bool{}
	}
	w.unsupported[h.URL] = true
}

// Set writes e when its value changed since the last write; it reports
// whether something was written (or sent as fallback).
func (w *Writer) Set(ctx context.Context, h adapters.ServerHandle, sessionID string, e Entry) (bool, error) {
	data, err := json.Marshal(e.Value)
	if err != nil {
		return false, err
	}
	w.mu.Lock()
	unchanged := bytes.Equal(w.load(sessionID)[e.Key], data)
	w.mu.Unlock()
	if unchanged {
		return false, nil
	}
	setter, ok := w.supported(h)
	if ok {
		err = setter.SetSessionContext(ctx, h, sessionID, e.Key, json.RawMessage(data))
		if errors.Is(err, adapters.ErrUnsupported) {
			slog.Info("session context unavailable on this server: fallback", "session", sessionID, "error", err)
			w.markUnsupported(h)
			ok = false
		} else if err != nil {
			return false, err
		}
	}
	if !ok {
		if e.Fallback == "" {
			return false, nil
		}
		if err := w.Adapter.Control(ctx, h, sessionID, adapters.ControlOp{Kind: adapters.ControlSynthetic, Text: e.Fallback}); err != nil {
			return false, err
		}
	}
	w.mu.Lock()
	m := w.load(sessionID)
	m[e.Key] = data
	w.save(sessionID, m)
	w.mu.Unlock()
	return true, nil
}

// Clear removes key when it was written.
func (w *Writer) Clear(ctx context.Context, h adapters.ServerHandle, sessionID, key string) error {
	w.mu.Lock()
	_, had := w.load(sessionID)[key]
	w.mu.Unlock()
	if !had {
		return nil
	}
	if setter, ok := w.supported(h); ok {
		if err := setter.ClearSessionContext(ctx, h, sessionID, key); err != nil && !errors.Is(err, adapters.ErrUnsupported) {
			return err
		}
	}
	w.mu.Lock()
	m := w.load(sessionID)
	delete(m, key)
	w.save(sessionID, m)
	w.mu.Unlock()
	return nil
}
