package deploy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeepMerge_ScalarOverride(t *testing.T) {
	dst := map[string]interface{}{
		"read":  "allow",
		"write": "allow",
		"edit":  "allow",
	}
	src := map[string]interface{}{
		"write": "deny",
	}
	result := deepMerge(dst, src)
	assert.Equal(t, "allow", result["read"])
	assert.Equal(t, "deny", result["write"])
	assert.Equal(t, "allow", result["edit"])
}

func TestDeepMerge_NestedMapAddition(t *testing.T) {
	dst := map[string]interface{}{
		"bash": map[string]interface{}{
			"*":            "deny",
			"git status*":  "allow",
			"git diff*":    "allow",
		},
	}
	src := map[string]interface{}{
		"bash": map[string]interface{}{
			"alembic*": "allow",
		},
	}
	result := deepMerge(dst, src)
	bash := result["bash"].(map[string]interface{})
	assert.Equal(t, "deny", bash["*"])
	assert.Equal(t, "allow", bash["git status*"])
	assert.Equal(t, "allow", bash["git diff*"])
	assert.Equal(t, "allow", bash["alembic*"])
}

func TestDeepMerge_NewTopLevelKey(t *testing.T) {
	dst := map[string]interface{}{
		"read": "allow",
	}
	src := map[string]interface{}{
		"websearch": "allow",
	}
	result := deepMerge(dst, src)
	assert.Equal(t, "allow", result["read"])
	assert.Equal(t, "allow", result["websearch"])
}

func TestDeepMerge_DoesNotMutateSrc(t *testing.T) {
	dst := map[string]interface{}{"a": "1"}
	src := map[string]interface{}{"b": "2"}
	_ = deepMerge(dst, src)
	_, ok := dst["b"]
	assert.False(t, ok, "dst should not be mutated")
}

func TestResolvePermissions_NoBase(t *testing.T) {
	fm := &AgentFrontmatter{
		Permission: map[string]interface{}{
			"read": "allow",
		},
	}
	result, err := ResolvePermissions("/nonexistent", fm)
	require.NoError(t, err)
	assert.Equal(t, "allow", result["read"])
}

func TestResolvePermissions_WithBase(t *testing.T) {
	// Create a temp dir with a permissions file
	tmpDir := t.TempDir()
	permDir := filepath.Join(tmpDir, "permissions")
	require.NoError(t, os.MkdirAll(permDir, 0o755))

	baseContent := `read: allow
write: allow
bash:
  "*": deny
  "git status*": allow
`
	require.NoError(t, os.WriteFile(filepath.Join(permDir, "test-base.yaml"), []byte(baseContent), 0o644))

	fm := &AgentFrontmatter{
		PermissionBase: "test-base",
		Permission:     nil, // no overrides
	}
	result, err := ResolvePermissions(tmpDir, fm)
	require.NoError(t, err)
	assert.Equal(t, "allow", result["read"])
	assert.Equal(t, "allow", result["write"])
	bash := result["bash"].(map[string]interface{})
	assert.Equal(t, "deny", bash["*"])
	assert.Equal(t, "allow", bash["git status*"])
}

func TestResolvePermissions_WithBaseAndOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	permDir := filepath.Join(tmpDir, "permissions")
	require.NoError(t, os.MkdirAll(permDir, 0o755))

	baseContent := `read: allow
write: allow
edit: allow
bash:
  "*": deny
  "git status*": allow
task:
  "*": deny
  "documentarian": allow
`
	require.NoError(t, os.WriteFile(filepath.Join(permDir, "dev-base.yaml"), []byte(baseContent), 0o644))

	fm := &AgentFrontmatter{
		PermissionBase: "dev-base",
		Permission: map[string]interface{}{
			"bash": map[string]interface{}{
				"alembic*": "allow",
			},
			"task": map[string]interface{}{
				"reviewer": "allow",
			},
		},
	}
	result, err := ResolvePermissions(tmpDir, fm)
	require.NoError(t, err)

	// Base values preserved
	assert.Equal(t, "allow", result["read"])
	assert.Equal(t, "allow", result["write"])
	assert.Equal(t, "allow", result["edit"])

	// Bash: base preserved + override added
	bash := result["bash"].(map[string]interface{})
	assert.Equal(t, "deny", bash["*"])
	assert.Equal(t, "allow", bash["git status*"])
	assert.Equal(t, "allow", bash["alembic*"])

	// Task: base preserved + override added
	task := result["task"].(map[string]interface{})
	assert.Equal(t, "deny", task["*"])
	assert.Equal(t, "allow", task["documentarian"])
	assert.Equal(t, "allow", task["reviewer"])
}

func TestResolvePermissions_BaseNotFound(t *testing.T) {
	fm := &AgentFrontmatter{
		PermissionBase: "nonexistent-base",
	}
	_, err := ResolvePermissions("/tmp/empty", fm)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent-base")
}
