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

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/runtime/container"
)

// ServerUser is the account of the tool server in job images.
const ServerUser = "oh"

// serverUID is the uid given to ServerUser when oh creates it.
const serverUID = 10001

// InstallOptions configure `oh runner install` (thin layer of the job image).
type InstallOptions struct {
	Root        string // filesystem root ("" = /), tests
	ToolVersion string // version of the tool (remote.VarToolVersion; "" = the adapter's former variable)
	// Tool installs the tool of the machine adapter.
	Tool     adapters.LinuxInstaller
	CacheDir string // download cache
	Arch     string // default runtime.GOARCH
	Out      io.Writer
}

func (o InstallOptions) path(p string) string { return filepath.Join(o.Root, p) }

// Libc returns glibc or musl for the image at root.
func Libc(root string) string {
	if m, _ := filepath.Glob(filepath.Join(root, "lib", "ld-musl-*")); len(m) > 0 {
		return "musl"
	}
	return "glibc"
}

// Install installs the tool (pinned to the machine's adapter version), the
// fake bd and the tool server account in the job image.
func Install(ctx context.Context, o InstallOptions) error {
	if o.Tool == nil {
		return errors.New("no tool adapter can install itself for Linux")
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
	got, err := o.Tool.InstallLinux(ctx, adapters.LinuxInstall{Version: o.ToolVersion, Arch: arch, Libc: libc, BinDir: bin, CacheDir: cache})
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "oh runner install: %s %s (%s, %s)\n", filepath.Base(got.Binary), got.Version, arch, libc)
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
