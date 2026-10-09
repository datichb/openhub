package remote

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackDeterministicAndRoundtrip(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "skills", "a"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bundle.json"), []byte(`{"x":1}`), 0o444))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "skills", "a", "SKILL.md"), []byte("# a"), 0o444))

	a, err := PackDir(dir)
	require.NoError(t, err)
	b, err := PackDir(dir)
	require.NoError(t, err)
	assert.Equal(t, SHA256(a), SHA256(b), "same content, same bytes")

	files, err := UnpackFiles(bytes.NewReader(a))
	require.NoError(t, err)
	assert.Equal(t, map[string][]byte{"bundle.json": []byte(`{"x":1}`), "skills/a/SKILL.md": []byte("# a")}, files)

	out := t.TempDir()
	require.NoError(t, UnpackDir(bytes.NewReader(a), out))
	got, err := os.ReadFile(filepath.Join(out, "skills", "a", "SKILL.md"))
	require.NoError(t, err)
	assert.Equal(t, "# a", string(got))

	require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(dir, "link")))
	_, err = PackDir(dir)
	assert.Error(t, err)
}

func TestPackRefusesUnsafeNames(t *testing.T) {
	for _, n := range []string{"../x", "/abs", "a/../../b", "a\\b", ""} {
		_, err := PackFiles(map[string][]byte{n: nil})
		assert.Error(t, err, n)
	}
}

func TestProjectTokenVar(t *testing.T) {
	assert.Equal(t, "OH_PROJECT_TOKEN_42", ProjectTokenVar(42))
	id, ok := ProjectTokenID("OH_PROJECT_TOKEN_42")
	assert.True(t, ok)
	assert.Equal(t, int64(42), id)
	_, ok = ProjectTokenID("OH_PROJECT_TOKEN_x")
	assert.False(t, ok)
}

// N21: the journal key travels encrypted for the target, bound to its
// session; only the private key of the target opens it.
func TestSealJournalKey(t *testing.T) {
	pub, priv, err := NewJournalKeyPair()
	require.NoError(t, err)
	sealed, err := SealJournalKey(pub, "ses_1", "journal-key")
	require.NoError(t, err)
	assert.NotContains(t, sealed, "journal-key")
	got, err := OpenJournalKey(priv, "ses_1", sealed)
	require.NoError(t, err)
	assert.Equal(t, "journal-key", got)

	_, err = OpenJournalKey(priv, "ses_2", sealed)
	assert.ErrorIs(t, err, ErrJournalKeySealed, "bound to the session")
	_, other, _ := NewJournalKeyPair()
	_, err = OpenJournalKey(other, "ses_1", sealed)
	assert.ErrorIs(t, err, ErrJournalKeySealed, "another key pair")
	raw := []byte(sealed)
	raw[len(raw)-2] ^= 1
	_, err = OpenJournalKey(priv, "ses_1", string(raw))
	assert.Error(t, err, "modified")
	again, _ := SealJournalKey(pub, "ses_1", "journal-key")
	assert.NotEqual(t, sealed, again, "ephemeral key")
	_, err = SealJournalKey("not-a-key", "ses_1", "k")
	assert.Error(t, err)
	derived, err := JournalPublicKey(priv)
	require.NoError(t, err)
	assert.Equal(t, pub, derived)
	assert.GreaterOrEqual(t, len(priv), 8, "GitLab masks it")
	assert.Regexp(t, `^[A-Za-z0-9+/]+$`, priv, "base64 alphabet only (maskable)")
}
