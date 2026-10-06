package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/semver"
	"github.com/datichb/openhub/cli/internal/termlaunch"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

func init() {
	views.ExtraDoctorChecks = v5DoctorChecks
}

// v5DoctorChecks reports the v5 runtime health (shared by `oh doctor` and the TUI).
func v5DoctorChecks() []views.DoctorCheck {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var out []views.DoctorCheck

	out = append(out, opencodeV2Check(ctx))

	out = append(out, workflowIntegrityChecks()...)
	out = append(out, deployLeftoversCheck(ctx))
	out = append(out, remoteDoctorChecks()...)

	if !v5Available(ctx) {
		// No supported opencode: the runtime checks below do not apply.
		return out
	}

	dc := daemon.NewClient(daemon.Paths{Dir: ohRunDir()})
	if h, err := dc.Health(ctx); err == nil {
		detail := i18n.Tf("cmd.doctor.v5.daemon_ok", h.Version, h.Servers, h.Grants)
		ok := h.PendingGrants == 0
		if !ok {
			detail += " · " + i18n.Tf("cmd.doctor.v5.daemon_pending", h.PendingGrants)
		}
		out = append(out, views.DoctorCheck{Name: i18n.T("cmd.doctor.v5.daemon"), OK: ok, Detail: detail})
	} else {
		out = append(out, views.DoctorCheck{Name: i18n.T("cmd.doctor.v5.daemon"), OK: true, Detail: i18n.T("cmd.doctor.v5.daemon_off")})
	}

	out = append(out, securityDoctorChecks(ctx)...)
	out = append(out, limitsDoctorChecks(ctx)...)
	out = append(out, inProcessDoctorChecks()...)

	// Informational until containers (phase 4) need --relative-paths.
	_, gitDetail := gitRelativeWorktrees()
	out = append(out, views.DoctorCheck{Name: i18n.T("cmd.doctor.v5.git"), OK: true, Detail: gitDetail})

	pref := termlaunch.PrefAuto
	if app := TryApp(); app != nil && app.Config.Session.Attach != "" {
		pref = termlaunch.Pref(app.Config.Session.Attach)
	}
	var methods []string
	for _, m := range termlaunch.Chain(pref) {
		methods = append(methods, string(m))
	}
	if len(methods) == 0 {
		out = append(out, views.DoctorCheck{Name: i18n.T("cmd.doctor.v5.terminal"), OK: false, Detail: i18n.T("cmd.doctor.v5.terminal_none")})
	} else {
		out = append(out, views.DoctorCheck{Name: i18n.T("cmd.doctor.v5.terminal"), OK: true, Detail: strings.Join(methods, " → ")})
	}
	for _, fn := range doctorChecks {
		cctx, cancel := context.WithTimeout(context.Background(), doctorCheckTimeout)
		out = append(out, fn(cctx)...)
		cancel()
	}
	return out
}

// doctorChecks are the v5 checks registered by their own file (one check per
// file: container, gateways…), run after the checks above with opencode V2.
var doctorChecks []func(ctx context.Context) []views.DoctorCheck

// doctorCheckTimeout bounds each registered check (short containers, first
// pull of the probe image).
const doctorCheckTimeout = 90 * time.Second

// registerDoctorCheck adds a v5 check (called from init).
func registerDoctorCheck(fn func(ctx context.Context) []views.DoctorCheck) {
	doctorChecks = append(doctorChecks, fn)
}

// gitRelativeWorktrees checks git ≥ 2.48 (`git worktree add --relative-paths`,
// needed to mount worktrees in containers).
func gitRelativeWorktrees() (ok bool, detail string) {
	out, err := exec.Command("git", "--version").Output()
	if err != nil {
		return false, err.Error()
	}
	fields := strings.Fields(string(out))
	if len(fields) < 3 {
		return false, strings.TrimSpace(string(out))
	}
	v := fields[2]
	if semver.Parse(v).LessThan(semver.Parse("2.48.0")) {
		return false, i18n.Tf("cmd.doctor.v5.git_old", v)
	}
	return true, fmt.Sprintf("git %s", v)
}
