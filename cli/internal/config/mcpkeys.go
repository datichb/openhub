package config

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
)

// Keychain key names of the MCP service tokens. Every writer (CLI and TUI)
// goes through these functions, so that the name written is always the
// current one:
//
//	hub:     openhub.mcp.<service>.token        (legacy: <service>-token)
//	project: openhub.mcp.<service>.token.<id>   (legacy: <service>-token-<id>)

// MCPTokenKey is the current keychain key of the hub token of a service.
func MCPTokenKey(service string) string { return DefaultTokenKeyForService(service) }

// MCPProjectTokenKey is the current keychain key of the token a project
// keeps for a service.
func MCPProjectTokenKey(service, projectID string) string {
	return MCPTokenKey(service) + "." + projectID
}

// LegacyMCPTokenKey is the former keychain key of the hub token of a service.
func LegacyMCPTokenKey(service string) string { return service + "-token" }

// LegacyMCPProjectTokenKey is the former keychain key of a project token.
func LegacyMCPProjectTokenKey(service, projectID string) string {
	return LegacyMCPTokenKey(service) + "-" + projectID
}

// MCPTokenKeyCandidates lists the keychain keys to read, in order, for the
// token designated by key: the current name first, then the legacy one. A key
// that follows neither convention (a name chosen by the user) is returned
// alone. An empty key designates the hub token of the service.
func MCPTokenKeyCandidates(service, key string) []string {
	hubCurrent, hubLegacy := MCPTokenKey(service), LegacyMCPTokenKey(service)
	switch {
	case key == "" || key == hubCurrent || key == hubLegacy:
		return []string{hubCurrent, hubLegacy}
	case strings.HasPrefix(key, hubCurrent+"."):
		id := strings.TrimPrefix(key, hubCurrent+".")
		return []string{MCPProjectTokenKey(service, id), LegacyMCPProjectTokenKey(service, id)}
	case strings.HasPrefix(key, hubLegacy+"-"):
		id := strings.TrimPrefix(key, hubLegacy+"-")
		return []string{MCPProjectTokenKey(service, id), LegacyMCPProjectTokenKey(service, id)}
	}
	return []string{key}
}

// SecretReadWriter is the part of a secret store the token readers need.
type SecretReadWriter interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

// ReadMCPToken reads the token designated by key (see MCPTokenKeyCandidates):
// the current name, then the legacy one. A value found only under the legacy
// name is copied under the current name; the legacy key is kept. The copy is
// best effort: a failure does not hide the value. The error is only set when
// nothing was found and a keychain read failed.
func ReadMCPToken(ctx context.Context, store SecretReadWriter, service, key string) (string, error) {
	if store == nil {
		return "", nil
	}
	candidates := MCPTokenKeyCandidates(service, key)
	var firstErr error
	for i, k := range candidates {
		val, err := store.Get(ctx, k)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if val == "" {
			continue
		}
		if i > 0 {
			if err := store.Set(ctx, candidates[0], val); err != nil {
				slog.Debug("keychain: could not copy the legacy token key", "from", k, "to", candidates[0], "error", err)
			}
		}
		return val, nil
	}
	return "", firstErr
}

// StoreMCPToken writes the token of a service in the keychain under the
// current key name and returns that name. An empty projectID stores the hub
// token; otherwise the token private to that project. It is the single
// writer of MCP tokens (CLI and TUI).
func StoreMCPToken(ctx context.Context, store SecretReadWriter, service, projectID, token string) (string, error) {
	key := MCPTokenKey(service)
	if projectID != "" {
		key = MCPProjectTokenKey(service, projectID)
	}
	if store == nil {
		return key, errors.New("no secret store available")
	}
	if err := store.Set(ctx, key, token); err != nil {
		return key, err
	}
	return key, nil
}

// ProjectTokenKeys lists the keychain keys that can hold the token a project
// keeps for a service: the key recorded in the project, the current name and
// the legacy one, without duplicates.
func ProjectTokenKeys(service, projectID, recorded string) []string {
	var keys []string
	for _, k := range []string{recorded, MCPProjectTokenKey(service, projectID), LegacyMCPProjectTokenKey(service, projectID)} {
		if k != "" && !slices.Contains(keys, k) {
			keys = append(keys, k)
		}
	}
	return keys
}
