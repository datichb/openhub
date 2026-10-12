// Package prompt provides project context detection and prompt building
// for agentic sessions.
package prompt

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/datichb/openhub/cli/internal/bricks"
)

// StackInfo holds detected technology stack information for a project.
type StackInfo struct {
	// Language is the primary language; Languages are all the languages
	// found, primary first (see languagePriority).
	Language  string
	Languages []string
	// Framework is the primary framework; Frameworks are all the
	// frameworks found, primary first (see frameworkPriority).
	Framework      string
	Frameworks     []string
	PackageManager string
	TestRunner     string
	HasDocker      bool
	HasCI          bool
	// Signals are the raw stack signals (bricks.DetectSignals).
	Signals bricks.Signals
}

// languagePriority orders the language signals (signal, label): the primary
// language is the first one found at the project root, else the first one
// found anywhere (monorepo without root manifest).
var languagePriority = [][2]string{
	{"golang", "go"}, {"typescript", "typescript"}, {"javascript", "javascript"},
	{"python", "python"}, {"ruby", "ruby"}, {"rust", "rust"}, {"android", "kotlin"},
	{"java", "java"}, {"php", "php"}, {"dart", "dart"}, {"swift", "swift"},
}

// frameworkPriority orders the framework signals (signal, label).
var frameworkPriority = [][2]string{
	{"nextjs", "Next.js"}, {"nuxtjs", "Nuxt"}, {"svelte", "SvelteKit"}, {"angular", "Angular"},
	{"react-native", "React Native"}, {"react", "React"}, {"vuejs", "Vue"}, {"nestjs", "NestJS"},
	{"express", "Express"}, {"fastify", "Fastify"}, {"django", "django"}, {"fastapi", "fastapi"},
	{"flask", "flask"}, {"rails", "rails"}, {"laravel", "laravel"}, {"springboot", "Spring Boot"},
	{"flutter", "Flutter"},
}

// DetectStack analyzes a project directory to detect its technology stack,
// from the signals of bricks.DetectSignals.
func DetectStack(projectPath string) StackInfo {
	sig := bricks.DetectSignals(projectPath)
	info := StackInfo{Signals: sig}

	var found [][2]string
	for _, l := range languagePriority {
		if sig.Has(l[0]) && (l[0] != "javascript" || !sig.Has("typescript")) {
			found = append(found, l)
		}
	}
	primary := ""
	if len(found) > 0 {
		main := found[0]
		for _, l := range found {
			if sig.At(l[0], ".") {
				main = l
				break
			}
		}
		primary, info.Language = main[0], main[1]
		info.Languages = []string{main[1]}
		for _, l := range found {
			if l != main {
				info.Languages = append(info.Languages, l[1])
			}
		}
	}
	for _, f := range frameworkPriority {
		if sig.Has(f[0]) {
			info.Frameworks = append(info.Frameworks, f[1])
		}
	}
	if len(info.Frameworks) > 0 {
		info.Framework = info.Frameworks[0]
	}

	info.PackageManager, info.TestRunner = toolsFor(primary, sig)
	info.HasDocker = sig.Has("docker")
	info.HasCI = sig.Has("github-actions") || sig.Has("gitlab-ci") || sig.Has("jenkins")
	return info
}

// firstOf returns the label of the first signal found (root first).
func firstOf(sig bricks.Signals, pairs ...string) string {
	for _, root := range []bool{true, false} {
		for i := 0; i+1 < len(pairs); i += 2 {
			if (root && sig.At(pairs[i], ".")) || (!root && sig.Has(pairs[i])) {
				return pairs[i+1]
			}
		}
	}
	return ""
}

// toolsFor returns the package manager and test command of the primary
// language.
func toolsFor(primary string, sig bricks.Signals) (pm, test string) {
	switch primary {
	case "golang":
		if sig.At("make", ".") {
			return "go modules", "make test"
		}
		return "go modules", "go test ./..."
	case "typescript", "javascript":
		pm = firstOf(sig, "bun", "bun", "pnpm", "pnpm", "yarn", "yarn")
		if pm == "" {
			pm = "npm"
		}
		test = firstOf(sig, "vitest", "vitest", "jest", "jest", "mocha", "mocha")
		if test == "" {
			test = "npm test"
		}
		return pm, test
	case "python":
		pm = firstOf(sig, "poetry", "poetry", "pipenv", "pipenv", "uv", "uv")
		if pm == "" {
			pm = "pip"
		}
		return pm, "pytest"
	case "ruby":
		switch {
		case sig.Has("rspec"):
			return "bundler", "rspec"
		case sig.Has("rails"):
			return "bundler", "bin/rails test"
		}
		return "bundler", ""
	case "rust":
		return "cargo", "cargo test"
	case "android", "java":
		if sig.Has("gradle") {
			return "gradle", "gradle test"
		}
		if sig.Has("maven") {
			return "maven", "mvn test"
		}
	case "php":
		return "composer", ""
	}
	return "", ""
}

// BuildContext builds a context string from stack info for injection into prompts.
func BuildContext(info StackInfo) string {
	var parts []string

	if info.Language != "" {
		parts = append(parts, "Language: "+info.Language)
	}
	if others := others(info.Languages, info.Language); others != "" {
		parts = append(parts, "Other languages: "+others)
	}
	if info.Framework != "" {
		parts = append(parts, "Framework: "+info.Framework)
	}
	if others := others(info.Frameworks, info.Framework); others != "" {
		parts = append(parts, "Other frameworks: "+others)
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

// others joins the values of list other than primary.
func others(list []string, primary string) string {
	var out []string
	for _, v := range list {
		if v != primary {
			out = append(out, v)
		}
	}
	return strings.Join(out, ", ")
}

func fileExists(base, name string) bool {
	_, err := os.Stat(filepath.Join(base, name))
	return err == nil
}

func dirExists(base, name string) bool {
	info, err := os.Stat(filepath.Join(base, name))
	return err == nil && info.IsDir()
}

// HasGitHubActions returns true if the project has GitHub Actions workflows.
func HasGitHubActions(projectPath string) bool {
	return dirExists(projectPath, ".github/workflows")
}

// HasGitLabCI returns true if the project has a GitLab CI configuration.
func HasGitLabCI(projectPath string) bool {
	return fileExists(projectPath, ".gitlab-ci.yml")
}
