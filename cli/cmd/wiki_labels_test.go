package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// A4: « wiki » alone is read as the GitLab wiki. Where the labels speak of the
// wiki oh manages, they name docs/wiki/ and say what it is.
func TestWikiLabelsNameTheVersionedDocumentation(t *testing.T) {
	defer func() { i18n.SetLocale("en"); localizeCommands(rootCmd) }()
	precise := map[string]string{"fr": "docs/wiki/ (documentation versionnée du dépôt", "en": "docs/wiki/ (versioned repository documentation"}
	for lang, want := range precise {
		i18n.SetLocale(lang)
		localizeCommands(rootCmd)

		pending, _, err := rootCmd.Find([]string{"wiki", "pending"})
		require.NoError(t, err)
		assert.Contains(t, pending.Short, want, lang+" oh wiki pending")
		assert.Contains(t, pending.Long, want, lang+" oh wiki pending (long)")

		for _, k := range []string{"cmd.mcp.wiki.propose_desc", "cmd.mcp.wiki.pending_desc", "cmd.wiki.short", "cmd.wiki.decide.short"} {
			assert.Contains(t, i18n.T(k), want, lang+" "+k)
		}
		assert.Contains(t, i18n.T("cmd.mcp.wiki.propose_desc"), "GitLab", lang+": wiki_propose says it is not the GitLab wiki")
	}
}

func TestSpecLabelsNameTheVersionedDocumentation(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "workflows", "spec.yaml"))
	if err != nil {
		t.Skip("no hub sources (not the development repository)")
	}
	text := string(raw)
	for _, want := range []string{
		"fr: Comprendre une demande, écrire sa spec (glossaire, critères et points de test) dans docs/wiki/ (documentation versionnée du dépôt)",
		"en: Understand a request, write its spec (glossary, criteria and test points) in docs/wiki/ (versioned repository documentation)",
		"fr: Page de docs/wiki/ (documentation versionnée du dépôt)",
		"en: Page of docs/wiki/ (versioned repository documentation)",
	} {
		assert.True(t, strings.Contains(text, want), want)
	}
}
