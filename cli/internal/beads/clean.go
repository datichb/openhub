package beads

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Zero impact (A5): what `bd init` writes in a project besides .beads/ —
// agent instruction files (AGENTS.md, CLAUDE.md), the folders of other AI
// tools (.claude/, .codex/, .cursor/, .agents/), a .gitignore block — is
// taken back out by CleanImpact. Only what bd wrote is removed: the blocks
// it manages, its hook entries, its own files; a file of the user keeps
// the rest of its content.
//
// Files tracked by git whose committed version already has bd's content are
// left alone unless CleanOptions.Tracked is set (`oh doctor --fix`): an
// automatic clean-up only reverts changes that were never committed.

// CleanOptions tunes CleanImpact.
type CleanOptions struct {
	// DryRun lists the actions without changing anything.
	DryRun bool
	// Tracked also cleans files whose committed version has bd's content
	// (the change then shows in git status).
	Tracked bool
}

// CleanAction is one change of CleanImpact.
type CleanAction struct {
	Path string // relative to the project
	Kind string // removed | edited | exclude | skipped (committed content, see Tracked)
}

// CleanReport lists the actions of CleanImpact.
type CleanReport struct {
	Actions []CleanAction
}

// Changed reports whether something was (or would be) changed.
func (r CleanReport) Changed() bool {
	for _, a := range r.Actions {
		if a.Kind != CleanSkipped {
			return true
		}
	}
	return false
}

// Skipped returns the files left alone because their bd content is committed.
func (r CleanReport) Skipped() []string {
	var out []string
	for _, a := range r.Actions {
		if a.Kind == CleanSkipped {
			out = append(out, a.Path)
		}
	}
	return out
}

// Kinds of CleanAction.
const (
	CleanRemoved = "removed"
	CleanEdited  = "edited"
	CleanExclude = "exclude"
	CleanSkipped = "skipped"
	// CleanUncommit: the commit of `bd init`, not pushed, was undone (the
	// files stay in the working tree); Path is "commit <hash>".
	CleanUncommit = "uncommit"
)

// bdInitCommitSubject is the subject of the commit `bd init` makes.
const bdInitCommitSubject = "bd init: initialize beads issue tracking"

// beadsBlockRe matches a block managed by bd in a markdown file
// (<!-- BEGIN BEADS INTEGRATION … --> … <!-- END BEADS INTEGRATION -->,
// <!-- BEGIN BEADS CODEX SETUP … --> …).
var beadsBlockRe = regexp.MustCompile(`(?s)\n*<!-- BEGIN BEADS [^>]*-->.*?<!-- END BEADS [^>]*-->[^\S\n]*\n?`)

// bdTemplateLines are the lines of the CLAUDE.md template of bd that are not
// project content (headings and placeholders).
var bdTemplateLines = map[string]bool{
	"# Project Instructions for AI Agents":                                                      true,
	"This file provides instructions and context for AI coding agents working on this project.": true,
	"## Build & Test":           true,
	"## Architecture Overview":  true,
	"## Conventions & Patterns": true,
	"```bash":                   true,
	"```":                       true,
	"# Example:":                true,
	"# npm install":             true,
	"# npm test":                true,
	"_Add your build and test commands here_":             true,
	"_Add a brief overview of your project architecture_": true,
	"_Add your project-specific conventions here_":        true,
}

// isBdTemplate reports a markdown file that is only bd's template once its
// managed blocks are removed.
func isBdTemplate(rest string) bool {
	trimmed := strings.TrimSpace(rest)
	if trimmed == "" {
		return true
	}
	// AGENTS.md of bd: its own introduction and shell advice.
	if strings.HasPrefix(trimmed, "# Agent Instructions") && strings.Contains(trimmed, "`bd prime`") &&
		strings.Contains(trimmed, "## Non-Interactive Shell Commands") {
		return true
	}
	if !strings.HasPrefix(trimmed, "# Project Instructions for AI Agents") {
		return false
	}
	for _, l := range strings.Split(trimmed, "\n") {
		if l = strings.TrimSpace(l); l != "" && !bdTemplateLines[l] {
			return false
		}
	}
	return true
}

type cleaner struct {
	root    string
	opts    CleanOptions
	report  CleanReport
	tracked map[string]bool // relative paths tracked by git
	git     bool
}

