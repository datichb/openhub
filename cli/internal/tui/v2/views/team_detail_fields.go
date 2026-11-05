package views

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
)

// The fields of the team detail, by section. Three kinds of setting, said in
// the title of each section:
//
//   - shared with the team (config.toml of the team-state): a draft, published
//     as a whole with Ctrl+S; each line says where its value comes from (the
//     team wrote it, or it is the default) and an empty entry puts the
//     default back;
//   - personal (hub.toml, the section of this team): written by itself;
//   - written at once: the profile of the member and the fields of the team in
//     hub.toml (name, member id, default).

// fieldWebhookURL is the key of the field of the webhook (a secret: shown
// masked, typed in a password window).
const fieldWebhookURL = "webhook_url"

// sharedSetting is a field of config.toml: key is the key of the field,
// setting its dotted key in the file, prefix the i18n prefix of its label
// (.label) and its description (.desc).
func (v *TeamDetailView) sharedSetting(key, setting, prefix string, kind FieldKind) configField {
	return configField{
		Key: key, Kind: kind, Label: i18n.T(prefix + ".label"), Description: i18n.T(prefix + ".desc"),
		Scope: ScopeTeamShared, Source: v.origin(setting),
		Get: func() string { s, _ := v.teamCfg.Get(setting); return s },
		Set: func(val string) { v.setSetting(setting, val) },
	}
}

// setSetting sets (or, empty, puts back the default of) a setting of the
// draft; a value the setting refuses is said and not kept.
func (v *TeamDetailView) setSetting(setting, val string) {
	var err error
	if strings.TrimSpace(val) == "" {
		err = v.teamCfg.Unset(setting)
	} else {
		err = v.teamCfg.Set(setting, val)
	}
	if err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg(err.Error(), false)
		}
		return
	}
	v.dirtyTeam = true
}

// origin annotates a shared setting: written by the team, or a default.
func (v *TeamDetailView) origin(setting string) func() string {
	return func() string {
		if v.fileKeys == nil {
			return ""
		}
		if teamstate.InFile(v.fileKeys, setting) {
			return i18n.T("v54c.origin.team")
		}
		return i18n.T("v54c.origin.default")
	}
}

// leadOnly marks a shared field the leads alone may change.
func (v *TeamDetailView) leadOnly(f configField) configField {
	f.Locked = func() bool { return !v.canEnforce }
	if v.leadKeys == nil {
		v.leadKeys = map[string]bool{}
	}
	v.leadKeys[f.Key] = true
	return f
}

func numberRule(lo, hi int) *FieldValidator {
	return &FieldValidator{Numeric: true, MinInt: intPtr(lo), MaxInt: intPtr(hi), AllowEmpty: true}
}

func nonNegativeNumber(val string) error {
	if strings.TrimSpace(val) == "" {
		return nil
	}
	if n, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err != nil || n < 0 {
		return errors.New(i18n.Tf("v54c.field.number_invalid", val))
	}
	return nil
}

func (v *TeamDetailView) header(key string) configField {
	return configField{Key: key, Kind: CfgFieldSectionHeader, Label: i18n.T(key)}
}

// The sections of the detail, by the key of their title: OpenSection takes
// one, so that a screen can open the detail on the section it names.
const (
	TeamSectionTeam          = "v54c.section.team"
	TeamSectionMember        = "tui.team.detail.member.section"
	TeamSectionTracker       = "v54c.section.tracker"
	TeamSectionReserve       = "v54c.section.reserve"
	TeamSectionBoard         = "v54c.section.board"
	TeamSectionClaims        = "v54c.section.claims"
	TeamSectionNotifications = "v54c.section.notifications"
	TeamSectionLimits        = "v54c.section.limits"
	TeamSectionGovernance    = "tui.team.governance.section"
	TeamSectionMine          = "v54c.section.mine"
)

// OpenSection puts the cursor on the first line of a section (TeamSection*)
// the next time the fields are shown: call it before the view is mounted,
// or after, once it is loaded. An unknown section is ignored.
func (v *TeamDetailView) OpenSection(section string) {
	v.section = section
	if v.list != nil && len(v.fields) > 0 {
		v.renderFields()
	}
}

// sectionItem is the index, in the list, of the first line of the section
// asked for; -1 when none was asked for or it is not there.
func (v *TeamDetailView) sectionItem() int {
	if v.section == "" {
		return -1
	}
	item := 0
	found := false
	for _, f := range v.fields {
		if f.Visible != nil && !f.Visible() {
			continue
		}
		if found {
			return item
		}
		if f.Kind == CfgFieldSectionHeader && f.Key == v.section {
			found = true
		}
		item++
	}
	return -1
}

