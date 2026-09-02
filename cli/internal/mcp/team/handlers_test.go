package team

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/deploy"
)

// setupHandlerTest creates a fully functional team-state repo and project dir
// for integration-testing the MCP handlers. Returns a cleanup function.
func setupHandlerTest(t *testing.T) string {
	t.Helper()
	resetRepoCache()

	// Create team-state directory structure
	teamState := t.TempDir()
	initGitRepo(t, teamState)

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
	gitCmd(t, teamState, "add", ".")
	gitCmd(t, teamState, "commit", "-m", "initial state")

	// Create project directory with .opencode/team.json
	projectDir := t.TempDir()
	writeTeamJSON(t, projectDir, deploy.DeployedTeamConfig{
		Enabled:   true,
		StateRepo: teamState, // point to the local dir (acting as both remote and local)
		StatePath: teamState,
		MemberID:  "benjamin",
	})

	chdir(t, projectDir)
	return teamState
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	gitCmd(t, dir, "init")
	gitCmd(t, dir, "config", "user.email", "test@test.com")
	gitCmd(t, dir, "config", "user.name", "Test")
	// Create initial commit so repo is not empty
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitkeep"), []byte(""), 0o644))
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "init")
}

func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v in %s failed: %s", args, dir, string(out))
}

// --- Handler integration tests ---

func TestHandleTeamMembers(t *testing.T) {
	setupHandlerTest(t)

	result, err := handleTeamMembers(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err)
	require.Len(t, result.Content, 1)
	assert.Contains(t, result.Content[0].Text, "benjamin")
	assert.Contains(t, result.Content[0].Text, "alice")
}

func TestHandleTeamClaims(t *testing.T) {
	setupHandlerTest(t)

	result, err := handleTeamClaims(context.Background(), json.RawMessage(`{"project":"myproject"}`))
	require.NoError(t, err)
	require.Len(t, result.Content, 1)
	assert.Contains(t, result.Content[0].Text, "SRU-142")
	assert.Contains(t, result.Content[0].Text, "BD-99")
}

func TestHandleTeamClaims_UnknownProject(t *testing.T) {
	setupHandlerTest(t)

	result, err := handleTeamClaims(context.Background(), json.RawMessage(`{"project":"nonexistent"}`))
	require.NoError(t, err)
	// Should return empty or no claims
	require.Len(t, result.Content, 1)
}

func TestHandleTeamWikiList(t *testing.T) {
	setupHandlerTest(t)

	result, err := handleTeamWikiList(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err)
	require.Len(t, result.Content, 1)
	assert.Contains(t, result.Content[0].Text, "conventions")
	assert.Contains(t, result.Content[0].Text, "decisions")
}

func TestHandleTeamWikiRead(t *testing.T) {
	setupHandlerTest(t)

	result, err := handleTeamWikiRead(context.Background(), json.RawMessage(`{"page":"conventions"}`))
	require.NoError(t, err)
	require.Len(t, result.Content, 1)
	assert.Contains(t, result.Content[0].Text, "Conventional Commits")
}

