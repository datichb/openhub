package bricks

import (
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// Signals is the set of stack signals found in a project by DetectSignals:
// each signal name maps to the folders (relative to the project, slash
// separated, "." for the root) where it was seen, sorted and capped.
//
// Signal names:
//   - stacks, named like their skill (developer/stacks/dev-standards-<name>):
//     typescript, python, golang, rust, vuejs, react, nextjs, nuxtjs,
//     angular, express, nestjs, django, fastapi, laravel, rails, springboot,
//     prisma, typeorm, sqlalchemy, mongodb, openapi, vitest, jest,
//     playwright, cypress, react-native, flutter, swift, pandas, dbt,
//     airflow, pyspark, docker, github-actions, gitlab-ci, terraform,
//     kubernetes, helm, argocd; android (Gradle Android project: the
//     dev-standards-kotlin skill);
//   - other languages and frameworks: javascript (any package.json), java
//     (Maven/Gradle), ruby, php, dart, svelte, fastify, flask;
//   - tools, for the project commands: npm, pnpm, yarn, bun (lockfiles),
//     poetry, pipenv, uv, pip, bundler, composer, cargo, gradle, maven,
//     make (Makefile), mocha, pytest, rspec, jenkins (Jenkinsfile);
//   - frontend: the project has a web frontend (see HasFrontend).
type Signals map[string][]string

// Has reports whether the signal was found.
func (s Signals) Has(name string) bool { return len(s[name]) > 0 }

// At reports whether the signal was found in the folder dir ("." for the root).
func (s Signals) At(name, dir string) bool {
	for _, d := range s[name] {
		if d == dir {
			return true
		}
	}
	return false
}

// Names returns the signals found, sorted.
func (s Signals) Names() []string {
	names := make([]string, 0, len(s))
	for n, dirs := range s {
		if len(dirs) > 0 {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names
}

func (s Signals) add(name, dir string) {
	dirs := s[name]
	i := sort.SearchStrings(dirs, dir)
	if i < len(dirs) && dirs[i] == dir {
		return
	}
	if len(dirs) >= signalMaxDirs {
		return
	}
	dirs = append(dirs, "")
	copy(dirs[i+1:], dirs[i:])
	dirs[i] = dir
	s[name] = dirs
}

const (
	// signalMaxDirs bounds the folders kept per signal.
	signalMaxDirs = 16
	// signalMaxReads bounds the manifests read (package.json, pyproject…).
	signalMaxReads = 500
	// signalMaxSniffs bounds the YAML manifests and DAG files sniffed.
	signalMaxSniffs = 40
	// signalMaxFileSize bounds the bytes read from one file.
	signalMaxFileSize = 512 * 1024
	// workspaceMaxDirs bounds the workspace folders scanned after the walk
	// (folders deeper than the walk or left out by its entry limit).
	workspaceMaxDirs = 64
	// workspaceMaxEntries bounds the entries read in one workspace folder.
	workspaceMaxEntries = 200
)

// signalMaxEntries bounds the entries visited by the walk (a variable for
// the tests).
var signalMaxEntries = frontendMaxFiles

// manifestDirs are the folders whose YAML files are sniffed for Kubernetes
// and Argo CD manifests.
var manifestDirs = map[string]bool{
	"k8s": true, "kubernetes": true, "deploy": true, "deployment": true, "deployments": true,
	"manifests": true, "argocd": true, "gitops": true,
}

var (
	pep503Sep     = regexp.MustCompile(`[-_.]+`)
	pyReqName     = regexp.MustCompile(`^\s*([A-Za-z0-9][A-Za-z0-9._-]*)`)
	gemLine       = regexp.MustCompile(`(?m)^\s*gem\s+['"]([^'"]+)['"]`)
	flutterKey    = regexp.MustCompile(`(?m)^\s*flutter:`)
	k8sAPIVersion = regexp.MustCompile(`(?m)^apiVersion:\s*\S`)
	k8sKind       = regexp.MustCompile(`(?m)^kind:\s*(Deployment|Service|StatefulSet|DaemonSet|Ingress|ConfigMap|Secret|CronJob|Job|Namespace|Pod|PersistentVolumeClaim|NetworkPolicy|ServiceAccount|Role|RoleBinding|ClusterRole|ClusterRoleBinding|HorizontalPodAutoscaler)\s*$`)
	argoAPI       = regexp.MustCompile(`(?m)^apiVersion:\s*argoproj\.io/`)
	composeFile   = regexp.MustCompile(`^(docker-)?compose([.-].*)?\.ya?ml$`)
)

// npmSignals maps npm packages (dependencies and devDependencies, exact
// names) to signals. react is handled apart (not for a React Native app).
var npmSignals = map[string][]string{
	"typescript":       {"typescript"},
	"vue":              {"vuejs"},
	"next":             {"nextjs"},
	"nuxt":             {"nuxtjs"},
	"@angular/core":    {"angular"},
	"express":          {"express"},
	"@nestjs/core":     {"nestjs"},
	"prisma":           {"prisma"},
	"@prisma/client":   {"prisma"},
	"typeorm":          {"typeorm"},
	"mongoose":         {"mongodb"},
	"mongodb":          {"mongodb"},
	"vitest":           {"vitest"},
	"jest":             {"jest"},
	"@playwright/test": {"playwright"},
	"cypress":          {"cypress"},
	"react-native":     {"react-native"},
	"expo":             {"react-native"},
	"svelte":           {"svelte"},
	"@sveltejs/kit":    {"svelte"},
	"fastify":          {"fastify"},
	"mocha":            {"mocha"},
}

// pythonSignals maps Python distributions (PEP 503 normalised) to signals.
var pythonSignals = map[string]string{
	"django": "django", "fastapi": "fastapi", "flask": "flask", "sqlalchemy": "sqlalchemy",
	"pandas": "pandas", "apache-airflow": "airflow", "pyspark": "pyspark", "pytest": "pytest",
}

// configPrefixes are config files (<prefix>.<ext>) marking a stack.
var configPrefixes = map[string]string{
	"next.config": "nextjs", "nuxt.config": "nuxtjs", "playwright.config": "playwright",
	"cypress.config": "cypress",
}

// exactFiles are file names marking a stack (and its language).
var exactFiles = map[string][]string{
	"go.mod":              {"golang"},
	"go.work":             {"golang"},
	"Cargo.toml":          {"rust", "cargo"},
	"setup.py":            {"python"},
	"manage.py":           {"python", "django"},
	"artisan":             {"php", "laravel"},
	"angular.json":        {"angular"},
	"nest-cli.json":       {"nestjs"},
	"schema.prisma":       {"prisma"},
	"prisma.config.ts":    {"prisma"},
	"cypress.json":        {"cypress"},
	"AndroidManifest.xml": {"android"},
	"Package.swift":       {"swift"},
	"dbt_project.yml":     {"dbt"},
	".gitlab-ci.yml":      {"gitlab-ci"},
	"Jenkinsfile":         {"jenkins"},
	"Containerfile":       {"docker"},
	"Chart.yaml":          {"helm"},
	"kustomization.yaml":  {"kubernetes"},
	"kustomization.yml":   {"kubernetes"},
	"Makefile":            {"make"},
	"package-lock.json":   {"npm"},
	"yarn.lock":           {"yarn"},
	"pnpm-lock.yaml":      {"pnpm"},
	"bun.lockb":           {"bun"},
	"bun.lock":            {"bun"},
	"bunfig.toml":         {"bun"},
	"poetry.lock":         {"poetry"},
	"uv.lock":             {"uv"},
}

// DetectSignals scans a project once and returns its stack signals (see
// Signals). The walk is bounded: folders up to frontendMaxDepth levels,
// frontendMaxFiles entries, skippedDirs never entered; manifests are read
// (signalMaxReads files of at most signalMaxFileSize bytes) for their exact
// dependency names. Monorepos are followed: nested manifests are found by the
// walk, and the workspace folders declared by package.json `workspaces`,
// pnpm-workspace.yaml or go.work are scanned even beyond the walk.
// An empty, missing or unreadable path has no signal.
func DetectSignals(projectPath string) Signals {
	sc := &signalScan{root: projectPath, sig: Signals{}, walked: map[string]bool{}}
	if projectPath == "" {
		return sc.sig
	}
	if st, err := os.Stat(projectPath); err != nil || !st.IsDir() {
		return sc.sig
	}
	seen := 0
	_ = filepath.WalkDir(projectPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // unreadable entry: skipped
		}
		seen++
		if seen > signalMaxEntries {
			return fs.SkipAll
		}
		rel, _ := filepath.Rel(projectPath, p)
		if d.IsDir() {
			if p == projectPath {
				sc.walked["."] = true
				return nil
			}
			depth := len(strings.Split(rel, string(filepath.Separator)))
			if skippedDirs[d.Name()] || depth > frontendMaxDepth {
				return fs.SkipDir
			}
			if strings.HasSuffix(d.Name(), ".xcodeproj") {
				sc.sig.add("swift", filepath.ToSlash(filepath.Dir(rel)))
				return fs.SkipDir
			}
			sc.walked[filepath.ToSlash(rel)] = true
			return nil
		}
		sc.file(p, filepath.ToSlash(rel))
		return nil
	})
	sc.followWorkspaces()
	if sc.webFrontend || (sc.jsxSources && !sc.nativeApp) {
		sc.sig.add("frontend", ".")
	}
	return sc.sig
}

type signalScan struct {
	root       string
	sig        Signals
	walked     map[string]bool
	reads      int
	sniffs     int
	workspaces []string // slash globs relative to the root

	webFrontend bool // web frontend dependency, sources or HTML
	jsxSources  bool // .tsx/.jsx sources (web or React Native)
	nativeApp   bool // a React Native package without react-dom
}

// file records the signals of one file; rel is slash separated.
func (sc *signalScan) file(p, rel string) {
	name := path.Base(rel)
	dir := path.Dir(rel)
	ext := path.Ext(name)
	for _, s := range exactFiles[name] {
		sc.sig.add(s, dir)
	}
	if prefix := strings.TrimSuffix(name, ext); configPrefixes[prefix] != "" && ext != "" {
		sc.sig.add(configPrefixes[prefix], dir)
	}
	isYAML := ext == ".yml" || ext == ".yaml"
	switch {
	case name == "package.json":
		sc.sig.add("javascript", dir)
		sc.packageJSON(p, dir)
	case strings.HasPrefix(name, "tsconfig") && ext == ".json":
		sc.sig.add("typescript", dir)
	case name == "pnpm-workspace.yaml":
		sc.pnpmWorkspace(p, dir)
	case name == "go.work":
		sc.goWork(p, dir)
	case name == "pyproject.toml":
		sc.sig.add("python", dir)
		sc.pyproject(p, dir)
	case name == "Pipfile":
		sc.sig.add("python", dir)
		sc.sig.add("pipenv", dir)
		sc.pipfile(p, dir)
	case ext == ".txt" && (strings.HasPrefix(name, "requirements") || path.Base(dir) == "requirements"):
		sc.sig.add("python", dir)
		sc.sig.add("pip", dir)
		sc.requirements(p, dir)
	case name == "Gemfile":
		sc.sig.add("ruby", dir)
		sc.sig.add("bundler", dir)
		sc.gemfile(p, dir)
	case name == "application.rb" && path.Base(dir) == "config":
		sc.sig.add("ruby", path.Dir(dir))
		sc.sig.add("rails", path.Dir(dir))
	case name == "composer.json":
		sc.sig.add("php", dir)
		sc.sig.add("composer", dir)
		sc.composer(p, dir)
	case name == "pom.xml":
		sc.sig.add("java", dir)
		sc.sig.add("maven", dir)
		sc.jvmBuild(p, dir)
	case name == "build.gradle" || name == "build.gradle.kts":
		sc.sig.add("gradle", dir)
		sc.jvmBuild(p, dir)
	case name == "settings.gradle" || name == "settings.gradle.kts" || name == "libs.versions.toml":
		sc.jvmBuild(p, dir)
	case name == "pubspec.yaml":
		sc.sig.add("dart", dir)
		if flutterKey.MatchString(sc.read(p)) {
			sc.sig.add("flutter", dir)
		}
	case isOpenAPI(name):
		sc.sig.add("openapi", dir)
	case strings.HasPrefix(name, "Dockerfile") || strings.HasSuffix(strings.ToLower(name), ".dockerfile") ||
		composeFile.MatchString(name):
		sc.sig.add("docker", dir)
	case isYAML && (dir == ".github/workflows" || strings.HasSuffix(dir, "/.github/workflows")):
		sc.sig.add("github-actions", dir)
	case ext == ".tf" || ext == ".tofu":
		sc.sig.add("terraform", dir)
	case isYAML && inManifestDir(dir):
		sc.manifest(p, dir)
	case ext == ".py" && path.Base(dir) == "dags":
		if sc.sniffs < signalMaxSniffs {
			sc.sniffs++
			if src := sc.readN(p, 64*1024); strings.Contains(src, "from airflow") || strings.Contains(src, "import airflow") {
				sc.sig.add("airflow", path.Dir(dir))
			}
		}
	case frontendExts[ext]:
		if ext == ".tsx" || ext == ".jsx" {
			sc.jsxSources = true
		} else {
			sc.webFrontend = true
		}
	case ext == ".html" && (rel == "index.html" || inHTMLDir(filepath.FromSlash(rel))):
		sc.webFrontend = true
	}
}

func isOpenAPI(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range []string{".yaml", ".yml", ".json"} {
		if !strings.HasSuffix(lower, ext) {
			continue
		}
		base := strings.TrimSuffix(lower, ext)
		return base == "openapi" || base == "swagger" || strings.HasSuffix(base, ".openapi") || strings.HasSuffix(base, ".swagger")
	}
	return false
}

func inManifestDir(dir string) bool {
	for _, part := range strings.Split(dir, "/") {
		if manifestDirs[part] {
			return true
		}
	}
	return false
}

// read returns the start of a manifest ("" past the read budget).
func (sc *signalScan) read(p string) string {
	if sc.reads >= signalMaxReads {
		return ""
	}
	sc.reads++
	return sc.readN(p, signalMaxFileSize)
}

func (sc *signalScan) readN(p string, n int64) string {
	f, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, n))
	if err != nil {
		return ""
	}
	return string(data)
}

