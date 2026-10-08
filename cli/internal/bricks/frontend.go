package bricks

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// FrontendSkills are the domain skills of a web frontend: shipped only when
// the project has one (A8: they were delivered to every project through the
// agents' menus of domain skills).
var FrontendSkills = []string{
	"developer/dev-standards-frontend",
	"developer/dev-standards-frontend-a11y",
	"developer/dev-standards-frontend-data",
}

// frontendDeps are npm packages of a web frontend.
var frontendDeps = []string{
	"react", "react-dom", "preact", "vue", "nuxt", "next", "svelte", "@sveltejs/kit",
	"@angular/core", "solid-js", "lit", "astro", "@remix-run/react", "react-native", "expo", "vite",
}

// frontendExts are source files of a web frontend.
var frontendExts = map[string]bool{".tsx": true, ".jsx": true, ".vue": true, ".svelte": true, ".astro": true}

// skippedDirs are never scanned (dependencies, builds, reports, tooling).
var skippedDirs = map[string]bool{
	"node_modules": true, ".git": true, "vendor": true, "dist": true, "build": true, "target": true,
	".venv": true, "venv": true, "__pycache__": true, "htmlcov": true, "coverage": true, "site": true,
	"docs": true, ".beads": true, ".next": true, ".nuxt": true, ".svelte-kit": true,
}

// htmlDirs are the folders whose HTML files are a frontend (server templates
// and static pages, not reports).
var htmlDirs = map[string]bool{"templates": true, "public": true, "static": true, "src": true, "app": true, "pages": true}

const (
	frontendMaxDepth = 4
	frontendMaxFiles = 20000
)

// HasFrontend reports whether a project has a web frontend: a package.json
// (root or sub-folder) depending on a frontend framework, frontend sources
// (.tsx, .jsx, .vue, .svelte, .astro), a root index.html or HTML templates.
// The scan is bounded (depth and number of entries); an unreadable or
// missing path counts as a frontend (nothing is filtered).
func HasFrontend(projectPath string) bool {
	if projectPath == "" {
		return true
	}
	if st, err := os.Stat(projectPath); err != nil || !st.IsDir() {
		return true
	}
	if fileExistsIn(projectPath, "index.html") {
		return true
	}
	found, seen := false, 0
	_ = filepath.WalkDir(projectPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // unreadable entry: skipped
		}
		seen++
		if seen > frontendMaxFiles {
			return fs.SkipAll
		}
		rel, _ := filepath.Rel(projectPath, path)
		depth := len(strings.Split(rel, string(filepath.Separator)))
		if d.IsDir() {
			if path != projectPath && (skippedDirs[d.Name()] || depth > frontendMaxDepth) {
				return fs.SkipDir
			}
			return nil
		}
		name := d.Name()
		switch {
		case frontendExts[filepath.Ext(name)]:
			found = true
		case name == "package.json" && packageHasFrontend(path):
			found = true
		case filepath.Ext(name) == ".html" && inHTMLDir(rel):
			found = true
		}
		if found {
			return fs.SkipAll
		}
		return nil
	})
	return found
}

func packageHasFrontend(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var pkg struct {
		Dependencies    map[string]any `json:"dependencies"`
		DevDependencies map[string]any `json:"devDependencies"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return false
	}
	for _, dep := range frontendDeps {
		if _, ok := pkg.Dependencies[dep]; ok {
			return true
		}
		if _, ok := pkg.DevDependencies[dep]; ok {
			return true
		}
	}
	return false
}

func inHTMLDir(rel string) bool {
	for _, part := range strings.Split(filepath.Dir(rel), string(filepath.Separator)) {
		if htmlDirs[part] {
			return true
		}
	}
	return false
}

func fileExistsIn(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}
