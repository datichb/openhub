package session

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/fsutil"
)

// Export and import of the session history (oh history export / import and
// the History view of the TUI): one JSON file, the same on both sides.

// HistoryFile is the JSON file of an export.
type HistoryFile struct {
	ExportedAt time.Time        `json:"exported_at"`
	MemberID   string           `json:"member_id,omitempty"`
	Sessions   []domain.Session `json:"sessions"`
}

// HistoryStage tells which step of an export or import failed.
type HistoryStage string

const (
	HistoryList   HistoryStage = "list"   // reading the sessions of the database
	HistoryEncode HistoryStage = "encode" // building the JSON
	HistoryWrite  HistoryStage = "write"  // writing the file
	HistoryRead   HistoryStage = "read"   // reading the file
	HistoryParse  HistoryStage = "parse"  // the file is not an export
)

// HistoryError is the failure of an export or import, with its step: the
// caller words it in its own language (Error is the cause).
type HistoryError struct {
	Stage HistoryStage
	Err   error
}

func (e *HistoryError) Error() string { return e.Err.Error() }
func (e *HistoryError) Unwrap() error { return e.Err }

// HistoryFilePerm is the mode of an exported file.
const HistoryFilePerm os.FileMode = 0o644

// ExportOptions are the choices of an export.
type ExportOptions struct {
	// MemberID is written in the file ("" = none): the member that owns the
	// history, by default the one of the active team (the caller knows it).
	MemberID string
	// Now is the export time (zero = now, in UTC).
	Now time.Time
}

// ExportHistory reads every session and writes them to path, atomically (a
// reader, or a crash, sees the previous file or the whole new one). Returns
// the number of sessions exported.
func ExportHistory(ctx context.Context, sessions domain.SessionStore, path string, opts ExportOptions) (int, error) {
	list, err := sessions.List(ctx, "")
	if err != nil {
		return 0, &HistoryError{HistoryList, err}
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	file := HistoryFile{ExportedAt: now.UTC(), MemberID: opts.MemberID, Sessions: list}
	encoded, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return 0, &HistoryError{HistoryEncode, err}
	}
	if err := fsutil.WriteFileAtomic(path, encoded, HistoryFilePerm); err != nil {
		return 0, &HistoryError{HistoryWrite, err}
	}
	return len(list), nil
}

// ImportOptions are the choices of an import.
type ImportOptions struct {
	// MemberID overrides the member of the file (--member-id).
	MemberID string
	// DefaultMemberID is the member to use when neither the override nor the
	// file name one: the member of the active team ("" = sessions kept as
	// they are).
	DefaultMemberID string
}

// ImportResult counts the sessions of an import.
type ImportResult struct {
	Imported int
	// Skipped are the sessions already in the database, or refused by it.
	Skipped int
}

// ReadHistory reads and parses an export.
func ReadHistory(path string) (*HistoryFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, &HistoryError{HistoryRead, err}
	}
	var file HistoryFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, &HistoryError{HistoryParse, err}
	}
	return &file, nil
}

// ImportHistory adds the sessions of an export that the database does not
// have yet. The member is, in this order: the override, the one of the file,
// the default one; when there is one, it is given to every session imported.
func ImportHistory(ctx context.Context, sessions domain.SessionStore, file *HistoryFile, opts ImportOptions) ImportResult {
	member := opts.MemberID
	if member == "" {
		member = file.MemberID
	}
	if member == "" {
		member = opts.DefaultMemberID
	}
	var res ImportResult
	for _, s := range file.Sessions {
		if member != "" {
			s.MemberID = &member
		}
		if _, err := sessions.Get(ctx, s.ID); err == nil {
			continue
		}
		if err := sessions.Create(ctx, &s); err != nil {
			continue
		}
		res.Imported++
	}
	res.Skipped = len(file.Sessions) - res.Imported
	return res
}

// ImportHistoryFile reads path and imports it.
func ImportHistoryFile(ctx context.Context, sessions domain.SessionStore, path string, opts ImportOptions) (ImportResult, error) {
	file, err := ReadHistory(path)
	if err != nil {
		return ImportResult{}, err
	}
	return ImportHistory(ctx, sessions, file, opts), nil
}
