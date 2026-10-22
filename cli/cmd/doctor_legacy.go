package cmd

import (
	"context"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Doctor › former keys (A-08). Three things still work in v5 and stop in v6:
// the former keys of hub.toml, the former keychain names of the MCP tokens,
// and, as a reminder, the keys no setting uses (ignored, and lost at the
// next write of hub.toml).

// legacyKeysChecks reports the former keys hub.toml still holds, the keys
// it holds that nothing reads, and the MCP tokens kept under a former
// keychain name only.
func legacyKeysChecks(ctx context.Context) []views.DoctorCheck {
	var secrets domain.SecretStore
	if a := TryApp(); a != nil {
		secrets = a.Secrets
	}
	keychain := legacyKeychainCheck(ctx, secrets)
	notes, err := config.Inspect()
	if err != nil {
		// A hub.toml that does not load is reported by its own check.
		return []views.DoctorCheck{keychain}
	}
	return []views.DoctorCheck{legacyConfigCheck(notes), unknownKeysCheck(notes), keychain}
}

func legacyConfigCheck(notes []config.Notice) views.DoctorCheck {
	name := i18n.T("cmd.doctor.legacy.name")
	var found []string
	for _, n := range notes {
		if n.Kind != config.NoticeLegacyKey {
			continue
		}
		found = append(found, i18n.Tf("cmd.doctor.legacy.key_item", n.Key, n.Replacement, noticeWhere(n)))
	}
	if len(found) == 0 {
		return views.DoctorCheck{Name: name, OK: true, Detail: i18n.T("cmd.doctor.legacy.none")}
	}
	return views.DoctorCheck{Name: name, OK: true, Warn: true,
		Detail: i18n.Tf("cmd.doctor.legacy.keys", strings.Join(found, " · "), config.LegacySupportEnds)}
}

func unknownKeysCheck(notes []config.Notice) views.DoctorCheck {
	name := i18n.T("cmd.doctor.legacy.unknown_name")
	var found []string
	for _, n := range notes {
		if n.Kind == config.NoticeUnknownKey {
			found = append(found, i18n.Tf("cmd.doctor.legacy.unknown_item", n.Key, noticeWhere(n)))
		}
	}
	if len(found) == 0 {
		return views.DoctorCheck{Name: name, OK: true, Detail: i18n.T("cmd.doctor.legacy.unknown_none")}
	}
	return views.DoctorCheck{Name: name, OK: true, Warn: true,
		Detail: i18n.Tf("cmd.doctor.legacy.unknown", strings.Join(found, " · "))}
}

// legacyTokenServices are the services whose hub token had a former keychain
// name (<service>-token).
var legacyTokenServices = []string{"figma", "github", "gitlab", "gslides", "jira", "linear"}

// legacyKeychainCheck lists the MCP tokens that exist under the former
// keychain name only: they are still found, until v6. A token under both
// names is fine (the former entry is just left behind). The keychain is only
// read.
func legacyKeychainCheck(ctx context.Context, secrets domain.SecretStore) views.DoctorCheck {
	name := i18n.T("cmd.doctor.legacy.keychain_name")
	if secrets == nil {
		return views.DoctorCheck{Name: name, OK: true, Detail: i18n.T("cmd.doctor.legacy.keychain_unavailable")}
	}
	var found []string
	for _, svc := range legacyTokenServices {
		old, current := config.LegacyMCPTokenKey(svc), config.MCPTokenKey(svc)
		if v, _ := secrets.Get(ctx, old); v == "" {
			continue
		}
		if v, _ := secrets.Get(ctx, current); v != "" {
			continue
		}
		found = append(found, i18n.Tf("cmd.doctor.legacy.keychain_item", old, current))
	}
	sort.Strings(found)
	if len(found) == 0 {
		return views.DoctorCheck{Name: name, OK: true, Detail: i18n.T("cmd.doctor.legacy.keychain_none")}
	}
	return views.DoctorCheck{Name: name, OK: true, Warn: true,
		Detail: i18n.Tf("cmd.doctor.legacy.keychain", strings.Join(found, " · "), config.LegacySupportEnds)}
}
