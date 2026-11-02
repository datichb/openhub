package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// oh patterns add: the answers of the wizard make a validated manual pattern,
// with the content of the file given or a default one.
func TestPatternsAddWizard(t *testing.T) {
	i18ntest.Set(t, "en")
	info := func(name, tags string, complexity int) map[int]func(f *tview.Form) {
		return map[int]func(f *tview.Form){0: func(f *tview.Form) {
			typeInto(f, 0, name)
			typeInto(f, 1, tags)
			choose(f, 2, complexity)
		}}
	}
	tests := []struct {
		name        string
		answers     map[int]func(f *tview.Form)
		abortAt     int
		file        string
		wantErr     string
		wantTags    []string
		wantCompl   string
		wantContent string
		none        bool
	}{
		{
			name: "default content", answers: info("retry-queue", "queue, retry ,,", 2), abortAt: -1,
			wantTags: []string{"queue", "retry"}, wantCompl: "high", wantContent: i18n.Tf("cmd.patterns.new_content", "retry-queue"),
		},
		{
			name: "content of the file", answers: info("from-file", "a", 0), abortAt: -1, file: "# my pattern\n",
			wantTags: []string{"a"}, wantCompl: "low", wantContent: "# my pattern\n",
		},
		{name: "spaces in the name: the command fails", answers: info("two words", "a", 1), abortAt: -1, wantErr: "hyphen", none: true},
		{name: "no name: the command fails", answers: info("", "a", 1), abortAt: -1, wantErr: "name", none: true},
		{name: "the user quits: nothing is written", answers: info("x", "a", 1), abortAt: 0, none: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, repo, _ := setupClaimTestApp(t)
			s := scriptWizard(t, tc.answers)
			s.abortAt = tc.abortAt
			var args []string
			if tc.file != "" {
				path := filepath.Join(t.TempDir(), "pattern.md")
				require.NoError(t, os.WriteFile(path, []byte(tc.file), 0o644))
				args = []string{path}
			}

			patternsAddCmd.SetContext(t.Context())
			err := runPatternsAdd(patternsAddCmd, args)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, "patterns add", s.command, "command of the wizard")
			list, err := repo.ListPatterns(nil, 0)
			require.NoError(t, err)
			if tc.none {
				assert.Empty(t, list, "patterns written")
				return
			}
			require.Len(t, list, 1, "patterns written")
			p := list[0]
			assert.Equal(t, tc.wantTags, p.Tags, "tags")
			assert.Equal(t, tc.wantCompl, p.Complexity, "complexity")
			assert.Equal(t, "manual", p.Source, "source")
			assert.True(t, p.Validated, "validated")
			content, err := repo.ReadPattern(p.Name)
			require.NoError(t, err)
			assert.Equal(t, tc.wantContent, content, "content")
		})
	}
}
