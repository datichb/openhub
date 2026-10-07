package container

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// layout creates a project repository with a sibling linked worktree.
func layout(t *testing.T) (root, project, worktree, bundle, data string) {
	t.Helper()
	root = clean(t.TempDir())
	project = filepath.Join(root, "src", "app")
	worktree = filepath.Join(root, "src", "app-feat")
	bundle = filepath.Join(root, ".oh", "bundles", "abc")
	data = filepath.Join(root, ".oh", "servers", "g1", "data")
	gitdir := filepath.Join(project, ".git", "worktrees", "app-feat")
	for _, d := range []string{gitdir, worktree, bundle, data} {
		require.NoError(t, os.MkdirAll(d, 0o755))
	}
	require.NoError(t, os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: "+gitdir+"\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(gitdir, "commondir"), []byte("../..\n"), 0o644))
	return root, project, worktree, bundle, data
}

func TestBuildSpecLayout(t *testing.T) {
	_, project, worktree, bundle, data := layout(t)
	g := ohruntime.Group{ProjectID: "App", ProjectDir: project, Locations: []string{project, worktree},
		BundleDir: bundle, DataDir: data, Volumes: []string{"node_modules", "/opt/cache"}}
	e := Engine{Kind: EngineColima, Host: hostDocker, VM: true}
	s, err := buildSpec(e, specInput{GOOS: "darwin", UID: 501, GID: 20}, g, "oh-dev/app:1", "app-123-local")
	require.NoError(t, err)

	assert.Equal(t, "oh-app-123-local", s.Name)
	in := func(p string) string {
		got, ok := s.Paths.ToInner(p)
		require.True(t, ok, p)
		return got
	}
	assert.Equal(t, InnerBundle+"/skills", in(filepath.Join(bundle, "skills")))
	assert.Equal(t, InnerData, in(data))
	assert.Equal(t, "/work/app/main.go", in(filepath.Join(project, "main.go")))
	assert.Equal(t, "/work/app-feat", in(worktree))
	common := filepath.Join(project, ".git")
	assert.Equal(t, common, in(common), "git common dir visible at its machine path")
	assert.ElementsMatch(t, []string{project, worktree}, s.Locations)

	var ro, volumes []string
	for _, m := range s.Mounts {
		if m.ReadOnly {
			ro = append(ro, m.Target)
		}
		if m.Volume {
			volumes = append(volumes, m.Target)
		}
	}
	assert.Equal(t, []string{InnerBundle}, ro)
	assert.ElementsMatch(t, []string{InnerHome, "/opt/cache", "/work/app/node_modules", "/work/app-feat/node_modules"}, volumes)

	assert.Equal(t, InnerHome, s.Env["HOME"])
	assert.Equal(t, "safe.directory", s.Env["GIT_CONFIG_KEY_0"])
	assert.Equal(t, "never", s.Env["GIT_CONFIG_VALUE_1"])
	assert.Equal(t, []string{"--user", "501:20"}, s.UserArgs)
	assert.Empty(t, s.AddHosts)
}

func TestBuildSpecUserAndHosts(t *testing.T) {
	_, project, _, _, _ := layout(t)
	g := ohruntime.Group{ProjectID: "p", ProjectDir: project}
	s, err := buildSpec(Engine{Kind: EnginePodman, Rootless: true, Host: hostPodman}, specInput{GOOS: "linux", UID: 1000, GID: 1000}, g, "i", "k")
	require.NoError(t, err)
	assert.Equal(t, []string{"--userns=keep-id:uid=1000,gid=1000", "--user", "1000:1000"}, s.UserArgs)

	s, err = buildSpec(Engine{Kind: EngineDocker, Host: hostDocker}, specInput{GOOS: "linux", UID: 1000, GID: 1000}, g, "i", "k")
	require.NoError(t, err)
	assert.Equal(t, []string{"host.docker.internal:host-gateway"}, s.AddHosts)
}

func TestBuildSpecRefusesUnsharedPaths(t *testing.T) {
	_, project, _, _, _ := layout(t)
	e := Engine{Kind: EngineColima, VM: true, Shared: []string{"/Users/someone-else"}}
	_, err := buildSpec(e, specInput{}, ohruntime.Group{ProjectID: "p", ProjectDir: project}, "i", "k")
	assert.ErrorContains(t, err, "not shared with the colima VM")
}