func (v *TeamDetailView) subHeader(key string) configField {
	return configField{Kind: CfgFieldSubHeader, Label: i18n.T(key)}
}

func (v *TeamDetailView) buildFields() {
	v.fields, v.leadKeys = nil, nil
	if v.solo() {
		// Team features are off in a solo space (tracker, notifications,
		// collaboration): only what applies to it is shown.
		v.buildWorkflowFields()
		v.appendLinks()
		return
	}
	for _, section := range [][]configField{
		v.identityFields(), v.memberFields(), v.trackerFields(), v.reserveFields(), v.boardFields(),
		v.claimSection(), v.notificationFields(), v.limitFields(), v.governanceFields(), v.overrideFields(),
	} {
		v.fields = append(v.fields, section...)
	}
	v.appendLinks()
}

// solo reports whether the detail is the one of a solo space.
func (v *TeamDetailView) solo() bool {
	return v.cfg.ResolveTeam != nil && v.cfg.ResolveTeam().Solo
}

// ─── Team: hub.toml, written at once ────────────────────────────────────────

func (v *TeamDetailView) identityFields() []configField {
	if v.cfg.GetTeamInfo == nil && v.cfg.OnReconfigure == nil {
		return []configField{v.header("v54c.section.team"), v.teamGitLabField()}
	}
	out := []configField{v.header("v54c.section.team")}
	teamID := ""
	if v.cfg.ResolveTeam != nil {
		teamID = v.cfg.ResolveTeam().TeamID
	}
	if v.cfg.GetTeamInfo != nil && v.cfg.SetTeamField != nil {
		out = append(out,
			configField{
				Key: "team_name", Kind: CfgFieldString, Label: i18n.T("v54c.field.team_name.label"), Description: i18n.T("v54c.field.team_name.desc"),
				Scope: ScopeTeamLocal, Immediate: true, Placeholder: teamID,
				Get: func() string { return v.info.Name },
				Set: func(val string) { v.setTeamField(TeamFieldName, strings.TrimSpace(val)) },
			},
			configField{
				Key: "team_member_id", Kind: CfgFieldString, Label: i18n.T("v54c.field.team_member_id.label"), Description: i18n.T("v54c.field.team_member_id.desc"),
				Scope: ScopeTeamLocal, Immediate: true,
				Validator: &FieldValidator{Check: func(val string) error {
					if strings.TrimSpace(val) == "" {
						return errors.New(i18n.T("v54c.field.team_member_id.required"))
					}
					return nil
				}},
				Get: func() string { return v.info.MemberID },
				Set: func(val string) { v.setTeamField(TeamFieldMemberID, strings.TrimSpace(val)) },
			},
			configField{
				Key: "team_default", Kind: CfgFieldBool, Label: i18n.T("v54c.field.team_default.label"), Description: i18n.T("v54c.field.team_default.desc"),
				Scope: ScopeTeamLocal, Immediate: true,
				Get: func() string { return tdBoolToStr(v.info.Default) },
				Set: func(val string) { v.setTeamField(TeamFieldDefault, val) },
			},
			configField{
				Key: "team_repo", Kind: CfgFieldReadonly, Label: i18n.T("v54c.field.team_repo.label"), Description: i18n.Tf("v54c.field.team_repo.desc", teamID),
				Scope: ScopeTeamLocal, Get: func() string { return maskRepo(v.info.StateRepo) },
			},
		)
	}
	out = append(out, v.teamGitLabField())
	if v.cfg.OnReconfigure != nil {
		out = append(out, configField{
			Key: "team_reconfigure", Kind: CfgFieldAction, Label: i18n.T("v54c.field.reconfigure.label"), Description: i18n.T("v54c.field.reconfigure.desc"),
			Get: func() string { return "→" },
			Set: func(string) { v.cfg.OnReconfigure(teamID) },
		})
	}
	return out
}

// setTeamField writes a field of the team to hub.toml at once and shows it.
func (v *TeamDetailView) setTeamField(field, value string) {
	if v.shell == nil || v.cfg.SetTeamField == nil {
		return
	}
	teamID := v.cfg.ResolveTeam().TeamID
	if err := v.cfg.SetTeamField(teamID, field, value); err != nil {
		v.shell.ShowToastMsg(i18n.Tf("tui.team.detail.save_local_error", err.Error()), false)
		return
	}
	if v.cfg.GetTeamInfo != nil {
		v.info = v.cfg.GetTeamInfo(teamID)
	}
	v.shell.ShowToastMsg(i18n.Tf("tui.team.detail.saved", i18n.T("tui.team.detail.saved_local")), true)
	v.renderFields()
}

