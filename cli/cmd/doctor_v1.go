package cmd

import (
	"context"
	"errors"

	"github.com/datichb/openhub/cli/internal/adapters/opencodev2"
	"github.com/datichb/openhub/cli/internal/buildinfo"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// opencodeV2Check reports whether the installed opencode is a supported V2
// release (opencode V1 is no longer supported in oh v5, P3-T30).
func opencodeV2Check(ctx context.Context) views.DoctorCheck {
	name := i18n.T("cmd.v1.unsupported.doctor_name")
	if err := requireV2(ctx); err != nil {
		var ue *opencodev2.UnsupportedError
		detail := i18n.T("cmd.v1.unsupported.missing")
		if errors.As(v5Err, &ue) {
			detail = i18n.Tf("cmd.v1.unsupported.version", ue.Found, ue.Min)
		}
		return views.DoctorCheck{Name: name, OK: false, Detail: detail + " " + i18n.T("cmd.v1.unsupported.hint")}
	}
	r := opencodev2.SupportedRange(buildinfo.Version)
	return views.DoctorCheck{Name: name, OK: true,
		Detail: i18n.Tf("cmd.v1.unsupported.doctor_ok", v5Adapter.Ver, v5Adapter.Name(), r.OpencodeMin)}
}
