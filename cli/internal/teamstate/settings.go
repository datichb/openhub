package teamstate

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// The settings of config.toml addressed by their dotted key
// (tracker.type, claim.lease_hours, mcp.gitlab.url, notification.destinations.0.type…),
// typed by the field they name. oh team config get|set|unset|show and the
// screens that edit one setting at a time go through them: a value is parsed
// with the type of its key and bounded (UpdateConfig validates the sections
// it touches), never written as text.

// settingDefaults are the settings whose absence in config.toml means a value
// other than the zero one; unsetting them puts that value back (config.toml
// does not write it).
var settingDefaults = map[string]any{
	"takeover.stale_days":           defaultStaleDays,
	"claim.done_retention_days":     defaultDoneDays,
	"claim.lease_hours":             DefaultLeaseHours,
	"claim.heartbeat_minutes":       DefaultHeartbeatMinutes,
	"tracker.max_unassigned_issues": defaultUnassigned,
	"tracker.sync_interval_minutes": defaultSyncInterval,
	"notification.bot_name":         defaultBotName,
}

// IsSecretSetting reports whether the value of a setting is a secret (a
// webhook): it is shown masked, never in an event nor in a finding.
func IsSecretSetting(key string) bool {
	if key == "notification.webhook_url" || key == "notification.mattermost_webhook" {
		return true
	}
	return strings.HasPrefix(key, "notification.destinations.") && strings.HasSuffix(key, ".webhook_url")
}

// SettingText is the value of a setting as it may be shown.
func SettingText(key, value string) string {
	if IsSecretSetting(key) {
		return MaskWebhook(value)
	}
	return value
}

// Get is the value of the setting key as text ("" for a setting that is not
// set), or an error when no such setting exists.
func (cfg *TeamConfig) Get(key string) (string, error) {
	v, err := lookupSetting(reflect.ValueOf(cfg).Elem(), strings.Split(key, "."), key)
	if err != nil {
		return "", err
	}
	return settingString(v), nil
}

// Set parses value with the type of the setting key and sets it.
func (cfg *TeamConfig) Set(key, value string) error {
	return editSetting(reflect.ValueOf(cfg).Elem(), strings.Split(key, "."), key, value, false)
}

// Unset puts the setting back to its default: the zero value, or the default
// value of settingDefaults; a map entry or a list item is removed.
func (cfg *TeamConfig) Unset(key string) error {
	return editSetting(reflect.ValueOf(cfg).Elem(), strings.Split(key, "."), key, "", true)
}

// Setting is a leaf of the configuration with its value as text.
type Setting struct {
	Key   string
	Value string
}

// Settings lists the settings of cfg in the order of the file: every leaf
// that has a value, and the maps and lists that have entries.
func (cfg *TeamConfig) Settings() []Setting {
	var out []Setting
	walkSettings(reflect.ValueOf(cfg).Elem(), "", &out)
	return out
}

func unknownSetting(key string) error {
	return fmt.Errorf("%s", i18n.Tf("v54c.settings.unknown", key))
}

func fieldByTomlName(v reflect.Value, name string) (reflect.Value, bool) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if n, ok := tomlName(f); ok && f.IsExported() && n == name {
			return v.Field(i), true
		}
	}
	return reflect.Value{}, false
}

func lookupSetting(v reflect.Value, path []string, key string) (reflect.Value, error) {
	for _, step := range path {
		for v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return reflect.Value{}, nil
			}
			v = v.Elem()
		}
		switch v.Kind() {
		case reflect.Struct:
			f, ok := fieldByTomlName(v, step)
			if !ok {
				return reflect.Value{}, unknownSetting(key)
			}
			v = f
		case reflect.Map:
			if v.Type().Key().Kind() != reflect.String {
				return reflect.Value{}, unknownSetting(key)
			}
			e := v.MapIndex(reflect.ValueOf(step).Convert(v.Type().Key()))
			if !e.IsValid() {
				return reflect.Value{}, nil
			}
			v = e
		case reflect.Slice:
			n, err := strconv.Atoi(step)
			if err != nil || n < 0 {
				return reflect.Value{}, unknownSetting(key)
			}
			if n >= v.Len() {
				return reflect.Value{}, nil
			}
			v = v.Index(n)
		default:
			return reflect.Value{}, unknownSetting(key)
		}
	}
	return v, nil
}

func settingString(v reflect.Value) string {
	if !v.IsValid() {
		return ""
	}
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	case reflect.Int, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64)
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.String {
			parts := make([]string, v.Len())
			for i := range parts {
				parts[i] = v.Index(i).String()
			}
			return strings.Join(parts, ", ")
		}
		return i18n.Tf("v54c.settings.entries", v.Len())
	case reflect.Map, reflect.Struct:
		return ""
	}
	return fmt.Sprint(v.Interface())
}

