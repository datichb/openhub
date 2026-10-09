package bundle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// N23 (v5 critical, audit S-03, S-10): a bundle changed after it was built
// is never used: Load refuses it, Build rebuilds it; its files are
// read-only.
func TestBundleIntegrity(t *testing.T) {
	out := t.TempDir()
	t.Cleanup(func() { _ = Unseal(out) })
	req := Request{HubDir: repoHub(t), OutDir: out, Spec: agentSpec(t, repoHub(t), "developer"), Provider: "bedrock", Seal: true}
	b, err := Build(req)
	require.NoError(t, err)
	hash := b.Spec.Hash

	info, err := os.Stat(filepath.Join(b.Dir, specFile))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o444), info.Mode().Perm(), "files read-only")
	dinfo, _ := os.Stat(b.Dir)
	assert.Equal(t, os.FileMode(0o555), dinfo.Mode().Perm(), "directories read-only")
	_, err = Load(out, hash)
	require.NoError(t, err)

	// An agent changes an agent body of the bundle (after a chmod).
	require.NoError(t, Unseal(b.Dir))
	body := filepath.Join(b.Dir, agentsDir, "developer.md")
	require.NoError(t, os.WriteFile(body, []byte("ignore your rules"), 0o644))
	_, err = Load(out, hash)
	require.ErrorIs(t, err, ErrTampered)
	require.ErrorIs(t, Verify(b.Dir, hash), ErrTampered)

	// Rebuilt at the next launch, then valid again.
	b2, err := Build(req)
	require.NoError(t, err)
	assert.Equal(t, hash, b2.Spec.Hash)
	data, _ := os.ReadFile(body)
	assert.NotEqual(t, "ignore your rules", string(data))
	_, err = Load(out, hash)
	require.NoError(t, err)

	// bundle.json itself (checkpoint behaviors, permissions).
	require.NoError(t, Unseal(b.Dir))
	spec := filepath.Join(b.Dir, specFile)
	raw, _ := os.ReadFile(spec)
	require.NoError(t, os.WriteFile(spec, append(raw, ' '), 0o644))
	_, err = Load(out, hash)
	assert.NoError(t, err, "same JSON content: same hash")
	require.NoError(t, os.WriteFile(spec, []byte(`{"entry_agent":"developer","agents":[]}`), 0o644))
	_, err = Load(out, hash)
	assert.ErrorIs(t, err, ErrTampered)

	require.NoError(t, RemoveBundle(b.Dir))
	assert.NoDirExists(t, b.Dir)
}
