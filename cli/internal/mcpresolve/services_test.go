package mcpresolve

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServicesAreTheKnownOnes(t *testing.T) {
	assert.Equal(t, []string{"figma", "github", "gitlab", "gslides", "jira", "linear"}, Names())

	var order []string
	for _, s := range Services() {
		order = append(order, s.Name)
		assert.NotEmpty(t, s.Label, s.Name)
		assert.NotEmpty(t, s.TokenEnv, s.Name)
	}
	assert.Equal(t, []string{"gitlab", "jira", "figma", "gslides", "github", "linear"}, order, "display order")

	got := Services()
	got[0].Name = "changed"
	assert.Equal(t, "gitlab", Services()[0].Name, "the catalog is not shared")
}

func TestLookup(t *testing.T) {
	s, ok := Lookup("  Linear ")
	require.True(t, ok)
	assert.Equal(t, "linear", s.Name)
	assert.Equal(t, "LINEAR_API_KEY", s.TokenEnv)

	for _, unknown := range []string{"", "team", "workflow", "slack"} {
		_, ok := Lookup(unknown)
		assert.False(t, ok, unknown)
	}
}

func TestServiceHasURL(t *testing.T) {
	for name, want := range map[string]bool{
		"gitlab": true, "jira": true,
		"figma": false, "gslides": false, "github": false, "linear": false,
		"custom": true, // unknown: the benefit of the doubt
	} {
		assert.Equal(t, want, ServiceHasURL(name), name)
	}
}

// The catalog tells the truth about the servers: each token variable, and
// each URL variable, appears in the source of the server that is said to read
// it; a server that has no URL variable does not read a <NAME>_URL one.
func TestCatalogMatchesTheSourceOfTheServers(t *testing.T) {
	for _, s := range Services() {
		src, err := os.ReadFile(filepath.Join("..", "mcp", s.Name, "server.go"))
		require.NoError(t, err, s.Name)
		text := string(src)

		assert.Contains(t, text, `"`+s.TokenEnv+`"`, "%s reads %s", s.Name, s.TokenEnv)
		urlVar := strings.ToUpper(s.Name) + "_URL"
		if s.URLEnv != "" {
			assert.Equal(t, urlVar, s.URLEnv, s.Name)
			assert.Contains(t, text, `"`+s.URLEnv+`"`, "%s reads %s", s.Name, s.URLEnv)
		} else {
			assert.NotContains(t, text, `"`+urlVar+`"`, "%s declares no URL variable but reads %s", s.Name, urlVar)
		}
	}
}

func TestServicesAreSortedInNames(t *testing.T) {
	assert.True(t, sort.StringsAreSorted(Names()))
}
