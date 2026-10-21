package bundle

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// cpIDRe matches a checkpoint id written in the content, in any case (CP-2
// is not the id cp-2): a dead or misspelled id is reported as written.
var cpIDRe = regexp.MustCompile(`(?:^|[^A-Za-z0-9_-])([cC][pP]-[A-Za-z0-9]+(?:-[A-Za-z0-9]+)*)`)

const cpOptionalMarker = "<!-- capability: optional -->"

// cpIDRefs lists the checkpoint ids named in a text (its body, after the
// frontmatter), except on the lines marked cpOptionalMarker (a line that
// describes another workflow) and in the code block that follows the marker
// alone on its line.
func cpIDRefs(text string) []string {
	text = stripFrontmatter(text)
	var out []string
	inFence, skipNext, skipFence := false, false, false
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") {
			if !inFence && skipNext {
				skipFence = true
			}
			inFence, skipNext = !inFence, false
			if !inFence {
				skipFence = false
			}
			continue
		}
		if t == cpOptionalMarker {
			skipNext = true
			continue
		}
		if skipFence || strings.Contains(line, cpOptionalMarker) {
			continue
		}
		for _, m := range cpIDRe.FindAllStringSubmatch(line, -1) {
			out = append(out, m[1])
		}
	}
	return out
}

// stripFrontmatter drops a leading YAML frontmatter (a block opened by
// "---" whose lines are keys); a leading horizontal rule is kept.
func stripFrontmatter(text string) string {
	if !strings.HasPrefix(text, "---\n") {
		return text
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return text
	}
	head := text[4 : 4+end]
	if !regexp.MustCompile(`(?m)^(name|id|description):`).MatchString(head) {
		return text
	}
	return text[4+end+4:]
}

// P1.4 (C-16): every checkpoint id named in the content (cp-… or CP-…) is a
// checkpoint of a shipped workflow, written as the workflow writes it. In
// the body of a shipped agent, it is a checkpoint of one of the workflows
// that run the agent. A line marked `<!-- capability: optional -->` may name
// a checkpoint of another workflow.
func TestContentCheckpointIDsExist(t *testing.T) {
	t.Parallel()
	hub, cat, _ := shippedWorkflows(t)
	all := map[string]bool{}
	byAgent := map[string]map[string]bool{}
	for _, ref := range cat.Refs() {
		r, diags := workflow.ResolveSpec(cat, ref, nil)
		require.Empty(t, diags.Errors(), ref.ID)
		ids := r.Spec.Checkpoints.Keys()
		for _, id := range ids {
			all[id] = true
		}
		for _, agent := range r.Spec.Agents.Keys() {
			if byAgent[agent] == nil {
				byAgent[agent] = map[string]bool{}
			}
			for _, id := range ids {
				byAgent[agent][id] = true
			}
		}
	}
	require.NotEmpty(t, all)

	agentOf := map[string]string{}
	files, err := bricks.FindAgentFiles(hub)
	require.NoError(t, err)
	for _, f := range files {
		fm, err := bricks.ParseAgentFrontmatter(f)
		require.NoError(t, err)
		agentOf[f] = fm.ID
	}

	for _, dir := range []string{"agents", "skills", "workflows/prompts"} {
		require.NoError(t, filepath.WalkDir(filepath.Join(hub, dir), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || (filepath.Ext(p) != ".md" && filepath.Ext(p) != ".tmpl") {
				return err
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(hub, p)
			known, scope := all, "any shipped workflow"
			if id, ok := agentOf[p]; ok && byAgent[id] != nil {
				known, scope = byAgent[id], "the workflows of "+id
			}
			for _, cp := range cpIDRefs(string(data)) {
				assert.True(t, known[cp], "%s names %s, not a checkpoint of %s", rel, cp, scope)
			}
			return nil
		}))
	}
}

func TestCpIDRefs(t *testing.T) {
	text := "---\nname: x\ndescription: au CP-feature\n---\n" +
		"Passe `cp-1`, puis `workflow_checkpoint({ id: \"cp-2\" })`.\n" +
		"Ancien CP-path, CP-0.\n" +
		"Dans `quick`, `cp-commit`. <!-- capability: optional -->\n" +
		"<!-- capability: optional -->\n```\ncp-feature\n```\n" +
		"Le checkpoint `cp-<id>`, un mot comme scp-x ou cp-.\n"
	assert.Equal(t, []string{"cp-1", "cp-2", "CP-path", "CP-0"}, cpIDRefs(text))
	assert.Equal(t, "---\n\n## Retour\n", stripFrontmatter("---\n\n## Retour\n"))
}
