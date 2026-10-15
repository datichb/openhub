package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/workflow"
)

func brickShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <id>|agent:<id>|skill:<ref>",
		Short: i18n.T("cmd.brick.show.short"),
		Long:  i18n.T("cmd.brick.show.long"),
		Args:  cobra.ExactArgs(1),
		RunE:  runBrickShow,
	}
	cmd.Flags().Bool("raw", false, i18n.T("cmd.brick.show.flags.raw"))
	cmd.Flags().Bool("json", false, i18n.T("cmd.brick.show.flags.json"))
	addWorkflowContextFlags(cmd)
	return cmd
}

// brickShowJSON is the JSON form of `oh brick show`.
type brickShowJSON struct {
	Ref         string                  `json:"ref"`
	Origin      string                  `json:"origin"`
	Path        string                  `json:"path"`
	Draft       string                  `json:"draft,omitempty"`
	Entry       *workflowsvc.BrickEntry `json:"entry,omitempty"`
	Diagnostics workflow.Diagnostics    `json:"diagnostics"`
}

func runBrickShow(cmd *cobra.Command, args []string) error {
	c, err := workflowReadContext(cmd)
	if err != nil {
		return err
	}
	svc := newWorkflowService(cmd.Context())
	errOut, out := cmd.ErrOrStderr(), cmd.OutOrStdout()
	t, err := svc.BrickText(ctxOf(cmd), c, args[0])
	if err != nil {
		return brickError(errOut, err)
	}
	if raw, _ := cmd.Flags().GetBool("raw"); raw {
		_, err := out.Write(t.Data)
		return err
	}
	ws, err := svc.BrickWorkspace(ctxOf(cmd), c)
	if err != nil {
		return brickError(errOut, err)
	}
	res := brickShowJSON{Ref: t.Ref.String(), Origin: t.Origin, Path: t.Path, Diagnostics: workflow.Diagnostics{}}
	if t.Origin == "draft" {
		res.Draft = t.Path
	}
	for i, e := range ws.Bricks {
		if e.Kind == t.Ref.Kind && (brickID(e) == t.Ref.ID || (e.TeamID == "" && e.ID == t.Ref.ID)) {
			res.Entry = &ws.Bricks[i]
			break
		}
	}
	if res.Entry != nil && res.Entry.TeamID != "" {
		res.Diagnostics = append(res.Diagnostics, ws.Problems[t.Ref.String()]...)
	}
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	printBrickShow(out, res)
	return nil
}

func printBrickShow(w io.Writer, r brickShowJSON) {
	line := func(label, value string) {
		if value == "" {
			value = "—"
		}
		fmt.Fprintf(w, "  %-14s %s\n", label, value)
	}
	title := r.Ref
	e := r.Entry
	if e != nil && e.Label != "" {
		title += " · " + e.Label
	}
	fmt.Fprintln(w, theme.Title.Render(title))
	if e != nil && e.Description != "" {
		fmt.Fprintln(w, theme.Subtitle.Render(e.Description))
	}
	if e != nil {
		line(i18n.T("cmd.brick.show.origin"), brickOrigin(*e))
		line(i18n.T("cmd.brick.show.file"), r.Path)
		if e.Version > 0 {
			line(i18n.T("cmd.brick.show.version"), fmt.Sprintf("v%d", e.Version))
		}
		if e.Kind == workflowsvc.BrickAgent {
			line(i18n.T("cmd.brick.show.family"), e.Family)
			line(i18n.T("cmd.brick.show.mode"), e.Mode)
			line(i18n.T("cmd.brick.show.skills"), strings.Join(e.Skills, ", "))
		} else {
			line(i18n.T("cmd.brick.show.requires"), strings.Join(e.Requires, ", "))
			line(i18n.T("cmd.brick.show.agents"), strings.Join(e.Agents, ", "))
		}
		line(i18n.T("cmd.brick.show.workflows"), strings.Join(e.Workflows, ", "))
		line(i18n.T("cmd.brick.show.tokens"), fmt.Sprintf("≈ %d", e.Tokens))
	} else {
		line(i18n.T("cmd.brick.show.origin"), r.Origin)
		line(i18n.T("cmd.brick.show.file"), r.Path)
	}
	if r.Draft != "" {
		fmt.Fprintf(w, "  %s\n", theme.Subtitle.Render(i18n.Tf("cmd.brick.show.draft", r.Draft)))
	}
	if len(r.Diagnostics) > 0 {
		fmt.Fprintln(w)
		printDiagnostics(w, r.Diagnostics)
	}
}
