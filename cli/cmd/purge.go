package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/storage/keychain"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

var purgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Suppression totale du hub et de ses données",
	RunE:  runPurge,
}

func init() {
	rootCmd.AddCommand(purgeCmd)
	purgeCmd.Flags().Bool("dry-run", false, i18n.T("cmd.purge.flags.dry_run"))
	purgeCmd.Flags().Bool("force", false, i18n.T("cmd.purge.flags.force"))
	purgeCmd.Flags().Bool("keep-binary", false, i18n.T("cmd.purge.flags.keep_binary"))
	purgeCmd.Flags().Bool("include-opencode", false, i18n.T("cmd.purge.flags.include_opencode"))
}

// purgeInventory collects all items that would be deleted.
type purgeInventory struct {
	// Keychain secrets to delete
	secrets []keychain.SecretEntry

	// Projects with their deploy artifacts
	projects []purgeProject

	// Hub directory path
	hubDir string

	// OpenCode data paths (only if --include-opencode)
	opencodeDataDir   string // ~/.local/share/opencode/
	opencodeConfigDir string // ~/.config/opencode/

	// Binary info
	binaryPath string
	isHomebrew bool
	keepBinary bool
	includeOC  bool
}

type purgeProject struct {
	name         string
	path         string
	hasOpencode  bool // .opencode/ exists
	hasOCJson    bool // opencode.json exists
	hasBeadsDolt bool // .beads/dolt/ exists
}

