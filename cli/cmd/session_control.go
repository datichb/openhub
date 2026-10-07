package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
)

// v5 session control from the CLI (P3-T15): inbox, decisions, instructions,
// live follow-up, results. All the logic lives in the SessionService.

func sessionService(cmd *cobra.Command) (*sessionsvc.Service, error) {
	return newSessionService(cmd.Context(), MustApp())
}

// decisionError turns service errors into user messages.
func decisionError(err error) error {
	var re *sessionsvc.ResolvedError
	var several *sessionsvc.ErrSeveralDecisions
	switch {
	case errors.As(err, &re):
		by := re.By
		if by == "" {
			by = domain.ResolvedByTool
		}
		return errors.New(i18n.T("cmd.session.resolved_by." + string(by)))
	case errors.As(err, &several):
		var b strings.Builder
		b.WriteString(i18n.T("cmd.session.several_decisions"))
		for _, d := range several.Decisions {
			fmt.Fprintf(&b, "\n  %s  %s %s", d.ID, sessionsvc.KindIcon(d.Kind), sessionsvc.DecisionSummary(d))
		}
		return errors.New(b.String())
	case errors.Is(err, sessionsvc.ErrNoDecision):
		return errors.New(i18n.T("cmd.session.no_decision"))
	case errors.Is(err, sessionsvc.ErrAlwaysForbidden):
		return errors.New(i18n.T("cmd.session.always_forbidden"))
	case errors.Is(err, sessionsvc.ErrServerNotRunning):
		return errors.New(i18n.T("cmd.session.server_not_running"))
	}
	return err
}

var sessionInboxCmd = &cobra.Command{
	Use:   "inbox",
	Short: "Décisions en attente de toutes les sessions (⏸ checkpoint · ? question · ! permission · $ budget · ✗ erreur)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		svc, err := sessionService(cmd)
		if err != nil {
			return err
		}
		items, err := svc.Inbox(cmd.Context(), domain.DecisionFilter{})
		if err != nil {
			return err
		}
		if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
			out := make([]decisionJSON, 0, len(items))
			for _, it := range items {
				out = append(out, toDecisionJSON(it.Decision))
			}
			return printJSON(cmd, out)
		}
		if len(items) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T("cmd.session.inbox_empty"))
			return nil
		}
		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
		fmt.Fprintln(tw, i18n.T("cmd.session.inbox_header"))
		for _, it := range items {
			d := it.Decision
			agent := ""
			if it.Session != nil {
				agent = it.Session.EntryAgent
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", sessionsvc.KindIcon(d.Kind), d.ID, agent, sessionsvc.DecisionSummary(d), since(d.CreatedAt))
		}
		return tw.Flush()
	},
}

func since(t time.Time) string {
	d := time.Since(t).Round(time.Minute)
	if d < time.Minute {
		return i18n.T("cmd.session.just_now")
	}
	return i18n.Tf("cmd.session.ago", d.String())
}

var sessionApproveCmd = &cobra.Command{
	Use:   "approve <session-id|decision-id>",
	Short: "Répond à une permission (ou à un checkpoint) en attente, sans attacher la session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := sessionService(cmd)
		if err != nil {
			return err
		}
		d, err := svc.Pick(cmd.Context(), args[0], domain.DecisionPermission, domain.DecisionCheckpoint)
		if err != nil {
			return decisionError(err)
		}
		decision, _ := cmd.Flags().GetString("decision")
		msg, _ := cmd.Flags().GetString("message")
		if err := svc.Decide(cmd.Context(), sessionsvc.Reply{DecisionID: d.ID, Decision: decision, Message: msg}); err != nil {
			return decisionError(err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf("cmd.session.decided", sessionsvc.DecisionSummary(*d), decision))
		return nil
	},
}

var sessionAnswerCmd = &cobra.Command{
	Use:   "answer <session-id|decision-id> --field key=value…",
	Short: "Répond à une question de l'agent (formulaire), sans attacher la session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := sessionService(cmd)
		if err != nil {
			return err
		}
		d, err := svc.Pick(cmd.Context(), args[0], domain.DecisionQuestion)
		if err != nil {
			return decisionError(err)
		}
		pairs, _ := cmd.Flags().GetStringArray("field")
		if len(pairs) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), describeQuestion(*d))
			return errors.New(i18n.T("cmd.session.answer_needs_fields"))
		}
		raw := map[string]string{}
		for _, p := range pairs {
			k, v, ok := strings.Cut(p, "=")
			if !ok {
				return fmt.Errorf("%s: %s", i18n.T("cmd.session.field_format"), p)
			}
			raw[strings.TrimSpace(k)] = v
		}
		answer, err := sessionsvc.ParseAnswers(d.Payload.Fields, raw)
		if err != nil {
			fmt.Fprintln(cmd.ErrOrStderr(), describeQuestion(*d))
			return err
		}
		if err := svc.Decide(cmd.Context(), sessionsvc.Reply{DecisionID: d.ID, Answer: answer}); err != nil {
			return decisionError(err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf("cmd.session.answered", sessionsvc.DecisionSummary(*d)))
		return nil
	},
}