// CleanImpact removes from a project what `bd init` wrote outside .beads/
// and makes sure .beads/ is in .git/info/exclude. It never touches .beads/
// nor the git hooks set by bd (core.hooksPath), which oh supports.
func CleanImpact(projectPath string, opts CleanOptions) (CleanReport, error) {
	c := &cleaner{root: projectPath, opts: opts}
	c.undoInitCommit()
	c.loadTracked()
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		c.markdown(name)
	}
	c.markdown(filepath.Join(".cursor", "rules", "beads.mdc"))
	for _, name := range []string{filepath.Join(".claude", "settings.json"), filepath.Join(".codex", "hooks.json"), filepath.Join(".cursor", "hooks.json")} {
		c.hooksJSON(name)
	}
	c.codexConfig()
	c.agentsSkill()
	if err := c.gitignore(); err != nil {
		return c.report, err
	}
	c.exclude()
	if !opts.DryRun {
		for _, d := range []string{filepath.Join(".cursor", "rules"), ".cursor", ".codex", ".claude", filepath.Join(".agents", "skills"), ".agents"} {
			removeIfEmpty(filepath.Join(projectPath, d))
		}
	}
	return c.report, nil
}

func (c *cleaner) gitOut(args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", c.root}, args...)...).Output()
	return strings.TrimSpace(string(out)), err
}

// undoInitCommit undoes the commit `bd init` makes (HEAD, not pushed): a
// mixed reset keeps every file in the working tree. A pushed commit, or one
// followed by other commits, is never touched (reported as skipped).
func (c *cleaner) undoInitCommit() {
	subject, err := c.gitOut("log", "-1", "--format=%s")
	if err != nil {
		return
	}
	hash, _ := c.gitOut("rev-parse", "--short", "HEAD")
	if subject != bdInitCommitSubject {
		// An older commit of bd init: left alone, the files are cleaned as
		// committed content (oh doctor --fix).
		if old, err := c.gitOut("log", "--format=%h", "--fixed-strings", "--grep="+bdInitCommitSubject, "-n", "1"); err == nil && old != "" {
			c.report.Actions = append(c.report.Actions, CleanAction{Path: "commit " + old, Kind: CleanSkipped})
		}
		return
	}
	if remote, _ := c.gitOut("branch", "-r", "--contains", "HEAD"); remote != "" {
		c.report.Actions = append(c.report.Actions, CleanAction{Path: "commit " + hash, Kind: CleanSkipped})
		return
	}
	c.report.Actions = append(c.report.Actions, CleanAction{Path: "commit " + hash, Kind: CleanUncommit})
	if c.opts.DryRun {
		return
	}
	if _, err := c.gitOut("rev-parse", "--verify", "-q", "HEAD~1"); err == nil {
		_, _ = c.gitOut("reset", "-q", "HEAD~1")
		return
	}
	// First commit of the repository: back to an unborn branch.
	if _, err := c.gitOut("update-ref", "-d", "HEAD"); err == nil {
		_, _ = c.gitOut("rm", "-r", "-q", "--cached", ".")
	}
}

func (c *cleaner) loadTracked() {
	if _, err := os.Stat(filepath.Join(c.root, ".git")); err == nil {
		c.git = true
	}
	c.tracked = map[string]bool{}
	out, err := exec.Command("git", "-C", c.root, "ls-files", "-z").Output()
	if err != nil {
		return
	}
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			c.tracked[filepath.FromSlash(p)] = true
		}
	}
}

// committed reports a tracked file whose committed version already has
// marker (bd's content was committed: changing it is a change of the
// repository).
func (c *cleaner) committed(rel, marker string) bool {
	if !c.tracked[rel] {
		return false
	}
	out, err := exec.Command("git", "-C", c.root, "show", "HEAD:"+filepath.ToSlash(rel)).Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), marker)
}

// allowed tells whether rel may be changed; a refused file is reported.
func (c *cleaner) allowed(rel, marker string) bool {
	if c.opts.Tracked || !c.committed(rel, marker) {
		return true
	}
	c.report.Actions = append(c.report.Actions, CleanAction{Path: rel, Kind: CleanSkipped})
	return false
}

func (c *cleaner) remove(rel string) {
	c.report.Actions = append(c.report.Actions, CleanAction{Path: rel, Kind: CleanRemoved})
	if !c.opts.DryRun {
		_ = os.RemoveAll(filepath.Join(c.root, rel))
	}
}

