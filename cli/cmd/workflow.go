package cmd

import (
	"github.com/spf13/cobra"
)

// workflowCmd groups the workflow commands.
var workflowCmd = &cobra.Command{
	Use: "workflow",
}

func init() {
	rootCmd.AddCommand(workflowCmd)
	workflowCmd.AddCommand(workflowValidateCmd())
}
