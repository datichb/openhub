package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/datichb/openhub/cli/internal/adapters/opencodev2"
	"github.com/datichb/openhub/cli/internal/runtime/container"
)

// ServerUser is the account of the tool server in job images.
const ServerUser = "oh"

// serverUID is the uid given to ServerUser when oh creates it.
const serverUID = 10001

// InstallOptions configure `oh runner install` (thin layer of the job image).
type InstallOptions struct {
	Root        string // filesystem root ("" = /), tests
	ToolVersion string // opencode version (OH_OPENCODE_VERSION)
	CacheDir    string // download cache
	Arch        string // default runtime.GOARCH
	Out         io.Writer
}

func (o InstallOptions) path(p string) string { return filepath.Join(o.Root, p) }

// Libc returns glibc or musl for the image at root.
func Libc(root string) string {
	if m, _ := filepath.Glob(filepath.Join(root, "lib", "ld-musl-*")); len(m) > 0 {
		return "musl"
	}
	return "glibc"
}

// Install installs opencode (pinned to the machine's adapter version), the
// fake bd and the tool server account in the job image.
func Install(ctx context.Context, o InstallOptions) error {
	if o.ToolVersion == "" {
		return errors.New("OH_OPENCODE_VERSION is not set (build argument of the oh layer)")
	}
	arch := o.Arch
	if arch == "" {
		arch = runtime.GOARCH
	}
	out := o.Out
	if out == nil {
		out = io.Discard
	}
	if _, err := exec.LookPath("git"); err != nil && o.Root == "" {
		return errors.New("git is missing from the project image: add it to the dev Dockerfile")
	}
	bin := o.path("/usr/local/bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return err
	}
	libc := Libc(o.Root)
	cache := o.CacheDir
	if cache == "" {
		cache = filepath.Join(os.TempDir(), "oh-install")
	}
	tool := &opencodev2.LinuxTool{Ver: strings.TrimPrefix(o.ToolVersion, "v"), CacheDir: cache}
	src, err := tool.LinuxBinary(ctx, arch, libc)
	if err != nil {
		return fmt.Errorf("opencode %s for linux/%s (%s): %w", tool.Ver, arch, libc, err)
	}
	if err := copyFile(src, filepath.Join(bin, "opencode"), 0o755); err != nil {
		return err
	}
	fmt.Fprintf(out, "oh runner install: opencode %s (%s, %s)\n", tool.Ver, arch, libc)
	bd, err := container.FakeBD(arch)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(bin, "bd"), bd, 0o755); err != nil {
		return err
	}
	fmt.Fprintln(out, "oh runner install: fake bd (journal mode)")
	if err := os.MkdirAll(o.path("/opt/oh"), 0o755); err != nil {
		return err
	}
	_ = os.RemoveAll(cache)
	return ensureUser(o)
}

// ensureUser adds the tool server account (no tool needed: passwd and group
// entries are appended when missing).
func ensureUser(o InstallOptions) error {
	passwd, group := o.path("/etc/passwd"), o.path("/etc/group")
	data, _ := os.ReadFile(passwd)
	if hasEntry(string(data), ServerUser) {
		return nil
	}
	home := "/home/" + ServerUser
	id := strconv.Itoa(serverUID)
	if err := appendLine(group, ServerUser+":x:"+id+":"); err != nil {
		return err
	}
	if err := appendLine(passwd, ServerUser+":x:"+id+":"+id+":oh tool server:"+home+":/bin/sh"); err != nil {
		return err
	}
	if err := os.MkdirAll(o.path(home), 0o700); err != nil {
		return err
	}
	if o.Root == "" {
		return os.Chown(home, serverUID, serverUID)
	}
	return nil
}

func hasEntry(content, name string) bool {
	for _, l := range strings.Split(content, "\n") {
		if strings.HasPrefix(l, name+":") {
			return true
		}
	}
	return false
}

func appendLine(path, line string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	data, _ := os.ReadFile(path)
	prefix := ""
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		prefix = "\n"
	}
	_, werr := f.WriteString(prefix + line + "\n")
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	return werr
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

// LookupServerUser returns the tool server account when the runner runs as
// root and the account exists (nil otherwise: same user, warned).
func LookupServerUser() *User {
	if os.Geteuid() != 0 {
		return nil
	}
	u, err := user.Lookup(ServerUser)
	if err != nil {
		return nil
	}
	uid, err1 := strconv.ParseUint(u.Uid, 10, 32)
	gid, err2 := strconv.ParseUint(u.Gid, 10, 32)
	if err1 != nil || err2 != nil {
		return nil
	}
	return &User{UID: uint32(uid), GID: uint32(gid), Home: u.HomeDir}
}
