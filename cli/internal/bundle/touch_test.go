package bundle

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// N16: a reused bundle is marked as used (oh gc keeps it for a grace period).
func TestBuildTouchesAReusedBundle(t *testing.T) {
	out := t.TempDir()
	req := Request{HubDir: repoHub(t), OutDir: out, ProjectPath: t.TempDir(), Spec: agentSpec(t, repoHub(t), "orchestrator-dev"), Provider: "bedrock"}
	b, err := Build(req)
	require.NoError(t, err)
	old := time.Now().Add(-72 * time.Hour)
	require.NoError(t, os.Chtimes(b.Dir, old, old))
	again, err := Build(req)
	require.NoError(t, err)
	require.Equal(t, b.Dir, again.Dir)
	fi, err := os.Stat(filepath.Clean(b.Dir))
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now(), fi.ModTime(), time.Minute)
}