func runPurge(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	force, _ := cmd.Flags().GetBool("force")
	keepBinary, _ := cmd.Flags().GetBool("keep-binary")
	includeOC, _ := cmd.Flags().GetBool("include-opencode")

	out := os.Stdout
	ctx := cmd.Context()

	// ── Step 1: Build inventory ──
	inv := buildPurgeInventory(ctx, keepBinary, includeOC)

	// ── Step 2: Display inventory ──
	printPurgeInventory(out, inv, dryRun)

	if dryRun {
		return nil
	}

	// ── Step 3: Confirmation ──
	if !force {
		// Offer backup first
		var wantBackup bool
		_ = theme.NewForm(huh.NewGroup(huh.NewConfirm().
			Title(i18n.T("cmd.purge.confirm_backup")).
			Affirmative("Yes").
			Negative("No").
			Value(&wantBackup))).Run()

		if wantBackup {
			fmt.Fprintf(out, "\n")
			if err := runExport(cmd, nil); err != nil {
				fmt.Fprintf(out, "  %s %s: %v\n",
					theme.ErrorStyle.Render(theme.IconError),
					i18n.T("cmd.purge.backup_failed"), err)
			} else {
				fmt.Fprintf(out, "\n")
			}
		}

		// Final confirmation
		var confirm bool
		_ = theme.NewForm(huh.NewGroup(huh.NewConfirm().
			Title(i18n.T("cmd.purge.confirm_final")).
			Description(i18n.T("cmd.purge.confirm_warning")).
			Affirmative("Yes").
			Negative("No").
			Value(&confirm))).Run()

		if !confirm {
			fmt.Fprintf(out, "\n%s\n", theme.Subtitle.Render(i18n.T("cmd.purge.cancelled")))
			return nil
		}
	}

	fmt.Fprintf(out, "\n")

	totalSteps := countPurgeSteps(inv)
	step := 0

	// ── Step 4: Kill processes ──
	step++
	printStep(out, step, totalSteps, i18n.T("cmd.purge.section.processes"))
	killPurgeProcesses(out)

	// ── Step 5: Delete keychain secrets ──
	if len(inv.secrets) > 0 {
		step++
		printStep(out, step, totalSteps, i18n.T("cmd.purge.section.secrets"))
		deletePurgeSecrets(ctx, out, inv.secrets)
	}

	// ── Step 6: Clean project deployments + beads ──
	if len(inv.projects) > 0 {
		step++
		printStep(out, step, totalSteps, i18n.T("cmd.purge.section.projects"))
		cleanPurgeProjects(out, inv.projects)
	}

	// ── Step 7: OpenCode global data ──
	if inv.includeOC {
		step++
		printStep(out, step, totalSteps, i18n.T("cmd.purge.section.opencode"))
		cleanOpenCodeData(out, inv.opencodeDataDir, inv.opencodeConfigDir)
	}

	// ── Step 8: Remove hub directory ──
	step++
	printStep(out, step, totalSteps, i18n.T("cmd.purge.section.hub"))
	removeHubDir(out, inv.hubDir)

	// ── Step 9: Uninstall binary ──
	if !inv.keepBinary {
		step++
		printStep(out, step, totalSteps, i18n.T("cmd.purge.section.binary"))
		uninstallBinary(out, inv)
	}

	// ── Done ──
	fmt.Fprintf(out, "\n%s %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		i18n.T("cmd.purge.done"))

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Inventory
// ─────────────────────────────────────────────────────────────────────────────

func buildPurgeInventory(ctx context.Context, keepBinary, includeOC bool) purgeInventory {
	inv := purgeInventory{
		hubDir:     config.HubDir(),
		keepBinary: keepBinary,
		includeOC:  includeOC,
	}

	// Keychain secrets
	if ks, ok := resolveKeychainStore(); ok {
		entries, err := ks.ListAll(ctx)
		if err == nil {
			inv.secrets = entries
		}
	}

	// Projects from DB (best-effort — DB may be broken)
	if store != nil {
		ps := sqlite.NewProjectStore(store)
		projects, err := ps.List(ctx, "")
		if err == nil {
			for _, p := range projects {
				pp := purgeProject{
					name: p.Name,
					path: p.Path,
				}
				if p.Path != "" {
					_, err := os.Stat(filepath.Join(p.Path, ".opencode"))
					pp.hasOpencode = err == nil
					_, err = os.Stat(filepath.Join(p.Path, "opencode.json"))
					pp.hasOCJson = err == nil
					pp.hasBeadsDolt = beads.IsInitialized(p.Path) && dirExists(filepath.Join(p.Path, ".beads", "dolt"))
				}
				inv.projects = append(inv.projects, pp)
			}
		}
	}

	// OpenCode paths
	if includeOC {
		home, _ := os.UserHomeDir()
		inv.opencodeDataDir = filepath.Join(home, ".local", "share", "opencode")
		inv.opencodeConfigDir = filepath.Join(home, ".config", "opencode")
	}

	// Binary detection
	if !keepBinary {
		inv.binaryPath, inv.isHomebrew = detectBinaryInstall()
	}

	return inv
}

// resolveKeychainStore returns a keychain.Store if the OS keychain is available.
func resolveKeychainStore() (*keychain.Store, bool) {
	if err := keychain.Probe(); err != nil {
		return nil, false
	}
	ks := keychain.New(config.HubDir())
	return ks, true
}

func detectBinaryInstall() (string, bool) {
	// Check Homebrew first
	out, err := exec.Command("brew", "list", "openhub").CombinedOutput()
	if err == nil && len(out) > 0 {
		// Find the actual binary path from brew output
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasSuffix(strings.TrimSpace(line), "/oh") {
				return strings.TrimSpace(line), true
			}
		}
		return "openhub (homebrew)", true
	}

	// Fallback: find our own binary
	exe, err := os.Executable()
	if err == nil {
		return exe, false
	}
	return "", false
}

// ─────────────────────────────────────────────────────────────────────────────
// Display
// ─────────────────────────────────────────────────────────────────────────────

func printPurgeInventory(out *os.File, inv purgeInventory, dryRun bool) {
	if dryRun {
		fmt.Fprintf(out, "\n  %s\n\n",
			theme.WarningStyle.Render(i18n.T("cmd.purge.dry_run_header")))
	} else {
		fmt.Fprintf(out, "\n")
	}

	// Section: Secrets
	fmt.Fprintf(out, "  %s\n", theme.Bold.Render(
		i18n.Tf("cmd.purge.section.secrets", "openhub-oh")))
	if len(inv.secrets) == 0 {
		fmt.Fprintf(out, "      %s\n", theme.Subtitle.Render(i18n.T("cmd.purge.none_detected")))
	} else {
		for _, s := range inv.secrets {
			scope := s.Scope
			if scope == "" {
				scope = "global"
			}
			fmt.Fprintf(out, "      %s %s (%s)\n",
				theme.ErrorStyle.Render(theme.IconError), s.Key, scope)
		}
	}

	// Section: Projects
	fmt.Fprintf(out, "\n  %s\n", theme.Bold.Render(i18n.T("cmd.purge.section.projects")))
	if len(inv.projects) == 0 {
		fmt.Fprintf(out, "      %s\n", theme.Subtitle.Render(i18n.T("cmd.purge.none_detected")))
	} else {
		for _, p := range inv.projects {
			if p.hasOpencode {
				fmt.Fprintf(out, "      %s %s/.opencode/\n",
					theme.ErrorStyle.Render(theme.IconError), p.path)
			}
			if p.hasOCJson {
				fmt.Fprintf(out, "      %s %s/opencode.json\n",
					theme.ErrorStyle.Render(theme.IconError), p.path)
			}
			if p.hasBeadsDolt {
				fmt.Fprintf(out, "      %s %s/.beads/dolt/\n",
					theme.ErrorStyle.Render(theme.IconError), p.path)
			}
			if !p.hasOpencode && !p.hasOCJson && !p.hasBeadsDolt {
				fmt.Fprintf(out, "      %s %s (%s)\n",
					theme.Subtitle.Render(theme.IconPending), p.name,
					i18n.T("cmd.purge.nothing_to_clean"))
			}
		}
	}

	// Section: Hub directory
	fmt.Fprintf(out, "\n  %s\n", theme.Bold.Render(i18n.T("cmd.purge.section.hub")))
	if dirExists(inv.hubDir) {
		size := dirSizeHuman(inv.hubDir)
		fmt.Fprintf(out, "      %s %s/ (%s)\n",
			theme.ErrorStyle.Render(theme.IconError), inv.hubDir, size)
	} else {
		fmt.Fprintf(out, "      %s\n", theme.Subtitle.Render(i18n.T("cmd.purge.none_detected")))
	}

	// Section: OpenCode (conditional)
	if inv.includeOC {
		fmt.Fprintf(out, "\n  %s\n", theme.Bold.Render(i18n.T("cmd.purge.section.opencode")))
		printed := false
		if dirExists(inv.opencodeDataDir) {
			size := dirSizeHuman(inv.opencodeDataDir)
			fmt.Fprintf(out, "      %s %s/ (%s)\n",
				theme.ErrorStyle.Render(theme.IconError), inv.opencodeDataDir, size)
			printed = true
		}
		if dirExists(inv.opencodeConfigDir) {
			fmt.Fprintf(out, "      %s %s/\n",
				theme.ErrorStyle.Render(theme.IconError), inv.opencodeConfigDir)
			printed = true
		}
		if !printed {
			fmt.Fprintf(out, "      %s\n", theme.Subtitle.Render(i18n.T("cmd.purge.none_detected")))
		}
	}

	// Section: Binary
	if !inv.keepBinary {
		fmt.Fprintf(out, "\n  %s\n", theme.Bold.Render(i18n.T("cmd.purge.section.binary")))
		if inv.binaryPath != "" {
			method := "manual"
			if inv.isHomebrew {
				method = "homebrew"
			}
			fmt.Fprintf(out, "      %s %s (%s)\n",
				theme.ErrorStyle.Render(theme.IconError), inv.binaryPath, method)
		} else {
			fmt.Fprintf(out, "      %s\n", theme.Subtitle.Render(i18n.T("cmd.purge.none_detected")))
		}
	}

	fmt.Fprintf(out, "\n")
}

func printStep(out *os.File, current, total int, label string) {
	fmt.Fprintf(out, "  [%d/%d] %s... ", current, total, label)
}

func printStepDone(out *os.File, detail string) {
	if detail != "" {
		fmt.Fprintf(out, "%s (%s)\n", theme.SuccessStyle.Render(theme.IconSuccess), detail)
	} else {
		fmt.Fprintf(out, "%s\n", theme.SuccessStyle.Render(theme.IconSuccess))
	}
}

func printStepSkipped(out *os.File) {
	fmt.Fprintf(out, "%s\n", theme.Subtitle.Render(i18n.T("cmd.purge.skipped")))
}

// ─────────────────────────────────────────────────────────────────────────────
// Execution steps
// ─────────────────────────────────────────────────────────────────────────────

func killPurgeProcesses(out *os.File) {
	killed := 0
	for _, pattern := range []string{"oh serve", "oh mcp"} {
		// Use pkill with -f (full command line match)
		cmd := exec.Command("pkill", "-f", pattern)
		if err := cmd.Run(); err == nil {
			killed++
		}
	}
	if killed > 0 {
		printStepDone(out, fmt.Sprintf("%d", killed))
	} else {
		printStepDone(out, i18n.T("cmd.purge.none_detected"))
	}
}

func deletePurgeSecrets(ctx context.Context, out *os.File, secrets []keychain.SecretEntry) {
	deleted := 0
	for _, s := range secrets {
		scope := s.Scope
		if scope == "" {
			scope = "global"
		}
		ks, ok := resolveKeychainStore()
		if !ok {
			break
		}
		if err := ks.DeleteScoped(ctx, s.Key, scope); err == nil {
			deleted++
		}
	}
	printStepDone(out, i18n.Tf("cmd.purge.secrets_deleted", deleted))
}

func cleanPurgeProjects(out *os.File, projects []purgeProject) {
	cleaned := 0
	for _, p := range projects {
		if p.path == "" {
			continue
		}
		if p.hasOpencode {
			_ = os.RemoveAll(filepath.Join(p.path, ".opencode"))
		}
		if p.hasOCJson {
			_ = os.Remove(filepath.Join(p.path, "opencode.json"))
		}
		if p.hasBeadsDolt {
			// Remove dolt database and runtime files, keep git-tracked files
			_ = os.RemoveAll(filepath.Join(p.path, ".beads", "dolt"))
			_ = os.RemoveAll(filepath.Join(p.path, ".beads", "export-state"))
			// Remove dolt-server runtime files
			beadsDir := filepath.Join(p.path, ".beads")
			entries, _ := os.ReadDir(beadsDir)
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), "dolt-server.") {
					_ = os.Remove(filepath.Join(beadsDir, e.Name()))
				}
			}
		}
		cleaned++
	}
	printStepDone(out, i18n.Tf("cmd.purge.projects_cleaned", cleaned))
}

