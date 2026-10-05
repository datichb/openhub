package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/buildinfo"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Démon oh (proxy d'identifiants, supervision des sessions)",
}

var daemonRunCmd = &cobra.Command{
	Use:    "run",
	Short:  "Exécute le démon au premier plan (lancé automatiquement par oh)",
	Hidden: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		a := MustApp()
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		err := daemon.Run(ctx, daemon.Options{
			Paths:   daemon.Paths{Dir: ohRunDir()},
			Version: buildinfo.Version,
			Grants:  sqlite.NewGrantStore(store),
			Servers: sqlite.NewServerStore(store),
			Secrets: a.Secrets,
		})
		if errors.Is(err, daemon.ErrAlreadyRunning) {
			fmt.Fprintln(cmd.ErrOrStderr(), i18n.T("cmd.daemon.already_running"))
			return nil
		}
		return err
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Affiche l'état du démon oh",
	RunE: func(cmd *cobra.Command, _ []string) error {
		c := daemon.NewClient(daemon.Paths{Dir: ohRunDir()})
		h, err := c.Health(cmd.Context())
		if errors.Is(err, daemon.ErrNotRunning) {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T("cmd.daemon.not_running"))
			return nil
		}
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf("cmd.daemon.status_line", h.Version, h.PID, h.ProxyURL, h.Servers, h.Grants, h.PendingGrants))
		return nil
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Arrête le démon oh (refusé si des sessions tournent, sauf --force)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		force, _ := cmd.Flags().GetBool("force")
		c := daemon.NewClient(daemon.Paths{Dir: ohRunDir()})
		err := c.Shutdown(cmd.Context(), force)
		if errors.Is(err, daemon.ErrNotRunning) {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T("cmd.daemon.not_running"))
			return nil
		}
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T("cmd.daemon.stopping"))
		return nil
	},
}

func init() {
	daemonStopCmd.Flags().Bool("force", false, "Arrêter même si des sessions tournent")
	daemonCmd.AddCommand(daemonRunCmd, daemonStatusCmd, daemonStopCmd)
	rootCmd.AddCommand(daemonCmd)
}
