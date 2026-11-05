package team

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/mcp/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupHandlerTest creates a fully functional team-state repo and project dir
// for integration-testing the MCP handlers. Returns a cleanup function.
func setupHandlerTest(t *testing.T) string {
	t.Helper()
	resetRepoCache()

	// Create team-state directory structure
	teamState := gittest.Repo(t)

	// members.toml
	membersContent := `
[members.benjamin]
display_name = "Benjamin D"
gitlab_username = "bdatiche"
mattermost_username = "benjamin"
role = "lead"
default_mode = "semi-auto"

[members.alice]
display_name = "Alice M"
gitlab_username = "alicem"
mattermost_username = "alice"
role = "dev"
default_mode = "manual"
`
	require.NoError(t, os.WriteFile(filepath.Join(teamState, "members.toml"), []byte(membersContent), 0o644))

	// config.toml
	configContent := `
[notification]
enabled = false
`
	require.NoError(t, os.WriteFile(filepath.Join(teamState, "config.toml"), []byte(configContent), 0o644))

	// policies.toml
	policiesContent := `
[policies.branch_naming]
type = "regex"
enforcement = "warn"
rule = "^(feat|fix|chore)/.+"
message = "Branch must match pattern"
`
	require.NoError(t, os.WriteFile(filepath.Join(teamState, "policies.toml"), []byte(policiesContent), 0o644))

	// Wiki pages
	wikiDir := filepath.Join(teamState, "wiki")
	require.NoError(t, os.MkdirAll(filepath.Join(wikiDir, ".pending"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(wikiDir, "conventions.md"), []byte("# Conventions\n\nUse Conventional Commits."), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(wikiDir, "decisions.md"), []byte("# Decisions\n\nArchitecture decisions log."), 0o644))

	// Claims
	claimsDir := filepath.Join(teamState, "projects", "myproject", "claims")
	require.NoError(t, os.MkdirAll(claimsDir, 0o755))
	claim1 := fmt.Sprintf(`claimed_by = "benjamin"
claimed_at = %s
status = "in_progress"
worktree = "feat/SRU-142"
`, time.Now().UTC().Add(-2*time.Hour).Format("2006-01-02T15:04:05Z"))
	claim2 := fmt.Sprintf(`claimed_by = "alice"
claimed_at = %s
status = "review"
labels = ["agent-reviewed"]
`, time.Now().UTC().Add(-5*time.Hour).Format("2006-01-02T15:04:05Z"))
	require.NoError(t, os.WriteFile(filepath.Join(claimsDir, "SRU-142.toml"), []byte(claim1), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(claimsDir, "BD-99.toml"), []byte(claim2), 0o644))

	// Events
	eventsDir := filepath.Join(teamState, "projects", "myproject", "events")
	require.NoError(t, os.MkdirAll(eventsDir, 0o755))
	now := time.Now().UTC()
	month := now.Format("2006-01")
	event1 := fmt.Sprintf(`{"event":"claim.taken","project":"myproject","actor":"benjamin","ticket":"SRU-142","ts":"%s"}`, now.Add(-2*time.Hour).Format(time.RFC3339))
	event2 := fmt.Sprintf(`{"event":"session.complete","project":"myproject","actor":"alice","ticket":"BD-99","ts":"%s","data":{"duration_min":45}}`, now.Add(-1*time.Hour).Format(time.RFC3339))
	eventsContent := event1 + "\n" + event2 + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(eventsDir, month+".jsonl"), []byte(eventsContent), 0o644))

	// Patterns
	patternsDir := filepath.Join(teamState, "patterns")
	require.NoError(t, os.MkdirAll(patternsDir, 0o755))
	indexContent := `
[[patterns]]
name = "crud-api"
tags = ["api", "crud", "backend"]
complexity = "M"
validated = true
created_at = "2026-07-01"

[[patterns]]
name = "db-migration"
tags = ["database", "migration"]
complexity = "S"
validated = false
created_at = "2026-07-15"
`
	require.NoError(t, os.WriteFile(filepath.Join(patternsDir, "index.toml"), []byte(indexContent), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(patternsDir, "crud-api.md"), []byte("# CRUD API Pattern\n\nSteps:\n1. Define model\n2. Create handler\n3. Add routes"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(patternsDir, "db-migration.md"), []byte("# DB Migration Pattern\n\nSteps:\n1. Create migration file"), 0o644))

	// Takeover briefs
	briefsDir := filepath.Join(teamState, "projects", "myproject", "takeover-briefs")
	require.NoError(t, os.MkdirAll(briefsDir, 0o755))
	briefContent := "# Takeover Brief: SRU-142\n\n## Context\nWorking on user auth.\n\n## Next Steps\n- Finish validation"
	require.NoError(t, os.WriteFile(filepath.Join(briefsDir, "SRU-142_2026-07-20.md"), []byte(briefContent), 0o644))

	// Commit everything
	gittest.Run(t, teamState, "add", ".")
	gittest.Run(t, teamState, "commit", "-m", "initial state")

	projectDir := t.TempDir()
	useSessionTeam(t, effectiveTeam{
		Enabled:   true,
		StateRepo: teamState, // point to the local dir (acting as both remote and local)
		StatePath: teamState,
		MemberID:  "benjamin",
	})

	chdir(t, projectDir)
	return teamState
}

