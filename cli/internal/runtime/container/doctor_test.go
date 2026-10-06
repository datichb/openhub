package container

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var dockerEngine = Engine{Kind: EngineDocker, CLI: "/usr/bin/docker", Host: hostDocker}

func TestProjectImagesLatestPerProject(t *testing.T) {
	r := newFakeRunner("docker").
		on("docker images", "oh-dev/a:1\noh-dev/a:2\noh-dev/b:1\noh-dev/a:2\nx:<none>", nil)
	labels := map[string]string{"oh-dev/a:1": "a|opencode@2.0.19|100", "oh-dev/a:2": "a|opencode@2.0.20|200", "oh-dev/b:1": "b|opencode@2.0.20|150"}
	r.onFunc("docker image inspect --format", func(line string) (string, error) { return labels[lastField(line)], nil })
	rt := New(Options{Runner: r, GOOS: "darwin", CacheDir: t.TempDir()})
	list, err := rt.ProjectImages(context.Background(), dockerEngine)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, ProjectImage{ProjectID: "a", Ref: "oh-dev/a:2", Tool: "opencode@2.0.20", Built: 200}, list[0])
	assert.Equal(t, "b", list[1].ProjectID)
}

func TestCheckTool(t *testing.T) {
	r := newFakeRunner("docker").
		on("docker run --rm --entrypoint opencode", "", errors.New("Error loading shared library libstdc++.so.6")).
		on("docker run --rm --entrypoint sh", "musl\nmissing:libstdc++\nmissing:libgcc\n", nil)
	rt := New(Options{Runner: r, GOOS: "darwin", CacheDir: t.TempDir()})
	c := rt.CheckTool(context.Background(), dockerEngine, "img", "opencode")
	assert.True(t, c.Musl)
	assert.Equal(t, []string{"libstdc++", "libgcc"}, c.MissingLibs)
	assert.Empty(t, c.Version)
	assert.Error(t, c.Err)

	r = newFakeRunner("docker").
		on("docker run --rm --entrypoint opencode", "opencode 2.0.20\n", nil).
		on("docker run --rm --entrypoint sh", "", nil)
	rt = New(Options{Runner: r, GOOS: "darwin", CacheDir: t.TempDir()})
	c = rt.CheckTool(context.Background(), dockerEngine, "img", "opencode")
	assert.Equal(t, "2.0.20", c.Version)
	assert.False(t, c.Musl)
	assert.Empty(t, c.MissingLibs)
}

func TestKeepID(t *testing.T) {
	uid := strconv.Itoa(os.Getuid())
	r := newFakeRunner("podman").on("podman run --rm", uid+"\n", nil)
	rt := New(Options{Runner: r, GOOS: "darwin", CacheDir: t.TempDir()})
	ok, err := rt.KeepID(context.Background(), Engine{Kind: EnginePodman, CLI: "/usr/bin/podman"}, ProbeImage)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Contains(t, r.called("podman run")[0], "--userns=keep-id:uid="+uid+",gid=")

	r = newFakeRunner("podman").on("podman run --rm", "1000\n", nil)
	rt = New(Options{Runner: r, GOOS: "darwin", CacheDir: t.TempDir()})
	ok, err = rt.KeepID(context.Background(), Engine{Kind: EnginePodman, CLI: "/usr/bin/podman"}, ProbeImage)
	require.NoError(t, err)
	assert.Equal(t, uid == "1000", ok)
}

func TestNotShared(t *testing.T) {
	e := Engine{VM: true, Shared: []string{"/Users/me"}}
	assert.Equal(t, []string{"/opt/src/app"}, NotShared(e, []string{"/Users/me/src/app", "/opt/src/app", ""}))
	assert.Empty(t, NotShared(Engine{}, []string{"/opt/x"}), "no VM: everything is shared")
}

func TestProbeHTTP(t *testing.T) {
	urls := []string{"http://host.docker.internal:5556/oh-gateway/beads/v1/exec", "http://host.docker.internal:5556/oh/v1/hooks/mcp/team"}
	r := newFakeRunner("docker").
		on("docker run --rm --entrypoint sh oh-dev/a:1", "", errors.New("exit status 127")).
		onFunc("docker run --rm --entrypoint sh "+ProbeImage, func(line string) (string, error) {
			assert.NotContains(t, line, "--add-host", "macOS: no extra host")
			return urls[0] + " 405\n" + urls[1] + " 401\n", nil
		})
	rt := New(Options{Runner: r, GOOS: "darwin", CacheDir: t.TempDir()})
	codes, img, err := rt.ProbeHTTP(context.Background(), dockerEngine, []string{"oh-dev/a:1", ProbeImage}, urls)
	require.NoError(t, err)
	assert.Equal(t, ProbeImage, img, "first image without HTTP client skipped")
	assert.Equal(t, map[string]int{urls[0]: 405, urls[1]: 401}, codes)

	r = newFakeRunner("docker").onFunc("docker run --rm --entrypoint sh", func(line string) (string, error) {
		assert.True(t, strings.Contains(line, "--add-host host.docker.internal:host-gateway"), "docker on Linux")
		return urls[0] + " 0\n", nil
	})
	rt = New(Options{Runner: r, GOOS: "linux", CacheDir: t.TempDir()})
	codes, _, err = rt.ProbeHTTP(context.Background(), dockerEngine, []string{ProbeImage}, urls[:1])
	require.NoError(t, err)
	assert.Equal(t, 0, codes[urls[0]])

	_, _, err = rt.ProbeHTTP(context.Background(), dockerEngine, nil, urls)
	assert.Error(t, err)
}

func TestURLHost(t *testing.T) {
	assert.Equal(t, "http://host.docker.internal:5556", URLHost("http://127.0.0.1:5556", "host.docker.internal"))
	assert.Equal(t, "http://172.17.0.1:5556/x", URLHost("http://127.0.0.1:5556/x", "172.17.0.1"))
	assert.Equal(t, "https://a", URLHost("https://a", "h"))
}
