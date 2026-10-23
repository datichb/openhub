package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// oh budget report: what the sessions spent, per project or workflow, over a
// period (the usage ledger joined with the sessions).

const defaultReportSince = "30d"

func init() {
	report := &cobra.Command{
		Use:   "report",
		Short: "Dépenses par projet ou par workflow sur une période (sessions supprimées sous « inconnu »)",
		Args:  cobra.NoArgs,
		RunE:  runBudgetReport,
	}
	report.Flags().String("by", string(domain.UsageByProject), "Regroupement : project ou workflow")
	report.Flags().String("since", defaultReportSince, "Période : 7d, 12h… (0 = depuis toujours)")
	report.Flags().Bool("json", false, "Sortie JSON")
	budgetCmd.AddCommand(report)
}

type budgetReportGroup struct {
	Key       string  `json:"key"`
	Label     string  `json:"label"`
	Sessions  int     `json:"sessions"`
	CostUSD   float64 `json:"cost_usd"`
	TokensIn  int64   `json:"tokens_in"`
	TokensOut int64   `json:"tokens_out"`
}

type budgetSpendReport struct {
	By       string              `json:"by"`
	Since    string              `json:"since,omitempty"`
	TotalUSD float64             `json:"total_usd"`
	Groups   []budgetReportGroup `json:"groups"`
}

func runBudgetReport(cmd *cobra.Command, _ []string) error {
	ctx, a := cmd.Context(), MustApp()
	f := cmd.Flags()
	byRaw, _ := f.GetString("by")
	sinceRaw, _ := f.GetString("since")
	asJSON, _ := f.GetBool("json")
	by := domain.UsageBy(byRaw)
	if by != domain.UsageByProject && by != domain.UsageByWorkflow {
		return errors.New(i18n.Tf("cmd.budget.report.bad_by", byRaw))
	}
	age, err := parseAge(sinceRaw)
	if err != nil {
		return err
	}
	us := a.Usage
	if us == nil {
		return errors.New(i18n.T("cmd.v5_runtime.database_not_initialized"))
	}
	rep, err := buildBudgetReport(ctx, a, us, by, age, time.Now())
	if err != nil {
		return err
	}
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}
	printBudgetReport(cmd.OutOrStdout(), rep)
	return nil
}

// buildBudgetReport reads the spending since now-age (age 0 = everything).
func buildBudgetReport(ctx context.Context, a *app.App, us domain.UsageStore, by domain.UsageBy, age time.Duration, now time.Time) (*budgetSpendReport, error) {
	rep := &budgetSpendReport{By: string(by), Groups: []budgetReportGroup{}}
	if age > 0 {
		rep.Since = domain.UsageDay(now.Add(-age))
	}
	groups, err := us.UsageReport(ctx, by, rep.Since)
	if err != nil {
		return nil, err
	}
	for _, g := range groups {
		label := g.Key
		switch {
		case g.Key == "":
			label = i18n.T("cmd.budget.report.unknown")
		case by == domain.UsageByProject:
			label = projectLabel(ctx, a, g.Key)
		}
		rep.Groups = append(rep.Groups, budgetReportGroup{Key: g.Key, Label: label, Sessions: g.Sessions,
			CostUSD: g.CostUSD, TokensIn: g.TokensIn, TokensOut: g.TokensOut})
		rep.TotalUSD += g.CostUSD
	}
	return rep, nil
}

func printBudgetReport(out io.Writer, rep *budgetSpendReport) {
	period := i18n.T("cmd.budget.report.all_time")
	if rep.Since != "" {
		period = i18n.Tf("cmd.budget.report.since", rep.Since)
	}
	fmt.Fprintln(out, i18n.Tf("cmd.budget.report.title."+rep.By, period))
	if len(rep.Groups) == 0 {
		fmt.Fprintln(out, i18n.T("cmd.budget.report.empty"))
		return
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, i18n.T("cmd.budget.report.header."+rep.By))
	for _, g := range rep.Groups {
		fmt.Fprintf(tw, "%s\t%d\t%.2f $\t%d\t%d\n", g.Label, g.Sessions, g.CostUSD, g.TokensIn, g.TokensOut)
	}
	_ = tw.Flush()
	fmt.Fprintln(out, i18n.Tf("cmd.budget.report.total", rep.TotalUSD))
}
