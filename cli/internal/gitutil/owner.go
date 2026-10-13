package gitutil

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Git as the owner of the repository (R3-remote-git). In a remote job oh
// runs as root and the clone belongs to the account of the agents: git
// refuses to work in a repository of another owner (safe.directory), and
// it is right to — the configuration of the repository is written by the
// agent, and what the command-line overrides of SafeArgs cannot reach
// (filter drivers named by an attribute, for one) would run as root. oh
// never lifts that refusal (no safe.directory): a git process of oh in a
// repository owned by a registered account runs as that account, with its
// folders, so that whatever the repository makes git run has the rights of
// the agent and nothing more.
//
// Only root switches account, and only to a registered one: elsewhere, or
// for a repository of an unknown owner, git runs as oh and refuses on its
// own; the callers fail closed on that error.

// Owner is an account oh runs git as in the repositories it owns.
type Owner struct {
	UID, GID uint32
	// Home is HOME of the account (configuration, cache and temporary
	// folders below it).
	Home string
}

// Env returns parent with the folders and identity of the account in place
// of those of oh.
func (o Owner) Env(parent []string) []string {
	out := make([]string, 0, len(parent)+4)
	for _, kv := range parent {
		if k, _, _ := strings.Cut(kv, "="); !ownerEnvKeys[k] {
			out = append(out, kv)
		}
	}
	return append(out, "HOME="+o.Home, "XDG_CONFIG_HOME="+filepath.Join(o.Home, ".config"),
		"XDG_CACHE_HOME="+filepath.Join(o.Home, ".cache"), "TMPDIR="+filepath.Join(o.Home, "tmp"))
}

// ownerEnvKeys are replaced by the folders of the account.
var ownerEnvKeys = map[string]bool{
	"HOME": true, "USER": true, "LOGNAME": true, "TMPDIR": true,
	"XDG_CACHE_HOME": true, "XDG_CONFIG_HOME": true, "XDG_DATA_HOME": true,
}

var (
	ownersMu sync.RWMutex
	owners   map[uint32]Owner
	// geteuid and fileOwner are replaced by the tests.
	geteuid   = os.Geteuid
	fileOwner = statOwner
)

// RegisterOwner lets oh, when it runs as root, run git as o in the
// repositories o owns (remote job: the account of the agents).
func RegisterOwner(o Owner) {
	ownersMu.Lock()
	defer ownersMu.Unlock()
	if owners == nil {
		owners = map[uint32]Owner{}
	}
	owners[o.UID] = o
}

// ResetOwners forgets the registered accounts (tests).
func ResetOwners() {
	ownersMu.Lock()
	owners = nil
	ownersMu.Unlock()
}

// OwnerOf returns the account git runs as in dir: its owner when oh runs
// as root, dir belongs to another account and that account is registered.
func OwnerOf(dir string) (Owner, bool) {
	if dir == "" || geteuid() != 0 {
		return Owner{}, false
	}
	uid, ok := fileOwner(dir)
	if !ok || uid == 0 {
		return Owner{}, false
	}
	ownersMu.RLock()
	defer ownersMu.RUnlock()
	o, ok := owners[uid]
	return o, ok
}

// AsOwner makes cmd, a git process of oh in dir, run as the owner of dir
// when OwnerOf says so (its environment is set: cmd.Env, or the
// environment of oh). It reports whether it did.
func AsOwner(cmd *exec.Cmd, dir string) bool {
	o, ok := OwnerOf(dir)
	if !ok {
		return false
	}
	ApplyOwner(cmd, o)
	return true
}

// ApplyOwner makes cmd run as o, with its folders.
func ApplyOwner(cmd *exec.Cmd, o Owner) {
	env := cmd.Env
	if env == nil {
		env = os.Environ()
	}
	cmd.Env = o.Env(env)
	if err := runAsOwner(cmd, o); err != nil {
		cmd.Err = err
	}
}

// CommandAs is Command run as o, whoever owns dir. Nothing of the
// configuration of oh is passed to it (IsolatedArgs: no SSH command nor
// credential helper of the user of oh).
func CommandAs(ctx context.Context, dir string, o Owner, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", IsolatedArgs(args...)...)
	cmd.Dir = dir
	cmd.Env = safeEnv(os.Environ())
	ApplyOwner(cmd, o)
	return cmd
}

// LooksLikeRepository reports whether dir or one of its parents holds a
// .git entry (read on the file system, not by git: true for a repository
// git refuses).
func LooksLikeRepository(dir string) bool {
	if dir == "" {
		return false
	}
	d, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	for {
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(d)
		if parent == d {
			return false
		}
		d = parent
	}
}