// describeQuestion lists the fields of a question and their choices.
func describeQuestion(d domain.Decision) string {
	var b strings.Builder
	b.WriteString("? " + sessionsvc.DecisionSummary(d))
	for _, f := range d.Payload.Fields {
		fmt.Fprintf(&b, "\n  --field %s=…  (%s", f.Key, f.Type)
		if f.Required {
			b.WriteString(", " + i18n.T("cmd.session.field_required"))
		}
		b.WriteString(")")
		if f.Title != "" || f.Description != "" {
			b.WriteString("  " + strings.TrimSpace(f.Title+" "+f.Description))
		}
		if len(f.Options) > 0 {
			vals := make([]string, 0, len(f.Options))
			for _, o := range f.Options {
				vals = append(vals, o.Value)
			}
			b.WriteString("\n      " + strings.Join(vals, " | "))
			if f.Custom {
				b.WriteString(" | " + i18n.T("cmd.session.field_custom"))
			}
		}
	}
	return b.String()
}

var sessionDismissCmd = &cobra.Command{
	Use:   "dismiss <session-id|decision-id>",
	Short: "Classe une alerte (✗ erreur, ✗ coupe-circuit, $ budget) d'une session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := sessionService(cmd)
		if err != nil {
			return err
		}
		d, err := svc.Pick(cmd.Context(), args[0], domain.DecisionError, domain.DecisionBudget, domain.DecisionCircuit)
		if err != nil {
			return decisionError(err)
		}
		if err := svc.Decide(cmd.Context(), sessionsvc.Reply{DecisionID: d.ID, Decision: "dismiss"}); err != nil {
			return decisionError(err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf("cmd.session.dismissed", sessionsvc.DecisionSummary(*d)))
		return nil
	},
}

var sessionSendCmd = &cobra.Command{
	Use:   "send <session-id> \"consigne…\"",
	Short: "Envoie une consigne courte à une session (prise en compte à la prochaine étape)",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := sessionService(cmd)
		if err != nil {
			return err
		}
		sess, err := svc.Resolve(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		synthetic, _ := cmd.Flags().GetBool("synthetic")
		queue, _ := cmd.Flags().GetBool("queue")
		o := sessionsvc.SendOptions{Synthetic: synthetic}
		if queue {
			o.Delivery = adapters.DeliveryQueue
		}
		if err := svc.Send(cmd.Context(), sess.ID, strings.Join(args[1:], " "), o); err != nil {
			return decisionError(err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf("cmd.session.sent", sess.ID))
		return nil
	},
}

// sessionOp builds a command applying one operation to a session.
func sessionOp(use, short, done string, nargs int, op func(ctx context.Context, svc *sessionsvc.Service, id string, args []string) error) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(nargs),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := sessionService(cmd)
			if err != nil {
				return err
			}
			sess, err := svc.Resolve(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if err := op(cmd.Context(), svc, sess.ID, args[1:]); err != nil {
				return decisionError(err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(done, sess.ID))
			return nil
		},
	}
}

var (
	sessionInterruptCmd = sessionOp("interrupt <session-id>", "Interrompt l'étape en cours (la session reste ouverte)", "cmd.session.interrupted", 1,
		func(ctx context.Context, svc *sessionsvc.Service, id string, _ []string) error {
			return svc.Interrupt(ctx, id)
		})
	sessionCompactCmd = sessionOp("compact <session-id>", "Compacte l'historique d'une session", "cmd.session.compacted", 1,
		func(ctx context.Context, svc *sessionsvc.Service, id string, _ []string) error {
			return svc.Compact(ctx, id)
		})
	sessionModelCmd = sessionOp("model <session-id> <fournisseur/modèle>", "Change le modèle des prochaines étapes d'une session", "cmd.session.model_switched", 2,
		func(ctx context.Context, svc *sessionsvc.Service, id string, args []string) error {
			return svc.SwitchModel(ctx, id, args[0])
		})
)

var sessionForkCmd = &cobra.Command{
	Use:   "fork <session-id>",
	Short: "Crée une variante d'une session (copie de son historique, même serveur)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		id, err := resolveSessionRef(ctx, args[0])
		if err != nil {
			return err
		}
		svc, err := newRunService(ctx, MustApp())
		if err != nil {
			return err
		}
		child, err := svc.ForkSession(ctx, id)
		if err != nil {
			return decisionError(err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf("cmd.session.forked", id, child))
		return nil
	},
}

var sessionResumeCmd = &cobra.Command{
	Use:   "resume <session-id>",
	Short: "Reprend une session en veille (redémarre son serveur) sans ouvrir d'interface",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		a := MustApp()
		id, err := resolveSessionRef(ctx, args[0])
		if err != nil {
			return err
		}
		svc, err := newRunService(ctx, a)
		if err != nil {
			return err
		}
		if _, _, err := svc.AttachCommand(ctx, id); err == nil {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf("cmd.session.already_running", id))
			return nil
		}
		if err := resumeV5Session(ctx, a, svc, id); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf("cmd.session.resumed", id))
		return nil
	},
}

