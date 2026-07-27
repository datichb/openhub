package shell

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationStore_Add(t *testing.T) {
	store := NewNotificationStore(50)
	assert.Equal(t, 0, store.Len())

	store.Add(ToastInfo, "hello")
	assert.Equal(t, 1, store.Len())

	store.Add(ToastError, "world")
	assert.Equal(t, 2, store.Len())
}

func TestNotificationStore_AllMostRecentFirst(t *testing.T) {
	store := NewNotificationStore(50)

	store.Add(ToastInfo, "A")
	// Small sleep to guarantee distinct timestamps (and insertion order)
	time.Sleep(time.Millisecond)
	store.Add(ToastError, "B")
	time.Sleep(time.Millisecond)
	store.Add(ToastSuccess, "C")

	all := store.All()
	require.Len(t, all, 3)
	assert.Equal(t, "C", all[0].Message, "most recent should be first")
	assert.Equal(t, "B", all[1].Message)
	assert.Equal(t, "A", all[2].Message, "oldest should be last")

	// Levels should match
	assert.Equal(t, ToastSuccess, all[0].Level)
	assert.Equal(t, ToastError, all[1].Level)
	assert.Equal(t, ToastInfo, all[2].Level)
}

func TestNotificationStore_MaxCapacityEvicts(t *testing.T) {
	store := NewNotificationStore(3)

	for _, msg := range []string{"a", "b", "c", "d", "e"} {
		store.Add(ToastInfo, msg)
	}

	assert.Equal(t, 3, store.Len(), "store should not exceed max capacity")

	all := store.All()
	require.Len(t, all, 3)
	// Most recent first: e, d, c
	assert.Equal(t, "e", all[0].Message, "newest entry should be first")
	assert.Equal(t, "d", all[1].Message)
	assert.Equal(t, "c", all[2].Message, "oldest retained should be last")

	// Ensure evicted entries are gone
	for _, n := range all {
		assert.NotEqual(t, "a", n.Message, "oldest entry should have been evicted")
		assert.NotEqual(t, "b", n.Message, "second-oldest should have been evicted")
	}
}

func TestNotificationStore_ZeroMaxDefaultsTo50(t *testing.T) {
	store := NewNotificationStore(0)
	// Fill past any reasonable small cap
	for i := range 60 {
		store.Add(ToastInfo, string(rune('a'+i%26)))
	}
	// Should have retained up to 50
	assert.Equal(t, 50, store.Len())
}

func TestNotificationStore_Entries(t *testing.T) {
	store := NewNotificationStore(50)

	store.Add(ToastError, "error message")
	time.Sleep(time.Millisecond)
	store.Add(ToastSuccess, "success message")

	entries := store.Entries()
	require.Len(t, entries, 2)

	// Most recent first: Success (level 0), then Error (level 1)
	assert.Equal(t, 0, entries[0].Level, "ToastSuccess maps to level 0")
	assert.Equal(t, "success message", entries[0].Message)

	assert.Equal(t, 1, entries[1].Level, "ToastError maps to level 1")
	assert.Equal(t, "error message", entries[1].Message)

	// TimeLabel should be formatted as HH:MM:SS
	timePattern := regexp.MustCompile(`^\d{2}:\d{2}:\d{2}$`)
	assert.Regexp(t, timePattern, entries[0].TimeLabel, "TimeLabel should be HH:MM:SS")
	assert.Regexp(t, timePattern, entries[1].TimeLabel)
}

func TestNotificationStore_ConcurrentAccess(t *testing.T) {
	store := NewNotificationStore(50)
	done := make(chan struct{})

	// 10 concurrent writers
	for range 10 {
		go func() {
			for range 5 {
				store.Add(ToastInfo, "concurrent")
			}
			done <- struct{}{}
		}()
	}
	for range 10 {
		<-done
	}

	// Should not exceed max, no panic
	assert.LessOrEqual(t, store.Len(), 50)
}

// ── AppendToFile ──────────────────────────────────────────────────────────────

func TestAppendToFile_CreatesFileAndDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subdir", "notif.jsonl")
	require.NoError(t, AppendToFile(path, ToastInfo, "hello"))
	_, err := os.Stat(path)
	assert.NoError(t, err, "file should be created")
}

