package teamstate

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// ConfigIssueKind classes what ConfigIssues finds in a config.toml.
type ConfigIssueKind string

const (
	// IssueUnreadable: the file does not parse; nothing else can be said.
	IssueUnreadable ConfigIssueKind = "unreadable"
	// IssueUnknownKey: a key this oh does not know (a newer oh, a typo).
	IssueUnknownKey ConfigIssueKind = "unknown_key"
	// IssueDeprecated: a former key (DeprecatedKeys), repaired by RepairConfig.
	IssueDeprecated ConfigIssueKind = "deprecated"
	// IssueOutOfBounds: a number outside its bounds, brought back by RepairConfig.
	IssueOutOfBounds ConfigIssueKind = "out_of_bounds"
	// IssueBoard: the columns of the board are not valid.
	IssueBoard ConfigIssueKind = "board"
	// IssueNotification: the notifications are not coherent.
	IssueNotification ConfigIssueKind = "notification"
)

// ConfigIssue is one finding of ConfigIssues.
type ConfigIssue struct {
	Kind ConfigIssueKind
	// Key is the setting concerned (dotted, "" for the file).
	Key string
	// Detail is the finding, translated.
	Detail string
	// Fixable: RepairConfig repairs it.
	Fixable bool
}

// ConfigIssues reads config.toml as it is written and reports what is wrong or
// outdated in it: unknown and former keys, numbers out of their bounds, a
// board that is not valid, notifications that cannot work. A team without
// config.toml has none.
func (r *Repo) ConfigIssues(ctx context.Context) ([]ConfigIssue, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(r.path, "config.toml"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return configIssues(data), nil
}

func configIssues(data []byte) []ConfigIssue {
	var raw TeamConfig
	if err := toml.Unmarshal(data, &raw); err != nil {
		return []ConfigIssue{{Kind: IssueUnreadable, Detail: err.Error()}}
	}
	var out []ConfigIssue
	deprecated := map[string]bool{}
	for _, k := range DeprecatedKeys {
		deprecated[k] = true
	}
	for _, k := range UnknownKeys(data) {
		if deprecated[k] {
			out = append(out, ConfigIssue{Kind: IssueDeprecated, Key: k, Detail: i18n.Tf("v54c.check.deprecated_"+strings.ReplaceAll(k, ".", "_"), k), Fixable: true})
			continue
		}
		out = append(out, ConfigIssue{Kind: IssueUnknownKey, Key: k, Detail: i18n.Tf("v54c.check.unknown_key", k)})
	}
	if raw.Notification.UsesFormerWebhookKey() {
		out = append(out, ConfigIssue{Kind: IssueDeprecated, Key: "notification.mattermost_webhook",
			Detail: i18n.T("v54c.check.deprecated_notification_mattermost_webhook"), Fixable: true})
	}
	if raw.Tracker.UsesFormerInProgressKey() {
		out = append(out, ConfigIssue{Kind: IssueDeprecated, Key: "tracker.max_auto_plan_per_member",
			Detail: i18n.Tf("v54c.check.deprecated_tracker_max_auto_plan_per_member", raw.Tracker.MaxAutoPlanPerMember), Fixable: true})
	}
	out = append(out, boundsIssues(&raw)...)
	if err := raw.Board.Validate(); err != nil {
		out = append(out, ConfigIssue{Kind: IssueBoard, Key: "board", Detail: err.Error()})
	}
	out = append(out, notificationIssues(&raw.Notification)...)
	return out
}

// bound is a number of config.toml with the range it must be in; 0 stands for
// "the default" where zeroOK.
type bound struct {
	key    string
	get    func(*TeamConfig) int
	set    func(*TeamConfig, int)
	lo, hi int
	zeroOK bool
}

var configBounds = []bound{
	{"takeover.stale_days", func(c *TeamConfig) int { return c.Takeover.StaleDays }, func(c *TeamConfig, n int) { c.Takeover.StaleDays = n }, MinStaleDays, MaxStaleDays, true},
	{"claim.done_retention_days", func(c *TeamConfig) int { return c.Claim.DoneRetentionDays }, func(c *TeamConfig, n int) { c.Claim.DoneRetentionDays = n }, MinDoneRetentionDays, MaxDoneRetentionDays, true},
	{"claim.lease_hours", func(c *TeamConfig) int { return c.Claim.LeaseHours }, func(c *TeamConfig, n int) { c.Claim.LeaseHours = n }, MinLeaseHours, MaxLeaseHours, true},
	{"claim.lease_hours_manual", func(c *TeamConfig) int { return c.Claim.LeaseHoursManual }, func(c *TeamConfig, n int) { c.Claim.LeaseHoursManual = n }, MinLeaseHours, MaxLeaseHoursManual, true},
	{"claim.sleeping_renewal_days", func(c *TeamConfig) int { return c.Claim.SleepingRenewalDays }, func(c *TeamConfig, n int) { c.Claim.SleepingRenewalDays = n }, MinRenewalDays, MaxRenewalDays, true},
	{"claim.remote_renewal_days", func(c *TeamConfig) int { return c.Claim.RemoteRenewalDays }, func(c *TeamConfig, n int) { c.Claim.RemoteRenewalDays = n }, MinRenewalDays, MaxRenewalDays, true},
	{"tracker.sync_interval_minutes", func(c *TeamConfig) int { return c.Tracker.SyncIntervalMinutes }, func(c *TeamConfig, n int) { c.Tracker.SyncIntervalMinutes = n }, 0, MaxSyncIntervalMinute, true},
	{"tracker.max_unassigned_issues", func(c *TeamConfig) int { return c.Tracker.MaxUnassignedIssues }, func(c *TeamConfig, n int) { c.Tracker.MaxUnassignedIssues = n }, MinUnassignedIssues, MaxUnassignedIssues, true},
}

func boundsIssues(raw *TeamConfig) []ConfigIssue {
	var out []ConfigIssue
	for _, b := range configBounds {
		v := b.get(raw)
		if (v == 0 && b.zeroOK) || (v >= b.lo && v <= b.hi) {
			continue
		}
		out = append(out, ConfigIssue{Kind: IssueOutOfBounds, Key: b.key, Fixable: true,
			Detail: i18n.Tf("v54c.check.out_of_bounds", b.key, v, b.lo, b.hi, clamp(v, b.lo, b.hi))})
	}
	if n := raw.Tracker.MaxInProgressPerMember; n != nil && (*n < 0 || *n > MaxInProgressLimit) {
		out = append(out, ConfigIssue{Kind: IssueOutOfBounds, Key: "tracker.max_in_progress_per_member", Fixable: true,
			Detail: i18n.Tf("v54c.check.out_of_bounds", "tracker.max_in_progress_per_member", *n, 0, MaxInProgressLimit, clamp(*n, 0, MaxInProgressLimit))})
	}
	if h := raw.Claim.HeartbeatMinutes; h != 0 && (h < MinHeartbeatMinutes || h > MaxHeartbeatMinutes) {
		out = append(out, ConfigIssue{Kind: IssueOutOfBounds, Key: "claim.heartbeat_minutes", Fixable: true,
			Detail: i18n.Tf("v54c.check.out_of_bounds", "claim.heartbeat_minutes", h, MinHeartbeatMinutes, MaxHeartbeatMinutes, clamp(h, MinHeartbeatMinutes, MaxHeartbeatMinutes))})
	}
	return out
}

func clamp(v, lo, hi int) int { return min(max(v, lo), hi) }

func notificationIssues(n *NotificationConfig) []ConfigIssue {
	var out []ConfigIssue
	hasTarget := n.EffectiveURL() != "" || len(n.Destinations) > 0
	switch {
	case n.Enabled && !hasTarget:
		out = append(out, ConfigIssue{Kind: IssueNotification, Key: "notification.enabled", Detail: i18n.T("v54c.check.notif_no_target")})
	case !n.Enabled && hasTarget:
		out = append(out, ConfigIssue{Kind: IssueNotification, Key: "notification.enabled", Detail: i18n.T("v54c.check.notif_disabled")})
	}
	if !validNotifierType(n.Type) {
		out = append(out, ConfigIssue{Kind: IssueNotification, Key: "notification.type", Detail: i18n.Tf("v54c.check.notif_type", n.Type, strings.Join(NotifierTypes, ", "))})
	}
	for _, p := range n.Events {
		if !ValidEventPattern(strings.TrimSpace(p)) {
			out = append(out, ConfigIssue{Kind: IssueNotification, Key: "notification.events", Detail: i18n.Tf("v54n.config.unknown_event", p, strings.Join(NotificationEvents, ", "))})
		}
	}
	for i, d := range n.Destinations {
		for _, p := range d.Events {
			if !ValidEventPattern(strings.TrimSpace(p)) {
				out = append(out, ConfigIssue{Kind: IssueNotification, Key: fmt.Sprintf("notification.destinations[%d].events", i), Detail: i18n.Tf("v54n.config.unknown_event", p, strings.Join(NotificationEvents, ", "))})
			}
		}
		switch {
		case d.WebhookURL == "":
			out = append(out, ConfigIssue{Kind: IssueNotification, Key: fmt.Sprintf("notification.destinations[%d]", i), Detail: i18n.Tf("v54c.check.notif_dest_empty", i+1)})
		case d.Type == "" || !validNotifierType(d.Type):
			out = append(out, ConfigIssue{Kind: IssueNotification, Key: fmt.Sprintf("notification.destinations[%d].type", i), Detail: i18n.Tf("v54c.check.notif_dest_type", i+1, strings.Join(NotifierTypes, ", "))})
		}
	}
	return out
}

// FixableIssues are the findings RepairConfig repairs.
func FixableIssues(issues []ConfigIssue) []ConfigIssue {
	var out []ConfigIssue
	for _, i := range issues {
		if i.Fixable {
			out = append(out, i)
		}
	}
	return out
}

// RepairConfig repairs the findings of ConfigIssues that can be: the former
// keys are migrated or dropped ([parallel] and type_enforced are dropped,
// mattermost_webhook becomes webhook_url and type, max_auto_plan_per_member
// becomes max_in_progress_per_member), the numbers out of their bounds are
// brought back to the nearest one. One commit and one config_changed event
// signed by the member of ctx (WithActor); it returns the keys it repaired
// and writes nothing when there is nothing to repair. The unknown keys and the
// board are left for the team.
func (r *Repo) RepairConfig(ctx context.Context) (repaired []string, err error) {
	by := ActorOf(ctx)
	err = r.Transact(ctx, func(_ context.Context, tx *Tx) (TxResult, error) {
		repaired = nil
		data, rerr := tx.ReadFile("config.toml")
		if rerr != nil {
			if os.IsNotExist(rerr) {
				return TxResult{}, nil
			}
			return TxResult{}, rerr
		}
		var raw TeamConfig
		if perr := toml.Unmarshal(data, &raw); perr != nil {
			return TxResult{}, fmt.Errorf("parsing config.toml: %w", perr)
		}
		cfg, cerr := tx.Config()
		if cerr != nil {
			return TxResult{}, cerr
		}
		for _, i := range FixableIssues(configIssues(data)) {
			repaired = append(repaired, i.Key)
		}
		if len(repaired) == 0 {
			return TxResult{}, nil
		}
		for _, b := range configBounds {
			if v := b.get(&raw); (v != 0 || !b.zeroOK) && (v < b.lo || v > b.hi) {
				b.set(cfg, clamp(v, b.lo, b.hi))
			}
		}
		if n := raw.Tracker.MaxInProgressPerMember; n != nil && (*n < 0 || *n > MaxInProgressLimit) {
			v := clamp(*n, 0, MaxInProgressLimit)
			cfg.Tracker.MaxInProgressPerMember = &v
		}
		if h := raw.Claim.HeartbeatMinutes; h != 0 && (h < MinHeartbeatMinutes || h > MaxHeartbeatMinutes) {
			cfg.Claim.HeartbeatMinutes = clamp(h, MinHeartbeatMinutes, MaxHeartbeatMinutes)
		}
		cfg.Claim.normalize()
		if raw.Notification.UsesFormerWebhookKey() {
			cfg.Notification.WebhookURL = raw.Notification.MattermostWebhook
			cfg.Notification.Type = raw.Notification.EffectiveType()
			cfg.Notification.MattermostWebhook = ""
		}
		kept, derr := dropKeys(data, "parallel", "tracker.type_enforced")
		if derr != nil {
			return TxResult{}, derr
		}
		out, merr := marshalConfig(cfg, kept)
		if merr != nil {
			return TxResult{}, merr
		}
		if werr := tx.WriteFile("config.toml", out); werr != nil {
			return TxResult{}, werr
		}
		sort.Strings(repaired)
		if aerr := tx.AppendEvent(Event{Actor: by, Type: EventConfigChanged, Project: TeamEventsProject,
			Data: map[string]any{"keys": repaired, "repair": true}}); aerr != nil {
			return TxResult{}, aerr
		}
		return TxResult{Message: "config: repair team config"}, nil
	})
	return repaired, err
}

// dropKeys removes the dotted keys from a TOML document (a missing key is
// fine); the document is returned unchanged when none is there.
func dropKeys(data []byte, keys ...string) ([]byte, error) {
	var m map[string]any
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing config.toml: %w", err)
	}
	changed := false
	for _, k := range keys {
		path := strings.Split(k, ".")
		cur := m
		for _, p := range path[:len(path)-1] {
			next, ok := cur[p].(map[string]any)
			if !ok {
				cur = nil
				break
			}
			cur = next
		}
		if cur == nil {
			continue
		}
		if _, ok := cur[path[len(path)-1]]; ok {
			delete(cur, path[len(path)-1])
			changed = true
		}
	}
	if !changed {
		return data, nil
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(m); err != nil {
		return nil, fmt.Errorf("marshaling config.toml: %w", err)
	}
	return buf.Bytes(), nil
}
