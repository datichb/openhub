package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/launcher"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// brickCmd groups the commands of the team brick catalogue (ADR-054):
// agents and skills of catalog/ in the team-state, edited as drafts and
// published like the workflows.
var brickCmd = &cobra.Command{
	Use:   "brick",
	Short: i18n.T("cmd.brick.short"),
	Long:  i18n.T("cmd.brick.long"),
}

func init() {
	rootCmd.AddCommand(brickCmd)
	brickCmd.AddCommand(brickListCmd(), brickShowCmd(), brickNewCmd(), brickEditCmd(), brickPublishCmd(), brickHistoryCmd())
}

// brickError explains the errors of the brick service.
func brickError(w io.Writer, err error) error {
	var invalid *workflowsvc.InvalidError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &invalid):
		printDiagnostics(w, invalid.Diagnostics)
		return errors.New(i18n.Tf("cmd.brick.invalid", invalid.Ref, len(invalid.Diagnostics.Errors())))
	case errors.Is(err, workflowsvc.ErrNoTeamState):
		return errors.New(i18n.T("teamstate.workflow.cli.no_team_state"))
	case errors.Is(err, workflowsvc.ErrHubReadOnly):
		return errors.New(i18n.T("cmd.brick.hub_read_only"))
	case errors.Is(err, workflowsvc.ErrNoDraft):
		return errors.New(i18n.Tf("cmd.brick.no_draft", errDetail(err, workflowsvc.ErrNoDraft)))
	case errors.Is(err, workflowsvc.ErrUnknownBrick):
		return errors.New(i18n.Tf("cmd.brick.unknown", errDetail(err, workflowsvc.ErrUnknownBrick)))
	case errors.Is(err, workflowsvc.ErrAmbiguousBrick):
		return errors.New(i18n.Tf("cmd.brick.ambiguous", errDetail(err, workflowsvc.ErrAmbiguousBrick)))
	case errors.Is(err, workflowsvc.ErrBrickExists):
		return errors.New(i18n.Tf("cmd.brick.exists", errDetail(err, workflowsvc.ErrBrickExists)))
	case errors.Is(err, workflowsvc.ErrBrickInHub):
		return errors.New(i18n.Tf("cmd.brick.in_hub", errDetail(err, workflowsvc.ErrBrickInHub)))
	case errors.Is(err, workflowsvc.ErrBrickNotInHub):
		return errors.New(i18n.Tf("cmd.brick.not_in_hub", errDetail(err, workflowsvc.ErrBrickNotInHub)))
	case errors.Is(err, workflowsvc.ErrBricksUnsealed):
		return errors.New(i18n.Tf("cmd.brick.unsealed", errDetail(err, workflowsvc.ErrBricksUnsealed)))
	case errors.Is(err, workflowsvc.ErrBricksSealed):
		return errors.New(i18n.T("cmd.brick.sealed"))
	case errors.Is(err, workflowsvc.ErrNotPublished):
		return errors.New(i18n.Tf("cmd.brick.not_published", errDetail(err, workflowsvc.ErrNotPublished)))
	case errors.Is(err, teamstate.ErrInvalidBrickID):
		return errors.New(i18n.Tf("cmd.brick.bad_id", errDetail(err, teamstate.ErrInvalidBrickID)))
	case errors.Is(err, teamstate.ErrOffline):
		return errors.New(i18n.T("cmd.brick.offline"))
	case errors.Is(err, teamstate.ErrPushRejected):
		return errors.New(i18n.T("teamstate.workflow.cli.push_rejected"))
	}
	return err
}

// errDetail is the text an error adds to its sentinel ("<sentinel>: <detail>").
func errDetail(err, sentinel error) string {
	return strings.TrimPrefix(err.Error(), sentinel.Error()+": ")
}

// saveBrickEdit opens the editor when asked, validates and saves the draft;
// on validation errors it offers to edit again (interactive) or keeps the
// edited file and fails.
func saveBrickEdit(cmd *cobra.Command, svc *workflowsvc.Service, c workflowsvc.Context, in workflowsvc.BrickInput, edit bool) (*workflowsvc.BrickDraft, error) {
	original := append([]byte(nil), in.Data...)
	name := in.Ref.ID[strings.LastIndex(in.Ref.ID, "/")+1:] + ".md"
	for {
		var path string
		if edit {
			data, p, err := editText(name, in.Data)
			if err != nil {
				return nil, err
			}
			in.Data, path = data, p
			if bytes.Equal(in.Data, original) {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.T("teamstate.workflow.cli.unchanged"))
			}
		}
		d, err := svc.SaveBrickDraft(ctxOf(cmd), c, in)
		var invalid *workflowsvc.InvalidError
		if err == nil || !errors.As(err, &invalid) || !edit || !interactive() {
			if err != nil && path != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), theme.Subtitle.Render(i18n.Tf("teamstate.workflow.cli.kept", path)))
			}
			return d, err
		}
		printDiagnostics(cmd.ErrOrStderr(), invalid.Diagnostics)
		again, cerr := launcher.NewCLIUI(cmd.ErrOrStderr()).Confirm(i18n.T("teamstate.workflow.cli.edit_again"))
		if cerr != nil || !again {
			fmt.Fprintln(cmd.ErrOrStderr(), theme.Subtitle.Render(i18n.Tf("teamstate.workflow.cli.kept", path)))
			return nil, err
		}
	}
}

// printBrickDraft reports a saved draft brick.
func printBrickDraft(w io.Writer, d *workflowsvc.BrickDraft) {
	printDiagnostics(w, d.Diagnostics)
	fmt.Fprintf(w, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.brick.draft_saved", d.Ref.String(), d.Path))
	if !d.Pushed {
		fmt.Fprintf(w, "  %s\n", theme.WarningStyle.Render(i18n.Tf("teamstate.workflow.cli.draft_not_pushed", d.PushError)))
	}
	fmt.Fprintf(w, "  %s\n", theme.Subtitle.Render(i18n.Tf("cmd.brick.draft_next", d.Ref.String())))
}