// parseSetting sets the scalar v (a settable value) from text.
func parseSetting(v reflect.Value, value, key string) error {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}
	value = strings.TrimSpace(value)
	switch v.Kind() {
	case reflect.String:
		v.SetString(value)
	case reflect.Bool:
		switch strings.ToLower(value) {
		case "true", "1", "yes", "on":
			v.SetBool(true)
		case "false", "0", "no", "off":
			v.SetBool(false)
		default:
			return fmt.Errorf("%s: %s", key, i18n.Tf("v54c.settings.want_bool", value))
		}
	case reflect.Int, reflect.Int64:
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%s: %s", key, i18n.Tf("v54c.settings.want_int", value))
		}
		v.SetInt(int64(n))
	case reflect.Float64:
		n, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("%s: %s", key, i18n.Tf("v54c.settings.want_number", value))
		}
		v.SetFloat(n)
	case reflect.Slice:
		if v.Type().Elem().Kind() != reflect.String {
			return fmt.Errorf("%s: %s", key, i18n.T("v54c.settings.not_scalar"))
		}
		var items []string
		for _, p := range strings.Split(value, ",") {
			if p = strings.TrimSpace(p); p != "" {
				items = append(items, p)
			}
		}
		v.Set(reflect.ValueOf(items))
	default:
		return fmt.Errorf("%s: %s", key, i18n.T("v54c.settings.not_scalar"))
	}
	return nil
}

// editSetting walks to the setting and sets it (or unsets it).
func editSetting(v reflect.Value, path []string, key, value string, unset bool) error {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			if unset {
				return nil
			}
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}
	if len(path) == 0 {
		if unset {
			return resetSetting(v, key)
		}
		return parseSetting(v, value, key)
	}
	step, rest := path[0], path[1:]
	switch v.Kind() {
	case reflect.Struct:
		f, ok := fieldByTomlName(v, step)
		if !ok {
			return unknownSetting(key)
		}
		if unset && len(rest) == 0 {
			return resetSetting(f, key)
		}
		return editSetting(f, rest, key, value, unset)
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return unknownSetting(key)
		}
		k := reflect.ValueOf(step).Convert(v.Type().Key())
		if unset && len(rest) == 0 {
			if !v.IsNil() {
				v.SetMapIndex(k, reflect.Value{})
			}
			return nil
		}
		if v.IsNil() {
			if unset {
				return nil
			}
			v.Set(reflect.MakeMap(v.Type()))
		}
		elem := reflect.New(v.Type().Elem()).Elem()
		if cur := v.MapIndex(k); cur.IsValid() {
			elem.Set(cloneValue(cur))
		} else if unset {
			return nil
		}
		if err := editSetting(elem, rest, key, value, unset); err != nil {
			return err
		}
		v.SetMapIndex(k, elem)
		return nil
	case reflect.Slice:
		n, err := strconv.Atoi(step)
		if err != nil || n < 0 || v.Type().Elem().Kind() == reflect.String {
			return unknownSetting(key)
		}
		if n > v.Len() && unset {
			return nil
		}
		if n > v.Len() {
			return unknownSetting(key)
		}
		if unset && len(rest) == 0 {
			if n < v.Len() {
				v.Set(reflect.AppendSlice(v.Slice(0, n), v.Slice(n+1, v.Len())))
			}
			return nil
		}
		if n == v.Len() {
			if unset {
				return nil
			}
			v.Set(reflect.Append(v, reflect.New(v.Type().Elem()).Elem()))
		}
		return editSetting(v.Index(n), rest, key, value, unset)
	}
	return unknownSetting(key)
}

func resetSetting(v reflect.Value, key string) error {
	if def, ok := settingDefaults[key]; ok {
		for v.Kind() == reflect.Pointer {
			if v.IsNil() {
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
		switch d := def.(type) {
		case int:
			v.SetInt(int64(d))
		case string:
			v.SetString(d)
		}
		return nil
	}
	v.Set(reflect.Zero(v.Type()))
	return nil
}

func walkSettings(v reflect.Value, prefix string, out *[]Setting) {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			name, ok := tomlName(f)
			if !ok || !f.IsExported() {
				continue
			}
			walkSettings(v.Field(i), joinKey(prefix, name), out)
		}
	case reflect.Map:
		keys := make([]string, 0, v.Len())
		for _, k := range v.MapKeys() {
			keys = append(keys, k.String())
		}
		sort.Strings(keys)
		for _, k := range keys {
			walkSettings(v.MapIndex(reflect.ValueOf(k).Convert(v.Type().Key())), joinKey(prefix, k), out)
		}
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.String {
			if v.Len() > 0 {
				*out = append(*out, Setting{prefix, settingString(v)})
			}
			return
		}
		for i := 0; i < v.Len(); i++ {
			walkSettings(v.Index(i), joinKey(prefix, strconv.Itoa(i)), out)
		}
	default:
		if s := settingString(v); s != "" {
			*out = append(*out, Setting{prefix, s})
		}
	}
}

// ConfigKeysSet are the settings config.toml of the clone writes (dotted
// keys; a list is the key of the list): what the team chose, as opposed to
// what is a default.
func (r *Repo) ConfigKeysSet() (map[string]bool, error) {
	data, err := os.ReadFile(filepath.Join(r.path, "config.toml"))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	var m map[string]any
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing config.toml: %w", err)
	}
	out := map[string]bool{}
	var walk func(prefix string, m map[string]any)
	walk = func(prefix string, m map[string]any) {
		for k, v := range m {
			key := joinKey(prefix, k)
			if sub, ok := v.(map[string]any); ok && len(sub) > 0 {
				walk(key, sub)
				continue
			}
			out[key] = true
		}
	}
	walk("", m)
	return out, nil
}

// InFile reports whether the setting key is written in config.toml (set is
// ConfigKeysSet): the key itself, or the list or table that holds it.
func InFile(set map[string]bool, key string) bool {
	for k := key; k != ""; {
		if set[k] {
			return true
		}
		i := strings.LastIndex(k, ".")
		if i < 0 {
			break
		}
		k = k[:i]
	}
	return false
}
