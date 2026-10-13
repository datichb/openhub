package opencodev2

import (
	"context"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/adapters"
)

// Ambient instructions of opencode V2 (Q35, checked on opencode 2.0.20 and
// https://opencode.ai/v2/docs/instructions/):
//
//   - The AGENTS.md of the workspace and of its parents (up to the project
//     root or the home folder) are loaded by the plugin
//     opencode.config.instruction only when the project config is on: oh
//     starts the server with OPENCODE_DISABLE_PROJECT_CONFIG=1 (buildEnv,
//     runtimeEnv), so they never reach an oh session.
//   - The read tool (opencode.tool.read) loads every AGENTS.md between the
//     file or folder read and the workspace, the workspace itself excepted
//     (nested discovery). It ignores that variable and no setting turns it
//     off: an AGENTS.md of a subfolder reaches the prompt as soon as an
//     agent reads under it.
//   - The global $XDG_CONFIG_HOME/opencode/AGENTS.md is loaded unless the
//     config home is isolated (strict isolation: isolatedConfigHome leaves
//     the opencode folder out; never visible in a container).

const instructionFile = "AGENTS.md"

var _ adapters.AmbientInstructions = (*Adapter)(nil)

// ProjectAmbientInstructions implements adapters.AmbientInstructions: the
// AGENTS.md of the subfolders of dir (tracked files and untracked ones not
// ignored by git). Not a git work tree: none.
func (a *Adapter) ProjectAmbientInstructions(ctx context.Context, dir string) []string {
	if dir == "" {
		return nil
	}
	if _, err := os.Stat(dir); err != nil {
		return nil
	}
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", ":(glob)**/"+instructionFile)
	data, err := cmd.Output()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, f := range strings.Split(string(data), "\x00") {
		f = filepath.ToSlash(f)
		if f == "" || f == instructionFile || seen[f] || path.Base(f) != instructionFile {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// UserAmbientInstructions implements adapters.AmbientInstructions: the
// global AGENTS.md of the user config home, when it exists.
func (a *Adapter) UserAmbientInstructions() []string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		dir = filepath.Join(home, ".config")
	}
	p := filepath.Join(dir, toolConfigDir, instructionFile)
	if _, err := os.Stat(p); err != nil {
		return nil
	}
	return []string{p}
}
