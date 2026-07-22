package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

var repairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Diagnostiquer et réparer la base de données oh",
	Long: `Vérifie l'intégrité de la base de données SQLite.
En cas de corruption, tente une récupération et propose les options de restauration.`,
	RunE: runRepair,
}

func init() {
	rootCmd.AddCommand(repairCmd)
	repairCmd.Flags().Bool("auto", false, "Mode non-interactif (pour scripts)")
	repairCmd.Flags().Bool("check-only", false, "Vérifier uniquement, sans réparer")
}

func runRepair(cmd *cobra.Command, args []string) error {
	a := MustApp()
	checkOnly, _ := cmd.Flags().GetBool("check-only")
	dbPath := sqlite.DBPath()

	fmt.Fprintf(a.IO.Out, "Diagnostic de la base de données: %s\n\n", dbPath)

	// Check if DB exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Fprintf(a.IO.Out, "  Base de données absente — sera créée au prochain démarrage.\n")
		return nil
	}

	// Open DB for integrity check
	store, err := sqlite.Open(dbPath)
	if err != nil {
		fmt.Fprintf(a.IO.Out, "  ERREUR: Impossible d'ouvrir la base: %v\n", err)
		if checkOnly {
			return err
		}
		return attemptRecovery(a, dbPath)
	}
	defer store.Close()

	// Run integrity check
	fmt.Fprintf(a.IO.Out, "  Vérification de l'intégrité... ")
	if err := store.IntegrityCheck(); err != nil {
		fmt.Fprintf(a.IO.Out, "ÉCHEC\n")
		fmt.Fprintf(a.IO.Out, "  Détail: %v\n\n", err)
		if checkOnly {
			return fmt.Errorf("base de données corrompue: %w", err)
		}
		return attemptRecovery(a, dbPath)
	}
	fmt.Fprintf(a.IO.Out, "OK\n")

	// Show schema version
	version, err := store.SchemaVersion()
	if err == nil {
		fmt.Fprintf(a.IO.Out, "  Version du schéma: v%d\n", version)
	}

	// Count projects and sessions
	projects, _ := a.Projects.List(cmd.Context(), "")
	fmt.Fprintf(a.IO.Out, "  Projets enregistrés: %d\n", len(projects))

	fmt.Fprintf(a.IO.Out, "\nBase de données saine. Aucune action nécessaire.\n")
	return nil
}

// attemptRecovery tries to recover a corrupted database.
func attemptRecovery(a *app.App, dbPath string) error {
	fmt.Fprintf(a.IO.Out, "Tentative de récupération...\n\n")

	// Step 1: Backup the corrupt file
	backupPath := dbPath + ".corrupt-backup"
	if err := copyFile(dbPath, backupPath); err == nil {
		fmt.Fprintf(a.IO.Out, "  Sauvegarde créée: %s\n", backupPath)
	}

	// Step 2: Look for a recent export backup
	hubDir := config.HubDir()
	backupFiles, _ := filepath.Glob(filepath.Join(hubDir, "oh-backup-*.tar.gz"))
	if len(backupFiles) == 0 {
		backupFiles, _ = filepath.Glob("oh-backup-*.tar.gz")
	}

	fmt.Fprintf(a.IO.Out, "\nOptions de récupération:\n\n")

	if len(backupFiles) > 0 {
		fmt.Fprintf(a.IO.Out, "  1. Restaurer depuis un backup:\n")
		for _, bf := range backupFiles {
			fmt.Fprintf(a.IO.Out, "     oh import %s\n", bf)
		}
		fmt.Fprintln(a.IO.Out)
	}

	fmt.Fprintf(a.IO.Out, "  2. Réinitialiser (perte des données):\n")
	fmt.Fprintf(a.IO.Out, "     rm %s && oh init\n\n", dbPath)

	fmt.Fprintf(a.IO.Out, "  3. Re-enregistrer vos projets manuellement:\n")
	fmt.Fprintf(a.IO.Out, "     rm %s\n", dbPath)
	fmt.Fprintf(a.IO.Out, "     oh project add <nom> <chemin> pour chaque projet\n\n")

	return fmt.Errorf("base de données corrompue — voir les options de récupération ci-dessus")
}

// copyFile copies src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}
