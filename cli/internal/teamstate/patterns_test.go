package teamstate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateAndListPatterns(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	p := Pattern{
		Name:       "crud-api",
		Tags:       []string{"backend", "api", "crud"},
		Complexity: "medium",
		Source:     "manual",
		Project:    "T-SRU",
		Validated:  true,
	}
	content := "# CRUD API Pattern\n\nDecomposition type..."

	if err := repo.CreatePattern(ctx, p, content); err != nil {
		t.Fatalf("CreatePattern failed: %v", err)
	}

	// List all
	patterns, err := repo.ListPatterns(nil, 0)
	if err != nil {
		t.Fatalf("ListPatterns failed: %v", err)
	}
	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(patterns))
	}
	if patterns[0].Name != "crud-api" {
		t.Errorf("expected name crud-api, got %s", patterns[0].Name)
	}
	if patterns[0].CreatedAt == "" {
		t.Error("expected created_at to be set")
	}

	// Check file exists
	mdPath := filepath.Join(repo.path, "patterns", "crud-api.md")
	if _, err := os.Stat(mdPath); os.IsNotExist(err) {
		t.Error("pattern .md file not created")
	}
}

func TestListPatterns_FilterByTags(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	patterns := []struct {
		p       Pattern
		content string
	}{
		{Pattern{Name: "crud-api", Tags: []string{"backend", "api", "crud"}, Source: "manual", Validated: true}, "# CRUD"},
		{Pattern{Name: "migration-db", Tags: []string{"backend", "database", "migration"}, Source: "manual", Validated: true}, "# Migration"},
		{Pattern{Name: "ui-component", Tags: []string{"frontend", "ui", "component"}, Source: "manual", Validated: true}, "# UI"},
	}

	for _, pp := range patterns {
		if err := repo.CreatePattern(ctx, pp.p, pp.content); err != nil {
			t.Fatalf("CreatePattern %s failed: %v", pp.p.Name, err)
		}
	}

	// Filter: at least 2 tags matching ["backend", "api"]
	filtered, err := repo.ListPatterns([]string{"backend", "api"}, 2)
	if err != nil {
		t.Fatalf("ListPatterns with filter failed: %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("expected 1 match, got %d", len(filtered))
	}
	if filtered[0].Name != "crud-api" {
		t.Errorf("expected crud-api, got %s", filtered[0].Name)
	}

	// Filter: at least 1 tag matching ["backend"]
	filtered, err = repo.ListPatterns([]string{"backend"}, 1)
	if err != nil {
		t.Fatalf("ListPatterns with backend filter failed: %v", err)
	}
	if len(filtered) != 2 {
		t.Fatalf("expected 2 matches (crud-api, migration-db), got %d", len(filtered))
	}
}

func TestReadPattern(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	p := Pattern{Name: "test-pattern", Tags: []string{"test"}, Source: "manual", Validated: true}
	expectedContent := "# Test Pattern\n\nThis is the content."
	if err := repo.CreatePattern(ctx, p, expectedContent); err != nil {
		t.Fatal(err)
	}

	content, err := repo.ReadPattern("test-pattern")
	if err != nil {
		t.Fatalf("ReadPattern failed: %v", err)
	}
	if content != expectedContent {
		t.Errorf("expected %q, got %q", expectedContent, content)
	}
}

func TestReadPattern_NotFound(t *testing.T) {
	repo := setupTestRepo(t)

	_, err := repo.ReadPattern("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent pattern")
	}
}

func TestValidatePattern(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	p := Pattern{Name: "proposed", Tags: []string{"test"}, Source: "planner", Validated: false}
	if err := repo.CreatePattern(ctx, p, "# Proposed"); err != nil {
		t.Fatal(err)
	}

	// Verify not validated
	patterns, _ := repo.ListPatterns(nil, 0)
	if patterns[0].Validated {
		t.Error("should not be validated initially")
	}

	// Validate
	if err := repo.ValidatePattern(ctx, "proposed"); err != nil {
		t.Fatalf("ValidatePattern failed: %v", err)
	}

	// Verify validated
	patterns, _ = repo.ListPatterns(nil, 0)
	if !patterns[0].Validated {
		t.Error("should be validated after ValidatePattern")
	}
}

func TestRemovePattern(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	p := Pattern{Name: "to-remove", Tags: []string{"test"}, Source: "manual", Validated: true}
	if err := repo.CreatePattern(ctx, p, "# Remove me"); err != nil {
		t.Fatal(err)
	}

	if err := repo.RemovePattern(ctx, "to-remove"); err != nil {
		t.Fatalf("RemovePattern failed: %v", err)
	}

	patterns, _ := repo.ListPatterns(nil, 0)
	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns after remove, got %d", len(patterns))
	}

	// File should be removed
	mdPath := filepath.Join(repo.path, "patterns", "to-remove.md")
	if _, err := os.Stat(mdPath); !os.IsNotExist(err) {
		t.Error("pattern .md file should have been removed")
	}
}

func TestCreatePattern_Duplicate(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	p := Pattern{Name: "dup", Tags: []string{"test"}, Source: "manual", Validated: true}
	if err := repo.CreatePattern(ctx, p, "# First"); err != nil {
		t.Fatal(err)
	}

	err := repo.CreatePattern(ctx, p, "# Second")
	if err == nil {
		t.Error("expected error for duplicate pattern")
	}
}
