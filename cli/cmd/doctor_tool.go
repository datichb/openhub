package cmd

import (
	"context"
	"errors"

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
	return views.DoctorCheck{Name: name, OK: true,
		Detail: i18n.Tf("cmd.v1.unsupported.doctor_ok", toolName(), v5Tool.Version, v5Adapter.Name(), v5Tool.MinVersion)}
}
