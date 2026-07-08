package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/tui/common"
)

var conventionsCmd = &cobra.Command{
	Use:   "conventions",
	Short: "Vérification des conventions d'équipe",
}

var conventionsCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Vérifie que le travail courant respecte les conventions",
	Long: `Vérifie la branche courante et les derniers commits par rapport
aux conventions documentées dans le wiki projet (docs/wiki/technical/conventions.md)
et le wiki d'équipe.

Affiche des warnings (non bloquants) si des écarts sont détectés.`,
	RunE: runConventionsCheck,
}

func init() {
	rootCmd.AddCommand(conventionsCmd)
	conventionsCmd.AddCommand(conventionsCheckCmd)
}

func runConventionsCheck(cmd *cobra.Command, args []string) error {
	a := MustApp()

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintln(a.IO.Out, common.Title.Render("  Conventions Check  "))
	fmt.Fprintln(a.IO.Out)

	cwd, _ := os.Getwd()
	var issues int

	// 1. Check branch naming
	branch := getCurrentBranch(cwd)
	if branch != "" {
		branchPattern := findBranchPattern(cwd)
		if branchPattern != "" {
			re, err := regexp.Compile(branchPattern)
			if err == nil {
				if re.MatchString(branch) {
					fmt.Fprintf(a.IO.Out, "  %s Branch : %s (conforme)\n",
						common.SuccessStyle.Render(common.IconSuccess), branch)
				} else {
					fmt.Fprintf(a.IO.Out, "  %s Branch : %s ne suit pas le pattern %s\n",
						common.WarningStyle.Render(common.IconWarning), branch, branchPattern)
					issues++
				}
			}
		} else {
			fmt.Fprintf(a.IO.Out, "  %s Branch : %s (pas de pattern configuré)\n",
				common.Subtitle.Render(common.IconDot), branch)
		}
	}

	// 2. Check last commit messages
	commits := getLastCommits(cwd, 5)
	commitPattern := findCommitPattern(cwd)
	if commitPattern != "" && len(commits) > 0 {
		re, err := regexp.Compile(commitPattern)
		if err == nil {
			nonConform := 0
			for _, c := range commits {
				if !re.MatchString(c) {
					nonConform++
				}
			}
			if nonConform == 0 {
				fmt.Fprintf(a.IO.Out, "  %s Commits : %d derniers conformes au format\n",
					common.SuccessStyle.Render(common.IconSuccess), len(commits))
			} else {
				fmt.Fprintf(a.IO.Out, "  %s Commits : %d/%d non conformes au pattern %s\n",
					common.WarningStyle.Render(common.IconWarning), nonConform, len(commits), commitPattern)
				issues++
			}
		}
	} else if len(commits) > 0 {
		fmt.Fprintf(a.IO.Out, "  %s Commits : %d récents (pas de format configuré)\n",
			common.Subtitle.Render(common.IconDot), len(commits))
	}

	// 3. Check if ticket is claimed (if team is enabled)
	if a.Config.Team.Enabled {
		// Try to detect ticket from branch name
		ticketRef := extractTicketFromBranch(branch)
		if ticketRef != "" {
			fmt.Fprintf(a.IO.Out, "  %s Ticket : %s détecté depuis la branche\n",
				common.SuccessStyle.Render(common.IconSuccess), ticketRef)
		}
	}

	// 4. Summary
	fmt.Fprintln(a.IO.Out)
	if issues == 0 {
		fmt.Fprintf(a.IO.Out, "  %s Tout est conforme\n",
			common.SuccessStyle.Render(common.IconSuccess))
	} else {
		fmt.Fprintf(a.IO.Out, "  %s %d warning(s) détecté(s)\n",
			common.WarningStyle.Render(common.IconWarning), issues)
	}

	return nil
}

// getCurrentBranch returns the current git branch name.
func getCurrentBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// getLastCommits returns the subjects of the last N commits.
func getLastCommits(dir string, n int) []string {
	cmd := exec.Command("git", "log", fmt.Sprintf("-%d", n), "--pretty=format:%s")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

// findBranchPattern looks for a branch naming pattern in the wiki conventions.
func findBranchPattern(dir string) string {
	content := readConventionsFile(dir)
	if content == "" {
		return ""
	}

	// Look for patterns like: branch_pattern = "..." or `pattern: ...`
	patterns := []string{
		`(?i)branch.*pattern.*[=:]\s*` + "`" + `([^` + "`" + `]+)` + "`",
		`(?i)branch.*pattern.*[=:]\s*"([^"]+)"`,
		`(?i)branch.*format.*[=:]\s*` + "`" + `([^` + "`" + `]+)` + "`",
	}
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		if m := re.FindStringSubmatch(content); len(m) > 1 {
			return m[1]
		}
	}

	// Default: conventional branching (feat/fix/chore + ticket reference)
	return ""
}

// findCommitPattern looks for a commit message format in the wiki conventions.
func findCommitPattern(dir string) string {
	content := readConventionsFile(dir)
	if content == "" {
		return ""
	}

	// Look for commit format specification
	patterns := []string{
		`(?i)commit.*pattern.*[=:]\s*` + "`" + `([^` + "`" + `]+)` + "`",
		`(?i)commit.*format.*[=:]\s*` + "`" + `([^` + "`" + `]+)` + "`",
	}
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		if m := re.FindStringSubmatch(content); len(m) > 1 {
			return m[1]
		}
	}

	// Check if "conventional commits" is mentioned
	if strings.Contains(strings.ToLower(content), "conventional commit") {
		return `^(feat|fix|docs|style|refactor|perf|test|chore|ci|build|revert)(\(.+\))?!?:\s.+`
	}

	return ""
}

// readConventionsFile reads the project's conventions wiki page.
func readConventionsFile(dir string) string {
	paths := []string{
		filepath.Join(dir, "docs", "wiki", "technical", "conventions.md"),
		filepath.Join(dir, "docs", "wiki", "conventions.md"),
		filepath.Join(dir, "CONVENTIONS.md"),
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err == nil {
			return string(data)
		}
	}
	return ""
}

// extractTicketFromBranch attempts to extract a ticket reference from a branch name.
// Common patterns: feat/SRU-142-description, fix/PROJ-123-thing
func extractTicketFromBranch(branch string) string {
	re := regexp.MustCompile(`(?i)(?:feat|fix|chore|refactor|docs)/([A-Z]+-\d+)`)
	if m := re.FindStringSubmatch(branch); len(m) > 1 {
		return m[1]
	}
	// Try plain ticket ref anywhere
	re2 := regexp.MustCompile(`([A-Z]{2,}-\d+)`)
	if m := re2.FindStringSubmatch(branch); len(m) > 1 {
		return m[1]
	}
	return ""
}
