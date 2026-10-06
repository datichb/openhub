// Package remote holds the contract shared by the machine side of the remote
// runtime (oh remote setup, sending, fetching) and the CI job (oh runner):
// CI variable names, package names and the pipeline schema version.
package remote

import (
	"fmt"
	"regexp"
	"strconv"
)

// PipelineSchema is the version of the generated .gitlab-ci.yml contract
// (variables, stages, artifacts). Bump it on any incompatible change: the
// machine refuses to send to an oh-runner project with another schema.
const PipelineSchema = 1

// CI variables of the oh-runner project (set by oh remote setup). Secret ones
// are masked and protected; they never leave GitLab except into the job.
const (
	VarLLMProvider     = "OH_LLM_PROVIDER"    // oh provider name (bedrock, anthropic, openrouter)
	VarLLMRegion       = "OH_LLM_REGION"      // provider region (Bedrock)
	VarLLMKey          = "OH_LLM_KEY"         // secret: provider API key, read by the job's proxy only
	VarTeamStateToken  = "OH_TEAMSTATE_TOKEN" // secret: write_repository on the team-state repository
	projectTokenPrefix = "OH_PROJECT_TOKEN_"  // secret, + project ID: write_repository (+ read_api) on that project
)

// ProjectTokenVar returns the CI variable holding the access token of the
// target project id.
func ProjectTokenVar(projectID int64) string {
	return projectTokenPrefix + strconv.FormatInt(projectID, 10)
}

var projectTokenVarRe = regexp.MustCompile(`^OH_PROJECT_TOKEN_(\d+)$`)

// ProjectTokenID returns the project ID of a ProjectTokenVar name.
func ProjectTokenID(name string) (int64, bool) {
	m := projectTokenVarRe.FindStringSubmatch(name)
	if m == nil {
		return 0, false
	}
	id, err := strconv.ParseInt(m[1], 10, 64)
	return id, err == nil
}

// Pipeline (trigger) variables, set per session by the machine. None of them
// is secret: they are visible in the pipeline page.
const (
	VarSessionID      = "OH_SESSION_ID"      // oh session id (ses_…)
	VarProjectID      = "OH_PROJECT_ID"      // GitLab ID of the target project
	VarProjectPath    = "OH_PROJECT_PATH"    // full path of the target project
	VarRef            = "OH_REF"             // branch of the target project to start from
	VarCommit         = "OH_COMMIT"          // commit expected at OH_REF (optional)
	VarWorkflow       = "OH_WORKFLOW"        // workflow id (display)
	VarBundleURL      = "OH_BUNDLE_URL"      // generic package file: the session bundle (by hash)
	VarSessionURL     = "OH_SESSION_URL"     // generic package file: the session envelope (by hash)
	VarImage          = "OH_IMAGE"           // project image of the job (registry, tag = hash)
	VarImageBase      = "OH_IMAGE_BASE"      // project base image (dev Dockerfile)
	VarImageBuild     = "OH_IMAGE_BUILD"     // "true" when OH_IMAGE is missing from the registry
	VarDockerfile     = "OH_DOCKERFILE"      // dev Dockerfile path in the project ("" = oh default base)
	VarCLIVersion     = "OH_CLI_VERSION"     // oh version (release download when OH_CLI_URL is empty)
	VarCLIURL         = "OH_CLI_URL"         // generic package file of a development oh binary
	VarCLISHA256      = "OH_CLI_SHA256"      // expected SHA-256 of the oh binary or release archive
	VarPipelineSchema = "OH_PIPELINE_SCHEMA" // set by the generated file itself
)

// Generic packages of the oh-runner project.
const (
	PackageCLI     = "oh-cli"     // development oh binaries: version = SHA-256 prefix, file oh-linux-<arch>
	PackageBundle  = "oh-bundle"  // session bundles: version = bundle hash, file bundle.tar.gz
	PackageSession = "oh-session" // session envelopes: version = envelope hash, file session.tar.gz
)

// CLIFile is the file name of a development oh binary for arch.
func CLIFile(arch string) string { return "oh-linux-" + arch }

// OutDir is the artifacts directory of the run job, relative to
// $CI_PROJECT_DIR (journal.jsonl, summary.json, session.export).
const OutDir = "oh-out"

// RunnerProjectName is the default path of the oh-runner project in a group.
const RunnerProjectName = "oh-runner"

// ReleaseURL is the release archive of oh for a version and architecture
// (goreleaser archives).
func ReleaseURL(version, arch string) string {
	return fmt.Sprintf("https://github.com/datichb/openhub/releases/download/v%s/openhub_linux_%s.tar.gz", version, arch)
}

// ReleaseChecksumsURL is the checksums file of a release.
func ReleaseChecksumsURL(version string) string {
	return fmt.Sprintf("https://github.com/datichb/openhub/releases/download/v%s/checksums.txt", version)
}
