package cmd

import (
	"context"
	"os"

	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// securityDoctorChecks reports the local mode protections of the oh daemon
// (M12): where the issuing capability is kept, the daemon socket, and proxy
// tokens left in clear by an older oh.
func securityDoctorChecks(ctx context.Context) []views.DoctorCheck {
	var out []views.DoctorCheck
	name := i18n.T("cmd.doctor.security.capability")
	// Read only: Doctor never creates the capability (first launch does).
	switch src, found, err := daemon.PeekCapability(ctx, capabilityStore(), daemon.Paths{Dir: ohRunDir()}); {
	case err != nil:
		out = append(out, views.DoctorCheck{Name: name, OK: false, Detail: i18n.Tf("cmd.doctor.security.capability_error", err)})
	case !found:
		out = append(out, views.DoctorCheck{Name: name, OK: true, Detail: i18n.T("cmd.doctor.security.capability_none")})
	case src == daemon.CapabilityFile:
		out = append(out, views.DoctorCheck{Name: name, OK: true, Detail: i18n.Tf("cmd.doctor.security.capability_file", daemon.Paths{Dir: ohRunDir()}.CapabilityFile())})
	default:
		out = append(out, views.DoctorCheck{Name: name, OK: true, Detail: i18n.T("cmd.doctor.security.capability_keychain")})
	}

	sock := daemon.Paths{Dir: ohRunDir()}.Socket()
	if st, err := os.Stat(sock); err == nil {
		ok := st.Mode().Perm()&0o077 == 0 && ownedByMe(st)
		detail := i18n.T("cmd.doctor.security.socket_ok")
		if !ok {
			detail = i18n.Tf("cmd.doctor.security.socket_open", sock, st.Mode().Perm())
		}
		out = append(out, views.DoctorCheck{Name: i18n.T("cmd.doctor.security.socket"), OK: ok, Detail: detail})
	}

	if store != nil {
		n := 0
		for _, c := range []interface {
			CountLegacyTokens(ctx context.Context, prefix string) (int, error)
		}{sqlite.NewGrantStore(store), sqlite.NewServerStore(store)} {
			if k, err := c.CountLegacyTokens(ctx, credproxy.TokenPrefix); err == nil {
				n += k
			}
		}
		if n == 0 {
			out = append(out, views.DoctorCheck{Name: i18n.T("cmd.doctor.security.tokens"), OK: true, Detail: i18n.T("cmd.doctor.security.tokens_hashed")})
		} else {
			out = append(out, views.DoctorCheck{Name: i18n.T("cmd.doctor.security.tokens"), OK: false, Detail: i18n.Tf("cmd.doctor.security.tokens_clear", n)})
		}
	}
	return out
}
