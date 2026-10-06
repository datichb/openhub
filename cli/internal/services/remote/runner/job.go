// Package runner is the CI side of the remote runtime (`oh runner`, v5
// phase 5, lot 5.B): it fetches the session sent by the machine, clones the
// project, runs the session with the same daemon, proxy and RunService code
// as on the machine, answers its decisions by the workflow policy (never
// --auto), pushes the branch and the merge request, and writes the artifacts.
package runner

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/datichb/openhub/cli/internal/remote"
)

// Job is what the pipeline gives the run job.
type Job struct {
	SessionID   string
	ProjectID   int64
	ProjectPath string
	Ref         string
	Commit      string
	BundleURL   string
	SessionURL  string
	ServerURL   string // CI_SERVER_URL
	PipelineID  int64  // CI_PIPELINE_ID
	PipelineURL string // CI_PIPELINE_URL
	Provider    string
	Region      string
	Secrets     Secrets
}

// Secrets are the masked CI variables of the job. They never reach the
// tool server, the agents' shells or the artifacts.
type Secrets struct {
	LLMKey         string
	ProjectToken   string
	TeamStateToken string
	JobToken       string // CI_JOB_TOKEN: packages of oh-runner
}

// Values returns the secret values (for scrubbing logs and artifacts).
func (s Secrets) Values() []string {
	var out []string
	for _, v := range []string{s.LLMKey, s.ProjectToken, s.TeamStateToken, s.JobToken} {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// ReadJob reads the job variables.
func ReadJob(getenv func(string) string) (*Job, error) {
	j := &Job{
		SessionID: getenv(remote.VarSessionID), ProjectPath: getenv(remote.VarProjectPath), Ref: getenv(remote.VarRef),
		Commit: getenv(remote.VarCommit), BundleURL: getenv(remote.VarBundleURL), SessionURL: getenv(remote.VarSessionURL),
		ServerURL: strings.TrimRight(getenv("CI_SERVER_URL"), "/"),
		Provider:  getenv(remote.VarLLMProvider), Region: getenv(remote.VarLLMRegion),
	}
	j.PipelineID, _ = strconv.ParseInt(getenv("CI_PIPELINE_ID"), 10, 64)
	j.PipelineURL = getenv("CI_PIPELINE_URL")
	var missing []string
	for name, v := range map[string]string{remote.VarSessionID: j.SessionID, remote.VarProjectPath: j.ProjectPath,
		remote.VarRef: j.Ref, remote.VarBundleURL: j.BundleURL, remote.VarSessionURL: j.SessionURL,
		remote.VarLLMProvider: j.Provider} {
		if v == "" {
			missing = append(missing, name)
		}
	}
	id, err := strconv.ParseInt(getenv(remote.VarProjectID), 10, 64)
	if err != nil || id <= 0 {
		missing = append(missing, remote.VarProjectID)
	}
	j.ProjectID = id
	if getenv(remote.VarPipelineSchema) != "" && getenv(remote.VarPipelineSchema) != strconv.Itoa(remote.PipelineSchema) {
		return nil, fmt.Errorf("pipeline schema %s, this oh expects %d: run oh remote setup", getenv(remote.VarPipelineSchema), remote.PipelineSchema)
	}
	j.Secrets = Secrets{
		LLMKey: getenv(remote.VarLLMKey), TeamStateToken: getenv(remote.VarTeamStateToken), JobToken: getenv("CI_JOB_TOKEN"),
	}
	if id > 0 {
		j.Secrets.ProjectToken = getenv(remote.ProjectTokenVar(id))
	}
	if j.Secrets.LLMKey == "" {
		missing = append(missing, remote.VarLLMKey)
	}
	if j.Secrets.ProjectToken == "" && id > 0 {
		missing = append(missing, remote.ProjectTokenVar(id))
	}
	if j.Secrets.JobToken == "" {
		missing = append(missing, "CI_JOB_TOKEN")
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("missing CI variables: %s", strings.Join(missing, ", "))
	}
	if j.ServerURL == "" {
		return nil, errors.New("missing CI variable CI_SERVER_URL")
	}
	return j, nil
}

// secretNameMarks identify the environment variables removed from the job
// process before anything is started (daemon, bd, MCP servers, tool server).
var secretNameMarks = []string{"TOKEN", "PASSWORD", "SECRET", "_KEY", "PASSPHRASE", "CREDENTIAL", "JWT", "AUTH"}

// ScrubEnv removes the secret variables from the process environment (the
// processes started afterwards inherit the cleaned environment). It returns
// the names removed.
func ScrubEnv() []string {
	var removed []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		up := strings.ToUpper(name)
		for _, m := range secretNameMarks {
			if strings.Contains(up, m) {
				_ = os.Unsetenv(name)
				removed = append(removed, name)
				break
			}
		}
	}
	return removed
}

// Redact replaces the secret values in s.
func (s Secrets) Redact(text string) string {
	for _, v := range s.Values() {
		if len(v) >= 4 {
			text = strings.ReplaceAll(text, v, "[masked]")
		}
	}
	return text
}
