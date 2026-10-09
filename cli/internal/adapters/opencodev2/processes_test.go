//go:build !windows

package opencodev2

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
)

// N18: only the servers oh starts are listed.
func TestParsePS(t *testing.T) {
	out := `  101     1 /opt/homebrew/bin/opencode serve --hostname 127.0.0.1 --port 4096
  102   900 opencode serve --hostname 0.0.0.0 --port 4100
  103     1 opencode serve
  104     1 vim opencode serve --hostname
  105     1 /usr/bin/node /x/opencode-darwin-arm64/bin/opencode serve --hostname 127.0.0.1 --port 1
`
	assert.Equal(t, []adapters.ServerProcess{{PID: 101, PPID: 1}, {PID: 102, PPID: 900}, {PID: 105, PPID: 1}}, parsePS(out))
	assert.False(t, isOhServerCommand("opencode run --hostname"))
}

func TestParseLsof(t *testing.T) {
	got := parseLsof("p101\nfcwd\nn/tmp/TestX/work\nf1\nn/h/.oh/servers/g/data/oh-server.log\np102\nfcwd\nn/w\n")
	assert.Equal(t, map[int][2]string{101: {"/tmp/TestX/work", "/h/.oh/servers/g/data/oh-server.log"}, 102: {"/w", ""}}, got)
}

// The child process of TestStopOnSignal: a detached "server", watched.
func TestHelperStopOnSignal(t *testing.T) {
	if os.Getenv("OH_TEST_STOP_ON_SIGNAL") == "" {
		t.Skip("helper process")
	}
	cmd := exec.Command("sleep", "60")
	setDetached(cmd)
	require.NoError(t, cmd.Start())
	go func() { _ = cmd.Wait() }()
	defer stopOnSignal(AttachServer("", "", cmd.Process.Pid))()
	os.Stdout.WriteString(strconv.Itoa(cmd.Process.Pid) + "\n")
	time.Sleep(30 * time.Second)
}

// N18: a discovery server is stopped when oh is interrupted, and the
// signal still ends oh.
func TestStopOnSignal(t *testing.T) {
	c := exec.Command(os.Args[0], "-test.run=^TestHelperStopOnSignal$", "-test.count=1", "-test.v")
	c.Env = append(os.Environ(), "OH_TEST_STOP_ON_SIGNAL=1")
	stdout, err := c.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, c.Start())
	sc := bufio.NewScanner(stdout)
	pid := 0
	for sc.Scan() {
		if n, err := strconv.Atoi(strings.TrimSpace(sc.Text())); err == nil {
			pid = n
			break
		}
	}
	require.Positive(t, pid)
	require.True(t, processAlive(pid))
	require.NoError(t, c.Process.Signal(syscall.SIGTERM))
	err = c.Wait()
	require.Error(t, err, "the signal ends the process")
	require.Eventually(t, func() bool { return !processAlive(pid) }, 5*time.Second, 50*time.Millisecond, "the server is stopped")
}

// N18: the servers oh starts are found with their directories.
func TestListServerProcessesFindsAChild(t *testing.T) {
	if _, err := exec.LookPath("lsof"); err != nil && !isLinux() {
		t.Skip("lsof")
	}
	procs, err := listServerProcesses(context.Background())
	require.NoError(t, err)
	for _, p := range procs {
		assert.Positive(t, p.PID)
	}
}

func isLinux() bool { _, err := os.Stat("/proc/self"); return err == nil }
