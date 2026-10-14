package wiki

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// p4-debts: the format of one page, as verify.spec reads a spec: the
// frontmatter checks of wiki_check, the keys of doc-wiki-protocol only
// (synced in index.md), the tag lines well formed; the warnings of the
// whole wiki (untagged page, CONFIRMÉ without source) are left out.
func TestFormatIssues(t *testing.T) {
	kinds := func(rel, text string) []string {
		var out []string
		for _, i := range FormatIssues(rel, text) {
			out = append(out, i.Kind)
		}
		return out
	}
	ok := "---\nupdated: 2026-10-14\nconfidence: mixed\nagents: [planner]\n---\n# Spec\n"
	assert.Empty(t, kinds("specs/a.md", ok), "no tag, no source: nothing to report")
	assert.Empty(t, kinds("specs/a.md", ok+"- x — `CONFIRMÉ` · planner · 2026-10-14\n- y — `DÉDUIT` · planner · 2026-10-14\n"))
	assert.Equal(t, []string{IssueFrontmatter}, kinds("specs/a.md", "# Spec\n"))
	assert.Equal(t, []string{IssueConfidence}, kinds("specs/a.md", "---\nupdated: 2026-10-14\nconfidence: high\nagents: [planner]\n---\n"))
	assert.Equal(t, []string{IssueUpdated, IssueAgents}, kinds("specs/a.md", "---\nupdated: 14/10/2026\nconfidence: mixed\n---\n"))
	assert.Equal(t, []string{IssueConfidenceMismatch}, kinds("specs/a.md", ok+"- x — `INCERTAIN` · planner · 2026-10-14\n"))
	assert.Equal(t, []string{IssueTag}, kinds("specs/a.md", ok+"- x — `DÉDUIT` planner\n"))
	assert.Equal(t, []string{IssueFrontmatterKey, IssueFrontmatterKey}, kinds("specs/a.md", "---\ntitle: A\nupdated: 2026-10-14\nconfidence: mixed\nagents: [planner]\nsynced: abc\n---\n"))
	assert.Empty(t, kinds(IndexPage, "---\nupdated: 2026-10-14\nconfidence: mixed\nagents: [onboarder]\nsynced: abc\n---\n"), "synced belongs to the index")
	assert.Equal(t, []string{IssueFrontmatter}, kinds("specs/a.md", "---\nupdated: [\n---\n"))
}

// p5-fixes: wiki_check reports a frontmatter key out of doc-wiki-protocol
// on every page, as a warning, and as an error in a spec (the format of
// verify.spec); synced belongs to the index.
func TestCheckFrontmatterKeys(t *testing.T) {
	fm := func(extra string) string {
		return "---\nupdated: 2099-01-01\nconfidence: confirmed\nagents: [onboarder]\n" + extra + "---\n# P\n\n- x\n  — `CONFIRMÉ` · onboarder · 2099-01-01 · go.mod\n"
	}
	root := writeWiki(t, map[string]string{
		"index.md":                 fm("synced: abc\n"),
		"technical/stack.md":       fm("title: Stack\n"),
		"specs/export.md":          fm("status: draft\n"),
		"business/billing.md":      fm("synced: abc\n"),
		"technical/conventions.md": fm(""),
	})
	rep, err := Check(context.Background(), root, func(context.Context, string, string) (time.Time, bool) { return time.Time{}, true })
	require.NoError(t, err)
	got := map[string]string{}
	for _, i := range rep.Issues {
		if i.Kind == IssueFrontmatterKey {
			got[i.Page] = i.Severity + " " + i.Message
		}
	}
	assert.Len(t, got, 3, "%v", got)
	assert.Contains(t, got["technical/stack.md"], SeverityWarning+" ")
	assert.Contains(t, got["technical/stack.md"], "title")
	assert.Contains(t, got["specs/export.md"], SeverityError+" ")
	assert.Contains(t, got["business/billing.md"], SeverityWarning+" ", "synced out of the index")
}
