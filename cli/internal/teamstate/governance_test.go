package teamstate

import (
	"os"
	"path/filepath"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGovernance_PublishPolicy(t *testing.T) {
	assert.Equal(t, GovernancePublishAnyMember, GovernanceConfig{}.PublishPolicy())
	assert.Equal(t, GovernancePublishAnyMember, DefaultGovernance().PublishPolicy())
	assert.Equal(t, "leads", GovernanceConfig{Publish: "leads"}.PublishPolicy())
}

func TestGovernance_CanPublish(t *testing.T) {
	members := func(id string) bool { return id == "alice" }
	assert.NoError(t, GovernanceConfig{}.CanPublish("alice", members))
	assert.ErrorIs(t, GovernanceConfig{}.CanPublish("mallory", members), ErrNotMember)
	assert.ErrorIs(t, GovernanceConfig{}.CanPublish("", members), ErrNotMember)
	// Unknown policies fail closed.
	err := GovernanceConfig{Publish: "leads"}.CanPublish("alice", members)
	assert.ErrorIs(t, err, ErrGovernanceUnsupported)
	assert.Contains(t, err.Error(), "leads")
}

func TestGovernance_ConfigTOML(t *testing.T) {
	repo := setupTestRepo(t)
	writeMembersToml(t, repo, "[members.alice]\ndisplay_name = \"Alice\"\n")

	// No config.toml: any member may publish.
	require.NoError(t, repo.CheckPublish("alice"))
	assert.ErrorIs(t, repo.CheckPublish("bob"), ErrNotMember)

	require.NoError(t, os.WriteFile(filepath.Join(repo.Path(), "config.toml"), []byte("[governance]\npublish = \"any_member\"\n"), 0o644))
	cfg, err := repo.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, GovernancePublishAnyMember, cfg.Governance.Publish)
	require.NoError(t, repo.CheckPublish("alice"))

	require.NoError(t, os.WriteFile(filepath.Join(repo.Path(), "config.toml"), []byte("[governance]\npublish = \"two_reviewers\"\n"), 0o644))
	_, err = repo.LoadConfig()
	require.NoError(t, err, "an unknown policy must not break the config")
	assert.ErrorIs(t, repo.CheckPublish("alice"), ErrGovernanceUnsupported)

	// Round trip: the section is written when set, omitted when empty.
	data, err := toml.Marshal(&TeamConfig{Governance: DefaultGovernance()})
	require.NoError(t, err)
	assert.Contains(t, string(data), "[governance]")
	assert.Contains(t, string(data), "publish = 'any_member'")
	data, err = toml.Marshal(&TeamConfig{})
	require.NoError(t, err)
	assert.NotContains(t, string(data), "governance")
}
