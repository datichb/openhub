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

	var rules []sessionspec.PermissionRule
	for _, k := range keys {
		action := k
		if n, ok := v1ToNeutral[k]; ok {
			action = n
		}
		switch v := perms[k].(type) {
		case map[string]interface{}:
			patterns := make([]string, 0, len(v))
			for p := range v {
				patterns = append(patterns, p)
			}
			sort.Slice(patterns, func(i, j int) bool {
				li, lj := literalLen(patterns[i]), literalLen(patterns[j])
				if li != lj {
					return li < lj
				}
				return patterns[i] < patterns[j]
			})
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
	}
	return rules
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