var sessionOpenCmd = &cobra.Command{
	Use:   "open <session-id> --browser",
	Short: "Ouvre une session dans le navigateur (code d'appairage à usage unique, 5 min)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		a := MustApp()
		id, err := resolveSessionRef(ctx, args[0])
		if err != nil {
			return err
		}
		svc, err := newRunService(ctx, a)
		if err != nil {
			return err
		}
		if err := ensureSessionAwake(ctx, cmd, a, svc, id); err != nil {
			return err
		}
		printOnly, _ := cmd.Flags().GetBool("print")
		return openSessionInBrowser(ctx, cmd, svc, id, printOnly)
	},
}

var sessionFollowCmd = &cobra.Command{
	Use:   "follow <session-id>",
	Short: "Suit une session en direct (lecture seule : agent courant, outils, messages, coût) ; Ctrl+C pour quitter",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := sessionService(cmd)
		if err != nil {
			return err
		}
		sess, err := svc.Resolve(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
		defer stop()
		feed, err := svc.Follow(ctx, sess.ID)
		if errors.Is(err, sessionsvc.ErrNoLive) {
			return errors.New(i18n.T("cmd.session.follow_no_daemon"))
		}
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		fmt.Fprintln(out, i18n.Tf("cmd.session.following", sess.ID, stateLabel(sess.State)))
		cost := sess.Cost
		for it := range feed {
			if it.Kind == domain.FeedUsage {
				cost = it.Cost
				continue
			}
			line := sessionsvc.FeedLine(it)
			if line == "" {
				continue
			}
			fmt.Fprintf(out, "%s  %s\n", it.Time.Local().Format("15:04:05"), line)
		}
		if ctx.Err() == nil {
			fmt.Fprintln(out, i18n.T("cmd.session.follow_ended"))
		}
		fmt.Fprintf(out, "$%.3f\n", cost)
		return nil
	},
}

var sessionResultsCmd = &cobra.Command{
	Use:   "results <session-id>",
	Short: "Résultats d'une session : fichiers modifiés, branche, coût ; --mr pour une description de MR, --patch pour le diff",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := sessionService(cmd)
		if err != nil {
			return err
		}
		sess, err := svc.Resolve(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		r, err := svc.Results(cmd.Context(), sess.ID)
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
			return printJSON(cmd, map[string]any{"results": r.Summary, "live": r.Live})
		}
		if mr, _ := cmd.Flags().GetBool("mr"); mr {
			fmt.Fprint(out, sessionsvc.MRDescription(r))
			return nil
		}
		if patch, _ := cmd.Flags().GetBool("patch"); patch {
			fmt.Fprint(out, r.Patch)
			return nil
		}
		fmt.Fprintln(out, sessionsvc.Recap(r))
		if r.Branch != "" {
			fmt.Fprintln(out, i18n.Tf("cmd.session.results_branch", r.Branch))
		}
		for _, f := range r.Files {
			fmt.Fprintf(out, "  %s  +%d −%d\n", f.File, f.Additions, f.Deletions)
		}
		if !r.Live {
			fmt.Fprintln(out, i18n.T("cmd.session.results_snapshot"))
		}
		return nil
	},
}

func init() {
	sessionInboxCmd.Flags().Bool("json", false, "Sortie JSON")
	sessionApproveCmd.Flags().String("decision", "once", "once | always | reject ; checkpoint : once (valider) | fix (corriger d'abord) | other (autre consigne) | reject — fix/other exigent -m")
	sessionApproveCmd.Flags().StringP("message", "m", "", "Message transmis à l'agent")
	sessionAnswerCmd.Flags().StringArray("field", nil, "Réponse clé=valeur (répétable ; liste : a,b)")
	sessionSendCmd.Flags().Bool("synthetic", false, "Message d'oh (non utilisateur)")
	sessionSendCmd.Flags().Bool("queue", false, "Après l'étape en cours (au lieu de la prochaine étape)")
	sessionOpenCmd.Flags().Bool("browser", true, "Ouvrir dans le navigateur (seul mode disponible)")
	sessionOpenCmd.Flags().Bool("print", false, "Afficher l'URL sans ouvrir le navigateur")
	sessionResultsCmd.Flags().Bool("mr", false, "Description de MR (Markdown)")
	sessionResultsCmd.Flags().Bool("patch", false, "Diff complet")
	sessionResultsCmd.Flags().Bool("json", false, "Sortie JSON")
	sessionCmd.AddCommand(sessionInboxCmd, sessionApproveCmd, sessionAnswerCmd, sessionDismissCmd, sessionSendCmd,
		sessionInterruptCmd, sessionCompactCmd, sessionModelCmd, sessionForkCmd, sessionResumeCmd, sessionOpenCmd,
		sessionFollowCmd, sessionResultsCmd)
}
