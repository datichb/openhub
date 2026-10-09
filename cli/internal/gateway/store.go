package gateway

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
)

// Grant is what a gateway token gives access to: one session of a server
// group, in its location, with the workflow Beads allow-list.
type Grant struct {
	SessionID string `json:"session_id"` // oh session
	// Holder is the tool sub-session (subagent) using the token; "" = the
	// session itself. Sub-sessions do not inherit the session environment.
	Holder     string    `json:"holder,omitempty"`
	GatewayURL string    `json:"gateway_url,omitempty"` // as seen from the runtime
	GroupKey   string    `json:"group_key"`
	ProjectID  string    `json:"project_id,omitempty"`
	WorkflowID string    `json:"workflow_id,omitempty"`
	Location   string    `json:"location"`    // machine directory of the session
	BeadsAllow []string  `json:"beads_allow"` // nil = DefaultBeadsAllow, [] = nothing
	CreatedAt  time.Time `json:"created_at"`
	// Base is the Beads workspace fixed when the grant was issued (nil =
	// not resolved yet, see ResolveBase).
	Base *Base `json:"beads_base,omitempty"`
}

// Store keeps the active gateway grants. Tokens themselves are never
// written: only their SHA-256 and the grant scope are saved (0600), so that
// grants survive a daemon restart while tool servers keep running.
type Store struct {
	// ResolveBase fixes the Beads workspace of a new grant (nil = later).
	ResolveBase func(ctx context.Context, location string) *Base

	path   string // "" = memory only
	key    []byte // signs the saved grants (nil = not signed)
	mu     sync.Mutex
	byHash map[string]Grant
	hooks  map[string]hookTicket // one-time hook tokens (memory only)
}

type storedGrant struct {
	Hash string `json:"hash"`
	Grant
}

type storeFile struct {
	Grants []storedGrant `json:"grants"`
	// MAC authenticates the grants with the key of the daemon (HMAC-SHA256
	// of their JSON): a file written by someone else is not trusted.
	MAC string `json:"mac,omitempty"`
}

// maxStoreFile bounds the grant file read at startup.
const maxStoreFile = 16 << 20

// OpenStore loads the grants saved at path ("" = memory only). A missing
// or unreadable file starts empty.
func OpenStore(path string) (*Store, error) { return OpenSignedStore(path, nil) }

// OpenSignedStore is OpenStore with the grants authenticated by key (the
// issuing capability of the daemon; nil = not signed). At startup (audit
// v5, S-14) the file must be a regular file of mode 0600 at most, its MAC
// must match, and each grant must be well formed; otherwise the grants are
// dropped (their sessions get new tokens when resumed).
func OpenSignedStore(path string, key []byte) (*Store, error) {
	s := &Store{path: path, key: key, byHash: map[string]Grant{}}
	if path == "" {
		return s, nil
	}
	fi, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if !fi.Mode().IsRegular() || (runtime.GOOS != "windows" && fi.Mode().Perm()&0o077 != 0) || fi.Size() > maxStoreFile {
		return s, fmt.Errorf("gateway grants not trusted: %s is not a private regular file (%s)", path, fi.Mode())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	var f storeFile
	if err := json.Unmarshal(data, &f); err != nil {
		return s, fmt.Errorf("reading gateway grants: %w", err)
	}
	if len(key) > 0 {
		want, err := grantsMAC(key, f.Grants)
		if err != nil || !hmac.Equal([]byte(want), []byte(f.MAC)) {
			return s, errors.New("gateway grants not trusted: their signature does not match")
		}
	}
	for _, g := range f.Grants {
		if validGrant(g) {
			s.byHash[g.Hash] = g.Grant
		}
	}
	return s, nil
}

var tokenHash = regexp.MustCompile(`^[0-9a-f]{64}$`)

func validGrant(g storedGrant) bool {
	if !tokenHash.MatchString(g.Hash) || g.SessionID == "" || g.GroupKey == "" || !filepath.IsAbs(g.Location) {
		return false
	}
	return g.Base == nil || g.Base.Dir == "" || g.Base.Dir == journalBase || filepath.IsAbs(g.Base.Dir)
}

func grantsMAC(key []byte, grants []storedGrant) (string, error) {
	data, err := json.Marshal(grants)
	if err != nil {
		return "", err
	}
	m := hmac.New(sha256.New, key)
	m.Write(data)
	return hex.EncodeToString(m.Sum(nil)), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// NewToken returns a new random gateway token ("ohg_" + 64 hex chars).
func NewToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("gateway: crypto/rand failed: %v", err))
	}
	return beadswire.TokenPrefix + hex.EncodeToString(b)
}

