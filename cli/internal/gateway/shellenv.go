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
)

// Shell start-up of local sessions (A16): the user's own start-up files
// (~/.zshenv…) may put their directories before the fake bd of oh on the
// PATH, so that a real bd (npm, volta, ~/.local/bin) would bypass the
// gateway and beads.allow. The shell of a local session reads oh's start-up
// files instead (ZDOTDIR for zsh, BASH_ENV for bash): they load the user's
// files first, then put the oh directories back at the head of the PATH.

// Variables of the shell start-up of local sessions.
const (
	EnvZDOTDIR      = "ZDOTDIR"
	EnvBashEnv      = "BASH_ENV"
	EnvUserZDOTDIR  = "OH_USER_ZDOTDIR"  // the user's ZDOTDIR ("" = HOME)
	EnvUserBashEnv  = "OH_USER_BASH_ENV" // the user's BASH_ENV
	EnvPathFirst    = "OH_PATH_FIRST"    // directories kept first on the PATH
	shellHeader     = "# oh: start-up of the shell of an oh session (generated, do not edit).\n# Loads your own file, then keeps the oh commands (fake bd) first on the PATH.\n"
	shellPathAppend = "[ -n \"$OH_PATH_FIRST\" ] && PATH=\"$OH_PATH_FIRST:$PATH\"\n"
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
	zdir, bashFile := filepath.Join(dir, "zsh"), filepath.Join(dir, "bash", "bashenv")
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
	for p, content := range files {
		if err := writeIfChanged(p, content); err != nil {
			return nil, fmt.Errorf("shell start-up of local sessions: %w", err)
		}
	}
	env := map[string]string{
		EnvZDOTDIR:   zdir,
		EnvBashEnv:   bashFile,
		EnvPathFirst: strings.Join(first, string(os.PathListSeparator)),
	}
	if v := getenv(EnvZDOTDIR); v != "" && v != zdir {
		env[EnvUserZDOTDIR] = v
	}
	if v := getenv(EnvBashEnv); v != "" && v != bashFile {
		env[EnvUserBashEnv] = v
	}
	return env, nil
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

// CheckShellBD runs `command -v bd` in the shell of a local session (the
// shell named by SHELL, with only the session environment env, as the tool
// does) and returns what it found; ErrShellBD when it is not want.
func CheckShellBD(ctx context.Context, env map[string]string, want string) (string, error) {
	if runtime.GOOS == "windows" {
		return "", nil
	}
	sh := env["SHELL"]
	if sh == "" {
		sh = "/bin/sh"
	}
	cmd := exec.CommandContext(ctx, sh, "-c", "command -v bd")
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
		got = got[i+1:] // start-up files may print
	}
	if same(got, want) {
		return got, nil
	}
	return got, ErrShellBD
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
