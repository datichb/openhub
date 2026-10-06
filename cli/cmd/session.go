package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/adapters/opencodev2"
	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/runsvc"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	"github.com/datichb/openhub/cli/internal/termlaunch"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Gestion des sessions agentiques (v5)",
}

// resolveSessionRef turns an ID or unique ID prefix into a session ID.
func resolveSessionRef(ctx context.Context, ref string) (string, error) {
	svc, err := newSessionService(ctx, MustApp())
	if err != nil {
		return "", err
	}
	sess, err := svc.Resolve(ctx, ref)
	if err != nil {
		return "", err
	}
	return sess.ID, nil
}

var sessionAttachCmd = &cobra.Command{
	Use:   "attach <session-id>",
	Short: "Ouvre l'interface d'une session (nouvel onglet/fenêtre, tmux, navigateur ou terminal courant) ; reprend une session en veille",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		a := MustApp()
		svc, err := newRunService(ctx, a)
		if err != nil {
			return err
		}
		id, err := resolveSessionRef(ctx, args[0])
		if err != nil {
			return err
		}
		if execHere, _ := cmd.Flags().GetBool("exec"); execHere {
			return runAttachChild(ctx, a, svc, id)
		}
		how, _ := cmd.Flags().GetString("how")
		if how == "suspend" || how == "here" {
			return runAttachChild(ctx, a, svc, id)
		}
		if err := ensureSessionAwake(ctx, cmd, a, svc, id); err != nil {
			return err
		}
		if how == "browser" {
			return openSessionInBrowser(ctx, cmd, svc, id, false)
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

// ensureSessionAwake resumes a sleeping session (its server is restarted).
func ensureSessionAwake(ctx context.Context, cmd *cobra.Command, a *app.App, svc *runsvc.Service, id string) error {
	if _, _, err := svc.AttachCommand(ctx, id); errors.Is(err, runsvc.ErrServerNotRunning) {
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T("cmd.session.resuming"))
		return resumeV5Session(ctx, a, svc, id)
	} else if err != nil {
		return err
	}
	return nil
}

func openSessionInBrowser(ctx context.Context, cmd *cobra.Command, svc *runsvc.Service, id string, printOnly bool) error {
	url, err := svc.PairURL(ctx, id, pairURL)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), url)
	if printOnly {
		return nil
	}
	return openURL(url)
}

// sessionJSON is the `oh session list --json` item.
type sessionJSON struct {
	ID         string         `json:"id"`
	ProjectID  string         `json:"project_id"`
	Project    string         `json:"project,omitempty"`
	Title      string         `json:"title,omitempty"`
	Workflow   string         `json:"workflow,omitempty"`
	Agent      string         `json:"agent,omitempty"`
	Mode       string         `json:"mode,omitempty"`
	State      string         `json:"state"`
	Location   string         `json:"location,omitempty"`
	Group      string         `json:"group"`
	Cost       float64        `json:"cost"`
	TokensIn   int64          `json:"tokens_in"`
	TokensOut  int64          `json:"tokens_out"`
	StartedAt  time.Time      `json:"started_at"`
	ChangedAt  *time.Time     `json:"state_changed_at,omitempty"`
	Decisions  []decisionJSON `json:"decisions"`
	EndedAt    *time.Time     `json:"ended_at,omitempty"`
	BundleHash string         `json:"bundle,omitempty"`
}

type decisionJSON struct {
	ID        string                 `json:"id"`
	SessionID string                 `json:"session_id"`
	Kind      string                 `json:"kind"`
	Summary   string                 `json:"summary"`
	Payload   domain.DecisionPayload `json:"payload"`
	CreatedAt time.Time              `json:"created_at"`
}

func toDecisionJSON(d domain.Decision) decisionJSON {
	return decisionJSON{ID: d.ID, SessionID: d.SessionID, Kind: string(d.Kind), Summary: sessionsvc.DecisionSummary(d), Payload: d.Payload, CreatedAt: d.CreatedAt}
}

func printJSON(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "Liste les sessions v5 (en cours, en attente, en veille ; --all pour les terminées)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		trackRemoteSessions(ctx, MustApp())
		svc, err := newSessionService(ctx, MustApp())
		if err != nil {
			return err
		}
		all, _ := cmd.Flags().GetBool("all")
		views, err := svc.List(ctx, sessionsvc.ListFilter{All: all})
		if err != nil {
			return err
		}
		if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
			out := make([]sessionJSON, 0, len(views))
			for _, v := range views {
				s := v.Session
				j := sessionJSON{ID: s.ID, ProjectID: s.ProjectID, Project: v.ProjectName, Workflow: s.WorkflowID, Agent: s.EntryAgent,
					Mode: s.Mode, State: string(s.State), Location: s.LaunchPath, Group: s.GroupKey, Cost: s.Cost, TokensIn: s.TokensIn,
					TokensOut: s.TokensOut, StartedAt: s.StartedAt, ChangedAt: s.StateChangedAt, EndedAt: s.EndedAt, BundleHash: s.BundleHash,
					Decisions: []decisionJSON{}}
				if s.Title != nil {
					j.Title = *s.Title
				}
				for _, d := range v.Decisions {
					j.Decisions = append(j.Decisions, toDecisionJSON(d))
				}
				out = append(out, j)
			}
			return printJSON(cmd, out)
		}
		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
		fmt.Fprintln(tw, i18n.T("cmd.session.list_header"))
		for _, v := range views {
			s := v.Session
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s %s\t%s\t$%.3f\t%s\n", s.ID, v.ProjectName, s.EntryAgent, sessionsvc.StateIcon(s.State), stateLabel(s.State),
				dash(sessionsvc.DecisionBadges(v.Decisions)), s.Cost, s.StartedAt.Format("02/01 15:04"))
		}
		_ = tw.Flush()
		if len(views) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T("cmd.session.list_empty"))
		}
		return nil
	},
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

var sessionStopCmd = &cobra.Command{
	Use:   "stop <session-id>",
	Short: "Arrête une session (et son serveur si plus aucune autre session ne l'utilise)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		svc, err := newRunService(ctx, MustApp())
		if err != nil {
			return err
		}
		id, err := resolveSessionRef(ctx, args[0])
		if err != nil {
			return err
		}
		if err := svc.StopSession(ctx, id); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf("cmd.session.stopped", id))
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
	sessionListCmd.Flags().Bool("json", false, "Sortie JSON (sessions et décisions en attente)")
	sessionCmd.AddCommand(sessionAttachCmd, sessionListCmd, sessionStopCmd)
	rootCmd.AddCommand(sessionCmd)
}
