package deploy

import (
	"github.com/datichb/openhub/cli/internal/prompt"
)

// stackSkillMapping maps detected stack attributes to skill refs in skills/developer/stacks/.
// Each key corresponds to a StackInfo field value (Language, Framework, or feature flag).
// The values are skill refs relative to the skills/ directory.
var stackSkillMapping = map[string][]string{
	// Languages
	"typescript": {"developer/stacks/dev-standards-typescript"},
	"python":     {"developer/stacks/dev-standards-python"},
	"java":       {"developer/stacks/dev-standards-kotlin"}, // Kotlin standards apply to JVM
	"rust":       {},                                        // no rust-specific stack skill yet
	"go":         {},                                        // no go-specific stack skill yet

	// JS/TS Frameworks
	"Next.js":    {"developer/stacks/dev-standards-nextjs", "developer/stacks/dev-standards-react"},
	"Nuxt":       {"developer/stacks/dev-standards-nuxtjs", "developer/stacks/dev-standards-vuejs"},
	"SvelteKit":  {},
	"React":      {"developer/stacks/dev-standards-react"},
	"Vue":        {"developer/stacks/dev-standards-vuejs"},
	"Express":    {"developer/stacks/dev-standards-express"},
	"Fastify":    {},

	// Python frameworks (detected via pyproject.toml/requirements.txt content)
	"django":  {"developer/stacks/dev-standards-django"},
	"fastapi": {"developer/stacks/dev-standards-fastapi"},
	"flask":   {},
	"rails":   {"developer/stacks/dev-standards-rails"},

	// Test runners
	"vitest": {"developer/stacks/dev-standards-vitest"},
	"jest":   {"developer/stacks/dev-standards-jest"},

	// Infrastructure (detected from HasDocker/HasCI flags)
	"docker":         {"developer/stacks/dev-standards-docker"},
	"github-actions": {"developer/stacks/dev-standards-github-actions"},
	"gitlab-ci":      {"developer/stacks/dev-standards-gitlab-ci"},
}

// ResolveStackSkills detects the project's tech stack and returns the skill refs
// that should be deployed as native skills (Bucket B) for the detected stack.
func ResolveStackSkills(projectPath string) []string {
	info := prompt.DetectStack(projectPath)

	seen := make(map[string]bool)
	var skills []string

	addSkills := func(key string) {
		refs, ok := stackSkillMapping[key]
		if !ok {
			return
		}
		for _, ref := range refs {
			if !seen[ref] {
				seen[ref] = true
				skills = append(skills, ref)
			}
		}
	}

	// Language
	if info.Language != "" {
		addSkills(info.Language)
	}

	// Framework
	if info.Framework != "" {
		addSkills(info.Framework)
	}

	// Test runner
	if info.TestRunner != "" {
		addSkills(info.TestRunner)
	}

	// Infrastructure features
	if info.HasDocker {
		addSkills("docker")
	}
	if info.HasCI {
		// Detect CI type from available files
		if prompt.HasGitHubActions(projectPath) {
			addSkills("github-actions")
		}
		if prompt.HasGitLabCI(projectPath) {
			addSkills("gitlab-ci")
		}
	}

	return skills
}
