package opencode

import (
	"context"
	"fmt"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
)

// ghostSessionThreshold is the maximum age of a "running" session before it is
// considered a ghost (likely the process was killed without cleanup).
const ghostSessionThreshold = 24 * time.Hour

// ActiveSessionInfo holds the relevant info for displaying active session warnings.
type ActiveSessionInfo struct {
	SessionID  string
	StartedAt  time.Time
	LaunchPath string
}

// FindActiveSessionsOnPath returns running sessions that match a specific launch path.
// Ghost sessions older than ghostSessionThreshold are excluded.
// If path is empty, all non-ghost running sessions for the project are returned.
func FindActiveSessionsOnPath(ctx context.Context, store domain.SessionStore, projectID, path string) ([]ActiveSessionInfo, error) {
	running, err := store.ListRunning(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("querying running sessions: %w", err)
	}

	var result []ActiveSessionInfo
	for _, s := range running {
		if IsGhostSession(s) {
			continue
		}
		if path != "" && s.LaunchPath != path {
			continue
		}
		result = append(result, ActiveSessionInfo{
			SessionID:  s.ID,
			StartedAt:  s.StartedAt,
			LaunchPath: s.LaunchPath,
		})
	}
	return result, nil
}

// FindAllActiveSessionsByPath returns a map of launch_path → active sessions for a project.
// Useful for annotating multiple worktree options with session badges.
// Ghost sessions are excluded.
func FindAllActiveSessionsByPath(ctx context.Context, store domain.SessionStore, projectID string) (map[string][]ActiveSessionInfo, error) {
	running, err := store.ListRunning(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("querying running sessions: %w", err)
	}

	result := make(map[string][]ActiveSessionInfo)
	for _, s := range running {
		if IsGhostSession(s) {
			continue
		}
		info := ActiveSessionInfo{
			SessionID:  s.ID,
			StartedAt:  s.StartedAt,
			LaunchPath: s.LaunchPath,
		}
		result[s.LaunchPath] = append(result[s.LaunchPath], info)
	}
	return result, nil
}

// IsGhostSession returns true if a "running" session is likely dead.
// A session is considered a ghost if it has been running for longer than
// ghostSessionThreshold without completing. This heuristic handles cases
// where the process was killed (SIGKILL, OOM, power loss) without updating
// the session status.
func IsGhostSession(s domain.Session) bool {
	if s.Status != domain.SessionStatusRunning {
		return false
	}
	return time.Since(s.StartedAt) > ghostSessionThreshold
}

// FormatActiveSessionWarning produces a human-readable warning for display in modals.
// Returns an empty string if no active sessions are provided.
func FormatActiveSessionWarning(sessions []ActiveSessionInfo) string {
	if len(sessions) == 0 {
		return ""
	}
	if len(sessions) == 1 {
		elapsed := time.Since(sessions[0].StartedAt).Truncate(time.Minute)
		return fmt.Sprintf("Une session est active sur ce chemin depuis %s.", formatDuration(elapsed))
	}
	return fmt.Sprintf("%d sessions sont actives sur ce chemin.", len(sessions))
}

// formatDuration formats a duration in a human-friendly way.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "moins d'une minute"
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours > 0 && minutes > 0 {
		return fmt.Sprintf("%dh%02dmin", hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dmin", minutes)
}
