package ciconfig

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

var update = flag.Bool("update", false, "update golden files")

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(path, got, 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "missing golden file (go test ./internal/remote/ciconfig -update)")
	assert.Equal(t, string(want), string(got))
}

func TestGenerateGolden(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts Options
	}{
		{"kaniko.gitlab-ci.yml", Options{}},
		{"dind-arm64.gitlab-ci.yml", Options{Builder: BuilderDinD, Arch: "arm64", Tag: "oh-arm", Timeout: "90m"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := Generate(tc.opts)
			require.NoError(t, err)
			golden(t, tc.name, out)

			var doc map[string]any
			require.NoError(t, yaml.Unmarshal(out, &doc), "generated file is valid YAML")
			for _, job := range []string{"oh-cli", "oh-image", "oh-run"} {
				assert.Contains(t, doc, job)
			}
			assert.True(t, strings.HasPrefix(string(out), Marker))
			run := doc["oh-run"].(map[string]any)
			assert.Equal(t, "$OH_IMAGE", run["image"].(map[string]any)["name"])
		})
	}
}

// The job scripts must be valid POSIX shell.
func TestGenerateScriptsParse(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	for _, opts := range []Options{{}, {Builder: BuilderDinD}} {
		out, err := Generate(opts)
		require.NoError(t, err)
		var doc map[string]map[string]any
		var raw map[string]any
		require.NoError(t, yaml.Unmarshal(out, &raw))
		doc = map[string]map[string]any{}
		for k, v := range raw {
			if m, ok := v.(map[string]any); ok {
				doc[k] = m
			}
		}
		for _, job := range []string{"oh-cli", "oh-image", "oh-run"} {
			lines := doc[job]["script"].([]any)
			for _, l := range lines {
				cmd := exec.Command(sh, "-n", "-c", l.(string))
				outp, err := cmd.CombinedOutput()
				assert.NoError(t, err, "%s: %s", job, outp)
			}
		}
	}
}

func TestValidate(t *testing.T) {
	assert.Error(t, Options{Builder: "buildah"}.Validate())
	assert.Error(t, Options{Arch: "386"}.Validate())
	assert.Error(t, Options{Tag: "oh: x"}.Validate())
	assert.Error(t, Options{KanikoImage: "img\"x"}.Validate())
	assert.NoError(t, Options{Tag: "oh-runner", KanikoImage: "registry:5000/kaniko:1"}.Validate())
}
