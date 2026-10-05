package container

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
)

// layerVersion changes whenever the thin oh layer changes (new tag for all images).
const layerVersion = "1"

// dockerfileCandidates are probed in order in the project directory.
var dockerfileCandidates = []string{"Dockerfile.dev", "dev.Dockerfile", ".devcontainer/Dockerfile", "Dockerfile"}

// defaultBaseDockerfile is used when the project has no dev Dockerfile.
const defaultBaseDockerfile = `FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates git ripgrep \
 && rm -rf /var/lib/apt/lists/*
`

// Image labels.
const (
	labelManaged = "oh.managed"
	labelRole    = "oh.role" // base | dev
	labelProject = "oh.project"
	labelBuilt   = "oh.built" // unix time
	labelTool    = "oh.tool"
)

// ErrUnavailable is returned when no container engine can be used.
var ErrUnavailable = errors.New("container engine unavailable")

// Image is a project image ready to run tool servers.
type Image struct {
	Ref        string // oh-dev/<project>:<hash>
	BaseRef    string // oh-base/<project>:<hash> (project dev environment)
	Dockerfile string // dev Dockerfile used ("" = oh default base)
	Arch       string // amd64 | arm64
	Libc       string // glibc | musl
	Built      bool   // built by this call (false = cache hit)
}

// DetectDockerfile returns the dev Dockerfile of a project: the configured
// path (absolute or relative to the project), else the first candidate found.
// "" means none (the oh default base image is used).
func DetectDockerfile(projectDir, configured string) (string, error) {
	if configured != "" {
		p := configured
		if !filepath.IsAbs(p) {
			p = filepath.Join(projectDir, p)
		}
		if st, err := os.Stat(p); err != nil || !st.Mode().IsRegular() {
			return "", fmt.Errorf("dev Dockerfile %s not found", p)
		}
		return p, nil
	}
	for _, c := range dockerfileCandidates {
		p := filepath.Join(projectDir, c)
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
			return p, nil
		}
	}
	return "", nil
}

// imageName turns a project id into a valid image repository component.
func imageName(projectID string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(projectID) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	s := strings.Trim(b.String(), ".-_")
	if s == "" {
		s = "project"
	}
	return s
}

