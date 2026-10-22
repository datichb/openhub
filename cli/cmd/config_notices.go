package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// What reading hub.toml and the keychain has to say (A-08): a former key is
// announced once per process (the config package deduplicates), an unknown
// key is ignored with a warning.

func init() {
	config.SetNoticeHandler(warnConfigNotice)
}

// logJSON is set when --log-format json asks for structured logs only.
var logJSON bool

// warnConfigNotice writes the warning on stderr like the other warnings of
// the startup; in the TUI (stderr would corrupt the screen) and with
// --log-format json it goes through slog.
func warnConfigNotice(n config.Notice) {
	text := noticeText(n)
	if tuiShell != nil || logJSON {
		slog.Warn(text)
		return
	}
	fmt.Fprintf(os.Stderr, "%s %s\n", theme.WarningStyle.Render(theme.IconWarning), text)
}

// noticeText is the translated sentence of a notice.
func noticeText(n config.Notice) string {
	where := noticeWhere(n)
	switch n.Kind {
	case config.NoticeLegacyKey:
		if where == "" {
			return i18n.Tf("cmd.config.legacy_key_alias", n.Key, n.Replacement, config.LegacySupportEnds)
		}
		return i18n.Tf("cmd.config.legacy_key", n.Key, where, n.Replacement, config.LegacySupportEnds)
	case config.NoticeLegacyKeychain:
		return i18n.Tf("cmd.config.legacy_keychain", n.Key, n.Replacement, config.LegacySupportEnds)
	default:
		return i18n.Tf("cmd.config.unknown_hub_key", n.Key, where)
	}
}

// noticeWhere is "file:line:column" ("" without a position).
func noticeWhere(n config.Notice) string {
	if n.File == "" {
		return ""
	}
	if n.Line == 0 {
		return n.File
	}
	return fmt.Sprintf("%s:%d:%d", n.File, n.Line, n.Column)
}

// describeConfigError is the translated detail of a hub.toml that does not
// load: line and column, key, what is wrong, how to fix it. The file name is
// left to the caller.
func describeConfigError(ce *config.ConfigError) string {
	var parts []string
	if ce.Line > 0 {
		parts = append(parts, i18n.Tf("cmd.config.err.position", ce.Line, ce.Column))
	}
	if ce.Key != "" {
		parts = append(parts, i18n.Tf("cmd.config.err.key", ce.Key))
	}
	head := strings.Join(parts, ", ")

	var detail string
	switch {
	case ce.Expected != "" && ce.Found != "" && ce.InList:
		detail = i18n.Tf("cmd.config.err.type_in_list", kindLabel(ce.Expected), kindLabel(ce.Found))
	case ce.Expected != "" && ce.Found != "":
		detail = i18n.Tf("cmd.config.err.type", kindLabel(ce.Expected), kindLabel(ce.Found))
	default:
		detail = ce.Msg
	}
	switch ce.Hint() {
	case config.HintUnquote:
		detail += " (" + i18n.T("cmd.config.hint.unquote") + ")"
	case config.HintList:
		detail += " (" + i18n.T("cmd.config.hint.list") + ")"
	}
	if head == "" {
		return detail
	}
	return i18n.Tf("cmd.config.err.head_detail", head, detail)
}

// kindLabel names a kind of TOML value ("a boolean").
func kindLabel(kind string) string {
	switch kind {
	case config.KindBoolean, config.KindInteger, config.KindNumber, config.KindString, config.KindList, config.KindTable:
		return i18n.T("cmd.config.kind." + kind)
	}
	return kind
}

// configLoadError is the message of a hub.toml that does not load, when err
// is one (ok false otherwise).
func configLoadError(err error) (string, bool) {
	var ce *config.ConfigError
	if !errors.As(err, &ce) {
		return "", false
	}
	return i18n.Tf("cmd.config.invalid_file", ce.Path, describeConfigError(ce)), true
}
