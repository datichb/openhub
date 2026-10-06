package opencodev2

import (
	"fmt"
	"os"
	"path/filepath"
)

// toolConfigDir is the opencode directory of the user config home
// ($XDG_CONFIG_HOME/opencode): global config, plugins, agents, commands.
const toolConfigDir = "opencode"

// isolatedConfigHome prepares the config home of a server group under strict
// isolation: a directory mirroring the user config home (one symlink per
// entry: git, gh… keep working) without its opencode directory. The links
// are rebuilt at each start (entries added or removed meanwhile).
func isolatedConfigHome(dir string) (string, error) {
	user := os.Getenv("XDG_CONFIG_HOME")
	if user == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		user = filepath.Join(home, ".config")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	old, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range old {
		if e.Type()&os.ModeSymlink != 0 {
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
				return "", err
			}
		}
	}
	entries, err := os.ReadDir(user)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("reading the user config home: %w", err)
	}
	for _, e := range entries {
		if e.Name() == toolConfigDir {
			continue
		}
		dst := filepath.Join(dir, e.Name())
		if _, err := os.Lstat(dst); err == nil {
			continue // not a link: left as is
		}
		if err := os.Symlink(filepath.Join(user, e.Name()), dst); err != nil {
			return "", err
		}
	}
	return dir, nil
}
