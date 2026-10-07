// Package deploycleanup removes what the former per-project deployment
// (`oh deploy`, removed in v5) left in a project (P3-T28, `oh migrate
// deploy-cleanup`): the deployed agents and skills, its state files, the
// team file of the team MCP server, and — in opencode.json — only the keys
// oh wrote that are still unchanged since the last deployment.
//
// The reference is the config_snapshot of .opencode/.deploy-state: a copy
// of the whole opencode.json taken right after the deployment, user keys
// included. A key is removed only when it is one oh writes (agents, MCP
// servers, plugin, compaction, instructions, provider…) and its value is
// still the one of the snapshot; a key changed since is kept and reported.
// Without .deploy-state, opencode.json is left untouched.
package deploycleanup

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

// Files of the former deployment, relative to the project.
const (
	AgentsDir       = ".opencode/agents"
	SkillsDir       = ".opencode/skills"
	DeployStateFile = ".opencode/.deploy-state"
	ManifestFile    = ".opencode/context-manifest.json"
	TeamFile        = ".opencode/team.json"
	ConfigFile      = "opencode.json"
)

// schemaURL is the `$schema` oh wrote.
const schemaURL = "https://opencode.ai/config.json"

// nativeAgents are the opencode agents oh disabled in opencode.json.
var nativeAgents = []string{"build", "plan", "general", "explore", "scout"}

// defaultInstructionFiles are the project files oh added to `instructions`.
var defaultInstructionFiles = []string{"ONBOARDING.md", "CONVENTIONS.md", ".claude/CLAUDE.md"}

// Options are what oh knows about the former deployments.
type Options struct {
	// AgentIDs are the hub agents (their opencode.json `agent.<id>` blocks).
	AgentIDs []string
	// InstructionFiles are the extra instruction files of hub.toml.
	InstructionFiles []string
}

// Item is a file or folder to remove.
type Item struct {
	Rel   string `json:"path"`
	Dir   bool   `json:"dir,omitempty"`
	Count int    `json:"count,omitempty"` // files in a folder
	Link  bool   `json:"link,omitempty"`  // symbolic link (worktree)
}

// Plan is the cleanup of one project.
type Plan struct {
	Dir   string `json:"dir"`
	Items []Item `json:"items,omitempty"`
	// Removed lists the opencode.json keys removed (paths, e.g. agent.developer).
	Removed []string `json:"removed,omitempty"`
	// Kept lists the opencode.json keys oh wrote but changed since (kept).
	Kept []string `json:"kept,omitempty"`
	// ConfigUntouched explains why opencode.json is not cleaned ("" = it is,
	// or there is nothing to do): "no_deploy_state", "unreadable".
	ConfigUntouched string `json:"config_untouched,omitempty"`
	// DeleteConfig: nothing of the user remains in opencode.json.
	DeleteConfig bool `json:"delete_config,omitempty"`

	before, after []byte
}

// Empty reports whether nothing is left to remove.
func (p *Plan) Empty() bool {
	return len(p.Items) == 0 && len(p.Removed) == 0 && !p.DeleteConfig
}

// Leftovers reports whether former deployment files remain (Doctor),
// including an opencode.json that cannot be cleaned automatically.
func (p *Plan) Leftovers() bool { return !p.Empty() }

