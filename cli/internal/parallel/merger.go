package parallel

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// MergeResult holds the outcome of a merge attempt.
type MergeResult struct {
	TicketID string
	Branch   string
	Success  bool
	Message  string
	Conflict bool
}

// Merger handles the post-session merge of parallel branches.
type Merger struct {
	state       *ParallelState
	projectPath string
	config      Config
	out         io.Writer
	in          io.Reader
}

// NewMerger creates a new merger.
func NewMerger(state *ParallelState, projectPath string, cfg Config) *Merger {
	return &Merger{
		state:       state,
		projectPath: projectPath,
		config:      cfg,
		out:         os.Stdout,
		in:          os.Stdin,
	}
}

// SetOutput sets the output writer (for consistency with CLI patterns).
func (m *Merger) SetOutput(w io.Writer) {
	m.out = w
}

// SetInput sets the input reader (for testing).
func (m *Merger) SetInput(r io.Reader) {
	m.in = r
}

// ProposeMerge attempts to merge completed sessions back into the base branch.
// For mergeable tasks (beads tickets, sweep tasks): sequential merge with conflict
// detection and human validation. For external tasks: just reports the branches.
//
// The isMergeable callback determines which sessions are auto-merge candidates.
// Callers typically pass task.Task.IsMergeable() or a simple prefix check.
func (m *Merger) ProposeMerge(isMergeable func(ticketID string) bool) ([]MergeResult, error) {
	snap := m.state.Snapshot()

	// Separate mergeable vs external sessions
	var mergeableSessions, externalSessions []SessionInfo
	for _, sess := range snap.Sessions {
		if sess.Status != StatusCompleted {
			continue
		}
		if isMergeable(sess.TicketID) {
			mergeableSessions = append(mergeableSessions, sess)
		} else {
			externalSessions = append(externalSessions, sess)
		}
	}

	var results []MergeResult

	// Sort mergeable: priority first
	sortByPriority(mergeableSessions)

	// Merge mergeable sessions sequentially
	if len(mergeableSessions) > 0 {
		fmt.Fprintf(m.out, "\n  Merge des sessions (%d branches) :\n\n", len(mergeableSessions))

		baseBranch, err := DetectBaseBranch(m.projectPath)
		if err != nil {
			baseBranch = "main"
		}

		for _, sess := range mergeableSessions {
			result := m.mergeSession(sess, baseBranch)
			results = append(results, result)

			if result.Conflict {
				// Stop on conflict — user must resolve
				break
			}
		}
	}

	// Report external branches (never merge)
	for _, sess := range externalSessions {
		results = append(results, MergeResult{
			TicketID: sess.TicketID,
			Branch:   sess.Branch,
			Success:  true,
			Message:  fmt.Sprintf("Branche %s prête pour MR/PR (merge manuel)", sess.Branch),
		})
	}

	return results, nil
}

// mergeSession attempts to merge a single session branch into base.
func (m *Merger) mergeSession(sess SessionInfo, baseBranch string) MergeResult {
	result := MergeResult{
		TicketID: sess.TicketID,
		Branch:   sess.Branch,
	}

	// Show what we're about to do
	fmt.Fprintf(m.out, "  → Merge %s (%s) into %s\n", sess.TicketID, sess.Branch, baseBranch)

	// Check if branch has changes vs base
	diffCmd := exec.Command("git", "log", "--oneline", fmt.Sprintf("%s..%s", baseBranch, sess.Branch))
	diffCmd.Dir = m.projectPath
	diffOut, err := diffCmd.Output()
	if err != nil || strings.TrimSpace(string(diffOut)) == "" {
		result.Success = true
		result.Message = "No changes to merge"
		fmt.Fprintf(m.out, "    ✓ Pas de changements à merger\n")
		return result
	}

	// Show diff summary
	statCmd := exec.Command("git", "diff", "--stat", fmt.Sprintf("%s...%s", baseBranch, sess.Branch))
	statCmd.Dir = m.projectPath
	statOut, _ := statCmd.Output()
	if len(statOut) > 0 {
		fmt.Fprintf(m.out, "    Diff:\n")
		for _, line := range strings.Split(strings.TrimSpace(string(statOut)), "\n") {
			fmt.Fprintf(m.out, "      %s\n", line)
		}
	}

	// Ask for human confirmation
	fmt.Fprintf(m.out, "\n    Merger cette branche ? [Y/n/s(kip)] ")
	reader := bufio.NewReader(m.in)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	if response == "n" {
		result.Success = false
		result.Message = "Merge refusé par l'utilisateur"
		fmt.Fprintf(m.out, "    ⊘ Merge annulé\n\n")
		return result
	}
	if response == "s" {
		result.Success = true
		result.Message = "Merge skipped"
		fmt.Fprintf(m.out, "    → Skipped\n\n")
		return result
	}

	// Attempt merge
	mergeCmd := exec.Command("git", "merge", "--no-ff", sess.Branch, "-m",
		fmt.Sprintf("merge: parallel session %s", sess.TicketID))
	mergeCmd.Dir = m.projectPath
	mergeOut, mergeErr := mergeCmd.CombinedOutput()

	if mergeErr != nil {
		// Conflict detected
		if strings.Contains(string(mergeOut), "CONFLICT") {
			result.Conflict = true
			result.Success = false
			result.Message = "Conflit de merge détecté"
			fmt.Fprintf(m.out, "    ✗ CONFLIT détecté !\n")
			fmt.Fprintf(m.out, "    %s\n", strings.TrimSpace(string(mergeOut)))
			fmt.Fprintf(m.out, "\n    Résous le conflit manuellement dans %s\n", m.projectPath)
			fmt.Fprintf(m.out, "    Puis exécute : git add . && git merge --continue\n\n")

			// Abort the merge so it doesn't stay in conflicted state
			abortCmd := exec.Command("git", "merge", "--abort")
			abortCmd.Dir = m.projectPath
			_ = abortCmd.Run()

			return result
		}
		result.Success = false
		result.Message = fmt.Sprintf("Merge échoué: %s", strings.TrimSpace(string(mergeOut)))
		fmt.Fprintf(m.out, "    ✗ Erreur: %s\n\n", result.Message)
		return result
	}

	result.Success = true
	result.Message = "Merge réussi"
	fmt.Fprintf(m.out, "    ✓ Merge réussi\n\n")
	return result
}

// sortByPriority puts priority sessions first.
func sortByPriority(sessions []SessionInfo) {
	for i := 0; i < len(sessions); i++ {
		if sessions[i].Priority {
			// Move to front
			sessions[0], sessions[i] = sessions[i], sessions[0]
			break
		}
	}
}

// DetectBaseBranch detects the default branch.
func DetectBaseBranch(projectPath string) (string, error) {
	// Try git symbolic-ref
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = projectPath
	out, err := cmd.Output()
	if err == nil {
		ref := strings.TrimSpace(string(out))
		parts := strings.Split(ref, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1], nil
		}
	}

	// Fallback: check if main or master exists
	for _, branch := range []string{"main", "master"} {
		checkCmd := exec.Command("git", "rev-parse", "--verify", branch)
		checkCmd.Dir = projectPath
		if err := checkCmd.Run(); err == nil {
			return branch, nil
		}
	}

	return "main", fmt.Errorf("could not detect base branch")
}