// --- Handler integration tests ---

// The read handlers, and the writes they refuse before touching the repo, on
// one fixture; a name that would leave the team-state is refused by each.
func TestHandlers(t *testing.T) {
	setupHandlerTest(t)

	tests := []struct {
		name    string
		handler func(context.Context, json.RawMessage) (*protocol.ToolResult, error)
		args    string
		want    []string // in the text of the result
		wantErr string   // in the error ("" : no error)
	}{
		{"members", handleTeamMembers, `{}`, []string{"benjamin", "alice"}, ""},
		{"claims", handleTeamClaims, `{"project":"myproject"}`, []string{"SRU-142", "BD-99"}, ""},
		{"claims of an unknown project", handleTeamClaims, `{"project":"nonexistent"}`, nil, ""},
		{"wiki list", handleTeamWikiList, `{}`, []string{"conventions", "decisions"}, ""},
		{"wiki read", handleTeamWikiRead, `{"page":"conventions"}`, []string{"Conventional Commits"}, ""},
		{"wiki read of an unknown page", handleTeamWikiRead, `{"page":"nonexistent"}`, nil, "not found"},
		{"wiki read refuses ../", handleTeamWikiRead, `{"page":"../etc/passwd"}`, nil, "unsafe"},
		{"events", handleTeamEvents, `{"project":"myproject","limit":10}`, []string{"benjamin", "alice"}, ""},
		{"events with a huge limit are capped, not refused", handleTeamEvents, `{"project":"myproject","limit":999999}`, nil, ""},
		{"policies", handleTeamPolicies, `{}`, []string{"branch_naming"}, ""},
		{"takeover brief", handleTeamTakeoverBrief, `{"project":"myproject","ticket_id":"SRU-142"}`, []string{"user auth"}, ""},
		{"takeover brief refuses ../", handleTeamTakeoverBrief, `{"project":"../etc","ticket_id":"passwd"}`, nil, "unsafe"},
		{"patterns list", handleTeamPatternsList, `{}`, []string{"crud-api", "db-migration"}, ""},
		{"patterns read", handleTeamPatternsRead, `{"name":"crud-api"}`, []string{"CRUD API Pattern"}, ""},
		{"patterns read refuses ../", handleTeamPatternsRead, `{"name":"../../etc/passwd"}`, nil, "unsafe"},
		{"wiki write refuses ../", handleTeamWikiWrite, `{"page":"../../../etc/passwd","content":"evil","confidence":"CONFIRMED"}`, nil, "invalid"},
		{"wiki write needs a page", handleTeamWikiWrite, `{"page":"","content":"stuff","confidence":"CONFIRMED","project":"x"}`, nil, "page is required"},
		{"wiki write needs a known confidence", handleTeamWikiWrite, `{"page":"test","content":"stuff","confidence":"MAYBE","project":"x"}`, nil, "CONFIRMED, INFERRED, or UNCERTAIN"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.handler(context.Background(), json.RawMessage(tt.args))
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Len(t, result.Content, 1)
			for _, w := range tt.want {
				assert.Contains(t, result.Content[0].Text, w)
			}
		})
	}
}

