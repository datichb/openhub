package ciconfig

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/datichb/openhub/cli/internal/testutil/golden"
)

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
			golden.Assert(t, tc.name, out)

			var doc map[string]any
			require.NoError(t, yaml.Unmarshal(out, &doc), "generated file is valid YAML")
			for _, job := range []string{"oh-cli", "oh-image", "oh-run"} {
				assert.Contains(t, doc, job)
			}
			assert.True(t, strings.HasPrefix(string(out), Marker))
			run := doc["oh-run"].(map[string]any)
			assert.Equal(t, "$OH_RUN_IMAGE", run["image"].(map[string]any)["name"], "by digest, never by tag")
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

// S-02: the image jobs run the RUN steps of a project Dockerfile: they use
// nothing but CI_JOB_TOKEN (no oh secret, no target project token, never a
// token in a URL); the secrets are scoped to the environment of the run job
// of the project.
func TestImageJobsHoldNoSecret(t *testing.T) {
	for _, opts := range []Options{{}, {Builder: BuilderDinD}} {
		out, err := Generate(opts)
		require.NoError(t, err)
		var doc map[string]any
		require.NoError(t, yaml.Unmarshal(out, &doc))
		for _, job := range []string{"oh-cli", "oh-image"} {
			j, _ := yaml.Marshal(doc[job])
			for _, secret := range []string{"OH_LLM_KEY", "OH_TEAMSTATE_TOKEN", "OH_PROJECT_TOKEN", "OH_JOURNAL_KEY", "GIT_PASSWORD", "oauth2", "environment"} {
				assert.NotContains(t, string(j), secret, "%s uses %s", job, secret)
			}
			assert.NotRegexp(t, `://[^/ "]*@`, string(j), "%s: credentials in a URL", job)
		}
		env := doc["oh-run"].(map[string]any)["environment"].(map[string]any)
		assert.Equal(t, "oh-run/$OH_PROJECT_ID", env["name"])
		assert.Equal(t, "prepare", env["action"], "no deployment recorded")
	}
}

// imageScript returns the script of the image job.
func imageScript(t *testing.T, opts Options) string {
	t.Helper()
	out, err := Generate(opts)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(out, &doc))
	return doc["oh-image"].(map[string]any)["script"].([]any)[0].(string)
}

// runImageScript runs the script of the image job with fake builders that
// log their arguments, and returns the log and the dotenv written.
func runImageScript(t *testing.T, script string, env map[string]string) (calls, dotenv string, err error) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell")
	}
	dir := t.TempDir()
	cmd := exec.Command("sh", "-c", script)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CI_PROJECT_DIR="+dir, "CALLS="+filepath.Join(dir, "calls"), "PATH="+filepath.Join(dir, "bin")+":"+os.Getenv("PATH"),
		"CI_REGISTRY=r.example", "CI_REGISTRY_USER=u", "CI_REGISTRY_PASSWORD=p", "CI_JOB_TOKEN=j",
		"OH_IMAGE_BASE=r.example:5050/g/oh-runner/acme-api-base:b1", "OH_IMAGE=r.example:5050/g/oh-runner/acme-api:i1", "OH_TOOL_VERSION=2.0.20")
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "bin"), 0o755))
	for name, body := range map[string]string{
		// fake docker: push prints the digest of the tag pushed
		"docker": `echo "docker $*" >> "$CALLS"
case "$1" in push) printf '%s: digest: sha256:%s size: 1\n' "${2##*:}" "$(echo "$2" | sha256sum | cut -c1-64)" ;; esac
`,
		"executor": `echo "executor $*" >> "$CALLS"
while [ $# -gt 0 ]; do case "$1" in --destination) dest=$2 ;; --digest-file) df=$2 ;; esac; shift; done
printf 'sha256:%s' "$(echo "$dest" | sha256sum | cut -c1-64)" > "$df"
`,
		"sha256sum": "shasum -a 256 \"$@\"\n",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "bin", name), []byte("#!/bin/sh\n"+body), 0o755))
	}
	script = strings.ReplaceAll(script, "/kaniko/", filepath.Join(dir, "kaniko")+"/")
	script = strings.ReplaceAll(script, filepath.Join(dir, "kaniko")+"/executor", "executor")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "kaniko"), 0o755))
	cmd.Args = []string{"sh", "-c", script}
	out, err := cmd.CombinedOutput()
	c, _ := os.ReadFile(filepath.Join(dir, "calls"))
	d, _ := os.ReadFile(filepath.Join(dir, "oh-image.env"))
	if err != nil {
		t.Logf("%s", out)
	}
	return string(c), string(d), err
}

// N21: the oh layer is built FROM the base by the digest just pushed, and
// the image job reports the run image by digest (dotenv), with both
// builders.
func TestImageJobPinsByDigest(t *testing.T) {
	if _, err := exec.LookPath("shasum"); err != nil {
		t.Skip("no shasum")
	}
	digestOf := func(ref string) string {
		out, err := exec.Command("sh", "-c", "echo \""+ref+"\" | shasum -a 256 | cut -c1-64").Output()
		require.NoError(t, err)
		return "sha256:" + strings.TrimSpace(string(out))
	}
	base := digestOf("r.example:5050/g/oh-runner/acme-api-base:b1")
	img := digestOf("r.example:5050/g/oh-runner/acme-api:i1")
	for _, opts := range []Options{{}, {Builder: BuilderDinD}} {
		calls, dotenv, err := runImageScript(t, imageScript(t, opts), nil)
		require.NoError(t, err, opts.Builder)
		assert.Contains(t, calls, "OH_IMAGE_BASE=r.example:5050/g/oh-runner/acme-api-base@"+base, "%s: layer FROM the base by digest", opts.Builder)
		assert.Equal(t, "OH_RUN_IMAGE=r.example:5050/g/oh-runner/acme-api@"+img+"\n", dotenv, opts.Builder)
	}
}

// No digest reported by the builder: the image job fails, no dotenv.
func TestImageJobFailsWithoutDigest(t *testing.T) {
	script := strings.ReplaceAll(imageScript(t, Options{Builder: BuilderDinD}), "digest: ", "nodigest: ")
	script = strings.ReplaceAll(script, "docker inspect", "true")
	_, dotenv, err := runImageScript(t, script, nil)
	assert.Error(t, err)
	assert.Empty(t, dotenv)
}
