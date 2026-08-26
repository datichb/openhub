package deploy

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// FileStatus represents the state of a file comparison.
type FileStatus int

const (
	FileUnchanged FileStatus = iota
	FileModified
	FileAdded
	FileRemoved
)

func (s FileStatus) String() string {
	switch s {
	case FileUnchanged:
		return "unchanged"
	case FileModified:
		return "modified"
	case FileAdded:
		return "added"
	case FileRemoved:
		return "removed"
	default:
		return "unknown"
	}
}

// FileDiff represents the comparison result for a single file.
type FileDiff struct {
	RelPath    string     // relative path (e.g., "agents/dev-senior.md")
	Status     FileStatus // added, modified, unchanged, removed
	SourceHash string     // SHA-256 of source file (empty if removed)
	DestHash   string     // SHA-256 of deployed file (empty if added)
}

// DiffReport holds the full comparison between hub source and deployed project.
type DiffReport struct {
	HubDir       string
	ProjectPath  string
	Files        []FileDiff
	ConfigDrift  bool   // true if opencode.json changed since last deploy
	ConfigDetail string // human-readable detail about config drift
}

// HasChanges returns true if there are any modifications to apply.
func (r *DiffReport) HasChanges() bool {
	if r.ConfigDrift {
		return true
	}
	for _, f := range r.Files {
		if f.Status != FileUnchanged {
			return true
		}
	}
	return false
}

// Summary returns counts by status.
func (r *DiffReport) Summary() (added, modified, removed, unchanged int) {
	for _, f := range r.Files {
		switch f.Status {
		case FileAdded:
			added++
		case FileModified:
			modified++
		case FileRemoved:
			removed++
		case FileUnchanged:
			unchanged++
		}
	}
	return
}

// ComputeDiff compares source hub files (agents/, skills/) against deployed
// files in the project directory. Returns a full report of differences.
// If selectedAgents is non-empty, only those agents are considered for comparison.
func ComputeDiff(hubDir, projectPath string, selectedAgents []string) (*DiffReport, error) {
	report := &DiffReport{
		HubDir:      hubDir,
		ProjectPath: projectPath,
	}

	// Build agent allow set
	allowSet := make(map[string]bool, len(selectedAgents))
	for _, a := range selectedAgents {
		allowSet[a] = true
	}

	// Compare agents (filtered by selection, hashed after assembly)
	skillsDir := filepath.Join(hubDir, "skills")
	agentHashFn := func(path string) (string, error) {
		// Hash the assembled output (skill inlining + hub field stripping)
		// to match what actually gets deployed
		assembled, err := assembleAgentWithSkills(path, skillsDir)
		if err != nil {
			return fileHash(path) // fallback to raw hash
		}
		return fmt.Sprintf("%x", sha256.Sum256(assembled)), nil
	}

	if err := diffDirectory(
		filepath.Join(hubDir, "agents"),
		filepath.Join(projectPath, ".opencode", "agents"),
		"agents",
		report,
		allowSet,
		agentHashFn,
		true, // flatten: agents are deployed flat
	); err != nil {
		return nil, fmt.Errorf("diff agents: %w", err)
	}

	// Compare skills (all — native skills are filtered at deploy time)
	if err := diffDirectory(
		filepath.Join(hubDir, "skills"),
		filepath.Join(projectPath, ".opencode", "skills"),
		"skills",
		report,
		nil,   // no filtering for skills
		nil,   // default hash function (raw file hash)
		false, // skills preserve directory structure
	); err != nil {
		return nil, fmt.Errorf("diff skills: %w", err)
	}

	// Check config drift (opencode.json changed since last deploy)
	checkConfigDrift(projectPath, report)

	return report, nil
}

// checkConfigDrift detects if opencode.json has been modified since the last deploy.
// When a snapshot is available, provides key-by-key detail of what changed.
func checkConfigDrift(projectPath string, report *DiffReport) {
	state := ReadDeployState(projectPath)
	if state == nil {
		// No deploy state → can't compare (first deploy or state lost)
		return
	}

	configPath := filepath.Join(projectPath, "opencode.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}

	currentHash := hashBytes(data)
	if currentHash == state.ConfigHash {
		return // no change
	}

	report.ConfigDrift = true

	// If we have a snapshot, do key-by-key diff
	if state.ConfigSnapshot != nil {
		var current map[string]interface{}
		if err := json.Unmarshal(data, &current); err == nil {
			changes := deepDiffMaps("", state.ConfigSnapshot, current)
			if len(changes) > 0 {
				report.ConfigDetail = fmt.Sprintf("opencode.json: %d clé(s) modifiée(s):\n%s",
					len(changes), formatConfigChanges(changes))
				return
			}
		}
	}

	// Fallback to hash-only message
	report.ConfigDetail = "opencode.json modifié depuis le dernier deploy (hash mismatch)"
}

// configChange represents a single key-path change.
type configChange struct {
	Path   string
	Action string // "added", "removed", "modified"
	Old    string // for modified/removed
	New    string // for modified/added
}

// deepDiffMaps recursively diffs two maps and returns a flat list of changes.
func deepDiffMaps(prefix string, old, new map[string]interface{}) []configChange {
	var changes []configChange

	for k, oldVal := range old {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		newVal, exists := new[k]
		if !exists {
			changes = append(changes, configChange{Path: path, Action: "removed", Old: formatVal(oldVal)})
			continue
		}
		// Both exist — compare
		oldMap, oldIsMap := oldVal.(map[string]interface{})
		newMap, newIsMap := newVal.(map[string]interface{})
		if oldIsMap && newIsMap {
			changes = append(changes, deepDiffMaps(path, oldMap, newMap)...)
		} else if fmt.Sprintf("%v", oldVal) != fmt.Sprintf("%v", newVal) {
			changes = append(changes, configChange{Path: path, Action: "modified", Old: formatVal(oldVal), New: formatVal(newVal)})
		}
	}

	for k, newVal := range new {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		if _, exists := old[k]; !exists {
			changes = append(changes, configChange{Path: path, Action: "added", New: formatVal(newVal)})
		}
	}

	return changes
}

func formatVal(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case map[string]interface{}, []interface{}:
		b, _ := json.Marshal(val)
		s := string(b)
		if len(s) > 60 {
			return s[:57] + "..."
		}
		return s
	default:
		return fmt.Sprintf("%v", val)
	}
}

