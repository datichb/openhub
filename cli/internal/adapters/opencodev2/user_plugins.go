package opencodev2

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// userConfigFiles are the files of the user tool configuration that may list
// plugins (`"plugin": [...]`), by order of precedence.
var userConfigFiles = []string{"opencode.json", "opencode.jsonc", "config.json"}

// userPluginDirs are the directories of the user tool configuration whose
// files the server loads as plugins.
var userPluginDirs = []string{"plugin", "plugins"}

// UserPlugins lists the plugins of the user's own tool configuration
// ($XDG_CONFIG_HOME/opencode): the entries of `plugin` in the global config
// and the files of its plugin directories. They run in the local sessions
// unless `execution.strict_isolation` hides the configuration (S-11); a
// container never sees them. An absent configuration lists nothing.
func (a *Adapter) UserPlugins() ([]string, error) {
	dir, err := userConfigPath()
	if err != nil {
		return nil, err
	}
	found := map[string]bool{}
	for _, name := range userConfigFiles {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		for _, p := range configPlugins(data) {
			found[p] = true
		}
	}
	for _, name := range userPluginDirs {
		entries, err := os.ReadDir(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasPrefix(e.Name(), ".") {
				found[filepath.Join(name, e.Name())] = true
			}
		}
	}
	out := make([]string, 0, len(found))
	for p := range found {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}

// userConfigPath is the opencode directory of the user config home.
func userConfigPath() (string, error) {
	user := os.Getenv("XDG_CONFIG_HOME")
	if user == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		user = filepath.Join(home, ".config")
	}
	return filepath.Join(user, toolConfigDir), nil
}

// configPlugins returns the plugins named by a configuration file (JSON with
// comments and trailing commas allowed): a string, or a [name, options] pair.
func configPlugins(data []byte) []string {
	var cfg struct {
		Plugin []json.RawMessage `json:"plugin"`
	}
	if json.Unmarshal(stripJSONC(data), &cfg) != nil {
		return nil
	}
	var out []string
	for _, raw := range cfg.Plugin {
		var name string
		var pair []json.RawMessage
		switch {
		case json.Unmarshal(raw, &name) == nil:
		case json.Unmarshal(raw, &pair) == nil && len(pair) > 0 && json.Unmarshal(pair[0], &name) == nil:
		}
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

// stripJSONC removes the comments and the trailing commas of a JSONC
// document, leaving strings untouched.
func stripJSONC(src []byte) []byte {
	out := make([]byte, 0, len(src))
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case c == '"':
			j := i + 1
			for j < len(src) && src[j] != '"' {
				if src[j] == '\\' {
					j++
				}
				j++
			}
			if j >= len(src) {
				j = len(src) - 1
			}
			out = append(out, src[i:j+1]...)
			i = j
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
			out = append(out, '\n')
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			i += 2
			for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
				i++
			}
			i++
		case c == ',':
			j := i + 1
			for j < len(src) && (src[j] == ' ' || src[j] == '\t' || src[j] == '\n' || src[j] == '\r') {
				j++
			}
			if j < len(src) && (src[j] == ']' || src[j] == '}') {
				continue
			}
			out = append(out, c)
		default:
			out = append(out, c)
		}
	}
	return out
}
