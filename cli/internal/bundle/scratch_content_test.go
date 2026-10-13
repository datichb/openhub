package bundle

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SCR: the content never leads an agent to put a session scratch path in the
// documentation or the wiki (wiki_propose refuses it too, LD-J; the files of
// docs/ written directly have no such guard).
//   - no content cites a concrete scratch path (it would be copied as is);
//   - the skills and agents that write the documentation or the wiki do not
//     mention the scratch folders at all;
//   - elsewhere, a line naming the scratch folders and the documentation or
//     the wiki says it never goes there.
var (
	scratchPath    = regexp.MustCompile(`\.oh/sessions/\S*scratch|scratch/<`)
	scratchMention = regexp.MustCompile(`oh\.scratch|[Bb]rouillons? de (session|reproduction)|dossier de brouillons|[Ss]cratch folder`)
	docDestination = regexp.MustCompile(`(?i)docs/|wiki|documentation|living-docs`)
	negation       = regexp.MustCompile(`(?i)\bjamais\b|\bn'est\b.*\bpas\b|\bnever\b|\bnot\b|\binterdit`)
)

// docWriters are the content files of the agents that write the docs.
func docWriter(rel string) bool {
	for _, p := range []string{"skills/documentarian/", "skills/templates/documentarian-", "skills/shared/living-docs-enrichment", "skills/shared/wiki-", "agents/documentation/"} {
		if strings.HasPrefix(rel, p) {
			return true
		}
	}
	return false
}

func TestScratchNeverInTheDocs(t *testing.T) {
	root := repoHub(t)
	checked := 0
	for _, dir := range []string{"agents", "skills", "workflows"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			switch filepath.Ext(p) {
			case ".md", ".tmpl", ".yaml":
			default:
				return nil
			}
			rel, _ := filepath.Rel(root, p)
			rel = filepath.ToSlash(rel)
			data, err := os.ReadFile(p)
			require.NoError(t, err)
			checked++
			for i, line := range strings.Split(string(data), "\n") {
				at := rel + ":" + strconv.Itoa(i+1)
				assert.False(t, scratchPath.MatchString(line), "%s: a concrete scratch path (oh gives it in the session context): %s", at, line)
				if !scratchMention.MatchString(line) {
					continue
				}
				assert.False(t, docWriter(rel), "%s: a documentation skill names the scratch folders: %s", at, line)
				if docDestination.MatchString(line) {
					assert.True(t, negation.MatchString(line), "%s: scratch folders and documentation on one line, without saying never: %s", at, line)
				}
			}
			return nil
		})
		require.NoError(t, err)
	}
	assert.Greater(t, checked, 100)
}

// The lint catches what it is for.
func TestScratchLintCatches(t *testing.T) {
	assert.True(t, scratchPath.MatchString("écris-le dans ~/.oh/sessions/<session>/scratch/debugger/"))
	assert.True(t, scratchPath.MatchString("copie scratch/<agent>/repro.sh"))
	bad := "Ajoute le brouillon de reproduction (`oh.scratch`) à la page du wiki."
	assert.True(t, scratchMention.MatchString(bad) && docDestination.MatchString(bad) && !negation.MatchString(bad))
	good := "Le chemin d'un brouillon de session n'est jamais cité dans la documentation ni le wiki."
	assert.True(t, scratchMention.MatchString(good) && negation.MatchString(good))
	assert.True(t, docWriter("skills/documentarian/doc-wiki-protocol.md"))
}
