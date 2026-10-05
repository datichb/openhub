package provider

import (
	"context"
	"fmt"

	"github.com/datichb/openhub/cli/internal/domain"
)

// TeamKeychainKey returns the team-scoped secret key of a provider credential:
// openhub.team.<teamID>.provider.<provider>.token. Empty for providers without secret.
func TeamKeychainKey(name Name, teamID string) string {
	if teamID == "" || KeychainKey(name, "") == "" {
		return ""
	}
	return "openhub.team." + teamID + ".provider." + string(name) + ".token"
}

// ResolvedCredential is a provider credential found on the host.
type ResolvedCredential struct {
	Source  domain.CredentialSource
	Secret  string // empty for SigV4 (credentials resolved by the AWS SDK)
	Region  string
	Profile string
}

// ResolveCredentialSource walks the credential cascade for a session:
//
//	projectTokenKey (explicit project override) → project key → team key → hub key
//	→ (Bedrock only) AWS profile / default credential chain (SigV4).
//
// The returned Source never contains the secret; it is what the oh daemon
// persists to restore proxy grants.
func ResolveCredentialSource(ctx context.Context, secrets SecretStore, prov Name, projectID, teamID, projectTokenKey string, cfg *Config) (ResolvedCredential, error) {
	out := ResolvedCredential{}
	if cfg != nil {
		out.Region, out.Profile = cfg.AWSRegion, cfg.AWSProfile
	}
	kind := domain.CredentialBearer
	if prov == Anthropic {
		kind = domain.CredentialAPIKey
	}

	type candidate struct{ key, scope string }
	var candidates []candidate
	if projectTokenKey != "" {
		candidates = append(candidates, candidate{projectTokenKey, "project"})
	}
	if projectID != "" {
		candidates = append(candidates, candidate{KeychainKey(prov, projectID), "project"})
	}
	if k := TeamKeychainKey(prov, teamID); k != "" {
		candidates = append(candidates, candidate{k, "team"})
	}
	candidates = append(candidates, candidate{KeychainKey(prov, ""), "hub"})

	if secrets != nil {
		for _, c := range candidates {
			if c.key == "" {
				continue
			}
			if v, err := secrets.Get(ctx, c.key); err == nil && v != "" {
				out.Source = domain.CredentialSource{Kind: kind, KeychainKey: c.key, Scope: c.scope}
				out.Secret = v
				return out, nil
			}
		}
	}
	if prov == Bedrock {
		out.Source = domain.CredentialSource{Kind: domain.CredentialSigV4, Profile: out.Profile, Scope: "aws"}
		return out, nil
	}
	return out, fmt.Errorf("no credential found for provider %s (project %q, team %q)", prov, projectID, teamID)
}