func cleanOpenCodeData(out *os.File, dataDir, configDir string) {
	if dirExists(dataDir) {
		_ = os.RemoveAll(dataDir)
	}
	if dirExists(configDir) {
		_ = os.RemoveAll(configDir)
	}
	printStepDone(out, "")
}

func removeHubDir(out *os.File, hubDir string) {
	// Close the global SQLite store before deleting the directory
	if store != nil {
		store.Close()
		store = nil
	}

	if dirExists(hubDir) {
		if err := os.RemoveAll(hubDir); err != nil {
			fmt.Fprintf(out, "%s %v\n", theme.ErrorStyle.Render(theme.IconError), err)
			return
		}
	}
	printStepDone(out, "")
}

func uninstallBinary(out *os.File, inv purgeInventory) {
	if inv.binaryPath == "" {
		printStepSkipped(out)
		return
	}

	if inv.isHomebrew {
		cmd := exec.Command("brew", "uninstall", "openhub")
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(out, "%s brew uninstall: %v\n",
				theme.ErrorStyle.Render(theme.IconError), err)
			return
		}
		printStepDone(out, "homebrew")
		return
	}

	// Manual removal of the binary
	if err := os.Remove(inv.binaryPath); err != nil {
		fmt.Fprintf(out, "%s %v\n", theme.ErrorStyle.Render(theme.IconError), err)
		return
	}
	printStepDone(out, inv.binaryPath)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func countPurgeSteps(inv purgeInventory) int {
	n := 1 // processes (always)
	if len(inv.secrets) > 0 {
		n++
	}
	if len(inv.projects) > 0 {
		n++
	}
	if inv.includeOC {
		n++
	}
	n++ // hub dir (always)
	if !inv.keepBinary {
		n++
	}
	return n
}

func dirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func dirSizeHuman(path string) string {
	var size int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip unreadable entries during size estimation
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return formatBytes(size)
}

func formatBytes(b int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// Ensure unused imports are referenced for compilation.
var (
	_ = domain.ErrNotFound
	_ = opencode.DefaultDBPath
	_ = sqlite.DBPath
)