func hashParts(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func sortedArgs(args map[string]string) []string {
	out := make([]string, 0, len(args))
	for k, v := range args {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}

// EnsureImage returns the project image of a group, building it when it is
// not cached: first the project dev environment (base), then the thin oh
// layer (tool binary pinned to the adapter version, fake bd). Tags are
// content hashes; older images of the project are pruned (KeepImages).
func (r *Runtime) EnsureImage(ctx context.Context, g ohruntime.Group) (Image, error) {
	if g.Tool == nil {
		return Image{}, errors.New("container: no tool to install in the image")
	}
	e, av := r.Engine(ctx)
	if !av.OK {
		return Image{}, fmt.Errorf("%w: %s", ErrUnavailable, av.Message())
	}
	df, err := DetectDockerfile(g.ProjectDir, g.Dockerfile)
	if err != nil {
		return Image{}, err
	}
	content := defaultBaseDockerfile
	if df != "" {
		data, err := os.ReadFile(df)
		if err != nil {
			return Image{}, err
		}
		content = string(data)
	}
	args := sortedArgs(g.BuildArgs)
	name := imageName(g.ProjectID)
	baseHash := hashParts(append([]string{"base", content}, args...)...)
	img := Image{BaseRef: "oh-base/" + name + ":" + baseHash[:12], Dockerfile: df}

	if !r.imageExists(ctx, e, img.BaseRef) {
		if err := r.buildBase(ctx, e, g, img.BaseRef, df, args); err != nil {
			return Image{}, err
		}
		img.Built = true
	}
	img.Arch, img.Libc, err = r.probeBase(ctx, e, img.BaseRef, baseHash)
	if err != nil {
		return Image{}, err
	}
	bd, err := r.bd(img.Arch)
	if err != nil {
		return Image{}, err
	}
	bdSum := sha256.Sum256(bd)
	tool := g.Tool.Name() + "@" + g.Tool.Version()
	finalHash := hashParts("dev", baseHash, layerVersion, tool, img.Arch, img.Libc, hex.EncodeToString(bdSum[:]))
	img.Ref = "oh-dev/" + name + ":" + finalHash[:12]
	if r.imageExists(ctx, e, img.Ref) {
		return img, nil
	}
	bin, err := g.Tool.LinuxBinary(ctx, img.Arch, img.Libc)
	if err != nil {
		return Image{}, fmt.Errorf("getting %s for linux/%s (%s): %w", tool, img.Arch, img.Libc, err)
	}
	if err := r.buildLayer(ctx, e, g, img, bin, bd); err != nil {
		return Image{}, err
	}
	img.Built = true
	r.prune(ctx, e, g.ProjectID, "dev", img.Ref)
	r.prune(ctx, e, g.ProjectID, "base", img.BaseRef)
	return img, nil
}

func (r *Runtime) imageExists(ctx context.Context, e Engine, ref string) bool {
	_, err := r.opts.Runner.Run(ctx, e.CLI, e.Command("image", "inspect", "--format", "{{.Id}}", ref)[1:]...)
	return err == nil
}

func labels(g ohruntime.Group, role string) []string {
	out := []string{
		"--label", labelManaged + "=true",
		"--label", labelRole + "=" + role,
		"--label", labelProject + "=" + g.ProjectID,
		"--label", labelBuilt + "=" + strconv.FormatInt(time.Now().Unix(), 10),
	}
	if role == "dev" {
		out = append(out, "--label", labelTool+"="+g.Tool.Name()+"@"+g.Tool.Version())
	}
	return out
}

func (r *Runtime) stream(ctx context.Context, e Engine, g ohruntime.Group, args ...string) error {
	return r.opts.Runner.Stream(ctx, g.Progress, e.CLI, e.Command(args...)[1:]...)
}

// buildBase builds the project dev environment (the project directory is
// the build context; the oh default base needs no context).
func (r *Runtime) buildBase(ctx context.Context, e Engine, g ohruntime.Group, ref, df string, args []string) error {
	contextDir := g.ProjectDir
	if df == "" {
		dir, err := r.buildDir("base-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		df = filepath.Join(dir, "Dockerfile")
		if err := os.WriteFile(df, []byte(defaultBaseDockerfile), 0o644); err != nil {
			return err
		}
		contextDir = dir
	}
	cmd := []string{"build", "-f", df, "-t", ref}
	cmd = append(cmd, labels(g, "base")...)
	for _, a := range args {
		cmd = append(cmd, "--build-arg", a)
	}
	cmd = append(cmd, contextDir)
	if err := r.stream(ctx, e, g, cmd...); err != nil {
		return fmt.Errorf("building the project dev image: %w", err)
	}
	return nil
}

// buildLayer adds the oh layer on top of the base image.
func (r *Runtime) buildLayer(ctx context.Context, e Engine, g ohruntime.Group, img Image, toolBin string, bd []byte) error {
	dir, err := r.buildDir("layer-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := linkOrCopy(toolBin, filepath.Join(dir, "oh-tool")); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "oh-bd"), bd, 0o755); err != nil {
		return err
	}
	dockerfile := fmt.Sprintf("FROM %s\nUSER root\nCOPY oh-tool /usr/local/bin/%s\nCOPY oh-bd /usr/local/bin/bd\n", img.BaseRef, g.Tool.Name())
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(dockerfile), 0o644); err != nil {
		return err
	}
	cmd := append([]string{"build", "-t", img.Ref}, labels(g, "dev")...)
	cmd = append(cmd, dir)
	if err := r.stream(ctx, e, g, cmd...); err != nil {
		return fmt.Errorf("building the oh image layer: %w", err)
	}
	return nil
}

func (r *Runtime) buildDir(pattern string) (string, error) {
	base := filepath.Join(r.opts.CacheDir, "build")
	if err := os.MkdirAll(base, 0o700); err != nil {
		return "", err
	}
	return os.MkdirTemp(base, pattern)
}

func linkOrCopy(src, dst string) error {
	if err := os.Link(src, dst); err == nil {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o755)
}

type baseProbe struct {
	Arch string `json:"arch"`
	Libc string `json:"libc"`
}

// probeBase returns the architecture and libc of the base image (cached per
// base hash: probing the libc starts a container).
func (r *Runtime) probeBase(ctx context.Context, e Engine, ref, baseHash string) (arch, libc string, err error) {
	cache := filepath.Join(r.opts.CacheDir, "images", baseHash[:16]+".json")
	var p baseProbe
	if data, err := os.ReadFile(cache); err == nil && json.Unmarshal(data, &p) == nil && p.Arch != "" {
		return p.Arch, p.Libc, nil
	}
	out, err := r.opts.Runner.Run(ctx, e.CLI, e.Command("image", "inspect", "--format", "{{.Architecture}}", ref)[1:]...)
	if err != nil {
		return "", "", fmt.Errorf("inspecting %s: %w", ref, err)
	}
	p.Arch = normalizeArch(strings.TrimSpace(string(out)))
	if p.Arch != "amd64" && p.Arch != "arm64" {
		return "", "", fmt.Errorf("unsupported image architecture %q", strings.TrimSpace(string(out)))
	}
	p.Libc = "glibc"
	probe := e.Command("run", "--rm", "--entrypoint", "sh", ref, "-c", "if ls /lib/ld-musl-* >/dev/null 2>&1; then echo musl; else echo glibc; fi")
	if out, err := r.opts.Runner.Run(ctx, e.CLI, probe[1:]...); err == nil && strings.TrimSpace(string(out)) == "musl" {
		p.Libc = "musl"
	}
	if err := os.MkdirAll(filepath.Dir(cache), 0o700); err == nil {
		data, _ := json.Marshal(p)
		_ = os.WriteFile(cache, data, 0o600)
	}
	return p.Arch, p.Libc, nil
}

func normalizeArch(a string) string {
	switch a {
	case "aarch64", "arm64":
		return "arm64"
	case "x86_64", "amd64":
		return "amd64"
	}
	return a
}

// prune removes the older images of a project and role, keeping the
// KeepImages most recent ones (and always keep). Removal errors (image in
// use) are ignored.
func (r *Runtime) prune(ctx context.Context, e Engine, projectID, role, keep string) {
	n := r.opts.KeepImages
	if n <= 0 {
		n = 2
	}
	out, err := r.opts.Runner.Run(ctx, e.CLI, e.Command("images",
		"--filter", "label="+labelManaged+"=true",
		"--filter", "label="+labelProject+"="+projectID,
		"--filter", "label="+labelRole+"="+role,
		"--format", "{{.Repository}}:{{.Tag}}")[1:]...)
	if err != nil {
		return
	}
	type entry struct {
		ref   string
		built int64
	}
	var list []entry
	seen := map[string]bool{}
	for _, ref := range strings.Fields(string(out)) {
		if seen[ref] || strings.HasSuffix(ref, ":<none>") {
			continue
		}
		seen[ref] = true
		if sameRef(ref, keep) {
			continue
		}
		b, err := r.opts.Runner.Run(ctx, e.CLI, e.Command("image", "inspect", "--format", `{{index .Config.Labels "`+labelBuilt+`"}}`, ref)[1:]...)
		if err != nil {
			continue
		}
		t, _ := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
		list = append(list, entry{ref, t})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].built > list[j].built })
	for i, x := range list {
		if i < n-1 {
			continue
		}
		_, _ = r.opts.Runner.Run(ctx, e.CLI, e.Command("rmi", x.ref)[1:]...)
	}
}

// sameRef compares image references, ignoring the "localhost/" prefix Podman
// adds to local images.
func sameRef(a, b string) bool {
	return strings.TrimPrefix(a, "localhost/") == strings.TrimPrefix(b, "localhost/")
}
