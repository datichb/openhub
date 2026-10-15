package docstest

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	wikisvc "github.com/datichb/openhub/cli/internal/services/wiki"
)

// TestOwnWiki: the wiki of the repository (docs/wiki/) passes wiki_check
// without any issue, freshness aside (a cited file is only required to
// exist: its date moves with every commit). On a god nodes finding, run
// `oh wiki check --fix .` at the root.
func TestOwnWiki(t *testing.T) {
	root := repoRoot(t)
	exists := func(_ context.Context, root, file string) (time.Time, bool) {
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(file)))
		return time.Time{}, err == nil
	}
	rep, err := wikisvc.Check(t.Context(), root, exists)
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range rep.Issues {
		t.Errorf("docs/wiki/%s:%d: %s %s: %s", i.Page, i.Line, i.Severity, i.Kind, i.Message)
	}
}
