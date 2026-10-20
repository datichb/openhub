package opencodev2

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const pinTarget = "linux-arm64-musl"

type syncBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func captureLogs(t *testing.T) *syncBuffer {
	t.Helper()
	var buf syncBuffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// pinRegistry serves a tarball whose own sha512 is the integrity it announces
// (a registry that is consistent with itself, as a compromised one would be).
func pinRegistry(t *testing.T, content string) (reg, integrity string, downloads *int32) {
	t.Helper()
	tgz := npmTarball(t, content)
	sum := sha512.Sum512(tgz)
	integrity = "sha512-" + base64.StdEncoding.EncodeToString(sum[:])
	downloads = new(int32)
	return fakeRegistry(t, tgz, integrity, downloads).URL, integrity, downloads
}

func otherIntegrity() string {
	sum := sha512.Sum512([]byte("the integrity oh pinned"))
	return "sha512-" + base64.StdEncoding.EncodeToString(sum[:])
}

// S-38: a pinned version is refused when the registry announces another
// integrity, even one that matches the archive it serves.
func TestLinuxBinaryPinnedMismatchIsRefused(t *testing.T) {
	reg, _, downloads := pinRegistry(t, "backdoored ELF")
	tool := &LinuxTool{Ver: "2.0.20", CacheDir: t.TempDir(), Registry: reg,
		Pins: IntegrityPins{"2.0.20": {pinTarget: otherIntegrity()}}}
	_, err := tool.LinuxBinary(context.Background(), "arm64", "musl")
	require.ErrorContains(t, err, "not the one pinned")
	assert.Zero(t, *downloads, "the archive is not even downloaded")
	_, statErr := os.Stat(filepath.Join(tool.CacheDir, "tools", "opencode", "2.0.20", pinTarget, "opencode"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestLinuxBinaryPinnedMatchIsAccepted(t *testing.T) {
	logs := captureLogs(t)
	reg, integrity, downloads := pinRegistry(t, "ELF-binary")
	tool := &LinuxTool{Ver: "2.0.20", CacheDir: t.TempDir(), Registry: reg,
		Pins: IntegrityPins{"2.0.20": {pinTarget: integrity}}, Strict: func() bool { return true }}
	p, err := tool.LinuxBinary(context.Background(), "arm64", "musl")
	require.NoError(t, err)
	data, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, "ELF-binary", string(data))
	assert.NotContains(t, logs.String(), "not pinned", "no warning for a pinned version")
	got, err := os.ReadFile(p + binaryPinSuffix)
	require.NoError(t, err)
	assert.Equal(t, integrity, strings.TrimSpace(string(got)))

	_, err = tool.LinuxBinary(context.Background(), "arm64", "musl")
	require.NoError(t, err)
	assert.EqualValues(t, 1, *downloads, "served from the cache")
}

// The pin of another target does not cover this one.
func TestLinuxBinaryPinIsPerTarget(t *testing.T) {
	reg, _, downloads := pinRegistry(t, "ELF-binary")
	tool := &LinuxTool{Ver: "2.0.20", CacheDir: t.TempDir(), Registry: reg,
		Pins: IntegrityPins{"2.0.20": {"linux-x64": otherIntegrity()}}, Strict: func() bool { return true }}
	_, err := tool.LinuxBinary(context.Background(), "arm64", "musl")
	require.ErrorContains(t, err, "not pinned")
	assert.Zero(t, *downloads)
}

// A binary cached before the version was pinned (or under another pin) was
// never checked against it: it is downloaded again and checked.
func TestLinuxBinaryCachedBeforeThePinIsDownloadedAgain(t *testing.T) {
	reg, integrity, downloads := pinRegistry(t, "ELF-binary")
	cache := t.TempDir()
	unpinned := &LinuxTool{Ver: "2.0.20", CacheDir: cache, Registry: reg, Pins: IntegrityPins{}}
	p, err := unpinned.LinuxBinary(context.Background(), "arm64", "musl")
	require.NoError(t, err)
	require.EqualValues(t, 1, *downloads)
	_, statErr := os.Stat(p + binaryPinSuffix)
	require.True(t, os.IsNotExist(statErr), "an unpinned download records no pin")

	pinned := &LinuxTool{Ver: "2.0.20", CacheDir: cache, Registry: reg, Pins: IntegrityPins{"2.0.20": {pinTarget: integrity}}}
	_, err = pinned.LinuxBinary(context.Background(), "arm64", "musl")
	require.NoError(t, err)
	assert.EqualValues(t, 2, *downloads, "downloaded again under the pin")

	// …and a cache made under a different pin is not reused either.
	repinned := &LinuxTool{Ver: "2.0.20", CacheDir: cache, Registry: reg, Pins: IntegrityPins{"2.0.20": {pinTarget: otherIntegrity()}}}
	_, err = repinned.LinuxBinary(context.Background(), "arm64", "musl")
	require.ErrorContains(t, err, "not the one pinned")
	_, statErr = os.Stat(p)
	assert.True(t, os.IsNotExist(statErr), "the binary of the old pin is not left to be used")
}

// A version that is not pinned is downloaded as before, with a warning.
func TestLinuxBinaryUnpinnedVersionWarns(t *testing.T) {
	logs := captureLogs(t)
	reg, _, downloads := pinRegistry(t, "ELF-binary")
	tool := &LinuxTool{Ver: "2.0.20", CacheDir: t.TempDir(), Registry: reg, Pins: IntegrityPins{"2.0.19": {pinTarget: otherIntegrity()}}}
	_, err := tool.LinuxBinary(context.Background(), "arm64", "musl")
	require.NoError(t, err)
	assert.EqualValues(t, 1, *downloads)
	assert.Contains(t, logs.String(), "level=WARN")
	assert.Contains(t, logs.String(), "not pinned")
	assert.Contains(t, logs.String(), "@opencode/cli-"+pinTarget)
	assert.Contains(t, logs.String(), "2.0.20")
}

// Under strict isolation an unpinned version is refused, also when its binary
// is already cached.
func TestLinuxBinaryUnpinnedVersionRefusedUnderStrictIsolation(t *testing.T) {
	reg, _, downloads := pinRegistry(t, "ELF-binary")
	cache := t.TempDir()
	tool := &LinuxTool{Ver: "2.0.20", CacheDir: cache, Registry: reg, Pins: IntegrityPins{}}
	_, err := tool.LinuxBinary(context.Background(), "arm64", "musl")
	require.NoError(t, err)
	require.EqualValues(t, 1, *downloads)

	tool.Strict = func() bool { return true }
	_, err = tool.LinuxBinary(context.Background(), "arm64", "musl")
	require.ErrorContains(t, err, "execution.strict_isolation")
	require.ErrorContains(t, err, "not pinned")

	fresh := &LinuxTool{Ver: "2.0.20", CacheDir: t.TempDir(), Registry: reg, Pins: IntegrityPins{}, Strict: func() bool { return true }}
	_, err = fresh.LinuxBinary(context.Background(), "arm64", "musl")
	require.ErrorContains(t, err, "not pinned")
	assert.EqualValues(t, 1, *downloads, "nothing is downloaded")
}

// With no table given, the one embedded in oh is used: a registry that
// announces something else than the pinned 2.0.20 is refused.
func TestLinuxBinaryUsesTheEmbeddedPins(t *testing.T) {
	reg, _, downloads := pinRegistry(t, "backdoored ELF")
	tool := &LinuxTool{Ver: "2.0.20", CacheDir: t.TempDir(), Registry: reg}
	_, err := tool.LinuxBinary(context.Background(), "arm64", "musl")
	require.ErrorContains(t, err, "not the one pinned")
	assert.Zero(t, *downloads)
}

func TestPinnedIntegrityTable(t *testing.T) {
	pins, err := PinnedIntegrity()
	require.NoError(t, err)
	require.Contains(t, pins, "2.0.20", "the version checked with this oh")
	for ver, targets := range pins {
		assert.NoError(t, CheckVersion("", ver), "%s is outside the supported opencode range: unpin it", ver)
		for _, target := range []string{"linux-x64", "linux-x64-musl", "linux-arm64", "linux-arm64-musl"} {
			integrity, ok := targets[target]
			assert.True(t, ok, "%s: %s is not pinned", ver, target)
			assert.NoError(t, checkIntegrity(integrity), "%s %s", ver, target)
		}
		assert.Len(t, targets, 4, ver)
	}
	// Every target npmTarget can give has a pin for each version.
	for _, a := range []string{"amd64", "arm64"} {
		for _, libc := range []string{"glibc", "musl"} {
			target, err := npmTarget(a, libc)
			require.NoError(t, err)
			for ver := range pins {
				_, ok := pins.lookup(ver, target)
				assert.True(t, ok, "%s %s", ver, target)
			}
		}
	}
}

func TestParsePinsRefusesMalformedTables(t *testing.T) {
	good := otherIntegrity()
	_, err := parsePins([]byte(`{"versions":{"2.0.20":{"linux-x64":"` + good + `"}}}`))
	require.NoError(t, err)
	for name, doc := range map[string]string{
		"not json":        `{`,
		"sha1":            `{"versions":{"2.0.20":{"linux-x64":"sha1-abc"}}}`,
		"not base64":      `{"versions":{"2.0.20":{"linux-x64":"sha512-***"}}}`,
		"short digest":    `{"versions":{"2.0.20":{"linux-x64":"sha512-AAAA"}}}`,
		"empty integrity": `{"versions":{"2.0.20":{"linux-x64":""}}}`,
	} {
		_, err := parsePins([]byte(doc))
		assert.Error(t, err, name)
	}
	pins, err := parsePins([]byte(`{}`))
	require.NoError(t, err)
	assert.Empty(t, pins)
}

// The strictness of the configuration reaches the tool of the images and the
// installer of the jobs.
func TestStrictPinsReachTheLinuxTool(t *testing.T) {
	a := New("", t.TempDir())
	a.Ver = "2.0.20"
	assert.Nil(t, a.ContainerTool().(*LinuxTool).Strict, "no configuration: a warning")
	a.StrictPins = func() bool { return true }
	tool := a.ContainerTool().(*LinuxTool)
	require.NotNil(t, tool.Strict)
	assert.True(t, tool.Strict())
}
