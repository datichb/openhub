package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

func budgetReportRun(t *testing.T, args ...string) (string, error) {
	t.Helper()
	c := &cobra.Command{}
	c.Flags().String("by", "project", "")
	c.Flags().String("since", defaultReportSince, "")
	c.Flags().Bool("json", false, "")
	require.NoError(t, c.ParseFlags(args))
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&out)
	c.SetContext(context.Background())
	err := runBudgetReport(c, nil)
	return out.String(), err
}

func seedBudgetReport(t *testing.T) {
	t.Helper()
	_, _ = gcTestApp(t) // project p, session ses_old (no workflow)
	us := sqlite.NewUsageStore(store)
	ctx := context.Background()
	today := domain.UsageDay(time.Now())
	old := domain.UsageDay(time.Now().Add(-90 * 24 * time.Hour))
	require.NoError(t, us.AddSession(ctx, domain.SessionUsage{Day: today, SessionID: "ses_old", ProjectID: "p", CostUSD: 1.5, TokensIn: 100, TokensOut: 20}))
	require.NoError(t, us.AddSession(ctx, domain.SessionUsage{Day: today, SessionID: "ses_gone", ProjectID: "p", CostUSD: 0.5}))
	require.NoError(t, us.AddSession(ctx, domain.SessionUsage{Day: old, SessionID: "ses_ancient", ProjectID: "p", CostUSD: 10}))
}

// oh budget report sums the ledger per project or workflow; the sessions
// deleted by gc (no row in sessions) are counted under « unknown ».
func TestBudgetReport(t *testing.T) {
	seedBudgetReport(t)

	out, err := budgetReportRun(t)
	require.NoError(t, err)
	assert.Contains(t, out, "Spending per project, since ")
	assert.Contains(t, out, "PROJECT")
	assert.Contains(t, out, "2.00 $")
	assert.NotContains(t, out, "12.00", "the old row is out of the period")
	assert.Contains(t, out, "Total: $2.00")

	out, err = budgetReportRun(t, "--by", "workflow", "--since", "0")
	require.NoError(t, err)
	assert.Contains(t, out, "all time")
	assert.Contains(t, out, "unknown", "no workflow known for these sessions")
	assert.Contains(t, out, "Total: $12.00")
}

func TestBudgetReportJSON(t *testing.T) {
	seedBudgetReport(t)
	out, err := budgetReportRun(t, "--json", "--by", "workflow")
	require.NoError(t, err)
	var rep budgetSpendReport
	require.NoError(t, json.Unmarshal([]byte(out), &rep))
	assert.Equal(t, "workflow", rep.By)
	assert.NotEmpty(t, rep.Since)
	assert.InDelta(t, 2, rep.TotalUSD, 1e-9)
	require.Len(t, rep.Groups, 1)
	assert.Equal(t, "", rep.Groups[0].Key)
	assert.Equal(t, "unknown", rep.Groups[0].Label)
	assert.Equal(t, 2, rep.Groups[0].Sessions)
}

func TestBudgetReportRefusals(t *testing.T) {
	seedBudgetReport(t)
	_, err := budgetReportRun(t, "--by", "model")
	assert.ErrorContains(t, err, "unknown grouping")
	_, err = budgetReportRun(t, "--since", "soon")
	assert.Error(t, err)
}