func (sc *signalScan) packageJSON(p, dir string) {
	var pkg struct {
		Dependencies    map[string]any  `json:"dependencies"`
		DevDependencies map[string]any  `json:"devDependencies"`
		Workspaces      json.RawMessage `json:"workspaces"`
	}
	if json.Unmarshal([]byte(sc.read(p)), &pkg) != nil {
		return
	}
	deps := map[string]bool{}
	for name := range pkg.Dependencies {
		deps[name] = true
	}
	for name := range pkg.DevDependencies {
		deps[name] = true
	}
	for name := range deps {
		for _, s := range npmSignals[name] {
			sc.sig.add(s, dir)
		}
	}
	native := (deps["react-native"] || deps["expo"]) && !deps["react-dom"]
	if native {
		sc.nativeApp = true
	} else if deps["react"] {
		sc.sig.add("react", dir)
		sc.webFrontend = true
	}
	for _, dep := range frontendDeps {
		if deps[dep] {
			sc.webFrontend = true
		}
	}
	var globs []string
	if json.Unmarshal(pkg.Workspaces, &globs) != nil {
		var obj struct {
			Packages []string `json:"packages"`
		}
		_ = json.Unmarshal(pkg.Workspaces, &obj)
		globs = obj.Packages
	}
	sc.addWorkspaces(dir, globs)
}

