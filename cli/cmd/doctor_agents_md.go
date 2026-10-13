package cmd

import (
	"context"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Doctor › AGENTS.md (Q35): what opencode V2 adds to the prompts of an oh
// session outside the session bundle.
//
//   - The AGENTS.md of the project root and of its parents is not loaded:
//     oh starts opencode with OPENCODE_DISABLE_PROJECT_CONFIG=1, which turns
//     off the project instruction discovery.
//   - An AGENTS.md of a subfolder is still loaded by the read tool when an
//     agent reads or lists a file under it (nested discovery, not covered by
//     the variable, and no setting turns it off): reported here, per project.
//   - The global ~/.config/opencode/AGENTS.md is loaded in local sessions
//     unless [execution] strict_isolation hides the user config home of
//     opencode (never visible in a container or a remote job).

func init() { registerDoctorCheck(agentsMDDoctorChecks) }

func agentsMDDoctorChecks(ctx context.Context) []views.DoctorCheck {
	name := i18n.T("cmd.doctor.agents_md")
	var out []views.DoctorCheck
	var found []string
	if a := TryApp(); a != nil && a.Projects != nil {
		if list, err := a.Projects.List(ctx, domain.ProjectStatusActive); err == nil {
			for _, p := range list {
				if files := nestedAgentsMD(ctx, p.Path); len(files) > 0 {
					found = append(found, p.Name+" ("+strings.Join(files, ", ")+")")
				}
			}
		}
	}
	if len(found) == 0 {
		out = append(out, views.DoctorCheck{Name: name, OK: true, Detail: i18n.T("cmd.doctor.agents_md.none")})
	} else {
		out = append(out, views.DoctorCheck{Name: name, OK: true, Warn: true,
			Detail: i18n.Tf("cmd.doctor.agents_md.nested", strings.Join(found, " · "))})
	}
	if p := globalAgentsMD(); p != "" {
		out = append(out, views.DoctorCheck{Name: name, OK: true, Warn: true,
			Detail: i18n.Tf("cmd.doctor.agents_md.global", p)})
	}
	return out
}

// nestedAgentsMD returns the AGENTS.md files of a project below its root
// (relative paths, sorted): tracked files and untracked ones not ignored by
// git. Not a git work tree: none.
func nestedAgentsMD(ctx context.Context, root string) []string {
	if root == "" {
		return nil
	}
	if _, err := os.Stat(root); err != nil {
		return nil
	}
	cmd := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", ":(glob)**/AGENTS.md")
	data, err := cmd.Output()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, f := range strings.Split(string(data), "\x00") {
		f = filepath.ToSlash(f)
		if f == "" || f == "AGENTS.md" || seen[f] || path.Base(f) != "AGENTS.md" {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// globalAgentsMD returns the path of the user AGENTS.md of opencode when it
// exists ("" otherwise).
func globalAgentsMD() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".config")
	}
	p := filepath.Join(dir, "opencode", "AGENTS.md")
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}
