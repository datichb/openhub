package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/i18n"
)

var beadsCmd = &cobra.Command{
	Use:                "beads",
	Short:              "Gestion des tickets beads (proxy vers bd)",
	DisableFlagParsing: true,
	Args:               cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := exec.LookPath("bd"); err != nil {
			return fmt.Errorf("%s", i18n.T("cmd.beads.bd_not_installed"))
		}
		// Intercept "init" subcommand to enforce zero-impact flags.
		// This prevents bd from installing hooks, generating agent files,
		// or modifying .gitignore even when called via the proxy.
		if len(args) > 0 && args[0] == "init" {
			args = beads.EnsureInitFlags(args)
		}
		// Delegate to bd command — all args/flags are passed through
		bdCmd := exec.Command("bd", args...)
		bdCmd.Stdin = os.Stdin
		bdCmd.Stdout = os.Stdout
		bdCmd.Stderr = os.Stderr
		return bdExit(bdCmd.Run())
	},
}

// bdExit passes the exit code of bd through (bd printed its own error).
func bdExit(err error) error {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return &ExitError{Code: exit.ExitCode()}
	}
	return err
}

func init() {
	rootCmd.AddCommand(withInitLevel(beadsCmd, levelNone))
}
