package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

func brickListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: i18n.T("cmd.brick.list.short"),
		Long:  i18n.T("cmd.brick.list.long"),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			agents, _ := cmd.Flags().GetBool("agents")
			skills, _ := cmd.Flags().GetBool("skills")
			if agents && skills {
				return errors.New(i18n.T("cmd.brick.list.agents_and_skills"))
			}
			c, err := workflowReadContext(cmd)
			if err != nil {
				return err
			}
			ws, err := newWorkflowService(cmd.Context()).BrickWorkspace(ctxOf(cmd), c)
			if err != nil {
				return brickError(cmd.ErrOrStderr(), err)
			}
			kind := workflowsvc.BrickKind("")
			switch {
			case agents:
				kind = workflowsvc.BrickAgent
			case skills:
				kind = workflowsvc.BrickSkill
			}
			filterBricks(ws, kind)
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(ws)
			}
			origin, _ := cmd.Flags().GetBool("origin")
			return printBrickList(cmd.OutOrStdout(), ws, origin)
		},
	}
	f := cmd.Flags()
	f.Bool("agents", false, i18n.T("cmd.brick.list.flags.agents"))
	f.Bool("skills", false, i18n.T("cmd.brick.list.flags.skills"))
	f.Bool("origin", false, i18n.T("cmd.brick.list.flags.origin"))
	f.Bool("json", false, i18n.T("cmd.brick.list.flags.json"))
	addWorkflowContextFlags(cmd)
	return cmd
}

// filterBricks keeps the bricks and drafts of kind ("" = all).
func filterBricks(ws *workflowsvc.BrickWorkspace, kind workflowsvc.BrickKind) {
	if kind == "" {
		return
	}
	bricks := ws.Bricks[:0]
	for _, b := range ws.Bricks {
		if b.Kind == kind {
			bricks = append(bricks, b)
		}
	}
	ws.Bricks = bricks
	drafts := ws.Drafts[:0]
	for _, d := range ws.Drafts {
		if d.Kind == string(kind) {
			drafts = append(drafts, d)
		}
	}
	ws.Drafts = drafts
}

// brickOrigin is the origin column of a brick: hub, team, or team
// replacing a hub brick.
func brickOrigin(e workflowsvc.BrickEntry) string {
	o := i18n.T("cmd.brick.origin." + string(e.Origin))
	if e.Shadows != "" {
		o += " " + i18n.Tf("cmd.brick.origin.replaces", e.Shadows)
	}
	return o
}

func brickID(e workflowsvc.BrickEntry) string {
	if e.TeamID != "" {
		return e.TeamID
	}
	return e.ID
}

func printBrickList(w io.Writer, ws *workflowsvc.BrickWorkspace, withOrigin bool) error {
	if len(ws.Bricks) == 0 && len(ws.Drafts) == 0 {
		fmt.Fprintln(w, i18n.T("cmd.brick.list.empty"))
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	header := []string{i18n.T("cmd.brick.list.col_id"), i18n.T("cmd.brick.list.col_kind"), i18n.T("cmd.brick.list.col_origin"),
		i18n.T("cmd.brick.list.col_version")}
	if withOrigin {
		header = append(header, i18n.T("cmd.brick.list.col_source"))
	}
	header = append(header, i18n.T("cmd.brick.list.col_description"))
	fmt.Fprintf(tw, "  %s\n", strings.Join(header, "\t"))
	invalid := 0
	for _, e := range ws.Bricks {
		icon := theme.SuccessStyle.Render(theme.IconSuccess)
		switch {
		case e.Errors > 0:
			icon = theme.ErrorStyle.Render(theme.IconError)
			invalid++
		case e.Warnings > 0:
			icon = theme.WarningStyle.Render(theme.IconWarning)
		}
		version := "—"
		if e.Version > 0 {
			version = fmt.Sprintf("v%d", e.Version)
		}
		cols := []string{icon + " " + brickID(e), string(e.Kind), brickOrigin(e), version}
		if withOrigin {
			src := e.Source
			if src == "" {
				src = "hub"
			}
			cols = append(cols, src)
		}
		desc := e.Description
		if desc == "" {
			desc = e.Label
		}
		cols = append(cols, truncate(desc, 80))
		fmt.Fprintln(tw, strings.Join(cols, "\t"))
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if len(ws.Drafts) > 0 {
		fmt.Fprintf(w, "\n%s\n", i18n.Tf("cmd.brick.list.drafts", ws.Member))
		for _, d := range ws.Drafts {
			fmt.Fprintf(w, "  ✎ %s:%s  %s\n", d.Kind, d.ID, theme.Subtitle.Render(d.Rel))
		}
	}
	if invalid > 0 {
		fmt.Fprintln(w, theme.Subtitle.Render(i18n.Tf("cmd.brick.list.invalid_hint", invalid)))
	}
	if len(ws.Unsealed) > 0 {
		fmt.Fprintf(w, "\n%s %s\n", theme.WarningStyle.Render(theme.IconWarning), i18n.Tf("cmd.brick.list.unsealed", len(ws.Unsealed)))
	}
	if len(ws.Integrity) > 0 {
		fmt.Fprintf(w, "\n%s %s\n", theme.WarningStyle.Render(theme.IconWarning), i18n.Tf("cmd.brick.list.integrity", len(ws.Integrity)))
		for _, d := range ws.Integrity {
			fmt.Fprintf(w, "  · %s\n", d.Message)
		}
	}
	return nil
}
