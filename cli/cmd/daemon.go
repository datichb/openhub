package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/adapters/opencodev2"
	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/buildinfo"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/sysnotify"
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
		capability, _, err := daemonCapability(ctx)
		if err != nil {
			return err
		}
		err = daemon.Run(ctx, daemonOptions(ctx, a, capability))
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
		capability, _, err := daemonCapability(cmd.Context())
		if err != nil {
			return err
		}
		c := daemon.NewClient(daemon.Paths{Dir: ohRunDir()}).WithCapability(capability)
		err = c.Shutdown(cmd.Context(), force)
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

// daemonNotifier returns the desktop notifier of the daemon ([session] notify).
func daemonNotifier(a *app.App) daemon.NotifyFunc {
	if !a.Config.Session.NotifyEnabled() {
		return nil
	}
	n := sysnotify.New()
	return func(ctx context.Context, title, message string) error {
		return n.Notify(ctx, sysnotify.Note{Title: title, Message: message, Group: "oh-sessions", Sound: true})
	}
}

func init() {
	daemonStopCmd.Flags().Bool("force", false, "Arrêter même si des sessions tournent")
	daemonCmd.AddCommand(daemonRunCmd, daemonStatusCmd, daemonStopCmd)
	rootCmd.AddCommand(daemonCmd)
}

// daemonOptions are the options of the oh daemon of this machine (`oh
// daemon run`, or inside the oh process when the daemon cannot run in the
// background: Windows).
func daemonOptions(ctx context.Context, a *app.App, capability string) daemon.Options {
	var (
		adOnce sync.Once
		ad     *opencodev2.Adapter
	)
	return daemon.Options{
		Capability:  capability,
		Paths:       daemon.Paths{Dir: ohRunDir()},
		Version:     buildinfo.Version,
		Grants:      sqlite.NewGrantStore(store),
		Servers:     sqlite.NewServerStore(store),
		GatewayView: gatewayView(sqlite.NewServerStore(store)),
		BeadsBinary: realBeadsBinary(),
		MCPCommand:  gatewayMCPCommand(ohBundlesDir()),
		Sessions:    a.Sessions,
		Decisions:   sqlite.NewDecisionStore(store),
		Usage:       sqlite.NewUsageStore(store),
		SessionsDir: ohSessionsDir(),
		ServersDir:  ohServersDir(),
		Checkpoints: newCheckpointService(a),
		Notify:      daemonNotifier(a),
		ProjectName: func(ctx context.Context, id string) string {
			if p, err := a.Projects.Get(ctx, id); err == nil {
				return p.Name
			}
			return ""
		},
		Secrets:   a.Secrets,
		Teardown:  daemonTeardown(a),
		Periodic:  daemonRemoteTracking(a),
		IdleSleep: time.Duration(a.Config.Session.IdleSleepMinutes) * time.Minute,
		// Async: a git push must not stall supervision (the daemon
		// outlives the last server by IdleAfter).
		OnSessionEnd: sessionEndHook(a, true),
		Adapter: func(name string) adapters.ToolAdapter {
			if name != opencodev2.Name {
				return nil
			}
			adOnce.Do(func() {
				if detected, err := detectV2Adapter(ctx); err == nil {
					ad = detected
				}
			})
			if ad == nil {
				return nil
			}
			return ad
		},
	}
}
