package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/datichb/openhub/cli/internal/cliout"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/mcpresolve"
)

// mcpDiagnostics lists the enabled services that the sessions leave out for
// want of a token. It reads the environment of the process.
func mcpDiagnostics(states []mcpState) []mcpresolve.Diagnostic {
	var out []mcpresolve.Diagnostic
	for _, st := range states {
		if d := mcpresolve.DiagnoseToken(st.Service.Name, st.Eff.Enabled, st.Eff.TokenKey, os.Getenv); d != nil {
			out = append(out, *d)
		}
	}
	return out
}

// mcpDiagnosticText is the sentence for a diagnostic, with what to do.
func mcpDiagnosticText(d mcpresolve.Diagnostic) string {
	return i18n.Tf("v55m.mcp.diag.no_token", d.Service, d.TokenEnv, d.Service)
}

// printMCPDiagnostics writes one warning line per diagnostic.
func printMCPDiagnostics(out io.Writer, diags []mcpresolve.Diagnostic) {
	if len(diags) == 0 {
		return
	}
	fmt.Fprintln(out)
	for _, d := range diags {
		fmt.Fprintf(out, "  %s %s\n", cliout.WarnIcon(), mcpDiagnosticText(d))
	}
}
