package container

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const colimaRunning = `{"runtime":"docker","mount_type":"virtiofs","arch":"aarch64"}`

func TestDetectColima(t *testing.T) {
	r := newFakeRunner("colima", "docker").
		on("colima status --profile default --json", colimaRunning, nil).
		on("docker --context colima version", "28.4.0\n", nil)
	e, av := Detector{Runner: r, GOOS: "darwin"}.Detect(context.Background(), EngineAuto)
	require.True(t, av.OK, av.Reason)
	assert.Equal(t, EngineColima, e.Kind)
	assert.Equal(t, []string{"/usr/bin/docker", "--context", "colima", "ps"}, e.Command("ps"))
	assert.Equal(t, "28.4.0", e.Version)
	assert.Equal(t, hostDocker, e.Host)
	assert.Contains(t, e.Details, "mount=virtiofs")
}

func TestDetectColimaProfile(t *testing.T) {
	r := newFakeRunner("colima", "docker").
		on("colima status --profile work --json", colimaRunning, nil).
		on("docker --context colima-work version", "28.4.0", nil)
	e, av := Detector{Runner: r, GOOS: "darwin", ColimaProfile: "work"}.Detect(context.Background(), EngineColima)
	require.True(t, av.OK)
	assert.Equal(t, []string{"--context", "colima-work"}, e.Args)
}

func TestDetectColimaStopped(t *testing.T) {
	r := newFakeRunner("colima", "docker").
		on("colima status", "", errors.New("colima is not running"))
	_, av := Detector{Runner: r, GOOS: "darwin"}.Detect(context.Background(), EngineColima)
	assert.False(t, av.OK)
	assert.Equal(t, reasonVMStopped, av.Reason)
	assert.Equal(t, []any{"Colima", "colima start"}, av.Args)
}

func TestDetectColimaContainerd(t *testing.T) {
	r := newFakeRunner("colima", "docker").
		on("colima status", `{"runtime":"containerd"}`, nil)
	_, av := Detector{Runner: r, GOOS: "darwin"}.Detect(context.Background(), EngineColima)
	assert.Equal(t, reasonColimaRuntime, av.Reason)
}

func TestDetectColimaWithoutDockerCLI(t *testing.T) {
	r := newFakeRunner("colima").on("colima status", colimaRunning, nil)
	_, av := Detector{Runner: r, GOOS: "darwin"}.Detect(context.Background(), EngineColima)
	assert.Equal(t, reasonCLIMissing, av.Reason)
}

func TestDetectPodman(t *testing.T) {
	r := newFakeRunner("podman").
		on("podman version", "5.6.1", nil).
		on("podman info", "true", nil)
	e, av := Detector{Runner: r, GOOS: "darwin"}.Detect(context.Background(), EngineAuto)
	require.True(t, av.OK)
	assert.Equal(t, EnginePodman, e.Kind)
	assert.True(t, e.Rootless)
	assert.True(t, e.VM)
	assert.Equal(t, hostPodman, e.Host)
}

func TestDetectPodmanMachineStopped(t *testing.T) {
	r := newFakeRunner("podman").
		on("podman version", "", errors.New("cannot connect")).
		on("podman machine inspect", "stopped", nil)
	_, av := Detector{Runner: r, GOOS: "darwin"}.Detect(context.Background(), EnginePodman)
	assert.Equal(t, reasonVMStopped, av.Reason)
	assert.Equal(t, []any{"Podman", "podman machine start"}, av.Args)
}

func TestDetectPodmanLinuxUnreachable(t *testing.T) {
	r := newFakeRunner("podman").on("podman version", "", errors.New("boom"))
	_, av := Detector{Runner: r, GOOS: "linux"}.Detect(context.Background(), EnginePodman)
	assert.Equal(t, reasonUnreachable, av.Reason)
}

func TestDetectDocker(t *testing.T) {
	r := newFakeRunner("docker").
		on("docker version", "27.0.1", nil).
		on("docker context show", "default", nil)
	e, av := Detector{Runner: r, GOOS: "linux"}.Detect(context.Background(), EngineAuto)
	require.True(t, av.OK)
	assert.Equal(t, EngineDocker, e.Kind)
	assert.Empty(t, e.Args)
	assert.False(t, e.VM)
}

