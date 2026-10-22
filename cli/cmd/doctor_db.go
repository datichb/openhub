package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/launcher"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Checks of the configuration and of the database that work when either is
// broken (N13): oh doctor runs with a degraded startup.

// checkConfig reports a hub.toml that does not load, with the line and
// column of a TOML error and the backup oh doctor --fix can restore.
func checkConfig() (string, bool) {
	if r := startupReport; r != nil && r.ConfigErr != nil {
		msg := i18n.Tf("cmd.doctor.db.config_invalid", config.ConfigPath(), configErrorDetail(r.ConfigErr))
		if b := latestConfigBackup(); b != "" {
			msg += " — " + i18n.Tf("cmd.doctor.db.config_backup", b)
		}
		return msg, false
	}
	a := TryApp()
	if a == nil || a.Config == nil {
		return i18n.T("cmd.doctor.config_not_loaded"), false
	}
	return i18n.Tf("cmd.doctor.config_ok", a.Config.CLI.Language), true
}

// configErrorDetail says what is wrong in a hub.toml that does not load: the
// line, column and key of the error (config.ConfigError), or, for another
// error, hub.toml read again to locate a syntax error.
func configErrorDetail(err error) string {
	var ce *config.ConfigError
	if errors.As(err, &ce) {
		return describeConfigError(ce)
	}
	data, rerr := os.ReadFile(config.ConfigPath())
	if rerr == nil {
		var m map[string]any
		var de *toml.DecodeError
		if perr := toml.Unmarshal(data, &m); errors.As(perr, &de) {
			row, col := de.Position()
			return i18n.Tf("cmd.doctor.db.config_position", row, col, de.Error())
		}
	}
	return err.Error()
}

// configBackups lists the copies of hub.toml that load (strictly: a copy with
// a value of the wrong type is skipped), most recent first:
// hub.toml.bak-<time> (written before each save) and hub.toml.<time>.bak
// (former team migration).
func configBackups() []string {
	dir := filepath.Dir(config.ConfigPath())
	var all []string
	for _, pat := range []string{"hub.toml.bak-*", "hub.toml.*.bak"} {
		m, _ := filepath.Glob(filepath.Join(dir, pat))
		all = append(all, m...)
	}
	type entry struct {
		path string
		mod  time.Time
	}
	var es []entry
	for _, p := range all {
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if config.Validate(p, data) != nil {
			continue
		}
		es = append(es, entry{p, fi.ModTime()})
	}
	sort.Slice(es, func(i, j int) bool { return es[i].mod.After(es[j].mod) })
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.path
	}
	return out
}

func latestConfigBackup() string {
	if b := configBackups(); len(b) > 0 {
		return b[0]
	}
	return ""
}

// fixConfig restores the latest valid backup of a hub.toml that does not
// load (oh doctor --fix), after confirmation; the broken file is kept.
func fixConfig(cmd *cobra.Command) error {
	r := startupReport
	if r == nil || r.ConfigErr == nil {
		return nil
	}
	w := cmd.OutOrStdout()
	backup := latestConfigBackup()
	if backup == "" {
		fmt.Fprintln(w, i18n.T("cmd.doctor.db.config_no_backup"))
		fmt.Fprintln(w)
		return nil
	}
	fmt.Fprintln(w, theme.Bold.Render(i18n.Tf("cmd.doctor.db.config_restore_plan", backup, config.ConfigPath())))
	yes, _ := cmd.Flags().GetBool("yes")
	if !yes {
		if !stdinIsTerminal() {
			return errors.New(i18n.T("cmd.doctor.fix.needs_yes"))
		}
		ok, err := launcher.NewCLIUI(cmd.ErrOrStderr()).Confirm(i18n.T("cmd.doctor.db.config_restore_confirm"))
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(w, i18n.T("cmd.doctor.fix.cancelled"))
			return nil
		}
	}
	kept, err := restoreConfig(backup)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "%s %s\n\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.doctor.db.config_restored", backup, kept))
	config.Reset()
	if c, err := config.Load(); err == nil {
		r.ConfigErr = nil
		if a := TryApp(); a != nil {
			a.Config = c
		}
	}
	return nil
}

