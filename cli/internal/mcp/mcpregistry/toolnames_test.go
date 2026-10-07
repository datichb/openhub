package mcpregistry

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// QB3: the built-in MCP servers name their tools without their server
// prefix: opencode shows `<server>_<tool>` to the model, so `gitlab_get_project`
// (the name the agents and skills use) is the tool `get_project` of the
// server `gitlab` (it was `gitlab_gitlab_get_project`, and the permission
// rules naming a GitLab tool never matched).
func TestBuiltinToolsHaveNoServerPrefix(t *testing.T) {
	for _, srv := range []string{"figma", "github", "gitlab", "gslides", "jira", "linear", "team"} {
		files, err := filepath.Glob(filepath.Join("..", srv, "*.go"))
		if err != nil || len(files) == 0 {
			t.Fatalf("%s: no source", srv)
		}
		re := regexp.MustCompile(`Name:\s+"` + srv + `_`)
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if re.Match(data) {
				t.Errorf("%s: a tool of %s is named with its server prefix", f, srv)
			}
		}
	}
}

var toolNameRe = regexp.MustCompile(`Name:\s+"([a-z_]+)"`)

// builtinTools reads the tool names of a built-in server from its source.
func builtinTools(t *testing.T, srv string) map[string]bool {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join("..", srv, "*.go"))
	out := map[string]bool{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range toolNameRe.FindAllSubmatch(data, -1) {
			out[string(m[1])] = true
		}
	}
	return out
}

// QB3: the MCP tools the hub content tells agents to call exist (the figma
// skills called search_figma_files, get_figma_file_nodes, detect_ui_signals…).
func TestHubContentCallsExistingMCPTools(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	if _, err := os.Stat(filepath.Join(root, "agents")); err != nil {
		t.Skip("hub content not found")
	}
	var files []string
	for _, dir := range []string{"agents", "skills", "workflows"} {
		_ = filepath.WalkDir(filepath.Join(root, dir), func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && (strings.HasSuffix(p, ".md") || strings.HasSuffix(p, ".tmpl")) {
				files = append(files, p)
			}
			return nil
		})
	}
	for _, srv := range []string{"figma", "github", "gitlab", "gslides", "jira", "linear", "team"} {
		tools := builtinTools(t, srv)
		if len(tools) == 0 {
			t.Fatalf("%s: no tool found", srv)
		}
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			// A called or quoted identifier naming the server (`gitlab_x`,
			// `search_figma_files(…)`) is one of its tools, as the model sees
			// it: <server>_<tool>.
			for _, m := range calledIdentRe.FindAllSubmatch(data, -1) {
				id := string(m[1])
				if !slices.Contains(strings.Split(id, "_"), srv) {
					continue
				}
				if tool, ok := strings.CutPrefix(id, srv+"_"); !ok || !tools[tool] {
					t.Errorf("%s: tool %s does not exist on the %s MCP server", f, id, srv)
				}
			}
		}
	}
}

var calledIdentRe = regexp.MustCompile("(?:`|\\b)([a-z]+(?:_[a-z]+)+)(?:`|\\()")
