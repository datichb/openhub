// Package keychain provides an OS keychain implementation of domain.SecretStore.
package keychain

import (
	"fmt"
	"sync"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/zalando/go-keyring"
)

const serviceName = "openhub-oh"

// Store implements domain.SecretStore using the OS keychain.
type Store struct {
	mu   sync.Mutex
	keys []string // in-memory cache of known keys
}

// New creates a new keychain-backed SecretStore.
func New() *Store {
	return &Store{}
}

// Ensure interface compliance at compile time.
var _ domain.SecretStore = (*Store)(nil)

func (s *Store) Get(key string) (string, error) {
	val, err := keyring.Get(serviceName, key)
	if err != nil {
		if err == keyring.ErrNotFound {
			return "", nil
		}
		return "", fmt.Errorf("keychain get %q: %w", key, err)
	}
	return val, nil
}

func (s *Store) Set(key, value string) error {
	if err := keyring.Set(serviceName, key, value); err != nil {
		return fmt.Errorf("keychain set %q: %w", key, err)
	}
	s.mu.Lock()
	s.addKey(key)
	s.mu.Unlock()
	return nil
}

func (s *Store) Delete(key string) error {
	if err := keyring.Delete(serviceName, key); err != nil {
		if err == keyring.ErrNotFound {
			return nil
		}
		return fmt.Errorf("keychain delete %q: %w", key, err)
	}
	s.mu.Lock()
	s.removeKey(key)
	s.mu.Unlock()
	return nil
}

func (s *Store) List() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]string, len(s.keys))
	copy(result, s.keys)
	return result, nil
}

func (s *Store) addKey(key string) {
	for _, k := range s.keys {
		if k == key {
			return
		}
	}
	s.keys = append(s.keys, key)
}

func (s *Store) removeKey(key string) {
	for i, k := range s.keys {
		if k == key {
			s.keys = append(s.keys[:i], s.keys[i+1:]...)
			return
		}
	}
}
