package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

type fakePurgeDaemon struct {
	running     bool
	shutdowns   int
	forced      bool
	shutdownErr error
	staysUp     bool
}

func (f *fakePurgeDaemon) Health(context.Context) (daemon.Health, error) {
	if !f.running {
		return daemon.Health{}, daemon.ErrNotRunning
	}
	return daemon.Health{}, nil
}

func (f *fakePurgeDaemon) Shutdown(_ context.Context, force bool) error {
	f.shutdowns++
	f.forced = force
	if f.shutdownErr != nil {
		return f.shutdownErr
	}
	f.running = f.staysUp
	return nil
}

type fakePurgeServers struct {
	list []domain.Server
	err  error
}

func (f fakePurgeServers) List(context.Context) ([]domain.Server, error) { return f.list, f.err }

// fakePurgeAdapter records the servers it is asked to stop. Like the real
// adapter, it only touches the pid of the handle it is given.
type fakePurgeAdapter struct {
	adapters.ToolAdapter
	stopped []adapters.ServerHandle
	kill    func(pid int)
}

func (f *fakePurgeAdapter) StopServer(_ context.Context, h adapters.ServerHandle) error {
	f.stopped = append(f.stopped, h)
	if f.kill != nil {
		f.kill(h.PID)
	}
	return nil
}

func purgeServer(group, adapter, runtimeKind string, pid int, status domain.ServerStatus) domain.Server {
	return domain.Server{GroupKey: group, Adapter: adapter, Runtime: runtimeKind, PID: pid, URL: "http://127.0.0.1:1", Password: "pw-" + group, Status: status}
}

func runStop(t *testing.T, p purgeProcesses) (purgeProcessReport, string) {
	t.Helper()
	i18ntest.Set(t, "en")
	var buf bytes.Buffer
	rep := stopPurgeProcesses(context.Background(), &buf, p)
	return rep, buf.String()
}

// The daemon is stopped through its client (forced: a purge removes the
// sessions anyway) and the purge waits until it stops answering.
func TestPurgeStopsTheDaemonThroughItsClient(t *testing.T) {
	d := &fakePurgeDaemon{running: true}
	rep, out := runStop(t, purgeProcesses{daemon: d})
	assert.Equal(t, 1, d.shutdowns)
	assert.True(t, d.forced)
	assert.True(t, rep.daemonStopped)
	assert.Contains(t, out, "daemon")
}

func TestPurgeLeavesAnAbsentDaemonAlone(t *testing.T) {
	d := &fakePurgeDaemon{}
	rep, _ := runStop(t, purgeProcesses{daemon: d})
	assert.Zero(t, d.shutdowns)
	assert.False(t, rep.daemonStopped)
}

func TestPurgeReportsADaemonItCannotStop(t *testing.T) {
	d := &fakePurgeDaemon{running: true, shutdownErr: errors.New("boom")}
	rep, out := runStop(t, purgeProcesses{daemon: d})
	assert.False(t, rep.daemonStopped)
	assert.Contains(t, out, "daemon stop --force")
	assert.Contains(t, out, "boom")

	rep, out = runStop(t, purgeProcesses{daemonErr: errors.New("no capability")})
	assert.False(t, rep.daemonStopped)
	assert.Contains(t, out, "no capability")
}

// Only the servers registered in the database as running are stopped, by
// their adapter, with their own handle; sleeping or stopped ones, remote
// ones and container ones are not signalled (containers are listed).
func TestPurgeStopsOnlyTheRegisteredRunningServers(t *testing.T) {
	ad := &fakePurgeAdapter{}
	p := purgeProcesses{
		servers: fakePurgeServers{list: []domain.Server{
			purgeServer("g-ready", "tool-a", "local", 111, domain.ServerReady),
			purgeServer("g-starting", "tool-a", "", 222, domain.ServerStarting),
			purgeServer("g-sleeping", "tool-a", "local", 333, domain.ServerSleeping),
			purgeServer("g-stopped", "tool-a", "local", 444, domain.ServerStopped),
			purgeServer("g-remote", "tool-a", "remote", 555, domain.ServerReady),
			purgeServer("g-box", "tool-a", "container", 666, domain.ServerReady),
		}},
		adapter: func(string) adapters.ToolAdapter { return ad },
		alive:   func(int) bool { return false },
	}
	rep, out := runStop(t, p)

	var pids []int
	for _, h := range ad.stopped {
		pids = append(pids, h.PID)
		assert.Equal(t, "http://127.0.0.1:1", h.URL)
	}
	assert.Equal(t, []int{111, 222}, pids)
	assert.Equal(t, "pw-g-ready", ad.stopped[0].Password)
	assert.Equal(t, 2, rep.serversStopped)
	assert.Contains(t, out, "g-box")
	assert.NotContains(t, out, "g-remote")
}

