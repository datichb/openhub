package cliout

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"text/tabwriter"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

func TestJSONIsIndentedWithTwoSpaces(t *testing.T) {
	var b bytes.Buffer
	require.NoError(t, JSON(&b, map[string]any{"a": 1, "b": []string{"x"}}))
	assert.Equal(t, "{\n  \"a\": 1,\n  \"b\": [\n    \"x\"\n  ]\n}\n", b.String())
}

func TestJSONCompactIsOneLine(t *testing.T) {
	var b bytes.Buffer
	require.NoError(t, JSONCompact(&b, map[string]any{"a": 1, "b": []string{"x"}}))
	assert.Equal(t, "{\"a\":1,\"b\":[\"x\"]}\n", b.String())
}

func TestJSONKeepsTheEncoderDefaults(t *testing.T) {
	var b bytes.Buffer
	require.NoError(t, JSONCompact(&b, "a<b>&"))
	var want bytes.Buffer
	require.NoError(t, json.NewEncoder(&want).Encode("a<b>&"))
	assert.Equal(t, want.String(), b.String(), "HTML characters are escaped as by json.Encoder")
}

func TestJSONReportsTheWriteError(t *testing.T) {
	assert.Error(t, JSON(failingWriter{}, 1))
	assert.Error(t, JSONCompact(failingWriter{}, 1))
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, fmt.Errorf("closed") }

// The tabwriters that the commands used to build had a minimum width of 0 or
// 2 and a tab width of 0 or 4: with a space as padding and a padding of 2,
// none of those changes a character of the output.
func TestTableAlignsLikeTheTabwritersItReplaces(t *testing.T) {
	rows := "NAME\tSTATE\t\nalpha\t\tx\n\t\t\nlonger-name\tok\tyes\nlast\tok\n"
	for _, p := range []struct{ minWidth, tabWidth int }{{0, 0}, {0, 4}, {2, 4}} {
		var want, got bytes.Buffer
		tw := tabwriter.NewWriter(&want, p.minWidth, p.tabWidth, 2, ' ', 0)
		fmt.Fprint(tw, rows)
		require.NoError(t, tw.Flush())

		table := Table(&got)
		fmt.Fprint(table, rows)
		require.NoError(t, table.Flush())
		assert.Equal(t, want.String(), got.String(), "minwidth %d, tabwidth %d", p.minWidth, p.tabWidth)
	}
	var b bytes.Buffer
	table := Table(&b)
	fmt.Fprintf(table, "%s\t%s\n", "a", "b")
	fmt.Fprintf(table, "%s\t%s\n", "ccc", "d")
	require.NoError(t, table.Flush())
	assert.Equal(t, "a    b\nccc  d\n", b.String())
}

func TestMessagesAreOneLineWithTheirMark(t *testing.T) {
	for _, c := range []struct {
		name string
		fn   func(w *bytes.Buffer, msg string)
		icon string
		mark string
	}{
		{"success", func(w *bytes.Buffer, m string) { Success(w, m) }, theme.IconSuccess, SuccessIcon()},
		{"warn", func(w *bytes.Buffer, m string) { Warn(w, m) }, theme.IconWarning, WarnIcon()},
		{"info", func(w *bytes.Buffer, m string) { Info(w, m) }, theme.IconInfo, InfoIcon()},
		{"error", func(w *bytes.Buffer, m string) { Error(w, m) }, theme.IconError, ErrorIcon()},
	} {
		var b bytes.Buffer
		c.fn(&b, "message")
		assert.Equal(t, c.mark+" message\n", b.String(), c.name)
		assert.True(t, strings.Contains(c.mark, c.icon), "%s: the mark holds the theme's symbol", c.name)
	}
}

func TestMarksAreTheThemeStyles(t *testing.T) {
	assert.Equal(t, theme.SuccessStyle.Render(theme.IconSuccess), SuccessIcon())
	assert.Equal(t, theme.WarningStyle.Render(theme.IconWarning), WarnIcon())
	assert.Equal(t, theme.InfoStyle.Render(theme.IconInfo), InfoIcon())
	assert.Equal(t, theme.ErrorStyle.Render(theme.IconError), ErrorIcon())
}