// maskRepo is a repository URL without the credentials it may carry.
func maskRepo(raw string) string {
	if i := strings.Index(raw, "://"); i >= 0 {
		rest := raw[i+3:]
		if at := strings.Index(rest, "@"); at >= 0 && !strings.Contains(rest[:at], "/") {
			return raw[:i+3] + rest[at+1:]
		}
	}
	return raw
}

// ─── Tracker (shared) ───────────────────────────────────────────────────────

var ticketPatternRule = &FieldValidator{AllowEmpty: true, Check: func(val string) error {
	re, err := regexp.Compile(strings.TrimSpace(val))
	if err != nil {
		return errors.New(i18n.Tf("v54c.field.ticket_pattern.invalid", err))
	}
	if re.NumSubexp() != 1 {
		return errors.New(i18n.T("v54c.field.ticket_pattern.one_group"))
	}
	return nil
}}

func (v *TeamDetailView) trackerFields() []configField {
	typeField := v.sharedSetting("type", "tracker.type", "tui.config.field.tracker_type", CfgFieldSelect)
	typeField.Options = []SelectOption{{Label: "GitLab", Value: "gitlab"}, {Label: "Jira", Value: "jira"}}
	url := v.sharedSetting("tracker_url", "tracker.tracker_url", "tui.config.field.tracker_url", CfgFieldString)
	url.Placeholder = i18n.T("tui.config.field.tracker_url.placeholder")
	project := v.sharedSetting("tracker_project", "tracker.tracker_project", "tui.config.field.tracker_project", CfgFieldString)
	project.Placeholder = i18n.T("tui.config.field.tracker_project.placeholder")
	pattern := v.sharedSetting("ticket_pattern", "tracker.ticket_pattern", "v54c.field.ticket_pattern", CfgFieldString)
	pattern.Placeholder = `SRU-(\d+)`
	pattern.Validator = ticketPatternRule
	enabled := v.sharedSetting("enabled", "tracker.enabled", "tui.config.field.tracker_enabled", CfgFieldBool)
	write := v.sharedSetting("write_enabled", "tracker.write_enabled", "v54c.field.write_enabled", CfgFieldBool)
	autoSync := v.sharedSetting("auto_sync", "tracker.auto_sync", "tui.config.field.auto_sync", CfgFieldBool)
	interval := v.sharedSetting("sync_interval", "tracker.sync_interval_minutes", "tui.config.field.sync_interval", CfgFieldInt)
	interval.Validator = numberRule(0, teamstate.MaxSyncIntervalMinute)
	push := v.sharedSetting("push_labels", "tracker.push_labels", "tui.config.field.push_labels", CfgFieldBool)
	push.Locked = func() bool { return v.teamCfg.Tracker.IsPushLabelsEnforced() && !v.canEnforce }
	pushEnforced := v.leadOnly(v.sharedSetting("push_labels_enforced", "tracker.push_labels_enforced", "tui.config.field.push_labels_enforced", CfgFieldBool))
	plan := v.sharedSetting("auto_plan_assigned", "tracker.auto_plan_assigned", "tui.config.field.auto_plan_assigned", CfgFieldBool)
	maxWip := v.sharedSetting("max_in_progress", "tracker.max_in_progress_per_member", "tui.config.field.max_in_progress", CfgFieldInt)
	maxWip.Placeholder = i18n.T("tui.config.field.max_in_progress.placeholder")
	maxWip.Validator = numberRule(0, teamstate.MaxInProgressLimit)
	maxWip.Get = func() string { return strconv.Itoa(v.teamCfg.Tracker.InProgressLimit()) }
	maxWip.Set = func(val string) {
		if strings.TrimSpace(val) == "" {
			v.setSetting("tracker.max_in_progress_per_member", "")
			return
		}
		v.setSetting("tracker.max_in_progress_per_member", val)
	}

	out := []configField{v.header("v54c.section.tracker"), typeField, url}
	out = append(out, v.tokenFields()...)
	return append(out, project, pattern, enabled, write, autoSync, interval, push, pushEnforced, plan, maxWip)
}

// ─── Reserve and auto-plan (shared) ─────────────────────────────────────────