func TestDetectAutoFallsBackAndReportsInstalled(t *testing.T) {
	// Colima installed but stopped, Podman reachable: Podman wins.
	r := newFakeRunner("colima", "docker", "podman").
		on("colima status", "", errors.New("not running")).
		on("podman version", "5.0", nil).
		on("podman info", "false", nil)
	e, av := Detector{Runner: r, GOOS: "darwin"}.Detect(context.Background(), EngineAuto)
	require.True(t, av.OK)
	assert.Equal(t, EnginePodman, e.Kind)

	// Nothing reachable: every installed engine explains why.
	r = newFakeRunner("colima", "docker").
		on("colima status", "", errors.New("not running")).
		on("docker version", "", errors.New("no daemon"))
	_, av = Detector{Runner: r, GOOS: "darwin"}.Detect(context.Background(), EngineAuto)
	assert.False(t, av.OK)
	assert.Equal(t, reasonAllFailed, av.Reason)
	msg := av.Message()
	assert.Contains(t, msg, "colima start")
	assert.Contains(t, msg, "no daemon")

	// A single installed engine keeps its own reason.
	r = newFakeRunner("colima").on("colima status", "", errors.New("colima is not running"))
	_, av = Detector{Runner: r, GOOS: "darwin"}.Detect(context.Background(), EngineAuto)
	assert.Equal(t, reasonVMStopped, av.Reason)
}

// colimaHome creates a Colima directory with the docker socket of a profile.
func colimaHome(t *testing.T, profile string) (home, sock string) {
	t.Helper()
	home = t.TempDir()
	sock = filepath.Join(home, ".colima", profile, "docker.sock")
	require.NoError(t, os.MkdirAll(filepath.Dir(sock), 0o700))
	require.NoError(t, os.WriteFile(sock, nil, 0o600))
	return home, sock
}

func TestDetectColimaStatusFailsSocketWorks(t *testing.T) {
	home, sock := colimaHome(t, "default")
	r := newFakeRunner("colima", "docker").
		on("colima status", "", errors.New("exit status 1: limactl not found")).
		on("docker --host unix://"+sock+" version", "28.4.0", nil)
	e, av := Detector{Runner: r, GOOS: "darwin", Home: home}.Detect(context.Background(), EngineColima)
	require.True(t, av.OK, av.Message())
	assert.Equal(t, []string{"--host", "unix://" + sock}, e.Args)
	assert.Empty(t, r.called("docker --context"))
}

func TestDetectColimaContextMissingSocketWorks(t *testing.T) {
	home, sock := colimaHome(t, "work")
	r := newFakeRunner("colima", "docker").
		on("colima status --profile work --json", colimaRunning, nil).
		on("docker --context colima-work version", "", errors.New(`context "colima-work": context not found`)).
		on("docker --host unix://"+sock+" version", "28.4.0", nil)
	e, av := Detector{Runner: r, GOOS: "darwin", Home: home, ColimaProfile: "work"}.Detect(context.Background(), EngineColima)
	require.True(t, av.OK, av.Message())
	assert.Equal(t, []string{"--host", "unix://" + sock}, e.Args)
	assert.Contains(t, e.Details, "mount=virtiofs")
}

func TestDetectColimaStatusSocket(t *testing.T) {
	r := newFakeRunner("colima", "docker").
		on("colima status", `{"runtime":"docker","docker_socket":"unix:///run/colima.sock"}`, nil).
		on("docker --context colima version", "", errors.New("context not found")).
		on("docker --host unix:///run/colima.sock version", "28.4.0", nil)
	e, av := Detector{Runner: r, GOOS: "darwin"}.Detect(context.Background(), EngineColima)
	require.True(t, av.OK, av.Message())
	assert.Equal(t, []string{"--host", "unix:///run/colima.sock"}, e.Args)
}

func TestDetectColimaStatusUnreadable(t *testing.T) {
	r := newFakeRunner("colima", "docker").on("colima status", "something odd", nil)
	_, av := Detector{Runner: r, GOOS: "darwin"}.Detect(context.Background(), EngineColima)
	assert.Equal(t, reasonUnreachable, av.Reason)
	assert.Contains(t, av.Message(), "unexpected output")

	r = newFakeRunner("colima", "docker").on("colima status", "", errors.New("exit status 1: permission denied"))
	_, av = Detector{Runner: r, GOOS: "darwin"}.Detect(context.Background(), EngineColima)
	assert.Equal(t, reasonUnreachable, av.Reason)
}

