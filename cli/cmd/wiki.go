package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	mcpwiki "github.com/datichb/openhub/cli/internal/mcp/wiki"
	"github.com/datichb/openhub/cli/internal/services/wiki"
)

// `oh wiki` (modernisation LD-J): the wiki of the project (docs/wiki/) as
// oh checks it, and the proposals of the agents waiting in its queue.

var wikiCmd = &cobra.Command{
	Use:   "wiki",
	Short: i18n.T("cmd.wiki.short"),
	Long:  i18n.T("cmd.wiki.long"),
}

// errWikiCheck makes `oh wiki check` exit non-zero when the wiki has errors.
var errWikiCheck = errors.New(i18n.T("cmd.wiki.wiki_check_failed"))

func init() {
	check := &cobra.Command{
		Use:   "check [dir]",
		Short: i18n.T("cmd.wiki.check.short"),
		Long:  i18n.T("cmd.wiki.check.long"),
		Args:  cobra.MaximumNArgs(1),
		RunE:  runWikiCheck,
	}
	check.Flags().Bool("fix", false, i18n.T("cmd.wiki.check.flags.fix"))
	check.Flags().Bool("json", false, i18n.T("cmd.wiki.check.flags.json"))
	check.Flags().StringSlice("path", nil, i18n.T("cmd.wiki.check.flags.path"))
	check.Flags().String("base", "", i18n.T("cmd.wiki.check.flags.base"))
	check.Flags().String("head", "", i18n.T("cmd.wiki.check.flags.head"))
	check.Flags().Bool("since-sync", false, i18n.T("cmd.wiki.check.flags.since_sync"))
	stats := &cobra.Command{
		Use:   "stats [dir]",
		Short: i18n.T("cmd.wiki.stats.short"),
		Long:  i18n.T("cmd.wiki.stats.long"),
		Args:  cobra.MaximumNArgs(1),
		RunE:  runWikiStats,
	}
	stats.Flags().Bool("json", false, i18n.T("cmd.wiki.check.flags.json"))
	pending := &cobra.Command{
		Use:   "pending [dir]",
		Short: i18n.T("cmd.wiki.pending.short"),
		Long:  i18n.T("cmd.wiki.pending.long"),
		Args:  cobra.MaximumNArgs(1),
		RunE:  runWikiPending,
	}
	pending.Flags().Bool("all", false, i18n.T("cmd.wiki.pending.flags.all"))
	pending.Flags().String("session", "", i18n.T("cmd.wiki.pending.flags.session"))
	pending.Flags().Bool("json", false, i18n.T("cmd.wiki.pending.flags.json"))
	wikiCmd.AddCommand(withInitLevel(check, levelNone), withInitDB(pending), withInitDB(stats))
	rootCmd.AddCommand(wikiCmd)
}

// wikiRoot returns the project of a directory: the closest one holding
// docs/wiki/, else the directory itself.
func wikiRoot(args []string) (string, error) {
	dir := "."
	if len(args) == 1 {
		dir = args[0]
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for d := abs; ; {
		if wiki.Exists(d) {
			return d, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			return abs, nil
		}
		d = parent
	}
}

func runWikiCheck(cmd *cobra.Command, args []string) error {
	root, err := wikiRoot(args)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if fix, _ := cmd.Flags().GetBool("fix"); fix {
		res, err := wiki.Fix(root, time.Now())
		if err != nil {
			return wikiError(root, err)
		}
		if len(res.Pages) == 0 {
			fmt.Fprintln(out, i18n.T("cmd.wiki.check.nothing_fixed"))
		} else {
			fmt.Fprintln(out, i18n.Tf("cmd.wiki.check.fixed", strings.Join(res.Pages, ", ")))
		}
	}
	var opts wiki.CheckOptions
	opts.Paths, _ = cmd.Flags().GetStringSlice("path")
	opts.Base, _ = cmd.Flags().GetString("base")
	opts.Head, _ = cmd.Flags().GetString("head")
	opts.SinceSync, _ = cmd.Flags().GetBool("since-sync")
	rep, err := wiki.CheckWith(cmd.Context(), root, opts, nil)
	if err != nil {
		return wikiError(root, err)
	}
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		if err := writeIndentedJSON(out, rep); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(out, mcpwiki.CheckText(rep, 0))
		if len(rep.GodNodes) > 0 {
			fmt.Fprintln(out)
			fmt.Fprintln(out, i18n.Tf("cmd.wiki.check.god_nodes", len(rep.GodNodes)))
			for _, g := range rep.GodNodes {
				fmt.Fprintf(out, "  %s : %s (%s)\n", g.Concept, strings.Join(g.Pages, ", "), g.Criticality)
			}
		}
	}
	if rep.Errors() > 0 {
		cmd.SilenceUsage = true
		return errWikiCheck
	}
	return nil
}

// runWikiStats prints the metrics of the wiki (LD-I): coverage of the
// domains, median age of the pages, share of INCERTAIN, proposals waiting.
func runWikiStats(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	root, err := wikiRoot(args)
	if err != nil {
		return err
	}
	svc := newWikiService()
	projectID := ""
	if a := application; a != nil && a.Projects != nil {
		projectID, _ = wikiProject(ctx, root, func(ctx context.Context, path string) (string, error) {
			p, err := a.Projects.GetByPath(ctx, path)
			if err != nil {
				return "", err
			}
			return p.ID, nil
		})
	}
	m, err := svc.Metrics(ctx, root, projectID)
	if err != nil {
		return wikiError(root, err)
	}
	out := cmd.OutOrStdout()
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		return writeIndentedJSON(out, m)
	}
	printWikiMetrics(out, m)
	return nil
}