// A server whose process still lives after StopServer (the adapter refused
// to signal it: it does not answer with its credentials, the pid may belong
// to another program), an unknown adapter and an unreadable database are
// listed, not killed.
func TestPurgeListsWhatItCannotStop(t *testing.T) {
	ad := &fakePurgeAdapter{}
	p := purgeProcesses{
		servers: fakePurgeServers{list: []domain.Server{
			purgeServer("g-silent", "tool-a", "local", 777, domain.ServerReady),
			purgeServer("g-other", "tool-b", "local", 888, domain.ServerReady),
		}},
		adapter: func(name string) adapters.ToolAdapter {
			if name == "tool-a" {
				return ad
			}
			return nil
		},
		alive: func(pid int) bool { return pid == 777 },
	}
	rep, out := runStop(t, p)
	assert.Zero(t, rep.serversStopped)
	assert.Contains(t, out, "pid 777 (group g-silent)")
	assert.Contains(t, out, "pid 888 (group g-other)")
	assert.Contains(t, out, "tool-b")

	_, out = runStop(t, purgeProcesses{})
	assert.Contains(t, out, "database unreadable")
	_, out = runStop(t, purgeProcesses{servers: fakePurgeServers{err: errors.New("locked")}})
	assert.Contains(t, out, "database unreadable")
}

// What oh does not track is always listed, never searched by name.
func TestPurgeAlwaysListsTheProcessesItDoesNotTrack(t *testing.T) {
	rep, out := runStop(t, purgeProcesses{servers: fakePurgeServers{}})
	assert.Contains(t, out, "oh serve")
	assert.Len(t, rep.manual, 1)
}

// With real processes: the registered server (a sleep) is stopped through
// the adapter, while a foreign process whose command line says "oh serve",
// another "oh mcp" one, and a sleep of another hub are left alone.
func TestPurgeDoesNotTouchForeignProcessesNamedLikeOh(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signals")
	}
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("no sleep")
	}
	start := func(argv0 string) *exec.Cmd {
		c := &exec.Cmd{Path: sleep, Args: []string{argv0, "60"}}
		require.NoError(t, c.Start())
		t.Cleanup(func() { _ = c.Process.Kill(); _ = c.Wait() })
		return c
	}
	managed := start("sleep")
	foreignServe := start("oh serve")
	foreignMCP := start("python oh mcp serve")
	otherHub := start("sleep")

	waited := make(chan struct{})
	go func() { _ = managed.Wait(); close(waited) }()

	ad := &fakePurgeAdapter{kill: func(pid int) {
		if pid == managed.Process.Pid {
			_ = managed.Process.Kill()
		}
	}}
	p := purgeProcesses{
		servers: fakePurgeServers{list: []domain.Server{purgeServer("g", "tool-a", "local", managed.Process.Pid, domain.ServerReady)}},
		adapter: func(string) adapters.ToolAdapter { return ad },
		alive:   func(pid int) bool { return pid != managed.Process.Pid && pidAlive(pid) },
	}
	rep, _ := runStop(t, p)

	select {
	case <-waited:
	case <-time.After(5 * time.Second):
		t.Fatal("the registered server was not stopped")
	}
	assert.Equal(t, 1, rep.serversStopped)
	for name, c := range map[string]*exec.Cmd{"oh serve": foreignServe, "oh mcp": foreignMCP, "other hub": otherHub} {
		assert.True(t, pidAlive(c.Process.Pid), "%s must still run", name)
	}
}

// The purge never looks processes up by name.
func TestPurgeSourceHasNoPkill(t *testing.T) {
	for _, f := range []string{"purge.go", "purge_processes.go"} {
		data, err := os.ReadFile(filepath.Join(".", f))
		require.NoError(t, err)
		for _, banned := range []string{"pkill", "killall", `"pgrep"`} {
			assert.False(t, strings.Contains(string(data), banned), "%s uses %s", f, banned)
		}
	}
}

// Without a daemon socket in the hub, nothing is contacted (and no
// capability is created).
func TestDefaultPurgeProcessesNeedsTheSocketOfTheHub(t *testing.T) {
	configtest.Home(t)
	p := defaultPurgeProcesses(context.Background())
	assert.Nil(t, p.daemon)
	assert.NoError(t, p.daemonErr)
	assert.NoDirExists(t, ohRunDir())
}
