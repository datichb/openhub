package bundle

import (
	"fmt"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// v1ToNeutral maps hub (opencode V1 style) permission keys to neutral actions.
var v1ToNeutral = map[string]string{
	"bash":  sessionspec.ActionShell,
	"task":  sessionspec.ActionSubagent,
	"write": sessionspec.ActionEdit,
	"patch": sessionspec.ActionEdit,
}

// BeadsShellGuard returns the shell rules, last of every agent, that refuse
// to run a bd named by a path ("/opt/homebrew/bin/bd close …"): sessions
// call `bd`, found first on their PATH as the fake bd that goes through the
// Beads gateway and the workflow beads.allow (QB1). In a container, the
// only bd is the fake one anyway.
func BeadsShellGuard() []sessionspec.PermissionRule {
	return []sessionspec.PermissionRule{
		{Action: sessionspec.ActionShell, Resource: "*/bd", Effect: sessionspec.EffectDeny},
		{Action: sessionspec.ActionShell, Resource: "*/bd *", Effect: sessionspec.EffectDeny},
	}
}

// ConvertPermissions converts a hub permission map (frontmatter + base file,
// V1 shape: key → "allow"|"deny"|"ask"|bool or key → {pattern: effect}) into
// ordered neutral rules (last match wins).
//
// Within a pattern map, rules are ordered from the most general to the most
// specific ("*" first, then by number of literal characters) so that specific
// exceptions such as "git push*": deny override broader allows.
func ConvertPermissions(perms map[string]interface{}) []sessionspec.PermissionRule {
	keys := make([]string, 0, len(perms))
	for k := range perms {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Several V1 keys may map to one neutral action (write/patch → edit). Their
	// rule sets are merged so that no key widens another one: for each
	// resource, the most restrictive effect of the keys that cover it wins.
	byAction := map[string][][]sessionspec.PermissionRule{}
	var order []string
	for _, k := range keys {
		action := k
		if n, ok := v1ToNeutral[k]; ok {
			action = n
		}
		if _, ok := byAction[action]; !ok {
			order = append(order, action)
		}
		byAction[action] = append(byAction[action], keyRules(action, perms[k]))
	}

	var rules []sessionspec.PermissionRule
	for _, action := range order {
		sets := byAction[action]
		if len(sets) == 1 {
			rules = append(rules, sets[0]...)
			continue
		}
		rules = append(rules, mergeRuleSets(action, sets)...)
	}
	return rules
}

// keyRules converts the value of one V1 key, ordered from the most general to
// the most specific pattern.
func keyRules(action string, val interface{}) []sessionspec.PermissionRule {
	var rules []sessionspec.PermissionRule
	switch v := val.(type) {
	case map[string]interface{}:
		patterns := make([]string, 0, len(v))
		for p := range v {
			patterns = append(patterns, p)
		}
		sortPatterns(patterns)
		for _, p := range patterns {
			if eff, ok := toEffect(v[p]); ok {
				rules = append(rules, sessionspec.PermissionRule{Action: action, Resource: p, Effect: eff})
			}
		}
	default:
		if eff, ok := toEffect(v); ok {
			rules = append(rules, sessionspec.PermissionRule{Action: action, Resource: "*", Effect: eff})
		}
	}
	return rules
}

func sortPatterns(patterns []string) {
	sort.Slice(patterns, func(i, j int) bool {
		li, lj := literalLen(patterns[i]), literalLen(patterns[j])
		if li != lj {
			return li < lj
		}
		return patterns[i] < patterns[j]
	})
}

func mergeRuleSets(action string, sets [][]sessionspec.PermissionRule) []sessionspec.PermissionRule {
	seen := map[string]bool{}
	var resources []string
	for _, set := range sets {
		for _, r := range set {
			if !seen[r.Resource] {
				seen[r.Resource] = true
				resources = append(resources, r.Resource)
			}
		}
	}
	sortPatterns(resources)
	out := make([]sessionspec.PermissionRule, 0, len(resources))
	for _, res := range resources {
		var eff sessionspec.Effect
		for _, set := range sets {
			if e, ok := evaluate(set, res); ok && (eff == "" || restrictiveness(e) > restrictiveness(eff)) {
				eff = e
			}
		}
		if eff != "" {
			out = append(out, sessionspec.PermissionRule{Action: action, Resource: res, Effect: eff})
		}
	}
	return out
}

// evaluate returns the effect a rule set (last match wins) gives to a
// resource pattern, taken literally.
func evaluate(set []sessionspec.PermissionRule, resource string) (sessionspec.Effect, bool) {
	var eff sessionspec.Effect
	ok := false
	for _, r := range set {
		if wildcardMatch(r.Resource, resource) {
			eff, ok = r.Effect, true
		}
	}
	return eff, ok
}

func restrictiveness(e sessionspec.Effect) int {
	switch e {
	case sessionspec.EffectDeny:
		return 2
	case sessionspec.EffectAsk:
		return 1
	}
	return 0
}

// wildcardMatch matches s against a pattern where '*' matches any sequence and
// '?' any single character (opencode wildcard semantics).
func wildcardMatch(pattern, s string) bool {
	if pattern == "*" || pattern == s {
		return true
	}
	p, i := 0, 0
	star, mark := -1, 0
	for i < len(s) {
		switch {
		case p < len(pattern) && (pattern[p] == '?' || pattern[p] == s[i]):
			p++
			i++
		case p < len(pattern) && pattern[p] == '*':
			star, mark = p, i
			p++
		case star >= 0:
			p = star + 1
			mark++
			i = mark
		default:
			return false
		}
	}
	for p < len(pattern) && pattern[p] == '*' {
		p++
	}
	return p == len(pattern)
}

func literalLen(p string) int {
	return len(strings.NewReplacer("*", "", "?", "").Replace(p))
}

func toEffect(v interface{}) (sessionspec.Effect, bool) {
	switch val := v.(type) {
	case bool:
		if val {
			return sessionspec.EffectAllow, true
		}
		return sessionspec.EffectDeny, true
	case string:
		switch strings.ToLower(val) {
		case "allow", "true":
			return sessionspec.EffectAllow, true
		case "deny", "false":
			return sessionspec.EffectDeny, true
		case "ask":
			return sessionspec.EffectAsk, true
		}
	case nil:
		return "", false
	default:
		return toEffect(fmt.Sprint(val))
	}
	return "", false
}