func (sc *signalScan) pnpmWorkspace(p, dir string) {
	var ws struct {
		Packages []string `yaml:"packages"`
	}
	if yaml.Unmarshal([]byte(sc.read(p)), &ws) == nil {
		sc.addWorkspaces(dir, ws.Packages)
	}
}

func (sc *signalScan) goWork(p, dir string) {
	var uses []string
	block := false
	for _, line := range strings.Split(sc.read(p), "\n") {
		line = strings.TrimSpace(strings.SplitN(line, "//", 2)[0])
		switch {
		case block && line == ")":
			block = false
		case block && line != "":
			uses = append(uses, line)
		case line == "use (":
			block = true
		case strings.HasPrefix(line, "use "):
			uses = append(uses, strings.TrimSpace(strings.TrimPrefix(line, "use ")))
		}
	}
	sc.addWorkspaces(dir, uses)
}

func (sc *signalScan) addWorkspaces(dir string, globs []string) {
	for _, g := range globs {
		g = strings.Trim(strings.TrimSpace(g), `"'`)
		if g == "" || strings.HasPrefix(g, "!") || path.IsAbs(g) {
			continue
		}
		g = strings.TrimSuffix(strings.TrimSuffix(g, "/**"), "/")
		if g = path.Join(dir, g); strings.HasPrefix(g, "..") {
			continue
		}
		sc.workspaces = append(sc.workspaces, g)
	}
}

