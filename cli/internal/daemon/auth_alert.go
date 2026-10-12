package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/provider"
)

// The provider refuses the credential of a group (expired Bedrock API key,
// revoked key…): the proxy logs it and the sessions of the group get an
// alert (✗ in « À traiter », dismissed by the user only) naming the key
// used and how to renew it.

// alertProviderAuth is the alert of a session whose provider refuses the
// credential of its group.
const alertProviderAuth = "provider_auth"

// credentialRef names the credential of a grant for the proxy; its ID
// tells two values of the same key apart (never the secret).
func credentialRef(src domain.CredentialSource, secret string) credproxy.CredentialRef {
	ref := credproxy.CredentialRef{Kind: string(src.Kind), Key: src.KeychainKey, Scope: src.Scope, Profile: src.Profile}
	id := src.KeychainKey + "\x00" + src.Profile
	if secret != "" {
		sum := sha256.Sum256([]byte(secret))
		id += "\x00" + hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256([]byte(id))
	ref.ID = hex.EncodeToString(sum[:6])
	return ref
}

// authRejected is the credential refusal of a token for the API, if any.
func (d *Daemon) authRejected(token string) *AuthRejected {
	r, ok := d.proxy.AuthRejection(token)
	if !ok {
		return nil
	}
	return &AuthRejected{Status: r.Status, Message: r.Message, CredentialKey: r.Credential.Key, Scope: r.Credential.Scope, At: r.At}
}

func (d *Daemon) onAuthRejected(r credproxy.AuthRejection) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	d.raiseAuthRejected(ctx, r)
}

// raiseAuthRejected records the alert of the sessions of the group whose
// credential the provider refused. One alert per session and credential
// value: the same key refused again (or dismissed) is not raised twice, a
// renewed key refused is.
func (d *Daemon) raiseAuthRejected(ctx context.Context, r credproxy.AuthRejection) {
	storedAt, dated := time.Time{}, false
	if r.Credential.Key != "" && d.opts.Secrets != nil {
		storedAt, dated = provider.SecretModifiedAt(ctx, d.opts.Secrets, r.Credential.Key)
	}
	attrs := []any{"owner", r.Owner, "provider", r.Provider, "status", r.Status, "message", r.Message,
		"credential_key", r.Credential.Key, "credential_scope", r.Credential.Scope}
	if dated {
		attrs = append(attrs, "stored_at", storedAt.Format(time.RFC3339))
	}
	slog.Warn("ohd: the provider refused the credential of a server group", attrs...)
	if d.opts.Decisions == nil || d.opts.Sessions == nil || d.opts.Servers == nil {
		return
	}
	srv, err := d.opts.Servers.Get(ctx, r.Owner)
	if err != nil {
		return
	}
	sessions, err := d.opts.Sessions.List(ctx, srv.ProjectID)
	if err != nil {
		return
	}
	project := srv.ProjectID
	if d.opts.ProjectName != nil {
		if n := d.opts.ProjectName(ctx, srv.ProjectID); n != "" {
			project = n
		}
	}
	title, msg := authRejectedText(r, project, storedAt, dated)
	for _, s := range sessions {
		if s.GroupKey != r.Owner || isTerminal(s.State) {
			continue
		}
		dec := domain.Decision{SessionID: s.ID, GroupKey: s.GroupKey, Kind: domain.DecisionError, ToolRef: alertProviderAuth + ":" + r.Credential.ID,
			Payload: domain.DecisionPayload{Title: title, Message: msg, Data: map[string]any{dataOhAlert: alertProviderAuth}}}
		dec.ID = domain.DecisionID(dec.Kind, s.ID, dec.ToolRef)
		if prev, gerr := d.opts.Decisions.Get(ctx, dec.ID); gerr == nil && prev != nil {
			continue // already raised (and maybe dismissed)
		}
		if uerr := d.opts.Decisions.Upsert(ctx, &dec); uerr != nil {
			slog.Warn("ohd: provider credential alert not recorded", "session", s.ID, "error", uerr)
			continue
		}
		d.wmu.Lock()
		w := d.watchers[s.GroupKey]
		d.wmu.Unlock()
		if w != nil {
			w.refreshAlerts(ctx, s.ID)
			continue
		}
		d.feed.publishChange(domain.SessionChange{SessionID: s.ID, GroupKey: s.GroupKey, Decisions: true})
		d.notes.requestScan()
	}
}

// authRejectedText is the title and message of the alert.
func authRejectedText(r credproxy.AuthRejection, project string, storedAt time.Time, dated bool) (title, message string) {
	name, vendor := providerLabel(r.Provider)
	title = i18n.Tf("cmd.daemon.provider_auth.title", name, vendor)
	var which string
	switch r.Credential.Scope {
	case "project":
		which = i18n.Tf("cmd.daemon.provider_auth.key_project", project, r.Credential.Key)
	case "team":
		which = i18n.Tf("cmd.daemon.provider_auth.key_team", r.Credential.Key)
	case "hub":
		which = i18n.Tf("cmd.daemon.provider_auth.key_hub", r.Credential.Key)
	default:
		p := r.Credential.Profile
		if p == "" {
			p = "default"
		}
		which = i18n.Tf("cmd.daemon.provider_auth.key_aws", p)
	}
	if dated {
		which += i18n.Tf("cmd.daemon.provider_auth.stored_at", storedAt.Local().Format("2006-01-02 15:04"))
	}
	setup := r.Provider
	if r.Provider == credproxy.ProviderBedrock {
		setup = string(provider.Bedrock)
	}
	if r.Credential.Scope == "project" {
		setup += " -p " + project
	}
	hint := i18n.Tf("cmd.daemon.provider_auth.hint", setup)
	if r.Credential.Kind == string(domain.CredentialSigV4) {
		hint = i18n.T("cmd.daemon.provider_auth.hint_aws")
	}
	return title, i18n.Tf("cmd.daemon.provider_auth.detail", which, r.Status, r.Message, hint)
}

func providerLabel(p string) (name, vendor string) {
	switch p {
	case credproxy.ProviderBedrock:
		return "Bedrock", "AWS"
	case credproxy.ProviderAnthropic:
		return "Anthropic", "Anthropic"
	case credproxy.ProviderOpenRouter:
		return "OpenRouter", "OpenRouter"
	case credproxy.ProviderOpenAI:
		return "OpenAI", "OpenAI"
	}
	return p, p
}
