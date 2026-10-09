package gitutil

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Safe git for oh (v5 critical N5, DC5). oh runs git on the machine in
// project repositories an agent may have written to (its shell, or a
// container that mounts the repository): `core.fsmonitor`, hooks
// (`post-checkout`…), `diff.external`, textconv drivers, `core.sshCommand`,
// `remote.*.uploadpack`, credential helpers and `ext::` URLs of the
// repository configuration would then run on the machine. Every git call of
// oh on a project repository goes through Command: the configuration that
// runs programs is overridden on the command line (which wins over every
// configuration file, includes too), taken from the user's global
// configuration when it must exist (SSH command, credential helpers).
//
// What overrides cannot reach (filter drivers, textconv drivers named by an
// attribute) is covered by Check: the fingerprint of the repository
// configuration and hooks is recorded when a session starts (Trust) and git
// calls that matter (checkpoints, worktrees) are refused when it changed.

// noHooks is an empty hooks directory (git runs no hook from it).
func noHooks() string {
	dir := filepath.Join(os.TempDir(), "oh-git-no-hooks")
	_ = os.MkdirAll(dir, 0o700)
	return dir
}

var globalOnce sync.Once
var globalSSH string
var globalHelpers []string

// globalConfig reads, once, the settings oh keeps from the user's global
// configuration (never from the repository).
func globalConfig() (ssh string, helpers []string) {
	globalOnce.Do(func() {
		if out, err := exec.Command("git", "config", "--global", "--get", "core.sshCommand").Output(); err == nil {
			globalSSH = strings.TrimSpace(string(out))
		}
		if out, err := exec.Command("git", "config", "--global", "--get-all", "credential.helper").Output(); err == nil {
			for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				if l = strings.TrimSpace(l); l != "" {
					globalHelpers = append(globalHelpers, l)
				}
			}
		}
	})
	return globalSSH, globalHelpers
}

// SafeArgs returns the git arguments that run args in a repository with its
// program-running configuration neutralized.
func SafeArgs(args ...string) []string {
	ssh, helpers := globalConfig()
	if ssh == "" {
		ssh = "ssh"
	}
	out := []string{
		"-c", "core.fsmonitor=false",
		"-c", "core.hooksPath=" + noHooks(),
		"-c", "core.sshCommand=" + ssh,
		"-c", "core.gitProxy=",
		"-c", "core.askPass=",
		"-c", "core.pager=cat",
		"-c", "diff.external=",
		"-c", "protocol.ext.allow=never",
		"-c", "credential.helper=",
	}
	for _, h := range helpers {
		out = append(out, "-c", "credential.helper="+h)
	}
	sub := subcommand(args)
	out = append(out, args...)
	// remote.<name>.uploadpack is not overridden by -c (git uses the first
	// value): the transport options are given on the command line.
	switch sub {
	case "diff", "log", "show":
		out = insertAfter(out, len(out)-len(args), sub, "--no-ext-diff", "--no-textconv")
	case "fetch", "ls-remote", "pull", "clone":
		out = insertAfter(out, len(out)-len(args), sub, "--upload-pack=git-upload-pack")
	case "push":
		out = insertAfter(out, len(out)-len(args), sub, "--receive-pack=git-receive-pack")
	}
	return out
}

// subcommand returns the git sub-command of args (after global options).
func subcommand(args []string) string {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-C" || a == "-c" || a == "--git-dir" || a == "--work-tree":
			i++
		case strings.HasPrefix(a, "-"):
		default:
			return a
		}
	}
	return ""
}

// insertAfter inserts flags right after the sub-command found from index from.
func insertAfter(args []string, from int, sub string, flags ...string) []string {
	for i := from; i < len(args); i++ {
		if args[i] == sub {
			out := append([]string{}, args[:i+1]...)
			out = append(out, flags...)
			return append(out, args[i+1:]...)
		}
	}
	return args
}

// Command returns git running args in the repository at dir (working
// directory), through SafeArgs, without any terminal prompt and with no
// optional lock taken.
func Command(ctx context.Context, dir string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", SafeArgs(args...)...)
	cmd.Dir = dir
	cmd.Env = safeEnv(os.Environ())
	return cmd
}

