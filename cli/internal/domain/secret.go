package domain

// SecretStore defines the contract for credential storage.
// Implementations may use the OS keychain, an encrypted file, or an env-based provider.
type SecretStore interface {
	// Get retrieves a secret by key. Returns empty string if not found.
	Get(key string) (string, error)
	// Set stores a secret under the given key.
	Set(key, value string) error
	// Delete removes a secret by key. No-op if not found.
	Delete(key string) error
	// List returns all stored key names (not values).
	List() ([]string, error)
}