// followWorkspaces scans the direct files of the workspace folders the walk
// did not enter (too deep, or past its entry limit).
func (sc *signalScan) followWorkspaces() {
	scanned := 0
	for _, g := range sc.workspaces {
		matches, _ := filepath.Glob(filepath.Join(sc.root, filepath.FromSlash(g)))
		for _, m := range matches {
			rel, err := filepath.Rel(sc.root, m)
			if err != nil || strings.HasPrefix(rel, "..") {
				continue
			}
			rel = filepath.ToSlash(rel)
			if sc.walked[rel] || scanned >= workspaceMaxDirs {
				continue
			}
			if st, err := os.Stat(m); err != nil || !st.IsDir() {
				continue
			}
			sc.walked[rel] = true
			scanned++
			entries, _ := os.ReadDir(m)
			for i, e := range entries {
				if i >= workspaceMaxEntries {
					break
				}
				if !e.IsDir() {
					sc.file(filepath.Join(m, e.Name()), path.Join(rel, e.Name()))
				}
			}
		}
	}
}

// pyName returns the PEP 503 normalised name of a requirement ("" if none).
func pyName(req string) string {
	m := pyReqName.FindStringSubmatch(req)
	if m == nil {
		return ""
	}
	return pep503Sep.ReplaceAllString(strings.ToLower(m[1]), "-")
}

