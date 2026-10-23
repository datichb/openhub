package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gc"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/worktree"
)

// oh gc (N16, DC15): what oh leaves behind — session bundles, data of the
// server groups, session folders, automatic worktrees, temporary folders,
// orphan tool servers, old history — removed after a summary and a
// confirmation; the daemon does it every [gc] every_hours.

var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Nettoie ce qu'oh laisse derrière lui (paquets, données des serveurs, sessions arrêtées, worktrees automatiques, historique)",
	Args:  cobra.NoArgs,
	RunE:  runGC,
}

func init() {
	rootCmd.AddCommand(gcCmd)
	addGCFlags(gcCmd)
}

func addGCFlags(c *cobra.Command) {
	f := c.Flags()
	f.Bool("dry-run", false, "Afficher ce qui serait supprimé, sans rien supprimer")
	f.BoolP("yes", "y", false, "Supprimer sans confirmation (obligatoire sans terminal)")
	f.Bool("json", false, "Plan (ou résultat) au format JSON")
	f.StringSlice("only", nil, "Seulement ces catégories : bundles, servers, sessions, worktrees, fanout, temp, orphans, history")
	f.String("older-than", "", "Garder les fichiers des sessions arrêtées moins longtemps que cette durée (ex. 30d, 12h ; défaut : [gc] session_days)")
}

// gcStore is the database side of the clean-up.
type gcStore struct {
	sessions domain.SessionStore
	servers  domain.ServerStore
	grants   domain.GrantStore
	history  domain.HistoryStore
}

func (s gcStore) Sessions(ctx context.Context) ([]domain.Session, error) {
	return s.sessions.List(ctx, "")
}

func (s gcStore) Servers(ctx context.Context) ([]domain.Server, error) { return s.servers.List(ctx) }

func (s gcStore) DeleteServer(ctx context.Context, gk string) error {
	if err := s.grants.RevokeOwner(ctx, gk, time.Now()); err != nil {
		return err
	}
	return s.servers.Delete(ctx, gk)
}

func historyOf(r domain.HistoryRows) gc.History {
	return gc.History{Sessions: r.Sessions, AgentEvents: r.AgentEvents, Decisions: r.Decisions,
		UsageSessions: r.UsageSessions, UsageProxy: r.UsageProxy, BudgetExtra: r.BudgetExtra, WikiProposals: r.WikiProposals}
}

func (s gcStore) CountHistory(ctx context.Context, c gc.Cutoffs) (gc.History, error) {
	r, err := s.history.Count(ctx, c.History, c.Usage)
	return historyOf(r), err
}

func (s gcStore) DeleteHistory(ctx context.Context, c gc.Cutoffs) (gc.History, error) {
	r, err := s.history.Delete(ctx, c.History, c.Usage)
	return historyOf(r), err
}

// gcEnv is the clean-up of this hub.
func gcEnv(a *app.App) (*gc.Env, error) {
	if store == nil || a == nil || a.Sessions == nil || a.Servers == nil || a.Grants == nil || a.History == nil || a.Fanout == nil {
		return nil, errors.New(i18n.T("cmd.gc.no_database"))
	}
	env := &gc.Env{
		HubDir: config.HubDir(), BundlesDir: ohBundlesDir(), ServersDir: ohServersDir(), SessionsDir: ohSessionsDir(), RunDir: ohRunDir(),
		Store: gcStore{sessions: a.Sessions, servers: a.Servers, grants: a.Grants, history: a.History},
		ProjectPath: func(ctx context.Context, id string) string {
			if a.Projects == nil {
				return ""
			}
			if p, err := a.Projects.Get(ctx, id); err == nil {
				return p.Path
			}
			return ""
		},
		BaseBranch: func(repo string) string {
			if b := a.Config.Worktree.BaseBranch; b != "" {
				return b
			}
			return worktree.DetectBaseBranch(repo)
		},
		GroupLockPath: func(g string) string { return daemon.GroupLockPath(ohServersDir(), g) },
	}
	if l := serverLister(); l != nil {
		env.Processes = l
	}
	env.Fanout = fanoutCleaner{store: a.Fanout}
	return env, nil
}

// gcOptions are the retentions of [gc] (olderThan > 0 overrides the files
// of the stopped sessions).
func gcOptions(a *app.App, only []gc.Kind, olderThan time.Duration) gc.Options {
	g := a.Config.GC
	o := gc.Options{Only: only, SessionAge: g.Sessions(), HistoryAge: g.History(), UsageAge: g.Usage(), BundleGrace: g.BundleGrace(),
		AutoWorktrees: a.Config.Worktree.AutoCleanup}
	if olderThan > 0 {
		o.SessionAge = olderThan
	}
	return o
}

