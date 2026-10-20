package cmd

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// backupManifest describes the contents of an export archive.
type backupManifest struct {
	Version   string    `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	Checksum  string    `json:"checksum"` // SHA-256 of db + config files
	Files     []string  `json:"files"`
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Exporter la configuration et les données (backup)",
	Long:  `Crée une archive .oh-backup.tar.gz contenant la DB, hub.toml et les secrets chiffrés.`,
	RunE:  runExport,
}

var importCmd = &cobra.Command{
	Use:   "import [file]",
	Short: "Restaurer depuis une archive de backup",
	Args:  cobra.ExactArgs(1),
	RunE:  runImport,
}

func init() {
	rootCmd.AddCommand(withInitLevel(exportCmd, levelDegraded))
	rootCmd.AddCommand(importCmd)

	exportCmd.Flags().StringP("output", "o", "", "Chemin de sortie (défaut: ./oh-backup-<date>.tar.gz)")
	importCmd.Flags().Bool("overwrite", false, i18n.T("cmd.export.flag.overwrite"))
	importCmd.Flags().Bool("merge", false, i18n.T("cmd.export.flag.merge"))
}

func runExport(cmd *cobra.Command, args []string) error {
	a := MustApp()
	output, _ := cmd.Flags().GetString("output")
	if output == "" {
		output = fmt.Sprintf("oh-backup-%s.tar.gz", time.Now().Format("2006-01-02"))
	}
	return exportArchive(a.IO.Out, output)
}

// snapshotDatabase writes a consistent copy of the database at dbPath into
// dir (VACUUM INTO, WAL included) and returns its path. The database is
// opened without any migration, on its own connection: writes still sitting in
// oh.db-wal are part of the copy, and a database in use is not disturbed.
func snapshotDatabase(dbPath, dir string) (string, error) {
	s, err := sqlite.OpenRaw(dbPath)
	if err != nil {
		return "", err
	}
	defer s.Close()
	dest := filepath.Join(dir, "oh.db")
	if err := s.Backup(dest); err != nil {
		return "", err
	}
	return dest, nil
}

// exportArchive writes the backup archive of the hub to output.
func exportArchive(w io.Writer, output string) error {
	hubDir := config.HubDir()

	fmt.Fprint(w, i18n.Tf("cmd.export.exporting_to", output))

	// Files to include in the archive
	filesToArchive := []struct {
		src  string
		name string
	}{
		{sqlite.DBPath(), "oh.db"},
		{config.ConfigPath(), "hub.toml"},
	}

	// Optionally include secrets if they exist
	secretsPath := filepath.Join(hubDir, "secrets.enc")
	if _, err := os.Stat(secretsPath); err == nil {
		filesToArchive = append(filesToArchive, struct{ src, name string }{secretsPath, "secrets.enc"})
	}

	// The database is archived from a consistent copy, not from oh.db: in WAL
	// mode the file alone misses the writes not yet checkpointed. When the copy
	// cannot be made (corrupted database), the file is archived as is, with a
	// warning.
	if fileExists(filesToArchive[0].src) {
		snapDir, err := os.MkdirTemp(hubDir, ".export-*")
		if err != nil {
			return fmt.Errorf(i18n.T("cmd.export.creating_snapshot_directory"), err)
		}
		defer os.RemoveAll(snapDir)
		if snap, err := snapshotDatabase(filesToArchive[0].src, snapDir); err != nil {
			fmt.Fprintf(w, "  %s\n", i18n.Tf("cmd.export.snapshot_failed", err))
		} else {
			filesToArchive[0].src = snap
		}
	}

	// Each file is read once: the checksum and the archive see the same bytes.
	h := sha256.New()
	var fileNames []string
	var contents [][]byte
	for _, f := range filesToArchive {
		data, err := os.ReadFile(f.src)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(w, "  Ignoré (absent): %s\n", f.name)
				continue
			}
			return fmt.Errorf("reading %s: %w", f.name, err)
		}
		h.Write(data)
		fileNames = append(fileNames, f.name)
		contents = append(contents, data)
	}
	checksum := hex.EncodeToString(h.Sum(nil))

	manifest := backupManifest{
		Version:   "1",
		CreatedAt: time.Now(),
		Checksum:  checksum,
		Files:     fileNames,
	}

	// Create archive
	out, err := os.Create(output)
	if err != nil {
		return fmt.Errorf(i18n.T("cmd.export.creating_archive"), err)
	}
	defer out.Close()

	gw := gzip.NewWriter(out)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Write manifest
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf(i18n.T("cmd.export.marshaling_manifest"), err)
	}
	if err := writeTarEntry(tw, "manifest.json", manifestData); err != nil {
		return fmt.Errorf(i18n.T("cmd.export.writing_manifest"), err)
	}

	// Write each file
	for i, name := range fileNames {
		if err := writeTarEntry(tw, name, contents[i]); err != nil {
			return fmt.Errorf("archiving %s: %w", name, err)
		}
		fmt.Fprintf(w, "  Ajouté: %s (%d KB)\n", name, len(contents[i])/1024)
	}

	// Explicitly close writers to catch flush errors (defers remain as safety net).
	if err := tw.Close(); err != nil {
		return fmt.Errorf(i18n.T("cmd.export.finalizing_tar"), err)
	}
	if err := gw.Close(); err != nil {
		return fmt.Errorf(i18n.T("cmd.export.finalizing_gzip"), err)
	}

	fmt.Fprint(w, i18n.Tf("cmd.export.backup_created", output))
	fmt.Fprint(w, i18n.Tf("cmd.export.checksum_sha_256", checksum))
	return nil
}

func runImport(cmd *cobra.Command, args []string) error {
	a := MustApp()
	archivePath := args[0]
	overwrite, _ := cmd.Flags().GetBool("overwrite")
	merge, _ := cmd.Flags().GetBool("merge")
	if merge && overwrite {
		return errors.New(i18n.T("cmd.import.merge_overwrite"))
	}

	fmt.Fprint(a.IO.Out, i18n.Tf("cmd.export.importing_from", archivePath))

	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf(i18n.T("cmd.export.opening_archive"), err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("décompression: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	// Extract files to a temp dir first
	tmpDir, err := os.MkdirTemp("", "oh-import-*")
	if err != nil {
		return fmt.Errorf(i18n.T("cmd.export.creating_temporary_directory"), err)
	}
	defer os.RemoveAll(tmpDir)

	var manifest *backupManifest
	extracted := map[string][]byte{}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf(i18n.T("cmd.export.reading_archive"), err)
		}
		const maxFileSize = 100 * 1024 * 1024 // 100 MB
		data, err := io.ReadAll(io.LimitReader(tr, maxFileSize))
		if err != nil {
			return fmt.Errorf("lecture %s: %w", header.Name, err)
		}

		if header.Name == "manifest.json" {
			manifest = &backupManifest{}
			if err := json.Unmarshal(data, manifest); err != nil {
				return fmt.Errorf(i18n.T("cmd.export.parsing_manifest"), err)
			}
		} else {
			extracted[header.Name] = data
		}
	}

	if manifest == nil {
		return errors.New(i18n.T("cmd.export.invalid_archive_manifest_json_missing"))
	}

	// Verify checksum
	h := sha256.New()
	for _, name := range manifest.Files {
		if data, ok := extracted[name]; ok {
			h.Write(data)
		}
	}
	actualChecksum := hex.EncodeToString(h.Sum(nil))
	if actualChecksum != manifest.Checksum {
		return fmt.Errorf(i18n.T("cmd.export.invalid_checksum_archive_corrupted_or_modified"), manifest.Checksum, actualChecksum)
	}
	fmt.Fprint(a.IO.Out, i18n.Tf("cmd.export.checksum_ok", manifest.Checksum[:16]+"..."))

	if merge {
		return importMerge(ctxOf(cmd), a, extracted)
	}

	// Check if existing data would be overwritten
	hubDir := config.HubDir()
	dbExists := fileExists(sqlite.DBPath())
	if dbExists && !overwrite {
		fmt.Fprint(a.IO.Out, i18n.T("cmd.export.warning_a_database_already_exists"))
		fmt.Fprint(a.IO.Out, i18n.T("cmd.export.use_overwrite_to_overwrite_or_merge"))
		fmt.Fprint(a.IO.Out, i18n.T("cmd.export.back_up_the_current_data_first"))
		return errors.New(i18n.T("cmd.export.cancelled_use_overwrite_to_force"))
	}

	// Restore files
	if err := os.MkdirAll(hubDir, 0o700); err != nil {
		return fmt.Errorf(i18n.T("cmd.export.creating_the_hub_directory"), err)
	}

	destinations := map[string]string{
		"oh.db":       sqlite.DBPath(),
		"hub.toml":    config.ConfigPath(),
		"secrets.enc": filepath.Join(hubDir, "secrets.enc"),
	}

	for name, data := range extracted {
		dest, ok := destinations[name]
		if !ok {
			continue
		}
		// Atomic write via temp file
		tmp := dest + ".import-tmp"
		if err := os.WriteFile(tmp, data, 0o600); err != nil {
			return fmt.Errorf("écriture %s: %w", name, err)
		}
		if name == "oh.db" {
			// A journal left by the replaced database must not be replayed on the
			// restored one.
			_ = os.Remove(dest + "-wal")
			_ = os.Remove(dest + "-shm")
		}
		if err := os.Rename(tmp, dest); err != nil {
			os.Remove(tmp)
			return fmt.Errorf("finalisation %s: %w", name, err)
		}
		fmt.Fprintf(a.IO.Out, "  Restauré: %s\n", name)
	}

	fmt.Fprint(a.IO.Out, i18n.Tf("cmd.export.import_complete_backup_of_restored", manifest.CreatedAt.Format("2006-01-02 15:04")))
	fmt.Fprint(a.IO.Out, i18n.T("cmd.export.run_oh_doctor_to_validate_the"))
	return nil
}

// importMerge adds the projects of the backup that the hub does not know yet
// and leaves everything else (existing projects, hub.toml, secrets) as is.
func importMerge(ctx context.Context, a *app.App, extracted map[string][]byte) error {
	data, ok := extracted["oh.db"]
	if !ok {
		return errors.New(i18n.T("cmd.import.merge_no_db"))
	}
	added, skipped, err := mergeBackupProjects(ctx, data, a.Projects)
	if err != nil {
		return err
	}
	for _, name := range added {
		fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.import.merge_added", name))
	}
	for _, name := range skipped {
		fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.import.merge_skipped", name))
	}
	fmt.Fprintf(a.IO.Out, "\n%s\n", i18n.Tf("cmd.import.merge_done", len(added), len(skipped)))
	return nil
}

// mergeBackupProjects copies into dst the projects of the backup database db
// whose id, name and path are all absent from dst; the others are reported as
// skipped (the existing project wins). Returns the names of each group.
func mergeBackupProjects(ctx context.Context, db []byte, dst domain.ProjectStore) (added, skipped []string, err error) {
	dir, err := os.MkdirTemp("", "oh-import-merge-*")
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "oh.db")
	if err := os.WriteFile(path, db, 0o600); err != nil {
		return nil, nil, err
	}
	backup, err := sqlite.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", i18n.T("cmd.import.merge_open"), err)
	}
	defer backup.Close()
	projects, err := sqlite.NewProjectStore(backup).List(ctx, "")
	if err != nil {
		return nil, nil, err
	}
	existing, err := dst.List(ctx, "")
	if err != nil {
		return nil, nil, err
	}
	known := map[string]bool{}
	for _, p := range existing {
		known["id:"+p.ID], known["name:"+p.Name], known["path:"+p.Path] = true, true, true
	}
	for i := range projects {
		p := &projects[i]
		if known["id:"+p.ID] || known["name:"+p.Name] || (p.Path != "" && known["path:"+p.Path]) {
			skipped = append(skipped, p.Name)
			continue
		}
		if err := dst.Create(ctx, p); err != nil {
			return added, skipped, fmt.Errorf("%s: %w", p.Name, err)
		}
		added = append(added, p.Name)
	}
	return added, skipped, nil
}

// writeTarEntry writes bytes to a tar archive under the given name.
func writeTarEntry(tw *tar.Writer, name string, data []byte) error {
	header := &tar.Header{
		Name:    name,
		Mode:    0o600,
		Size:    int64(len(data)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