// setupHandlerWriteTest creates a team-state repo backed by a bare remote, enabling
// commitAndPush to succeed. Returns the clone (state) path for file assertions.
func setupHandlerWriteTest(t *testing.T) string {
	t.Helper()
	resetRepoCache()

	bare, teamState := gittest.BareClone(t)

	// members.toml
	membersContent := `
[members.benjamin]
display_name = "Benjamin D"
gitlab_username = "bdatiche"
mattermost_username = "benjamin"
role = "lead"
default_mode = "semi-auto"

[members.alice]
display_name = "Alice M"
gitlab_username = "alicem"
mattermost_username = "alice"
role = "dev"
default_mode = "manual"
`
	require.NoError(t, os.WriteFile(filepath.Join(teamState, "members.toml"), []byte(membersContent), 0o644))

	// config.toml — notifications disabled by default
	configContent := `
[notification]
enabled = false
`
	require.NoError(t, os.WriteFile(filepath.Join(teamState, "config.toml"), []byte(configContent), 0o644))

	// Wiki
	wikiDir := filepath.Join(teamState, "wiki")
	require.NoError(t, os.MkdirAll(filepath.Join(wikiDir, ".pending"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(wikiDir, "conventions.md"), []byte("# Conventions\n\nUse Conventional Commits."), 0o644))

	// Patterns
	patternsDir := filepath.Join(teamState, "patterns")
	require.NoError(t, os.MkdirAll(patternsDir, 0o755))
	indexContent := `
[[patterns]]
name = "crud-api"
tags = ["api", "crud", "backend"]
complexity = "M"
validated = true
created_at = "2026-07-01"
`
	require.NoError(t, os.WriteFile(filepath.Join(patternsDir, "index.toml"), []byte(indexContent), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(patternsDir, "crud-api.md"), []byte("# CRUD API\n\nSteps..."), 0o644))

	// Commit all structure
	gittest.Run(t, teamState, "add", ".")
	gittest.Run(t, teamState, "commit", "-m", "initial state")
	gittest.Run(t, teamState, "push")

	projectDir := t.TempDir()
	useSessionTeam(t, effectiveTeam{
		Enabled:   true,
		StateRepo: bare,
		StatePath: teamState,
		MemberID:  "benjamin",
	})

	chdir(t, projectDir)
	return teamState
}

// ─── Write handler tests ─────────────────────────────────────────────────

func TestHandleTeamWikiWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	stateDir := setupHandlerWriteTest(t)

	payload := `{
		"page": "architecture",
		"content": "## Hexagonal Architecture\n\nAll services must follow hexagonal architecture.",
		"confidence": "CONFIRMED",
		"project": "myproject"
	}`
	result, err := handleTeamWikiWrite(context.Background(), json.RawMessage(payload))
	require.NoError(t, err)
	require.Len(t, result.Content, 1)
	assert.Contains(t, result.Content[0].Text, "Proposal created")
	assert.Contains(t, result.Content[0].Text, "architecture")

	// Verify file was created in .pending/
	pendingDir := filepath.Join(stateDir, "wiki", ".pending")
	entries, err := os.ReadDir(pendingDir)
	require.NoError(t, err)
	assert.NotEmpty(t, entries, "expected a proposal file in .pending/")

	// Verify content
	data, err := os.ReadFile(filepath.Join(pendingDir, entries[0].Name()))
	require.NoError(t, err)
	assert.Contains(t, string(data), "Hexagonal Architecture")
	assert.Contains(t, string(data), "CONFIRMED")
}

func TestHandleTeamNotify_Disabled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	setupHandlerWriteTest(t)

	// The team turned notifications off: the agent is told nothing was sent.
	result, err := handleTeamNotify(context.Background(), json.RawMessage(`{"message":"hello team"}`))
	require.NoError(t, err)
	require.Len(t, result.Content, 1)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "disabled")
	assert.Contains(t, result.Content[0].Text, "nothing was sent")
	assert.NotContains(t, result.Content[0].Text, "Notification sent")
}

// enableNotify turns the notifications of the test team on, towards a fake
// webhook whose messages it returns.
func enableNotify(t *testing.T, stateDir string, code int) *[]string {
	t.Helper()
	var mu sync.Mutex
	var texts []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m struct {
			Text string `json:"text"`
		}
		_ = json.NewDecoder(r.Body).Decode(&m)
		if code != http.StatusOK {
			w.WriteHeader(code)
			return
		}
		mu.Lock()
		texts = append(texts, m.Text)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(ts.Close)
	configContent := fmt.Sprintf(`
[notification]
enabled = true
type = "mattermost"
webhook_url = "%s/hooks/test"
channel = "test-channel"
bot_name = "TestBot"
`, ts.URL)
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "config.toml"), []byte(configContent), 0o644))
	return &texts
}

