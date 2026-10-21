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

func TestNotificationStore(t *testing.T) {
	t.Run("most recent first", func(t *testing.T) {
		store := NewNotificationStore(50)
		assert.Equal(t, 0, store.Len(), "empty store")

		store.Add(ToastInfo, "A")
		assert.Equal(t, 1, store.Len(), "length after one add")
		store.Add(ToastError, "B")
		store.Add(ToastSuccess, "C")

		all := store.All()
		require.Len(t, all, 3, "three notifications")
		assert.Equal(t, "C", all[0].Message, "most recent is first")
		assert.Equal(t, "B", all[1].Message, "second")
		assert.Equal(t, "A", all[2].Message, "oldest is last")
		assert.Equal(t, ToastSuccess, all[0].Level, "level of C")
		assert.Equal(t, ToastError, all[1].Level, "level of B")
		assert.Equal(t, ToastInfo, all[2].Level, "level of A")
	})

	t.Run("capacity", func(t *testing.T) {
		tests := []struct {
			name    string
			max     int
			added   int
			wantLen int
		}{
			{"evicts the oldest past the maximum", 3, 5, 3},
			{"zero maximum defaults to 50", 0, 60, 50},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				store := NewNotificationStore(tc.max)
				for i := range tc.added {
					store.Add(ToastInfo, fmt.Sprintf("m%d", i))
				}
				require.Equal(t, tc.wantLen, store.Len(), "retained notifications")
				all := store.All()
				assert.Equal(t, fmt.Sprintf("m%d", tc.added-1), all[0].Message, "newest is kept first")
				assert.Equal(t, fmt.Sprintf("m%d", tc.added-tc.wantLen), all[len(all)-1].Message, "oldest retained is last")
			})
		}
	})

	t.Run("entries carry the level and a time label", func(t *testing.T) {
		store := NewNotificationStore(50)
		store.Add(ToastError, "error message")
		store.Add(ToastSuccess, "success message")

		entries := store.Entries()
		require.Len(t, entries, 2, "two entries")
		assert.Equal(t, 0, entries[0].Level, "ToastSuccess maps to level 0")
		assert.Equal(t, "success message", entries[0].Message, "most recent first")
		assert.Equal(t, 1, entries[1].Level, "ToastError maps to level 1")
		assert.Equal(t, "error message", entries[1].Message, "oldest last")
		timePattern := regexp.MustCompile(`^\d{2}:\d{2}:\d{2}$`)
		assert.Regexp(t, timePattern, entries[0].TimeLabel, "TimeLabel is HH:MM:SS")
		assert.Regexp(t, timePattern, entries[1].TimeLabel, "TimeLabel is HH:MM:SS")
	})

	t.Run("concurrent writers", func(t *testing.T) {
		store := NewNotificationStore(50)
		done := make(chan struct{})
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
		assert.LessOrEqual(t, store.Len(), 50, "the maximum holds under concurrency")
	})
}

func TestNotificationFile(t *testing.T) {
	appendN := func(t *testing.T, path string, n int) {
		t.Helper()
		for i := range n {
			require.NoError(t, AppendToFile(path, ToastInfo, fmt.Sprintf("msg %d", i)), "append %d", i)
		}
	}

	t.Run("append creates the folders and writes one JSON line per entry", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "subdir", "notif.jsonl")
		require.NoError(t, AppendToFile(path, ToastError, "test message"), "first append")
		appendN(t, path, 2)

		data, err := os.ReadFile(path)
		require.NoError(t, err, "file created with its folder")
		lines := splitLines(data)
		require.Len(t, lines, 3, "three JSONL lines")

		var pn persistedNotif
		require.NoError(t, json.Unmarshal([]byte(lines[0]), &pn), "line is valid JSON")
		assert.Equal(t, 1, pn.Level, "ToastError is level 1")
		assert.Equal(t, "test message", pn.Message, "message")
		assert.False(t, pn.Time.IsZero(), "time is set")
	})

	t.Run("read", func(t *testing.T) {
		t.Run("absent file", func(t *testing.T) {
			entries, err := ReadLastN(filepath.Join(t.TempDir(), "nonexistent.jsonl"), 50)
			require.NoError(t, err, "absent file is not an error")
			assert.Empty(t, entries, "no entries")
		})
		t.Run("last N, most recent first", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "notif.jsonl")
			appendN(t, path, 10)
			entries, err := ReadLastN(path, 5)
			require.NoError(t, err, "read")
			require.Len(t, entries, 5, "exactly 5 entries")
			assert.Contains(t, entries[0].Message, "msg 9", "most recent first")
			assert.Contains(t, entries[4].Message, "msg 5", "oldest of the 5 last")
		})
		t.Run("fewer than N returns all", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "notif.jsonl")
			require.NoError(t, AppendToFile(path, ToastInfo, "first"), "append")
			require.NoError(t, AppendToFile(path, ToastError, "second"), "append")
			require.NoError(t, AppendToFile(path, ToastSuccess, "third"), "append")
			entries, err := ReadLastN(path, 50)
			require.NoError(t, err, "read")
			require.Len(t, entries, 3, "all entries")
			assert.Equal(t, "third", entries[0].Message, "most recent first")
			assert.Equal(t, "first", entries[2].Message, "oldest last")
		})
	})

	t.Run("rotate", func(t *testing.T) {
		t.Run("absent file is a no-op", func(t *testing.T) {
			assert.NoError(t, RotateFile(filepath.Join(t.TempDir(), "nonexistent.jsonl"), 500, 7*24*time.Hour), "rotate")
		})
		t.Run("truncates at the maximum of lines", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "notif.jsonl")
			appendN(t, path, 60)
			require.NoError(t, RotateFile(path, 50, 7*24*time.Hour), "rotate")
			entries, err := ReadLastN(path, 100)
			require.NoError(t, err, "read")
			assert.LessOrEqual(t, len(entries), 50, "at most 50 entries after rotation")
		})
		t.Run("removes the expired entries", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "notif.jsonl")
			oldEntry := persistedNotif{Time: time.Now().Add(-8 * 24 * time.Hour), Level: int(ToastInfo), Message: "old message"}
			f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
			require.NoError(t, err, "open")
			require.NoError(t, json.NewEncoder(f).Encode(oldEntry), "write the old entry")
			require.NoError(t, f.Close(), "close")
			require.NoError(t, AppendToFile(path, ToastInfo, "new message"), "append")

			require.NoError(t, RotateFile(path, 500, 7*24*time.Hour), "rotate")
			entries, err := ReadLastN(path, 100)
			require.NoError(t, err, "read")
			require.Len(t, entries, 1, "only the recent entry remains")
			assert.Equal(t, "new message", entries[0].Message, "the recent entry")
		})
	})
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
