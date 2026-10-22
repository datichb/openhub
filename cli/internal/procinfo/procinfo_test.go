package procinfo

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetChild(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		_, err := Get(os.Getpid())
		assert.True(t, errors.Is(err, ErrUnsupported))
		return
	}
	// Not the last command of the script: dash would replace itself with
	// sleep (exec), and the arguments read right after the start would be
	// those of sleep, or empty for a moment during the exec.
	cmd := exec.Command("/bin/sh", "-c", "sleep 5; exit 0", "procinfo-marker")
	if err := cmd.Start(); err != nil {
		t.Skip("no sleep command")
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	// Right after the start, the child may not have run exec yet: its
	// arguments are then empty or those of the test binary.
	want := []string{"/bin/sh", "-c", "sleep 5; exit 0", "procinfo-marker"}
	var p Process
	require.Eventually(t, func() bool {
		got, err := Get(cmd.Process.Pid)
		if err != nil {
			return false
		}
		p = got
		return slices.Equal(got.Args, want)
	}, 5*time.Second, 10*time.Millisecond, "the arguments of the child once it ran exec")
	assert.Equal(t, os.Getpid(), p.PPID)
	assert.Equal(t, want, p.Args)

	_, err := Get(1 << 30)
	assert.Error(t, err)
}
