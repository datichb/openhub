package bricks

import "sort"

// stackSkillMapping maps the signals of DetectSignals to skill refs in
// skills/developer/stacks/ (relative to the skills/ directory). Every stack
// skill is reachable from at least one signal; the other signals (java,
// ruby, svelte, flask, package managers…) ship no skill.
var stackSkillMapping = map[string][]string{
	// Languages
	"typescript": {"developer/stacks/dev-standards-typescript"},
	"python":     {"developer/stacks/dev-standards-python"},
	"golang":     {"developer/stacks/dev-standards-golang"},
	"rust":       {"developer/stacks/dev-standards-rust"},

	// Frontend
	"vuejs":   {"developer/stacks/dev-standards-vuejs"},
	"react":   {"developer/stacks/dev-standards-react"},
	"nextjs":  {"developer/stacks/dev-standards-nextjs", "developer/stacks/dev-standards-react"},
	"nuxtjs":  {"developer/stacks/dev-standards-nuxtjs", "developer/stacks/dev-standards-vuejs"},
	"angular": {"developer/stacks/dev-standards-angular"},

	// Backend
	"express":    {"developer/stacks/dev-standards-express"},
	"nestjs":     {"developer/stacks/dev-standards-nestjs"},
	"django":     {"developer/stacks/dev-standards-django"},
	"fastapi":    {"developer/stacks/dev-standards-fastapi"},
	"laravel":    {"developer/stacks/dev-standards-laravel"},
	"rails":      {"developer/stacks/dev-standards-rails"},
	"springboot": {"developer/stacks/dev-standards-springboot"},

	// ORM / DB, API spec
	"prisma":     {"developer/stacks/dev-standards-prisma"},
	"typeorm":    {"developer/stacks/dev-standards-typeorm"},
	"sqlalchemy": {"developer/stacks/dev-standards-sqlalchemy"},
	"mongodb":    {"developer/stacks/dev-standards-mongodb"},
	"openapi":    {"developer/stacks/dev-standards-openapi"},

	// Testing
	"vitest":     {"developer/stacks/dev-standards-vitest"},
	"jest":       {"developer/stacks/dev-standards-jest"},
	"playwright": {"developer/stacks/dev-standards-playwright"},
	"cypress":    {"developer/stacks/dev-standards-cypress"},

	// Mobile (the Kotlin skill is the Android one)
	"react-native": {"developer/stacks/dev-standards-react-native"},
	"flutter":      {"developer/stacks/dev-standards-flutter"},
	"swift":        {"developer/stacks/dev-standards-swift"},
	"android":      {"developer/stacks/dev-standards-kotlin"},

	// Data / ML
	"pandas":  {"developer/stacks/dev-standards-pandas"},
	"dbt":     {"developer/stacks/dev-standards-dbt"},
	"airflow": {"developer/stacks/dev-standards-airflow"},
	"pyspark": {"developer/stacks/dev-standards-pyspark"},

	// DevOps / CI, platform
	"docker":         {"developer/stacks/dev-standards-docker"},
	"github-actions": {"developer/stacks/dev-standards-github-actions"},
	"gitlab-ci":      {"developer/stacks/dev-standards-gitlab-ci"},
	"terraform":      {"developer/stacks/dev-standards-terraform"},
	"kubernetes":     {"developer/stacks/dev-standards-kubernetes"},
	"helm":           {"developer/stacks/dev-standards-helm"},
	"argocd":         {"developer/stacks/dev-standards-argocd"},
}

// ResolveStackSkills detects the project's tech stack (DetectSignals) and
// returns the skill refs that should be deployed as native skills (Bucket B)
// for it: the union of the refs of its signals, sorted and deduplicated.
func ResolveStackSkills(projectPath string) []string {
	return StackSkillsFor(DetectSignals(projectPath))
}

// StackSkillsFor returns the stack skill refs of signals, sorted and
// deduplicated.
func StackSkillsFor(signals Signals) []string {
	seen := make(map[string]bool)
	var skills []string
	for _, name := range signals.Names() {
		for _, ref := range stackSkillMapping[name] {
			if !seen[ref] {
				seen[ref] = true
				skills = append(skills, ref)
			}
		}
	}
	sort.Strings(skills)
	return skills
}
