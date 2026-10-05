package opencodev2

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func npmTarball(t *testing.T, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range map[string]string{"package/package.json": "{}", "package/bin/opencode": content} {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}))
		_, err := tw.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func fakeRegistry(t *testing.T, tgz []byte, integrity string, downloads *int32) *httptest.Server {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.EscapedPath() {
		case "/@opencode%2Fcli-linux-arm64-musl/2.0.20":
			meta := map[string]any{"dist": map[string]string{"tarball": srv.URL + "/t.tgz", "integrity": integrity}}
			_ = json.NewEncoder(w).Encode(meta)
		case "/t.tgz":
			atomic.AddInt32(downloads, 1)
			_, _ = w.Write(tgz)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestLinuxBinaryDownloadsVerifiesAndCaches(t *testing.T) {
	tgz := npmTarball(t, "ELF-binary")
	sum := sha512.Sum512(tgz)
	var downloads int32
	srv := fakeRegistry(t, tgz, "sha512-"+base64.StdEncoding.EncodeToString(sum[:]), &downloads)

	tool := &LinuxTool{Ver: "2.0.20", CacheDir: t.TempDir(), Registry: srv.URL}
	p, err := tool.LinuxBinary(context.Background(), "aarch64", "musl")
	require.NoError(t, err)
	data, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, "ELF-binary", string(data))
	st, _ := os.Stat(p)
	assert.Equal(t, os.FileMode(0o755), st.Mode().Perm())

	_, err = tool.LinuxBinary(context.Background(), "arm64", "musl")
	require.NoError(t, err)
	assert.EqualValues(t, 1, downloads, "second call served from cache")
}

func TestLinuxBinaryRejectsIntegrityMismatch(t *testing.T) {
	tgz := npmTarball(t, "tampered")
	other := sha512.Sum512([]byte("something else"))
	var downloads int32
	srv := fakeRegistry(t, tgz, "sha512-"+base64.StdEncoding.EncodeToString(other[:]), &downloads)
	tool := &LinuxTool{Ver: "2.0.20", CacheDir: t.TempDir(), Registry: srv.URL}
	_, err := tool.LinuxBinary(context.Background(), "arm64", "musl")
	require.ErrorContains(t, err, "integrity mismatch")
}

func TestNPMTarget(t *testing.T) {
	for _, c := range []struct{ arch, libc, want string }{
		{"amd64", "glibc", "linux-x64"}, {"x86_64", "", "linux-x64"}, {"arm64", "musl", "linux-arm64-musl"},
	} {
		got, err := npmTarget(c.arch, c.libc)
		require.NoError(t, err)
		assert.Equal(t, c.want, got)
	}
	_, err := npmTarget("s390x", "")
	assert.Error(t, err)
	_, err = (&LinuxTool{}).LinuxBinary(context.Background(), "arm64", "")
	assert.Error(t, err)
}
