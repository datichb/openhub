package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// providerOnDisk is a Provider view over a hub.toml of a temporary home: what
// the view changes is read back from the file. update is the host's write path
// (config.UpdateAndGet, then the live config follows the file).
func providerOnDisk(t *testing.T, seed func(*config.Config)) (*ProviderView, *viewShell, *config.Config) {
	t.Helper()
	frLocale(t)
	hubConfig(t, seed)
	live, err := config.Read()
	require.NoError(t, err)
	v := NewProviderView(nil, ProviderViewConfig{
		GetConfig: func() *config.Config { return live },
		UpdateConfig: func(fn func(*config.Config) error) error {
			fresh, err := config.UpdateAndGet(fn)
			if err == nil {
				*live = *fresh
			}
			return err
		},
	})
	sh := &viewShell{}
	v.SetShell(sh)
	v.list = widgets.NewSectionedList()
	v.buildLines()
	v.renderList()
	return v, sh, live
}

// selectProviderLine puts the cursor on the first line of the view that matches.
func selectProviderLine(t *testing.T, v *ProviderView, match func(providerConfigLine) bool) {
	t.Helper()
	for i, it := range v.list.GetItems() {
		if ref, ok := it.Reference.(int); ok && match(v.lines[ref]) {
			v.list.SelectIndex(i)
			return
		}
	}
	t.Fatal("line not shown")
}

func onDisk(t *testing.T) *config.Config {
	t.Helper()
	c, err := config.Read()
	require.NoError(t, err)
	return c
}

// Space on a provider makes it the default one: the choice is in hub.toml, not
// only in the config in memory (which the next write of the file replaces).
func TestProviderViewDefaultIsWrittenToHubToml(t *testing.T) {
	v, sh, live := providerOnDisk(t, func(c *config.Config) { c.LLM.DefaultProvider = "bedrock" })
	selectProviderLine(t, v, func(l providerConfigLine) bool { return l.kind == "provider-item" && l.provider == "anthropic" })

	assert.Nil(t, v.HandleKey(runeKey(' ')))

	assert.Equal(t, "anthropic", onDisk(t).LLM.DefaultProvider, "written to hub.toml")
	assert.Equal(t, "anthropic", live.LLM.DefaultProvider, "the live config follows")
	assert.Contains(t, sh.lastToast(), "anthropic")
}

// A write that fails says so and does not pretend the choice was made.
func TestProviderViewDefaultWriteFailureIsShown(t *testing.T) {
	v, sh, live := providerOnDisk(t, func(c *config.Config) { c.LLM.DefaultProvider = "bedrock" })
	v.vcfg.UpdateConfig = func(func(*config.Config) error) error { return assert.AnError }
	selectProviderLine(t, v, func(l providerConfigLine) bool { return l.kind == "provider-item" && l.provider == "anthropic" })

	v.HandleKey(runeKey(' '))

	assert.Equal(t, "bedrock", onDisk(t).LLM.DefaultProvider)
	assert.Equal(t, "bedrock", live.LLM.DefaultProvider, "the live config is not changed behind the file's back")
	assert.Contains(t, sh.lastToast(), assert.AnError.Error())
}

// The fields of the provider (region…) go through the same write path.
func TestProviderViewFieldEditIsWrittenToHubToml(t *testing.T) {
	v, sh, _ := providerOnDisk(t, func(c *config.Config) { c.Provider.Bedrock.AWSRegion = "eu-west-1" })
	selectProviderLine(t, v, func(l providerConfigLine) bool { return l.key == "aws_region" })
	v.editCurrent()
	require.NotNil(t, sh.form)

	sh.form.OnSubmit(map[string]string{"value": "eu-west-3"}, nil)

	assert.Equal(t, "eu-west-3", onDisk(t).Provider.Bedrock.AWSRegion)
}

// Reset clears the provider in hub.toml (and the default if it was the one).
func TestProviderViewResetIsWrittenToHubToml(t *testing.T) {
	v, sh, _ := providerOnDisk(t, func(c *config.Config) {
		c.LLM.DefaultProvider = "bedrock"
		c.Provider.Bedrock.AWSRegion = "eu-west-1"
	})
	v.selectedProvider = "bedrock"

	v.HandleKey(runeKey('d'))
	sh.confirmed(t, nil)

	disk := onDisk(t)
	assert.Empty(t, disk.Provider.Bedrock.AWSRegion)
	assert.Empty(t, disk.LLM.DefaultProvider)
}