func (sc *signalScan) addPython(dir string, names []string) {
	for _, n := range names {
		if s := pythonSignals[pyName(n)]; s != "" {
			sc.sig.add(s, dir)
		}
	}
}

func (sc *signalScan) pyproject(p, dir string) {
	var doc struct {
		Project struct {
			Dependencies         []string            `toml:"dependencies"`
			OptionalDependencies map[string][]string `toml:"optional-dependencies"`
		} `toml:"project"`
		DependencyGroups map[string][]any `toml:"dependency-groups"`
		Tool             struct {
			Poetry struct {
				Dependencies    map[string]any `toml:"dependencies"`
				DevDependencies map[string]any `toml:"dev-dependencies"`
				Group           map[string]struct {
					Dependencies map[string]any `toml:"dependencies"`
				} `toml:"group"`
			} `toml:"poetry"`
		} `toml:"tool"`
	}
	if toml.Unmarshal([]byte(sc.read(p)), &doc) != nil {
		return
	}
	names := append([]string{}, doc.Project.Dependencies...)
	for _, reqs := range doc.Project.OptionalDependencies {
		names = append(names, reqs...)
	}
	for _, reqs := range doc.DependencyGroups {
		for _, r := range reqs {
			if s, ok := r.(string); ok {
				names = append(names, s)
			}
		}
	}
	poetry := doc.Tool.Poetry
	tables := []map[string]any{poetry.Dependencies, poetry.DevDependencies}
	for _, g := range poetry.Group {
		tables = append(tables, g.Dependencies)
	}
	for _, t := range tables {
		for n := range t {
			names = append(names, n)
		}
	}
	sc.addPython(dir, names)
}

