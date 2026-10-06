package termlaunch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShellCommandQuoting(t *testing.T) {
	assert.Equal(t, "cd /tmp/x && exec /usr/local/bin/oh session attach ses_1 --exec",
		ShellCommand("/tmp/x", []string{"/usr/local/bin/oh", "session", "attach", "ses_1", "--exec"}))
	assert.Equal(t, `cd '/tmp/my proj' && exec 'it'\''s' ''`, ShellCommand("/tmp/my proj", []string{"it's", ""}))
}

func TestAppleScriptEscaping(t *testing.T) {
	s := iTermScript(`cd '/a "b"' && exec x\y`, ITermTab)
	assert.Contains(t, s, `write text "cd '/a \"b\"' && exec x\\y"`)
	assert.Contains(t, iTermScript("c", ITermSplit), "split vertically")
	assert.Contains(t, iTermScript("c", ITermWindow), "create window")
	assert.Contains(t, terminalScript("c"), `do script "c"`)
}

func TestChainExplicitPrefs(t *testing.T) {
	assert.Equal(t, []Method{MethodITerm}, Chain(PrefITerm))
	assert.Equal(t, []Method{MethodTerminal}, Chain(PrefTerminal))
	assert.Equal(t, []Method{MethodTmux}, Chain(PrefTmux))
}

func TestChainAutoOrder(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("TERM_PROGRAM", "Apple_Terminal")
	c := Chain(PrefAuto)
	if len(c) > 0 {
		assert.Equal(t, MethodTerminal, c[0], "the current terminal comes first")
	}
	t.Setenv("TMUX", "/tmp/tmux-1/default,1,0")
	c = Chain(PrefAuto)
	assert.Equal(t, MethodTmux, c[len(c)-1])
}

func TestLaunchExhaustedChain(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("PATH", "") // no tmux, no osascript
	_, attempts, err := Launch(context.Background(), Options{Pref: PrefTmux, Argv: []string{"true"}})
	assert.ErrorIs(t, err, ErrNoTerminal)
	assert.Len(t, attempts, 1)
	assert.Error(t, attempts[0].Err)
}

func TestTmuxArgs(t *testing.T) {
	assert.Equal(t, []string{"new-window", "-c", "/p", "-n", "oh"}, tmuxArgs(Options{Dir: "/p", Title: "oh"}))
	assert.Equal(t, []string{"split-window", "-h", "-c", "/p"}, tmuxArgs(Options{Dir: "/p", Title: "oh", ITermStyle: ITermSplit}))
}

func TestShellCommandEnv(t *testing.T) {
	assert.Equal(t, "cd /p && A=1 OH_HOME='/tmp/my oh' exec oh x",
		ShellCommandEnv("/p", map[string]string{"OH_HOME": "/tmp/my oh", "A": "1", "bad name": "x", "1X": "y"}, []string{"oh", "x"}))
}

// tmux < 3.0 takes the command as one shell command argument only.
func TestTmuxGetsOneShellCommand(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "args")
	script := "#!/bin/sh\nfor a in \"$@\"; do printf '%s\\n' \"$a\" >> " + out + "; done\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tmux"), []byte(script), 0o755))
	t.Setenv("PATH", dir)
	m, _, err := Launch(context.Background(), Options{Pref: PrefTmux, Dir: "/p q", Title: "oh",
		Env: map[string]string{"OH_HOME": "/h"}, Argv: []string{"/bin/oh", "session", "attach", "ses_1", "--exec"}})
	require.NoError(t, err)
	assert.Equal(t, MethodTmux, m)
	data, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, []string{"new-window", "-c", "/p q", "-n", "oh", "cd '/p q' && OH_HOME=/h exec /bin/oh session attach ses_1 --exec"},
		strings.Split(strings.TrimSpace(string(data)), "\n"))
}
