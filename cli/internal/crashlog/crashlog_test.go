package crashlog

import (
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteKeepsTheValueAndTheStack(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OH_HOME", home)

	path, err := Write("boom", debug.Stack(), time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, "run", "crash-20261103T100000.000Z.log"), path)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	body := string(data)
	assert.Contains(t, body, "oh dev")
	assert.Contains(t, body, "panic: boom")
	assert.Contains(t, body, "time: 2026-11-03T10:00:00Z")
	assert.Contains(t, body, "TestWriteKeepsTheValueAndTheStack")

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Zero(t, info.Mode().Perm()&0o077, "the crash file is private to the user")
	}
}

func TestWriteTwiceInTheSameMillisecond(t *testing.T) {
	t.Setenv("OH_HOME", t.TempDir())
	now := time.Now()
	a, err := Write("one", nil, now)
	require.NoError(t, err)
	b, err := Write("two", nil, now)
	require.NoError(t, err)
	assert.NotEqual(t, a, b, "the second report does not replace the first")
}

func TestWriteKeepsTheLatestFiles(t *testing.T) {
	t.Setenv("OH_HOME", t.TempDir())
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var first string
	for i := 0; i < keep+5; i++ {
		p, err := Write("p", nil, start.Add(time.Duration(i)*time.Second))
		require.NoError(t, err)
		if i == 0 {
			first = p
		}
	}
	files, err := filepath.Glob(filepath.Join(Dir(), "crash-*.log"))
	require.NoError(t, err)
	assert.Len(t, files, keep)
	assert.NoFileExists(t, first, "the oldest report goes first")
}

func TestWriteIsOffInATestWithoutAHubOfItsOwn(t *testing.T) {
	t.Setenv("OH_HOME", "")
	_, err := Write("boom", nil, time.Now())
	assert.ErrorIs(t, err, ErrDisabled)
}

func TestWriteFailsWhenTheFolderCannotBeCreated(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	t.Setenv("OH_HOME", file)
	_, err := Write("boom", nil, time.Now())
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrDisabled)
}
