package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/wiki"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

func decideFlagsCmd(t *testing.T, args ...string) (*cobra.Command, error) {
	t.Helper()
	cmd := &cobra.Command{}
	for _, f := range []string{"accept", "refuse", "reject", "pending", "edit"} {
		cmd.Flags().Bool(f, false, "")
	}
	cmd.Flags().String("content", "", "")
	cmd.Flags().String("file", "", "")
	return cmd, cmd.ParseFlags(args)
}

func TestWikiDecisionFlags(t *testing.T) {
	prev := i18n.Locale()
	i18n.SetLocale("fr")
	t.Cleanup(func() { i18n.SetLocale(prev) })
	file := filepath.Join(t.TempDir(), "c.md")
	require.NoError(t, os.WriteFile(file, []byte("ligne 1\nligne 2\n"), 0o644))
	for _, c := range []struct {
		args []string
		want wikiDecision
		err  string
	}{
		{args: []string{"--accept"}, want: wikiDecision{Choice: wiki.ChoiceAccept}},
		{args: []string{"--reject"}, want: wikiDecision{Choice: wiki.ChoiceRefuse}},
		{args: []string{"--refuse"}, want: wikiDecision{Choice: wiki.ChoiceRefuse}},
		{args: []string{"--pending"}, want: wikiDecision{Choice: wiki.ChoicePend}},
		{args: []string{"--edit"}, want: wikiDecision{Choice: wiki.ChoiceEdit, Editor: true}},
		{args: []string{"--content", "neuf"}, want: wikiDecision{Choice: wiki.ChoiceEdit, Content: "neuf"}},
		{args: []string{"--edit", "--file", file}, want: wikiDecision{Choice: wiki.ChoiceEdit, Content: "ligne 1\nligne 2\n"}},
		{args: nil, err: "une seule"},
		{args: []string{"--accept", "--refuse"}, err: "une seule"},
		{args: []string{"--accept", "--content", "x"}, err: "une seule"},
		{args: []string{"--content", "x", "--file", file}, err: "exclusifs"},
		{args: []string{"--content", "  "}, err: "vide"},
	} {
		cmd, err := decideFlagsCmd(t, c.args...)
		require.NoError(t, err)
		got, err := wikiDecisionFlags(cmd)
		if c.err != "" {
			assert.ErrorContains(t, err, c.err, c.args)
			continue
		}
		require.NoError(t, err, c.args)
		assert.Equal(t, c.want, got, c.args)
	}
	cmd, err := decideFlagsCmd(t, "--file", "-")
	require.NoError(t, err)
	cmd.SetIn(strings.NewReader("depuis stdin"))
	got, err := wikiDecisionFlags(cmd)
	require.NoError(t, err)
	assert.Equal(t, "depuis stdin", got.Content)
}

// `oh wiki decide`: the choices of the card, from the command line, on
// the queue of the project.
func TestWikiDecide(t *testing.T) {
	prev := i18n.Locale()
	i18n.SetLocale("fr")
	t.Cleanup(func() { i18n.SetLocale(prev) })
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	for _, id := range []string{"p1", "p2"} {
		_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES (?, ?, ?)`, id, id, "/"+id)
		require.NoError(t, err)
	}
	svc := &wiki.Service{Store: sqlite.NewWikiProposalStore(st)}
	propose := func(project, page, content string) string {
		p, err := svc.Propose(ctx, wiki.Filed{ProjectID: project, SessionID: "ses_a", Agent: "reviewer",
			Proposal: wiki.Proposal{Page: page, Content: content, Tag: "DÉDUIT"}})
		require.NoError(t, err)
		return p.ID()
	}
	a := propose("p1", "index.md", "un")
	b := propose("p1", "technical/conventions.md", "deux")
	c := propose("p1", "index.md", "trois")
	other := propose("p2", "index.md", "ailleurs")
	run := func(ids []string, d wikiDecision) (string, error) {
		var out bytes.Buffer
		err := wikiDecide(ctx, svc, "p1", ids, d, &out)
		return out.String(), err
	}
	status := func(id string) *domain.WikiProposal {
		p, err := svc.Get(ctx, "p1", id)
		require.NoError(t, err)
		return p
	}

	out, err := run([]string{a, b}, wikiDecision{Choice: wiki.ChoiceAccept})
	require.NoError(t, err)
	assert.Contains(t, out, a+" (index.md) : acceptée")
	assert.Contains(t, out, b+" (technical/conventions.md) : acceptée")
	assert.Equal(t, domain.WikiAccepted, status(a).Status)
	assert.Equal(t, "oh", status(a).DecidedBy)

	out, err = run([]string{b}, wikiDecision{Choice: wiki.ChoicePend})
	require.NoError(t, err)
	assert.Contains(t, out, "en attente")
	assert.Equal(t, domain.WikiPending, status(b).Status)

	// Edit with a given content, on several lines; the original is kept.
	out, err = run([]string{b}, wikiDecision{Choice: wiki.ChoiceEdit, Content: "deux\n\nsuite"})
	require.NoError(t, err)
	assert.Contains(t, out, "modifiée et acceptée")
	assert.Equal(t, "deux\n\nsuite", status(b).Content)
	assert.Equal(t, "deux", status(b).Original)

	// Edit in $EDITOR: untouched records nothing, changed is an edit.
	prevEdit := wikiEditText
	t.Cleanup(func() { wikiEditText = prevEdit })
	var opened []string
	result := "trois\n"
	wikiEditText = func(name, content string) (string, error) {
		opened = append(opened, name+"|"+content)
		return result, nil
	}
	out, err = run([]string{c}, wikiDecision{Choice: wiki.ChoiceEdit, Editor: true})
	require.NoError(t, err)
	assert.Contains(t, out, "inchangé")
	assert.Equal(t, domain.WikiPending, status(c).Status)
	result = "trois\nquatre\n"
	_, err = run([]string{c}, wikiDecision{Choice: wiki.ChoiceEdit, Editor: true})
	require.NoError(t, err)
	assert.Equal(t, []string{c + ".md|trois", c + ".md|trois"}, opened)
	assert.Equal(t, "trois\nquatre", status(c).Content)

	// Refuse; then a closed one, an unknown one and a bad id fail, the
	// others are still decided; the ids are those of the project (another
	// project has its own wp-1).
	_, err = run([]string{c}, wikiDecision{Choice: wiki.ChoiceRefuse})
	require.NoError(t, err)
	assert.Equal(t, domain.WikiRefused, status(c).Status)
	out, err = run([]string{c, "wp-99", "x-1", a}, wikiDecision{Choice: wiki.ChoiceRefuse})
	assert.ErrorContains(t, err, "au moins une proposition")
	assert.Contains(t, out, c+" : proposition déjà appliquée ou refusée")
	assert.Contains(t, out, "wp-99 : proposition introuvable")
	assert.Contains(t, out, "x-1 : un identifiant de proposition s'écrit wp-<numéro>")
	assert.Equal(t, domain.WikiRefused, status(a).Status, "decided after the failures")
	p2, err := svc.Get(ctx, "p2", other)
	require.NoError(t, err)
	assert.Equal(t, domain.WikiPending, p2.Status, "another project's queue untouched")
	out, err = run([]string{c}, wikiDecision{Choice: wiki.ChoiceEdit, Editor: true})
	assert.Error(t, err)
	assert.Contains(t, out, "déjà appliquée ou refusée", "no editor for a closed proposal")
	assert.Len(t, opened, 2)
}
