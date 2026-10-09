package gc

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
)

// Reasons of an orphan tool server (N18).
const (
	OrphanTemp    = "temp"     // test or discovery server whose parent is gone
	OrphanUnowned = "unowned"  // server of this hub that oh does not run any more
	OrphanHubGone = "hub_gone" // server of an oh home that no longer exists
)

// Orphan is a tool server that belongs to nobody.
type Orphan struct {
	adapters.ServerProcess
	Reason string
}

// ClassifyOrphans keeps the processes that belong to nobody: a server of a
// test or of the native agents discovery whose parent is gone; a server of
// this hub (its output in <hub>/servers/<group>/data/) that oh does not run
// (live: PIDs of the running servers; nil when unknown: left alone) and
// that did not write for minutes (not being started); a server of an oh
// home that has no database any more. A server of another living oh home is
// never one.
func ClassifyOrphans(procs []adapters.ServerProcess, hubDir, tmpDir string, live map[int]bool, exists, recent func(string) bool) []Orphan {
	hubServers := filepath.Join(ResolvedPath(hubDir), "servers") + string(filepath.Separator)
	tmp := ResolvedPath(tmpDir) + string(filepath.Separator)
	var out []Orphan
	for _, p := range procs {
		work, logPath := ResolvedPath(p.WorkDir), ResolvedPath(p.LogPath)
		switch {
		case p.PPID == 1 && (isTempServerDir(work, tmp) || isTempServerDir(logPath, tmp)):
			out = append(out, Orphan{p, OrphanTemp})
		case logPath != "" && strings.HasPrefix(logPath, hubServers):
			if live != nil && !live[p.PID] && !recent(logPath) {
				out = append(out, Orphan{p, OrphanUnowned})
			}
		case logPath != "":
			// <oh home>/servers/<group>/data/<log>
			data := filepath.Dir(logPath)
			if filepath.Base(data) != "data" {
				continue
			}
			servers := filepath.Dir(filepath.Dir(data))
			if filepath.Base(servers) != "servers" {
				continue
			}
			if home := filepath.Dir(servers); !exists(filepath.Join(home, "oh.db")) {
				out = append(out, Orphan{p, OrphanHubGone})
			}
		}
	}
	return out
}

// isTempServerDir reports a directory of a test (t.TempDir: <tmp>/Test…) or
// of the native agents discovery (<tmp>/oh-natives-…).
func isTempServerDir(p, tmp string) bool {
	rest, ok := strings.CutPrefix(p, tmp)
	return ok && (strings.HasPrefix(rest, "Test") || strings.HasPrefix(rest, "oh-natives-"))
}

// ResolvedPath resolves the symbolic links of the closest existing ancestor
// of p (a deleted test directory, a group whose data directory is gone).
func ResolvedPath(p string) string {
	if p == "" {
		return ""
	}
	p = filepath.Clean(p)
	rest := ""
	for dir := p; ; dir = filepath.Dir(dir) {
		if r, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(r, rest)
		}
		if parent := filepath.Dir(dir); parent == dir {
			return p
		}
		rest = filepath.Join(filepath.Base(dir), rest)
	}
}

// RecentlyWritten reports a file changed in the last two minutes.
func RecentlyWritten(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && time.Since(fi.ModTime()) < 2*time.Minute
}
