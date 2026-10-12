package bricks

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mkProject writes files (relative path → content) in a temp dir.
func mkProject(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	return dir
}

// C3.2: one signal per detection rule.
func TestDetectSignals(t *testing.T) {
	cases := []struct {
		name   string
		files  map[string]string
		want   []string
		absent []string
	}{
		{"tsconfig", map[string]string{"tsconfig.base.json": "{}"}, []string{"typescript"}, nil},
		{"typescript dep", map[string]string{"package.json": `{"devDependencies":{"typescript":"5"}}`}, []string{"typescript", "javascript"}, nil},
		{"plain js", map[string]string{"package.json": `{"dependencies":{"lodash":"4"}}`}, []string{"javascript"}, []string{"typescript"}},
		{"pyproject", map[string]string{"pyproject.toml": "[project]\nname = 'x'\n"}, []string{"python"}, nil},
		{"requirements-dev", map[string]string{"requirements-dev.txt": "pytest\n"}, []string{"python", "pip", "pytest"}, nil},
		{"requirements folder", map[string]string{"requirements/base.txt": "Django==5\n"}, []string{"python", "django"}, nil},
		{"setup.py", map[string]string{"setup.py": ""}, []string{"python"}, nil},
		{"pipfile", map[string]string{"Pipfile": "[packages]\nfastapi = \"*\"\n[dev-packages]\npytest = \"*\"\n"}, []string{"python", "pipenv", "fastapi", "pytest"}, nil},
		{"go.mod", map[string]string{"go.mod": "module x"}, []string{"golang"}, nil},
		{"go.work", map[string]string{"go.work": "go 1.22\n"}, []string{"golang"}, nil},
		{"cargo", map[string]string{"Cargo.toml": "[package]"}, []string{"rust", "cargo"}, nil},
		{"vue", map[string]string{"package.json": `{"dependencies":{"vue":"3"}}`}, []string{"vuejs", "frontend"}, nil},
		{"react", map[string]string{"package.json": `{"dependencies":{"react":"19","react-dom":"19"}}`}, []string{"react", "frontend"}, nil},
		{"next dep", map[string]string{"package.json": `{"dependencies":{"next":"15"}}`}, []string{"nextjs"}, nil},
		{"next config", map[string]string{"next.config.mjs": ""}, []string{"nextjs"}, nil},
		{"nuxt dep", map[string]string{"package.json": `{"dependencies":{"nuxt":"3"}}`}, []string{"nuxtjs"}, nil},
		{"nuxt config", map[string]string{"nuxt.config.ts": ""}, []string{"nuxtjs"}, nil},
		{"angular.json", map[string]string{"angular.json": "{}"}, []string{"angular"}, nil},
		{"angular dep", map[string]string{"package.json": `{"dependencies":{"@angular/core":"18"}}`}, []string{"angular"}, nil},
		{"express", map[string]string{"package.json": `{"dependencies":{"express":"5"}}`}, []string{"express"}, nil},
		{"nest dep", map[string]string{"package.json": `{"dependencies":{"@nestjs/core":"10"}}`}, []string{"nestjs"}, nil},
		{"nest-cli", map[string]string{"nest-cli.json": "{}"}, []string{"nestjs"}, nil},
		{"django pyproject", map[string]string{"pyproject.toml": "[project]\ndependencies = [\"Django>=5.0\", \"psycopg[binary]\"]\n"}, []string{"django"}, nil},
		{"manage.py", map[string]string{"manage.py": ""}, []string{"django", "python"}, nil},
		{"fastapi poetry", map[string]string{"pyproject.toml": "[tool.poetry.dependencies]\npython = \"^3.12\"\nFastAPI = \"^0.110\"\n[tool.poetry.group.dev.dependencies]\npytest = \"*\"\n"}, []string{"fastapi", "pytest"}, nil},
		{"laravel composer", map[string]string{"composer.json": `{"require":{"php":"^8.2","laravel/framework":"^11"}}`}, []string{"laravel", "php", "composer"}, nil},
		{"artisan", map[string]string{"artisan": "#!/usr/bin/env php"}, []string{"laravel"}, nil},
		{"rails gem", map[string]string{"Gemfile": "gem \"rails\", \"~> 7.1\"\ngem 'rspec-rails'\n"}, []string{"rails", "ruby", "rspec"}, nil},
		{"rails app", map[string]string{"config/application.rb": ""}, []string{"rails"}, nil},
		{"spring maven", map[string]string{"pom.xml": "<artifactId>spring-boot-starter-parent</artifactId>"}, []string{"springboot", "java", "maven"}, []string{"android"}},
		{"spring gradle", map[string]string{"build.gradle.kts": "plugins {\n  id(\"org.springframework.boot\") version \"3.3.0\"\n  kotlin(\"jvm\")\n}\n"}, []string{"springboot", "gradle"}, []string{"android"}},
		{"android gradle", map[string]string{"app/build.gradle.kts": "plugins { id(\"com.android.application\"); kotlin(\"android\") }"}, []string{"android", "gradle"}, []string{"springboot"}},
		{"android catalog", map[string]string{"gradle/libs.versions.toml": "[plugins]\nandroid-application = { id = \"com.android.application\" }\n"}, []string{"android"}, nil},
		{"android manifest", map[string]string{"app/src/main/AndroidManifest.xml": "<manifest/>"}, []string{"android"}, nil},
		{"prisma schema", map[string]string{"prisma/schema.prisma": ""}, []string{"prisma"}, nil},
		{"prisma config", map[string]string{"prisma.config.ts": ""}, []string{"prisma"}, nil},
		{"prisma dep", map[string]string{"package.json": `{"devDependencies":{"prisma":"5"}}`}, []string{"prisma"}, nil},
		{"typeorm", map[string]string{"package.json": `{"dependencies":{"typeorm":"0.3"}}`}, []string{"typeorm"}, nil},
		{"sqlalchemy", map[string]string{"requirements.txt": "SQLAlchemy[asyncio]>=2.0 ; python_version>'3.9'\n"}, []string{"sqlalchemy"}, nil},
		{"mongoose", map[string]string{"package.json": `{"dependencies":{"mongoose":"8"}}`}, []string{"mongodb"}, nil},
		{"mongodb driver", map[string]string{"package.json": `{"dependencies":{"mongodb":"6"}}`}, []string{"mongodb"}, nil},
		{"openapi", map[string]string{"api/openapi.yaml": ""}, []string{"openapi"}, nil},
		{"swagger", map[string]string{"swagger.yml": ""}, []string{"openapi"}, nil},
		{"suffixed openapi", map[string]string{"specs/billing.openapi.yml": ""}, []string{"openapi"}, nil},
		{"vitest", map[string]string{"package.json": `{"devDependencies":{"vitest":"2"}}`}, []string{"vitest"}, nil},
		{"jest", map[string]string{"package.json": `{"devDependencies":{"jest":"29"}}`}, []string{"jest"}, nil},
		{"playwright config", map[string]string{"playwright.config.ts": ""}, []string{"playwright"}, nil},
		{"playwright dep", map[string]string{"package.json": `{"devDependencies":{"@playwright/test":"1"}}`}, []string{"playwright"}, nil},
		{"cypress config", map[string]string{"cypress.config.js": ""}, []string{"cypress"}, nil},
		{"cypress dep", map[string]string{"package.json": `{"devDependencies":{"cypress":"13"}}`}, []string{"cypress"}, nil},
		{"expo", map[string]string{"package.json": `{"dependencies":{"expo":"51","react":"18"}}`}, []string{"react-native"}, []string{"react", "frontend"}},
		{"flutter", map[string]string{"pubspec.yaml": "dependencies:\n  flutter:\n    sdk: flutter\n"}, []string{"flutter", "dart"}, nil},
		{"dart only", map[string]string{"pubspec.yaml": "dependencies:\n  http: ^1.0\n"}, []string{"dart"}, []string{"flutter"}},
		{"swift package", map[string]string{"Package.swift": ""}, []string{"swift"}, nil},
		{"xcodeproj", map[string]string{"App.xcodeproj/project.pbxproj": ""}, []string{"swift"}, nil},
		{"pandas", map[string]string{"requirements.txt": "pandas>=2\n"}, []string{"pandas"}, nil},
		{"dbt", map[string]string{"dbt_project.yml": "name: x"}, []string{"dbt"}, nil},
		{"airflow dep", map[string]string{"requirements.txt": "apache_airflow==2.9\n"}, []string{"airflow"}, nil},
		{"airflow dags", map[string]string{"dags/etl.py": "from airflow import DAG\n"}, []string{"airflow"}, nil},
		{"pyspark", map[string]string{"pyproject.toml": "[project]\ndependencies = [\"pyspark==3.5\"]\n"}, []string{"pyspark"}, nil},
		{"dockerfile", map[string]string{"Dockerfile.dev": ""}, []string{"docker"}, nil},
		{"compose", map[string]string{"compose.yaml": ""}, []string{"docker"}, nil},
		{"docker-compose", map[string]string{"docker-compose.override.yml": ""}, []string{"docker"}, nil},
		{"github actions", map[string]string{".github/workflows/ci.yml": "on: push"}, []string{"github-actions"}, nil},
		{"empty workflows dir", map[string]string{".github/workflows/README.md": ""}, nil, []string{"github-actions"}},
		{"gitlab", map[string]string{".gitlab-ci.yml": ""}, []string{"gitlab-ci"}, nil},
		{"terraform", map[string]string{"infra/main.tf": ""}, []string{"terraform"}, nil},
		{"opentofu", map[string]string{"main.tofu": ""}, []string{"terraform"}, nil},
		{"kustomize", map[string]string{"overlays/prod/kustomization.yaml": ""}, []string{"kubernetes"}, nil},
		{"k8s manifest", map[string]string{"k8s/api.yaml": "apiVersion: apps/v1\nkind: Deployment\n"}, []string{"kubernetes"}, nil},
		{"yaml outside manifest dirs", map[string]string{"config/app.yaml": "apiVersion: apps/v1\nkind: Deployment\n"}, nil, []string{"kubernetes"}},
		{"helm", map[string]string{"charts/api/Chart.yaml": "apiVersion: v2"}, []string{"helm"}, nil},
		{"argocd", map[string]string{"deploy/app.yaml": "apiVersion: argoproj.io/v1alpha1\nkind: Application\n"}, []string{"argocd"}, []string{"kubernetes"}},
		{"lockfiles", map[string]string{"pnpm-lock.yaml": "", "poetry.lock": "", "Makefile": ""}, []string{"pnpm", "poetry", "make"}, nil},
		// False positives of the former substring search.
		{"eslint-config-next", map[string]string{"package.json": `{"devDependencies":{"eslint-config-next":"15","next-themes":"1","react-i18next":"1"}}`}, nil, []string{"nextjs"}},
		{"jest-dom", map[string]string{"package.json": `{"devDependencies":{"@testing-library/jest-dom":"6"}}`}, nil, []string{"jest"}},
		{"prettier-plugin-svelte", map[string]string{"package.json": `{"devDependencies":{"prettier-plugin-svelte":"3"}}`}, nil, []string{"svelte", "frontend"}},
		{"django in a comment", map[string]string{"requirements.txt": "# not django\ndjango-environ\n"}, nil, []string{"django"}},
		{"skipped dirs", map[string]string{"node_modules/next/package.json": `{"dependencies":{"next":"1"}}`, "vendor/x/go.mod": "", ".venv/lib/requirements.txt": "django"}, nil, []string{"nextjs", "golang", "python"}},
		{"depth limit", map[string]string{"a/b/c/d/e/go.mod": "module x"}, nil, []string{"golang"}},
		{"depth bound", map[string]string{"a/b/c/d/go.mod": "module x"}, []string{"golang"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sig := DetectSignals(mkProject(t, c.files))
			for _, w := range c.want {
				assert.True(t, sig.Has(w), "missing signal %s (got %v)", w, sig.Names())
			}
			for _, a := range c.absent {
				assert.False(t, sig.Has(a), "unexpected signal %s (got %v)", a, sig.Names())
			}
		})
	}
}

