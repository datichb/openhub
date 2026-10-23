package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

var repairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Diagnostiquer et réparer la base de données oh",
	Long:  i18n.T("cmd.repair.long"),
	Args:  cobra.NoArgs,
	RunE:  runRepair,
}

func init() {
	rootCmd.AddCommand(withInitLevel(repairCmd, levelDegraded))
	repairCmd.Flags().Bool("auto", false, "Mode non-interactif (pour scripts)")
	repairCmd.Flags().Bool("check-only", false, "Vérifier uniquement, sans réparer")
	repairCmd.Flags().Bool("migrate", false, "Appliquer les migrations en attente, après une sauvegarde")
	addYesFlag(repairCmd)
	repairCmd.Flags().String("restore", "", "Restaurer la base depuis une sauvegarde (fichier .bak)")
}

// runRepair never needs a working startup (N13): the database is opened
// without migrating it; migrations run only with --migrate, after a backup.
func runRepair(cmd *cobra.Command, _ []string) error {
	w := cmd.OutOrStdout()
	checkOnly, _ := cmd.Flags().GetBool("check-only")
	auto, _ := cmd.Flags().GetBool("auto")
	migrate, _ := cmd.Flags().GetBool("migrate")
	restore, _ := cmd.Flags().GetString("restore")
	dbPath := sqlite.DBPath()

	if restore != "" {
		return repairRestore(cmd, restore, dbPath, auto)
	}

	fmt.Fprintln(w, i18n.Tf("cmd.repair.title", dbPath))
	fmt.Fprintln(w)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Fprint(w, i18n.T("cmd.repair.db_missing"))
		return nil
	}

	s := store
	if s == nil {
		raw, err := sqlite.OpenRaw(dbPath)
		if err != nil {
			fmt.Fprintln(w, i18n.Tf("cmd.repair.open_failed", err))
			if checkOnly {
				return &ExitError{Code: ExitFailure}
			}
			return repairRecovery(w, dbPath)
		}
		defer raw.Close()
		s = raw
	}

	fmt.Fprint(w, i18n.T("cmd.repair.integrity"))
	if err := s.IntegrityCheck(); err != nil {
		fmt.Fprintln(w, i18n.T("cmd.repair.failed"))
		fmt.Fprintln(w, i18n.Tf("cmd.repair.detail", err))
		fmt.Fprintln(w)
		if checkOnly {
			return &ExitError{Code: ExitFailure}
		}
		return repairRecovery(w, dbPath)
	}
	fmt.Fprintln(w, i18n.T("cmd.repair.ok"))

	st, err := dbStatus(s)
	if err != nil {
		return err
	}
	fmt.Fprintln(w, i18n.Tf("cmd.repair.schema", st.version, st.known))
	if st.lastErr != "" {
		fmt.Fprintln(w, i18n.Tf("cmd.repair.last_error", st.lastErr))
	}
	if len(st.newer) > 0 {
		fmt.Fprintln(w, i18n.Tf("cmd.repair.newer", joinInts(st.newer)))
		return &ExitError{Code: ExitFailure}
	}
	if len(st.pending) > 0 {
		fmt.Fprintln(w, i18n.Tf("cmd.repair.pending", joinInts(st.pending)))
		if !migrate || checkOnly {
			fmt.Fprintln(w, i18n.T("cmd.repair.migrate_hint"))
			return nil
		}
		return repairMigrate(w, s)
	}
	if st.projects >= 0 {
		fmt.Fprintln(w, i18n.Tf("cmd.repair.projects", st.projects))
	}
	if b := s.Backups(); len(b) > 0 {
		fmt.Fprintln(w, i18n.Tf("cmd.repair.backups", len(b), b[0]))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("cmd.repair.healthy"))
	return nil
}

// repairMigrate applies the pending migrations: Migrate backs the database
// up first (VACUUM INTO, WAL included).
func repairMigrate(w io.Writer, s *sqlite.Store) error {
	if err := s.Migrate(); err != nil {
		var mig *sqlite.MigrationError
		if errors.As(err, &mig) && mig.Backup != "" {
			fmt.Fprintln(w, i18n.Tf("cmd.repair.migrate_failed_backup", mig.Version, mig.Err, mig.Backup, mig.Backup))
		} else {
			fmt.Fprintln(w, i18n.Tf("cmd.repair.migrate_failed", err))
		}
		return &ExitError{Code: ExitFailure}
	}
	if b := s.Backups(); len(b) > 0 {
		fmt.Fprintln(w, i18n.Tf("cmd.repair.backup_done", b[0]))
	}
	fmt.Fprintln(w, i18n.Tf("cmd.repair.migrated", sqlite.MaxKnownVersion()))
	return nil
}

