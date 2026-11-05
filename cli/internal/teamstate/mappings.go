package teamstate

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/fsutil"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// Names of the two mapping tables of [tracker], as config.toml writes them.
const (
	MappingTableLabelStatus = "label_status_mapping"
	MappingTableStatus      = "status_mapping"
)

// legacyInitialColumnID is the ID the discovery assistant used to write as the
// target of the labels of the initial column ("todo"): the ID of the initial
// column of the default board, which is not a claim status (the claim status
// of that column is "planned").
const legacyInitialColumnID = "todo"

// MappingIssue is a value of label_status_mapping or status_mapping that is
// not a status of the board.
type MappingIssue struct {
	// Table is MappingTableLabelStatus or MappingTableStatus.
	Table string
	// Key is the tracker label or status name.
	Key string
	// Value is the status as config.toml writes it.
	Value string
	// Status is the status the value stands for on this board (the one of the
	// initial column for "todo" or for the ID of a column); empty when it
	// stands for nothing.
	Status string
}

// Migrated reports whether the value stands for a status of the board (it is
// read as Status) rather than for nothing.
func (i MappingIssue) Migrated() bool { return i.Status != "" }

// Text says what is wrong and how to fix it.
func (i MappingIssue) Text() string {
	if i.Migrated() {
		return i18n.Tf("v53d.mapping.issue_migrated", i.Table, i.Key, i.Value, i.Status)
	}
	return i18n.Tf("v53d.mapping.issue_unknown", i.Table, i.Key, i.Value)
}

// MappingError is the refusal to write a config.toml whose mappings hold
// values that are not statuses of the board.
type MappingError struct {
	Issues []MappingIssue
	// Valid are the statuses of the board.
	Valid []string
}

func (e *MappingError) Error() string {
	lines := make([]string, 0, len(e.Issues)+1)
	for _, i := range e.Issues {
		lines = append(lines, i.Text())
	}
	lines = append(lines, i18n.Tf("v53d.mapping.valid_statuses", strings.Join(e.Valid, ", ")))
	return strings.Join(lines, "\n")
}

// MappableStatuses are the values a mapping accepts on this board: its
// statuses and "planned", always valid.
func (cfg BoardConfig) MappableStatuses() []string {
	out := slices.Clone(cfg.AllStatuses())
	if !slices.Contains(out, ClaimStatusPlanned) {
		out = append([]string{ClaimStatusPlanned}, out...)
	}
	return out
}

// ResolveStatus tells what a value of label_status_mapping or status_mapping
// stands for on this board:
//
//   - a status of the board is itself (migrated is false);
//   - "todo", the ID of the initial column of the default board, and the ID
//     of any column of the board, are not claim statuses unless the board
//     declares them: they stand for the status of the initial column;
//   - a status written in another case or with spaces around is the status;
//   - anything else stands for nothing (ok is false).
func (cfg BoardConfig) ResolveStatus(value string) (status string, migrated, ok bool) {
	if cfg.IsValidStatus(value) {
		return value, false, true
	}
	norm := strings.ToLower(strings.TrimSpace(value))
	if norm != "" && cfg.IsValidStatus(norm) {
		return norm, true, true
	}
	if norm == legacyInitialColumnID || cfg.ColumnByID(norm) != nil {
		return cfg.InitialStatus(), true, true
	}
	return "", false, false
}

// OrderedStatuses are the statuses of the board in the order of its columns.
// The default board is todo, in_progress, review, validation, done, blocked,
// "todo" being the status "planned".
func (cfg BoardConfig) OrderedStatuses() []string {
	cols := cfg.Columns
	if len(cols) == 0 {
		cols = DefaultBoardConfig().Columns
	}
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.ID
		if !cfg.HasCustomColumns() && c.Role == ColumnRoleInitial {
			out[i] = ClaimStatusPlanned
		}
	}
	return out
}

// StatusRank is the position of a status among the columns of the board
// (-1 for a status the board does not have, which cannot be ordered).
// "planned" is the initial column.
func (cfg BoardConfig) StatusRank(status string) int {
	ordered := cfg.OrderedStatuses()
	if status == ClaimStatusPlanned {
		status = cfg.InitialStatus()
	}
	for i, id := range ordered {
		if id == status {
			return i
		}
	}
	return -1
}

// IsBlockedStatus reports whether the status is that of an impediment column:
// a column with the role "blocked" on a board with columns of its own, the
// status "blocked" on the default board.
func (cfg BoardConfig) IsBlockedStatus(status string) bool {
	if !cfg.HasCustomColumns() {
		return status == ClaimStatusBlocked
	}
	c := cfg.ColumnByID(status)
	return c != nil && c.Role == ColumnRoleBlocked
}

