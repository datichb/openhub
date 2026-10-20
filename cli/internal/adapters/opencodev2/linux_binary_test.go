package opencodev2

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

// cachedTool downloads the binary once from a fake registry and returns the
// tool, the cached path and the download counter.
func cachedTool(t *testing.T, content string) (*LinuxTool, string, *int32, *httptest.Server) {
	t.Helper()
	tgz := npmTarball(t, content)
	sum := sha512.Sum512(tgz)
	var downloads int32
	srv := fakeRegistry(t, tgz, "sha512-"+base64.StdEncoding.EncodeToString(sum[:]), &downloads)
	tool := &LinuxTool{Ver: "2.0.20", CacheDir: t.TempDir(), Registry: srv.URL}
	p, err := tool.LinuxBinary(context.Background(), "arm64", "musl")
	require.NoError(t, err)
	require.EqualValues(t, 1, downloads)
	return tool, p, &downloads, srv
}

// S-38: the SHA-256 of the binary is written next to it at download time.
func TestLinuxBinaryWritesItsFingerprint(t *testing.T) {
	_, p, _, _ := cachedTool(t, "ELF-binary")
	sum := sha256.Sum256([]byte("ELF-binary"))
	data, err := os.ReadFile(p + binarySumSuffix)
	require.NoError(t, err)
	assert.Equal(t, hex.EncodeToString(sum[:]), strings.TrimSpace(string(data)))
}

// S-38: a cached binary is checked against its fingerprint before each reuse,
// and downloaded again when it differs.
func TestLinuxBinaryIsCheckedBeforeReuse(t *testing.T) {
	sidecar := func(p string) string { return p + binarySumSuffix }
	cases := map[string]func(t *testing.T, p string){
		"binary modified": func(t *testing.T, p string) {
			require.NoError(t, os.WriteFile(p, []byte("backdoored"), 0o755))
		},
		"binary appended": func(t *testing.T, p string) {
			f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0)
			require.NoError(t, err)
			_, err = f.WriteString("#!payload")
			require.NoError(t, err)
			require.NoError(t, f.Close())
		},
		"fingerprint missing": func(t *testing.T, p string) {
			require.NoError(t, os.Remove(sidecar(p)))
		},
		"fingerprint wrong": func(t *testing.T, p string) {
			require.NoError(t, os.WriteFile(sidecar(p), []byte(strings.Repeat("0", 64)+"\n"), 0o644))
		},
		"fingerprint empty": func(t *testing.T, p string) {
			require.NoError(t, os.WriteFile(sidecar(p), nil, 0o644))
		},
		"binary replaced by a link": func(t *testing.T, p string) {
			evil := filepath.Join(t.TempDir(), "evil")
			require.NoError(t, os.WriteFile(evil, []byte("ELF-binary"), 0o755))
			require.NoError(t, os.Remove(p))
			require.NoError(t, os.Symlink(evil, p))
		},
	}
	for name, tamper := range cases {
		t.Run(name, func(t *testing.T) {
			tool, p, downloads, _ := cachedTool(t, "ELF-binary")
			tamper(t, p)
			got, err := tool.LinuxBinary(context.Background(), "arm64", "musl")
			require.NoError(t, err)
			assert.Equal(t, p, got)
			assert.EqualValues(t, 2, atomic.LoadInt32(downloads), "downloaded again")
			data, err := os.ReadFile(got)
			require.NoError(t, err)
			assert.Equal(t, "ELF-binary", string(data))
			st, err := os.Lstat(got)
			require.NoError(t, err)
			assert.True(t, st.Mode().IsRegular())
			assert.Equal(t, os.FileMode(0o755), st.Mode().Perm())

			_, err = tool.LinuxBinary(context.Background(), "arm64", "musl")
			require.NoError(t, err)
			assert.EqualValues(t, 2, atomic.LoadInt32(downloads), "then served from the cache again")
		})
	}
}

// A tampered binary is never used, even when the registry cannot be reached.
func TestLinuxBinaryTamperedAndOffline(t *testing.T) {
	tool, p, _, srv := cachedTool(t, "ELF-binary")
	require.NoError(t, os.WriteFile(p, []byte("backdoored"), 0o755))
	srv.Close()
	_, err := tool.LinuxBinary(context.Background(), "arm64", "musl")
	require.Error(t, err)
	_, statErr := os.Stat(p)
	assert.True(t, os.IsNotExist(statErr), "the tampered binary is removed")
}

// A binary cached by an earlier version of oh has no fingerprint: it is
// downloaded again rather than trusted.
func TestLinuxBinaryWithoutFingerprintIsDownloadedAgain(t *testing.T) {
	tgz := npmTarball(t, "ELF-binary")
	sum := sha512.Sum512(tgz)
	var downloads int32
	srv := fakeRegistry(t, tgz, "sha512-"+base64.StdEncoding.EncodeToString(sum[:]), &downloads)
	cache := t.TempDir()
	dir := filepath.Join(cache, "tools", "opencode", "2.0.20", "linux-arm64-musl")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "opencode"), []byte("old cached binary"), 0o755))
	tool := &LinuxTool{Ver: "2.0.20", CacheDir: cache, Registry: srv.URL}
	p, err := tool.LinuxBinary(context.Background(), "arm64", "musl")
	require.NoError(t, err)
	assert.EqualValues(t, 1, downloads)
	data, _ := os.ReadFile(p)
	assert.Equal(t, "ELF-binary", string(data))
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
