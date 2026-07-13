package parallel

import (
	"sort"
	"strings"
)

// SharedContext tracks which files each session touches to detect conflicts.
type SharedContext struct {
	state *ParallelState
}

// NewSharedContext creates a new shared context tracker.
func NewSharedContext(state *ParallelState) *SharedContext {
	return &SharedContext{state: state}
}

// UpdateFromServers polls file status from each server and updates the state.
func (sc *SharedContext) UpdateFromServers(servers []*OpenCodeServer) {
	for _, srv := range servers {
		if !srv.IsAlive() {
			continue
		}

		files, err := srv.GetFileStatus()
		if err != nil || len(files) == 0 {
			continue
		}

		// Separate modified vs created
		var modified, created []string
		for _, f := range files {
			// If we can't determine, treat all as modified
			modified = append(modified, f)
		}

		sc.state.UpdateSession(srv.TicketID, func(s *SessionInfo) {
			s.FilesModified = modified
			s.FilesCreated = created
		})
	}

	// Detect conflicts
	sc.detectConflicts()
}

// detectConflicts finds files touched by multiple sessions.
func (sc *SharedContext) detectConflicts() {
	snap := sc.state.Snapshot()

	// Map file → list of ticket IDs that touch it
	fileToSessions := make(map[string][]string)
	for _, sess := range snap.Sessions {
		if sess.Status != StatusRunning && sess.Status != StatusCompleted {
			continue
		}
		allFiles := append(sess.FilesModified, sess.FilesCreated...)
		for _, f := range allFiles {
			fileToSessions[f] = append(fileToSessions[f], sess.TicketID)
		}
	}

	// Build conflict list
	var conflicts []ConflictInfo
	for file, sessions := range fileToSessions {
		if len(sessions) > 1 {
			// Deduplicate
			unique := uniqueStrings(sessions)
			if len(unique) > 1 {
				severity := classifyConflictSeverity(file)
				conflicts = append(conflicts, ConflictInfo{
					File:     file,
					Sessions: unique,
					Severity: severity,
				})
			}
		}
	}

	// Sort by severity (high first)
	sort.Slice(conflicts, func(i, j int) bool {
		return severityRank(conflicts[i].Severity) > severityRank(conflicts[j].Severity)
	})

	sc.state.SetConflicts(conflicts)
}

// GetConflicts returns the current detected conflicts.
func (sc *SharedContext) GetConflicts() []ConflictInfo {
	snap := sc.state.Snapshot()
	return snap.Conflicts
}

// classifyConflictSeverity guesses severity based on file type/path.
func classifyConflictSeverity(file string) string {
	// Config files and lock files are usually trivial to merge
	lower := strings.ToLower(file)
	if strings.Contains(lower, "lock") || strings.HasSuffix(lower, ".lock") {
		return "low"
	}
	if strings.Contains(lower, "config") || strings.HasSuffix(lower, ".json") || strings.HasSuffix(lower, ".toml") {
		return "medium"
	}
	// Source files are typically higher severity
	return "high"
}

func severityRank(s string) int {
	switch s {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func uniqueStrings(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	result := make([]string, 0, len(ss))
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
