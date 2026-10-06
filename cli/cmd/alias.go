package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// Aliases of the former launch commands (O15): `oh start`, `--dev`,
// `--onboard`, `--parallel`, `--sweep`, `oh audit|review|debug`, `oh review
// feedback` run their workflow through `oh run`, with a deprecation warning.
// While the target workflow is not in the catalogue (hub content without
// workflows) or opencode V2 is missing, the former launch runs unchanged.

// warnDeprecatedAlias tells that an old command is an alias of a v5 one (O15).
func warnDeprecatedAlias(w io.Writer, old, replacement string) {
	fmt.Fprintf(w, "%s %s\n", theme.WarningStyle.Render(theme.IconWarning), i18n.Tf("cmd.alias.deprecated", old, replacement))
}

// workflowAlias describes the `oh run` equivalent of a former command.
type workflowAlias struct {
	Old      string // former command line, e.g. "oh audit --type security"
	Workflow string
	Opts     runOptions
	Recap    bool
}

// aliasAvailable reports whether an alias can run its workflow: opencode
// V2 present and the workflow in the catalogue.
func aliasAvailable(ctx context.Context, workflowID string) bool {
	if !v5Available(ctx) {
		return false
	}
	if !newWorkflowService(ctx).Has(ctx, workflowsvc.Context{}, workflowID) {
		slog.Debug("alias target workflow not in the catalogue, former launch", "workflow", workflowID)
		return false
	}
	return true
}

// tryWorkflowAlias runs the alias through `oh run` when possible. handled is
// false when the former launch must run (no opencode V2, workflow absent).
// al.Opts.Project must be set.
func tryWorkflowAlias(cmd *cobra.Command, al workflowAlias) (handled bool, err error) {
	if !aliasAvailable(cmd.Context(), al.Workflow) {
		return false, nil
	}
	return true, runAlias(cmd, al)
}

// runAlias warns and runs the workflow of an alias.
func runAlias(cmd *cobra.Command, al workflowAlias) error {
	warnDeprecatedAlias(cmd.ErrOrStderr(), al.Old, aliasReplacement(al))
	al.Opts.Workflow = al.Workflow
	if al.Opts.Inputs == nil {
		al.Opts.Inputs = map[string]string{}
	}
	return runWorkflowCLI(cmd, al.Opts, al.Recap)
}

// aliasReplacement is the `oh run` command line shown in the warning.
func aliasReplacement(al workflowAlias) string {
	parts := []string{"oh run", al.Workflow}
	if len(al.Opts.Tickets) > 0 {
		parts = append(parts, "--tickets", strings.Join(al.Opts.Tickets, ","))
	}
	for _, k := range sortedStringKeys(al.Opts.LooseInputs) {
		if v := al.Opts.LooseInputs[k]; v != "" {
			parts = append(parts, "-i", k+"="+shellQuote(v))
		}
	}
	if al.Opts.Location != "" {
		parts = append(parts, "--location", al.Opts.Location)
	}
	return strings.Join(parts, " ")
}

func sortedStringKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func shellQuote(s string) string {
	if strings.ContainsAny(s, " \t\"'$`\\") {
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}
	return s
}
