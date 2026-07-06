// Package filecrypt provides an encrypted-file implementation of domain.SecretStore.
// It is used as a fallback when the OS keychain (go-keyring) is unavailable —
// typically on Linux without D-Bus, containers, or headless servers.
//
// Secrets are stored in a single file encrypted with AES-256-GCM.
// The encryption key is derived from a user passphrase using Argon2id.
//
// File format (binary):
//
//	[4B] magic "OHSF"
//	[1B] version (0x01)
//	[16B] salt (Argon2id)
//	[12B] nonce (AES-GCM)
//	[NB] ciphertext (AES-256-GCM sealed JSON payload)
package filecrypt

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"golang.org/x/crypto/argon2"

	"github.com/datichb/openhub/cli/internal/domain"
)

const (
	magic      = "OHSF"
	version    = 0x01
	headerSize = 4 + 1 + 16 + 12 // magic + version + salt + nonce
	saltLen    = 16
	nonceLen   = 12
	keyLen     = 32 // AES-256

	// Argon2id parameters — OWASP minimum: t=3, memory=64MB.
	argonTime    = 3
	argonMemory  = 64 * 1024 // 64 MB
	argonThreads = 4
)

// PromptFunc is a function that obtains a passphrase from the user.
// It receives a boolean indicating whether this is a creation prompt (true)
// or an unlock prompt (false).
type PromptFunc func(creating bool) (string, error)

// Store implements domain.SecretStore using an AES-256-GCM encrypted file.
type Store struct {
	mu       sync.Mutex
	path     string
	secrets  map[string]string
	key      []byte // derived AES key, kept in memory for the session
	salt     []byte // persisted salt
	loaded   bool
	promptFn PromptFunc
}

// Ensure interface compliance at compile time.
var _ domain.SecretStore = (*Store)(nil)

// New creates a new file-based encrypted SecretStore.
// path is the location of the encrypted secrets file (e.g., ~/.oh/secrets.enc).
// promptFn obtains the passphrase — it is called at most once per session.
func New(path string, promptFn PromptFunc) *Store {
	return &Store{
		path:     path,
		secrets:  make(map[string]string),
		promptFn: promptFn,
	}
}

// Get retrieves a secret by key. Returns empty string if not found.
func (s *Store) Get(ctx context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureLoaded(); err != nil {
		return "", err
	}
	return s.secrets[key], nil
}

// Set stores a secret under the given key.
func (s *Store) Set(ctx context.Context, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureLoaded(); err != nil {
		return err
	}
	s.secrets[key] = value
	return s.save()
}

// Delete removes a secret by key. No-op if not found.
func (s *Store) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureLoaded(); err != nil {
		return err
	}
	if _, exists := s.secrets[key]; !exists {
		return nil
	}
	delete(s.secrets, key)
	return s.save()
}

// List returns all stored key names sorted alphabetically.
func (s *Store) List(ctx context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureLoaded(); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(s.secrets))
	for k := range s.secrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

// ensureLoaded loads the secrets file on first access. Must be called with mu held.
func (s *Store) ensureLoaded() error {
	if s.loaded {
		return nil
	}
	return s.load()
}

// load reads and decrypts the secrets file, or initializes a new store if the file doesn't exist.
func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// First use — initialize with a new passphrase
			return s.initialize()
		}
		return fmt.Errorf("filecrypt: reading secrets file: %w", err)
	}

	if len(data) < headerSize {
		return fmt.Errorf("filecrypt: file too short — corrupted. Delete %s to reset", s.path)
	}

	// Validate magic
	if string(data[:4]) != magic {
		return fmt.Errorf("filecrypt: invalid file format — corrupted. Delete %s to reset", s.path)
	}

	// Validate version
	if data[4] != version {
		return fmt.Errorf("filecrypt: unsupported version %d", data[4])
	}

	salt := data[5:21]
	nonce := data[21:33]
	ciphertext := data[33:]

	if len(ciphertext) == 0 {
		return fmt.Errorf("filecrypt: empty ciphertext — corrupted. Delete %s to reset", s.path)
	}

	// Get passphrase (unlock)
	passphrase, err := s.resolvePassphrase(false)
	if err != nil {
		return fmt.Errorf("filecrypt: obtaining passphrase: %w", err)
	}

	// Derive key
	key := deriveKey(passphrase, salt)

	// Decrypt
	plaintext, err := decrypt(ciphertext, key, nonce)
	if err != nil {
		return fmt.Errorf("filecrypt: wrong passphrase or corrupted file")
	}

	// Parse JSON payload
	var payload secretsPayload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return fmt.Errorf("filecrypt: corrupted payload. Delete %s to reset", s.path)
	}

	s.secrets = payload.Keys
	if s.secrets == nil {
		s.secrets = make(map[string]string)
	}
	s.key = key
	s.salt = salt
	s.loaded = true
	return nil
}

