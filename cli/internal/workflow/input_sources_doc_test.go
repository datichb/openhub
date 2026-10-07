package workflow

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// QB2: the sources of computed inputs are a public contract: the workflow
// schema reference (fr and en) lists exactly the sources of InputSources.
func TestInputSourcesAreDocumented(t *testing.T) {
	want := append([]string(nil), InputSources...)
	sort.Strings(want)
	re := regexp.MustCompile("`([a-z]+\\.[a-z_]+)\\([a-z]+\\)`")
	for _, lang := range []string{"fr", "en"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "reference", "workflow-schema."+lang+".md"))
		if err != nil {
			t.Fatal(err)
		}
		var section string
		for _, block := range strings.Split(string(data), "\n### ") {
			if strings.HasPrefix(block, "Entrées calculées (`from`)") || strings.HasPrefix(block, "Computed inputs (`from`)") {
				section = block
			}
		}
		if section == "" {
			t.Fatalf("%s: no section « computed inputs (`from`) »", lang)
		}
		seen := map[string]bool{}
		for _, m := range re.FindAllStringSubmatch(section, -1) {
			seen[m[1]] = true
		}
		var got []string
		for s := range seen {
			got = append(got, s)
		}
		sort.Strings(got)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s: documented sources %v, InputSources %v", lang, got, want)
		}
	}
}
