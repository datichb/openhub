package cmd

import (
	"fmt"
	"io"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// warnDeprecatedAlias tells that an old command is an alias of a v5 one (O15).
func warnDeprecatedAlias(w io.Writer, old, replacement string) {
	fmt.Fprintf(w, "%s %s\n", theme.WarningStyle.Render(theme.IconWarning), i18n.Tf("cmd.alias.deprecated", old, replacement))
}