// initialize sets up a new secrets file with a fresh passphrase.
func (s *Store) initialize() error {
	passphrase, err := s.resolvePassphrase(true)
	if err != nil {
		return fmt.Errorf("filecrypt: obtaining passphrase: %w", err)
	}

	// Generate random salt
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("filecrypt: generating salt: %w", err)
	}

	s.key = deriveKey(passphrase, salt)
	s.salt = salt
	s.secrets = make(map[string]string)
	s.loaded = true
	return nil
}

// save encrypts and writes the secrets to disk atomically (tmp + rename).
func (s *Store) save() error {
	// Serialize payload
	payload := secretsPayload{Keys: s.secrets}
	plaintext, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("filecrypt: marshaling secrets: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("filecrypt: generating nonce: %w", err)
	}

	// Encrypt
	ciphertext, err := encrypt(plaintext, s.key, nonce)
	if err != nil {
		return fmt.Errorf("filecrypt: encrypting: %w", err)
	}

	// Build file content
	buf := make([]byte, 0, headerSize+len(ciphertext))
	buf = append(buf, []byte(magic)...)
	buf = append(buf, version)
	buf = append(buf, s.salt...)
	buf = append(buf, nonce...)
	buf = append(buf, ciphertext...)

	// Atomic write: write to temp file then rename
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("filecrypt: creating directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".secrets-*.tmp")
	if err != nil {
		return fmt.Errorf("filecrypt: creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(buf); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("filecrypt: writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("filecrypt: closing temp file: %w", err)
	}

	// Restrictive permissions before rename
	if err := os.Chmod(tmpName, 0o600); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("filecrypt: setting permissions: %w", err)
	}

	if err := os.Rename(tmpName, s.path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("filecrypt: renaming temp file: %w", err)
	}

	return nil
}

// resolvePassphrase obtains the passphrase using the configured strategy:
// 1. If OH_PASSPHRASE env var is set, use it
// 2. If promptFn is available, call it
// 3. Otherwise, return an error
func (s *Store) resolvePassphrase(creating bool) (string, error) {
	// Strategy 1: environment variable
	if env := os.Getenv("OH_PASSPHRASE"); env != "" {
		return env, nil
	}

	// Strategy 2: interactive prompt
	if s.promptFn != nil {
		return s.promptFn(creating)
	}

	// Strategy 3: no passphrase source available
	return "", errors.New("no passphrase available (set OH_PASSPHRASE or run in interactive terminal)")
}

// secretsPayload is the JSON structure stored in the encrypted file.
type secretsPayload struct {
	Keys map[string]string `json:"keys"`
}

// deriveKey derives a 256-bit key from a passphrase and salt using Argon2id.
func deriveKey(passphrase string, salt []byte) []byte {
	return argon2.IDKey([]byte(passphrase), salt, argonTime, argonMemory, argonThreads, keyLen)
}

// encrypt seals plaintext with AES-256-GCM using the given key and nonce.
func encrypt(plaintext, key, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Seal(nil, nonce, plaintext, nil), nil
}

// decrypt opens AES-256-GCM ciphertext with the given key and nonce.
func decrypt(ciphertext, key, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// IsAvailable checks whether a passphrase source is available for the filecrypt store.
// Returns true if either a terminal is available for prompting or OH_PASSPHRASE is set.
func IsAvailable() bool {
	if os.Getenv("OH_PASSPHRASE") != "" {
		return true
	}
	return isTerminal()
}

// isTerminal checks if stdin is a terminal.
func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// VerifyPassphrase checks that a passphrase matches without loading secrets.
// Used internally — not part of the domain.SecretStore interface.
func VerifyPassphrase(path, passphrase string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) < headerSize {
		return errors.New("file too short")
	}
	if string(data[:4]) != magic {
		return errors.New("invalid format")
	}

	salt := data[5:21]
	nonce := data[21:33]
	ciphertext := data[33:]

	key := deriveKey(passphrase, salt)
	_, err = decrypt(ciphertext, key, nonce)
	if err != nil {
		return errors.New("wrong passphrase")
	}
	return nil
}

// ConstantTimeEqual compares two strings in constant time to prevent timing attacks.
func ConstantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