// NormalizeMappings rewrites in place the values of label_status_mapping and
// status_mapping that stand for a status of the board (see ResolveStatus) and
// returns every value that is not one, sorted: the ones it migrated and the
// ones that stand for nothing (left as they are).
func (t *TrackerConfig) NormalizeMappings(board BoardConfig) []MappingIssue {
	var issues []MappingIssue
	for _, table := range []struct {
		name string
		m    map[string]string
	}{{MappingTableLabelStatus, t.LabelStatusMapping}, {MappingTableStatus, t.StatusMapping}} {
		for key, value := range table.m {
			status, migrated, ok := board.ResolveStatus(value)
			if ok && !migrated {
				continue
			}
			issues = append(issues, MappingIssue{Table: table.name, Key: key, Value: value, Status: status})
			if ok {
				table.m[key] = status
			}
		}
	}
	sort.Slice(issues, func(a, b int) bool {
		if issues[a].Table != issues[b].Table {
			return issues[a].Table < issues[b].Table // label_status_mapping first
		}
		return issues[a].Key < issues[b].Key
	})
	return issues
}

// DropStaleMappings removes from the mappings the values that stand for
// nothing on board (a column the team no longer has), after having read the
// migratable ones as the status they stand for. It returns what it dropped:
// the caller says so. A board that was just replaced calls it, so that the old
// mappings do not make the new config invalid.
func (t *TrackerConfig) DropStaleMappings(board BoardConfig) []MappingIssue {
	var dropped []MappingIssue
	for _, issue := range t.NormalizeMappings(board) {
		if issue.Migrated() {
			continue
		}
		switch issue.Table {
		case MappingTableLabelStatus:
			delete(t.LabelStatusMapping, issue.Key)
		case MappingTableStatus:
			delete(t.StatusMapping, issue.Key)
		}
		dropped = append(dropped, issue)
	}
	return dropped
}

// MappingIssues lists the values of the mappings that are not statuses of the
// board, without changing anything.
func (t TrackerConfig) MappingIssues(board BoardConfig) []MappingIssue {
	return (&TrackerConfig{
		LabelStatusMapping: copyStringMap(t.LabelStatusMapping),
		StatusMapping:      copyStringMap(t.StatusMapping),
	}).NormalizeMappings(board)
}

// ValidateMappings refuses the mappings that hold a value which is not a
// status of the board: nothing is migrated on the way to the file, the caller
// writes what it means. nil when all are valid.
func (t TrackerConfig) ValidateMappings(board BoardConfig) error {
	issues := t.MappingIssues(board)
	if len(issues) == 0 {
		return nil
	}
	return &MappingError{Issues: issues, Valid: board.MappableStatuses()}
}

func copyStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// TrackerMappingIssues lists the values of label_status_mapping and
// status_mapping that config.toml holds as they are written and that are not
// statuses of its board (LoadConfig reads the migratable ones as the status
// they stand for, so it cannot tell).
func (r *Repo) TrackerMappingIssues(ctx context.Context) ([]MappingIssue, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	cfg, err := r.readConfig()
	if err != nil {
		return nil, err
	}
	return cfg.Tracker.MappingIssues(cfg.Board), nil
}

// RepairTrackerMappings writes the values of the mappings that stand for a
// status of the board as that status (one commit), and returns what it
// rewrote and the values that stand for nothing, which it leaves for the team
// to correct. Nothing is written when no value can be migrated.
func (r *Repo) RepairTrackerMappings(ctx context.Context) (migrated, unknown []MappingIssue, err error) {
	err = r.withWriteLock(ctx, func(ctx context.Context) error {
		cfg, err := r.readConfig()
		if err != nil {
			return err
		}
		migrated, unknown = nil, nil
		for _, issue := range cfg.Tracker.NormalizeMappings(cfg.Board) {
			if issue.Migrated() {
				migrated = append(migrated, issue)
			} else {
				unknown = append(unknown, issue)
			}
		}
		if len(migrated) == 0 {
			return nil
		}
		path := filepath.Join(r.path, "config.toml")
		existing, _ := os.ReadFile(path)
		data, err := marshalConfig(cfg, existing)
		if err != nil {
			return err
		}
		if err := fsutil.WriteFileAtomic(path, data, 0o644); err != nil {
			return err
		}
		return r.commitAndPush(ctx, "config: tracker mappings use claim statuses", "config.toml")
	})
	return migrated, unknown, err
}
