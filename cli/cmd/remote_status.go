package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
)

func init() {
	remoteCmd.AddCommand(&cobra.Command{
		Use:   "status [target]",
		Short: i18n.T("cmd.remote.status.short"),
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()
			out := cmd.OutOrStdout()
			targets := a.Config.Remote.Targets
			if len(args) == 1 {
				t := a.Config.Remote.Target(args[0])
				if t == nil {
					return errors.New(i18n.Tf("cmd.remote.status.unknown", args[0]))
				}
				targets = []config.RemoteTarget{*t}
			}
			if len(targets) == 0 {
				fmt.Fprintln(out, i18n.T("cmd.remote.status.none"))
				return nil
			}
			svc := newRemoteService(a)
			failed := false
			for i, t := range targets {
				if i > 0 {
					fmt.Fprintln(out)
				}
				rep := svc.Check(ctxOf(cmd), t)
				printRemoteReport(out, rep)
				failed = failed || !rep.OK()
			}
			if failed {
				return errors.New(i18n.T("cmd.remote.status.failed"))
			}
			return nil
		},
	})
}
