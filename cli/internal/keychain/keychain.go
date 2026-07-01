// Package keychain provides cross-platform secret storage.
// Uses the OS keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service)
// with a fallback to encrypted file storage.
package keychain

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

const serviceName = "openhub-oh"

// Set stores a secret in the OS keychain.
func Set(key, value string) error {
	if err := keyring.Set(serviceName, key, value); err != nil {
		return fmt.Errorf("keychain set %q: %w", key, err)
	}
	return nil
}

// Get retrieves a secret from the OS keychain.
func Get(key string) (string, error) {
	val, err := keyring.Get(serviceName, key)
	if err != nil {
		if err == keyring.ErrNotFound {
			return "", nil
		}
		return "", fmt.Errorf("keychain get %q: %w", key, err)
	}
	return val, nil
}

// Delete removes a secret from the OS keychain.
func Delete(key string) error {
	if err := keyring.Delete(serviceName, key); err != nil {
		if err == keyring.ErrNotFound {
			return nil
		}
		return fmt.Errorf("keychain delete %q: %w", key, err)
	}
	return nil
}
