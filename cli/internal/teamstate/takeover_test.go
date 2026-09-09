package teamstate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
)

func TestSaveAndReadBrief(t *testing.T) {
	dir := t.TempDir()
	repo := &Repo{path: dir}

	brief := &TakeoverBrief{
		Meta: TakeoverMeta{
			TicketID:        "bd-42",
			Project:         "T-SRU",
			TransferredFrom: "benjamin",
			TransferredTo:   "alice",
			TransferDate:    time.Date(2026, 7, 13, 14, 30, 0, 0, time.UTC),
			Reason:          "transfer",
		},
		Activity: TakeoverActivity{
			SessionsCount:        3,
			FirstSession:         time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC),
			LastSession:          time.Date(2026, 7, 12, 16, 45, 0, 0, time.UTC),
			TotalDurationMinutes: 185,
		},
		Git: TakeoverGit{
			Branch:            "feat/bd-42-auth",
			CommitsCount:      7,
			LastCommitMessage: "fix: handle token expiry",
			LastCommitDate:    time.Date(2026, 7, 12, 16, 30, 0, 0, time.UTC),
			FilesModified: []TakeoverFileChange{
				{Path: "src/auth/service.ts", Additions: 142, Deletions: 23},
			},
			FilesCreated: []TakeoverFileChange{
				{Path: "src/auth/token-rotation.ts", Additions: 89},
			},
		},
		Events: []TakeoverEvent{
			{
				Timestamp: time.Date(2026, 7, 12, 16, 45, 0, 0, time.UTC),
				Type:      EventSessionComplete,
				Summary:   "Fixed edge case",
			},
			{
				Timestamp: time.Date(2026, 7, 10, 9, 15, 0, 0, time.UTC),
				Type:      EventSessionComplete,
				Summary:   "Initial implementation",
			},
		},
	}

	// Create a minimal git repo to avoid CommitAndPush errors
	// For this test, we just verify file creation (skip git operations)
	projectDir := filepath.Join(dir, "projects", "T-SRU", "takeover-briefs")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Test RenderTemplateBrief
	md := RenderTemplateBrief(brief)
	if md == "" {
		t.Fatal("RenderTemplateBrief returned empty string")
	}
	if !contains(md, "bd-42") {
		t.Error("markdown should contain ticket ID")
	}
	if !contains(md, "benjamin") {
		t.Error("markdown should contain transferrer name")
	}
	if !contains(md, "alice") {
		t.Error("markdown should contain transferee name")
	}
	if !contains(md, "src/auth/service.ts") {
		t.Error("markdown should contain modified file")
	}
	if !contains(md, "fix: handle token expiry") {
		t.Error("markdown should contain last commit message")
	}
	if !contains(md, "3h05m") {
		t.Error("markdown should contain duration (185min = 3h05m)")
	}

	// Write brief files directly (skip git)
	baseName := "bd-42_2026-07-13"
	tomlPath := filepath.Join(projectDir, baseName+".toml")
	mdPath := filepath.Join(projectDir, baseName+".md")

	// Simulate SaveBrief without git
	data, _ := tomlMarshal(brief)
	os.WriteFile(tomlPath, data, 0o644)
	os.WriteFile(mdPath, []byte(md), 0o644)

	// Test ReadBrief
	content, err := repo.ReadBrief("T-SRU", "bd-42")
	if err != nil {
		t.Fatalf("ReadBrief failed: %v", err)
	}
	if !contains(content, "bd-42") {
		t.Error("ReadBrief should return content with ticket ID")
	}

	// Test BriefExists
	if !repo.BriefExists("T-SRU", "bd-42") {
		t.Error("BriefExists should return true")
	}
	if repo.BriefExists("T-SRU", "bd-99") {
		t.Error("BriefExists should return false for non-existent")
	}
}

func TestReadBrief_NotFound(t *testing.T) {
	dir := t.TempDir()
	repo := &Repo{path: dir}

	_, err := repo.ReadBrief("T-SRU", "bd-99")
	if err != ErrBriefNotFound {
		t.Errorf("expected ErrBriefNotFound, got: %v", err)
	}
}

func TestReadBrief_PrefersEnriched(t *testing.T) {
	dir := t.TempDir()
	repo := &Repo{path: dir}

	projectDir := filepath.Join(dir, "projects", "T-SRU", "takeover-briefs")
	os.MkdirAll(projectDir, 0o755)

	baseName := "bd-42_2026-07-13"
	os.WriteFile(filepath.Join(projectDir, baseName+".toml"), []byte("raw"), 0o644)
	os.WriteFile(filepath.Join(projectDir, baseName+".md"), []byte("template brief"), 0o644)
	os.WriteFile(filepath.Join(projectDir, baseName+".enriched.md"), []byte("enriched brief"), 0o644)

	content, err := repo.ReadBrief("T-SRU", "bd-42")
	if err != nil {
		t.Fatalf("ReadBrief failed: %v", err)
	}
	if content != "enriched brief" {
		t.Errorf("expected enriched brief, got: %q", content)
	}
}

func TestIsStale(t *testing.T) {
	dir := t.TempDir()
	repo := &Repo{path: dir}

	now := time.Now()
	tests := []struct {
		lastActivity time.Time
		staleDays    int
		expected     bool
	}{
		{now.Add(-1 * time.Hour), 3, false},                     // 1h ago, not stale
		{now.Add(-4 * 24 * time.Hour), 3, true},                 // 4 days ago, stale
		{now.Add(-2 * 24 * time.Hour), 3, false},                // 2 days ago, not stale
		{now.Add(-3*24*time.Hour - time.Hour), 3, true},         // 3d1h ago, stale
		{time.Time{}, 3, false},                                  // zero time uses ClaimedAt
	}

	for i, tc := range tests {
		claim := &Claim{
			ClaimedAt:    now, // if LastActivity is zero, use ClaimedAt
			LastActivity: tc.lastActivity,
		}
		result := repo.IsStale(claim, tc.staleDays)
		if result != tc.expected {
			t.Errorf("test %d: expected stale=%v, got %v (lastActivity=%v, staleDays=%d)",
				i, tc.expected, result, tc.lastActivity, tc.staleDays)
		}
	}
}

