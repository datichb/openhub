package workflow

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// TestDiagnosticCodesAreTranslated checks that every diagnostic code used by
// the package has a message in both locales.
func TestDiagnosticCodesAreTranslated(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`(?:errDiag|warnDiag|v\.err|v\.warn|nodeError\(node,)\(?\s*"([a-z_]+)"`)
	codes := map[string]bool{"skill_closure": true, "skill_duplicate": true}
	for _, f := range files {
		if filepath.Ext(f) != ".go" || len(f) > 8 && f[len(f)-8:] == "_test.go" {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(data), -1) {
			codes[m[1]] = true
		}
	}
	if len(codes) < 60 {
		t.Fatalf("only %d codes found: the scan is broken", len(codes))
	}
	prev := i18n.Locale()
	defer i18n.SetLocale(prev)
	for _, locale := range []string{"fr", "en"} {
		i18n.SetLocale(locale)
		for code := range codes {
			key := "workflow.diag." + code
			if i18n.T(key) == key {
				t.Errorf("[%s] missing translation for %s", locale, key)
			}
		}
	}
}
