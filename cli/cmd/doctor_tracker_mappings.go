package cmd

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/cliout"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// Doctor › the tracker mappings of the team. A value of label_status_mapping
// or status_mapping that is not a status of the board of the team ("todo" on
// a board without a todo column, which the discovery assistant used to write)
// was refused by the sync in silence, and the issue landed in progress.
// oh now reads "todo", or the ID of a column of the board, as the status of
// the initial column, and ignores (with a warning) a value that stands for
// nothing. A warning, named after the team clone, for each config.toml that
// still holds such values as they are written; oh doctor --fix writes the
// migratable ones as the status they stand for (one commit, after a
// confirmation), and leaves the others for the team to correct.

// Identifiers of the results: the one with a repair, and the one that needs a
// manual step.
const (
	doctorIDTrackerMappings       = "tracker_mappings"
	doctorIDTrackerMappingsManual = "tracker_mappings_manual"
)

func init() { registerDoctorCheck(trackerMappingsDoctorChecks) }

// mappingsStore is what the check and its repair ask of a team clone.
type mappingsStore interface {
	teamstate.RepoInfo
	TrackerMappingIssues(ctx context.Context) ([]teamstate.MappingIssue, error)
	RepairTrackerMappings(ctx context.Context) (migrated, unknown []teamstate.MappingIssue, err error)
}

// maxMappingLines bounds the values listed in a result.
const maxMappingLines = 8

// mappingTeam is a team clone with the values of its mappings that are not
// statuses of its board.
type mappingTeam struct {
	repo   mappingsStore
	issues []teamstate.MappingIssue
}

func (t mappingTeam) fixable() int {
	n := 0
	for _, i := range t.issues {
		if i.Migrated() {
			n++
		}
	}
	return n
}

// mappingTeams lists the team clones of the active projects whose config.toml
// holds mappings that are not statuses; a clone listed once.
func mappingTeams(ctx context.Context) (teams []mappingTeam, unreadable []string) {
	a := TryApp()
	if a == nil || a.Config == nil {
		return nil, nil
	}
	seen := map[string]bool{}
	for _, t := range claimTeams(ctx, a, a.Config) {
		store, ok := t.repo.(mappingsStore)
		if !ok || seen[store.Path()] {
			continue
		}
		seen[store.Path()] = true
		issues, err := store.TrackerMappingIssues(ctx)
		if err != nil {
			unreadable = append(unreadable, store.Path()+": "+err.Error())
			continue
		}
		if len(issues) > 0 {
			teams = append(teams, mappingTeam{repo: store, issues: issues})
		}
	}
	return teams, unreadable
}

func trackerMappingsDoctorChecks(ctx context.Context) []doctor.Result {
	teams, unreadable := mappingTeams(ctx)
	out := make([]doctor.Result, 0, len(teams)+len(unreadable))
	for _, t := range teams {
		out = append(out, trackerMappingsResult(t))
	}
	for _, u := range unreadable {
		out = append(out, doctor.Result{Name: i18n.T("v53d.doctor.mappings.name"), OK: true, Warn: true,
			Detail: i18n.Tf("v53d.doctor.mappings.unreadable", u)})
	}
	return out
}

func trackerMappingsResult(t mappingTeam) doctor.Result {
	lines := make([]string, 0, maxMappingLines+1)
	for i, issue := range t.issues {
		if i == maxMappingLines {
			lines = append(lines, i18n.Tf("v53d.doctor.mappings.more", len(t.issues)-maxMappingLines))
			break
		}
		lines = append(lines, "  - "+issue.Text())
	}
	detail := i18n.Tf("v53d.doctor.mappings.found", filepath.Join(t.repo.Path(), "config.toml"), len(t.issues)) +
		"\n" + strings.Join(lines, "\n")
	if t.fixable() > 0 {
		detail += "\n" + i18n.T("v53d.doctor.mappings.fix_hint")
	}
	if t.fixable() < len(t.issues) {
		detail += "\n" + i18n.T("v53d.doctor.mappings.manual_hint")
	}
	id := doctorIDTrackerMappingsManual
	if t.fixable() > 0 {
		id = doctorIDTrackerMappings
	}
	return doctor.Result{ID: id, Name: i18n.T("v53d.doctor.mappings.name"), OK: true, Warn: true, Detail: detail}
}

// fixTrackerMappings is oh doctor --fix for the tracker mappings.
func fixTrackerMappings(cmd *cobra.Command) error {
	return repairTrackerMappings(cmd.Context(), cmd.OutOrStdout(), cmdConfirm(cmd))
}

// repairTrackerMappings writes the values of the mappings that stand for a
// status as that status, after one confirmation. It asks nothing and does
// nothing when no clone holds such a value; the values that stand for nothing
// are named, not touched.
func repairTrackerMappings(ctx context.Context, w io.Writer, confirm confirmFunc) error {
	teams, _ := mappingTeams(ctx)
	var todo []mappingTeam
	count := 0
	for _, t := range teams {
		if n := t.fixable(); n > 0 {
			todo = append(todo, t)
			count += n
		}
	}
	if len(todo) == 0 {
		return nil
	}
	fmt.Fprintln(w, i18n.Tf("v53d.doctor.mappings.fix_plan", count, len(todo)))
	for _, t := range todo {
		for _, issue := range t.issues {
			if issue.Migrated() {
				fmt.Fprintln(w, "  - "+i18n.Tf("v53d.doctor.mappings.fix_line", issue.Table, issue.Key, issue.Value, issue.Status))
			}
		}
	}
	ok, err := confirm(i18n.T("v53d.doctor.mappings.fix_confirm"))
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(w, i18n.T("cmd.doctor.fix.cancelled"))
		return nil
	}
	for _, t := range todo {
		migrated, unknown, err := t.repo.RepairTrackerMappings(ctx)
		if err != nil {
			return fmt.Errorf("%s", i18n.Tf("v53d.doctor.mappings.fix_failed", t.repo.Path(), err))
		}
		cliout.Success(w, i18n.Tf("v53d.doctor.mappings.fix_done", len(migrated), t.repo.Path()))
		for _, issue := range unknown {
			cliout.Warn(w, issue.Text())
		}
	}
	return nil
}

// planTrackerMappingsFix is the plan shown by the Doctor view.
func planTrackerMappingsFix(ctx context.Context) (string, error) {
	teams, _ := mappingTeams(ctx)
	count := 0
	for _, t := range teams {
		count += t.fixable()
	}
	if count == 0 {
		return i18n.T("cmd.doctor.fix.nothing"), nil
	}
	lines := []string{i18n.Tf("v53d.doctor.mappings.fix_plan", count, len(teams))}
	for _, t := range teams {
		for _, issue := range t.issues {
			if issue.Migrated() {
				lines = append(lines, "  - "+i18n.Tf("v53d.doctor.mappings.fix_line", issue.Table, issue.Key, issue.Value, issue.Status))
			}
		}
	}
	return strings.Join(lines, "\n"), nil
}

// trackerMappingsFix is the repair of the tracker mappings, for the Doctor
// view; none for the results that only hold values that stand for nothing.
func trackerMappingsFix(r doctor.Result) *doctor.Fix {
	if r.ID != doctorIDTrackerMappings {
		return nil
	}
	return &doctor.Fix{Plan: planTrackerMappingsFix, Apply: doctorFixRun(func(ctx context.Context, w io.Writer) error {
		return repairTrackerMappings(ctx, w, alwaysConfirm)
	})}
}