func TestDetectSignals_Monorepo(t *testing.T) {
	dir := mkProject(t, map[string]string{
		"apps/web/package.json":   `{"dependencies":{"next":"15","react":"19","react-dom":"19"},"devDependencies":{"typescript":"5"}}`,
		"services/api/go.mod":     "module api",
		"services/api/Dockerfile": "FROM golang",
	})
	sig := DetectSignals(dir)
	for _, w := range []string{"nextjs", "react", "typescript", "golang", "docker", "frontend"} {
		assert.True(t, sig.Has(w), w)
	}
	assert.Equal(t, []string{"services/api"}, sig["golang"])
	assert.Equal(t, []string{"apps/web"}, sig["nextjs"])
}

// Workspaces deeper than the walk are followed.
func TestDetectSignals_Workspaces(t *testing.T) {
	deep := "a/b/c/d/e"
	dir := mkProject(t, map[string]string{
		"package.json":               `{"workspaces":["a/b/c/d/*"]}`,
		deep + "/package.json":       `{"dependencies":{"vue":"3"}}`,
		"pnpm-workspace.yaml":        "packages:\n  - 'x/y/z/w/*'\n",
		"x/y/z/w/v/package.json":     `{"dependencies":{"express":"5"}}`,
		"go.work":                    "go 1.22\n\nuse (\n\t./m/n/o/p/q\n)\n",
		"m/n/o/p/q/requirements.txt": "fastapi\n",
	})
	sig := DetectSignals(dir)
	assert.Equal(t, []string{deep}, sig["vuejs"])
	assert.True(t, sig.Has("express"))
	assert.True(t, sig.Has("fastapi"))
}

func TestDetectSignals_EntryLimit(t *testing.T) {
	defer func(n int) { signalMaxEntries = n }(signalMaxEntries)
	signalMaxEntries = 50
	dir := t.TempDir()
	for i := 0; i < signalMaxEntries+10; i++ {
		require.NoError(t, os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%06d", i)), nil, 0o644))
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "zzz.tf"), nil, 0o644))
	assert.False(t, DetectSignals(dir).Has("terraform"), "entries past the limit are not scanned")
}

func TestDetectSignals_UnknownPath(t *testing.T) {
	assert.Empty(t, DetectSignals(""))
	assert.Empty(t, DetectSignals(filepath.Join(t.TempDir(), "missing")))
}
