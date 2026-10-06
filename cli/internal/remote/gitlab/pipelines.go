package gitlab

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Pipeline is a CI pipeline.
type Pipeline struct {
	ID        int64      `json:"id"`
	Status    string     `json:"status"` // created | pending | running | success | failed | canceled | skipped | manual
	Ref       string     `json:"ref"`
	SHA       string     `json:"sha"`
	WebURL    string     `json:"web_url"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
}

// Finished reports whether the pipeline will not change any more.
func (p *Pipeline) Finished() bool {
	switch p.Status {
	case "success", "failed", "canceled", "skipped":
		return true
	}
	return false
}

// TriggerPipeline starts a pipeline of project on ref with a trigger token
// and pipeline variables (sorted, form-encoded: the token never appears in a
// URL).
func (c *Client) TriggerPipeline(ctx context.Context, project, ref, token string, vars map[string]string) (*Pipeline, error) {
	form := url.Values{"token": {token}, "ref": {ref}}
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		form.Set("variables["+k+"]", vars[k])
	}
	body := strings.NewReader(form.Encode())
	resp, err := c.send(ctx, http.MethodPost, projectRef(project)+"/trigger/pipeline", body, "application/x-www-form-urlencoded")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var p Pipeline
	if err := decodeJSON(resp.Body, &p); err != nil {
		return nil, fmt.Errorf("gitlab: trigger: %w", err)
	}
	return &p, nil
}

// Pipeline returns a pipeline of project.
func (c *Client) Pipeline(ctx context.Context, project string, id int64) (*Pipeline, error) {
	var p Pipeline
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("%s/pipelines/%d", projectRef(project), id), nil, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// RegistryRepository is a container repository of a project.
type RegistryRepository struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Location string `json:"location"`
}

// RegistryTagExists reports whether the container repository name (relative
// to the project) has tag.
func (c *Client) RegistryTagExists(ctx context.Context, project, name, tag string) (bool, error) {
	var repos []RegistryRepository
	q := url.Values{"search": {name}, "per_page": {"100"}}
	if err := c.do(ctx, http.MethodGet, projectRef(project)+"/registry/repositories?"+q.Encode(), nil, &repos); err != nil {
		return false, err
	}
	for _, r := range repos {
		if r.Name != name {
			continue
		}
		err := c.do(ctx, http.MethodGet, fmt.Sprintf("%s/registry/repositories/%d/tags/%s", projectRef(project), r.ID, url.PathEscape(tag)), nil, nil)
		if IsNotFound(err) {
			return false, nil
		}
		return err == nil, err
	}
	return false, nil
}

// Job is a CI job of a pipeline.
type Job struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Stage  string `json:"stage"`
	Status string `json:"status"`
	WebURL string `json:"web_url"`
}

// PipelineJobs lists the jobs of a pipeline.
func (c *Client) PipelineJobs(ctx context.Context, project string, pipeline int64) ([]Job, error) {
	var jobs []Job
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("%s/pipelines/%d/jobs?per_page=100", projectRef(project), pipeline), nil, &jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

// JobArtifacts downloads the artifacts archive (zip) of a job.
func (c *Client) JobArtifacts(ctx context.Context, project string, job int64) ([]byte, error) {
	resp, err := c.send(ctx, http.MethodGet, fmt.Sprintf("%s/jobs/%d/artifacts", projectRef(project), job), nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, 1<<30))
}
