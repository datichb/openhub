package cmd

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
)

// legacySkillsCheck warns about the packages left in ~/.oh/skills by the
// former community registry, disconnected in v5 (ADR-051): they are neither
// shipped in the session bundles nor checked any more. Nothing is deleted. A
// folder that cannot be read is a warning too: "none" would say it was empty.
func legacySkillsCheck() doctor.Result {
	name := i18n.T("cmd.doctor.legacy_skills.name")
	dir := filepath.Join(config.HubDir(), "skills")
	names, err := legacySkillPackages(dir)
	if err != nil {
		detail := i18n.Tf("cmd.doctor.legacy_skills.unreadable", dir, err)
		if len(names) > 0 {
			detail += " — " + i18n.Tf("cmd.doctor.legacy_skills.found", len(names), dir, strings.Join(names, ", "))
		}
		return doctor.Result{Name: name, OK: true, Warn: true, Detail: detail}
	}
	if len(names) == 0 {
		return doctor.Result{Name: name, OK: true, Detail: i18n.T("cmd.doctor.legacy_skills.none")}
	}
	return doctor.Result{Name: name, OK: true, Warn: true,
		Detail: i18n.Tf("cmd.doctor.legacy_skills.found", len(names), dir, strings.Join(names, ", "))}
}

// legacySkillPackages lists the folders of dir holding a manifest.json (a
// package of the former registry). A folder that does not exist has none; any
// other failure to read dir, or a manifest.json that cannot be looked at, is
// an error.
func legacySkillPackages(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	var unreadable []error
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		_, err := os.Stat(filepath.Join(dir, e.Name(), "manifest.json"))
		switch {
		case err == nil:
			names = append(names, e.Name())
		case !errors.Is(err, fs.ErrNotExist):
			unreadable = append(unreadable, err)
		}
	}
	sort.Strings(names)
	return names, errors.Join(unreadable...)
}