func (c *cleaner) write(rel string, data []byte) {
	c.report.Actions = append(c.report.Actions, CleanAction{Path: rel, Kind: CleanEdited})
	if !c.opts.DryRun {
		p := filepath.Join(c.root, rel)
		mode := os.FileMode(0o644)
		if st, err := os.Stat(p); err == nil {
			mode = st.Mode().Perm()
		}
		_ = os.WriteFile(p, data, mode)
	}
}

// markdown removes bd's managed blocks; a file that is only bd's template
// afterwards is removed, unless git tracks it.
func (c *cleaner) markdown(rel string) {
	data, err := os.ReadFile(filepath.Join(c.root, rel))
	if err != nil || !strings.Contains(string(data), "<!-- BEGIN BEADS ") {
		return
	}
	if !c.allowed(rel, "<!-- BEGIN BEADS ") {
		return
	}
	rest := beadsBlockRe.ReplaceAllString(string(data), "\n")
	frontmatterOnly := strings.HasSuffix(rel, ".mdc") && onlyFrontmatter(rest)
	if (isBdTemplate(rest) || frontmatterOnly) && !c.tracked[rel] {
		c.remove(rel)
		return
	}
	rest = regexp.MustCompile(`\n{3,}`).ReplaceAllString(rest, "\n\n")
	c.write(rel, []byte(strings.TrimRight(rest, "\n")+"\n"))
}

// onlyFrontmatter reports a Cursor rule left with nothing but its
// frontmatter (`---`, `alwaysApply: true`).
func onlyFrontmatter(rest string) bool {
	for _, l := range strings.Split(rest, "\n") {
		switch strings.TrimSpace(l) {
		case "", "---", "alwaysApply: true":
		default:
			return false
		}
	}
	return true
}

// isBdCommand reports a hook command of bd ("bd prime --hook-json",
// "bd cursor-hook …", "bd codex-hook …").
func isBdCommand(v any) bool {
	m, ok := v.(map[string]any)
	if !ok {
		return false
	}
	cmd, _ := m["command"].(string)
	return strings.HasPrefix(strings.TrimSpace(cmd), "bd ")
}

// pruneBd removes bd's hook entries, then the lists and objects they leave
// empty; a hook group (an object of a list) whose "hooks" are all bd's goes
// too. changed is false when nothing of bd was found.
func pruneBd(v any, inList bool) (out any, empty, changed bool) {
	switch t := v.(type) {
	case map[string]any:
		if isBdCommand(t) {
			return nil, true, true
		}
		res := map[string]any{}
		for k, e := range t {
			ne, emp, ch := pruneBd(e, false)
			changed = changed || ch
			if emp && ch {
				if k == "hooks" && inList {
					return nil, true, true
				}
				continue
			}
			res[k] = ne
		}
		return res, len(res) == 0, changed
	case []any:
		var res []any
		for _, e := range t {
			ne, emp, ch := pruneBd(e, true)
			changed = changed || ch
			if emp && ch {
				continue
			}
			res = append(res, ne)
		}
		return res, len(res) == 0, changed
	}
	return v, false, false
}

// hooksJSON removes bd's hooks from a tool settings file; a file left with
// nothing but its format version is removed.
func (c *cleaner) hooksJSON(rel string) {
	data, err := os.ReadFile(filepath.Join(c.root, rel))
	if err != nil || !bytes.Contains(data, []byte(`"bd `)) {
		return
	}
	var doc any
	if json.Unmarshal(data, &doc) != nil {
		return
	}
	out, empty, changed := pruneBd(doc, false)
	if !changed || !c.allowed(rel, `"bd `) {
		return
	}
	if m, ok := out.(map[string]any); ok {
		if _, onlyVersion := m["version"]; onlyVersion && len(m) == 1 {
			empty = true
		}
	}
	if empty && !c.tracked[rel] {
		c.remove(rel)
		return
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return
	}
	c.write(rel, append(b, '\n'))
}

// codexConfig removes the .codex/config.toml bd writes to enable the hooks,
// once its hooks are gone.
func (c *cleaner) codexConfig() {
	rel := filepath.Join(".codex", "config.toml")
	data, err := os.ReadFile(filepath.Join(c.root, rel))
	if err != nil || c.tracked[rel] {
		return
	}
	if strings.Join(strings.Fields(string(data)), " ") != "[features] hooks = true" {
		return
	}
	if _, err := os.Stat(filepath.Join(c.root, ".codex", "hooks.json")); err == nil && !c.removedPath(filepath.Join(".codex", "hooks.json")) {
		return
	}
	c.remove(rel)
}

