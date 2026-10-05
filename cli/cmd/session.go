package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/adapters/opencodev2"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/termlaunch"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Gestion des sessions agentiques (v5)",
}

var sessionAttachCmd = &cobra.Command{
	Use:   "attach <session-id>",
	Short: "Ouvre l'interface d'une session (nouvel onglet/fenêtre, navigateur ou terminal courant) ; reprend une session en veille",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		a := MustApp()
		svc, err := newRunService(ctx, a)
		if err != nil {
			return err
		}
		id := args[0]
		if execHere, _ := cmd.Flags().GetBool("exec"); execHere {
			return runAttachChild(ctx, a, svc, id)
		}
		how, _ := cmd.Flags().GetString("how")
		if how == "suspend" || how == "here" {
			return runAttachChild(ctx, a, svc, id)
		}
		// Wake a sleeping session before opening a new terminal on it.
		if _, _, err := svc.AttachCommand(ctx, id); errors.Is(err, runsvc.ErrServerNotRunning) {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T("cmd.session.resuming"))
			if err := resumeV5Session(ctx, a, svc, id); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if how == "browser" {
			url, err := svc.PairURL(ctx, id, pairURL)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), url)
			return openURL(url)
		}
		style, _ := cmd.Flags().GetString("iterm-style")
		dir, _ := os.Getwd()
		if sess, err := svc.Session(ctx, id); err == nil && sess.LaunchPath != "" {
			dir = sess.LaunchPath
		}
		m, err := svc.Attach(ctx, id, dir, termlaunch.Pref(how), termlaunch.ITermStyle(style), "")
		if errors.Is(err, termlaunch.ErrNoTerminal) {
			fmt.Fprintln(cmd.ErrOrStderr(), i18n.T("cmd.session.no_terminal"))
			return runAttachChild(ctx, a, svc, id)
		}
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf("cmd.session.opened_in", string(m)))
		return nil
	},
}

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "Liste les sessions v5 (en cours, en attente, en veille, terminées)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		a := MustApp()
		all, _ := cmd.Flags().GetBool("all")
		sessions, err := a.Sessions.List(cmd.Context(), "")
		if err != nil {
			return err
		}
		names := map[string]string{}
		if projects, err := a.Projects.List(cmd.Context(), ""); err == nil {
			for _, p := range projects {
				names[p.ID] = p.Name
			}
		}
		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
		fmt.Fprintln(tw, i18n.T("cmd.session.list_header"))
		n := 0
		for _, s := range sessions {
			if s.GroupKey == "" {
				continue // legacy (pre-v5) session
			}
			if !all && (s.State == domain.RunStopped || s.State == domain.RunCompleted || s.State == domain.RunFailed) {
				continue
			}
			n++
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t$%.3f\t%s\n", s.ID, names[s.ProjectID], s.EntryAgent, stateLabel(s.State), s.Cost, s.StartedAt.Format("02/01 15:04"))
		}
		_ = tw.Flush()
		if n == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T("cmd.session.list_empty"))
		}
		return nil
	},
}

var sessionStopCmd = &cobra.Command{
	Use:   "stop <session-id>",
	Short: "Arrête une session (et son serveur si plus aucune autre session ne l'utilise)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newRunService(cmd.Context(), MustApp())
		if err != nil {
			return err
		}
		if err := svc.StopSession(cmd.Context(), args[0]); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf("cmd.session.stopped", args[0]))
		return nil
	},
}

func stateLabel(s domain.RunState) string {
	if s == "" {
		return "-"
	}
	if t := i18n.T("cmd.session.state." + string(s)); t != "cmd.session.state."+string(s) {
		return t
	}
	return string(s)
}

func pairURL(ctx context.Context, url, password string) (string, error) {
	c := opencodev2.NewClient(url, password)
	code, err := c.Pair(ctx)
	if err != nil {
		return "", err
	}
	return c.PairURL(code.Code), nil
}

func openURL(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

func init() {
	sessionAttachCmd.Flags().Bool("exec", false, "Exécute l'interface dans le terminal courant (usage interne)")
	sessionAttachCmd.Flags().String("how", "auto", "auto | iterm | terminal | tmux | browser | suspend")
	sessionAttachCmd.Flags().String("iterm-style", "tab", "tab | split | window")
	_ = sessionAttachCmd.Flags().MarkHidden("exec")
	sessionListCmd.Flags().Bool("all", false, "Inclure les sessions terminées")
	sessionCmd.AddCommand(sessionAttachCmd, sessionListCmd, sessionStopCmd)
	rootCmd.AddCommand(sessionCmd)
}
