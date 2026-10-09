package procinfo

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetChild(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		_, err := Get(os.Getpid())
		assert.True(t, errors.Is(err, ErrUnsupported))
		return
	}
	cmd := exec.Command("/bin/sh", "-c", "sleep 5", "procinfo-marker")
	if err := cmd.Start(); err != nil {
		t.Skip("no sleep command")
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	p, err := Get(cmd.Process.Pid)
	require.NoError(t, err)
	assert.Equal(t, os.Getpid(), p.PPID)
	assert.Equal(t, []string{"/bin/sh", "-c", "sleep 5", "procinfo-marker"}, p.Args)

	_, err = Get(1 << 30)
	assert.Error(t, err)
}
