package provider

import (
	"context"
	"errors"
	"fmt"
	"os"

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

// ErrNoCredential reports that no key of the cascade holds a credential.
var ErrNoCredential = errors.New("no credential found")

// Bedrock authentication modes (Config.AuthMode). Empty = keychain cascade,
// then the AWS profile / default chain.
const (
	AuthBearer  = "bearer"  // Bedrock API key from the keychain cascade only
	AuthProfile = "profile" // SigV4 with the AWS profile (stored keys ignored)
	AuthEnv     = "env"     // SigV4 with the AWS default chain (stored keys ignored)
)

// ErrBearerNotStored reports a Bedrock project in bearer mode without any
// stored API key (it also matches ErrNoCredential).
var ErrBearerNotStored = fmt.Errorf("%w (auth_mode bearer)", ErrNoCredential)

// EnvBearerToken is the Bedrock API key of the user's shell
// (AWS_BEARER_TOKEN_BEDROCK), which oh never passes to the tool: it can only
// be imported into the keychain (oh provider setup bedrock --from-env).
func EnvBearerToken() string { return os.Getenv(EnvVar(Bedrock)) }

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
// For Bedrock, cfg.AuthMode narrows it: bearer = stored keys only
// (ErrBearerNotStored without one), profile/env = SigV4 only.
//
// The returned Source never contains the secret; it is what the oh daemon
// persists to restore proxy grants.
func ResolveCredentialSource(ctx context.Context, secrets SecretStore, prov Name, projectID, teamID, projectTokenKey string, cfg *Config) (ResolvedCredential, error) {
	out := ResolvedCredential{}
	mode := ""
	if cfg != nil {
		out.Region, out.Profile = cfg.AWSRegion, cfg.AWSProfile
		if prov == Bedrock {
			mode = cfg.AuthMode
		}
	}
	if mode == AuthProfile || mode == AuthEnv {
		secrets = nil // SigV4 chosen: a stored key is never used
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
			v, err := secrets.Get(ctx, c.key)
			if errors.Is(err, domain.ErrNotFound) {
				continue
			}
			if err != nil {
				// Locked keychain, access denied...: never fall back silently
				// to another key (it would bill another account).
				return out, fmt.Errorf("reading %s credential %q: %w", c.scope, c.key, err)
			}
			if v != "" {
				out.Source = domain.CredentialSource{Kind: kind, KeychainKey: c.key, Scope: c.scope}
				out.Secret = v
				return out, nil
			}
		}
	}
	if prov == Bedrock {
		if mode == AuthBearer {
			return out, fmt.Errorf("%w for provider %s (project %q, team %q)", ErrBearerNotStored, prov, projectID, teamID)
		}
		out.Source = domain.CredentialSource{Kind: domain.CredentialSigV4, Profile: out.Profile, Scope: "aws"}
		return out, nil
	}
	return out, fmt.Errorf("%w for provider %s (project %q, team %q)", ErrNoCredential, prov, projectID, teamID)
}
