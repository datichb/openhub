package container

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
)

// Fixed paths inside the container.
const (
	InnerBundle = "/opt/oh/bundle"
	InnerData   = "/opt/oh/data"
	InnerHome   = "/opt/oh/home"
	InnerWork   = "/work"
)

// Mount is a bind mount (Source = machine path) or a named volume.
type Mount struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"read_only,omitempty"`
	Volume   bool   `json:"volume,omitempty"`
}

// Spec is everything needed to run the container of a server group.
type Spec struct {
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	Engine    EngineKind        `json:"engine"`
	Mounts    []Mount           `json:"mounts"`
	Paths     ohruntime.PathMap `json:"paths"`
	Env       map[string]string `json:"env"`
	UserArgs  []string          `json:"user_args"`
	AddHosts  []string          `json:"add_hosts,omitempty"`
	Host      string            `json:"host"`
	Locations []string          `json:"locations"` // machine locations visible in the container
}

// ContainerName is the container name of a group (stable: a stale container
// of the same group is replaced at the next start).
func ContainerName(groupKey string) string {
	var b strings.Builder
	for _, r := range groupKey {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return "oh-" + b.String()
}

// specInput gathers the facts buildSpec depends on (machine user, OS).
type specInput struct {
	GOOS     string
	UID, GID int
}

// buildSpec computes mounts, path translation, user, environment and host
// address of a group container.
//
// Layout: bundle → /opt/oh/bundle (read-only), group data → /opt/oh/data,
// project → /work/<name>, other locations (worktrees) → /work/<name>; the
// git common directory of a worktree is also mounted at its machine path so
// that absolute `gitdir:` links resolve; /opt/oh/home is a per-project volume.
func buildSpec(e Engine, in specInput, g ohruntime.Group, image, groupKey string) (Spec, error) {
	s := Spec{Name: ContainerName(groupKey), Image: image, Engine: e.Kind, Host: e.Host, Env: map[string]string{}}
	add := func(m Mount) error {
		if !m.Volume {
			if strings.ContainsAny(m.Source, ",\n") || strings.ContainsAny(m.Target, ",\n") {
				return fmt.Errorf("path %q cannot be mounted (comma or newline)", m.Source)
			}
			if !shared(e.Shared, m.Source) {
				return fmt.Errorf("%s is not shared with the %s VM (shared: %s)", m.Source, e.Kind, strings.Join(e.Shared, ", "))
			}
			s.Paths = append(s.Paths, ohruntime.Mapping{Host: m.Source, Inner: m.Target})
		}
		s.Mounts = append(s.Mounts, m)
		return nil
	}
	if g.BundleDir != "" {
		if err := add(Mount{Source: clean(g.BundleDir), Target: InnerBundle, ReadOnly: true}); err != nil {
			return Spec{}, err
		}
	}
	if g.DataDir != "" {
		if err := add(Mount{Source: clean(g.DataDir), Target: InnerData}); err != nil {
			return Spec{}, err
		}
	}

	used := map[string]bool{}
	workName := func(dir string) string {
		base := imageName(filepath.Base(dir))
		name := base
		for i := 2; used[name]; i++ {
			name = base + "-" + strconv.Itoa(i)
		}
		used[name] = true
		return filepath.Join(InnerWork, name)
	}
	var locs []string
	if g.ProjectDir != "" {
		locs = append(locs, clean(g.ProjectDir))
	}
	for _, l := range g.Locations {
		locs = append(locs, clean(l))
	}
	var workMounts []string
	for _, l := range locs {
		if covered(s.Paths, l) {
			s.Locations = appendUnique(s.Locations, l)
			continue
		}
		if err := add(Mount{Source: l, Target: workName(l)}); err != nil {
			return Spec{}, err
		}
		s.Locations = appendUnique(s.Locations, l)
		workMounts = append(workMounts, l)
	}
	for _, l := range locs {
		common := gitCommonDir(l)
		if common == "" || covered(s.Paths, common) && sameInner(s.Paths, common) {
			continue
		}
		if err := add(Mount{Source: common, Target: common}); err != nil {
			return Spec{}, err
		}
	}

	name := imageName(g.ProjectID)
	s.Mounts = append(s.Mounts, Mount{Source: "oh-" + name + "-home", Target: InnerHome, Volume: true})
	for _, v := range g.Volumes {
		if filepath.IsAbs(v) {
			s.Mounts = append(s.Mounts, Mount{Source: volumeName(name, v), Target: clean(v), Volume: true})
			continue
		}
		for _, l := range workMounts {
			inner, _ := s.Paths.ToInner(l)
			t := filepath.Join(inner, v)
			s.Mounts = append(s.Mounts, Mount{Source: volumeName(name, t), Target: t, Volume: true})
		}
	}

	s.Env["HOME"] = InnerHome
	s.Env["XDG_CONFIG_HOME"] = InnerHome + "/.config"
	s.Env["XDG_CACHE_HOME"] = InnerHome + "/.cache"
	// The repository is owned by another uid in some setups; worktree
	// metadata must never be pruned from inside (the worktrees live on the
	// machine, at paths the container does not see).
	s.Env["GIT_CONFIG_COUNT"] = "2"
	s.Env["GIT_CONFIG_KEY_0"], s.Env["GIT_CONFIG_VALUE_0"] = "safe.directory", "*"
	s.Env["GIT_CONFIG_KEY_1"], s.Env["GIT_CONFIG_VALUE_1"] = "gc.worktreePruneExpire", "never"

	ids := strconv.Itoa(in.UID) + ":" + strconv.Itoa(in.GID)
	if e.Kind == EnginePodman && e.Rootless {
		s.UserArgs = []string{"--userns=keep-id:uid=" + strconv.Itoa(in.UID) + ",gid=" + strconv.Itoa(in.GID), "--user", ids}
	} else {
		s.UserArgs = []string{"--user", ids}
	}
	if e.Kind == EngineDocker && in.GOOS == "linux" {
		s.AddHosts = []string{hostDocker + ":host-gateway"}
	}
	return s, nil
}

// runArgs returns the engine arguments (after the CLI and its global
// arguments) that run p in the container, and the content of the env file
// they reference: environment values are never on the command line, and the
// container variables (HOME…) never mix with the CLI process environment.
func runArgs(s Spec, p ohruntime.Proc, envFile string) (args []string, envContent string, err error) {
	args = []string{"run", "--rm", "--init", "--name", s.Name,
		"--label", labelManaged + "=true", "--label", "oh.container=" + s.Name}
	args = append(args, s.UserArgs...)
	for _, h := range s.AddHosts {
		args = append(args, "--add-host", h)
	}
	for _, m := range s.Mounts {
		typ := "bind"
		if m.Volume {
			typ = "volume"
		}
		spec := "type=" + typ + ",source=" + m.Source + ",target=" + m.Target
		if m.ReadOnly {
			spec += ",readonly"
		}
		args = append(args, "--mount", spec)
	}
	for _, port := range p.Ports {
		args = append(args, "-p", fmt.Sprintf("127.0.0.1:%d:%d", port, port))
	}
	vars := map[string]string{}
	for k, v := range s.Env {
		vars[k] = v
	}
	for k, v := range p.Env {
		vars[k] = v
	}
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		if strings.ContainsAny(k, "=\n") || strings.ContainsAny(vars[k], "\r\n") {
			return nil, "", fmt.Errorf("environment variable %q cannot be passed to a container (newline)", k)
		}
		b.WriteString(k + "=" + vars[k] + "\n")
	}
	if len(keys) > 0 {
		args = append(args, "--env-file", envFile)
	}
	if p.Dir != "" {
		args = append(args, "-w", p.Dir)
	}
	args = append(args, s.Image)
	args = append(args, p.Argv...)
	return args, b.String(), nil
}

func clean(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		p = r
	}
	return filepath.Clean(p)
}

func shared(roots []string, p string) bool {
	if len(roots) == 0 {
		return true
	}
	for _, r := range roots {
		r = filepath.Clean(r)
		if p == r || strings.HasPrefix(p, r+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func covered(m ohruntime.PathMap, p string) bool { return len(m) > 0 && m.Covers(p) }

// sameInner reports whether p is visible at its own machine path.
func sameInner(m ohruntime.PathMap, p string) bool {
	in, ok := m.ToInner(p)
	return ok && in == p
}

func appendUnique(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}

func volumeName(project, target string) string {
	h := sha256.Sum256([]byte(target))
	return "oh-" + project + "-" + hex.EncodeToString(h[:4])
}

// gitCommonDir returns the git common directory of a worktree location ("" =
// not a linked worktree). Pure file reads: no git binary needed.
func gitCommonDir(loc string) string {
	data, err := os.ReadFile(filepath.Join(loc, ".git"))
	if err != nil {
		return "" // no .git, or a directory (main worktree)
	}
	gitdir, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir:")
	if !ok {
		return ""
	}
	gitdir = strings.TrimSpace(gitdir)
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(loc, gitdir)
	}
	common := gitdir
	if c, err := os.ReadFile(filepath.Join(gitdir, "commondir")); err == nil {
		common = strings.TrimSpace(string(c))
		if !filepath.IsAbs(common) {
			common = filepath.Join(gitdir, common)
		}
	}
	return clean(common)
}