func restoreConfig(backup string) (string, error) {
	path := config.ConfigPath()
	data, err := os.ReadFile(backup)
	if err != nil {
		return "", err
	}
	kept := path + ".broken-" + time.Now().UTC().Format("20060102T150405Z")
	if err := os.Rename(path, kept); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return kept, nil
}

// checkDatabase opens the database without migrating it: schema version
// against this oh, pending and unknown migrations, the last failed
// migration, a quick integrity check and the size of the WAL.
func checkDatabase() views.DoctorCheck {
	path := sqlite.DBPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return views.DoctorCheck{OK: true, Detail: i18n.T("cmd.doctor.db.missing")}
	}
	if r := startupReport; r != nil && r.DBErr != nil {
		return views.DoctorCheck{Detail: i18n.Tf("cmd.doctor.db.unreadable", r.DBErr)}
	}
	s := store
	if s == nil {
		raw, err := sqlite.OpenRaw(path)
		if err != nil {
			return views.DoctorCheck{Detail: i18n.Tf("cmd.doctor.db.unreadable", err)}
		}
		defer raw.Close()
		s = raw
	}
	st, err := dbStatus(s)
	if err != nil {
		return views.DoctorCheck{Detail: i18n.Tf("cmd.doctor.db.unreadable", err)}
	}
	return st.result()
}

type dbState struct {
	version, known int
	pending, newer []int
	lastErr        string
	integrity      error
	projects       int
	wal            int64
}

func dbStatus(s *sqlite.Store) (dbState, error) {
	st := dbState{known: sqlite.MaxKnownVersion(), projects: -1}
	v, err := s.SchemaVersion()
	if err != nil {
		return st, err
	}
	st.version = v
	if st.pending, err = s.PendingMigrations(); err != nil {
		return st, err
	}
	st.newer, _ = s.UnknownMigrations()
	st.lastErr = sqlite.LastMigrationError(s.Path())
	st.integrity = quickCheck(s)
	if fi, err := os.Stat(s.Path() + "-wal"); err == nil {
		st.wal = fi.Size()
	}
	if len(st.pending) == 0 && st.integrity == nil {
		var n int
		if s.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM projects`).Scan(&n) == nil {
			st.projects = n
		}
	}
	return st, nil
}

func quickCheck(s *sqlite.Store) error {
	var res string
	if err := s.DB().QueryRow(`PRAGMA quick_check`).Scan(&res); err != nil {
		return err
	}
	if res != "ok" {
		return errors.New(res)
	}
	return nil
}

func (st dbState) result() views.DoctorCheck {
	schema := i18n.Tf("cmd.doctor.db.schema", st.version, st.known)
	wal := ""
	if st.wal > 0 {
		wal = " · " + i18n.Tf("cmd.doctor.db.wal", formatBytes(st.wal))
	}
	switch {
	case st.integrity != nil:
		return views.DoctorCheck{Detail: i18n.Tf("cmd.doctor.db.corrupt", st.integrity)}
	case len(st.newer) > 0:
		return views.DoctorCheck{Detail: i18n.Tf("cmd.doctor.db.newer", joinInts(st.newer), st.known)}
	case st.lastErr != "" && len(st.pending) > 0:
		return views.DoctorCheck{Detail: i18n.Tf("cmd.doctor.db.migration_failed", st.lastErr, joinInts(st.pending))}
	case len(st.pending) > 0:
		return views.DoctorCheck{OK: true, Warn: true, Detail: schema + " · " + i18n.Tf("cmd.doctor.db.pending", joinInts(st.pending)) + wal}
	}
	detail := schema + wal
	if st.projects >= 0 {
		detail = i18n.Tf("cmd.doctor.db_ok", st.projects) + " · " + detail
	}
	return views.DoctorCheck{OK: true, Detail: detail}
}

// printStartupHints is printed above the checks of a degraded startup that
// failed: what still works.
func printStartupHints(w io.Writer) {
	r := startupReport
	if r == nil || (r.ConfigErr == nil && r.DBErr == nil) {
		return
	}
	fmt.Fprintln(w, theme.WarningStyle.Render(i18n.T("cmd.init.level.degraded")))
	fmt.Fprintln(w)
}
