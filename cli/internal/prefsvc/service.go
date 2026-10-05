// Package prefsvc is the PreferenceService (v5, T1): pinned workflows,
// recently used workflows (derived from the sessions), default suggestions
// for the « Démarrer » block, and generic UI settings.
package prefsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
)

const (
	// MaxPins is the maximum number of pinned workflows per scope (T1).
	MaxPins = 5
	// MaxRecents is the maximum number of recent workflows shown (T1).
	MaxRecents = 3

	keyPins = "start.pins"
)

// DefaultSuggestions are shown when nothing is pinned nor recent; only the
// IDs present in the catalogue are kept.
var DefaultSuggestions = []string{"ticket", "feature", "review"}

// ErrPinLimit is returned when a scope already has MaxPins pinned workflows.
var ErrPinLimit = errors.New("pinned workflow limit reached")

// Context is the active context of the TUI (both empty = hub).
type Context struct {
	ProjectID string
	TeamID    string
}

// Scopes returns the preference scopes of the context, most specific first,
// always ending with the global scope.
func (c Context) Scopes() []string {
	var out []string
	if c.ProjectID != "" {
		out = append(out, domain.ProjectPreferenceScope(c.ProjectID))
	}
	if c.TeamID != "" {
		out = append(out, domain.TeamPreferenceScope(c.TeamID))
	}
	return append(out, domain.PreferenceScopeGlobal)
}

// PinnedEntry is a pinned workflow and the scope it was pinned in.
type PinnedEntry struct {
	WorkflowID string
	Scope      string
}

// RecentEntry is a recently used workflow.
type RecentEntry struct {
	WorkflowID string
	LastUsed   time.Time
}

// StartEntries feeds the « Démarrer » block.
type StartEntries struct {
	Pinned    []PinnedEntry // at most MaxPins, most specific scope first
	Recent    []RecentEntry // at most MaxRecents, never pinned ones
	Suggested []string      // only when Pinned and Recent are empty
}

// Service implements the preference operations.
type Service struct {
	store domain.PreferenceStore
	usage domain.WorkflowUsageReader
	// Suggestions overrides DefaultSuggestions (tests, configuration).
	Suggestions []string
}

// New creates the service. usage may be nil (no recent workflows).
func New(store domain.PreferenceStore, usage domain.WorkflowUsageReader) *Service {
	return &Service{store: store, usage: usage}
}

// Pins returns the workflows pinned in a scope, in pin order.
func (s *Service) Pins(ctx context.Context, scope string) ([]string, error) {
	var ids []string
	if _, err := s.Get(ctx, scope, keyPins, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

// IsPinned reports whether a workflow is pinned in a scope.
func (s *Service) IsPinned(ctx context.Context, scope, workflowID string) (bool, error) {
	ids, err := s.Pins(ctx, scope)
	return slices.Contains(ids, workflowID), err
}

// Pin pins a workflow in a scope (no-op when already pinned).
func (s *Service) Pin(ctx context.Context, scope, workflowID string) error {
	if workflowID == "" {
		return errors.New("pin: empty workflow id")
	}
	ids, err := s.Pins(ctx, scope)
	if err != nil {
		return err
	}
	if slices.Contains(ids, workflowID) {
		return nil
	}
	if len(ids) >= MaxPins {
		return ErrPinLimit
	}
	return s.Set(ctx, scope, keyPins, append(ids, workflowID))
}

// Unpin removes a pinned workflow (no-op when not pinned).
func (s *Service) Unpin(ctx context.Context, scope, workflowID string) error {
	ids, err := s.Pins(ctx, scope)
	if err != nil {
		return err
	}
	i := slices.Index(ids, workflowID)
	if i < 0 {
		return nil
	}
	ids = slices.Delete(ids, i, i+1)
	if len(ids) == 0 {
		return s.store.Delete(ctx, scope, keyPins)
	}
	return s.Set(ctx, scope, keyPins, ids)
}

// TogglePin pins or unpins a workflow (`*` in the TUI) and returns the new state.
func (s *Service) TogglePin(ctx context.Context, scope, workflowID string) (bool, error) {
	pinned, err := s.IsPinned(ctx, scope, workflowID)
	if err != nil {
		return false, err
	}
	if pinned {
		return false, s.Unpin(ctx, scope, workflowID)
	}
	return true, s.Pin(ctx, scope, workflowID)
}

// Start computes the « Démarrer » entries of a context. catalog lists the
// workflow IDs available in the context (nil = no filtering): pins and
// recents of unknown workflows (removed, or legacy sessions whose workflow ID
// is an agent) are skipped.
func (s *Service) Start(ctx context.Context, c Context, catalog []string) (*StartEntries, error) {
	known := func(id string) bool { return catalog == nil || slices.Contains(catalog, id) }
	out := &StartEntries{}
	pinned := map[string]bool{}
	for _, scope := range c.Scopes() {
		ids, err := s.Pins(ctx, scope)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			if len(out.Pinned) < MaxPins && !pinned[id] && known(id) {
				pinned[id] = true
				out.Pinned = append(out.Pinned, PinnedEntry{WorkflowID: id, Scope: scope})
			}
		}
	}
	if s.usage != nil {
		uses, err := s.usage.RecentWorkflows(ctx, c.ProjectID, 0)
		if err != nil {
			return nil, err
		}
		for _, u := range uses {
			if len(out.Recent) == MaxRecents {
				break
			}
			if !pinned[u.WorkflowID] && known(u.WorkflowID) {
				out.Recent = append(out.Recent, RecentEntry{WorkflowID: u.WorkflowID, LastUsed: u.LastUsed})
			}
		}
	}
	if len(out.Pinned) == 0 && len(out.Recent) == 0 {
		suggestions := s.Suggestions
		if suggestions == nil {
			suggestions = DefaultSuggestions
		}
		for _, id := range suggestions {
			if known(id) {
				out.Suggested = append(out.Suggested, id)
			}
		}
	}
	return out, nil
}

// Get decodes a preference into out; it reports whether the key was set.
func (s *Service) Get(ctx context.Context, scope, key string, out any) (bool, error) {
	p, err := s.store.Get(ctx, scope, key)
	if errors.Is(err, domain.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(p.Value, out); err != nil {
		return false, fmt.Errorf("decoding preference %s/%s: %w", scope, key, err)
	}
	return true, nil
}

// Set stores a preference (encoded as JSON).
func (s *Service) Set(ctx context.Context, scope, key string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encoding preference %s/%s: %w", scope, key, err)
	}
	return s.store.Set(ctx, scope, key, data)
}

// ScopeLabel is the localized label of a scope (« tout le hub », « ce projet »…).
func ScopeLabel(scope string) string {
	switch {
	case strings.HasPrefix(scope, domain.ProjectPreferenceScope("")):
		return i18n.T("tui.prefs.scope.project")
	case strings.HasPrefix(scope, domain.TeamPreferenceScope("")):
		return i18n.T("tui.prefs.scope.team")
	default:
		return i18n.T("tui.prefs.scope.global")
	}
}

// PinMessage is the toast shown after TogglePin.
func PinMessage(workflowID, scope string, pinned bool) string {
	if pinned {
		return i18n.Tf("tui.prefs.pinned", workflowID, ScopeLabel(scope))
	}
	return i18n.Tf("tui.prefs.unpinned", workflowID, ScopeLabel(scope))
}

// ErrorMessage returns a localized message for the errors of the service.
func ErrorMessage(err error) string {
	if errors.Is(err, ErrPinLimit) {
		return i18n.Tf("tui.prefs.pin_limit", MaxPins)
	}
	return i18n.Tf("tui.prefs.error", err)
}