// parseAge reads 30d, 12h, 90m (Go durations and days).
func parseAge(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	if d, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(d)
		if err != nil || n < 0 {
			return 0, errors.New(i18n.Tf("cmd.gc.bad_age", s))
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < 0 {
		return 0, errors.New(i18n.Tf("cmd.gc.bad_age", s))
	}
	return d, nil
}

func parseKinds(values []string) ([]gc.Kind, error) {
	var out []gc.Kind
	for _, v := range values {
		k := gc.Kind(strings.TrimSpace(v))
		known := false
		for _, x := range gc.Kinds {
			known = known || x == k
		}
		if !known {
			names := make([]string, len(gc.Kinds))
			for i, x := range gc.Kinds {
				names[i] = string(x)
			}
			return nil, errors.New(i18n.Tf("cmd.gc.bad_kind", v, strings.Join(names, ", ")))
		}
		out = append(out, k)
	}
	return out, nil
}

func runGC(cmd *cobra.Command, _ []string) error {
	a := MustApp()
	ctx := cmd.Context()
	w := cmd.OutOrStdout()
	f := cmd.Flags()
	dry, _ := f.GetBool("dry-run")
	asJSON, _ := f.GetBool("json")
	onlyRaw, _ := f.GetStringSlice("only")
	olderRaw, _ := f.GetString("older-than")
	only, err := parseKinds(onlyRaw)
	if err != nil {
		return err
	}
	olderThan, err := parseAge(olderRaw)
	if err != nil {
		return err
	}
	env, err := gcEnv(a)
	if err != nil {
		return err
	}
	opts := gcOptions(a, only, olderThan)
	plan, err := gc.Plan(ctx, env, opts)
	if err != nil {
		return err
	}
	if asJSON && (dry || plan.Empty()) {
		return json.NewEncoder(w).Encode(plan)
	}
	if !asJSON {
		printGCPlan(w, plan)
	}
	if plan.Empty() {
		return nil
	}
	if dry {
		fmt.Fprintln(w, i18n.T("cmd.gc.dry_run"))
		return nil
	}
	ok, err := confirmAction(cmd, i18n.T("cmd.gc.confirm"), false)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(w, i18n.T("cmd.gc.cancelled"))
		return nil
	}
	done, err := gc.Run(ctx, env, opts, &plan)
	if errors.Is(err, gc.ErrBusy) {
		return errors.New(i18n.T("cmd.gc.busy"))
	}
	if err != nil {
		return err
	}
	writeGCLast(gcLast{At: time.Now(), Bytes: done.Bytes(), Items: len(done.Items), Rows: done.History.Total(), Armed: true})
	if asJSON {
		return json.NewEncoder(w).Encode(done)
	}
	fmt.Fprintf(w, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.gc.done", len(done.Items), formatBytes(done.Bytes()), done.History.Total()))
	for _, e := range done.Errors {
		fmt.Fprintf(w, "  %s %s\n", theme.WarningStyle.Render(theme.IconWarning), e)
	}
	return nil
}

// printGCPlan prints the plan per category (five items each at most).
func printGCPlan(w io.Writer, plan gc.Report) {
	if plan.Empty() {
		fmt.Fprintln(w, i18n.T("cmd.gc.nothing"))
		return
	}
	fmt.Fprintln(w, theme.Bold.Render(i18n.Tf("cmd.gc.plan", formatBytes(plan.Bytes()))))
	by := plan.ByKind()
	for _, k := range gc.Kinds {
		items := by[k]
		if len(items) == 0 {
			continue
		}
		var size int64
		for _, it := range items {
			size += it.Bytes
		}
		line := i18n.Tf("cmd.gc.kind."+string(k), len(items), formatBytes(size))
		if k == gc.KindOrphans {
			line = i18n.Tf("cmd.gc.kind.orphans", len(items))
		}
		fmt.Fprintf(w, "  %s\n", line)
		sort.Slice(items, func(i, j int) bool { return items[i].Bytes > items[j].Bytes })
		for i, it := range items {
			if i == 5 {
				fmt.Fprintf(w, "      %s\n", i18n.Tf("cmd.gc.more", len(items)-5))
				break
			}
			label := it.Path
			if label == "" {
				label = it.ID
			}
			fmt.Fprintf(w, "      %s  %s\n", label, theme.Subtitle.Render(formatBytes(it.Bytes)))
		}
	}
	if h := plan.History; h.Total() > 0 {
		fmt.Fprintf(w, "  %s\n", i18n.Tf("cmd.gc.kind.history", h.Total(), h.Sessions, h.AgentEvents, h.Decisions, h.UsageSessions+h.UsageProxy+h.BudgetExtra, h.WikiProposals))
	}
	for _, e := range plan.Errors {
		fmt.Fprintf(w, "  %s %s\n", theme.WarningStyle.Render(theme.IconWarning), e)
	}
}

// gcLast is the last clean-up (~/.oh/run/gc.last): the daemon only
// simulates until oh gc ran once (Armed).
type gcLast struct {
	At     time.Time `json:"at"`
	DryRun bool      `json:"dry_run,omitempty"`
	Bytes  int64     `json:"bytes"`
	Items  int       `json:"items"`
	Rows   int       `json:"rows"`
	Armed  bool      `json:"armed,omitempty"`
}

func gcLastPath() string { return filepath.Join(ohRunDir(), "gc.last") }

func readGCLast() (gcLast, bool) {
	var l gcLast
	data, err := os.ReadFile(gcLastPath())
	if err != nil || json.Unmarshal(data, &l) != nil {
		return gcLast{}, false
	}
	return l, true
}

func writeGCLast(l gcLast) {
	if prev, ok := readGCLast(); ok && prev.Armed {
		l.Armed = true
	}
	data, _ := json.Marshal(l)
	if err := os.MkdirAll(ohRunDir(), 0o700); err == nil {
		_ = os.WriteFile(gcLastPath(), data, 0o600)
	}
}

// daemonGC is the periodic clean-up of the daemon: every [gc] every_hours,
// a simulation until oh gc ran once (first pass: "N reclaimable, run oh
// gc"), then the clean-up itself. It never keeps the daemon awake.
func daemonGC(a *app.App) func(ctx context.Context) {
	return func(ctx context.Context) {
		if a == nil || a.Config == nil || !a.Config.GC.Enabled {
			return
		}
		last, ok := readGCLast()
		if ok && time.Since(last.At) < a.Config.GC.Every() {
			return
		}
		env, err := gcEnv(a)
		if err != nil {
			return
		}
		opts := gcOptions(a, nil, 0)
		if !last.Armed {
			plan, err := gc.Plan(ctx, env, opts)
			if err != nil {
				slog.Warn("gc: planning failed", "error", err)
				return
			}
			writeGCLast(gcLast{At: time.Now(), DryRun: true, Bytes: plan.Bytes(), Items: len(plan.Items), Rows: plan.History.Total()})
			slog.Info("gc: simulation (run oh gc to clean up)", "reclaimable", formatBytes(plan.Bytes()), "items", len(plan.Items), "rows", plan.History.Total())
			return
		}
		done, err := gc.Run(ctx, env, opts, nil)
		if err != nil {
			if !errors.Is(err, gc.ErrBusy) {
				slog.Warn("gc failed", "error", err)
			}
			return
		}
		writeGCLast(gcLast{At: time.Now(), Bytes: done.Bytes(), Items: len(done.Items), Rows: done.History.Total(), Armed: true})
		slog.Info("gc", "freed", formatBytes(done.Bytes()), "items", len(done.Items), "rows", done.History.Total(), "errors", len(done.Errors))
	}
}

// Doctor › clean-up: what the last pass did, or what can be reclaimed.
func init() { registerDoctorCheck(gcDoctorCheck) }

func gcDoctorCheck(context.Context) []doctor.Result {
	name := i18n.T("cmd.gc.doctor.name")
	last, ok := readGCLast()
	switch {
	case !ok:
		return []doctor.Result{{ID: doctorIDGC, Name: name, OK: true, Detail: i18n.T("cmd.gc.doctor.never")}}
	case last.DryRun && (last.Items > 0 || last.Rows > 0):
		return []doctor.Result{{ID: doctorIDGC, Name: name, OK: true, Warn: true, Detail: i18n.Tf("cmd.gc.doctor.reclaimable", formatBytes(last.Bytes), last.Items, last.Rows)}}
	}
	return []doctor.Result{{ID: doctorIDGC, Name: name, OK: true, Detail: i18n.Tf("cmd.gc.doctor.last", last.At.Local().Format("2006-01-02 15:04"), formatBytes(last.Bytes), last.Items)}}
}
