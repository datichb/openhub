package cmd

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

// helpKeyShape is the key of a help text of the locales, by convention:
// cmd.<command path>.short|long|example|deprecated and
// cmd.<command path>.flags.<flag name> (localizeCommands reads them from the
// command tree, the code never names them).
var helpKeyShape = regexp.MustCompile(`^cmd\.[^ ]+\.(short|long|example|deprecated|flags\.[^.]+)$`)

// helpKeysOfTheTree returns the help keys that the command tree reads.
func helpKeysOfTheTree() map[string]bool {
	keys := map[string]bool{}
	var walk func(c *cobra.Command, inherited []*pflag.FlagSet)
	walk = func(c *cobra.Command, inherited []*pflag.FlagSet) {
		key := cmdI18nKey(c)
		for _, suffix := range []string{".short", ".long", ".example", ".deprecated"} {
			keys[key+suffix] = true
		}
		visit := func(fs *pflag.FlagSet) {
			fs.VisitAll(func(f *pflag.Flag) { keys[key+".flags."+f.Name] = true })
		}
		visit(c.Flags())
		visit(c.PersistentFlags())
		for _, fs := range inherited {
			visit(fs)
		}
		next := append(append([]*pflag.FlagSet{}, inherited...), c.PersistentFlags())
		for _, sub := range c.Commands() {
			walk(sub, next)
		}
	}
	walk(rootCmd, nil)
	return keys
}

// sourceText is the text of every non-test Go file of the module: a help key
// that the code names (i18n.T("cmd.mcp.list.short")) is used even though its
// command is not in the tree under that path.
func sourceText(t *testing.T) string {
	t.Helper()
	root := ".."
	var sb strings.Builder
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); path != root && (name == "testdata" || name == "hubcontent" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sb.Write(data)
		}
		return nil
	})
	require.NoError(t, err)
	return sb.String()
}

// The help keys of the locales are read from the command tree, so
// TestNoOrphanKeys (which looks for the keys in the code) leaves them out. This
// test is its counterpart for them: a help key whose command or option is gone
// is a translation to maintain for nothing.
func TestNoOrphanHelpKeys(t *testing.T) {
	tree := helpKeysOfTheTree()
	var source string
	for _, lang := range []string{"en", "fr"} {
		data, err := os.ReadFile(filepath.Join("..", "internal", "i18n", "locales", lang+".json"))
		require.NoError(t, err)
		var messages map[string]string
		require.NoError(t, json.Unmarshal(data, &messages))
		var orphans []string
		for key := range messages {
			if !helpKeyShape.MatchString(key) || tree[key] {
				continue
			}
			if source == "" {
				source = sourceText(t)
			}
			if strings.Contains(source, `"`+key+`"`) {
				continue
			}
			orphans = append(orphans, key)
		}
		sort.Strings(orphans)
		if len(orphans) > 0 {
			t.Errorf("%s.json: %d help key(s) match no command or option of the tree and are named nowhere in the code (remove them from en.json and fr.json):\n  %s",
				lang, len(orphans), strings.Join(orphans, "\n  "))
		}
	}
}