func (v *TeamDetailView) reserveFields() []configField {
	labels := v.sharedSetting("unassigned_labels", "tracker.unassigned_labels", "v54c.field.unassigned_labels", CfgFieldString)
	labels.Placeholder = "ready, bug"
	capped := v.sharedSetting("max_unassigned_issues", "tracker.max_unassigned_issues", "v54c.field.max_unassigned", CfgFieldInt)
	capped.Validator = numberRule(teamstate.MinUnassignedIssues, teamstate.MaxUnassignedIssues)
	capped.Get = func() string { return strconv.Itoa(v.teamCfg.Tracker.MaxUnassignedIssues) }
	return []configField{
		v.header("v54c.section.reserve"),
		v.sharedSetting("auto_plan_unassigned", "tracker.auto_plan_unassigned", "v54c.field.auto_plan_unassigned", CfgFieldBool),
		labels, capped,
	}
}

// ─── Board: a summary and a link to the editor ──────────────────────────────

func (v *TeamDetailView) boardFields() []configField {
	out := []configField{v.header("v54c.section.board"), {
		Key: "board_summary", Kind: CfgFieldReadonly, Label: i18n.T("v54c.field.board.label"), Description: i18n.T("v54c.field.board.desc"),
		Scope: ScopeTeamShared, Source: v.origin("board.columns"),
		Get: func() string {
			if !v.teamCfg.Board.HasCustomColumns() {
				return i18n.T("v54c.field.board.default")
			}
			return strings.Join(v.teamCfg.Board.AllStatuses(), " · ")
		},
	}}
	if v.cfg.OnEditBoard != nil {
		out = append(out, configField{
			Key: "board_edit", Kind: CfgFieldAction, Label: i18n.T("v54c.field.board_edit.label"), Description: i18n.T("v54c.field.board_edit.desc"),
			Get: func() string { return "→" },
			Set: func(string) {
				if v.dirtyTeam {
					v.shell.ShowToastMsg(i18n.Tf("v54c.field.board_edit.publish_first", keys.Save.Keys()), false)
					return
				}
				v.cfg.OnEditBoard()
			},
		})
	}
	return out
}

// ─── Claims (shared) ────────────────────────────────────────────────────────

func (v *TeamDetailView) claimSection() []configField {
	stale := v.sharedSetting("stale_days", "takeover.stale_days", "tui.config.field.stale_days", CfgFieldInt)
	stale.Placeholder = i18n.T("tui.config.field.stale_days.placeholder")
	stale.Validator = numberRule(teamstate.MinStaleDays, teamstate.MaxStaleDays)
	stale.Get = func() string { return strconv.Itoa(v.teamCfg.Takeover.StaleDays) }
	retention := v.sharedSetting("done_retention_days", "claim.done_retention_days", "tui.config.field.done_retention", CfgFieldInt)
	retention.Placeholder = i18n.T("tui.config.field.done_retention.placeholder")
	retention.Validator = numberRule(teamstate.MinDoneRetentionDays, teamstate.MaxDoneRetentionDays)
	retention.Get = func() string { return strconv.Itoa(v.teamCfg.Claim.DoneRetentionDays) }
	out := []configField{v.header("v54c.section.claims"), stale, retention}
	return append(out, v.claimFields()...)
}

// ─── Notifications (shared) ─────────────────────────────────────────────────

func (v *TeamDetailView) notificationFields() []configField {
	enabled := v.sharedSetting("notif_enabled", "notification.enabled", "v54c.field.notif_enabled", CfgFieldBool)
	kind := v.sharedSetting("notif_type", "notification.type", "tui.config.field.notif_type", CfgFieldSelect)
	kind.Options = []SelectOption{
		{Label: "↩ " + i18n.T("v54c.field.notif_type.auto"), Value: ""},
		{Label: "Mattermost", Value: "mattermost"}, {Label: "Slack", Value: "slack"},
		{Label: "Discord", Value: "discord"}, {Label: "Teams", Value: "teams"},
	}
	kind.Get = func() string { return v.teamCfg.Notification.EffectiveType() }
	kind.Set = func(val string) { v.setSetting("notification.type", val) }
	hook := v.sharedSetting(fieldWebhookURL, "notification.webhook_url", "tui.config.field.webhook_url", CfgFieldPassword)
	hook.Placeholder = i18n.T("tui.config.field.webhook_url.placeholder")
	hook.Get = func() string { return v.teamCfg.Notification.EffectiveURL() }
	hook.Source = func() string {
		if u := v.teamCfg.Notification.EffectiveURL(); u != "" {
			return teamstate.MaskWebhook(u)
		}
		return ""
	}
	channel := v.sharedSetting("channel", "notification.channel", "tui.config.field.channel", CfgFieldString)
	bot := v.sharedSetting("bot_name", "notification.bot_name", "tui.config.field.bot_name", CfgFieldString)
	bot.Get = func() string { return v.teamCfg.Notification.BotName }
	events := v.eventsField("notif_events", "notification.events")
	mention := v.mentionField("notif_mention", "notification.mention")
	out := []configField{v.header("v54c.section.notifications"), enabled, kind, hook, channel, bot, events, mention}
	for i, d := range v.teamCfg.Notification.Destinations {
		name := d.Type
		if name == "" {
			name = "?"
		}
		out = append(out,
			configField{Kind: CfgFieldSubHeader, Label: i18n.Tf("v54n.field.destination", i+1, name)},
			v.eventsField("notif_dest_"+strconv.Itoa(i)+"_events", "notification.destinations."+strconv.Itoa(i)+".events"),
			v.mentionField("notif_dest_"+strconv.Itoa(i)+"_mention", "notification.destinations."+strconv.Itoa(i)+".mention"))
	}
	return out
}

