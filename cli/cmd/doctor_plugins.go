package cmd

import (
	"strings"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
)

// userPluginsCheck reports the plugins of the user's own tool configuration
// (S-11): outside strict isolation they run in the local sessions, next to the
// bundle's. A warning, not an error: the default does not change.
func userPluginsCheck(lister adapters.UserPluginLister, strict bool) (doctor.Result, bool) {
	if lister == nil {
		return doctor.Result{}, false
	}
	plugins, err := lister.UserPlugins()
	if err != nil || len(plugins) == 0 {
		return doctor.Result{}, false
	}
	check := doctor.Result{Name: i18n.Tf("cmd.doctor.plugins.name", toolName()), OK: true}
	list := strings.Join(plugins, ", ")
	if strict {
		check.Detail = i18n.Tf("cmd.doctor.plugins.hidden", len(plugins), list)
		return check, true
	}
	check.Warn = true
	check.Detail = i18n.Tf("cmd.doctor.plugins.running", len(plugins), list)
	return check, true
}

// isolationWarningText localizes a warning of the launch attestation: the
// plugins of the user configuration that the session runs (S-11), else the
// raw text of the adapter.
func isolationWarningText(w string) string {
	if rest, ok := strings.CutPrefix(w, adapters.WarnUserPlugin); ok {
		return i18n.Tf("cmd.doctor.plugins.launch", toolName(), rest)
	}
	if rest, ok := strings.CutPrefix(w, adapters.WarnPluginsUnchecked); ok {
		return i18n.Tf("cmd.doctor.plugins.launch_unchecked", toolName(), strings.TrimSpace(rest))
	}
	return i18n.Tf("cmd.v5.isolation_warning", w)
}
