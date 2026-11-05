package cmd

import (
	"context"
	"strings"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/notify"
	"github.com/datichb/openhub/cli/internal/services/doctor"
	"github.com/datichb/openhub/cli/internal/sysnotify"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// Doctor › the notifications: the backend of the system notifications (when
// they are on), the messages that could not reach a webhook and wait for
// another attempt, and the servers of the webhooks of the teams (reachable, not
// tested with a message: the webhook is not written to).

const doctorIDNotifications = "notifications"

// reachWebhook checks that the server of a webhook answers (a variable for the tests).
var reachWebhook = notify.Reachable

func notificationsDoctorChecks(ctx context.Context) []doctor.Result {
	a := TryApp()
	if a == nil || a.Config == nil {
		return nil
	}
	var out []doctor.Result
	out = append(out, systemBackendResult(a.Config))
	if r := outboxResult(); r != nil {
		out = append(out, *r)
	}
	if !doctorOffline() {
		out = append(out, webhookResults(ctx, a.Config)...)
	}
	return out
}

func systemBackendResult(cfg *config.Config) doctor.Result {
	name := i18n.T("v54n.doctor.system.name")
	if !cfg.NotifyPolicy().SystemOn() {
		return doctor.Result{ID: doctorIDNotifications, Name: name, OK: true, Detail: i18n.T("v54n.doctor.system.off")}
	}
	if systemNotifier().Command(sysnotify.Note{Title: "oh", Message: "oh"}) == nil {
		return doctor.Result{ID: doctorIDNotifications, Name: name, OK: true, Warn: true, Detail: i18n.T("v54n.doctor.system.none")}
	}
	return doctor.Result{ID: doctorIDNotifications, Name: name, OK: true, Detail: i18n.T("v54n.doctor.system.ok")}
}

func outboxResult() *doctor.Result {
	n := notifyOutbox().Pending()
	if n == 0 {
		return nil
	}
	return &doctor.Result{ID: doctorIDNotifications, Name: i18n.T("v54n.doctor.outbox.name"), OK: true, Warn: true,
		Detail: i18n.Tf("v54n.doctor.outbox.detail", n)}
}

func webhookResults(ctx context.Context, cfg *config.Config) []doctor.Result {
	var out []doctor.Result
	for _, t := range teamConfigTargets(cfg) {
		tc, err := t.repo.LoadConfig(ctx)
		if err != nil || tc == nil || !tc.Notification.Enabled {
			continue
		}
		urls := map[string]bool{}
		if u := tc.Notification.EffectiveURL(); u != "" && len(tc.Notification.Destinations) == 0 {
			urls[u] = true
		}
		for _, d := range tc.Notification.Destinations {
			if d.WebhookURL != "" {
				urls[d.WebhookURL] = true
			}
		}
		var failed []string
		for u := range urls {
			cctx, cancel := context.WithTimeout(ctx, doctorConnectionTimeout)
			if err := reachWebhook(cctx, u); err != nil {
				failed = append(failed, teamstate.MaskWebhook(u)+" : "+err.Error())
			}
			cancel()
		}
		if len(urls) == 0 {
			continue
		}
		name := i18n.Tf("v54n.doctor.webhook.name", firstNonEmpty(t.team.ID, "hub"))
		if len(failed) > 0 {
			out = append(out, doctor.Result{ID: doctorIDNotifications, Name: name, OK: true, Warn: true,
				Detail: i18n.T("v54n.doctor.webhook.unreachable") + "\n  - " + strings.Join(failed, "\n  - ")})
			continue
		}
		out = append(out, doctor.Result{ID: doctorIDNotifications, Name: name, OK: true, Detail: i18n.Tf("v54n.doctor.webhook.ok", len(urls))})
	}
	return out
}
