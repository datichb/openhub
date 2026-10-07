package container

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Diagnostics of the container runtime (P4-T12, Doctor): facts only, the
// command layer turns them into checks. Each call runs at most a few short
// containers.

// ProbeImage is the small image used by the diagnostics when no project
// image has an HTTP client (pulled on first use).
const ProbeImage = "docker.io/library/busybox:1.36"

// ProjectImage is the latest oh image of a project.
type ProjectImage struct {
	ProjectID string
	Ref       string
	Tool      string // <tool>@<version> (oh.tool label)
	Built     int64  // unix time (oh.built label)
}

// ProjectImages returns the latest oh image of each project (newest first).
func (r *Runtime) ProjectImages(ctx context.Context, e Engine) ([]ProjectImage, error) {
	out, err := r.opts.Runner.Run(ctx, e.CLI, e.Command("images",
		"--filter", "label="+labelManaged+"=true", "--filter", "label="+labelRole+"=dev",
		"--format", "{{.Repository}}:{{.Tag}}")[1:]...)
	if err != nil {
		return nil, err
	}
	latest := map[string]ProjectImage{}
	seen := map[string]bool{}
	for _, ref := range strings.Fields(string(out)) {
		if seen[ref] || strings.HasSuffix(ref, ":<none>") {
			continue
		}
		seen[ref] = true
		lb, err := r.opts.Runner.Run(ctx, e.CLI, e.Command("image", "inspect", "--format",
			`{{index .Config.Labels "`+labelProject+`"}}|{{index .Config.Labels "`+labelTool+`"}}|{{index .Config.Labels "`+labelBuilt+`"}}`, ref)[1:]...)
		if err != nil {
			continue
		}
		parts := strings.SplitN(strings.TrimSpace(string(lb)), "|", 3)
		if len(parts) != 3 || parts[0] == "" {
			continue
		}
		built, _ := strconv.ParseInt(parts[2], 10, 64)
		img := ProjectImage{ProjectID: parts[0], Ref: ref, Tool: parts[1], Built: built}
		if cur, ok := latest[img.ProjectID]; !ok || img.Built > cur.Built {
			latest[img.ProjectID] = img
		}
	}
	list := make([]ProjectImage, 0, len(latest))
	for _, img := range latest {
		list = append(list, img)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Built > list[j].Built })
	return list, nil
}

// ToolCheck is the tool binary of an image, as run by the image.
type ToolCheck struct {
	Version string // output of `<tool> --version` ("" when it does not run)
	Musl    bool   // musl base (Alpine…)
	// MissingLibs lists the C++ runtime libraries a musl base lacks
	// (libstdc++, libgcc): the tool needs them.
	MissingLibs []string
	Err         error
}

var versionRe = regexp.MustCompile(`\d+\.\d+\.\d+`)

// CheckTool runs `<tool> --version` in an image and, on a musl base, looks
// for the C++ runtime libraries the tool needs.
func (r *Runtime) CheckTool(ctx context.Context, e Engine, image, tool string) ToolCheck {
	var c ToolCheck
	out, err := r.opts.Runner.Run(ctx, e.CLI, e.Command("run", "--rm", "--entrypoint", tool, image, "--version")[1:]...)
	if err == nil {
		c.Version = versionRe.FindString(string(out))
	} else {
		c.Err = err
	}
	probe := `if ls /lib/ld-musl-* >/dev/null 2>&1; then echo musl; ` +
		`ls /usr/lib/libstdc++.so* /lib/libstdc++.so* >/dev/null 2>&1 || echo missing:libstdc++; ` +
		`ls /usr/lib/libgcc_s.so* /lib/libgcc_s.so* >/dev/null 2>&1 || echo missing:libgcc; fi`
	if out, err := r.opts.Runner.Run(ctx, e.CLI, e.Command("run", "--rm", "--entrypoint", "sh", image, "-c", probe)[1:]...); err == nil {
		for _, f := range strings.Fields(string(out)) {
			switch {
			case f == "musl":
				c.Musl = true
			case strings.HasPrefix(f, "missing:"):
				c.MissingLibs = append(c.MissingLibs, strings.TrimPrefix(f, "missing:"))
			}
		}
	}
	return c
}

