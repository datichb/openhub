package config

// IsZero reports whether nothing is set: every setting inherits.
func (t TrackerLocalConfig) IsZero() bool { return t == TrackerLocalConfig{} }

// Overlay returns t with the settings o sets on top: a setting o leaves unset
// stays t's. The key of the token and the server it is valid for go together
// (a key of o does not inherit the server declared next to the key of t).
func (t TrackerLocalConfig) Overlay(o TrackerLocalConfig) TrackerLocalConfig {
	if o.Enabled != nil {
		t.Enabled = o.Enabled
	}
	if o.AutoSync != nil {
		t.AutoSync = o.AutoSync
	}
	if o.PushLabels != nil {
		t.PushLabels = o.PushLabels
	}
	if o.AutoPlanAssigned != nil {
		t.AutoPlanAssigned = o.AutoPlanAssigned
	}
	if o.MaxInProgressPerMember != nil {
		t.MaxInProgressPerMember = o.MaxInProgressPerMember
	}
	if o.WriteEnabled != nil {
		t.WriteEnabled = o.WriteEnabled
	}
	if o.TrackerURL != "" {
		t.TrackerURL = o.TrackerURL
	}
	if o.TrackerTokenKey != "" {
		t.TrackerTokenKey, t.TrackerTokenURL = o.TrackerTokenKey, o.TrackerTokenURL
	}
	return t
}

// TrackerFor is the personal tracker settings in force for the team: the
// [tracker] section of hub.toml (the default of every team) with what the
// team's own section ([teams.tracker]) sets on top. An unknown team, or none
// (""), gets the default.
func (c *Config) TrackerFor(teamID string) TrackerLocalConfig {
	if t := c.FindTeam(teamID); t != nil && teamID != "" {
		return c.Tracker.Overlay(t.Tracker)
	}
	return c.Tracker
}

// TeamTracker is the personal tracker overrides of the team alone (without
// the default of hub.toml); the zero value for an unknown team.
func (c *Config) TeamTracker(teamID string) TrackerLocalConfig {
	if t := c.FindTeam(teamID); t != nil {
		return t.Tracker
	}
	return TrackerLocalConfig{}
}

// TrackerForWrite is the tracker settings a change for the team goes to: the
// team's own section when the team is in hub.toml, the default [tracker]
// otherwise (no team, an unknown one).
func (c *Config) TrackerForWrite(teamID string) *TrackerLocalConfig {
	if t := c.FindTeam(teamID); t != nil && teamID != "" {
		return &t.Tracker
	}
	return &c.Tracker
}

// Sets reports whether the personal tracker settings set key (the name of
// the key in [tracker]: auto_sync, push_labels, tracker_url…).
func (t TrackerLocalConfig) Sets(key string) bool {
	switch key {
	case "enabled":
		return t.Enabled != nil
	case "auto_sync":
		return t.AutoSync != nil
	case "push_labels":
		return t.PushLabels != nil
	case "auto_plan_assigned":
		return t.AutoPlanAssigned != nil
	case "max_in_progress_per_member":
		return t.MaxInProgressPerMember != nil
	case "write_enabled":
		return t.WriteEnabled != nil
	case "tracker_url":
		return t.TrackerURL != ""
	case "tracker_token_key":
		return t.TrackerTokenKey != ""
	case "tracker_token_url":
		return t.TrackerTokenURL != ""
	}
	return false
}
