package beads

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Ticket follow-up (bd orphans, bd stale, bd epic status): work done but not
// closed, work claimed then left, epics whose children are all closed.

// Orphan is an open ticket named in a commit message ("… (bd-42)").
type Orphan struct {
	ID      string `json:"issue_id"`
	Title   string `json:"title"`
	Status  string `json:"status"`
	Commit  string `json:"latest_commit"`
	Message string `json:"latest_commit_message"`
}

// EpicProgress is the completion of an epic.
type EpicProgress struct {
	Epic struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"epic"`
	Total    int  `json:"total_children"`
	Closed   int  `json:"closed_children"`
	Eligible bool `json:"eligible_for_close"`
}

// Orphans lists the open tickets named in the commits of the project.
// Runs: bd -C <path> --readonly orphans --json
func Orphans(projectPath string) ([]Orphan, error) {
	var out []Orphan
	err := readJSON(projectPath, &out, "orphans")
	return out, err
}

// Stale lists the tickets of a status not updated for days (in_progress:
// claimed then left).
// Runs: bd -C <path> --readonly stale --status <s> --days <d> -n 0 --json
func Stale(projectPath, status string, days int) ([]Ticket, error) {
	if days < 1 {
		days = 1
	}
	return runBdJSON([]string{"-C", projectPath, "--readonly", "stale", "--status", status, "--days", strconv.Itoa(days), "-n", "0", "--json"})
}

// EpicStatus returns the completion of the open epics (only those whose
// children are all closed with eligibleOnly).
// Runs: bd -C <path> --readonly epic status [--eligible-only] --json
func EpicStatus(projectPath string, eligibleOnly bool) ([]EpicProgress, error) {
	args := []string{"epic", "status"}
	if eligibleOnly {
		args = append(args, "--eligible-only")
	}
	var out []EpicProgress
	err := readJSON(projectPath, &out, args...)
	return out, err
}

// readJSON runs a read command of bd and decodes its JSON answer ("null"
// or nothing: no record).
func readJSON(projectPath string, into any, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := BdCommand(ctx, append(append([]string{"-C", projectPath, "--readonly"}, args...), "--json")...)
	cmd.Dir = projectPath // bd orphans reads the git log of its working directory, not of -C (bd 1.3.1)
	data, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("bd %s: %s", args[0], strings.TrimSpace(string(exitErr.Stderr)))
		}
		return fmt.Errorf("bd %s: %w", args[0], err)
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	if err := json.Unmarshal([]byte(trimmed), into); err != nil {
		return fmt.Errorf("bd %s: %w", args[0], err)
	}
	return nil
}

// maxMemories bounds the project memories given to a session prompt.
const maxMemories = 4000

// Memories returns the persistent memories of the project (bd remember), one
// per line ("- text", sorted by key), truncated to a few thousand
// characters; "" without any.
// Runs: bd -C <path> --readonly memories --json
func Memories(projectPath string) (string, error) {
	var raw map[string]json.RawMessage
	if err := readJSON(projectPath, &raw, "memories"); err != nil {
		return "", err
	}
	keys := make([]string, 0, len(raw))
	for k := range raw {
		if k != "schema_version" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		var text string
		if json.Unmarshal(raw[k], &text) != nil || strings.TrimSpace(text) == "" {
			continue
		}
		line := "- " + strings.Join(strings.Fields(text), " ") + "\n"
		if sb.Len()+len(line) > maxMemories {
			sb.WriteString("- …\n")
			break
		}
		sb.WriteString(line)
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}