func formatConfigChanges(changes []configChange) string {
	var sb strings.Builder
	for _, c := range changes {
		switch c.Action {
		case "added":
			sb.WriteString(fmt.Sprintf("  + %s = %s\n", c.Path, c.New))
		case "removed":
			sb.WriteString(fmt.Sprintf("  - %s (was: %s)\n", c.Path, c.Old))
		case "modified":
			sb.WriteString(fmt.Sprintf("  ~ %s: %s → %s\n", c.Path, c.Old, c.New))
		}
	}
	return sb.String()
}

// diffDirectory compares all files between source and destination directories.
// If allowSet is non-empty, only source files whose base name (sans extension) is in the set are compared.
// If srcHashFn is non-nil, it is used to hash source files instead of raw file hashing
// (e.g., to hash the assembled output for agents).
// If flatten is true, source files are keyed by their base filename only (not their relative path),
// matching the flat output structure used by DeployAgents.
func diffDirectory(srcDir, destDir, prefix string, report *DiffReport, allowSet map[string]bool, srcHashFn func(string) (string, error), flatten bool) error {
	srcFiles := make(map[string]string) // relative path → sha256
	destFiles := make(map[string]string)

	// Walk source directory
	if _, err := os.Stat(srcDir); err == nil {
		err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			// Filter by allow set (match on filename without extension)
			if len(allowSet) > 0 {
				name := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
				if !allowSet[name] {
					return nil
				}
			}
			rel, _ := filepath.Rel(srcDir, path)
			// When flatten is true, use only the filename as key (matching flat deploy output)
			key := rel
			if flatten {
				key = d.Name()
			}
			var hash string
			var hashErr error
			if srcHashFn != nil {
				hash, hashErr = srcHashFn(path)
			} else {
				hash, hashErr = fileHash(path)
			}
			if hashErr != nil {
				return hashErr
			}
			srcFiles[key] = hash
			return nil
		})
		if err != nil {
			return err
		}
	}

	// Walk destination directory
	if _, err := os.Stat(destDir); err == nil {
		err := filepath.WalkDir(destDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(destDir, path)
			hash, err := fileHash(path)
			if err != nil {
				return err
			}
			destFiles[rel] = hash
			return nil
		})
		if err != nil {
			return err
		}
	}

	// Compare: files in source
	for rel, srcHash := range srcFiles {
		displayPath := filepath.Join(prefix, rel)
		destHash, exists := destFiles[rel]
		switch {
		case !exists:
			report.Files = append(report.Files, FileDiff{
				RelPath:    displayPath,
				Status:     FileAdded,
				SourceHash: srcHash,
			})
		case srcHash != destHash:
			report.Files = append(report.Files, FileDiff{
				RelPath:    displayPath,
				Status:     FileModified,
				SourceHash: srcHash,
				DestHash:   destHash,
			})
		default:
			report.Files = append(report.Files, FileDiff{
				RelPath:    displayPath,
				Status:     FileUnchanged,
				SourceHash: srcHash,
				DestHash:   destHash,
			})
		}
	}

	// Compare: files only in destination (removed from source)
	for rel, destHash := range destFiles {
		if _, exists := srcFiles[rel]; !exists {
			displayPath := filepath.Join(prefix, rel)
			report.Files = append(report.Files, FileDiff{
				RelPath:  displayPath,
				Status:   FileRemoved,
				DestHash: destHash,
			})
		}
	}

	return nil
}

// fileHash returns the SHA-256 hex digest of a file using streaming to avoid
// loading the entire file into memory.
func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// FormatDiffReport produces a human-readable diff summary.
// If verbose is true, includes unchanged files too.
func FormatDiffReport(report *DiffReport, verbose bool) string {
	var sb strings.Builder

	added, modified, removed, unchanged := report.Summary()

	for _, f := range report.Files {
		switch f.Status {
		case FileAdded:
			fmt.Fprintf(&sb, "  + %s (nouveau)\n", f.RelPath)
		case FileModified:
			fmt.Fprintf(&sb, "  ~ %s (modifié)\n", f.RelPath)
		case FileRemoved:
			fmt.Fprintf(&sb, "  - %s (supprimé du hub)\n", f.RelPath)
		case FileUnchanged:
			if verbose {
				fmt.Fprintf(&sb, "  = %s\n", f.RelPath)
			}
		}
	}

	fmt.Fprintf(&sb, "\n  Résumé: %d ajouté(s), %d modifié(s), %d supprimé(s), %d inchangé(s)\n",
		added, modified, removed, unchanged)

	if report.ConfigDrift {
		fmt.Fprintf(&sb, "\n  ⚠ %s\n", report.ConfigDetail)
	}

	return sb.String()
}
