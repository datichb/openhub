package teamstate

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

// « Créer » probes with a dry run of the push: a token that may read but not
// write is caught before anything is asked, a good one passes, an empty
// repository too, and nothing is sent.
func TestProbeRemoteWrite(t *testing.T) {
	skipWithoutSh(t)
	emptyTmp(t)
	ctx := t.Context()

	t.Run("a repository that accepts the push, empty or not", func(t *testing.T) {
		bare, _ := gittest.BareClone(t)
		require.NoError(t, ProbeRemoteWrite(ctx, bare))
		empty := gittest.EmptyBare(t)
		require.NoError(t, ProbeRemoteWrite(ctx, empty))
		out := gittest.Run(t, empty, "for-each-ref")
		assert.Empty(t, out, "a dry run creates nothing on the remote")
	})

	t.Run("a missing repository is not an access problem", func(t *testing.T) {
		err := ProbeRemoteWrite(ctx, filepath.Join(t.TempDir(), "absent.git"))
		require.Error(t, err)
		assert.Nil(t, AccessError(err), "%v", err)
		assert.Error(t, ProbeRemoteWrite(ctx, " "))
	})

	t.Run("tokens", func(t *testing.T) {
		bare, _ := gittest.BareClone(t)
		srv := gittest.ServeHTTP(t, bare, "team-state", "bob", "good")
		restore := SetCredentialSource(nil)
		defer restore()

		require.NoError(t, ProbeRemoteWriteAs(ctx, srv.URL, GitCredentials{Username: "bob", Token: "good"}))
		err := ProbeRemoteWriteAs(ctx, srv.URL, GitCredentials{Username: "bob", Token: "bad"})
		require.Error(t, err)
		assert.NotNil(t, AccessError(err), "%v", err)
		assert.NotNil(t, AccessError(ProbeRemoteWrite(ctx, srv.URL)), "no token at all")

		ro := readOnlyProxy(t, srv.URL)
		require.NoError(t, ProbeRemoteAs(ctx, ro, GitCredentials{Username: "bob", Token: "good"}), "the read probe accepts a read-only token")
		err = ProbeRemoteWriteAs(ctx, ro, GitCredentials{Username: "bob", Token: "good"})
		require.Error(t, err)
		assert.NotNil(t, AccessError(err), "the write probe does not: %v", err)
	})

	t.Run("the credential source is used", func(t *testing.T) {
		bare, _ := gittest.BareClone(t)
		srv := gittest.ServeHTTP(t, bare, "team-state", "bob", "good")
		restore := SetCredentialSource(credsFor(srv.URL, "bob", "good", nil))
		defer restore()
		require.NoError(t, ProbeRemoteWrite(ctx, srv.URL))
	})
}

// SSH: the key is refused on the dry run as on the read.
func TestProbeRemoteWriteSSHRefused(t *testing.T) {
	fakeSSH(t)
	assert.NotNil(t, AccessError(ProbeRemoteWrite(t.Context(), "git@gitlab.example.com:org/team.git")))
}
