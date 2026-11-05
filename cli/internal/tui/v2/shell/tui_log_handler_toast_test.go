package shell

import (
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// A warning is not an error: its toast says the message (and the reason when
// the record has one) on one line, without the key=value attributes; an error
// keeps its whole text.
func TestTUILogHandlerToastMessage(t *testing.T) {
	record := func(l slog.Level, msg string, attrs ...any) slog.Record {
		r := slog.NewRecord(time.Now(), l, msg, 0)
		r.Add(attrs...)
		return r
	}
	h := NewTUILogHandler(nil, nil, slog.LevelWarn)
	hg := h.WithGroup("tracker").WithGroup("sync").(*TUILogHandler)
	hp := h.WithAttrs([]slog.Attr{slog.String("project", "api-backend"), slog.Any("err", errors.New("boom"))}).(*TUILogHandler)

	tests := []struct {
		name string
		h    *TUILogHandler
		r    slog.Record
		want string
	}{
		{"attributes are not shown", h, record(slog.LevelWarn, "deploy failed", "project", "api-backend", "retries", 3), "deploy failed"},
		{"the reason is shown without its key", h, record(slog.LevelWarn, "pull failed", "project", "api", "error", errors.New("connection refused")), "pull failed : connection refused"},
		{"groups prefix the message", hg, record(slog.LevelWarn, "pull failed"), "tracker.sync: pull failed"},
		{"the attributes of the logger are not shown either", hp, record(slog.LevelWarn, "restarting"), "restarting : boom"},
		{"a multi-line message is one line", h, record(slog.LevelWarn, "first\n\tsecond  third \n"), "first second third"},
		{"a level between warning and error is a warning", h, record(slog.LevelWarn+2, "slow", "ms", 900), "slow"},
		{"an error keeps everything", h, record(slog.LevelError, "recovered panic", "crash_file", "/tmp/crash.log"), "recovered panic crash_file=/tmp/crash.log"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.h.toastMessage(tc.r, tc.h.formatMessage(tc.r)))
		})
	}
}

// cornerColors returns the foreground colour of the top-left corners drawn
// on the screen (the corner of a toast takes the colour of its level).
func (s *simShell) cornerColors() (colors []tcell.Color) {
	s.loop(func() {
		cells, _, _ := s.sim.GetContents()
		for _, c := range cells {
			if len(c.Runes) > 0 && c.Runes[0] == '┌' {
				fg, _, _ := c.Style.Decompose()
				colors = append(colors, fg)
			}
		}
	})
	return colors
}

// The toast of a slog.Warn has the colour and the icon of a warning (not the
// red of an error), a readable text and one entry in the notification centre,
// with the details of the record; a slog.Error stays red.
func TestTUILogHandlerWarningToastIsNotRed(t *testing.T) {
	store := NewNotificationStore(10)
	s := newKeyShell(t, &keyView{id: "home", focusList: true})
	s.notifications = store
	h := NewTUILogHandler(s.Shell, store, slog.LevelWarn)
	logger := slog.New(h)

	logger.Warn("pull failed", "project", "api-backend", "error", errors.New("connection refused"))
	require.Eventually(t, func() bool { return s.toastCount() == 1 }, 5*time.Second, 10*time.Millisecond)

	screen := s.screenText()
	assert.Contains(t, screen, theme.IconWarning+" pull failed : connection refused")
	assert.NotContains(t, screen, "project=")
	assert.NotContains(t, screen, "error=")
	assert.Contains(t, s.cornerColors(), theme.Warning, "the border of a warning")
	assert.NotContains(t, s.cornerColors(), theme.Error, "no red for a warning")

	all := store.All()
	require.Len(t, all, 1, "the toast and the log are one notification, not two")
	assert.Equal(t, ToastWarning, all[0].Level)
	assert.Equal(t, "pull failed project=api-backend error=connection refused", all[0].Message, "the details stay in the notification")

	s.loop(func() { s.DismissToasts() })
	logger.Error("sync broke", "project", "api-backend")
	require.Eventually(t, func() bool { return s.toastCount() == 1 }, 5*time.Second, 10*time.Millisecond)
	assert.Contains(t, s.cornerColors(), theme.Error, "an error stays red")
	assert.True(t, strings.Contains(s.screenText(), theme.IconError+" sync broke project=api-backend"), "an error keeps its text")
	assert.Equal(t, ToastError, store.All()[0].Level)
}