func TestDetectColimaStatusWithWarnings(t *testing.T) {
	r := newFakeRunner("colima", "docker").
		on("colima status", "WARN[0000] newer version available\n"+colimaRunning+"\n", nil).
		on("docker --context colima version", "28.4.0", nil)
	_, av := Detector{Runner: r, GOOS: "darwin"}.Detect(context.Background(), EngineColima)
	assert.True(t, av.OK, av.Message())
}

func TestDetectRetriesVersion(t *testing.T) {
	n := 0
	r := newFakeRunner("docker").
		onFunc("docker version", func(string) (string, error) {
			n++
			if n == 1 {
				return "", errors.New("timeout")
			}
			return "27.0", nil
		}).
		on("docker context show", "default", nil)
	e, av := Detector{Runner: r, GOOS: "linux"}.Detect(context.Background(), EngineDocker)
	require.True(t, av.OK, av.Message())
	assert.Equal(t, "27.0", e.Version)
	assert.Equal(t, 2, n)
}

func TestDetectNoEngine(t *testing.T) {
	_, av := Detector{Runner: newFakeRunner(), GOOS: "darwin"}.Detect(context.Background(), EngineAuto)
	assert.Equal(t, reasonNoEngine, av.Reason)
	assert.NotEmpty(t, av.Message())
}

func TestDetectWindows(t *testing.T) {
	_, av := Detector{Runner: newFakeRunner("docker"), GOOS: "windows"}.Detect(context.Background(), EngineAuto)
	assert.Equal(t, reasonUnsupportedOS, av.Reason)
}

func TestDetectForcedEngineNotInstalled(t *testing.T) {
	_, av := Detector{Runner: newFakeRunner("docker"), GOOS: "darwin"}.Detect(context.Background(), EnginePodman)
	assert.Equal(t, reasonNotInstalled, av.Reason)
}

func TestParseEngine(t *testing.T) {
	for in, want := range map[string]EngineKind{"": EngineAuto, "Auto": EngineAuto, "colima": EngineColima, " podman ": EnginePodman, "docker": EngineDocker} {
		got, ok := ParseEngine(in)
		assert.True(t, ok, in)
		assert.Equal(t, want, got, in)
	}
	_, ok := ParseEngine("nerdctl")
	assert.False(t, ok)
}

func TestRuntimeAvailableCachesEngine(t *testing.T) {
	r := newFakeRunner("docker").on("docker version", "27.0", nil).on("docker context show", "default", nil)
	rt := New(Options{Runner: r, GOOS: "linux", Engine: EngineDocker})
	assert.Empty(t, rt.HostAddress())
	av, err := rt.Available(context.Background())
	require.NoError(t, err)
	require.True(t, av.OK)
	assert.Equal(t, hostDocker, rt.HostAddress())
	e, _ := rt.Engine(context.Background())
	assert.Equal(t, EngineDocker, e.Kind)
}

func TestRuntimeAvailableReusesEngine(t *testing.T) {
	r := newFakeRunner("docker").on("docker version", "27.0", nil).on("docker context show", "default", nil)
	rt := New(Options{Runner: r, GOOS: "linux", Engine: EngineDocker})
	for range 3 {
		av, err := rt.Available(context.Background())
		require.NoError(t, err)
		require.True(t, av.OK)
	}
	assert.Len(t, r.called("docker version"), 3)
	assert.Len(t, r.called("docker context show"), 1)
}

func TestExecRunnerLookPathFallback(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "oh-fake-engine")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\necho ok\n"), 0o755))
	r := ExecRunner{Dirs: []string{filepath.Join(dir, "missing"), dir}}
	p, err := r.LookPath("oh-fake-engine")
	require.NoError(t, err)
	assert.Equal(t, bin, p)
	_, err = ExecRunner{Dirs: []string{}}.LookPath("oh-fake-engine")
	assert.Error(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "plain"), nil, 0o644))
	_, err = r.LookPath("plain")
	assert.Error(t, err)
}

func TestWithDirs(t *testing.T) {
	dir := t.TempDir()
	sep := string(os.PathListSeparator)
	assert.Equal(t, "/usr/bin"+sep+dir, withDirs("/usr/bin", []string{dir, filepath.Join(dir, "missing")}))
	assert.Equal(t, dir+sep+"/usr/bin", withDirs(dir+sep+"/usr/bin", []string{dir}))
	assert.Equal(t, dir, withDirs("", []string{dir}))
}