// eventsField is the list of events a destination receives (types, families
// « claim.* », « * »); empty puts the default back (review and wiki).
func (v *TeamDetailView) eventsField(key, setting string) configField {
	f := v.sharedSetting(key, setting, "v54n.field.events", CfgFieldString)
	f.Placeholder = i18n.T("v54n.field.events.placeholder")
	f.Validator = &FieldValidator{AllowEmpty: true, Check: func(val string) error {
		for _, p := range strings.Split(val, ",") {
			if p = strings.TrimSpace(p); p != "" && !teamstate.ValidEventPattern(p) {
				return errors.New(i18n.Tf("v54n.config.unknown_event", p, strings.Join(teamstate.NotificationEvents, ", ")))
			}
		}
		return nil
	}}
	return f
}

// mentionField says whether the person an event is for is mentioned
// (mattermost_username of their card): on unless the team turned it off.
func (v *TeamDetailView) mentionField(key, setting string) configField {
	f := v.sharedSetting(key, setting, "v54n.field.mention", CfgFieldTriBool)
	return f
}

// promptWebhook asks the webhook in a window that hides what is typed; an
// empty answer keeps the current one (oh team config unset removes it).
func (v *TeamDetailView) promptWebhook(f *configField) {
	v.shell.ShowPasswordModal(f.Label, func(val string) {
		val = strings.TrimSpace(val)
		if val == "" {
			return
		}
		if !strings.HasPrefix(val, "https://") && !strings.HasPrefix(val, "http://") {
			v.shell.ShowToastMsg(i18n.Tf("v54c.field.webhook.invalid", teamstate.MaskWebhook(val)), false)
			return
		}
		v.pushUndo()
		n := &v.teamCfg.Notification
		n.WebhookURL, n.MattermostWebhook = val, ""
		if n.Type == "" {
			n.Type = teamstate.InferNotifierType(val)
		}
		v.dirtyTeam = true
		v.renderFields()
	})
}

// ─── Restrictions: [limits] (shared) ────────────────────────────────────────

func (v *TeamDetailView) limitFields() []configField {
	out := []configField{v.header("v54c.section.limits")}
	for _, scope := range []string{"recommended", "enforced"} {
		out = append(out, v.subHeader("v54c.limits.sub."+scope))
		for _, name := range []string{"max_active_sessions", "session_budget_usd", "daily_budget_usd", "memory_mb", "models"} {
			setting := "limits." + scope + "." + name
			kind := CfgFieldString
			if name == "max_active_sessions" || name == "memory_mb" {
				kind = CfgFieldInt
			}
			f := v.sharedSetting(scope+"_"+name, setting, "v54c.limits.field."+name, kind)
			switch name {
			case "max_active_sessions", "memory_mb":
				f.Validator = numberRule(0, 1_000_000)
			case "session_budget_usd", "daily_budget_usd":
				f.Validator = &FieldValidator{AllowEmpty: true, Check: nonNegativeNumber}
			case "models":
				f.Placeholder = "eu.anthropic.*"
			}
			if scope == "enforced" {
				f = v.leadOnly(f)
			}
			out = append(out, f)
		}
	}
	return out
}

// ─── Governance ─────────────────────────────────────────────────────────────

