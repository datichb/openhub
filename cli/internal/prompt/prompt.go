// Package prompt provides project context detection and prompt building
// for opencode sessions.
package prompt

import (
	"os"
	"path/filepath"
	"strings"
)

// StackInfo holds detected technology stack information for a project.
type StackInfo struct {
	Language    string
	Framework   string
	PackageManager string
	TestRunner  string
	HasDocker   bool
	HasCI       bool
}

// DetectStack analyzes a project directory to detect its technology stack.
func DetectStack(projectPath string) StackInfo {
	info := StackInfo{}

	// Language detection
	switch {
	case fileExists(projectPath, "go.mod"):
		info.Language = "go"
		info.PackageManager = "go modules"
		if fileExists(projectPath, "Makefile") {
			info.TestRunner = "make test"
		} else {
			info.TestRunner = "go test ./..."
		}
	case fileExists(projectPath, "package.json"):
		info.Language = "typescript"
		if fileExists(projectPath, "bun.lockb") || fileExists(projectPath, "bunfig.toml") {
			info.PackageManager = "bun"
		} else if fileExists(projectPath, "pnpm-lock.yaml") {
			info.PackageManager = "pnpm"
		} else if fileExists(projectPath, "yarn.lock") {
			info.PackageManager = "yarn"
		} else {
			info.PackageManager = "npm"
		}
		info.TestRunner = detectJSTestRunner(projectPath)
		info.Framework = detectJSFramework(projectPath)
	case fileExists(projectPath, "pyproject.toml") || fileExists(projectPath, "setup.py"):
		info.Language = "python"
		if fileExists(projectPath, "poetry.lock") {
			info.PackageManager = "poetry"
		} else if fileExists(projectPath, "Pipfile") {
			info.PackageManager = "pipenv"
		} else {
			info.PackageManager = "pip"
		}
		info.TestRunner = "pytest"
	case fileExists(projectPath, "Cargo.toml"):
		info.Language = "rust"
		info.PackageManager = "cargo"
		info.TestRunner = "cargo test"
	case fileExists(projectPath, "build.gradle") || fileExists(projectPath, "build.gradle.kts"):
		info.Language = "java"
		info.PackageManager = "gradle"
		info.TestRunner = "gradle test"
	case fileExists(projectPath, "pom.xml"):
		info.Language = "java"
		info.PackageManager = "maven"
		info.TestRunner = "mvn test"
	}

	// Docker
	info.HasDocker = fileExists(projectPath, "Dockerfile") ||
		fileExists(projectPath, "docker-compose.yml") ||
		fileExists(projectPath, "docker-compose.yaml")

	// CI
	info.HasCI = dirExists(projectPath, ".github/workflows") ||
		fileExists(projectPath, ".gitlab-ci.yml") ||
		fileExists(projectPath, "Jenkinsfile")

	return info
}

// BuildContext builds a context string from stack info for injection into prompts.
func BuildContext(info StackInfo) string {
	var parts []string

	if info.Language != "" {
		parts = append(parts, "Language: "+info.Language)
	}
	if info.Framework != "" {
		parts = append(parts, "Framework: "+info.Framework)
	}
	if info.PackageManager != "" {
		parts = append(parts, "Package manager: "+info.PackageManager)
	}
	if info.TestRunner != "" {
		parts = append(parts, "Test command: "+info.TestRunner)
	}
	if info.HasDocker {
		parts = append(parts, "Docker: yes")
	}
	if info.HasCI {
		parts = append(parts, "CI: yes")
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n")
}

func detectJSTestRunner(path string) string {
	pkg := readFile(filepath.Join(path, "package.json"))
	switch {
	case strings.Contains(pkg, "vitest"):
		return "vitest"
	case strings.Contains(pkg, "jest"):
		return "jest"
	case strings.Contains(pkg, "mocha"):
		return "mocha"
	default:
		return "npm test"
	}
}

func detectJSFramework(path string) string {
	pkg := readFile(filepath.Join(path, "package.json"))
	switch {
	case strings.Contains(pkg, "next"):
		return "Next.js"
	case strings.Contains(pkg, "nuxt"):
		return "Nuxt"
	case strings.Contains(pkg, "svelte"):
		return "SvelteKit"
	case strings.Contains(pkg, "\"react\""):
		return "React"
	case strings.Contains(pkg, "\"vue\""):
		return "Vue"
	case strings.Contains(pkg, "\"express\""):
		return "Express"
	case strings.Contains(pkg, "\"fastify\""):
		return "Fastify"
	default:
		return ""
	}
}

func fileExists(base, name string) bool {
	_, err := os.Stat(filepath.Join(base, name))
	return err == nil
}

func dirExists(base, name string) bool {
	info, err := os.Stat(filepath.Join(base, name))
	return err == nil && info.IsDir()
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