func (c *cleaner) removedPath(rel string) bool {
	for _, a := range c.report.Actions {
		if a.Path == rel && a.Kind == CleanRemoved {
			return true
		}
	}
	return false
}

// agentsSkill removes the beads skill bd installs for Codex.
func (c *cleaner) agentsSkill() {
	rel := filepath.Join(".agents", "skills", "beads")
	data, err := os.ReadFile(filepath.Join(c.root, rel, "SKILL.md"))
	if err != nil || !strings.Contains(string(data), "bd ") {
		return
	}
	if c.tracked[filepath.Join(rel, "SKILL.md")] && !c.opts.Tracked {
		c.report.Actions = append(c.report.Actions, CleanAction{Path: rel, Kind: CleanSkipped})
		return
	}
	c.remove(rel)
}

// gitignore moves bd's .gitignore entries to .git/info/exclude.
func (c *cleaner) gitignore() error {
	rel := ".gitignore"
	data, err := os.ReadFile(filepath.Join(c.root, rel))
	if err != nil || !c.git {
		return nil //nolint:nilerr // no .gitignore, or no repository to exclude the entries in
	}
	newContent, moved := cleanGitignoreContent(string(data))
	if len(moved) == 0 {
		return nil
	}
	committed := false
	if c.tracked[rel] {
		out, err := exec.Command("git", "-C", c.root, "show", "HEAD:.gitignore").Output()
		if err == nil {
			_, committedMoved := cleanGitignoreContent(string(out))
			committed = len(committedMoved) > 0
		}
	}
	if committed && !c.opts.Tracked {
		c.report.Actions = append(c.report.Actions, CleanAction{Path: rel, Kind: CleanSkipped})
		return nil
	}
	if !c.opts.DryRun {
		if err := addToExclude(filepath.Join(c.root, ".git", "info", "exclude"), moved); err != nil {
			return err
		}
	}
	if newContent == "" && !c.tracked[rel] {
		c.remove(rel)
		return nil
	}
	c.write(rel, []byte(newContent))
	return nil
}

// exclude makes sure .beads/ is ignored locally (.git/info/exclude).
func (c *cleaner) exclude() {
	if !c.git {
		return
	}
	p := filepath.Join(c.root, ".git", "info", "exclude")
	if data, err := os.ReadFile(p); err == nil && excludesBeads(string(data)) {
		return
	}
	c.report.Actions = append(c.report.Actions, CleanAction{Path: filepath.Join(".git", "info", "exclude"), Kind: CleanExclude})
	if !c.opts.DryRun {
		_ = addToExclude(p, []string{".beads/"})
	}
}

// excludesBeads reports an exclude file with a .beads/ (or .beads) line.
func excludesBeads(content string) bool {
	for _, l := range strings.Split(content, "\n") {
		if l = strings.TrimSpace(l); l == ".beads/" || l == ".beads" || l == "/.beads/" || l == "/.beads" {
			return true
		}
	}
	return false
}

func removeIfEmpty(dir string) {
	entries, err := os.ReadDir(dir)
	if err == nil && len(entries) == 0 {
		_ = os.Remove(dir)
	}
}

// FixImpact is the clean-up of `oh doctor --fix`: BEADS INTEGRATION sections
// of .git/hooks, then CleanImpact with committed content (Tracked). dryRun
// lists the actions only (hooks included).
func FixImpact(projectPath string, dryRun bool) (CleanReport, error) {
	var hooks []CleanAction
	for _, name := range beadsHookNames {
		content, err := os.ReadFile(filepath.Join(projectPath, ".git", "hooks", name))
		if err == nil && beadsSectionRe.MatchString(string(content)) {
			hooks = append(hooks, CleanAction{Path: filepath.Join(".git", "hooks", name), Kind: CleanEdited})
		}
	}
	if !dryRun {
		sanitizeHooks(projectPath)
	}
	r, err := CleanImpact(projectPath, CleanOptions{DryRun: dryRun, Tracked: true})
	r.Actions = append(hooks, r.Actions...)
	return r, err
}
