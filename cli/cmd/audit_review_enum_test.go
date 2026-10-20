package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
)

// aliasEnv installs a hub with the repository content; the workflows are a
// copy that tests may edit.
func aliasEnv(t *testing.T) (workflows string) {
	t.Helper()
	root := repoRoot(t)
	home := t.TempDir()
	t.Setenv("OH_HOME", home)
	config.Reset()
	t.Cleanup(config.Reset)
	withoutV2(t, nil)
	require.NoError(t, os.Symlink(root, filepath.Join(home, "hub")))
	workflows = filepath.Join(t.TempDir(), "workflows")
	out, err := exec.Command("cp", "-R", filepath.Join(root, "workflows"), workflows).CombinedOutput()
	require.NoError(t, err, string(out))
	t.Setenv(workflowsDirEnv, workflows)
	io, _, _ := testIO()
	application = &app.App{Config: &config.Config{}, Projects: &mockProjectStore{}, IO: io}
	t.Cleanup(func() { application = nil })
	return workflows
}

func runEnumAlias(t *testing.T, c *cobra.Command, args ...string) error {
	t.Helper()
	require.NoError(t, c.Flags().Parse(args))
	t.Cleanup(func() {
		c.Flags().VisitAll(func(f *pflag.Flag) { _ = f.Value.Set(f.DefValue); f.Changed = false })
	})
	c.SetContext(t.Context())
	return c.RunE(c, nil)
}

func TestAuditAndReviewReadTheirChoicesFromTheWorkflow(t *testing.T) {
	aliasEnv(t)

	assert.Equal(t, []string{"security", "performance", "architecture", "accessibility", "ecodesign", "observability", "privacy", "infra", "database"},
		workflowInputValues(auditCmd, "audit", "type"), "the shipped audit workflow keeps the former types")
	assert.Equal(t, []string{"standard", "adversarial", "edge-case", "standard+adversarial", "all"},
		workflowInputValues(reviewCmd, "review", "review_mode"))

	err := runEnumAlias(t, auditCmd, "--type", "bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bogus")
	assert.Contains(t, err.Error(), "observability")
	err = runEnumAlias(t, reviewCmd, "--mode", "bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "standard+adversarial")
}

func TestAuditTypesFollowTheWorkflowYAML(t *testing.T) {
	workflows := aliasEnv(t)
	file := filepath.Join(workflows, "audit.yaml")
	data, err := os.ReadFile(file)
	require.NoError(t, err)
	edited := strings.Replace(string(data), "values: [security, performance,", "values: [security, supply-chain, performance,", 1)
	require.NotEqual(t, string(data), edited)
	require.NoError(t, os.WriteFile(file, []byte(edited), 0o644))

	assert.Contains(t, workflowInputValues(auditCmd, "audit", "type"), "supply-chain")

	// A type added to the YAML is accepted: the error is no longer about the type.
	err = runEnumAlias(t, auditCmd, "--type", "supply-chain")
	if err != nil {
		assert.NotContains(t, err.Error(), "supply-chain")
	}
	err = runEnumAlias(t, auditCmd, "--type", "bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "supply-chain", "the error lists the choices of the workflow")

	// Completion and help follow too.
	got, _ := completeWorkflowInput("audit", "type")(auditCmd, nil, "")
	assert.Contains(t, got, "supply-chain")
	flag := auditCmd.Flags().Lookup("type")
	usage := flag.Usage
	t.Cleanup(func() { flag.Usage = usage })
	listWorkflowEnumChoices(auditCmd)
	assert.Contains(t, flag.Usage, "supply-chain")
}
