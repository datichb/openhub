package teamstate

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// The values config.toml does not write because reading them back gives them
// again: a file only holds what the team chose.
const (
	defaultStaleDays    = 3
	defaultBotName      = "OpenHub"
	defaultDoneDays     = 7
	defaultUnassigned   = 20
	defaultSyncInterval = 5
)

// withoutDefaults is cfg with the settings that equal their default blanked:
// the omitempty of their key keeps them out of the file. heartbeat_minutes is
// apart (an explicit 0 disables the heartbeat, so 0 must be written):
// stripDefaultHeartbeat takes its default out of the text.
func withoutDefaults(cfg TeamConfig) TeamConfig {
	if cfg.Takeover.StaleDays == defaultStaleDays {
		cfg.Takeover.StaleDays = 0
	}
	if cfg.Notification.BotName == defaultBotName {
		cfg.Notification.BotName = ""
	}
	if cfg.Claim.DoneRetentionDays == defaultDoneDays {
		cfg.Claim.DoneRetentionDays = 0
	}
	if cfg.Claim.LeaseHours == DefaultLeaseHours {
		cfg.Claim.LeaseHours = 0
	}
	if cfg.Tracker.MaxUnassignedIssues == defaultUnassigned {
		cfg.Tracker.MaxUnassignedIssues = 0
	}
	if cfg.Tracker.SyncIntervalMinutes == defaultSyncInterval {
		cfg.Tracker.SyncIntervalMinutes = 0
	}
	return cfg
}

var defaultHeartbeatLine = regexp.MustCompile(`(?m)^heartbeat_minutes = ` + fmt.Sprint(DefaultHeartbeatMinutes) + `\n`)

func stripDefaultHeartbeat(data []byte, cfg TeamConfig) []byte {
	if cfg.Claim.HeartbeatMinutes != DefaultHeartbeatMinutes {
		return data
	}
	return defaultHeartbeatLine.ReplaceAll(data, nil)
}

// marshalConfig is the content of config.toml for cfg: the former
// max_auto_plan_per_member is replaced by max_in_progress_per_member, the
// settings equal to their default are left out, and the keys oh does not know
// (a newer oh wrote them, or a former one — [parallel] — read them) that
// existing, the file as it is, holds are kept as they are.
func marshalConfig(cfg *TeamConfig, existing []byte) ([]byte, error) {
	out := *cfg
	out.Tracker = cfg.Tracker.migrated()
	out = withoutDefaults(out)
	data, err := toml.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("marshaling config.toml: %w", err)
	}
	data = stripDefaultHeartbeat(data, out)
	if len(existing) == 0 {
		return data, nil
	}
	return mergeUnknownKeys(data, existing)
}

// unknownPaths are the keys of a config.toml that this version of oh does not
// know, a table counting for one key.
func unknownPaths(data []byte) [][]string {
	var cfg TeamConfig
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var strict *toml.StrictMissingError
	if err := dec.Decode(&cfg); !errors.As(err, &strict) {
		return nil
	}
	paths := make([][]string, 0, len(strict.Errors))
	for _, e := range strict.Errors {
		paths = append(paths, []string(e.Key()))
	}
	return topmost(paths)
}

// UnknownKeys are the keys of a config.toml that this version of oh does not
// know (sorted): a setting of a newer oh, a typo, or a former setting that is
// dropped (DeprecatedKeys).
func UnknownKeys(data []byte) []string {
	var keys []string
	for _, p := range unknownPaths(data) {
		keys = append(keys, strings.Join(p, "."))
	}
	sort.Strings(keys)
	return keys
}

// topmost drops the paths that lie under another one of the list.
func topmost(paths [][]string) [][]string {
	sort.Slice(paths, func(i, j int) bool { return len(paths[i]) < len(paths[j]) })
	var out [][]string
	for _, p := range paths {
		under := false
		for _, o := range out {
			if len(o) <= len(p) && strings.Join(o, "\x00") == strings.Join(p[:len(o)], "\x00") {
				under = true
				break
			}
		}
		if !under {
			out = append(out, p)
		}
	}
	return out
}

// mergeUnknownKeys adds to data (the marshaled config) the keys of existing
// that oh does not know. A key inside an array of tables (an unknown setting
// of a board column) cannot be addressed and is not carried over.
func mergeUnknownKeys(data, existing []byte) ([]byte, error) {
	paths := unknownPaths(existing)
	if len(paths) == 0 {
		return data, nil
	}
	var old, merged map[string]any
	if err := toml.Unmarshal(existing, &old); err != nil {
		return nil, fmt.Errorf("parsing config.toml: %w", err)
	}
	if err := toml.Unmarshal(data, &merged); err != nil {
		return nil, fmt.Errorf("marshaling config.toml: %w", err)
	}
	for _, p := range paths {
		val, ok := valueAt(old, p)
		if !ok {
			continue
		}
		setPath(merged, p, val)
	}
	pruneEmpty(merged)
	out, err := toml.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("marshaling config.toml: %w", err)
	}
	return out, nil
}

func valueAt(m map[string]any, path []string) (any, bool) {
	var cur any = m
	for _, k := range path {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		if cur, ok = mm[k]; !ok {
			return nil, false
		}
	}
	return cur, true
}

func setPath(m map[string]any, path []string, val any) {
	for _, k := range path[:len(path)-1] {
		next, ok := m[k].(map[string]any)
		if !ok {
			if _, exists := m[k]; exists {
				return // a value of another shape is in the way
			}
			next = map[string]any{}
			m[k] = next
		}
		m = next
	}
	m[path[len(path)-1]] = val
}

// DeprecatedKeys are the keys of a config.toml that a former oh wrote and
// this one only reads or ignores: oh doctor reports them, --fix migrates or
// removes them.
var DeprecatedKeys = []string{"parallel", "tracker.type_enforced", "tracker.max_auto_plan_per_member", "notification.mattermost_webhook"}

// pruneEmpty removes the tables that hold nothing (the struct marshaling does
// not write them either).
func pruneEmpty(m map[string]any) {
	for k, v := range m {
		sub, ok := v.(map[string]any)
		if !ok {
			continue
		}
		pruneEmpty(sub)
		if len(sub) == 0 {
			delete(m, k)
		}
	}
}
