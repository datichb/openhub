package cmd

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// Aliases of the former launch commands (O15): `oh start`, `--agent`,
// `--dev`, `--onboard`, `--parallel`, `--sweep`, `oh audit|review|debug`,
// `oh review feedback`, `oh takeover-brief enrich` run their workflow
// through `oh run`, with a deprecation warning. They require opencode V2 and
// the target workflow (the former launch was removed with opencode V1).

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

// runAlias warns and runs the workflow of an alias.
// The target workflow must be in the catalogue (the former launch is gone).
func runAlias(cmd *cobra.Command, al workflowAlias) error {
	if err := requireV2(cmd.Context()); err != nil {
		return err
	}
	if !newWorkflowService(cmd.Context()).Has(cmd.Context(), workflowsvc.Context{}, al.Workflow) {
		return errors.New(i18n.Tf("cmd.v1.unsupported.workflow_missing", al.Old, al.Workflow))
	}
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
	if al.Opts.Agent != "" {
		parts = append(parts, "--agent", al.Opts.Agent)
	}
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