func TestBuildSpecNameCollision(t *testing.T) {
	root := clean(t.TempDir())
	a, b := filepath.Join(root, "x", "app"), filepath.Join(root, "y", "app")
	require.NoError(t, os.MkdirAll(a, 0o755))
	require.NoError(t, os.MkdirAll(b, 0o755))
	s, err := buildSpec(Engine{Kind: EngineDocker}, specInput{GOOS: "darwin"}, ohruntime.Group{ProjectID: "p", Locations: []string{a, b}}, "i", "k")
	require.NoError(t, err)
	ia, _ := s.Paths.ToInner(a)
	ib, _ := s.Paths.ToInner(b)
	assert.Equal(t, "/work/app", ia)
	assert.Equal(t, "/work/app-2", ib)
}

func TestRunArgsKeepsValuesOffTheCommandLine(t *testing.T) {
	s := Spec{Name: "oh-g", Image: "oh-dev/p:1", UserArgs: []string{"--user", "1:2"},
		AddHosts: []string{"host.docker.internal:host-gateway"},
		Mounts:   []Mount{{Source: "/b", Target: InnerBundle, ReadOnly: true}, {Source: "vol", Target: InnerHome, Volume: true}},
		Env:      map[string]string{"HOME": InnerHome}}
	args, env, err := runArgs(s, ohruntime.Proc{Argv: []string{"faketool", "serve"}, Dir: "/work/p", Ports: []int{4096},
		Env: map[string]string{"TOOL_SERVER_PASSWORD": "secret-pw"}}, "/g/container.env")
	require.NoError(t, err)
	line := strings.Join(args, " ")
	assert.NotContains(t, line, "secret-pw")
	assert.Contains(t, line, "run --rm --init --name oh-g")
	assert.Contains(t, line, "--mount type=bind,source=/b,target=/opt/oh/bundle,readonly")
	assert.Contains(t, line, "--mount type=volume,source=vol,target=/opt/oh/home")
	assert.Contains(t, line, "-p 127.0.0.1:4096:4096")
	assert.Contains(t, line, "--add-host host.docker.internal:host-gateway")
	assert.Contains(t, line, "--env-file /g/container.env")
	assert.True(t, strings.HasSuffix(line, "-w /work/p oh-dev/p:1 faketool serve"))
	assert.Equal(t, "HOME=/opt/oh/home\nTOOL_SERVER_PASSWORD=secret-pw\n", env)

	_, _, err = runArgs(s, ohruntime.Proc{Env: map[string]string{"X": "a\nb"}}, "/f")
	assert.Error(t, err)
}

func TestPrepareAndCommand(t *testing.T) {
	_, project, worktree, bundle, data := layout(t)
	st := &fakeStore{images: map[string]string{}}
	r := newImageEngine(t, st)
	r.on("docker rm -f", "", nil)
	rt := newImageRuntime(t, r)
	g := ohruntime.Group{Key: sessionspec.GroupKey{BundleHash: "abc", ProjectID: "app", Runtime: sessionspec.RuntimeContainer},
		ProjectID: "app", ProjectDir: project, Locations: []string{worktree}, BundleDir: bundle, DataDir: data,
		Tool: &fakeTool{ver: "2.0.20", bin: writeBin(t)}}

	// Linux: the proxy must also listen on the address containers resolve.
	r.on("docker run --rm --entrypoint sh --add-host host.docker.internal:host-gateway", "172.17.0.1      host.docker.internal\n", nil)
	pg, err := rt.Prepare(context.Background(), g)
	require.NoError(t, err)
	assert.Equal(t, "172.17.0.1", pg.ListenHost)
	assert.Equal(t, hostDocker, pg.HostAddress)
	assert.True(t, pg.Paths.Covers(worktree))

	saved, err := LoadSpec(data)
	require.NoError(t, err)
	assert.Equal(t, pg.Spec.(*Spec).Name, saved.Name)

	t.Setenv("AWS_SECRET_ACCESS_KEY", "leak")
	cmd, err := rt.Command(context.Background(), pg, ohruntime.Proc{Argv: []string{"faketool", "serve"}, Env: map[string]string{"K": "v"}})
	require.NoError(t, err)
	assert.Equal(t, "/usr/bin/docker", cmd.Path)
	for _, kv := range cmd.Env {
		assert.False(t, strings.HasPrefix(kv, "AWS_") || kv == "K=v", kv)
	}
	envFile, err := os.ReadFile(filepath.Join(filepath.Dir(data), "container.env"))
	require.NoError(t, err)
	assert.Contains(t, string(envFile), "K=v\n")
	fi, _ := os.Stat(filepath.Join(filepath.Dir(data), "container.env"))
	assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm())
	assert.NotEmpty(t, r.called("docker rm -f oh-"), "stale container removed")
	require.NoError(t, rt.Teardown(context.Background(), pg))
}
