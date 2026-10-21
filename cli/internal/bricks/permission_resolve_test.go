package bricks

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeepMerge(t *testing.T) {
	t.Parallel()
	type perms = map[string]interface{}
	tests := []struct {
		name string
		dst  perms
		src  perms
		want perms
	}{
		{
			name: "a scalar of src overrides the one of dst",
			dst:  perms{"read": "allow", "write": "allow", "edit": "allow"},
			src:  perms{"write": "deny"},
			want: perms{"read": "allow", "write": "deny", "edit": "allow"},
		},
		{
			name: "nested maps are merged key by key",
			dst:  perms{"bash": perms{"*": "deny", "git status*": "allow", "git diff*": "allow"}},
			src:  perms{"bash": perms{"alembic*": "allow"}},
			want: perms{"bash": perms{"*": "deny", "git status*": "allow", "git diff*": "allow", "alembic*": "allow"}},
		},
		{
			name: "a new top-level key is added",
			dst:  perms{"read": "allow"},
			src:  perms{"websearch": "allow"},
			want: perms{"read": "allow", "websearch": "allow"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dstBefore, srcBefore := fmt.Sprint(tt.dst), fmt.Sprint(tt.src)

			result := deepMerge(tt.dst, tt.src)

			assert.Equal(t, tt.want, result)
			assert.Equal(t, dstBefore, fmt.Sprint(tt.dst), "dst should not be mutated")
			assert.Equal(t, srcBefore, fmt.Sprint(tt.src), "src should not be mutated")
		})
	}
}

func TestResolvePermissions_NoBase(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	fm := &AgentFrontmatter{
		PermissionBase: "nonexistent-base",
	}
	_, err := ResolvePermissions("/tmp/empty", fm)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent-base")
}
