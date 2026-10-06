package gitlab

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// User is the authenticated user.
type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

// CurrentUser returns the user of the token (checks API access).
func (c *Client) CurrentUser(ctx context.Context) (*User, error) {
	var u User
	if err := c.do(ctx, http.MethodGet, "/user", nil, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// Group is a GitLab group (namespace).
type Group struct {
	ID       int64  `json:"id"`
	FullPath string `json:"full_path"`
	WebURL   string `json:"web_url"`
}

// Group returns the group at fullPath.
func (c *Client) Group(ctx context.Context, fullPath string) (*Group, error) {
	var g Group
	if err := c.do(ctx, http.MethodGet, "/groups/"+PathID(fullPath), nil, &g); err != nil {
		return nil, err
	}
	return &g, nil
}

// Project is a GitLab project.
type Project struct {
	ID                int64  `json:"id"`
	PathWithNamespace string `json:"path_with_namespace"`
	DefaultBranch     string `json:"default_branch"`
	WebURL            string `json:"web_url"`
	HTTPURLToRepo     string `json:"http_url_to_repo"`
	EmptyRepo         bool   `json:"empty_repo"`
	PackagesEnabled   bool   `json:"packages_enabled"`
	// PackageRegistryAccessLevel replaces packages_enabled (GitLab 16+).
	PackageRegistryAccessLevel string `json:"package_registry_access_level"`
	// ContainerRegistryAccessLevel is disabled | private | enabled.
	ContainerRegistryAccessLevel string `json:"container_registry_access_level"`
	ContainerRegistryImagePrefix string `json:"container_registry_image_prefix"`
	BuildsAccessLevel            string `json:"builds_access_level"`
}

// RegistryEnabled reports whether the container registry is on.
func (p *Project) RegistryEnabled() bool {
	return p.ContainerRegistryAccessLevel != "" && p.ContainerRegistryAccessLevel != "disabled"
}

// PackagesOn reports whether the package registry is on.
func (p *Project) PackagesOn() bool {
	if p.PackageRegistryAccessLevel != "" {
		return p.PackageRegistryAccessLevel != "disabled"
	}
	return p.PackagesEnabled
}

// Project returns the project by full path or numeric ID.
func (c *Client) Project(ctx context.Context, project string) (*Project, error) {
	var p Project
	if err := c.do(ctx, http.MethodGet, projectRef(project), nil, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// CreateProjectRequest creates a private project in a group.
type CreateProjectRequest struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	NamespaceID int64  `json:"namespace_id"`
	Description string `json:"description,omitempty"`
}

// CreateProject creates a private project with packages and the container
// registry on, and an initialized default branch.
func (c *Client) CreateProject(ctx context.Context, req CreateProjectRequest) (*Project, error) {
	body := map[string]any{
		"name":                            req.Name,
		"path":                            req.Path,
		"namespace_id":                    req.NamespaceID,
		"description":                     req.Description,
		"visibility":                      "private",
		"initialize_with_readme":          true,
		"packages_enabled":                true,
		"container_registry_access_level": "private",
		"builds_access_level":             "private",
		"issues_access_level":             "disabled",
		"wiki_access_level":               "disabled",
	}
	var p Project
	if err := c.do(ctx, http.MethodPost, "/projects", body, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// File returns the content of a repository file at ref.
func (c *Client) File(ctx context.Context, project, path, ref string) ([]byte, error) {
	var f struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	p := projectRef(project) + "/repository/files/" + PathID(path) + "?ref=" + url.QueryEscape(ref)
	if err := c.do(ctx, http.MethodGet, p, nil, &f); err != nil {
		return nil, err
	}
	if f.Encoding == "base64" {
		data, err := base64.StdEncoding.DecodeString(f.Content)
		if err != nil {
			return nil, fmt.Errorf("gitlab: decoding %s: %w", path, err)
		}
		return data, nil
	}
	return []byte(f.Content), nil
}

// CommitFile creates (create=true) or updates a repository file on branch.
func (c *Client) CommitFile(ctx context.Context, project, branch, path string, content []byte, message string, create bool) error {
	method := http.MethodPut
	if create {
		method = http.MethodPost
	}
	body := map[string]string{
		"branch":         branch,
		"content":        base64.StdEncoding.EncodeToString(content),
		"encoding":       "base64",
		"commit_message": message,
	}
	return c.do(ctx, method, projectRef(project)+"/repository/files/"+PathID(path), body, nil)
}

// Variable is a project CI/CD variable. Value is never logged.
type Variable struct {
	Key         string `json:"key"`
	Value       string `json:"value,omitempty"`
	Masked      bool   `json:"masked"`
	Protected   bool   `json:"protected"`
	Raw         bool   `json:"raw"`
	Description string `json:"description,omitempty"`
}

// Variables lists the project variables (keys and flags; values are dropped).
func (c *Client) Variables(ctx context.Context, project string) ([]Variable, error) {
	var out []Variable
	for page := 1; ; page++ {
		var vs []Variable
		p := projectRef(project) + "/variables?per_page=100&page=" + strconv.Itoa(page)
		if err := c.do(ctx, http.MethodGet, p, nil, &vs); err != nil {
			return nil, err
		}
		for i := range vs {
			vs[i].Value = ""
		}
		out = append(out, vs...)
		if len(vs) < 100 {
			return out, nil
		}
	}
}

// SetVariable creates or replaces a project variable.
func (c *Client) SetVariable(ctx context.Context, project string, v Variable) error {
	body := map[string]any{
		"value":     v.Value,
		"masked":    v.Masked,
		"protected": v.Protected,
		"raw":       v.Raw,
	}
	if v.Description != "" {
		body["description"] = v.Description
	}
	err := c.do(ctx, http.MethodPut, projectRef(project)+"/variables/"+url.PathEscape(v.Key), body, nil)
	if !IsNotFound(err) {
		return err
	}
	body["key"] = v.Key
	return c.do(ctx, http.MethodPost, projectRef(project)+"/variables", body, nil)
}

// Trigger is a pipeline trigger token.
type Trigger struct {
	ID          int64  `json:"id"`
	Description string `json:"description"`
	Token       string `json:"token"`
}

// Triggers lists the pipeline triggers of the project.
func (c *Client) Triggers(ctx context.Context, project string) ([]Trigger, error) {
	var ts []Trigger
	if err := c.do(ctx, http.MethodGet, projectRef(project)+"/triggers?per_page=100", nil, &ts); err != nil {
		return nil, err
	}
	return ts, nil
}

// CreateTrigger creates a pipeline trigger token.
func (c *Client) CreateTrigger(ctx context.Context, project, description string) (*Trigger, error) {
	var t Trigger
	if err := c.do(ctx, http.MethodPost, projectRef(project)+"/triggers", map[string]string{"description": description}, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// Runner is a CI runner available to a project.
type Runner struct {
	ID          int64  `json:"id"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Online      bool   `json:"online"`
	RunnerType  string `json:"runner_type"`
}

// OnlineRunners lists the online runners available to the project that carry
// tag.
func (c *Client) OnlineRunners(ctx context.Context, project, tag string) ([]Runner, error) {
	q := url.Values{"status": {"online"}, "per_page": {"100"}}
	if tag != "" {
		q.Set("tag_list", tag)
	}
	var rs []Runner
	if err := c.do(ctx, http.MethodGet, projectRef(project)+"/runners?"+q.Encode(), nil, &rs); err != nil {
		return nil, err
	}
	return rs, nil
}

// BranchProtected reports whether branch is a protected branch (protected
// CI variables are only exposed to pipelines on protected refs).
func (c *Client) BranchProtected(ctx context.Context, project, branch string) (bool, error) {
	err := c.do(ctx, http.MethodGet, projectRef(project)+"/protected_branches/"+url.PathEscape(branch), nil, nil)
	if IsNotFound(err) {
		return false, nil
	}
	return err == nil, err
}
