package gateway

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Shell start-up of local sessions (A16): the user's own start-up files
// (~/.zshenv…) may put their directories before the fake bd of oh on the
// PATH, so that a real bd (npm, volta, ~/.local/bin) would bypass the
// gateway and beads.allow. The shell of a local session reads oh's start-up
// files instead (ZDOTDIR for zsh, BASH_ENV for bash): they load the user's
// files first, then put the oh directories back at the head of the PATH.
//
// Audit v5, S-15: the same for the other ways a shell reads code when it
// starts. ENV is read by sh and dash when they are interactive (and by bash
// in POSIX mode): oh's file loads the user's ENV, then fixes the PATH. fish
// has no variable for its configuration, only XDG_CONFIG_HOME: for a user
// whose shell is fish the session gets its own configuration folder, whose
// config.fish loads the user's conf.d, functions and config.fish, then fixes
// the PATH, and whose other entries are links to the user's own. bash, when
// interactive, reads ~/.bashrc and nothing redirects it: CheckShellBD starts
// the shell that way too, and the daemon raises its alert when ~/.bashrc
// (or ~/.zshrc, config.fish…) puts another bd first.

// Variables of the shell start-up of local sessions.
const (
	EnvZDOTDIR      = "ZDOTDIR"
	EnvBashEnv      = "BASH_ENV"
	EnvENV          = "ENV"                     // file read by an interactive sh
	EnvXDGConfig    = "XDG_CONFIG_HOME"         // configuration home (fish reads $XDG_CONFIG_HOME/fish)
	EnvUserZDOTDIR  = "OH_USER_ZDOTDIR"         // the user's ZDOTDIR ("" = HOME)
	EnvUserBashEnv  = "OH_USER_BASH_ENV"        // the user's BASH_ENV
	EnvUserENV      = "OH_USER_ENV"             // the user's ENV
	EnvUserXDG      = "OH_USER_XDG_CONFIG_HOME" // the user's configuration home (fish sessions)
	EnvPathFirst    = "OH_PATH_FIRST"           // directories kept first on the PATH
	shellHeader     = "# oh: start-up of the shell of an oh session (generated, do not edit).\n# Loads your own file, then keeps the oh commands (fake bd) first on the PATH.\n"
	shellPathAppend = "[ -n \"$OH_PATH_FIRST\" ] && PATH=\"$OH_PATH_FIRST:$PATH\"\n"
	// fishConfig loads the user's fish configuration, then keeps the oh
	// directories first on the PATH.
	fishConfig = "# oh: configuration of the fish shell of an oh session (generated, do not edit).\n" +
		"# Loads your own configuration, then keeps the oh commands (fake bd) first on the PATH.\n" +
		"set -l _oh_user $OH_USER_XDG_CONFIG_HOME/fish\n" +
		"if test -d $_oh_user/functions; set -p fish_function_path $_oh_user/functions; end\n" +
		"if test -d $_oh_user/completions; set -p fish_complete_path $_oh_user/completions; end\n" +
		"for _oh_file in $_oh_user/conf.d/*.fish\n" +
		"    test -r $_oh_file; and source $_oh_file\n" +
		"end\n" +
		"test -r $_oh_user/config.fish; and source $_oh_user/config.fish\n" +
		"if test -n \"$OH_PATH_FIRST\"\n" +
		"    set -gx PATH (string split : -- $OH_PATH_FIRST) $PATH\n" +
		"end\n"
)

// zshFiles are the zsh start-up files read from ZDOTDIR.
var zshFiles = []string{".zshenv", ".zprofile", ".zshrc", ".zlogin"}

// ShellStartup writes oh's shell start-up files under dir and returns the
// variables that make the shell of a local session read them, with first
// kept at the head of the PATH. getenv reads the environment of the
// machine (the user's ZDOTDIR and BASH_ENV). Nothing on Windows.
func ShellStartup(dir string, first []string, getenv func(string) string) (map[string]string, error) {
	if runtime.GOOS == "windows" || len(first) == 0 {
		return nil, nil
	}
	zdir, bashFile, shFile := filepath.Join(dir, "zsh"), filepath.Join(dir, "bash", "bashenv"), filepath.Join(dir, "sh", "env")
	files := map[string]string{}
	for _, name := range zshFiles {
		files[filepath.Join(zdir, name)] = shellHeader +
			"_oh_zdotdir=$ZDOTDIR\n" +
			"ZDOTDIR=${OH_USER_ZDOTDIR:-$HOME}\n" +
			"[ -r \"$ZDOTDIR/" + name + "\" ] && . \"$ZDOTDIR/" + name + "\"\n" +
			"ZDOTDIR=$_oh_zdotdir\nunset _oh_zdotdir\n" + shellPathAppend
	}
	files[bashFile] = shellHeader +
		"[ -n \"$OH_USER_BASH_ENV\" ] && [ -r \"$OH_USER_BASH_ENV\" ] && . \"$OH_USER_BASH_ENV\"\n" + shellPathAppend
	files[shFile] = shellHeader +
		"[ -n \"$OH_USER_ENV\" ] && [ -r \"$OH_USER_ENV\" ] && . \"$OH_USER_ENV\"\n" + shellPathAppend
	fishHome := filepath.Join(dir, "fish", "xdg")
	fish := isFish(getenv("SHELL"))
	if fish {
		files[filepath.Join(fishHome, "fish", "config.fish")] = fishConfig
	}
	for p, content := range files {
		if err := writeIfChanged(p, content); err != nil {
			return nil, fmt.Errorf("shell start-up of local sessions: %w", err)
		}
	}
	env := map[string]string{
		EnvZDOTDIR:   zdir,
		EnvBashEnv:   bashFile,
		EnvENV:       shFile,
		EnvPathFirst: strings.Join(first, string(os.PathListSeparator)),
	}
	if v := getenv(EnvZDOTDIR); v != "" && v != zdir {
		env[EnvUserZDOTDIR] = v
	}
	if v := getenv(EnvBashEnv); v != "" && v != bashFile {
		env[EnvUserBashEnv] = v
	}
	if v := getenv(EnvENV); v != "" && v != shFile {
		env[EnvUserENV] = v
	}
	if fish {
		userXDG := getenv(EnvXDGConfig)
		if userXDG == "" || userXDG == fishHome {
			userXDG = filepath.Join(getenv("HOME"), ".config")
		}
		if err := linkConfigHome(fishHome, userXDG); err != nil {
			return nil, fmt.Errorf("shell start-up of local sessions (fish): %w", err)
		}
		env[EnvXDGConfig] = fishHome
		env[EnvUserXDG] = userXDG
	}
	return env, nil
}

