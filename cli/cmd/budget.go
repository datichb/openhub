package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/limits"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// oh budget: session restrictions (I6) — effective values with their
// origin, hub and project settings, raise of a budget decision.

var budgetCmd = &cobra.Command{
	Use:   "budget",
	Short: "Restrictions des sessions : sessions actives max, budgets, plafond mémoire, modèles (désactivées par défaut)",
}

func init() {
	rootCmd.AddCommand(budgetCmd)
	show := &cobra.Command{
		Use:   "show",
		Short: "Affiche les restrictions effectives (avec leur origine) et les dépenses du jour",
		Args:  cobra.NoArgs,
		RunE:  runBudgetShow,
	}
	show.Flags().StringP("project", "p", "", "Projet (ID ou nom ; défaut : projet du dossier courant)")
	show.Flags().Bool("json", false, "Sortie JSON")
	set := &cobra.Command{
		Use:   "set <restriction> <value>",
		Short: "Règle une restriction du hub (hub.toml) ou d'un projet (--project) : " + strings.Join(limits.Fields, ", "),
		Args:  cobra.ExactArgs(2),
		RunE:  func(cmd *cobra.Command, args []string) error { return runBudgetSet(cmd, args[0], args[1]) },
	}
	set.Flags().StringP("project", "p", "", "Projet (ID ou nom) ; sans : le hub")
	unset := &cobra.Command{
		Use:   "unset <restriction>",
		Short: "Retire une restriction du hub ou d'un projet (--project)",
		Args:  cobra.ExactArgs(1),
		RunE:  func(cmd *cobra.Command, args []string) error { return runBudgetSet(cmd, args[0], "") },
	}
	unset.Flags().StringP("project", "p", "", "Projet (ID ou nom) ; sans : le hub")
	raise := &cobra.Command{
		Use:   "raise <session-id|decision-id> [amount-usd]",
		Short: "Relève le budget d'une session arrêtée par une décision $ (défaut : le budget configuré une fois de plus)",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  runBudgetRaise,
	}
	budgetCmd.AddCommand(show, set, unset, raise)
}

// budgetProject returns the project of --project, else of the current
// directory (nil when none).
func budgetProject(ctx context.Context, a *app.App, flag string) (*domain.Project, error) {
	if flag != "" {
		return resolveProject(ctx, a, flag)
	}
	cwd, _ := os.Getwd()
	projects, err := a.Projects.List(ctx, domain.ProjectStatusActive)
	if err != nil {
		return nil, err
	}
	for i, p := range projects {
		abs, _ := filepath.Abs(p.Path)
		if abs == cwd || isSubPath(cwd, abs) {
			return &projects[i], nil
		}
	}
	return nil, nil
}

type budgetRow struct {
	Field  string `json:"field"`
	Value  string `json:"value"`
	Origin string `json:"origin,omitempty"`
}

type budgetReport struct {
	ProjectID   string      `json:"project_id,omitempty"`
	Limits      []budgetRow `json:"limits"`
	TodayUSD    float64     `json:"today_usd"`
	ProjectUSD  float64     `json:"today_project_usd,omitempty"`
	MemoryMB    int         `json:"memory_mb,omitempty"`
	DailyScope  string      `json:"daily_scope,omitempty"`
	ActiveScope string      `json:"active_scope,omitempty"`
}

func runBudgetShow(cmd *cobra.Command, _ []string) error {
	ctx, a := cmd.Context(), MustApp()
	flag, _ := cmd.Flags().GetString("project")
	project, err := budgetProject(ctx, a, flag)
	if err != nil {
		return err
	}
	var r limits.Resolved
	rep := budgetReport{}
	if project != nil {
		r = sessionLimits(ctx, a, project, nil)
		rep.ProjectID = project.ID
	} else {
		r = limits.Resolve(limits.Input{Hub: a.Config.Limits})
	}
	for _, f := range limits.Fields {
		rep.Limits = append(rep.Limits, budgetRow{Field: f, Value: r.Value(f), Origin: string(r.Origins[f])})
	}
	if r.DailyBudgetUSD > 0 {
		rep.DailyScope = r.DailyScope()
	}
	if r.MaxActiveSessions > 0 {
		rep.ActiveScope = r.ActiveScope()
	}
	if store != nil {
		us := sqlite.NewUsageStore(store)
		day := domain.UsageDay(time.Now())
		rep.TodayUSD, _ = us.DayCost(ctx, day, "")
		if project != nil {
			rep.ProjectUSD, _ = us.DayCost(ctx, day, project.ID)
		}
	}
	if h, err := daemon.NewClient(daemon.Paths{Dir: ohRunDir()}).Health(ctx); err == nil {
		rep.MemoryMB = h.MemoryMB
	}
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}
	out := cmd.OutOrStdout()
	if r.IsZero() {
		fmt.Fprintln(out, i18n.T("cmd.budget.none"))
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, i18n.T("cmd.budget.header"))
	for _, row := range rep.Limits {
		v, o := row.Value, ""
		if v == "" {
			v = i18n.T("cmd.budget.off")
		}
		if row.Origin != "" {
			o = i18n.T("cmd.budget.origin." + row.Origin)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", i18n.T("cmd.budget.field."+row.Field), v, o)
	}
	_ = tw.Flush()
	fmt.Fprintln(out, i18n.Tf("cmd.budget.today", rep.TodayUSD))
	if project != nil {
		fmt.Fprintln(out, i18n.Tf("cmd.budget.today_project", project.Name, rep.ProjectUSD))
	}
	if rep.MemoryMB > 0 {
		fmt.Fprintln(out, i18n.Tf("cmd.budget.memory_used", rep.MemoryMB))
	}
	return nil
}

func runBudgetSet(cmd *cobra.Command, field, value string) error {
	ctx, a := cmd.Context(), MustApp()
	flag, _ := cmd.Flags().GetString("project")
	if flag == "" {
		l := a.Config.Limits
		if err := l.Set(field, value); err != nil {
			return err
		}
		a.Config.Limits = l
		if err := config.Save(a.Config); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf("cmd.budget.saved_hub", i18n.T("cmd.budget.field."+field)))
		return nil
	}
	project, err := resolveProject(ctx, a, flag)
	if err != nil {
		return err
	}
	l := projectLimits(ctx, a, project.ID)
	if err := l.Set(field, value); err != nil {
		return err
	}
	if field == limits.FieldMemory && l.MemoryMB > 0 {
		return errors.New(i18n.T("cmd.budget.memory_machine"))
	}
	if err := saveProjectLimits(ctx, a, project.ID, l); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf("cmd.budget.saved_project", i18n.T("cmd.budget.field."+field), project.Name))
	return nil
}

func runBudgetRaise(cmd *cobra.Command, args []string) error {
	svc, err := sessionService(cmd)
	if err != nil {
		return err
	}
	d, err := svc.Pick(cmd.Context(), args[0], domain.DecisionBudget)
	if err != nil {
		return decisionError(err)
	}
	amount := ""
	if len(args) == 2 {
		amount = args[1]
	}
	v, err := sessionsvc.RaiseAmount(*d, amount)
	if err != nil {
		return err
	}
	if err := svc.Decide(cmd.Context(), sessionsvc.Reply{DecisionID: d.ID, Decision: "raise", Message: amount}); err != nil {
		return decisionError(err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf("cmd.budget.raised", d.SessionID, v))
	return nil
}
