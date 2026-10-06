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

// fileFlags take a file path (read or written by bd on the machine).
var fileFlags = map[string]bool{
	"--body-file": true, "--design-file": true, "--file": true, "-f": true, "--graph": true,
	"--output": true, "-o": true, "--input": true, "-i": true,
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

// translateArgs rewrites the runtime paths of argv for the machine. File
// arguments (file flags, @file metadata, import files) must resolve inside
// the session locations; other absolute paths under a mount are translated.
func (v View) translateArgs(argv []string, c command, dir string) ([]string, error) {
	out := make([]string, len(argv))
	copy(out, argv)
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
	ended := false
	for i := 0; i < len(out); i++ {
		a := out[i]
		if a == "--" {
			ended = true
			continue
		}
		if !ended && strings.HasPrefix(a, "-") {
			name, val, hasValue := strings.Cut(a, "=")
			isFile := fileFlags[name] || (name == "--metadata" && strings.HasPrefix(val, "@"))
			switch {
			case isFile && hasValue:
				nv, err := file(val)
				if err != nil {
					return nil, err
				}
				out[i] = name + "=" + nv
			case fileFlags[name] && i+1 < len(out):
				nv, err := file(out[i+1])
				if err != nil {
					return nil, err
				}
				out[i+1] = nv
				i++
			case name == "--metadata" && i+1 < len(out) && strings.HasPrefix(out[i+1], "@"):
				nv, err := file(out[i+1])
				if err != nil {
					return nil, err
				}
				out[i+1] = nv
				i++
			}
			continue
		}
		if c.Name == "import" && i > c.At {
			nv, err := file(a)
			if err != nil {
				return nil, err
			}
			out[i] = nv
			continue
		}
		if filepath.IsAbs(a) && v.Paths != nil {
			if h, ok := v.Paths.ToHost(a); ok {
				out[i] = h
			}
		}
	}
	return out, nil
}

// exists reports whether a directory exists on the machine.
func exists(dir string) bool {
	st, err := os.Stat(dir)
	return err == nil && st.IsDir()
}
