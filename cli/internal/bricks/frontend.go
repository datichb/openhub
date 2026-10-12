package bricks

import (
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

// frontendDeps are npm packages of a web frontend (react counts only
// outside a React Native app: see DetectSignals).
var frontendDeps = []string{
	"react-dom", "preact", "vue", "nuxt", "next", "svelte", "@sveltejs/kit",
	"@angular/core", "solid-js", "lit", "astro", "@remix-run/react", "vite",
}

// frontendExts are source files of a web frontend.
var frontendExts = map[string]bool{".tsx": true, ".jsx": true, ".vue": true, ".svelte": true, ".astro": true}

// skippedDirs are never scanned (dependencies, builds, reports, tooling).
var skippedDirs = map[string]bool{
	"node_modules": true, ".git": true, "vendor": true, "dist": true, "build": true, "target": true,
	".venv": true, "venv": true, "__pycache__": true, "htmlcov": true, "coverage": true, "site": true,
	"docs": true, ".beads": true, ".next": true, ".nuxt": true, ".svelte-kit": true, ".output": true,
	".terraform": true, ".gradle": true, ".dart_tool": true, ".tox": true, ".mypy_cache": true,
	".pytest_cache": true, ".ruff_cache": true, ".turbo": true, ".cache": true, ".idea": true,
	".expo": true, "Pods": true, "DerivedData": true,
}

// htmlDirs are the folders whose HTML files are a frontend (server templates
// and static pages, not reports).
var htmlDirs = map[string]bool{"templates": true, "public": true, "static": true, "src": true, "app": true, "pages": true}

const (
	frontendMaxDepth = 4
	frontendMaxFiles = 20000
)

// HasFrontend reports whether a project has a web frontend (the "frontend"
// signal of DetectSignals): a package.json (root, sub-folder or workspace)
// depending on a web framework, web sources (.vue, .svelte, .astro; .tsx
// and .jsx unless the only JS app is a React Native one), a root index.html
// or HTML templates. A React Native app alone (react-native or expo without
// react-dom) is not a web frontend. An unreadable or missing path counts as
// a frontend (nothing is filtered).
func HasFrontend(projectPath string) bool {
	return FrontendIn(projectPath, DetectSignals(projectPath))
}

// FrontendIn is HasFrontend with the signals already detected in projectPath.
func FrontendIn(projectPath string, signals Signals) bool {
	if projectPath == "" {
		return true
	}
	if st, err := os.Stat(projectPath); err != nil || !st.IsDir() {
		return true
	}
	return signals.Has("frontend")
}

func inHTMLDir(rel string) bool {
	for _, part := range strings.Split(filepath.Dir(rel), string(filepath.Separator)) {
		if htmlDirs[part] {
			return true
		}
	}
	return false
}
