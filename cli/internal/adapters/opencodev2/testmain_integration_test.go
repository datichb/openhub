//go:build integration && !windows

package opencodev2

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/testenv"
)

// TestMain stops the servers left by interrupted test runs (N18): a test
// binary killed before its cleanups (timeout, Ctrl+C) leaves its servers
// running in their own session. Only servers whose parent is gone and whose
// directories are those of a test or of the native agents discovery are
// stopped (other packages may be running their own servers meanwhile).
func TestMain(m *testing.M) {
	finish := testenv.Isolate()
	sweepTestServers()
	code := gittest.Main(m)
	sweepTestServers()
	os.Exit(finish(code))
}

func sweepTestServers() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	procs, err := listServerProcesses(ctx)
	if err != nil {
		return
	}
	tmp := os.TempDir()
	if r, err := filepath.EvalSymlinks(tmp); err == nil {
		tmp = r
	}
	tmp += string(filepath.Separator)
	for _, p := range procs {
		if p.PPID != 1 || (!isTestDir(p.WorkDir, tmp) && !isTestDir(p.LogPath, tmp)) {
			continue
		}
		fmt.Fprintf(os.Stderr, "stopping a server left by an interrupted test: pid %d (%s)\n", p.PID, p.LogPath)
		_ = AttachServer("", "", p.PID).Stop(ctx, 3*time.Second)
	}
}

func isTestDir(p, tmp string) bool {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		p = r
	}
	rest, ok := strings.CutPrefix(p, tmp)
	return ok && (strings.HasPrefix(rest, "Test") || strings.HasPrefix(rest, "oh-natives-"))
}
