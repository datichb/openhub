package cmd

import (
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// `oh mr create` publishes the current branch as a GitLab merge request. It
// is the replacement of `oh review --publish` (U-02): the review itself is a
// workflow (`oh run review`), the publication is not, and it must not live
// under a deprecated command.

var mrCmd = &cobra.Command{
	Use:   "mr",
	Short: i18n.T("cmd.mr.short"),
	Long:  i18n.T("cmd.mr.long"),
}

var mrCreateCmd = &cobra.Command{
	Use:   "create",
	Short: i18n.T("cmd.mr.create.short"),
	Long:  i18n.T("cmd.mr.create.long"),
	Args:  cobra.NoArgs,
	RunE:  func(cmd *cobra.Command, _ []string) error { return runReviewPublish(cmd) },
}

func init() {
	rootCmd.AddCommand(mrCmd)
	mrCmd.AddCommand(mrCreateCmd)
	mrCreateCmd.Flags().StringP("project", "p", "", i18n.T("cmd.mr.create.flags.project"))
	mrCreateCmd.Flags().String("reviewer", "", i18n.T("cmd.mr.create.flags.reviewer"))
	_ = mrCreateCmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
}