// isFish reports whether a SHELL value is the fish shell.
func isFish(shell string) bool {
	return shell != "" && filepath.Base(shell) == "fish"
}

// linkConfigHome makes dir look like the configuration home of the user,
// except for fish: every entry of userHome but "fish" is a link to the
// user's own, so that the tools of a session that read XDG_CONFIG_HOME (git,
// gh…) find their configuration where they expect it. The links are redone
// each time (entries added or removed since), nothing else is touched.
func linkConfigHome(dir, userHome string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if cur, err := os.ReadDir(dir); err == nil {
		for _, e := range cur {
			if e.Type()&os.ModeSymlink != 0 {
				if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
					return err
				}
			}
		}
	}
	entries, err := os.ReadDir(userHome)
	if err != nil {
		return nil // no configuration home yet: nothing to link
	}
	for _, e := range entries {
		if e.Name() == "fish" {
			continue
		}
		if err := os.Symlink(filepath.Join(userHome, e.Name()), filepath.Join(dir, e.Name())); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
	}
	return nil
}

func writeIfChanged(path, content string) error {
	if cur, err := os.ReadFile(path); err == nil && string(cur) == content {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".oh-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// ErrShellBD tells that the shell of a session does not find the fake bd first.
var ErrShellBD = errors.New("the shell of the session does not find the bd of oh first")

// Modes in which the shell of a session is started when it is checked.
const (
	// ShellModeCommand is `$SHELL -c`: how the tool runs the commands of the
	// agents. It reads ZDOTDIR, BASH_ENV, fish's configuration…
	ShellModeCommand = "command"
	// ShellModeInteractive is `$SHELL -i -c`: an agent that starts an
	// interactive shell. It reads ENV, ~/.bashrc, ~/.zshrc…
	ShellModeInteractive = "interactive"
)

// shellModes are the checked modes, in order.
var shellModes = []struct {
	name string
	args []string
}{
	{ShellModeCommand, []string{"-c"}},
	{ShellModeInteractive, []string{"-i", "-c"}},
}

// ShellBDError is the ErrShellBD of a mode: which way of starting the shell
// finds another bd first, and which.
type ShellBDError struct {
	Mode string // ShellModeCommand or ShellModeInteractive
	Got  string // what `command -v bd` answered ("" = nothing)
}

func (e *ShellBDError) Error() string {
	return fmt.Sprintf("%s (%s shell finds %q)", ErrShellBD, e.Mode, e.Got)
}

// Is makes errors.Is(err, ErrShellBD) true.
func (e *ShellBDError) Is(target error) bool { return target == ErrShellBD }

// shellProbeTimeout bounds each start of the shell: start-up files may be
// slow (version managers).
const shellProbeTimeout = 15 * time.Second

// CheckShellBD runs `command -v bd` in the shell of a local session (the
// shell named by SHELL, with only the session environment env, as the tool
// does) and returns what it found; ErrShellBD (a *ShellBDError) when it is
// not want. The shell is started as the tool starts it (-c), then as an
// interactive shell (-i -c), which reads other start-up files (S-15). An
// interactive start that answers nothing (no terminal, a start-up file that
// stops it) proves nothing and is not held against the session.
func CheckShellBD(ctx context.Context, env map[string]string, want string) (string, error) {
	if runtime.GOOS == "windows" {
		return "", nil
	}
	var first string
	for i, m := range shellModes {
		got := probeShellBD(ctx, env, m.args)
		if i == 0 {
			first = got
		}
		if got == "" && m.name != ShellModeCommand {
			continue
		}
		if !same(got, want) {
			return got, &ShellBDError{Mode: m.name, Got: got}
		}
	}
	return first, nil
}

// probeShellBD returns the last line `command -v bd` prints in the shell
// started with args (start-up files may print).
func probeShellBD(ctx context.Context, env map[string]string, args []string) string {
	sh := env["SHELL"]
	if sh == "" {
		sh = "/bin/sh"
	}
	pctx, cancel := context.WithTimeout(ctx, shellProbeTimeout)
	defer cancel()
	cmd := exec.CommandContext(pctx, sh, append(append([]string(nil), args...), "command -v bd")...)
	cmd.Env = make([]string, 0, len(env))
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	if dir := env["HOME"]; dir != "" {
		cmd.Dir = dir
	}
	out, _ := cmd.Output()
	got := strings.TrimSpace(string(out))
	if i := strings.LastIndexByte(got, '\n'); i >= 0 {
		got = got[i+1:]
	}
	return got
}

func same(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	ra, err1 := filepath.EvalSymlinks(a)
	rb, err2 := filepath.EvalSymlinks(b)
	return err1 == nil && err2 == nil && ra == rb
}
