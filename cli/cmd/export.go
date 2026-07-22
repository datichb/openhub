package cmd

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/config"
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
	Use:   "import [fichier]",
	Short: "Restaurer depuis une archive de backup",
	Args:  cobra.ExactArgs(1),
	RunE:  runImport,
}

func init() {
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(importCmd)

	exportCmd.Flags().StringP("output", "o", "", "Chemin de sortie (défaut: ./oh-backup-<date>.tar.gz)")
	importCmd.Flags().Bool("overwrite", false, "Écraser les données existantes sans confirmation")
	importCmd.Flags().Bool("merge", false, "Fusionner avec les données existantes (projets uniquement)")
}

func runExport(cmd *cobra.Command, args []string) error {
	a := MustApp()
	hubDir := config.HubDir()

	output, _ := cmd.Flags().GetString("output")
	if output == "" {
		output = fmt.Sprintf("oh-backup-%s.tar.gz", time.Now().Format("2006-01-02"))
	}

	fmt.Fprintf(a.IO.Out, "Export vers %s...\n", output)

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

	// Compute checksum over all files
	h := sha256.New()
	var fileNames []string
	for _, f := range filesToArchive {
		data, err := os.ReadFile(f.src)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(a.IO.Out, "  Ignoré (absent): %s\n", f.name)
				continue
			}
			return fmt.Errorf("reading %s: %w", f.name, err)
		}
		h.Write(data)
		fileNames = append(fileNames, f.name)
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
		return fmt.Errorf("creating archive: %w", err)
	}
	defer out.Close()

	gw := gzip.NewWriter(out)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Write manifest
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	if err := writeTarEntry(tw, "manifest.json", manifestData); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	// Write each file
	for _, f := range filesToArchive {
		data, err := os.ReadFile(f.src)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("reading %s: %w", f.name, err)
		}
		if err := writeTarEntry(tw, f.name, data); err != nil {
			return fmt.Errorf("archiving %s: %w", f.name, err)
		}
		fmt.Fprintf(a.IO.Out, "  Ajouté: %s (%d KB)\n", f.name, len(data)/1024)
	}

	fmt.Fprintf(a.IO.Out, "\nBackup créé: %s\n", output)
	fmt.Fprintf(a.IO.Out, "Checksum SHA-256: %s\n", checksum)
	return nil
}

func runImport(cmd *cobra.Command, args []string) error {
	a := MustApp()
	archivePath := args[0]
	overwrite, _ := cmd.Flags().GetBool("overwrite")

	fmt.Fprintf(a.IO.Out, "Import depuis %s...\n", archivePath)

	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("ouverture archive: %w", err)
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
		return fmt.Errorf("création répertoire temp: %w", err)
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
			return fmt.Errorf("lecture archive: %w", err)
		}
		const maxFileSize = 100 * 1024 * 1024 // 100 MB
		data, err := io.ReadAll(io.LimitReader(tr, maxFileSize))
		if err != nil {
			return fmt.Errorf("lecture %s: %w", header.Name, err)
		}

		if header.Name == "manifest.json" {
			manifest = &backupManifest{}
			if err := json.Unmarshal(data, manifest); err != nil {
				return fmt.Errorf("parsing manifest: %w", err)
			}
		} else {
			extracted[header.Name] = data
		}
	}

	if manifest == nil {
		return fmt.Errorf("archive invalide: manifest.json manquant")
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
		return fmt.Errorf("checksum invalide — archive corrompue ou modifiée\n  attendu: %s\n  obtenu:  %s",
			manifest.Checksum, actualChecksum)
	}
	fmt.Fprintf(a.IO.Out, "  Checksum OK (%s)\n", manifest.Checksum[:16]+"...")

	// Check if existing data would be overwritten
	hubDir := config.HubDir()
	dbExists := fileExists(sqlite.DBPath())
	if dbExists && !overwrite {
		fmt.Fprintf(a.IO.Out, "\nAttention: une base de données existe déjà.\n")
		fmt.Fprintf(a.IO.Out, "Utilisez --overwrite pour écraser ou --merge pour fusionner.\n")
		fmt.Fprintf(a.IO.Out, "Backup actuel recommandé: oh export --output oh-backup-pre-import.tar.gz\n")
		return fmt.Errorf("annulé — utilisez --overwrite pour forcer")
	}

	// Restore files
	if err := os.MkdirAll(hubDir, 0o700); err != nil {
		return fmt.Errorf("création hubDir: %w", err)
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
		if err := os.Rename(tmp, dest); err != nil {
			os.Remove(tmp)
			return fmt.Errorf("finalisation %s: %w", name, err)
		}
		fmt.Fprintf(a.IO.Out, "  Restauré: %s\n", name)
	}

	fmt.Fprintf(a.IO.Out, "\nImport terminé. Backup du %s restauré.\n", manifest.CreatedAt.Format("2006-01-02 15:04"))
	fmt.Fprintf(a.IO.Out, "Lancez 'oh doctor' pour valider l'installation.\n")
	return nil
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
