package container

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const colimaRunning = `{"runtime":"docker","mount_type":"virtiofs","arch":"aarch64"}`

// colimaHome creates a Colima directory with the docker socket of a profile.
func colimaHome(t *testing.T, profile string) (home, sock string) {
	t.Helper()
	home = t.TempDir()
	sock = filepath.Join(home, ".colima", profile, "docker.sock")
	require.NoError(t, os.MkdirAll(filepath.Dir(sock), 0o700))
	require.NoError(t, os.WriteFile(sock, nil, 0o600))
	return home, sock
}

// Detect on a scripted runner: what is installed and what each command
// answers decide the engine, or the reason why there is none.
func TestDetect(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name      string
		installed []string
		script    func(r *fakeRunner, sock string)
		goos      string // darwin when empty
		engine    EngineKind
		profile   string
		socketOf  string // Colima profile whose docker socket exists in the home ({sock} in wantArgs)

		wantReason    string // empty: the engine is available
		wantAvArgs    []any
		wantMessage   []string
		wantExplained bool // the reason has a message
		wantKind      EngineKind
		wantArgs      []string
		wantVersion   string
		wantHost      string
		wantDetails   string
		wantCalls     map[string]int
		wantNo        string // no command starts with it
		check         func(t *testing.T, e Engine)
	}{
		{
			name:      "colima running through its docker context",
			installed: []string{"colima", "docker"},
			script: func(r *fakeRunner, _ string) {
				r.on("colima status --profile default --json", colimaRunning, nil).
					on("docker --context colima version", "28.4.0\n", nil)
			},
			engine: EngineAuto, wantKind: EngineColima, wantArgs: []string{"--context", "colima"},
			wantVersion: "28.4.0", wantHost: hostDocker, wantDetails: "mount=virtiofs",
			check: func(t *testing.T, e Engine) {
				assert.Equal(t, []string{"/usr/bin/docker", "--context", "colima", "ps"}, e.Command("ps"))
			},
		},
		{
			name:      "colima profile gives its own context",
			installed: []string{"colima", "docker"},
			script: func(r *fakeRunner, _ string) {
				r.on("colima status --profile work --json", colimaRunning, nil).
					on("docker --context colima-work version", "28.4.0", nil)
			},
			engine: EngineColima, profile: "work", wantKind: EngineColima, wantArgs: []string{"--context", "colima-work"},
		},
		{
			name:      "colima stopped",
			installed: []string{"colima", "docker"},
			script:    func(r *fakeRunner, _ string) { r.on("colima status", "", errors.New("colima is not running")) },
			engine:    EngineColima, wantReason: reasonVMStopped, wantAvArgs: []any{"Colima", "colima start"},
		},
		{
			name:      "colima on containerd",
			installed: []string{"colima", "docker"},
			script:    func(r *fakeRunner, _ string) { r.on("colima status", `{"runtime":"containerd"}`, nil) },
			engine:    EngineColima, wantReason: reasonColimaRuntime,
		},
		{
			name:      "colima without the docker CLI",
			installed: []string{"colima"},
			script:    func(r *fakeRunner, _ string) { r.on("colima status", colimaRunning, nil) },
			engine:    EngineColima, wantReason: reasonCLIMissing,
		},
		{
			name:      "colima status with warnings before the JSON",
			installed: []string{"colima", "docker"},
			script: func(r *fakeRunner, _ string) {
				r.on("colima status", "WARN[0000] newer version available\n"+colimaRunning+"\n", nil).
					on("docker --context colima version", "28.4.0", nil)
			},
			engine: EngineColima, wantKind: EngineColima, wantArgs: []string{"--context", "colima"},
		},
		{
			name:      "colima status unreadable",
			installed: []string{"colima", "docker"},
			script:    func(r *fakeRunner, _ string) { r.on("colima status", "something odd", nil) },
			engine:    EngineColima, wantReason: reasonUnreachable, wantMessage: []string{"unexpected output"},
		},
		{
			name:      "colima status fails with another error",
			installed: []string{"colima", "docker"},
			script: func(r *fakeRunner, _ string) {
				r.on("colima status", "", errors.New("exit status 1: permission denied"))
			},
			engine: EngineColima, wantReason: reasonUnreachable,
		},
		{
			name:      "colima status fails but its socket works",
			installed: []string{"colima", "docker"},
			socketOf:  "default",
			script: func(r *fakeRunner, sock string) {
				r.on("colima status", "", errors.New("exit status 1: limactl not found")).
					on("docker --host unix://"+sock+" version", "28.4.0", nil)
			},
			engine: EngineColima, wantKind: EngineColima, wantArgs: []string{"--host", "unix://{sock}"}, wantNo: "docker --context",
		},
		{
			name:      "colima context missing but its socket works",
			installed: []string{"colima", "docker"},
			socketOf:  "work",
			profile:   "work",
			script: func(r *fakeRunner, sock string) {
				r.on("colima status --profile work --json", colimaRunning, nil).
					on("docker --context colima-work version", "", errors.New(`context "colima-work": context not found`)).
					on("docker --host unix://"+sock+" version", "28.4.0", nil)
			},
			engine: EngineColima, wantKind: EngineColima, wantArgs: []string{"--host", "unix://{sock}"}, wantDetails: "mount=virtiofs",
		},
		{
			name:      "colima socket named by its status",
			installed: []string{"colima", "docker"},
			script: func(r *fakeRunner, _ string) {
				r.on("colima status", `{"runtime":"docker","docker_socket":"unix:///run/colima.sock"}`, nil).
					on("docker --context colima version", "", errors.New("context not found")).
					on("docker --host unix:///run/colima.sock version", "28.4.0", nil)
			},
			engine: EngineColima, wantKind: EngineColima, wantArgs: []string{"--host", "unix:///run/colima.sock"},
		},
		{
			name:      "podman machine",
			installed: []string{"podman"},
			script: func(r *fakeRunner, _ string) {
				r.on("podman version", "5.6.1", nil).on("podman info", "true", nil)
			},
			engine: EngineAuto, wantKind: EnginePodman, wantHost: hostPodman,
			check: func(t *testing.T, e Engine) {
				assert.True(t, e.Rootless, "rootless")
				assert.True(t, e.VM, "in a VM")
			},
		},
		{
			name:      "podman machine stopped",
			installed: []string{"podman"},
			script: func(r *fakeRunner, _ string) {
				r.on("podman version", "", errors.New("cannot connect")).on("podman machine inspect", "stopped", nil)
			},
			engine: EnginePodman, wantReason: reasonVMStopped, wantAvArgs: []any{"Podman", "podman machine start"},
		},
		{
			name:      "podman unreachable on linux",
			installed: []string{"podman"},
			script:    func(r *fakeRunner, _ string) { r.on("podman version", "", errors.New("boom")) },
			goos:      "linux", engine: EnginePodman, wantReason: reasonUnreachable,
		},
		{
			name:      "docker on linux",
			installed: []string{"docker"},
			script: func(r *fakeRunner, _ string) {
				r.on("docker version", "27.0.1", nil).on("docker context show", "default", nil)
			},
			goos: "linux", engine: EngineAuto, wantKind: EngineDocker,
			check: func(t *testing.T, e Engine) { assert.False(t, e.VM, "docker runs on the host") },
		},
		{
			name:      "docker version is retried once",
			installed: []string{"docker"},
			script: func(r *fakeRunner, _ string) {
				n := 0
				r.onFunc("docker version", func(string) (string, error) {
					n++
					if n == 1 {
						return "", errors.New("timeout")
					}
					return "27.0", nil
				}).on("docker context show", "default", nil)
			},
			goos: "linux", engine: EngineDocker, wantKind: EngineDocker, wantVersion: "27.0", wantCalls: map[string]int{"docker version": 2},
		},
		{
			name:      "auto falls back to podman when colima is stopped",
			installed: []string{"colima", "docker", "podman"},
			script: func(r *fakeRunner, _ string) {
				r.on("colima status", "", errors.New("not running")).
					on("podman version", "5.0", nil).on("podman info", "false", nil)
			},
			engine: EngineAuto, wantKind: EnginePodman,
		},
		{
			name:      "auto with nothing reachable: every installed engine explains why",
			installed: []string{"colima", "docker"},
			script: func(r *fakeRunner, _ string) {
				r.on("colima status", "", errors.New("not running")).on("docker version", "", errors.New("no daemon"))
			},
			engine: EngineAuto, wantReason: reasonAllFailed, wantMessage: []string{"colima start", "no daemon"},
		},
		{
			name:      "auto with a single installed engine keeps its own reason",
			installed: []string{"colima"},
			script:    func(r *fakeRunner, _ string) { r.on("colima status", "", errors.New("colima is not running")) },
			engine:    EngineAuto, wantReason: reasonVMStopped,
		},
		{
			name:   "no engine installed",
			engine: EngineAuto, wantReason: reasonNoEngine, wantExplained: true,
		},
		{
			name:      "windows is not supported",
			installed: []string{"docker"},
			goos:      "windows", engine: EngineAuto, wantReason: reasonUnsupportedOS,
		},
		{
			name:      "the forced engine is not installed",
			installed: []string{"docker"},
			engine:    EnginePodman, wantReason: reasonNotInstalled,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newFakeRunner(tc.installed...)
			d := Detector{Runner: r, GOOS: tc.goos, ColimaProfile: tc.profile}
			if d.GOOS == "" {
				d.GOOS = "darwin"
			}
			var sock string
			if tc.socketOf != "" {
				d.Home, sock = colimaHome(t, tc.socketOf)
			}
			if tc.script != nil {
				tc.script(r, sock)
			}

			e, av := d.Detect(ctx, tc.engine)

			require.Equal(t, tc.wantReason == "", av.OK, "available: %s", av.Message())
			if tc.wantReason != "" {
				assert.Equal(t, tc.wantReason, av.Reason, "reason: %s", av.Message())
			}
			if tc.wantAvArgs != nil {
				assert.Equal(t, tc.wantAvArgs, av.Args, "arguments of the reason")
			}
			if tc.wantExplained {
				assert.NotEmpty(t, av.Message(), "the reason is explained")
			}
			for _, part := range tc.wantMessage {
				assert.Contains(t, av.Message(), part, "message")
			}
			if tc.wantReason != "" {
				return
			}
			assert.Equal(t, tc.wantKind, e.Kind, "engine")
			if wantArgs := strings.ReplaceAll(strings.Join(tc.wantArgs, "\x00"), "{sock}", sock); wantArgs != "" {
				assert.Equal(t, strings.Split(wantArgs, "\x00"), e.Args, "arguments before the engine commands")
			} else {
				assert.Empty(t, e.Args, "plain engine command")
			}
			if tc.wantVersion != "" {
				assert.Equal(t, tc.wantVersion, e.Version, "version")
			}
			if tc.wantHost != "" {
				assert.Equal(t, tc.wantHost, e.Host, "host address")
			}
			if tc.wantDetails != "" {
				assert.Contains(t, e.Details, tc.wantDetails, "details")
			}
			for line, n := range tc.wantCalls {
				assert.Len(t, r.called(line), n, "calls of %q", line)
			}
			if tc.wantNo != "" {
				assert.Empty(t, r.called(tc.wantNo), "no %q call", tc.wantNo)
			}
			if tc.check != nil {
				tc.check(t, e)
			}
		})
	}
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
