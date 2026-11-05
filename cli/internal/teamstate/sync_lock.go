package teamstate

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

// SyncLockFile is the lease of the automatic tracker sync, at the root of the
// team-state: the member who synchronized last, and until when the others
// leave the next automatic sync to them (a lease: it expires by itself, so
// that a member who disappears blocks nobody for longer than one interval).
const SyncLockFile = "sync.lock"

// SyncLock is the content of sync.lock.
type SyncLock struct {
	// Member is the member whose sync holds the lease.
	Member string `toml:"member"`
	// AcquiredAt is when that sync ran.
	AcquiredAt Instant `toml:"acquired_at"`
	// ExpiresAt is the end of the lease: from then on, any member's automatic
	// sync may take it over.
	ExpiresAt Instant `toml:"expires_at"`
}

// Held reports whether the lease is still running at now. A lock without
// member or with an unreadable date is no lease.
func (l *SyncLock) Held(now time.Time) bool {
	if l == nil || l.Member == "" {
		return false
	}
	end := l.ExpiresAt.Time()
	return !end.IsZero() && now.Before(end)
}

// NewSyncLock is the lease of member from now for the duration of an interval.
func NewSyncLock(member string, now time.Time, interval time.Duration) SyncLock {
	return SyncLock{Member: member, AcquiredAt: InstantOf(now), ExpiresAt: InstantOf(now.Add(interval))}
}

// ReadSyncLock reads sync.lock from the clone, without network; a missing file
// is no lease (nil).
func (r *Repo) ReadSyncLock() (*SyncLock, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return readSyncLock(r.path)
}

// ReadSyncLock reads sync.lock within the transaction (on top of the latest
// remote state).
func (tx *Tx) ReadSyncLock() (*SyncLock, error) { return readSyncLock(tx.r.path) }

// WriteSyncLock writes sync.lock within the transaction: it is committed and
// pushed with it, and a member who pushed first makes the transaction run
// again, so that the lease has one holder.
func (tx *Tx) WriteSyncLock(l SyncLock) error {
	data, err := toml.Marshal(l)
	if err != nil {
		return fmt.Errorf("marshaling %s: %w", SyncLockFile, err)
	}
	return tx.WriteFile(SyncLockFile, data)
}

func readSyncLock(root string) (*SyncLock, error) {
	data, err := os.ReadFile(filepath.Join(root, SyncLockFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil //nolint:nilnil // no lock file: no lease
		}
		return nil, fmt.Errorf("reading %s: %w", SyncLockFile, err)
	}
	var l SyncLock
	if err := toml.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", SyncLockFile, err)
	}
	return &l, nil
}
