package main

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type goreleaserConfig struct {
	Brews         []yaml.Node  `yaml:"brews"`
	HomebrewCasks []caskConfig `yaml:"homebrew_casks"`
}

type caskConfig struct {
	Name         string   `yaml:"name"`
	Directory    string   `yaml:"directory"`
	SkipUpload   string   `yaml:"skip_upload"`
	Binaries     []string `yaml:"binaries"`
	Caveats      string   `yaml:"caveats"`
	Dependencies []struct {
		Formula string `yaml:"formula"`
		Cask    string `yaml:"cask"`
	} `yaml:"dependencies"`
	Repository struct {
		Owner string `yaml:"owner"`
		Name  string `yaml:"name"`
		Token string `yaml:"token"`
	} `yaml:"repository"`
	Hooks struct {
		Post struct {
			InstallSteps string `yaml:"install_steps"`
		} `yaml:"post"`
	} `yaml:"hooks"`
}

func TestGoreleaserPublishesACask(t *testing.T) {
	raw, err := os.ReadFile(".goreleaser.yml")
	require.NoError(t, err)

	var cfg goreleaserConfig
	require.NoError(t, yaml.Unmarshal(raw, &cfg))

	assert.Empty(t, cfg.Brews, "brews is deprecated: the tap gets a cask (homebrew_casks)")
	require.Len(t, cfg.HomebrewCasks, 1)
	cask := cfg.HomebrewCasks[0]

	assert.Equal(t, "openhub", cask.Name)
	assert.Equal(t, "Casks", cask.Directory)
	assert.Equal(t, "auto", cask.SkipUpload, "a pre-release tag must not update the tap")
	assert.Equal(t, []string{"oh"}, cask.Binaries)
	assert.Equal(t, "datichb", cask.Repository.Owner)
	assert.Equal(t, "homebrew-tap", cask.Repository.Name)
	assert.Equal(t, "{{ .Env.HOMEBREW_TAP_TOKEN }}", cask.Repository.Token, "the token stays an environment reference")

	var formulas []string
	for _, d := range cask.Dependencies {
		assert.Empty(t, d.Cask)
		formulas = append(formulas, d.Formula)
	}
	// The session tool is the second required formula (BL-22); archtest keeps
	// its name out of the Go code, so only git is named here.
	assert.Len(t, formulas, 2, "git and the session tool are required, nothing else")
	assert.Contains(t, formulas, "git")

	for _, tool := range []string{"brew install beads", "brew install terminal-notifier"} {
		assert.Contains(t, cask.Caveats, tool, "a cask has no recommended dependency: the caveats advertise it")
	}

	steps := cask.Hooks.Post.InstallSteps
	assert.True(t, strings.Contains(steps, "com.apple.quarantine") && strings.Contains(steps, "on_macos"),
		"oh is not notarized: the quarantine is removed on macOS only")
}
