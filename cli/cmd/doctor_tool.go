package cmd

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// toolCheck reports whether the installed tool is a supported release (the
// V1 of the tool is no longer supported in oh v5, P3-T30).
func toolCheck(ctx context.Context) views.DoctorCheck {
	err := requireV2(ctx)
	name := i18n.Tf("cmd.v1.unsupported.doctor_name", toolName())
	if err != nil {
		var ue *adapters.UnsupportedVersionError
		detail := i18n.Tf("cmd.v1.unsupported.missing", toolName())
		if errors.As(v5Err, &ue) {
			detail = i18n.Tf("cmd.v1.unsupported.version", toolName(), ue.Found, ue.Min)
		}
		return views.DoctorCheck{Name: name, OK: false, Detail: detail + " " + i18n.Tf("cmd.v1.unsupported.hint", toolName())}
	}
	check := views.DoctorCheck{Name: name, OK: true,
		Detail: i18n.Tf("cmd.v1.unsupported.doctor_ok", toolName(), v5Tool.Version, v5Adapter.Name(), v5Tool.MinVersion)}
	if warning := unpinnedToolWarning(v5Tool.Version); warning != "" {
		check.Warn = true
		check.Detail += " · " + warning
	}
	return check
}

// unpinnedToolWarning says that the integrity of the Linux packages of this
// tool version is not pinned in oh (audit v5, S-38): container images download
// them with an integrity that comes from the registry alone, and
// execution.strict_isolation refuses them. "" when the version is pinned.
func unpinnedToolWarning(version string) string {
	pinned, recommended := toolPins()
	if len(pinned) == 0 {
		return ""
	}
	version = strings.TrimPrefix(version, "v")
	if slices.Contains(pinned, version) {
		return ""
	}
	return i18n.Tf("cmd.doctor.pin.unpinned", version, strings.Join(pinned, ", "), recommended)
}
