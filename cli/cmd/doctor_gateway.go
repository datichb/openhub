package cmd

import (
	"context"
	"errors"
	"net/http"
	"os/exec"

	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/runtime/container"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Doctor › gateways (P4-T12): bd on the machine, daemon recent enough, and
// the Beads and MCP gateway routes reached from a container.

func init() { registerDoctorCheck(gatewayDoctorChecks) }

// Gateway routes probed from a container (POST without token: any answer
// but 404 proves the route is served on the address containers use).
const (
	gatewayBeadsProbePath = "/oh-gateway/beads/v1/exec"
	gatewayMCPProbePath   = "/oh/v1/hooks/mcp/team"
)

func gatewayDoctorChecks(ctx context.Context) []views.DoctorCheck {
	check := func(key string, ok bool, detail string) views.DoctorCheck {
		return views.DoctorCheck{Name: i18n.T("cmd.doctor.gateway." + key), OK: ok, Detail: detail}
	}
	var out []views.DoctorCheck
	if p, err := exec.LookPath("bd"); err == nil {
		out = append(out, check("bd", true, p))
	} else {
		out = append(out, check("bd", false, i18n.T("cmd.doctor.gateway.bd_missing")))
	}

	dc := daemon.NewClient(daemon.Paths{Dir: ohRunDir()})
	h, err := dc.Health(ctx)
	if err != nil {
		return append(out, check("daemon", true, i18n.T("cmd.doctor.gateway.daemon_off")))
	}
	// An empty grant request is refused (400) by a daemon serving the
	// gateway, and unknown (404) to an older one: nothing is issued.
	_, gerr := dc.IssueGatewayGrant(ctx, daemon.GatewayGrantRequest{})
	var apiErr *daemon.APIError
	if errors.As(gerr, &apiErr) && apiErr.Status == http.StatusNotFound {
		return append(out, check("daemon", false, i18n.T("cmd.gateway.beads.daemon_outdated")))
	}
	out = append(out, check("daemon", true, i18n.Tf("cmd.doctor.gateway.daemon_ok", h.Version)))

	rt := v5ContainerRuntime(TryApp())
	e, av := rt.Engine(ctx)
	if !av.OK || h.ProxyURL == "" {
		return append(out, check("reach", true, i18n.T("cmd.doctor.gateway.no_engine")))
	}
	base := container.URLHost(h.ProxyURL, e.Host)
	urls := []string{base + gatewayBeadsProbePath, base + gatewayMCPProbePath}
	images := []string{}
	if list, err := rt.ProjectImages(ctx, e); err == nil && len(list) > 0 {
		images = append(images, list[0].Ref)
	}
	images = append(images, container.ProbeImage)
	codes, img, err := rt.ProbeHTTP(ctx, e, images, urls)
	if err != nil {
		return append(out, check("reach", false, i18n.Tf("tui.doctor.container.probe_failed", err.Error())))
	}
	for i, key := range []string{"beads", "mcp"} {
		out = append(out, gatewayReachCheck(check, key, urls[i], codes[urls[i]], img, !e.VM))
	}
	return out
}

// gatewayReachCheck turns the HTTP status of a probe into a check.
func gatewayReachCheck(check func(string, bool, string) views.DoctorCheck, key, url string, code int, image string, linux bool) views.DoctorCheck {
	switch {
	case code == 0 && linux:
		return check(key, false, i18n.Tf("cmd.doctor.gateway.unreachable_linux", url))
	case code == 0:
		return check(key, false, i18n.Tf("cmd.doctor.gateway.unreachable", url))
	case code == http.StatusNotFound:
		return check(key, false, i18n.Tf("cmd.doctor.gateway.route_missing", url))
	}
	return check(key, true, i18n.Tf("cmd.doctor.gateway.reachable", url, code, image))
}
