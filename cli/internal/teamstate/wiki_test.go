package teamstate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWikiListPages(t *testing.T) {
	repo := setupTestRepo(t)

	dir := filepath.Join(repo.path, "wiki")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "decisions.md"), []byte("# Decisions"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "patterns.md"), []byte("# Patterns"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".pending"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "not-md.txt"), []byte("skip"), 0o644))

	pages, err := repo.WikiListPages()
	require.NoError(t, err)
	assert.Len(t, pages, 2)
	assert.Contains(t, pages, "decisions")
	assert.Contains(t, pages, "patterns")
}

func TestWikiListPagesEmpty(t *testing.T) {
	repo := setupTestRepo(t)
	// No wiki dir
	pages, err := repo.WikiListPages()
	require.NoError(t, err)
	assert.Empty(t, pages)
}

func TestWikiReadPage(t *testing.T) {
	repo := setupTestRepo(t)

	dir := filepath.Join(repo.path, "wiki")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "decisions.md"), []byte("# Decisions\n\nSome content"), 0o644))

	content, err := repo.WikiReadPage("decisions")
	require.NoError(t, err)
	assert.Equal(t, "# Decisions\n\nSome content", content)
}

func TestWikiReadPageNotFound(t *testing.T) {
	repo := setupTestRepo(t)

	_, err := repo.WikiReadPage("nonexistent")
	assert.ErrorIs(t, err, ErrWikiPageNotFound)
}

func TestValidateProposal(t *testing.T) {
	valid := WikiProposal{Page: "decisions", Content: "## New Decision\n\nWe decided to use X.", Confidence: "CONFIRMED", Author: "documentarian", Project: "T-SRU"}
	with := func(mod func(*WikiProposal)) WikiProposal {
		p := valid
		mod(&p)
		return p
	}
	tests := []struct {
		name    string
		p       WikiProposal
		wantErr error
		wantMsg string
	}{
		{"valid", valid, nil, ""},
		{"no page", with(func(p *WikiProposal) { p.Page = "" }), ErrProposalInvalid, "page is required"},
		{"no content", with(func(p *WikiProposal) { p.Content = "" }), ErrProposalInvalid, "content is required"},
		{"no confidence", with(func(p *WikiProposal) { p.Confidence = "" }), ErrProposalInvalid, "confidence"},
		{"unknown confidence", with(func(p *WikiProposal) { p.Confidence = "MAYBE" }), ErrProposalInvalid, "CONFIRMED, INFERRED, or UNCERTAIN"},
		{"no author", with(func(p *WikiProposal) { p.Author = "" }), ErrProposalInvalid, "author is required"},
		{"more than 200 lines", with(func(p *WikiProposal) { p.Content = strings.Repeat("line\n", 201) }), ErrProposalTooLarge, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProposal(tt.p)
			if tt.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tt.wantErr)
			if tt.wantMsg != "" {
				assert.Contains(t, err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestWikiListPending(t *testing.T) {
	repo := setupTestRepo(t)

	dir := filepath.Join(repo.path, "wiki", ".pending")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	proposal := `id = "abc12345"
page = "decisions"
content = "## New Decision\n\nContent here."
confidence = "CONFIRMED"
author = "documentarian"
project = "T-SRU"
created_at = 2026-07-07T14:30:00Z
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "2026-07-07T14h30-decisions.toml"), []byte(proposal), 0o644))

	pending, err := repo.WikiListPending()
	require.NoError(t, err)
	assert.Len(t, pending, 1)
	assert.Equal(t, "abc12345", pending[0].ID)
	assert.Equal(t, "decisions", pending[0].Page)
	assert.Equal(t, "CONFIRMED", pending[0].Confidence)
	assert.Equal(t, "documentarian", pending[0].Author)
}

func TestWikiListPendingEmpty(t *testing.T) {
	repo := setupTestRepo(t)

	pending, err := repo.WikiListPending()
	require.NoError(t, err)
	assert.Empty(t, pending)
}