func TestAppendToFile_AppendsMultipleEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notif.jsonl")
	for i := range 3 {
		require.NoError(t, AppendToFile(path, ToastInfo, fmt.Sprintf("msg %d", i)))
	}
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	lines := splitLines(data)
	assert.Len(t, lines, 3, "should have 3 JSONL lines")
}

func TestAppendToFile_EncodesValidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notif.jsonl")
	require.NoError(t, AppendToFile(path, ToastError, "test message"))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var pn persistedNotif
	require.NoError(t, json.Unmarshal(data[:len(data)-1], &pn), "line must be valid JSON")
	assert.Equal(t, 1, pn.Level, "ToastError should be level 1")
	assert.Equal(t, "test message", pn.Message)
	assert.False(t, pn.Time.IsZero(), "time must be set")
}

// ── ReadLastN ─────────────────────────────────────────────────────────────────

func TestReadLastN_FileAbsentReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.jsonl")
	entries, err := ReadLastN(path, 50)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestReadLastN_ReturnsLastNEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notif.jsonl")
	for i := range 10 {
		require.NoError(t, AppendToFile(path, ToastInfo, fmt.Sprintf("msg %d", i)))
	}

	entries, err := ReadLastN(path, 5)
	require.NoError(t, err)
	require.Len(t, entries, 5, "should return exactly 5 entries")
	// Most recent entry first: msg 9
	assert.Contains(t, entries[0].Message, "msg 9", "most recent should be first")
	assert.Contains(t, entries[4].Message, "msg 5", "oldest of the 5 should be last")
}

func TestReadLastN_LessThanNReturnsAll(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notif.jsonl")
	for i := range 3 {
		require.NoError(t, AppendToFile(path, ToastInfo, fmt.Sprintf("msg %d", i)))
	}
	entries, err := ReadLastN(path, 50)
	require.NoError(t, err)
	assert.Len(t, entries, 3)
}

func TestReadLastN_MostRecentFirst(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notif.jsonl")
	require.NoError(t, AppendToFile(path, ToastInfo, "first"))
	require.NoError(t, AppendToFile(path, ToastError, "second"))
	require.NoError(t, AppendToFile(path, ToastSuccess, "third"))

	entries, err := ReadLastN(path, 10)
	require.NoError(t, err)
	require.Len(t, entries, 3)
	assert.Equal(t, "third", entries[0].Message, "most recent must be first")
	assert.Equal(t, "first", entries[2].Message, "oldest must be last")
}

// ── RotateFile ────────────────────────────────────────────────────────────────

func TestRotateFile_NoFileIsNoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.jsonl")
	assert.NoError(t, RotateFile(path, 500, 7*24*time.Hour))
}

func TestRotateFile_TruncatesAtMaxLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notif.jsonl")
	for i := range 60 {
		require.NoError(t, AppendToFile(path, ToastInfo, fmt.Sprintf("msg %d", i)))
	}

	require.NoError(t, RotateFile(path, 50, 7*24*time.Hour))

	entries, err := ReadLastN(path, 100)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(entries), 50, "file should have at most 50 entries after rotation")
}

func TestRotateFile_RemovesExpiredEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notif.jsonl")

	// Write old entry manually (backdated by 8 days)
	oldEntry := persistedNotif{
		Time:    time.Now().Add(-8 * 24 * time.Hour),
		Level:   int(ToastInfo),
		Message: "old message",
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	require.NoError(t, json.NewEncoder(f).Encode(oldEntry))
	f.Close()

	// Write a fresh entry
	require.NoError(t, AppendToFile(path, ToastInfo, "new message"))

	require.NoError(t, RotateFile(path, 500, 7*24*time.Hour))

	entries, err := ReadLastN(path, 100)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "only the recent entry should remain")
	assert.Equal(t, "new message", entries[0].Message)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func splitLines(data []byte) []string {
	var lines []string
	start := 0
	for i, b := range data {
		if b == '\n' {
			if i > start {
				lines = append(lines, string(data[start:i]))
			}
			start = i + 1
		}
	}
	return lines
}
