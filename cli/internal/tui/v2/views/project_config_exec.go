package views

import (
	"errors"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// « Exécution » section of the project config (P4-T09, 10 §11): dev image of
// the container runtime (Dockerfile, build arguments, cache volumes) and the
// launch defaults (workflow, runtime).

// ProjectExecHints are computed off the event loop when the view mounts.
type ProjectExecHints struct {
	// DetectedDockerfile is the dev Dockerfile found in the project
	// (relative path, "" = none: oh default base image).
	DetectedDockerfile string
}

var buildArgName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ParseBuildArgs reads « KEY=value, KEY2=value » (nil when empty).
func ParseBuildArgs(s string) (map[string]string, error) {
	out := map[string]string{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, ok := strings.Cut(part, "=")
		k = strings.TrimSpace(k)
		if !ok || !buildArgName.MatchString(k) {
			return nil, errors.New(i18n.Tf("tui.pc.exec.build_args.invalid", part))
		}
		out[k] = strings.TrimSpace(v)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// FormatBuildArgs writes build arguments as « KEY=value, … » (sorted).
func FormatBuildArgs(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + m[k]
	}
	return strings.Join(parts, ", ")
}

// ParseVolumes reads « node_modules, /root/.cache » (nil when empty):
// absolute container paths or paths relative to each location, without
// `..` nor engine syntax (`:`).
func ParseVolumes(s string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		clean := filepath.ToSlash(filepath.Clean(part))
		if strings.Contains(part, ":") || clean == "." || clean == "/" || clean == ".." || strings.HasPrefix(clean, "../") {
			return nil, errors.New(i18n.Tf("tui.pc.exec.volumes.invalid", part))
		}
		if !seen[clean] {
			seen[clean] = true
			out = append(out, clean)
		}
	}
	return out, nil
}

// execConfig returns the project execution settings, created on first write.
func (v *ProjectConfigView) execConfig() *domain.ProjectExecConfig {
	if v.live.ExecConfig == nil {
		v.live.ExecConfig = &domain.ProjectExecConfig{}
	}
	return v.live.ExecConfig
}

// execGet reads a setting without creating the config.
func (v *ProjectConfigView) execGet(get func(c *domain.ProjectExecConfig) string) func() string {
	return func() string {
		if v.live.ExecConfig == nil {
			return ""
		}
		return get(v.live.ExecConfig)
	}
}

// execFields are the fields of the « Exécution » section.
func (v *ProjectConfigView) execFields() []configField {
	runtimeOptions := []SelectOption{
		{Label: i18n.T("tui.pc.exec.runtime.inherit"), Value: ""},
		{Label: i18n.T("tui.launch.runtime_local"), Value: "local"},
		{Label: i18n.T("tui.launch.runtime_container"), Value: "container"},
	}
	workflowOptions := func() []SelectOption {
		opts := []SelectOption{{Label: i18n.T("tui.pc.exec.workflow.none"), Value: ""}}
		if v.cfg.WorkflowIDs != nil {
			for _, id := range v.cfg.WorkflowIDs() {
				opts = append(opts, SelectOption{Label: id, Value: id})
			}
		}
		return opts
	}
	return []configField{
		{Kind: CfgFieldSectionHeader, Label: i18n.T("tui.pc.exec.section")},
		{Key: "exec_dockerfile", Kind: CfgFieldString, Label: i18n.T("tui.pc.exec.dockerfile.label"),
			Description: i18n.T("tui.pc.exec.dockerfile.desc"),
			Get:         v.execGet(func(c *domain.ProjectExecConfig) string { return c.Dockerfile }),
			Set:         func(val string) { v.execConfig().Dockerfile = strings.TrimSpace(val) },
			Source: func() string {
				if v.live.ExecConfig != nil && v.live.ExecConfig.Dockerfile != "" {
					return ""
				}
				if v.execHints.DetectedDockerfile != "" {
					return i18n.Tf("tui.pc.exec.dockerfile.detected", v.execHints.DetectedDockerfile)
				}
				return i18n.T("tui.pc.exec.dockerfile.none")
			}},
		{Key: "exec_build_args", Kind: CfgFieldString, Label: i18n.T("tui.pc.exec.build_args.label"),
			Description: i18n.T("tui.pc.exec.build_args.desc"),
			Validator:   &FieldValidator{Check: func(s string) error { _, err := ParseBuildArgs(s); return err }},
			Get:         v.execGet(func(c *domain.ProjectExecConfig) string { return FormatBuildArgs(c.BuildArgs) }),
			Set: func(val string) {
				if m, err := ParseBuildArgs(val); err == nil {
					v.execConfig().BuildArgs = m
				}
			}},
		{Key: "exec_volumes", Kind: CfgFieldString, Label: i18n.T("tui.pc.exec.volumes.label"),
			Description: i18n.T("tui.pc.exec.volumes.desc"),
			Validator:   &FieldValidator{Check: func(s string) error { _, err := ParseVolumes(s); return err }},
			Get:         v.execGet(func(c *domain.ProjectExecConfig) string { return strings.Join(c.Volumes, ", ") }),
			Set: func(val string) {
				if l, err := ParseVolumes(val); err == nil {
					v.execConfig().Volumes = l
				}
			}},
		{Key: "exec_default_workflow", Kind: CfgFieldSelect, Label: i18n.T("tui.pc.exec.workflow.label"),
			Description: i18n.T("tui.pc.exec.workflow.desc"),
			OptionsFunc: workflowOptions,
			Get:         v.execGet(func(c *domain.ProjectExecConfig) string { return c.DefaultWorkflow }),
			Set:         func(val string) { v.execConfig().DefaultWorkflow = val }},
		{Key: "exec_default_runtime", Kind: CfgFieldSelect, Label: i18n.T("tui.pc.exec.runtime.label"),
			Description: i18n.T("tui.pc.exec.runtime.desc"),
			Options:     runtimeOptions,
			Get:         v.execGet(func(c *domain.ProjectExecConfig) string { return c.DefaultRuntime }),
			Set:         func(val string) { v.execConfig().DefaultRuntime = val }},
	}
}
