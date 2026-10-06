package workflow

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// TestImpactCodesAreTranslated checks that every impact code has a message.
func TestImpactCodesAreTranslated(t *testing.T) {
	files, _ := filepath.Glob("*.go")
	re := regexp.MustCompile(`add\(Impact[A-Za-z]+, "([a-z_]+)"`)
	codes := map[string]bool{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
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
	if len(codes) < 25 {
		t.Fatalf("only %d codes found: the scan is broken", len(codes))
	}
	prev := i18n.Locale()
	defer i18n.SetLocale(prev)
	for _, locale := range []string{"fr", "en"} {
		i18n.SetLocale(locale)
		for code := range codes {
			if key := "teamstate.workflow.impact." + code; i18n.T(key) == key {
				t.Errorf("[%s] missing %s", locale, key)
			}
		}
	}
}