// repairRestore puts a backup in place of the database. The daemon must be
// stopped (it keeps the database open).
func repairRestore(cmd *cobra.Command, backup, dbPath string, auto bool) error {
	w := cmd.OutOrStdout()
	if _, err := os.Stat(backup); err != nil {
		return errors.New(i18n.Tf("cmd.repair.restore_missing", backup))
	}
	if daemonRunning(cmd.Context()) {
		return errors.New(i18n.T("cmd.repair.restore_daemon"))
	}
	fmt.Fprintln(w, i18n.Tf("cmd.repair.restore_plan", backup, dbPath))
	if !auto {
		ok, err := confirmAction(cmd, i18n.T("cmd.repair.restore_confirm"), false)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(w, i18n.T("cmd.doctor.fix.cancelled"))
			return nil
		}
	}
	if store != nil {
		store.Close()
		store = nil
	}
	kept, err := sqlite.Restore(backup, dbPath)
	if err != nil {
		return errors.New(i18n.Tf("cmd.repair.restore_failed", err))
	}
	fmt.Fprintln(w, i18n.Tf("cmd.repair.restored", backup))
	if kept != "" {
		fmt.Fprintln(w, i18n.Tf("cmd.repair.replaced_kept", kept))
	}
	return nil
}

func daemonRunning(ctx context.Context) bool {
	c := daemon.NewClient(daemon.Paths{Dir: ohRunDir()})
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, err := c.Health(cctx)
	return err == nil
}

// repairRecovery handles a database that does not open or fails its
// integrity check: its files are copied aside (db, -wal, -shm), then the
// ways out are listed, deletion last.
func repairRecovery(w io.Writer, dbPath string) error {
	fmt.Fprintln(w, i18n.T("cmd.repair.recovery"))
	fmt.Fprintln(w)
	if saved, err := copyDBFiles(dbPath, dbPath+".corrupt-"+time.Now().UTC().Format("20060102T150405Z")); err == nil {
		fmt.Fprintln(w, i18n.Tf("cmd.repair.corrupt_saved", saved))
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("cmd.repair.options"))
	n := 1
	if s, err := sqlite.OpenRaw(dbPath); err == nil {
		backups := s.Backups()
		s.Close()
		if len(backups) > 0 {
			fmt.Fprintln(w, i18n.Tf("cmd.repair.option_restore", n))
			for _, b := range backups {
				fmt.Fprintf(w, "     oh repair --restore %s\n", b)
			}
			n++
		}
	} else if backups, _ := filepath.Glob(filepath.Join(filepath.Dir(dbPath), "backups", filepath.Base(dbPath)+".v*-*.bak")); len(backups) > 0 {
		fmt.Fprintln(w, i18n.Tf("cmd.repair.option_restore", n))
		for i := len(backups) - 1; i >= 0; i-- {
			fmt.Fprintf(w, "     oh repair --restore %s\n", backups[i])
		}
		n++
	}
	exports, _ := filepath.Glob(filepath.Join(config.HubDir(), "oh-backup-*.tar.gz"))
	if len(exports) == 0 {
		exports, _ = filepath.Glob("oh-backup-*.tar.gz")
	}
	if len(exports) > 0 {
		fmt.Fprintln(w, i18n.Tf("cmd.repair.option_import", n))
		for _, e := range exports {
			fmt.Fprintf(w, "     oh import %s\n", e)
		}
		n++
	}
	fmt.Fprintln(w, i18n.Tf("cmd.repair.option_reset", n, dbPath))
	fmt.Fprintln(w)
	return errors.New(i18n.T("cmd.repair.corrupt"))
}

// copyDBFiles copies the database with its -wal and -shm files (the WAL
// holds committed transactions not yet in the main file).
func copyDBFiles(dbPath, dest string) (string, error) {
	if err := copyFile(dbPath, dest); err != nil {
		return "", err
	}
	for _, sfx := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(dbPath + sfx); err == nil {
			if err := copyFile(dbPath+sfx, dest+sfx); err != nil {
				return "", err
			}
		}
	}
	return dest, nil
}

// copyFile copies src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}
