package daemon

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/filelock"
)

// Issuing capability (M12). The daemon routes that hand out access (proxy
// and gateway tokens, credentials, listeners) or stop sessions require a
// secret shared by the oh CLI/TUI and the daemon. It lives in the machine
// secret store (or, without one, in a 0600 file) and is never put in the
// environment of the tool servers: a tool or an agent that only has the
// socket cannot obtain new tokens.

// CapabilityKey is the secret store key of the issuing capability.
const CapabilityKey = "openhub.daemon.capability"

// CapabilityHeader carries the capability on privileged requests.
const CapabilityHeader = "X-Oh-Capability"

// CapabilitySource tells where the capability is kept.
type CapabilitySource string

const (
	CapabilityKeychain CapabilitySource = "keychain"
	CapabilityFile     CapabilitySource = "file" // no usable secret store
)

// CapabilityStore is the secret store holding the capability.
type CapabilityStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

// CapabilityFile is the fallback file of the capability.
func (p Paths) CapabilityFile() string { return filepath.Join(p.Dir, "capability") }

func (p Paths) capabilityLock() string { return filepath.Join(p.Dir, "capability.lock") }

// LoadCapability returns the issuing capability, creating it on first use:
// in store when it is usable, otherwise in the fallback file (0600). The
// creation is serialized between processes. Order: secret store, then an
// existing file, then a new value in the store, else in a new file.
func LoadCapability(ctx context.Context, store CapabilityStore, paths Paths) (string, CapabilitySource, error) {
	if err := os.MkdirAll(paths.Dir, 0o700); err != nil {
		return "", "", err
	}
	lctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	unlock, err := filelock.LockContext(lctx, paths.capabilityLock())
	if err != nil {
		return "", "", err
	}
	defer unlock()
	usable := false
	if store != nil {
		v, err := store.Get(ctx, CapabilityKey)
		if err == nil && v != "" {
			return v, CapabilityKeychain, nil
		}
		usable = err == nil // a read error (locked, denied): do not write there
	}
	// An existing file is used as is: trying the secret store again at every
	// call would ask the user each time where it is not usable.
	if c, err := readCapabilityFile(paths); err == nil {
		return c, CapabilityFile, nil
	}
	if usable {
		c := newCapability()
		if err := store.Set(ctx, CapabilityKey, c); err == nil {
			return c, CapabilityKeychain, nil
		}
	}
	c := newCapability()
	f, err := os.OpenFile(paths.CapabilityFile(), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", "", fmt.Errorf("creating the daemon capability: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString(c); err != nil {
		return "", "", err
	}
	return c, CapabilityFile, nil
}

// PeekCapability tells where the capability is kept without creating it
// (Doctor): found false when none exists yet.
func PeekCapability(ctx context.Context, store CapabilityStore, paths Paths) (src CapabilitySource, found bool, err error) {
	if store != nil {
		if v, gerr := store.Get(ctx, CapabilityKey); gerr == nil && v != "" {
			return CapabilityKeychain, true, nil
		}
	}
	if _, ferr := readCapabilityFile(paths); ferr == nil {
		return CapabilityFile, true, nil
	} else if !errors.Is(ferr, os.ErrNotExist) {
		return "", false, ferr
	}
	return "", false, nil
}

func readCapabilityFile(paths Paths) (string, error) {
	data, err := os.ReadFile(paths.CapabilityFile())
	if err != nil {
		return "", err
	}
	c := strings.TrimSpace(string(data))
	if c == "" {
		return "", errors.New("empty capability file")
	}
	_ = os.Chmod(paths.CapabilityFile(), 0o600)
	return c, nil
}

func newCapability() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("daemon: crypto/rand failed: %v", err))
	}
	return "ohc_" + hex.EncodeToString(b)
}

// privileged guards a route with the capability (no capability configured =
// no check, for embedded uses and tests).
func (d *Daemon) privileged(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		want := d.opts.Capability
		if want != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get(CapabilityHeader)), []byte(want)) != 1 {
			writeErr(w, http.StatusForbidden, "this daemon route is reserved to the oh CLI")
			return
		}
		h(w, r)
	}
}