// Scan computes the cleanup of the project at dir (nothing is changed).
func Scan(dir string, opts Options) (*Plan, error) {
	p := &Plan{Dir: dir}
	for _, rel := range []string{AgentsDir, SkillsDir} {
		if it, ok := statItem(dir, rel); ok {
			p.Items = append(p.Items, it)
		}
	}
	for _, rel := range []string{DeployStateFile, ManifestFile, TeamFile} {
		if it, ok := statItem(dir, rel); ok {
			p.Items = append(p.Items, it)
		}
	}
	state, err := readState(dir)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, ConfigFile))
	switch {
	case errors.Is(err, os.ErrNotExist):
		return p, nil
	case err != nil:
		return nil, err
	}
	if state == nil || state.ConfigSnapshot == nil {
		if len(p.Items) > 0 {
			p.ConfigUntouched = "no_deploy_state"
		}
		return p, nil
	}
	root, perr := parseOrdered(data)
	obj, ok := root.(*object)
	if perr != nil || !ok {
		p.ConfigUntouched = "unreadable"
		return p, nil //nolint:nilerr // an unreadable opencode.json is reported, never rewritten
	}
	c := &cleaner{snap: state.ConfigSnapshot, agents: map[string]bool{}, instr: map[string]bool{}, provider: opencodeProvider(state.Provider)}
	for _, id := range append(append(append([]string(nil), opts.AgentIDs...), state.SelectedAgents...), nativeAgents...) {
		c.agents[id] = true
	}
	for _, f := range append(append([]string(nil), defaultInstructionFiles...), opts.InstructionFiles...) {
		c.instr[f] = true
	}
	c.clean(obj)
	sort.Strings(c.removed)
	sort.Strings(c.kept)
	p.Removed, p.Kept = c.removed, c.kept
	if len(obj.keys) == 0 || (len(obj.keys) == 1 && obj.keys[0] == "$schema" && obj.vals["$schema"] == schemaURL) {
		p.DeleteConfig = true
	}
	if len(p.Removed) > 0 || p.DeleteConfig {
		p.before, p.after = data, encodeOrdered(obj)
	}
	return p, nil
}

func statItem(dir, rel string) (Item, bool) {
	path := filepath.Join(dir, rel)
	fi, err := os.Lstat(path)
	if err != nil {
		return Item{}, false
	}
	it := Item{Rel: rel, Link: fi.Mode()&os.ModeSymlink != 0}
	if fi.IsDir() {
		it.Dir = true
		_ = filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				it.Count++
			}
			return nil
		})
	}
	return it, true
}

// deployState is what the cleanup reads of .opencode/.deploy-state.
type deployState struct {
	ConfigSnapshot map[string]any `json:"config_snapshot"`
	SelectedAgents []string       `json:"selected_agents"`
	Provider       string         `json:"provider"` // hub name (bedrock…)
}