func TestHandleTeamNotify_Enabled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	stateDir := setupHandlerWriteTest(t)
	texts := enableNotify(t, stateDir, http.StatusOK)
	notifyLimit = &rateLimit{max: notifyBurst, window: notifyBurstEvery}
	t.Setenv(EnvProjectID, "myproject")

	result, err := handleTeamNotify(context.Background(), json.RawMessage(`{"message":"deploy complete @channel"}`))
	require.NoError(t, err)
	require.Len(t, result.Content, 1)
	assert.False(t, result.IsError, result.Content[0].Text)
	assert.Contains(t, result.Content[0].Text, "Notification sent to 1 destination")

	require.Len(t, *texts, 1, "the webhook was called")
	got := (*texts)[0]
	assert.Contains(t, got, "deploy complete")
	assert.Contains(t, got, "myproject", "signed with the project")
	assert.Contains(t, got, "🤖")
	assert.NotContains(t, got, "@channel", "an agent does not ping the whole channel")
}

func TestHandleTeamNotify_Limits(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	stateDir := setupHandlerWriteTest(t)
	texts := enableNotify(t, stateDir, http.StatusOK)
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	notifyLimit = &rateLimit{max: 2, window: 10 * time.Minute, now: func() time.Time { return now }}
	t.Cleanup(func() { notifyLimit = &rateLimit{max: notifyBurst, window: notifyBurstEvery} })
	ctx := context.Background()

	empty, err := handleTeamNotify(ctx, json.RawMessage(`{"message":"   "}`))
	require.NoError(t, err)
	assert.True(t, empty.IsError)
	long, err := handleTeamNotify(ctx, json.RawMessage(`{"message":"`+strings.Repeat("é", maxNotifyRunes+1)+`"}`))
	require.NoError(t, err)
	assert.True(t, long.IsError)
	assert.Contains(t, long.Content[0].Text, "too long")
	assert.Empty(t, *texts, "a refused message sends nothing and does not use the allowance")

	for i := 0; i < 2; i++ {
		r, err := handleTeamNotify(ctx, json.RawMessage(`{"message":"one"}`))
		require.NoError(t, err)
		assert.False(t, r.IsError)
	}
	r, err := handleTeamNotify(ctx, json.RawMessage(`{"message":"three"}`))
	require.NoError(t, err)
	assert.True(t, r.IsError)
	assert.Contains(t, r.Content[0].Text, "Too many notifications")
	assert.Len(t, *texts, 2)

	now = now.Add(11 * time.Minute)
	r, err = handleTeamNotify(ctx, json.RawMessage(`{"message":"later"}`))
	require.NoError(t, err)
	assert.False(t, r.IsError, "the window passed")
}

func TestHandleTeamNotify_RevokedWebhookIsReported(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	stateDir := setupHandlerWriteTest(t)
	enableNotify(t, stateDir, http.StatusNotFound)
	notifyLimit = &rateLimit{max: notifyBurst, window: notifyBurstEvery}

	r, err := handleTeamNotify(context.Background(), json.RawMessage(`{"message":"hello"}`))
	require.NoError(t, err)
	assert.True(t, r.IsError)
	assert.Contains(t, r.Content[0].Text, "not delivered")
	assert.Contains(t, r.Content[0].Text, "404")
	assert.NotContains(t, r.Content[0].Text, "/hooks/test", "the webhook is never shown")
}

func TestHandleTeamPatternsPropose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	stateDir := setupHandlerWriteTest(t)

	payload := `{
		"name": "event-sourcing",
		"tags": ["architecture", "cqrs"],
		"complexity": "L",
		"project": "myproject",
		"content": "# Event Sourcing\n\nPattern for event-driven state management."
	}`
	result, err := handleTeamPatternsPropose(context.Background(), json.RawMessage(payload))
	require.NoError(t, err)
	require.Len(t, result.Content, 1)
	assert.Contains(t, result.Content[0].Text, "event-sourcing")
	assert.Contains(t, result.Content[0].Text, "proposed")

	// Verify pattern file created
	mdPath := filepath.Join(stateDir, "patterns", "event-sourcing.md")
	data, err := os.ReadFile(mdPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Event Sourcing")

	// Verify index.toml updated
	indexData, err := os.ReadFile(filepath.Join(stateDir, "patterns", "index.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(indexData), "event-sourcing")

	// "crud-api" already exists in the fixture
	_, err = handleTeamPatternsPropose(context.Background(), json.RawMessage(`{
		"name": "crud-api",
		"tags": ["api"],
		"complexity": "M",
		"project": "myproject",
		"content": "# Duplicate"
	}`))
	require.Error(t, err, "a pattern name is proposed once")
	assert.Contains(t, err.Error(), "already exists")
}
