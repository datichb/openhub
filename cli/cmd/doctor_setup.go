package cmd

import (
	"context"

	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
)

// Doctor › setup of hub.toml (K4a, bug 6): what the first-run wizard writes
// (setup_done, the default provider and, for Bedrock, its region) and that a
// hub.toml overwritten by a stale copy loses. Each missing key is a failure
// that tells the command that writes it; oh doctor used to be green.

// Identifiers of the results of the setup check.
const (
	doctorIDSetupDone       = "setup_done"
	doctorIDDefaultProvider = "default_provider"
	doctorIDProviderRegion  = "provider_region"
)

// setupDoctorChecks reports setup_done, default_provider and the region of
// the default provider. It reports nothing when hub.toml is not loaded (the
// config check says so).
func setupDoctorChecks(ctx context.Context) []doctor.Result {
	a := TryApp()
	if a == nil || a.Config == nil {
		return nil
	}
	c := a.Config
	out := make([]doctor.Result, 0, 3)

	done := doctor.Result{ID: doctorIDSetupDone, Name: i18n.T("cmd.doctor.setup.done.name"), OK: c.CLI.SetupDone}
	if c.CLI.SetupDone {
		done.Detail = i18n.T("cmd.doctor.setup.done.ok")
	} else {
		done.Detail = i18n.T("cmd.doctor.setup.done.missing")
	}
	out = append(out, done)

	prov := doctor.Result{ID: doctorIDDefaultProvider, Name: i18n.T("cmd.doctor.setup.provider.name"), OK: c.LLM.DefaultProvider != ""}
	if prov.OK {
		prov.Detail = c.LLM.DefaultProvider
	} else {
		prov.Detail = i18n.T("cmd.doctor.setup.provider.missing")
	}
	out = append(out, prov)

	// Without a default provider a launch uses Bedrock (provider.ResolveProvider).
	if c.LLM.DefaultProvider == "" || c.LLM.DefaultProvider == "bedrock" {
		out = append(out, bedrockRegionResult(ctx, c.Provider.Bedrock.AWSRegion, c.Provider.Bedrock.AWSProfile))
	}
	return out
}

// bedrockRegionResult is the region of Bedrock: the one of hub.toml; else the
// one the AWS environment or profile gives (a warning: write it down); else a
// failure.
func bedrockRegionResult(ctx context.Context, region, profile string) doctor.Result {
	r := doctor.Result{ID: doctorIDProviderRegion, Name: i18n.T("cmd.doctor.setup.region.name")}
	if region != "" {
		r.OK, r.Detail = true, region
		return r
	}
	if env := credproxy.AWSRegion(ctx, profile); env != "" {
		r.OK, r.Warn = true, true
		r.Detail = i18n.Tf("cmd.doctor.setup.region.from_env", env, env)
		return r
	}
	r.Detail = i18n.T("cmd.doctor.setup.region.missing")
	return r
}