// safeEnv removes the variables that redirect git, and disables prompts and
// optional locks.
func safeEnv(parent []string) []string {
	out := make([]string, 0, len(parent)+2)
	for _, kv := range parent {
		k, _, _ := strings.Cut(kv, "=")
		switch k {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_EXTERNAL_DIFF", "GIT_SSH", "GIT_SSH_COMMAND", "GIT_ASKPASS",
			"GIT_EXEC_PATH", "GIT_PAGER", "GIT_EDITOR", "GIT_SEQUENCE_EDITOR", "GIT_TERMINAL_PROMPT", "GIT_OPTIONAL_LOCKS",
			"GIT_CONFIG", "GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS":
			continue
		}
		if strings.HasPrefix(k, "GIT_CONFIG_KEY_") || strings.HasPrefix(k, "GIT_CONFIG_VALUE_") {
			continue
		}
		out = append(out, kv)
	}
	return append(out, "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
}

// Integrity of the repository configuration.

// ErrConfigChanged refuses a git call of oh on a repository whose
// configuration or hooks changed since the session started.
var ErrConfigChanged = errors.New("the git configuration of the repository changed during the session")

var (
	trustMu  sync.Mutex
	trustDir func() string
)

// SetTrustDir sets where the fingerprints recorded by Trust are kept
// (~/.oh/run/git-trust; "" disables Check).
func SetTrustDir(dir string) {
	SetTrustDirFunc(func() string { return dir })
}

// SetTrustDirFunc is SetTrustDir with a directory computed at each use (the
// oh home may be set after the program starts).
func SetTrustDirFunc(fn func() string) {
	trustMu.Lock()
	trustDir = fn
	trustMu.Unlock()
}

func trustFile(common string) string {
	trustMu.Lock()
	fn := trustDir
	trustMu.Unlock()
	if fn == nil {
		return ""
	}
	dir := fn()
	if dir == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(common))
	return filepath.Join(dir, hex.EncodeToString(sum[:16])+".json")
}

type trust struct {
	Common      string `json:"common"`
	Fingerprint string `json:"fingerprint"`
}

// commonDir returns the git common directory of dir ("" outside a repository).
func commonDir(ctx context.Context, dir string) string {
	out, err := Command(ctx, dir, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return ""
	}
	return filepath.Clean(strings.TrimSpace(string(out)))
}

// Fingerprint hashes what makes git run programs in the repository of dir:
// the configuration files of its common directory and of the worktree, the
// hooks, info/attributes and the `.git` pointer file of a worktree.
func Fingerprint(ctx context.Context, dir string) (common, fingerprint string, err error) {
	common = commonDir(ctx, dir)
	if common == "" {
		return "", "", fmt.Errorf("%s is not a git repository", dir)
	}
	h := sha256.New()
	add := func(name string) {
		data, err := os.ReadFile(name)
		if err != nil {
			fmt.Fprintf(h, "%s\x00-\x00", name)
			return
		}
		fmt.Fprintf(h, "%s\x00%d\x00", name, len(data))
		h.Write(data)
	}
	add(filepath.Join(common, "config"))
	add(filepath.Join(common, "info", "attributes"))
	var hooks []string
	_ = filepath.WalkDir(filepath.Join(common, "hooks"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && !strings.HasSuffix(p, ".sample") {
			hooks = append(hooks, p)
		}
		return nil
	})
	sort.Strings(hooks)
	for _, p := range hooks {
		add(p)
	}
	if out, err := Command(ctx, dir, "rev-parse", "--path-format=absolute", "--git-dir").Output(); err == nil {
		gitDir := filepath.Clean(strings.TrimSpace(string(out)))
		if gitDir != common {
			add(filepath.Join(gitDir, "config.worktree"))
			add(filepath.Join(gitDir, "commondir"))
			add(filepath.Join(gitDir, "gitdir"))
		}
	}
	if top, err := Command(ctx, dir, "rev-parse", "--show-toplevel").Output(); err == nil {
		if p := filepath.Join(strings.TrimSpace(string(top)), ".git"); isFile(p) {
			add(p)
		}
	}
	return common, hex.EncodeToString(h.Sum(nil)), nil
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// Trust records the fingerprint of the repository of dir (session start).
func Trust(ctx context.Context, dir string) error {
	common, fp, err := Fingerprint(ctx, dir)
	if err != nil {
		return err
	}
	f := trustFile(common)
	if f == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(f), 0o700); err != nil {
		return err
	}
	data, _ := json.Marshal(trust{Common: common, Fingerprint: fp})
	return os.WriteFile(f, data, 0o600)
}

// Check refuses (ErrConfigChanged) a repository whose fingerprint differs
// from the one recorded by Trust; nil when nothing was recorded (or dir is
// not a repository: the git call itself fails).
func Check(ctx context.Context, dir string) error {
	common, fp, ferr := Fingerprint(ctx, dir)
	if ferr != nil {
		common = ""
	}
	if want, ok := recorded(common); ok && want != fp {
		return fmt.Errorf("%w (%s)", ErrConfigChanged, common)
	}
	return nil
}

// recorded returns the fingerprint Trust recorded for a common directory.
func recorded(common string) (string, bool) {
	f := trustFile(common)
	if common == "" || f == "" {
		return "", false
	}
	data, err := os.ReadFile(f)
	if err != nil {
		return "", false
	}
	var t trust
	if json.Unmarshal(data, &t) != nil {
		return "", false
	}
	return t.Fingerprint, true
}
