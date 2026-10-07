package container

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
)

// fakeStore simulates the image store of a docker engine.
type fakeStore struct {
	mu     sync.Mutex
	images map[string]string // ref → built label
	clock  int
	libc   string
	layer  string // last thin layer Dockerfile
}

var tagRe = regexp.MustCompile(` -t (\S+)`)

func newImageEngine(t *testing.T, st *fakeStore) *fakeRunner {
	t.Helper()
	r := newFakeRunner("docker").on("docker version", "27.0", nil).on("docker context show", "default", nil)
	r.onFunc("docker image inspect --format {{.Id}}", func(line string) (string, error) {
		st.mu.Lock()
		defer st.mu.Unlock()
		if _, ok := st.images[lastField(line)]; ok {
			return "sha256:x", nil
		}
		return "", errors.New("no such image")
	})
	r.onFunc("docker build", func(line string) (string, error) {
		st.mu.Lock()
		defer st.mu.Unlock()
		st.clock++
		ref := tagRe.FindStringSubmatch(line)[1]
		st.images[ref] = strings.Repeat("9", st.clock)
		if df, err := os.ReadFile(filepath.Join(lastField(line), "Dockerfile")); err == nil && strings.HasPrefix(ref, "oh-dev/") {
			st.layer = string(df)
		}
		return "Step 1/3\nSuccessfully built", nil
	})
	r.on("docker image inspect --format {{.Architecture}}", "aarch64", nil)
	r.onFunc("docker run --rm --entrypoint sh", func(string) (string, error) { return st.libc, nil })
	r.onFunc("docker images", func(line string) (string, error) {
		st.mu.Lock()
		defer st.mu.Unlock()
		role := "dev"
		if strings.Contains(line, "oh.role=base") {
			role = "base"
		}
		var refs []string
		for ref := range st.images {
			if strings.HasPrefix(ref, "oh-"+role+"/") {
				refs = append(refs, ref)
			}
		}
		sort.Strings(refs)
		return strings.Join(refs, "\n"), nil
	})
	r.onFunc(`docker image inspect --format {{index .Config.Labels "oh.built"}}`, func(line string) (string, error) {
		st.mu.Lock()
		defer st.mu.Unlock()
		return st.images[lastField(line)], nil
	})
	r.onFunc("docker rmi", func(line string) (string, error) {
		st.mu.Lock()
		defer st.mu.Unlock()
		delete(st.images, lastField(line))
		return "", nil
	})
	return r
}

func lastField(s string) string {
	f := strings.Fields(s)
	return f[len(f)-1]
}

type fakeTool struct {
	ver   string
	bin   string
	calls []string
}

func (f *fakeTool) Name() string    { return "faketool" }
func (f *fakeTool) Version() string { return f.ver }
func (f *fakeTool) LinuxBinary(_ context.Context, arch, libc string) (string, error) {
	f.calls = append(f.calls, arch+"/"+libc)
	return f.bin, nil
}

func newImageRuntime(t *testing.T, r *fakeRunner) *Runtime {
	rt := New(Options{Runner: r, GOOS: "linux", CacheDir: t.TempDir()})
	rt.bd = func(arch string) ([]byte, error) { return []byte("bd-" + arch), nil }
	return rt
}

func testGroup(t *testing.T, tool *fakeTool) ohruntime.Group {
	dir := t.TempDir()
	return ohruntime.Group{ProjectID: "My App", ProjectDir: dir, Tool: tool}
}

func TestEnsureImageBuildsThenCaches(t *testing.T) {
	st := &fakeStore{images: map[string]string{}, libc: "musl"}
	r := newImageEngine(t, st)
	bin := filepath.Join(t.TempDir(), "faketool")
	require.NoError(t, os.WriteFile(bin, []byte("ELF"), 0o755))
	tool := &fakeTool{ver: "2.0.20", bin: bin}
	g := testGroup(t, tool)
	require.NoError(t, os.WriteFile(filepath.Join(g.ProjectDir, "Dockerfile.dev"), []byte("FROM alpine:3.20\n"), 0o644))
	g.BuildArgs = map[string]string{"NODE": "22"}
	var progress []string
	g.Progress = func(l string) { progress = append(progress, l) }

	rt := newImageRuntime(t, r)
	img, err := rt.EnsureImage(context.Background(), g)
	require.NoError(t, err)
	assert.True(t, img.Built)
	assert.Regexp(t, `^oh-dev/my-app:[0-9a-f]{12}$`, img.Ref)
	assert.Regexp(t, `^oh-base/my-app:[0-9a-f]{12}$`, img.BaseRef)
	assert.Equal(t, "arm64", img.Arch)
	assert.Equal(t, "musl", img.Libc)
	assert.Equal(t, []string{"arm64/musl"}, tool.calls)
	assert.NotEmpty(t, progress)

	builds := r.called("docker build")
	require.Len(t, builds, 2)
	assert.Contains(t, builds[0], "-f "+filepath.Join(g.ProjectDir, "Dockerfile.dev"))
	assert.Contains(t, builds[0], "--build-arg NODE=22")
	assert.Contains(t, builds[0], "--label oh.role=base")
	assert.True(t, strings.HasSuffix(builds[0], " "+g.ProjectDir))
	assert.Contains(t, builds[1], "--label oh.tool=faketool@2.0.20")
	assert.Contains(t, st.layer, "FROM "+img.BaseRef+"\n")
	assert.Contains(t, st.layer, "COPY oh-tool /usr/local/bin/faketool")
	assert.Contains(t, st.layer, "COPY oh-bd /usr/local/bin/bd")

	again, err := rt.EnsureImage(context.Background(), g)
	require.NoError(t, err)
	assert.False(t, again.Built)
	assert.Equal(t, img.Ref, again.Ref)
	assert.Len(t, r.called("docker build"), 2, "cache hit: no rebuild")
	assert.Len(t, r.called("docker run --rm --entrypoint sh"), 1, "libc probe cached")
}

