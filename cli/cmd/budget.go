package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/cliout"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon/api"
	"github.com/datichb/openhub/cli/internal/daemon/client"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/limits"
	"github.com/datichb/openhub/cli/internal/services/launch"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
)

// oh budget: session restrictions (I6) — effective values with their
// origin, hub and project settings, raise of a budget decision.

var budgetCmd = &cobra.Command{
	Use: "budget",
}

func init() {
	rootCmd.AddCommand(budgetCmd)
	show := &cobra.Command{
		Use:  "show",
		Args: cobra.NoArgs,
		RunE: runBudgetShow,
	}
	show.Flags().StringP("project", "p", "", "")
	show.Flags().Bool("json", false, "")
	set := &cobra.Command{
		Use:  "set <restriction> <value>",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error { return runBudgetSet(cmd, args[0], args[1]) },
	}
	set.Flags().StringP("project", "p", "", "")
	unset := &cobra.Command{
		Use:  "unset <restriction>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error { return runBudgetSet(cmd, args[0], "") },
	}
	unset.Flags().StringP("project", "p", "", "")
	raise := &cobra.Command{
		Use:  "raise <session-id|decision-id> [amount-usd]",
		Args: cobra.RangeArgs(1, 2),
		RunE: runBudgetRaise,
	}
	budgetCmd.AddCommand(show, set, unset, raise)
}

// budgetProject returns the project of --project, else of the current
// directory (nil when none).
func budgetProject(ctx context.Context, a *app.App, flag string) (*domain.Project, error) {
	if flag != "" {
		return resolveProject(ctx, a, flag, projectRead)
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
		r = launch.SessionLimits(ctx, a.Config, a.Preferences, project, nil)
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
	if us := a.Usage; us != nil {
		day := domain.UsageDay(time.Now())
		rep.TodayUSD, _ = us.DayCost(ctx, day, "")
		if project != nil {
			rep.ProjectUSD, _ = us.DayCost(ctx, day, project.ID)
		}
	}
	if h, err := client.NewClient(api.Paths{Dir: ohRunDir()}).Health(ctx); err == nil {
		rep.MemoryMB = h.MemoryMB
	}
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		return cliout.JSON(cmd.OutOrStdout(), rep)
	}
	out := cmd.OutOrStdout()
	if r.IsZero() {
		fmt.Fprintln(out, i18n.T("cmd.budget.none"))
	}
	tw := cliout.Table(out)
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
		var l limits.Limits
		if err := updateHubConfig(a, func(c *config.Config) error {
			l = c.Limits
			if err := l.Set(field, value); err != nil {
				return err
			}
			c.Limits = l
			return nil
		}); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), budgetSavedMessage(l, field, ""))
		return nil
	}
	project, err := resolveProject(ctx, a, flag, projectWrite)
	if err != nil {
		return err
	}
	l := launch.ProjectLimits(ctx, a.Preferences, project.ID)
	if err := l.Set(field, value); err != nil {
		return err
	}
	if field == limits.FieldMemory && l.MemoryMB > 0 {
		return errors.New(i18n.T("cmd.budget.memory_machine"))
	}
	if err := saveProjectLimits(ctx, a, project.ID, l); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), budgetSavedMessage(l, field, project.Name))
	return nil
}

// budgetSavedMessage confirms a change: "removed" when the restriction is
// off after it (unset, 0, off), "saved" otherwise (A28).
func budgetSavedMessage(l limits.Limits, field, projectName string) string {
	name := i18n.T("cmd.budget.field." + field)
	removed := l.Value(field) == ""
	switch {
	case projectName == "" && removed:
		return i18n.Tf("cmd.budget.removed_hub", name)
	case projectName == "":
		return i18n.Tf("cmd.budget.saved_hub", name)
	case removed:
		return i18n.Tf("cmd.budget.removed_project", name, projectName)
	}
	return i18n.Tf("cmd.budget.saved_project", name, projectName)
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
