package cmd

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// legacySkillsCheck warns about the packages left in ~/.oh/skills by the
// former community registry, disconnected in v5 (ADR-051): they are neither
// shipped in the session bundles nor checked any more. Nothing is deleted.
func legacySkillsCheck() views.DoctorCheck {
	name := i18n.T("cmd.doctor.legacy_skills.name")
	dir := filepath.Join(config.HubDir(), "skills")
	names := legacySkillPackages(dir)
	if len(names) == 0 {
		return views.DoctorCheck{Name: name, OK: true, Detail: i18n.T("cmd.doctor.legacy_skills.none")}
	}
	return views.DoctorCheck{Name: name, OK: true, Warn: true,
		Detail: i18n.Tf("cmd.doctor.legacy_skills.found", len(names), dir, strings.Join(names, ", "))}
}

// legacySkillPackages lists the folders of dir holding a manifest.json (a
// package of the former registry).
func legacySkillPackages(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, e.Name(), "manifest.json")); err == nil {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}