func TestEnsureImageTagChanges(t *testing.T) {
	st := &fakeStore{images: map[string]string{}}
	r := newImageEngine(t, st)
	tool := &fakeTool{ver: "2.0.20", bin: writeBin(t)}
	g := testGroup(t, tool)
	rt := newImageRuntime(t, r)

	a, err := rt.EnsureImage(context.Background(), g)
	require.NoError(t, err)
	assert.Empty(t, a.Dockerfile, "no project Dockerfile: oh default base")
	assert.Equal(t, "glibc", a.Libc)

	g.BuildArgs = map[string]string{"X": "1"}
	b, err := rt.EnsureImage(context.Background(), g)
	require.NoError(t, err)
	assert.NotEqual(t, a.BaseRef, b.BaseRef, "build args are part of the base hash")
	assert.NotEqual(t, a.Ref, b.Ref)

	tool.ver = "2.0.21"
	c, err := rt.EnsureImage(context.Background(), g)
	require.NoError(t, err)
	assert.Equal(t, b.BaseRef, c.BaseRef, "tool version only changes the oh layer")
	assert.NotEqual(t, b.Ref, c.Ref)
}

func TestEnsureImagePrunesOldImages(t *testing.T) {
	st := &fakeStore{images: map[string]string{}}
	r := newImageEngine(t, st)
	tool := &fakeTool{ver: "2.0.0", bin: writeBin(t)}
	g := testGroup(t, tool)
	rt := newImageRuntime(t, r)
	var refs []string
	for _, v := range []string{"2.0.1", "2.0.2", "2.0.3", "2.0.4"} {
		tool.ver = v
		img, err := rt.EnsureImage(context.Background(), g)
		require.NoError(t, err)
		refs = append(refs, img.Ref)
	}
	var dev []string
	for ref := range st.images {
		if strings.HasPrefix(ref, "oh-dev/") {
			dev = append(dev, ref)
		}
	}
	assert.ElementsMatch(t, refs[2:], dev, "the two most recent dev images are kept")
}

func TestEnsureImageErrors(t *testing.T) {
	rt := New(Options{Runner: newFakeRunner(), GOOS: "darwin", CacheDir: t.TempDir()})
	_, err := rt.EnsureImage(context.Background(), testGroup(t, &fakeTool{ver: "2"}))
	assert.ErrorIs(t, err, ErrUnavailable)

	st := &fakeStore{images: map[string]string{}}
	rt = newImageRuntime(t, newImageEngine(t, st))
	g := testGroup(t, &fakeTool{ver: "2"})
	g.Dockerfile = "missing/Dockerfile"
	_, err = rt.EnsureImage(context.Background(), g)
	assert.ErrorContains(t, err, "not found")
}

func TestDetectDockerfileOrder(t *testing.T) {
	dir := t.TempDir()
	p, err := DetectDockerfile(dir, "")
	require.NoError(t, err)
	assert.Empty(t, p)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM x"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".devcontainer"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".devcontainer", "Dockerfile"), []byte("FROM y"), 0o644))
	p, _ = DetectDockerfile(dir, "")
	assert.Equal(t, filepath.Join(dir, ".devcontainer", "Dockerfile"), p)
	p, _ = DetectDockerfile(dir, "Dockerfile")
	assert.Equal(t, filepath.Join(dir, "Dockerfile"), p)
}

func TestImageName(t *testing.T) {
	assert.Equal(t, "my-app", imageName("My App"))
	assert.Equal(t, "a.b_c", imageName("a.b_c"))
	assert.Equal(t, "project", imageName("--"))
	assert.True(t, sameRef("localhost/oh-dev/x:1", "oh-dev/x:1"))
}

func writeBin(t *testing.T) string {
	p := filepath.Join(t.TempDir(), "faketool")
	require.NoError(t, os.WriteFile(p, []byte("ELF"), 0o755))
	return p
}