// Issue creates a token for g. The previous token of the same session and
// holder is revoked (a restarted server gets new ones).
func (s *Store) Issue(g Grant) (string, error) {
	if g.SessionID == "" || g.GroupKey == "" || g.Location == "" {
		return "", errors.New("gateway: session, group and location are required")
	}
	if g.CreatedAt.IsZero() {
		g.CreatedAt = time.Now()
	}
	if g.Base == nil && s.ResolveBase != nil {
		g.Base = s.ResolveBase(context.Background(), g.Location)
	}
	tok := NewToken()
	s.mu.Lock()
	defer s.mu.Unlock()
	for h, old := range s.byHash {
		if old.SessionID == g.SessionID && old.Holder == g.Holder {
			delete(s.byHash, h)
		}
	}
	s.byHash[hashToken(tok)] = g
	if err := s.saveLocked(); err != nil {
		delete(s.byHash, hashToken(tok))
		return "", err
	}
	return tok, nil
}

// Lookup returns the grant of a token.
func (s *Store) Lookup(token string) (Grant, bool) {
	if token == "" {
		return Grant{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.byHash[hashToken(token)]
	return g, ok
}

// Root returns the grant of a session itself (not of its sub-sessions).
func (s *Store) Root(sessionID string) (Grant, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, g := range s.byHash {
		if g.SessionID == sessionID && g.Holder == "" {
			return g, true
		}
	}
	return Grant{}, false
}

// RevokeOwner revokes every grant of a server group.
func (s *Store) RevokeOwner(group string) {
	s.revoke(func(g Grant) bool { return g.GroupKey == group })
}

// RevokeSession revokes the grants of a session and of its sub-sessions.
func (s *Store) RevokeSession(id string) {
	s.revoke(func(g Grant) bool { return g.SessionID == id })
}

// RevokeHolder revokes the grant of one sub-session of a session (true
// when there was one).
func (s *Store) RevokeHolder(session, holder string) bool {
	if holder == "" {
		return false
	}
	found := false
	s.revoke(func(g Grant) bool {
		m := g.SessionID == session && g.Holder == holder
		found = found || m
		return m
	})
	return found
}

// Retain keeps only the grants for which keep returns true.
func (s *Store) Retain(keep func(Grant) bool) {
	s.revoke(func(g Grant) bool { return !keep(g) })
}

func (s *Store) revoke(match func(Grant) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.byHash)
	for h, g := range s.byHash {
		if match(g) {
			delete(s.byHash, h)
		}
	}
	if len(s.byHash) != n {
		_ = s.saveLocked()
	}
}

// Len is the number of active grants.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.byHash)
}

func (s *Store) saveLocked() error {
	if s.path == "" {
		return nil
	}
	f := storeFile{Grants: make([]storedGrant, 0, len(s.byHash))}
	for h, g := range s.byHash {
		f.Grants = append(f.Grants, storedGrant{Hash: h, Grant: g})
	}
	sort.Slice(f.Grants, func(i, j int) bool { return f.Grants[i].Hash < f.Grants[j].Hash })
	if len(s.key) > 0 {
		mac, err := grantsMAC(s.key, f.Grants)
		if err != nil {
			return err
		}
		f.MAC = mac
	}
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// setBase remembers the Beads workspace resolved for the grants of a
// session holder.
func (s *Store) setBase(g Grant, base *Base) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for h, old := range s.byHash {
		if old.SessionID == g.SessionID && old.Holder == g.Holder && old.Base == nil {
			old.Base = base
			s.byHash[h] = old
		}
	}
	_ = s.saveLocked()
}