// KeepID checks that Podman rootless maps the machine user into containers
// (--userns=keep-id): files created in mounts belong to the user.
func (r *Runtime) KeepID(ctx context.Context, e Engine, image string) (bool, error) {
	uid, gid := os.Getuid(), os.Getgid()
	out, err := r.opts.Runner.Run(ctx, e.CLI, e.Command("run", "--rm",
		"--userns=keep-id:uid="+strconv.Itoa(uid)+",gid="+strconv.Itoa(gid), "--user", strconv.Itoa(uid)+":"+strconv.Itoa(gid),
		"--entrypoint", "id", image, "-u")[1:]...)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == strconv.Itoa(uid), nil
}

// NotShared returns the directories that are not shared with the engine VM
// (mounting them would show an empty directory).
func NotShared(e Engine, dirs []string) []string {
	var out []string
	for _, d := range dirs {
		if d != "" && !shared(e.Shared, clean(d)) {
			out = append(out, d)
		}
	}
	return out
}

// HostIP resolves the machine address seen from containers (Linux: the
// credential proxy and the gateways need a second listener on it).
func (r *Runtime) HostIP(ctx context.Context, e Engine, image string) (string, error) {
	return r.hostIP(ctx, e, Spec{Image: image, AddHosts: r.addHosts(e)})
}

func (r *Runtime) addHosts(e Engine) []string {
	if e.Kind == EngineDocker && r.opts.GOOS == "linux" {
		return []string{hostDocker + ":host-gateway"}
	}
	return nil
}

// errNoHTTPClient is returned by an image without curl nor wget.
var errNoHTTPClient = errors.New("no HTTP client in the image")

// ProbeHTTP sends an unauthenticated POST (body `{}`) to each URL from a
// container and returns the HTTP status received (0 = not reachable): the
// gateway routes only serve POST, a 401 proves the route and the address. The first image with curl or wget
// is used (images are tried in order); the image used is returned.
func (r *Runtime) ProbeHTTP(ctx context.Context, e Engine, images, urls []string) (status map[string]int, image string, err error) {
	script := `for u in "$@"; do ` +
		`if command -v curl >/dev/null 2>&1; then c=$(curl -s -o /dev/null -m 5 -X POST -d '{}' -w '%{http_code}' "$u"); ` +
		`elif command -v wget >/dev/null 2>&1; then c=$(wget -S -q -T 5 --post-data '{}' -O /dev/null "$u" 2>&1 | grep -o 'HTTP/[0-9.]* [0-9]*' | tail -1 | cut -d' ' -f2); ` +
		`else exit 127; fi; echo "$u ${c:-0}"; done`
	lastErr := errNoHTTPClient
	for _, img := range images {
		if img == "" {
			continue
		}
		args := []string{"run", "--rm", "--entrypoint", "sh"}
		for _, h := range r.addHosts(e) {
			args = append(args, "--add-host", h)
		}
		args = append(args, img, "-c", script, "probe")
		args = append(args, urls...)
		out, err := r.opts.Runner.Run(ctx, e.CLI, e.Command(args...)[1:]...)
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", img, err)
			continue
		}
		res := map[string]int{}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			f := strings.Fields(line)
			if len(f) == 2 {
				res[f[0]], _ = strconv.Atoi(f[1])
			}
		}
		return res, img, nil
	}
	return nil, "", lastErr
}

// URLHost replaces the host of a machine URL (http://127.0.0.1:<port>) with
// the address seen from containers.
func URLHost(u, host string) string {
	rest, ok := strings.CutPrefix(u, "http://")
	if !ok {
		return u
	}
	hp, path, _ := strings.Cut(rest, "/")
	_, port, err := net.SplitHostPort(hp)
	if err != nil {
		return u
	}
	out := "http://" + net.JoinHostPort(host, port)
	if path != "" {
		out += "/" + path
	}
	return out
}
