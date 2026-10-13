package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// staleOptions are the flags of `oh skill check --stale` (modernisation R13).
type staleOptions struct {
	enabled bool
	maxAge  string
	strict  bool
	// hub is the hub directory to check (a checkout of the hub); empty:
	// the installed hub.
	hub string
}

// staleSkillsError is returned with --strict when skills are to review.
type staleSkillsError struct{ n int }

func (e *staleSkillsError) Error() string { return i18n.Tf("cmd.skill.check.stale.failed", e.n) }

// runSkillStale lists the skills to review on today (stack skills reviewed
// more than the maximum age ago, or without freshness metadata).
func runSkillStale(out io.Writer, hubDir string, today time.Time, opts staleOptions, asJSON bool) error {
	maxAge, err := bundle.ParseMaxAge(opts.maxAge)
	if err != nil {
		return err
	}
	rep, err := bundle.CheckFreshness(hubDir, today, maxAge)
	if err != nil {
		return err
	}
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			return err
		}
	} else {
		printStaleSkills(out, rep)
	}
	if opts.strict && len(rep.Skills) > 0 {
		return &staleSkillsError{n: len(rep.Skills)}
	}
	return nil
}

func printStaleSkills(out io.Writer, rep *bundle.FreshnessReport) {
	if len(rep.Skills) == 0 {
		fmt.Fprintf(out, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.skill.check.stale.ok", rep.Checked, rep.MaxAgeDays))
		return
	}
	fmt.Fprintf(out, "%s\n\n", i18n.Tf("cmd.skill.check.stale.header", rep.Checked, rep.MaxAgeDays, rep.Today))
	for _, s := range rep.Skills {
		if s.Status == bundle.FreshnessMissing {
			fmt.Fprintf(out, "  %s %s\n", theme.ErrorStyle.Render(theme.IconError), i18n.Tf("cmd.skill.check.stale.missing", s.Ref, s.Detail))
			continue
		}
		fmt.Fprintf(out, "  %s %s\n", theme.WarningStyle.Render(theme.IconWarning), i18n.Tf("cmd.skill.check.stale.stale", s.Ref, s.Reviewed, s.AgeDays, formatVersions(s.Versions)))
	}
	fmt.Fprintf(out, "\n  %s\n", i18n.Tf("cmd.skill.check.stale.summary", len(rep.Skills)))
}

// formatVersions writes versions as "react 19.3, react-compiler 1.0".
func formatVersions(v map[string]string) string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + " " + v[k]
	}
	return strings.Join(parts, ", ")
}
