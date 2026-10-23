package cmd

import (
	"context"
	"os"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/envfilter"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
)

// Doctor › agent environment (audit v5, S-28): the tool server of a local
// session, and so the shells of the agents, inherit only a short base list of
// the user's variables plus [execution] env_allow. The check says what the
// list lets through that looks like a secret, and which variables of the
// user's environment are held back (names only, never values).

func init() { registerDoctorCheck(envAllowDoctorChecks) }

// maxEnvNamesShown bounds the names listed by the check.
const maxEnvNamesShown = 30

// envNoise are variables of the terminal session nobody expects to pass.
var envNoise = map[string]bool{"_": true, "PWD": true, "OLDPWD": true, "SHLVL": true}

// envAllowFindings is what the doctor reports about env_allow and the
// environment.
type envAllowFindings struct {
	Invalid      []string // entries that are neither a name nor PREFIX_* (ignored)
	SecretLooks  []string // entries named like a secret
	SecretPassed []string // variables present that env_allow lets through and look secret
	Managed      []string // entries for credentials oh manages (removed anyway)
	Held         []string // variables present that do not pass
}

// analyzeEnvAllow reads env_allow against an environment ("NAME=value").
func analyzeEnvAllow(extra, environ []string) envAllowFindings {
	var f envAllowFindings
	for _, e := range extra {
		switch {
		case !envfilter.ValidPattern(e):
			f.Invalid = append(f.Invalid, e)
		case envfilter.HardDenied(e):
			f.Managed = append(f.Managed, e)
		case envfilter.LooksSecret(e):
			f.SecretLooks = append(f.SecretLooks, e)
		}
	}
	_, dropped := envfilter.Split(environ, extra)
	for _, kv := range environ {
		name, _, ok := strings.Cut(kv, "=")
		if !ok || name == "" || !envfilter.Allowed(name, extra) {
			continue
		}
		if !envfilter.FromBase(name) && envfilter.LooksSecret(name) && !contains(f.SecretLooks, name) {
			f.SecretPassed = append(f.SecretPassed, name)
		}
	}
	for _, n := range dropped {
		if !envNoise[n] {
			f.Held = append(f.Held, n)
		}
	}
	sort.Strings(f.SecretPassed)
	sort.Strings(f.Held)
	return f
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func shortNames(names []string) string {
	if len(names) <= maxEnvNamesShown {
		return strings.Join(names, ", ")
	}
	return strings.Join(names[:maxEnvNamesShown], ", ") + i18n.Tf("cmd.doctor.env_allow.more", len(names)-maxEnvNamesShown)
}

func envAllowDoctorChecks(context.Context) []doctor.Result {
	var extra []string
	if a := TryApp(); a != nil {
		extra = a.Config.Execution.EnvAllow
	}
	f := analyzeEnvAllow(extra, os.Environ())
	name := i18n.T("cmd.doctor.env_allow")
	out := []doctor.Result{{Name: name, OK: true, Detail: i18n.Tf("cmd.doctor.env_allow.base", len(extra))}}
	warn := func(key string, names []string) {
		out = append(out, doctor.Result{Name: name, OK: true, Warn: true, Detail: i18n.Tf(key, shortNames(names))})
	}
	if len(f.Invalid) > 0 {
		warn("cmd.doctor.env_allow.invalid", f.Invalid)
	}
	if len(f.SecretLooks) > 0 {
		warn("cmd.doctor.env_allow.secret_entries", f.SecretLooks)
	}
	if len(f.SecretPassed) > 0 {
		warn("cmd.doctor.env_allow.secret_passed", f.SecretPassed)
	}
	if len(f.Managed) > 0 {
		warn("cmd.doctor.env_allow.managed", f.Managed)
	}
	if len(f.Held) > 0 {
		out = append(out, doctor.Result{Name: name, OK: true,
			Detail: i18n.Tf("cmd.doctor.env_allow.held", len(f.Held), shortNames(f.Held))})
	}
	return out
}