func readState(dir string) (*deployState, error) {
	data, err := os.ReadFile(filepath.Join(dir, DeployStateFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s deployState
	if uerr := json.Unmarshal(data, &s); uerr != nil {
		return nil, nil //nolint:nilerr // unreadable state: opencode.json is not touched
	}
	return &s, nil
}

// cleaner removes oh's unchanged keys from an opencode.json object.
type cleaner struct {
	snap          map[string]any
	agents, instr map[string]bool
	provider      string // opencode id of the deployed provider ("" = unknown)
	removed, kept []string
}

// snapAt returns the snapshot value at a path.
func (c *cleaner) snapAt(path ...string) (any, bool) {
	var cur any = c.snap
	for _, k := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		if cur, ok = m[k]; !ok {
			return nil, false
		}
	}
	return cur, true
}

// unchanged reports whether v is still the snapshot value at path.
func (c *cleaner) unchanged(v any, path ...string) bool {
	s, ok := c.snapAt(path...)
	return ok && reflect.DeepEqual(plain(v), s)
}

// drop removes key of o when unchanged since the snapshot (else kept).
func (c *cleaner) drop(o *object, key string, path ...string) {
	v, ok := o.get(key)
	if !ok {
		return
	}
	full := append(append([]string(nil), path...), key)
	if c.unchanged(v, full...) {
		o.del(key)
		c.removed = append(c.removed, strings.Join(full, "."))
	} else {
		c.kept = append(c.kept, strings.Join(full, "."))
	}
}

func (c *cleaner) clean(o *object) {
	for _, k := range []string{"model", "enabled_providers", "compaction", "subagent_depth"} {
		c.drop(o, k)
	}
	// Only the block of the deployed provider, as oh wrote it.
	c.children(o, "provider", func(id string, v any) bool {
		m, ok := v.(*object)
		if !ok || id != c.provider {
			return false
		}
		if id == "anthropic" {
			return reflect.DeepEqual(plain(m), map[string]any{"options": map[string]any{"setCacheKey": true}})
		}
		return len(m.keys) == 0
	})
	c.children(o, "permission", func(id string, v any) bool {
		return (id == "websearch" || id == "webfetch") && v == "allow"
	})
	c.children(o, "agent", func(id string, _ any) bool { return c.agents[id] })
	c.children(o, "mcp", func(_ string, v any) bool { return ohMCP(v) })
	c.elements(o, "plugin", func(v any) bool { return v == "context-mode" })
	c.elements(o, "instructions", func(v any) bool { s, _ := v.(string); return c.instr[s] })
}

// children removes the oh-written, unchanged entries of an object key;
// the key goes when nothing is left.
func (c *cleaner) children(o *object, key string, oh func(id string, v any) bool) {
	v, ok := o.get(key)
	m, isObj := v.(*object)
	if !ok || !isObj {
		return
	}
	for _, id := range append([]string(nil), m.keys...) {
		if cv, _ := m.get(id); oh(id, cv) {
			c.drop(m, id, key)
		}
	}
	if len(m.keys) == 0 {
		o.del(key)
	}
}

// elements removes oh's elements of an array key when they are in the
// snapshot array too; the key goes when the array is left empty.
func (c *cleaner) elements(o *object, key string, oh func(v any) bool) {
	v, ok := o.get(key)
	arr, isArr := v.([]any)
	if !ok || !isArr {
		return
	}
	snap, _ := c.snapAt(key)
	snapArr, _ := snap.([]any)
	inSnap := func(e any) bool {
		for _, s := range snapArr {
			if reflect.DeepEqual(plain(e), s) {
				return true
			}
		}
		return false
	}
	var out []any
	for _, e := range arr {
		if oh(e) && inSnap(e) {
			c.removed = append(c.removed, fmt.Sprintf("%s[%v]", key, e))
			continue
		}
		out = append(out, e)
	}
	if len(out) == 0 {
		o.del(key)
	} else if len(out) != len(arr) {
		o.set(key, out)
	}
}

// opencodeProvider maps a hub provider name to the opencode provider id.
func opencodeProvider(name string) string {
	if name == "bedrock" {
		return "amazon-bedrock"
	}
	return name
}

// ohMCP reports an MCP entry written by oh (`<oh> mcp serve <name>`).
func ohMCP(v any) bool {
	m, ok := v.(*object)
	if !ok {
		return false
	}
	cmd, _ := m.vals["command"].([]any)
	return len(cmd) >= 3 && cmd[1] == "mcp" && cmd[2] == "serve"
}

// Diff is the unified diff of opencode.json ("" when unchanged).
func (p *Plan) Diff() string {
	if p.before == nil {
		return ""
	}
	after := string(p.after)
	if p.DeleteConfig {
		after = ""
	}
	d, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A: difflib.SplitLines(string(p.before)), B: difflib.SplitLines(after),
		FromFile: ConfigFile, ToFile: ConfigFile + " (" + "cleaned" + ")", Context: 2,
	})
	return d
}

// Apply removes the items and writes the cleaned opencode.json.
func (p *Plan) Apply() error {
	var errs []error
	for _, it := range p.Items {
		path := filepath.Join(p.Dir, it.Rel)
		var err error
		if it.Dir && !it.Link {
			err = os.RemoveAll(path)
		} else {
			err = os.Remove(path)
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	cfg := filepath.Join(p.Dir, ConfigFile)
	switch {
	case p.DeleteConfig:
		if err := os.Remove(cfg); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
	case p.after != nil:
		tmp := cfg + ".tmp"
		if err := os.WriteFile(tmp, p.after, 0o644); err != nil {
			errs = append(errs, err)
		} else if err := os.Rename(tmp, cfg); err != nil {
			errs = append(errs, err)
		}
	}
	// .opencode/ removed when nothing else is left in it.
	dir := filepath.Join(p.Dir, ".opencode")
	if entries, err := os.ReadDir(dir); err == nil && len(entries) == 0 {
		_ = os.Remove(dir)
	}
	return errors.Join(errs...)
}
