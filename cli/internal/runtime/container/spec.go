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

	// The git files that make git run programs are read-only inside (v5
	// critical N5, DC5): the git of the machine (oh, and the user after the
	// session) would run them.
	// A file may be visible at several places (the project mount and the
	// git common directory at its machine path): every view is covered.
	views := append(ohruntime.PathMap(nil), s.Paths...)
	protected := map[string]bool{}
	for _, l := range locs {
		for _, src := range gitProtected(l) {
			for _, inner := range innerViews(views, src) {
				if protected[inner] {
					continue
				}
				protected[inner] = true
				if err := add(Mount{Source: src, Target: inner, ReadOnly: true}); err != nil {
					return Spec{}, err
				}
			}
		}
	}

	name := imageName(g.ProjectID)
	// HOME is a volume of the group, emptied when its container starts
	// (Command): nothing an agent leaves there (.zshenv…) reaches another
	// session (audit S-33).
	s.Mounts = append(s.Mounts, Mount{Source: homeVolume(name, groupKey), Target: InnerHome, Volume: true})
	for _, v := range g.Volumes {
		if err := checkVolume(v); err != nil {
			return Spec{}, err
		}
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
	// The repository configuration is read-only: branches are created
	// without writing their tracking configuration.
	s.Env["GIT_CONFIG_COUNT"] = "3"
	s.Env["GIT_CONFIG_KEY_0"], s.Env["GIT_CONFIG_VALUE_0"] = "safe.directory", "*"
	s.Env["GIT_CONFIG_KEY_1"], s.Env["GIT_CONFIG_VALUE_1"] = "gc.worktreePruneExpire", "never"
	s.Env["GIT_CONFIG_KEY_2"], s.Env["GIT_CONFIG_VALUE_2"] = "branch.autoSetupMerge", "false"

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
		"--label", labelManaged + "=true", "--label", "oh.container=" + s.Name,
		// The agent's processes get no capability and cannot gain one
		// (setuid); a fork bomb stays in the container (audit S-33).
		"--cap-drop=ALL", "--security-opt=no-new-privileges", "--pids-limit=" + strconv.Itoa(pidsLimit)}
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

// pidsLimit bounds the processes of a group container.
const pidsLimit = 4096

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
// innerViews returns every path at which the bind mounts of views show the
// machine path src.
func innerViews(views ohruntime.PathMap, src string) []string {
	var out []string
	for _, m := range views {
		if src == m.Host {
			out = append(out, m.Inner)
		} else if rel, ok := strings.CutPrefix(src, m.Host+string(filepath.Separator)); ok {
			out = append(out, filepath.Join(m.Inner, rel))
		}
	}
	return out
}

// homeVolume is the HOME volume of a group container.
func homeVolume(project, groupKey string) string {
	sum := sha256.Sum256([]byte(groupKey))
	return "oh-" + project + "-" + hex.EncodeToString(sum[:4]) + "-home"
}

// checkVolume refuses a cache volume path that would inject mount options
// or hide oh's own directories.
func checkVolume(v string) error {
	if strings.ContainsAny(v, ",\"\n\r=") {
		return fmt.Errorf("volume %q: commas, quotes, '=' and newlines are not allowed", v)
	}
	if c := filepath.Clean(v); filepath.IsAbs(c) && (c == "/opt/oh" || strings.HasPrefix(c, "/opt/oh/") || c == "/") {
		return fmt.Errorf("volume %q: %s is reserved to oh", v, c)
	}
	if strings.Contains(filepath.ToSlash(v), "..") {
		return fmt.Errorf("volume %q: '..' is not allowed", v)
	}
	return nil
}

// gitProtected returns the machine paths of a location that hold what makes
// git run programs: configuration, hooks, attributes, and for a worktree its
// `.git` pointer file and the links of its administrative directory. The
// hooks directory is created when missing (empty), so that it can be mounted
// read-only. Paths that do not exist are left out.
func gitProtected(loc string) []string {
	dotGit := filepath.Join(loc, ".git")
	st, err := os.Stat(dotGit)
	if err != nil {
		return nil
	}
	var out []string
	keep := func(p string) {
		if _, err := os.Lstat(p); err == nil {
			out = append(out, p)
		}
	}
	commonFiles := func(common string) {
		hooks := filepath.Join(common, "hooks")
		_ = os.MkdirAll(hooks, 0o755)
		keep(filepath.Join(common, "config"))
		keep(hooks)
		keep(filepath.Join(common, "info", "attributes"))
		if mods, _ := filepath.Glob(filepath.Join(common, "modules", "*")); len(mods) > 0 {
			for _, m := range mods {
				keep(filepath.Join(m, "config"))
				keep(filepath.Join(m, "hooks"))
			}
		}
	}
	if st.IsDir() {
		commonFiles(clean(dotGit))
		return out
	}
	common := gitCommonDir(loc)
	if common == "" {
		return nil
	}
	keep(clean(dotGit))
	commonFiles(common)
	if data, err := os.ReadFile(dotGit); err == nil {
		if gd, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir:"); ok {
			gd = strings.TrimSpace(gd)
			if !filepath.IsAbs(gd) {
				gd = filepath.Join(loc, gd)
			}
			gd = clean(gd)
			keep(filepath.Join(gd, "commondir"))
			keep(filepath.Join(gd, "gitdir"))
			keep(filepath.Join(gd, "config.worktree"))
		}
	}
	return out
}

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
