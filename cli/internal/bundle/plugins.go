package bundle

import (
	"strings"

	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// WorkflowPlugins turns the `plugins:` of a workflow (O5) into bundle
// plugins. The id of a reference is an npm package spec ("context-mode",
// "@scope/pkg@1.2.0") installed by the tool; the plugin id is the package
// name, the spec is kept as is.
func WorkflowPlugins(refs []workflow.PluginRef) []sessionspec.PluginDef {
	out := make([]sessionspec.PluginDef, 0, len(refs))
	for _, r := range refs {
		def := sessionspec.PluginDef{ID: npmPackageName(r.ID), Dir: r.ID}
		if len(r.Options) > 0 {
			def.Options = make(map[string]any, len(r.Options))
			for k, v := range r.Options {
				def.Options[k] = v
			}
		}
		out = append(out, def)
	}
	return out
}

// npmPackageName strips the version or tag of an npm spec
// ("@scope/pkg@^1" → "@scope/pkg", "pkg@latest" → "pkg").
func npmPackageName(spec string) string {
	start := 0
	if strings.HasPrefix(spec, "@") {
		start = 1
	}
	if i := strings.Index(spec[start:], "@"); i >= 0 {
		return spec[:start+i]
	}
	return spec
}
