package tracker

// DefaultTokenKey is the key a tracker token is proposed under when the user
// has declared none: openhub.tracker.<type>.token.
func DefaultTokenKey(trackerType string) string {
	return "openhub.tracker." + trackerType + ".token"
}

// NeedsTokenDecl reports whether keeping the token of the tracker under key,
// for server, requires writing tracker_token_key (and tracker_token_url) to
// hub.toml, given what is declared already (the effective configuration).
//
// Nothing is needed when the declaration is the one in force. The default key
// of GitLab is read without being declared, as the former tracker key, for the
// server of the tracker: declaring it is only needed to give it another server.
// Jira only reads a declared key and has no server rule: the server is ignored.
func (eff EffectiveTrackerConfig) NeedsTokenDecl(key, server string) bool {
	if eff.Type == string(TypeJira) {
		return key != eff.TrackerTokenKey
	}
	if key == eff.TrackerTokenKey && server == eff.TrackerTokenURL {
		return false
	}
	return eff.TrackerTokenKey != "" || server != "" || key != DefaultTokenKey(eff.Type)
}
