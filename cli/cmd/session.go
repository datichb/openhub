package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/adapters/opencodev2"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/termlaunch"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Gestion des sessions agentiques (v5)",
}

var sessionAttachCmd = &cobra.Command{
	Use:   "attach <session-id>",
	Short: "Ouvre l'interface d'une session (nouvel onglet/fenêtre, navigateur ou terminal courant)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		svc, err := newRunService(ctx, MustApp())
		if err != nil {
			return err
		}
		id := args[0]
		if execHere, _ := cmd.Flags().GetBool("exec"); execHere {
			return execAttach(ctx, svc.AttachCommand, id)
		}
		how, _ := cmd.Flags().GetString("how")
		style, _ := cmd.Flags().GetString("iterm-style")
		switch how {
		case "suspend", "here":
			return execAttach(ctx, svc.AttachCommand, id)
		case "browser":
			url, err := svc.PairURL(ctx, id, pairURL)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), url)
			return openURL(url)
		}
		dir, _ := os.Getwd()
		m, err := svc.Attach(ctx, id, dir, termlaunch.Pref(how), termlaunch.ITermStyle(style), "")
		if errors.Is(err, termlaunch.ErrNoTerminal) {
			fmt.Fprintln(cmd.ErrOrStderr(), i18n.T("cmd.session.no_terminal"))
			return execAttach(ctx, svc.AttachCommand, id)
		}
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf("cmd.session.opened_in", string(m)))
		return nil
	},
}

// execAttach replaces the current process with the tool client attached to the session.
func execAttach(ctx context.Context, command func(context.Context, string) ([]string, []string, error), id string) error {
	argv, env, err := command(ctx, id)
	if err != nil {
		return err
	}
	bin, err := exec.LookPath(argv[0])
	if err != nil {
		return err
	}
	return execReplaceProcess(bin, argv, append(os.Environ(), env...))
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
	sessionAttachCmd.Flags().Bool("exec", false, "Remplace le processus courant par l'interface (usage interne)")
	sessionAttachCmd.Flags().String("how", "auto", "auto | iterm | terminal | tmux | browser | suspend")
	sessionAttachCmd.Flags().String("iterm-style", "tab", "tab | split | window")
	_ = sessionAttachCmd.Flags().MarkHidden("exec")
	sessionCmd.AddCommand(sessionAttachCmd)
	rootCmd.AddCommand(sessionCmd)
}