func TestListBriefs(t *testing.T) {
	dir := t.TempDir()
	repo := &Repo{path: dir}

	projectDir := filepath.Join(dir, "projects", "T-SRU", "takeover-briefs")
	os.MkdirAll(projectDir, 0o755)

	brief1 := &TakeoverBrief{
		Meta: TakeoverMeta{
			TicketID:        "bd-42",
			Project:         "T-SRU",
			TransferredFrom: "benjamin",
			TransferredTo:   "alice",
			TransferDate:    time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC),
			Reason:          "transfer",
		},
	}
	brief2 := &TakeoverBrief{
		Meta: TakeoverMeta{
			TicketID:        "bd-43",
			Project:         "T-SRU",
			TransferredFrom: "alice",
			TransferredTo:   "bob",
			TransferDate:    time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC),
			Reason:          "stale",
		},
	}

	// Write both
	data1, _ := tomlMarshal(brief1)
	data2, _ := tomlMarshal(brief2)
	os.WriteFile(filepath.Join(projectDir, "bd-42_2026-07-13.toml"), data1, 0o644)
	os.WriteFile(filepath.Join(projectDir, "bd-43_2026-07-14.toml"), data2, 0o644)

	metas, err := repo.ListBriefs("T-SRU")
	if err != nil {
		t.Fatalf("ListBriefs failed: %v", err)
	}
	if len(metas) != 2 {
		t.Fatalf("expected 2 briefs, got %d", len(metas))
	}
}

func TestListBriefs_AllProjects(t *testing.T) {
	dir := t.TempDir()
	repo := &Repo{path: dir}

	// Create briefs in two different projects
	for _, project := range []string{"project-alpha", "project-beta"} {
		projectDir := filepath.Join(dir, "projects", project, "takeover-briefs")
		os.MkdirAll(projectDir, 0o755)

		brief := &TakeoverBrief{
			Meta: TakeoverMeta{
				TicketID:        "tk-" + project,
				Project:         project,
				TransferredFrom: "alice",
				TransferredTo:   "bob",
				TransferDate:    time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
				Reason:          "transfer",
			},
		}
		data, _ := tomlMarshal(brief)
		os.WriteFile(filepath.Join(projectDir, "tk-"+project+"_2026-08-01.toml"), data, 0o644)
	}

	// ListBriefs("") should return briefs from all projects
	metas, err := repo.ListBriefs("")
	assert.NoError(t, err)
	assert.Len(t, metas, 2, "expected briefs from both projects")

	// Verify both projects are represented
	projects := map[string]bool{}
	for _, m := range metas {
		projects[m.Project] = true
	}
	assert.True(t, projects["project-alpha"], "should include project-alpha")
	assert.True(t, projects["project-beta"], "should include project-beta")

	// ListBriefs("project-alpha") should return only that project's briefs
	metas, err = repo.ListBriefs("project-alpha")
	assert.NoError(t, err)
	assert.Len(t, metas, 1)
	assert.Equal(t, "project-alpha", metas[0].Project)
}

func TestListBriefs_EmptyRepo(t *testing.T) {
	dir := t.TempDir()
	repo := &Repo{path: dir}

	// No projects dir at all — should return nil, no error
	metas, err := repo.ListBriefs("")
	assert.NoError(t, err)
	assert.Nil(t, metas)
}

func TestGenerateRawBrief(t *testing.T) {
	dir := t.TempDir()
	repo := &Repo{path: dir}

	// Create minimal events
	eventsDir := filepath.Join(dir, "projects", "T-SRU", "events")
	os.MkdirAll(eventsDir, 0o755)

	ctx := context.Background()
	brief, err := repo.GenerateRawBrief(ctx, "T-SRU", "bd-42", "benjamin", "alice", "transfer")
	if err != nil {
		t.Fatalf("GenerateRawBrief failed: %v", err)
	}

	if brief.Meta.TicketID != "bd-42" {
		t.Errorf("expected ticket bd-42, got %s", brief.Meta.TicketID)
	}
	if brief.Meta.TransferredFrom != "benjamin" {
		t.Errorf("expected from benjamin, got %s", brief.Meta.TransferredFrom)
	}
	if brief.Meta.Reason != "transfer" {
		t.Errorf("expected reason transfer, got %s", brief.Meta.Reason)
	}
}

// helpers

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || (len(s) >= len(substr) && containsSubstr(s, substr)))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func tomlMarshal(v interface{}) ([]byte, error) {
	return toml.Marshal(v)
}

// --- Path traversal security tests ---

func TestListBriefs_PathTraversal(t *testing.T) {
	repo := setupTestRepo(t)

	tests := []struct {
		name    string
		project string
	}{
		{"dot-dot traversal", "../../../etc"},
		{"slash", "foo/bar"},
		{"backslash", "foo\\bar"},
		{"null byte", "foo\x00bar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := repo.ListBriefs(tt.project)
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrUnsafeName)
		})
	}
}

func TestBriefExists_PathTraversal(t *testing.T) {
	repo := setupTestRepo(t)

	// Path traversal should be blocked — returns false, not panic
	assert.False(t, repo.BriefExists("../../../etc", "passwd"))
	assert.False(t, repo.BriefExists("T-SRU", "../../../etc/passwd"))
	assert.False(t, repo.BriefExists("foo/bar", "ticket"))
}