func (sc *signalScan) pipfile(p, dir string) {
	var doc struct {
		Packages    map[string]any `toml:"packages"`
		DevPackages map[string]any `toml:"dev-packages"`
	}
	if toml.Unmarshal([]byte(sc.read(p)), &doc) != nil {
		return
	}
	var names []string
	for n := range doc.Packages {
		names = append(names, n)
	}
	for n := range doc.DevPackages {
		names = append(names, n)
	}
	sc.addPython(dir, names)
}

func (sc *signalScan) requirements(p, dir string) {
	var names []string
	for _, line := range strings.Split(sc.read(p), "\n") {
		line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		if line == "" || strings.HasPrefix(line, "-") || strings.Contains(line, "://") {
			continue
		}
		names = append(names, line)
	}
	sc.addPython(dir, names)
}

func (sc *signalScan) gemfile(p, dir string) {
	for _, m := range gemLine.FindAllStringSubmatch(sc.read(p), -1) {
		switch m[1] {
		case "rails":
			sc.sig.add("rails", dir)
		case "rspec", "rspec-rails":
			sc.sig.add("rspec", dir)
		}
	}
}

func (sc *signalScan) composer(p, dir string) {
	var doc struct {
		Require map[string]any `json:"require"`
	}
	if json.Unmarshal([]byte(sc.read(p)), &doc) != nil {
		return
	}
	if _, ok := doc.Require["laravel/framework"]; ok {
		sc.sig.add("laravel", dir)
	}
}

// jvmBuild reads a Maven/Gradle build (or version catalog) for Spring Boot
// and the Android plugins; the language is java unless it is Android.
func (sc *signalScan) jvmBuild(p, dir string) {
	if path.Base(dir) == "gradle" && path.Base(p) == "libs.versions.toml" {
		dir = path.Dir(dir)
	}
	text := sc.read(p)
	if strings.Contains(text, "spring-boot") || strings.Contains(text, "org.springframework.boot") {
		sc.sig.add("springboot", dir)
	}
	for _, marker := range []string{"com.android.application", "com.android.library", `kotlin("android")`, "org.jetbrains.kotlin.android"} {
		if strings.Contains(text, marker) {
			sc.sig.add("android", dir)
			return
		}
	}
	if base := path.Base(p); base != "libs.versions.toml" && !strings.HasPrefix(base, "settings.gradle") {
		sc.sig.add("java", dir)
	}
}

// manifest sniffs a YAML file of a deployment folder for Kubernetes and
// Argo CD resources.
func (sc *signalScan) manifest(p, dir string) {
	if sc.sniffs >= signalMaxSniffs {
		return
	}
	sc.sniffs++
	text := sc.readN(p, 64*1024)
	if argoAPI.MatchString(text) {
		sc.sig.add("argocd", dir)
	}
	if k8sAPIVersion.MatchString(text) && k8sKind.MatchString(text) {
		sc.sig.add("kubernetes", dir)
	}
}
