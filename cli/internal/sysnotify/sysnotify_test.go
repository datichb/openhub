package sysnotify

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func notifier(goos string, have ...string) *Notifier {
	return &Notifier{GOOS: goos, LookPath: func(name string) (string, error) {
		for _, h := range have {
			if h == name {
				return "/usr/local/bin/" + name, nil
			}
		}
		return "", errors.New("not found")
	}}
}

func TestCommand(t *testing.T) {
	note := Note{Title: "oh", Message: `2 "sessions" wait`, Group: "oh-sessions", Sound: true}

	n := notifier("darwin", "terminal-notifier")
	n.Activate = "com.googlecode.iterm2"
	assert.Equal(t, []string{"/usr/local/bin/terminal-notifier", "-title", "oh", "-message", `2 "sessions" wait`, "-group", "oh-sessions", "-sound", "default", "-activate", "com.googlecode.iterm2"}, n.Command(note))

	assert.Equal(t, []string{"osascript", "-e", `display notification "2 \"sessions\" wait" with title "oh" sound name "default"`}, notifier("darwin").Command(note))
	assert.Equal(t, []string{"/usr/local/bin/notify-send", "--app-name=oh", "oh", `2 "sessions" wait`}, notifier("linux", "notify-send").Command(note))
	assert.Nil(t, notifier("linux").Command(note))
	assert.Nil(t, notifier("windows").Command(note))
}

func TestNotify(t *testing.T) {
	n := notifier("darwin")
	var got []string
	n.Run = func(_ context.Context, name string, args ...string) error {
		got = append([]string{name}, args...)
		return nil
	}
	require.NoError(t, n.Notify(context.Background(), Note{Title: "oh", Message: "x"}))
	assert.Equal(t, "osascript", got[0])
	assert.ErrorIs(t, notifier("windows").Notify(context.Background(), Note{}), ErrUnavailable)
}
