package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rivo/tview"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	providerPkg "github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: i18n.T("cmd.init.short"),
	Long:  i18n.T("cmd.init.long"),
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

// ─────────────────────────────────────────────────────────────────────────────
// Main init flow
// ─────────────────────────────────────────────────────────────────────────────
//
// runInit delegates to the same buildFirstRunInlineWizard used by the TUI,
// mounted inside a standalone tview.Application via RunInlineWizardStandalone.
// This ensures a single source of truth for the init wizard logic.

func runInit(cmd *cobra.Command, args []string) error {
	// ── Guard: warn if hub is already initialized ────────────────────────────
	if existing, err := config.Load(); err == nil && existing.CLI.SetupDone {
		fmt.Fprintf(os.Stderr, "%s\n", i18n.T("cmd.init.already_initialized"))
		fmt.Fprintf(os.Stderr, "%s ", i18n.T("cmd.init.confirm_reinit"))
		var answer string
		if _, err := fmt.Fscanln(os.Stdin, &answer); err != nil || (answer != "y" && answer != "Y") {
			return nil
		}
		// Backup existing config before overwriting.
		cfgPath := config.ConfigPath()
		if data, err := os.ReadFile(cfgPath); err == nil {
			if writeErr := os.WriteFile(cfgPath+".bak", data, 0o600); writeErr != nil {
				slog.Warn("failed to backup config", "error", writeErr)
			}
		}
	}

	// ── Lockfile: prevent concurrent wizard runs ─────────────────────────────
	lockPath := filepath.Join(config.HubDir(), "init.lock")
	if err := acquireInitLock(lockPath); err != nil {
		return fmt.Errorf("another wizard is already running: %w", err)
	}
	defer releaseInitLock(lockPath)

	// ── Pre-flight: check git availability ───────────────────────────────────
	if _, err := exec.LookPath("git"); err != nil {
		fmt.Fprintf(os.Stderr, "  %s %s\n", "⚠", i18n.T("cmd.init.git_not_found"))
	}

	// ── Ensure a minimal app exists for the wizard ───────────────────────────
	if err := os.MkdirAll(config.HubDir(), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	config.Reset()
	var a *app.App
	if err := initApp(); err == nil {
		a = MustApp()
	} else {
		// First run: create a minimal hub.toml so initApp can succeed.
		if saveErr := config.Save(&config.Config{}); saveErr != nil {
			return fmt.Errorf("writing initial config: %w", saveErr)
		}
		config.Reset()
		if err := initApp(); err != nil {
			return fmt.Errorf("initializing app: %w", err)
		}
		a = MustApp()
	}

	// ── Build and run the wizard ─────────────────────────────────────────────
	wizard := buildFirstRunInlineWizard(a)
	result := views.RunInlineWizardStandalone(wizard)

	if result.Aborted {
		return nil
	}
	return result.Err
}

// ─────────────────────────────────────────────────────────────────────────────
// Provider form helpers (shared with cmd/provider.go)
// ─────────────────────────────────────────────────────────────────────────────

// bedrockStepIndex finds a WizardStep by ID and returns its index, or -1.
func bedrockStepIndex(steps *[]views.WizardStep, id string) int {
	for i, s := range *steps {
		if s.ID == id {
			return i
		}
	}
	return -1
}

// addBedrockRegionDropDown appends a region DropDown (with "Custom..." option)
// to the form. It mutates the shared state via pointers and calls rerenderSafe
// when the selection changes so the form rebuilds with/without the custom input.
func addBedrockRegionDropDown(form *tview.Form, region *string, regionIdx *int, custom *bool, rerenderSafe func()) {
	regionLabels := append(
		[]string{i18n.T("cmd.init.wizard_region_placeholder")},
		providerPkg.BedrockRegionLabels()...,
	)
	regionLabels = append(regionLabels, i18n.T("cmd.init.wizard_custom_region"))
	customIdx := len(regionLabels) - 1

	form.AddDropDown(
		i18n.T("cmd.init.wizard_aws_region"),
		regionLabels, *regionIdx,
		func(_ string, idx int) {
			if *regionIdx == idx {
				return // no change — avoid spurious rerender
			}
			wasCustom := *custom
			if idx == 0 {
				// Placeholder selected — clear region.
				*custom = false
				*region = ""
			} else if idx == customIdx {
				*custom = true
				*region = ""
			} else if idx > 0 {
				*custom = false
				*region = providerPkg.BedrockRegions[idx-1].Code // -1 for placeholder offset
			}
			*regionIdx = idx
			// Only rerender when the custom input field needs to appear/disappear.
			if *custom != wasCustom {
				rerenderSafe()
			}
		},
	)
	if *custom {
		form.AddInputField(i18n.T("cmd.init.wizard_custom_region_input"), *region, 0, nil, func(t string) { *region = t })
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Git helpers
// ─────────────────────────────────────────────────────────────────────────────

// detectBranchPatternHeuristic scans local and remote branches in dir to infer
// the project's branch naming convention and returns a fmt.Sprintf-style pattern
// (e.g. "feat/%s") suitable for [worktree].branch_pattern.
//
// It counts the frequency of common prefixes in all branch names, picks the
// dominant one (>50% of non-main/master branches), and returns "<prefix>/%s".
// Returns an empty string when no dominant pattern is found or on git error.
func detectBranchPatternHeuristic(dir string) string {
	cmd := exec.Command("git", "branch", "-a", "--format=%(refname:short)")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Known prefixes to look for (ordered by priority)
	knownPrefixes := []string{
		"feat", "feature", "fix", "bugfix", "hotfix",
		"chore", "refactor", "docs", "test", "ci",
	}

	// Skip main/master/HEAD lines
	skip := map[string]bool{
		"main": true, "master": true, "develop": true, "HEAD": true,
	}

	counts := make(map[string]int)
	total := 0

	for _, raw := range strings.Split(string(out), "\n") {
		branch := strings.TrimSpace(raw)
		// Strip "origin/" prefix for remote branches
		branch = strings.TrimPrefix(branch, "origin/")
		if branch == "" || skip[branch] {
			continue
		}
		total++
		for _, prefix := range knownPrefixes {
			if strings.HasPrefix(branch, prefix+"/") {
				counts[prefix]++
				break
			}
		}
	}

	if total == 0 {
		return ""
	}

	// Find the prefix with the highest count
	type kv struct {
		prefix string
		count  int
	}
	var ranked []kv
	for _, prefix := range knownPrefixes {
		if counts[prefix] > 0 {
			ranked = append(ranked, kv{prefix, counts[prefix]})
		}
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].count > ranked[j].count })

	if len(ranked) == 0 {
		return ""
	}

	// Only suggest a pattern if the dominant prefix represents more than 50% of branches
	dominant := ranked[0]
	if dominant.count*2 > total {
		return dominant.prefix + "/%s"
	}

	return ""
}

// ─────────────────────────────────────────────────────────────────────────────
// Lockfile helpers
// ─────────────────────────────────────────────────────────────────────────────

// acquireInitLock creates a lockfile to prevent concurrent wizard runs.
// The lock is considered stale after 1 hour.
func acquireInitLock(path string) error {
	_ = os.MkdirAll(filepath.Dir(path), 0o755)

	// Check for stale lock (older than 1 hour)
	if info, err := os.Stat(path); err == nil {
		if time.Since(info.ModTime()) > time.Hour {
			if rmErr := os.Remove(path); rmErr != nil {
				return fmt.Errorf("removing stale lockfile %s: %w (delete it manually to proceed)", path, rmErr)
			}
		} else {
			return fmt.Errorf("lockfile %s exists (created %s ago)", path, time.Since(info.ModTime()).Truncate(time.Second))
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	return f.Close()
}

// releaseInitLock removes the lockfile.
func releaseInitLock(path string) {
	_ = os.Remove(path)
}
