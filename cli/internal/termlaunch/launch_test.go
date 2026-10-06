package termlaunch

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
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