func printWikiMetrics(w io.Writer, m wiki.Metrics) {
	fmt.Fprintln(w, i18n.Tf("cmd.wiki.stats.pages", m.Pages, m.ByKind[wiki.KindSpec], m.ByKind[wiki.KindDecision], m.ByKind[wiki.KindPostmortem]))
	fmt.Fprintln(w, i18n.Tf("cmd.wiki.stats.domains", m.DomainsCovered, m.Domains, int(m.Coverage()*100+0.5)))
	if len(m.Uncovered) > 0 {
		fmt.Fprintln(w, "  "+i18n.Tf("cmd.wiki.stats.uncovered", strings.Join(m.Uncovered, ", ")))
	}
	fmt.Fprintln(w, i18n.Tf("cmd.wiki.stats.age", m.MedianAgeDays, m.Dated, m.Oldest, m.OldestDays))
	fmt.Fprintln(w, i18n.Tf("cmd.wiki.stats.tags", int(m.UncertainShare()*100+0.5), m.Tags[wiki.TagUncertain], m.TaggedEnrichments(),
		m.Tags[wiki.TagConfirmed], m.Tags[wiki.TagDeduced]))
	if m.Pending < 0 {
		fmt.Fprintln(w, i18n.T("cmd.wiki.stats.no_queue"))
	} else {
		fmt.Fprintln(w, i18n.Tf("cmd.wiki.stats.pending", m.Pending, m.Accepted))
	}
	if m.Synced != "" {
		fmt.Fprintln(w, i18n.Tf("cmd.wiki.stats.synced", m.Synced))
	}
}

func wikiError(root string, err error) error {
	if errors.Is(err, wiki.ErrNoWiki) {
		return errors.New(i18n.Tf("cmd.wiki.no_wiki", root))
	}
	return err
}

func runWikiPending(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	root, err := wikiRoot(args)
	if err != nil {
		return err
	}
	a := MustApp()
	projectID, err := wikiProject(ctx, root, func(ctx context.Context, path string) (string, error) {
		p, err := a.Projects.GetByPath(ctx, path)
		if err != nil {
			return "", err
		}
		return p.ID, nil
	})
	if err != nil {
		return err
	}
	svc := newWikiServiceFor(a)
	session, _ := cmd.Flags().GetString("session")
	if session != "" {
		ss, err := newSessionService(ctx, a)
		if err != nil {
			return err
		}
		sess, err := ss.Resolve(ctx, session)
		if err != nil {
			return sessionRefError(session, err)
		}
		session = sess.ID
	}
	var list []domain.WikiProposal
	if all, _ := cmd.Flags().GetBool("all"); all {
		list, err = svc.List(ctx, projectID, session)
	} else {
		list, err = svc.Open(ctx, projectID, session)
	}
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		type row struct {
			ID string `json:"id"`
			domain.WikiProposal
		}
		rows := make([]row, 0, len(list))
		for _, p := range list {
			rows = append(rows, row{ID: p.ID(), WikiProposal: p})
		}
		return writeIndentedJSON(out, rows)
	}
	printWikiProposals(out, list)
	return nil
}

// wikiProject returns the oh project of a wiki root (or of the git
// repository above it).
func wikiProject(ctx context.Context, root string, byPath func(context.Context, string) (string, error)) (string, error) {
	for d := root; ; {
		if id, err := byPath(ctx, d); err == nil {
			return id, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", errors.New(i18n.Tf("cmd.wiki.pending.no_project", root))
		}
		d = parent
	}
}

func printWikiProposals(w io.Writer, list []domain.WikiProposal) {
	if len(list) == 0 {
		fmt.Fprintln(w, i18n.T("cmd.wiki.pending.none"))
		return
	}
	// The reason of a refusal gets a column only when one is to show.
	reasons := false
	for _, p := range list {
		reasons = reasons || p.Reason != ""
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	header := i18n.T("cmd.wiki.pending.header")
	if reasons {
		header += "\t" + i18n.T("cmd.wiki.pending.header_reason")
	}
	fmt.Fprintln(tw, header)
	for _, p := range list {
		page := p.Page
		if p.Section != "" {
			page += " § " + p.Section
		}
		first := clipLine(p.Content, 60)
		if p.Replaces != "" {
			// p5-fixes: a replacement, before → after.
			first = "« " + clipLine(p.Replaces, 28) + " » → « " + clipLine(p.Content, 28) + " »"
		}
		row := fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s", p.ID(), p.Status, firstNonEmpty(p.Agent, "?"), p.SessionID, page, p.Tag, first)
		if reasons {
			row += "\t" + clipLine(p.Reason, 60)
		}
		fmt.Fprintln(tw, row)
	}
	_ = tw.Flush()
}

// clipLine returns the first line of s, bounded to n runes.
func clipLine(s string, n int) string {
	first, _, _ := strings.Cut(s, "\n")
	if r := []rune(first); len(r) > n {
		first = string(r[:n-1]) + "…"
	}
	return first
}
