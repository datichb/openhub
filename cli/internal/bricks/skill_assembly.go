package bricks

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/datichb/openhub/cli/internal/skillregistry"
)

// resolveSkillPath resolves a skill reference to an absolute file path.
// It first tries the hub skills directory (skills/<ref>.md), and falls back
// to the community skills registry (~/.oh/skills/<name>/SKILL.md) if not found.
func resolveSkillPath(skillsDir, skillRef string) (string, error) {
	// Try hub path first
	hubPath := filepath.Join(skillsDir, skillRef+".md")
	if _, err := os.Stat(hubPath); err == nil {
		return hubPath, nil
	}

	// Fall back to community registry: extract skill name from ref
	parts := strings.Split(skillRef, "/")
	skillName := parts[len(parts)-1]

	reg := skillregistry.NewRegistry()
	communityPath, err := reg.SkillMDPath(skillName)
	if err == nil {
		if _, statErr := os.Stat(communityPath); statErr == nil {
			return communityPath, nil
		}
	}

	// Neither found — return error pointing to the hub path
	return "", fmt.Errorf("skill %q not found in hub (%s) or community registry", skillRef, hubPath)
}

// splitFrontmatterAndBody splits a markdown file into frontmatter (including delimiters)
// and body parts.
func splitFrontmatterAndBody(data []byte) (frontmatter, body []byte) {
	scanner := bufio.NewScanner(bytes.NewReader(data))

	// Check for opening ---
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return nil, data // No frontmatter
	}

	var fmBuf bytes.Buffer
	fmBuf.WriteString("---\n")

	// Read until closing ---
	found := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			fmBuf.WriteString("---\n")
			found = true
			break
		}
		fmBuf.WriteString(line)
		fmBuf.WriteByte('\n')
	}

	if !found {
		return nil, data // Malformed — return as body
	}

	// Everything after the closing --- is body
	var bodyBuf bytes.Buffer
	for scanner.Scan() {
		bodyBuf.WriteString(scanner.Text())
		bodyBuf.WriteByte('\n')
	}

	return fmBuf.Bytes(), bodyBuf.Bytes()
}
