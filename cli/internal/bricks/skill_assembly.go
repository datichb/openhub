package bricks

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolveSkillPath resolves a skill reference to its file in the skills
// directory (skills/<ref>.md: hub skills, merged with the team catalogue).
// The community skills of ~/.oh/skills are no longer read (ADR-051).
func resolveSkillPath(skillsDir, skillRef string) (string, error) {
	hubPath := filepath.Join(skillsDir, skillRef+".md")
	if _, err := os.Stat(hubPath); err != nil {
		return "", fmt.Errorf("skill %q not found in hub (%s)", skillRef, hubPath)
	}
	return hubPath, nil
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
