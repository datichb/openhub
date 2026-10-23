package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/services/doctor"
)

// credentialReport is the credential a launch of a project would use,
// found without any network call.
type credentialReport struct {
	prov    string
	name    provider.Name
	project *domain.Project
	cfg     provider.Config
	cred    provider.ResolvedCredential
	detail  string
	ok      bool
	// envBearerUnused: the shell has a Bedrock API key the launch does not
	// use (no stored key, or SigV4): it can be imported.
	envBearerUnused bool
	// envBearerDiffers: the shell key differs from the stored key used.
	envBearerDiffers bool
}

// credentialStatus follows the credential cascade of a launch of project
// with the provider override (-P), as the launch does (N12: checked before
// any launch).
func credentialStatus(ctx context.Context, a *app.App, project *domain.Project, providerFlag string) (string, bool) {
	r := resolveCredentialReport(ctx, a, project, providerFlag)
	return r.detail, r.ok
}

func resolveCredentialReport(ctx context.Context, a *app.App, project *domain.Project, providerFlag string) credentialReport {
	prov := provider.ResolveProvider(providerFlag, "", a.Config.LLM.DefaultProvider)
	cfg := hubProviderCfg(a, prov)
	var projectID, tokenKey, teamID string
	if project != nil {
		prov, tokenKey, cfg = projectProvider(a, project, providerFlag)
		projectID = project.ID
		if tc := config.ResolveTeamForProject(a.Config, project); tc.Enabled {
			teamID = tc.TeamID
		}
	}
	r := credentialReport{prov: prov, name: provider.Name(prov), project: project, cfg: cfg}
	if prov == "" {
		r.detail = i18n.T("cmd.doctor.no_provider")
		return r
	}
	scope := i18n.T("cmd.doctor.credentials.scope_hub")
	if project != nil {
		scope = i18n.Tf("cmd.doctor.credentials.scope_project", project.Name)
	}

	if provider.KeychainKey(r.name, "") == "" {
		// Provider without a secret (gh auth…): detected on the host.
		if det := provider.Detect(r.name); det.Available {
			r.detail, r.ok = fmt.Sprintf("%s — %s (%s)", prov, det.Source, det.Details), true
			return r
		}
		r.detail = i18n.Tf("cmd.doctor.credentials.missing", prov, scope)
		return r
	}

	var secrets provider.SecretStore
	if a.Secrets != nil {
		secrets = a.Secrets
	}
	envBearer := ""
	if r.name == provider.Bedrock {
		envBearer = provider.EnvBearerToken()
	}
	cred, err := provider.ResolveCredentialSource(ctx, secrets, r.name, projectID, teamID, tokenKey, &cfg)
	r.cred = cred
	switch {
	case errors.Is(err, provider.ErrBearerNotStored):
		r.detail = i18n.Tf("cmd.doctor.credentials.bearer_missing", prov, scope)
		r.envBearerUnused = envBearer != ""
	case errors.Is(err, provider.ErrNoCredential):
		r.detail = i18n.Tf("cmd.doctor.credentials.missing", prov, scope)
	case err != nil:
		r.detail = i18n.Tf("cmd.doctor.credentials.error", prov, err)
	case cred.Source.Scope == "project" || cred.Source.Scope == "team" || cred.Source.Scope == "hub":
		r.detail, r.ok = i18n.Tf("cmd.doctor.credentials.found_"+cred.Source.Scope, prov, cred.Source.KeychainKey), true
		r.envBearerDiffers = envBearer != "" && envBearer != cred.Secret
	default:
		// Bedrock SigV4: the AWS SDK resolves the credentials at launch (the
		// shell API key is not one of them).
		r.envBearerUnused = envBearer != ""
		if cfg.AWSProfile != "" {
			r.detail, r.ok = i18n.Tf("cmd.doctor.credentials.found_aws_profile", prov, cfg.AWSProfile), true
		} else if det := provider.DetectBedrockSigV4(); det.Available {
			r.detail, r.ok = i18n.Tf("cmd.doctor.credentials.found_aws", prov, det.Source, det.Details), true
		} else {
			r.detail = i18n.Tf("cmd.doctor.credentials.missing", prov, scope)
		}
	}
	switch {
	case r.envBearerUnused:
		r.detail += " — " + i18n.Tf("cmd.doctor.credentials.env_bearer_unused", bedrockSetupHint(project, true))
	case r.envBearerDiffers:
		r.detail += " — " + i18n.Tf("cmd.doctor.credentials.env_bearer_differs", bedrockSetupHint(project, true))
	}
	return r
}

