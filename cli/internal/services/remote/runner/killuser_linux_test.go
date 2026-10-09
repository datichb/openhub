//go:build linux

package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserPIDs(t *testing.T) {
	proc := t.TempDir()
	for pid, status := range map[string]string{
		"100":  "Name:\tsh\nState:\tS (sleeping)\nUid:\t10001\t10001\t10001\t10001\n",
		"101":  "Name:\tsetuid\nState:\tR (running)\nUid:\t0\t10001\t0\t0\n",
		"102":  "Name:\tdead\nState:\tZ (zombie)\nUid:\t10001\t10001\t10001\t10001\n",
		"103":  "Name:\troot\nState:\tS (sleeping)\nUid:\t0\t0\t0\t0\n",
		"self": "Name:\tx\nUid:\t10001\t10001\t10001\t10001\n",
	} {
		require.NoError(t, os.MkdirAll(filepath.Join(proc, pid), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(proc, pid, "status"), []byte(status), 0o644))
	}
	assert.ElementsMatch(t, []int{100, 101}, userPIDs(proc, 10001))
	assert.Empty(t, userPIDs(proc, 42))
	require.NoError(t, KillUser(nil))
}
