package remote

import (
	"bufio"
	"encoding/json"
	"path"
	"sort"
	"strings"
)

// The build context of a dev Dockerfile (S-02): the machine sends it to the
// generic packages of oh-runner (the image jobs never access the target
// project) and the base image tag hashes it with the Dockerfile.

// contextSources returns the paths of the build context (repository root)
// that the Dockerfile reads: the sources of COPY and ADD (without --from)
// and of the bind mounts of RUN. whole is true when they cannot be told
// apart (wildcards, heredocs, the whole context): the context is then the
// whole tree. .dockerignore is not applied (the context may be larger than
// what the build reads: a rebuild too many, never a stale image).
func contextSources(dockerfile []byte) (paths []string, whole bool) {
	seen := map[string]bool{}
	add := func(p string) {
		p = cleanContextPath(p)
		switch {
		case p == "":
			whole = true
		case !seen[p]:
			seen[p] = true
			paths = append(paths, p)
		}
	}
	for _, ins := range dockerInstructions(dockerfile) {
		f := strings.Fields(ins)
		if len(f) == 0 {
			continue
		}
		switch strings.ToUpper(f[0]) {
		case "COPY", "ADD":
			args, flags := splitFlags(strings.TrimSpace(ins[len(f[0]):]))
			if hasFlag(flags, "--from") {
				continue
			}
			if strings.Contains(ins, "<<") {
				whole = true
				continue
			}
			srcs := args
			if len(srcs) > 1 {
				srcs = srcs[:len(srcs)-1] // destination
			}
			for _, s := range srcs {
				switch {
				case isRemoteSource(s):
				case strings.ContainsAny(s, "*?[\\$"):
					whole = true
				default:
					add(s)
				}
			}
		case "RUN":
			for _, m := range strings.Fields(ins) {
				if !strings.HasPrefix(m, "--mount=") || !strings.Contains(m, "type=bind") || strings.Contains(m, "from=") {
					continue
				}
				src := "."
				for _, kv := range strings.Split(strings.TrimPrefix(m, "--mount="), ",") {
					if k, v, ok := strings.Cut(kv, "="); ok && (k == "source" || k == "src") {
						src = v
					}
				}
				add(src)
			}
		}
	}
	sort.Strings(paths)
	return paths, whole
}

// dockerInstructions joins the continuation lines (trailing backslash) and
// drops comments and blank lines.
func dockerInstructions(data []byte) []string {
	var out []string
	var cur strings.Builder
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if cur.Len() == 0 && (line == "" || strings.HasPrefix(line, "#")) {
			continue
		}
		if strings.HasSuffix(line, "\\") {
			cur.WriteString(strings.TrimSuffix(line, "\\") + " ")
			continue
		}
		cur.WriteString(line)
		out = append(out, cur.String())
		cur.Reset()
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// splitFlags separates the leading --flags of COPY/ADD from its arguments
// (shell or JSON form).
func splitFlags(rest string) (args, flags []string) {
	f := strings.Fields(rest)
	i := 0
	for ; i < len(f) && strings.HasPrefix(f[i], "--"); i++ {
		flags = append(flags, f[i])
	}
	tail := strings.TrimSpace(strings.Join(f[i:], " "))
	if strings.HasPrefix(tail, "[") {
		var arr []string
		if json.Unmarshal([]byte(tail), &arr) == nil {
			return arr, flags
		}
	}
	return f[i:], flags
}

func hasFlag(flags []string, name string) bool {
	for _, f := range flags {
		if f == name || strings.HasPrefix(f, name+"=") {
			return true
		}
	}
	return false
}

func isRemoteSource(s string) bool {
	for _, p := range []string{"http://", "https://", "git@", "git://", "ssh://"} {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// cleanContextPath makes a source relative to the context root ("" = the
// root itself).
func cleanContextPath(p string) string {
	p = path.Clean("/" + strings.Trim(p, "\"'"))
	if p == "/" {
		return ""
	}
	return strings.TrimPrefix(p, "/")
}