// checkProviderCredentials follows the credential cascade of a session
// started from the current directory: project (explicit key, project key) →
// team → hub → AWS profile or default chain for Bedrock (A2), then asks the
// provider whether it accepts a stored Bedrock API key.
func checkProviderCredentials() doctor.Result {
	a := TryApp()
	if a == nil || a.Config == nil {
		return doctor.Result{Detail: i18n.T("cmd.doctor.app_unavailable")}
	}
	ctx := context.Background()
	var project *domain.Project
	if a.Projects != nil {
		project, _ = budgetProject(ctx, a, "")
	}
	return providerCredentialCheck(ctx, a, project)
}

func providerCredentialStatus(ctx context.Context, a *app.App, project *domain.Project) (string, bool) {
	return credentialStatus(ctx, a, project, "")
}

// bedrockProbeURL is the endpoint the doctor validates a Bedrock API key on
// (replaced in tests).
var bedrockProbeURL = credproxy.BedrockControlURL

// bedrockProbeTimeout bounds the validation call.
const bedrockProbeTimeout = 5 * time.Second

// providerCredentialCheck is the doctor result of the credential of a
// launch of project: where it comes from, when a stored key was saved, and
// whether Bedrock accepts it (skipped offline).
func providerCredentialCheck(ctx context.Context, a *app.App, project *domain.Project) doctor.Result {
	r := resolveCredentialReport(ctx, a, project, "")
	res := doctor.Result{OK: r.ok, Detail: r.detail, Warn: r.ok && (r.envBearerUnused || r.envBearerDiffers)}
	if !r.ok {
		return res
	}
	if key := r.cred.Source.KeychainKey; key != "" && a.Secrets != nil {
		if t, ok := provider.SecretModifiedAt(ctx, a.Secrets, key); ok {
			res.Detail += i18n.Tf("cmd.doctor.credentials.stored_at", t.Local().Format("2006-01-02 15:04"))
		}
	}
	if r.name != provider.Bedrock || r.cred.Source.Kind != domain.CredentialBearer || r.cred.Secret == "" {
		return res
	}
	if doctorOffline() {
		res.Detail += " — " + i18n.T("cmd.doctor.credentials.probe_offline")
		return res
	}
	region := r.cfg.AWSRegion
	if region == "" {
		region = credproxy.AWSRegion(ctx, r.cfg.AWSProfile)
	}
	base, err := bedrockProbeURL(region)
	if err != nil {
		res.Warn = true
		res.Detail += " — " + i18n.Tf("cmd.doctor.credentials.probe_failed", err)
		return res
	}
	pctx, cancel := context.WithTimeout(ctx, bedrockProbeTimeout)
	defer cancel()
	c, err := credproxy.CheckBedrockBearer(pctx, &http.Client{Timeout: bedrockProbeTimeout}, base, r.cred.Secret)
	switch {
	case err != nil:
		res.Warn = true
		res.Detail += " — " + i18n.Tf("cmd.doctor.credentials.probe_failed", err)
	case c.Status < 300:
		res.Detail += " — " + i18n.T("cmd.doctor.credentials.probe_ok")
	case !c.Accepted():
		res.OK, res.Warn = false, false
		var hp *domain.Project
		if r.cred.Source.Scope == "project" {
			hp = project
		}
		res.Detail += " — " + i18n.Tf("cmd.doctor.credentials.probe_refused", c.Status, c.Message, bedrockSetupHint(hp, r.envBearerDiffers))
	default:
		res.Warn = true
		res.Detail += " — " + i18n.Tf("cmd.doctor.credentials.probe_inconclusive", c.Status, c.Message)
	}
	return res
}
