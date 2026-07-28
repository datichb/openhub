// Package keychain provides an OS keychain implementation of domain.SecretStore.
package keychain

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/zalando/go-keyring"
)

const serviceName = "openhub-oh"

// scopedKey returns the keychain account string for a (key, scope) pair.
// Global secrets use the key directly; project-scoped secrets are prefixed.
func scopedKey(key, scope string) string {
	if scope == "" || scope == "global" {
		return key
	}
	return scope + "/" + key
}

// Store implements domain.SecretStore using the OS keychain.
type Store struct {
	idx *secretsIndex
}

// New creates a new keychain-backed SecretStore.
// hubDir is the path to ~/.oh/ — used to locate the secrets index file.
func New(hubDir string) *Store {
	return &Store{
		idx: newSecretsIndex(hubDir),
	}
}

// NewLegacy creates a Store without a hubDir (index disabled).
// Kept for backward compatibility with callers that don't have hubDir yet.
func NewLegacy() *Store {
	homeDir := ""
	return &Store{
		idx: newSecretsIndex(filepath.Join(homeDir, ".oh")),
	}
}

// Ensure interface compliance at compile time.
var _ domain.SecretStore = (*Store)(nil)

// Get retrieves a secret by key (global scope).
func (s *Store) Get(ctx context.Context, key string) (string, error) {
	return s.GetScoped(ctx, key, "global")
}

// GetScoped retrieves a secret with explicit scope.
// Falls back to global scope if the project-scoped key is not found.
func (s *Store) GetScoped(ctx context.Context, key, scope string) (string, error) {
	// Try project scope first (if not already global)
	if scope != "" && scope != "global" {
		val, err := keyring.Get(serviceName, scopedKey(key, scope))
		if err == nil {
			return val, nil
		}
		if err != keyring.ErrNotFound {
			return "", fmt.Errorf("keychain get %q (scope %s): %w", key, scope, err)
		}
		// Fall through to global
	}

	val, err := keyring.Get(serviceName, key)
	if err != nil {
		if err == keyring.ErrNotFound {
			return "", nil
		}
		return "", fmt.Errorf("keychain get %q: %w", key, err)
	}
	return val, nil
}

// Set stores a secret under the given key (global scope).
func (s *Store) Set(ctx context.Context, key, value string) error {
	return s.SetScoped(ctx, key, value, "global")
}

// SetScoped stores a secret with explicit scope.
func (s *Store) SetScoped(ctx context.Context, key, value, scope string) error {
	account := scopedKey(key, scope)
	if err := keyring.Set(serviceName, account, value); err != nil {
		return fmt.Errorf("keychain set %q: %w", key, err)
	}
	s.idx.add(key, scope)
	return nil
}

// Delete removes a secret by key (global scope). No-op if not found.
func (s *Store) Delete(ctx context.Context, key string) error {
	return s.DeleteScoped(ctx, key, "global")
}

// DeleteScoped removes a secret with explicit scope. No-op if not found.
func (s *Store) DeleteScoped(ctx context.Context, key, scope string) error {
	account := scopedKey(key, scope)
	if err := keyring.Delete(serviceName, account); err != nil {
		if err == keyring.ErrNotFound {
			return nil
		}
		return fmt.Errorf("keychain delete %q: %w", key, err)
	}
	s.idx.remove(key, scope)
	return nil
}

// List returns all known key names (global scope only).
// To list all scopes, use ListAll.
func (s *Store) List(ctx context.Context) ([]string, error) {
	entries := s.idx.list()
	var keys []string
	for _, e := range entries {
		if e.Scope == "global" || e.Scope == "" {
			keys = append(keys, e.Key)
		}
	}
	return keys, nil
}

// ListAll returns all known secret entries (all scopes) with their presence status.
func (s *Store) ListAll(ctx context.Context) ([]SecretEntry, error) {
	entries := s.idx.list()
	result := make([]SecretEntry, 0, len(entries))
	for _, e := range entries {
		val, _ := keyring.Get(serviceName, scopedKey(e.Key, e.Scope))
		result = append(result, SecretEntry{
			Key:     e.Key,
			Scope:   e.Scope,
			Present: val != "",
		})
	}
	return result, nil
}

// SecretEntry describes a known secret entry without exposing its value.
type SecretEntry struct {
	Key     string // e.g. "gitlab-token"
	Scope   string // "global" or project ID
	Present bool   // whether a value exists in the keychain
}

// Probe tests whether the OS keychain is functional by attempting a read.
// Returns nil if the keychain is working, or an error describing why it's not.
func Probe() error {
	_, err := keyring.Get(serviceName, "__oh_keychain_probe__")
	if err == keyring.ErrNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	return nil
}

