package bricks

import (
	"os"
	"path/filepath"
	"testing"
)

// A8: frontend detection used to ship the frontend domain skills.
func TestHasFrontend(t *testing.T) {
	mk := func(files map[string]string) string {
		dir := t.TempDir()
		for rel, content := range files {
			p := filepath.Join(dir, rel)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}
	cases := []struct {
		name  string
		files map[string]string
		want  bool
	}{
		{"python api", map[string]string{"pyproject.toml": "fastapi", "app/main.py": "", "htmlcov/index.html": "", "docs/a.html": ""}, false},
		{"go service", map[string]string{"go.mod": "module x", "node_modules/react/index.jsx": ""}, false},
		{"tooling package.json", map[string]string{"package.json": `{"devDependencies":{"prettier":"3"}}`}, false},
		{"react", map[string]string{"package.json": `{"dependencies":{"react":"19"}}`}, true},
		{"monorepo web", map[string]string{"go.mod": "", "web/package.json": `{"devDependencies":{"vite":"6"}}`}, true},
		{"vue sources", map[string]string{"src/components/App.vue": ""}, true},
		{"django templates", map[string]string{"manage.py": "", "shop/templates/shop/list.html": ""}, true},
		{"root index.html", map[string]string{"index.html": ""}, true},
		{"react native", map[string]string{"package.json": `{"dependencies":{"react-native":"0.76","react":"18"}}`, "src/App.tsx": ""}, false},
		{"expo web", map[string]string{"package.json": `{"dependencies":{"expo":"51","react":"18","react-dom":"18"}}`}, true},
		{"native and web apps", map[string]string{"apps/mobile/package.json": `{"dependencies":{"expo":"51"}}`, "apps/web/package.json": `{"dependencies":{"next":"15"}}`}, true},
		{"workspace web app", map[string]string{"package.json": `{"workspaces":{"packages":["a/b/c/d/*"]}}`, "a/b/c/d/web/package.json": `{"dependencies":{"vue":"3"}}`}, true},
	}
	for _, c := range cases {
		if got := HasFrontend(mk(c.files)); got != c.want {
			t.Errorf("%s: HasFrontend = %v, want %v", c.name, got, c.want)
		}
	}
	if !HasFrontend("") || !HasFrontend(filepath.Join(t.TempDir(), "missing")) {
		t.Error("an unknown project must not filter anything")
	}
}