func TestHandleTeamWikiRead_NotFound(t *testing.T) {
	setupHandlerTest(t)

	_, err := handleTeamWikiRead(context.Background(), json.RawMessage(`{"page":"nonexistent"}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestHandleTeamWikiRead_PathTraversal(t *testing.T) {
	setupHandlerTest(t)

	_, err := handleTeamWikiRead(context.Background(), json.RawMessage(`{"page":"../etc/passwd"}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsafe")
}

func TestHandleTeamEvents(t *testing.T) {
	setupHandlerTest(t)

	result, err := handleTeamEvents(context.Background(), json.RawMessage(`{"project":"myproject","limit":10}`))
	require.NoError(t, err)
	require.Len(t, result.Content, 1)
	assert.Contains(t, result.Content[0].Text, "benjamin")
	assert.Contains(t, result.Content[0].Text, "alice")
}

func TestHandleTeamPolicies(t *testing.T) {
	setupHandlerTest(t)

	result, err := handleTeamPolicies(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err)
	require.Len(t, result.Content, 1)
	assert.Contains(t, result.Content[0].Text, "branch_naming")
}

func TestHandleTeamTakeoverBrief(t *testing.T) {
	setupHandlerTest(t)

	result, err := handleTeamTakeoverBrief(context.Background(), json.RawMessage(`{"project":"myproject","ticket_id":"SRU-142"}`))
	require.NoError(t, err)
	require.Len(t, result.Content, 1)
	assert.Contains(t, result.Content[0].Text, "user auth")
}

func TestHandleTeamTakeoverBrief_PathTraversal(t *testing.T) {
	setupHandlerTest(t)

	_, err := handleTeamTakeoverBrief(context.Background(), json.RawMessage(`{"project":"../etc","ticket_id":"passwd"}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsafe")
}

func TestHandleTeamPatternsList(t *testing.T) {
	setupHandlerTest(t)

	result, err := handleTeamPatternsList(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err)
	require.Len(t, result.Content, 1)
	assert.Contains(t, result.Content[0].Text, "crud-api")
	assert.Contains(t, result.Content[0].Text, "db-migration")
}

func TestHandleTeamPatternsRead(t *testing.T) {
	setupHandlerTest(t)

	result, err := handleTeamPatternsRead(context.Background(), json.RawMessage(`{"name":"crud-api"}`))
	require.NoError(t, err)
	require.Len(t, result.Content, 1)
	assert.Contains(t, result.Content[0].Text, "CRUD API Pattern")
}

func TestHandleTeamPatternsRead_PathTraversal(t *testing.T) {
	setupHandlerTest(t)

	_, err := handleTeamPatternsRead(context.Background(), json.RawMessage(`{"name":"../../etc/passwd"}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsafe")
}

func TestHandleTeamEvents_LimitCapped(t *testing.T) {
	setupHandlerTest(t)

	// A very large limit should be capped to 200 (not cause OOM)
	result, err := handleTeamEvents(context.Background(), json.RawMessage(`{"project":"myproject","limit":999999}`))
	require.NoError(t, err)
	require.Len(t, result.Content, 1)
	// Should still return results without error
}

func TestHandleTeamWikiWrite_PathTraversal(t *testing.T) {
	setupHandlerTest(t)

	payload := `{"page":"../../../etc/passwd","content":"evil","confidence":"CONFIRMED"}`
	_, err := handleTeamWikiWrite(context.Background(), json.RawMessage(payload))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

// setupHandlerWriteTest creates a team-state repo backed by a bare remote, enabling
// commitAndPush to succeed. Returns the clone (state) path for file assertions.
func setupHandlerWriteTest(t *testing.T) string {
	t.Helper()
	resetRepoCache()

	// Create bare remote
	bare := t.TempDir()
	gitCmd(t, bare, "init", "--bare")

	// Clone from bare
	teamState := filepath.Join(t.TempDir(), "clone")
	cmd := exec.Command("git", "clone", bare, teamState)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "clone failed: %s", string(out))

	// Configure user in clone
	gitCmd(t, teamState, "config", "user.email", "test@test.com")
	gitCmd(t, teamState, "config", "user.name", "Test")

	// Initial commit (required before push)
	require.NoError(t, os.WriteFile(filepath.Join(teamState, ".gitkeep"), []byte(""), 0o644))
	gitCmd(t, teamState, "add", ".")
	gitCmd(t, teamState, "commit", "-m", "init")
	// Detect branch name and push
	branchOut, _ := exec.Command("git", "-C", teamState, "branch", "--show-current").Output()
	branch := "main"
	if b := string(branchOut); b != "" {
		branch = b[:len(b)-1] // trim newline
	}
	gitCmd(t, teamState, "push", "-u", "origin", branch)

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
	gitCmd(t, teamState, "add", ".")
	gitCmd(t, teamState, "commit", "-m", "initial state")
	gitCmd(t, teamState, "push")

	// Create project directory with .opencode/team.json
	projectDir := t.TempDir()
	writeTeamJSON(t, projectDir, deploy.DeployedTeamConfig{
		Enabled:   true,
		StateRepo: bare,
		StatePath: teamState,
		MemberID:  "benjamin",
	})

	chdir(t, projectDir)
	return teamState
}

// ─── PR7: Write handler tests ─────────────────────────────────────────────────

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

func TestHandleTeamWikiWrite_MissingPage(t *testing.T) {
	setupHandlerTest(t)

	payload := `{"page":"","content":"stuff","confidence":"CONFIRMED","project":"x"}`
	_, err := handleTeamWikiWrite(context.Background(), json.RawMessage(payload))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "page is required")
}

func TestHandleTeamWikiWrite_InvalidConfidence(t *testing.T) {
	setupHandlerTest(t)

	payload := `{"page":"test","content":"stuff","confidence":"MAYBE","project":"x"}`
	_, err := handleTeamWikiWrite(context.Background(), json.RawMessage(payload))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CONFIRMED, INFERRED, or UNCERTAIN")
}

func TestHandleTeamNotify_Disabled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	setupHandlerWriteTest(t)

	// Config has notifications disabled — Dispatch returns nil immediately
	result, err := handleTeamNotify(context.Background(), json.RawMessage(`{"message":"hello team"}`))
	require.NoError(t, err)
	require.Len(t, result.Content, 1)
	assert.Contains(t, result.Content[0].Text, "Notification sent")
}

func TestHandleTeamNotify_Enabled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	stateDir := setupHandlerWriteTest(t)

	// Start a mock webhook server
	received := make(chan string, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Update config.toml to enable notifications with the test server
	configContent := fmt.Sprintf(`
[notification]
enabled = true
type = "mattermost"
webhook_url = "%s/hooks/test"
channel = "test-channel"
bot_name = "TestBot"
`, ts.URL)
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "config.toml"), []byte(configContent), 0o644))

	result, err := handleTeamNotify(context.Background(), json.RawMessage(`{"message":"deploy complete"}`))
	require.NoError(t, err)
	require.Len(t, result.Content, 1)
	assert.Contains(t, result.Content[0].Text, "Notification sent")

	// Verify webhook was called
	select {
	case path := <-received:
		assert.Equal(t, "/hooks/test", path)
	default:
		t.Error("webhook was not called")
	}
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
}

func TestHandleTeamPatternsPropose_Duplicate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	setupHandlerWriteTest(t)

	// "crud-api" already exists in the fixture
	payload := `{
		"name": "crud-api",
		"tags": ["api"],
		"complexity": "M",
		"project": "myproject",
		"content": "# Duplicate"
	}`
	_, err := handleTeamPatternsPropose(context.Background(), json.RawMessage(payload))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}