func (v *TeamDetailView) governanceFields() []configField {
	cfgPolicy := v.leadOnly(v.sharedSetting("governance.config", "governance.config", "v54c.field.governance_config", CfgFieldSelect))
	cfgPolicy.Options = []SelectOption{
		{Label: i18n.T("v54c.governance.lead"), Value: teamstate.GovernanceConfigLead},
		{Label: i18n.T("v54c.governance.any_member"), Value: teamstate.GovernanceConfigAnyMember},
	}
	cfgPolicy.Get = func() string { return v.teamCfg.Governance.ConfigPolicy() }
	return []configField{
		v.header(TeamSectionGovernance),
		{
			Key: "governance.publish", Kind: CfgFieldReadonly, Label: i18n.T("tui.team.governance.label"),
			Description: i18n.T("tui.team.governance.desc"), Scope: ScopeTeamShared,
			Get: func() string { return governanceValue(v.teamCfg.Governance.PublishPolicy()) },
		},
		cfgPolicy,
	}
}

// ─── Mine for this team: hub.toml, written by itself ────────────────────────

// local annotates a personal setting: set for this team, set by the default
// [tracker] of hub.toml, or not set (the team's value applies).
func (v *TeamDetailView) local(key string) func() string {
	return func() string {
		switch {
		case v.localTrk.Sets(key):
			return i18n.T("v54c.origin.me_team")
		case v.hubTrk.Sets(key):
			return i18n.T("v54c.origin.me_hub")
		}
		return ""
	}
}

func (v *TeamDetailView) overrideFields() []configField {
	tri := func(key, setting, prefix string, ptr func(*config.TrackerLocalConfig) **bool) configField {
		return configField{
			Key: key, Kind: CfgFieldTriBool, Label: i18n.T(prefix + ".label"), Description: i18n.T(prefix + ".desc"),
			Scope: ScopeTeamLocal, Source: v.local(setting),
			Get: func() string { return ptrBoolToTriState(*ptr(&v.localTrk)) },
			Set: func(val string) { *ptr(&v.localTrk) = triStateToPtrBool(val); v.touchLocal() },
		}
	}
	push := tri("push_labels", "push_labels", "tui.config.field.push_labels", func(t *config.TrackerLocalConfig) **bool { return &t.PushLabels })
	push.Locked = func() bool { return v.teamCfg.Tracker.IsPushLabelsEnforced() }
	maxWip := configField{
		Key: "my_max_in_progress", Kind: CfgFieldInt, Label: i18n.T("tui.config.field.max_in_progress.label"), Description: i18n.T("v54c.field.my_max_in_progress.desc"),
		Scope: ScopeTeamLocal, Source: v.local("max_in_progress_per_member"), Validator: numberRule(0, teamstate.MaxInProgressLimit),
		Placeholder: i18n.T("v54c.field.my_max_in_progress.placeholder"),
		Get: func() string {
			if v.localTrk.MaxInProgressPerMember == nil {
				return ""
			}
			return strconv.Itoa(*v.localTrk.MaxInProgressPerMember)
		},
		Set: func(val string) {
			if n, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
				v.localTrk.MaxInProgressPerMember = &n
			} else {
				v.localTrk.MaxInProgressPerMember = nil
			}
			v.touchLocal()
		},
	}
	return []configField{
		v.header("v54c.section.mine"),
		tri("tracker_enabled", "enabled", "tui.config.field.tracker_enabled", func(t *config.TrackerLocalConfig) **bool { return &t.Enabled }),
		tri("auto_sync", "auto_sync", "tui.config.field.auto_sync", func(t *config.TrackerLocalConfig) **bool { return &t.AutoSync }),
		push,
		tri("my_auto_plan_assigned", "auto_plan_assigned", "tui.config.field.auto_plan_assigned", func(t *config.TrackerLocalConfig) **bool { return &t.AutoPlanAssigned }),
		tri("my_write_enabled", "write_enabled", "v54c.field.write_enabled", func(t *config.TrackerLocalConfig) **bool { return &t.WriteEnabled }),
		maxWip,
	}
}

// ─── Links ──────────────────────────────────────────────────────────────────

func (v *TeamDetailView) appendLinks() {
	for _, l := range []struct{ key, label, target string }{
		{"mcp", "tui.config.link.mcp", "team.mcp"},
		{"models", "tui.config.link.models", "team.models"},
		{"workflow", "tui.config.link.workflow", "workflows"},
	} {
		v.fields = append(v.fields, configField{
			Key: l.key, Kind: CfgFieldLink, Label: i18n.T(l.label), LinkTarget: l.target,
			Get: func() string { return "" }})
	}
}
