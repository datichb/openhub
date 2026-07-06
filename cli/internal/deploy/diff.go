package deploy

import (
	"crypto/sha256"
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
	HubDir      string
	ProjectPath string
	Files       []FileDiff
}

// HasChanges returns true if there are any modifications to apply.
func (r *DiffReport) HasChanges() bool {
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
func ComputeDiff(hubDir, projectPath string) (*DiffReport, error) {
	report := &DiffReport{
		HubDir:      hubDir,
		ProjectPath: projectPath,
	}

	// Compare agents
	if err := diffDirectory(
		filepath.Join(hubDir, "agents"),
		filepath.Join(projectPath, ".opencode", "agents"),
		"agents",
		report,
	); err != nil {
		return nil, fmt.Errorf("diff agents: %w", err)
	}

	// Compare skills
	if err := diffDirectory(
		filepath.Join(hubDir, "skills"),
		filepath.Join(projectPath, ".opencode", "skills"),
		"skills",
		report,
	); err != nil {
		return nil, fmt.Errorf("diff skills: %w", err)
	}

	return report, nil
}

// diffDirectory compares all files between source and destination directories.
func diffDirectory(srcDir, destDir, prefix string, report *DiffReport) error {
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
			rel, _ := filepath.Rel(srcDir, path)
			hash, err := fileHash(path)
			if err != nil {
				return err
			}
			srcFiles[rel] = hash
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

	return sb.String()
}
