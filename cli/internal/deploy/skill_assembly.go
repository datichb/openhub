package deploy

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// assembleAgentWithSkills reads an agent .md file, parses its frontmatter to find
// Bucket A skills (the `skills:` field), reads each skill's content, and returns
// the assembled agent file with skills inlined at the end of the body.
//
// The assembled output preserves the original frontmatter (with `skills:` and `native_skills:`
// stripped since they are hub-internal metadata) and appends skill content after the body.
func assembleAgentWithSkills(agentPath, skillsDir string) ([]byte, error) {
	data, err := os.ReadFile(agentPath)
	if err != nil {
		return nil, fmt.Errorf("reading agent file: %w", err)
	}

	// Parse frontmatter to get skill references
	fm, err := ParseAgentFrontmatterFromBytes(data)
	if err != nil {
		// If we can't parse frontmatter, return the file as-is
		return data, nil //nolint:nilerr // intentional: graceful fallback to raw content
	}

	// If no Bucket A skills, return as-is (but still strip hub-internal fields)
	frontmatterBytes, bodyBytes := splitFrontmatterAndBody(data)

	// Strip hub-internal frontmatter fields (skills, native_skills, mcpServers)
	cleanedFrontmatter := stripHubFields(frontmatterBytes)

	// If no Bucket A skills to inline, just return cleaned file
	if len(fm.Skills) == 0 {
		var result bytes.Buffer
		result.Write(cleanedFrontmatter)
		result.Write(bodyBytes)
		return result.Bytes(), nil
	}

	// Assemble: cleaned frontmatter + body + inlined skills
	var result bytes.Buffer
	result.Write(cleanedFrontmatter)
	result.Write(bodyBytes)

	// Append each Bucket A skill content
	for _, skillRef := range fm.Skills {
		content, err := readSkillContent(skillsDir, skillRef)
		if err != nil {
			// Skip skills that can't be read (warning already logged elsewhere)
			continue
		}
		result.WriteString("\n\n---\n\n")
		result.Write(content)
	}

	return result.Bytes(), nil
}

// readSkillContent reads a skill file and returns its body content (without frontmatter).
// skillRef is a path like "posture/concision-posture" → reads "skills/posture/concision-posture.md"
func readSkillContent(skillsDir, skillRef string) ([]byte, error) {
	skillPath := filepath.Join(skillsDir, skillRef+".md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return nil, fmt.Errorf("reading skill %s: %w", skillRef, err)
	}

	// Extract body only (skip frontmatter)
	_, body := splitFrontmatterAndBody(data)
	return body, nil
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

// stripHubFields removes hub-internal frontmatter fields that opencode doesn't understand:
// skills, native_skills, mcpServers. These are hub metadata consumed by the deploy process only.
func stripHubFields(frontmatter []byte) []byte {
	if len(frontmatter) == 0 {
		return frontmatter
	}

	scanner := bufio.NewScanner(bytes.NewReader(frontmatter))
	var result bytes.Buffer

	hubFields := map[string]bool{
		"skills:":        true,
		"native_skills:": true,
		"mcpServers:":    true,
	}

	skipping := false
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Check if this line starts a hub-internal field
		isHubField := false
		for field := range hubFields {
			if strings.HasPrefix(trimmed, field) {
				isHubField = true
				// If it's an inline array (e.g., skills: [a, b, c]), skip just this line
				// If it's a block sequence, need to skip continuation lines
				skipping = !strings.Contains(trimmed, "[")
				break
			}
		}

		if isHubField {
			continue // Skip this line
		}

		// If we were skipping a multi-line block, check if this line is a continuation
		if skipping {
			// Continuation lines start with whitespace and "- "
			if strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t") {
				continue
			}
			skipping = false
		}

		result.WriteString(line)
		result.WriteByte('\n')
	}

	return result.Bytes()
}

// deployNativeSkill converts a hub skill file to the opencode-expected format
// (.opencode/skills/<name>/SKILL.md) and writes it to the destination.
// The source skill already has valid frontmatter (name:, description:) so we just copy it.
func deployNativeSkill(skillsDir, skillRef, destSkillsDir string) error {
	skillPath := filepath.Join(skillsDir, skillRef+".md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return fmt.Errorf("reading native skill %s: %w", skillRef, err)
	}

	// Derive the skill name from the ref (last component)
	parts := strings.Split(skillRef, "/")
	skillName := parts[len(parts)-1]

	// Create directory .opencode/skills/<name>/
	skillDir := filepath.Join(destSkillsDir, skillName)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return fmt.Errorf("creating skill directory %s: %w", skillName, err)
	}

	// Write as SKILL.md
	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, data, 0o644); err != nil {
		return fmt.Errorf("writing SKILL.md for %s: %w", skillName, err)
	}

	return nil
}
