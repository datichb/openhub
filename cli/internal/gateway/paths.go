package gateway

import (
	"os"
	"path/filepath"
	"strings"

	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
)

// View is how the runtime of a session group sees the machine: path
// translation and the session locations mounted read-write.
type View struct {
	Paths     ohruntime.PathMap
	Locations []string // machine directories the session may name
}

// fileFlags take a file path read or written by bd on the machine (bd 1.3
// long names; short forms are read by bdargs).
var fileFlags = map[string]bool{
	"body-file": true, "design-file": true, "file": true, "graph": true, "output": true, "input": true,
	"reason-file": true, "ids-file": true, "from-file": true, "path": true, "search-path": true,
	"source-db": true, "auth-token-file": true, "agents-file": true, "server-socket": true,
}

// IsFileFlag reports a flag (long name) whose value names a file bd reads
// or writes.
func IsFileFlag(name, value string) bool { return isFileFlag(name, value) }

func isFileFlag(name, value string) bool {
	switch {
	case fileFlags[name]:
		return true
	case name == "metadata":
		return strings.HasPrefix(value, "@")
	}
	return strings.HasPrefix(name, "proxied-server-") && strings.HasSuffix(name, "-path")
}

// within reports whether host (clean, absolute) is inside a location;
// symbolic links are resolved for existing paths.
func (v View) within(host string) bool {
	if r, err := filepath.EvalSymlinks(host); err == nil {
		host = r
	} else if d, err := filepath.EvalSymlinks(filepath.Dir(host)); err == nil {
		host = filepath.Join(d, filepath.Base(host))
	}
	for _, l := range v.Locations {
		if r, err := filepath.EvalSymlinks(l); err == nil {
			l = r
		}
		l = filepath.Clean(l)
		if host == l || strings.HasPrefix(host, l+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// hostDir translates the runtime working directory; outside the session
// locations, bd runs in fallback (the session location).
func (v View) hostDir(cwd, fallback string) string {
	if cwd != "" && filepath.IsAbs(cwd) {
		if h, ok := v.Paths.ToHost(cwd); ok && v.within(h) {
			return h
		}
	}
	return fallback
}

// toHost translates an absolute runtime path ("" when not visible on the machine).
func (v View) toHost(p string) string {
	if v.Paths == nil {
		return filepath.Clean(p)
	}
	if h, ok := v.Paths.ToHost(p); ok {
		return h
	}
	return ""
}

// translateArgs rewrites the runtime paths of a command for the machine
// and returns the argument list bd runs (canonical form, bdargs). File
// arguments (file flags, @file metadata, import files) must resolve inside
// the session locations; other absolute paths under a mount are translated.
func (v View) translateArgs(c command, dir string) ([]string, error) {
	file := func(val string) (string, error) {
		if val == "" || val == "-" {
			return val, nil
		}
		at := strings.HasPrefix(val, "@")
		p := strings.TrimPrefix(val, "@")
		var host string
		if filepath.IsAbs(p) {
			host = v.toHost(p)
		} else {
			host = filepath.Join(dir, p)
		}
		if host == "" || !v.within(host) {
			return "", refuse("cmd.gateway.beads.path_outside", val)
		}
		if !filepath.IsAbs(p) {
			return val, nil // relative to the translated working directory
		}
		if at {
			return "@" + host, nil
		}
		return host, nil
	}
	out := c.Cmd
	out.Flags = append(out.Flags[:0:0], c.Flags...)
	out.Args = append(out.Args[:0:0], c.Args...)
	for i, f := range out.Flags {
		if !isFileFlag(f.Name, f.Value) || !f.Explicit {
			continue
		}
		nv, err := file(f.Value)
		if err != nil {
			return nil, err
		}
		out.Flags[i].Value = nv
	}
	for i, a := range out.Args {
		if c.Name() == "import" {
			nv, err := file(a)
			if err != nil {
				return nil, err
			}
			out.Args[i] = nv
			continue
		}
		if filepath.IsAbs(a) && v.Paths != nil {
			if h, ok := v.Paths.ToHost(a); ok {
				out.Args[i] = h
			}
		}
	}
	return out.Argv(), nil
}

// exists reports whether a directory exists on the machine.
func exists(dir string) bool {
	st, err := os.Stat(dir)
	return err == nil && st.IsDir()
}
