package cmd

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/gitlabapi"
	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
)

// mrFeedback is a merge request read for the computed inputs of a launch.
type mrFeedback struct {
	mr          *gitlabapi.MRInfo
	branch      string
	discussions []gitlabapi.Discussion
}

// gitlabInputSources are the merge request sources of computed inputs
// (`from:` mr.*) for a GitLab project: the merge request named by an input
// (URL, !iid, iid, branch or ticket), read once per launch.
func gitlabInputSources(a *app.App) map[string]workflowsvc.InputSource {
	var (
		mu    sync.Mutex
		cache = map[string]*mrFeedback{}
	)
	load := func(ctx context.Context, c workflowsvc.Context, ref string) (*mrFeedback, error) {
		mu.Lock()
		defer mu.Unlock()
		if f, ok := cache[ref]; ok {
			return f, nil
		}
		f, err := fetchMRFeedback(ctx, a, c.ProjectID, ref)
		if err != nil {
			return nil, err
		}
		cache[ref] = f
		return f, nil
	}
	return map[string]workflowsvc.InputSource{
		"mr.discussions": func(ctx context.Context, c workflowsvc.Context, ref string) (string, error) {
			f, err := load(ctx, c, ref)
			if err != nil {
				return "", err
			}
			if len(f.discussions) == 0 {
				return "", errors.New(i18n.T("cmd.review.feedback.no_discussions"))
			}
			return formatFeedbackDiscussions(f.discussions), nil
		},
		"mr.source_branch": func(ctx context.Context, c workflowsvc.Context, ref string) (string, error) {
			f, err := load(ctx, c, ref)
			if err != nil {
				return "", err
			}
			return f.branch, nil
		},
		"mr.target_branch": func(ctx context.Context, c workflowsvc.Context, ref string) (string, error) {
			f, err := load(ctx, c, ref)
			if err != nil {
				return "", err
			}
			return f.mr.TargetBranch, nil
		},
	}
}

var reMRIID = regexp.MustCompile(`(?:merge_requests/|^!?)(\d+)/?$`)

// mrIID reads the IID of a merge request reference (URL, !12 or 12).
func mrIID(ref string) (int, bool) {
	m := reMRIID.FindStringSubmatch(strings.TrimSpace(ref))
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	return n, err == nil
}

// fetchMRFeedback reads a merge request of a project and its unresolved
// discussions (GitLab token and URL of the hub, tracker project).
func fetchMRFeedback(ctx context.Context, a *app.App, projectID, ref string) (*mrFeedback, error) {
	project, err := a.Projects.Get(ctx, projectID)
	if err != nil {
		return nil, err
	}
	token := resolveGitLabToken(ctx, a)
	if token == "" {
		return nil, errors.New(i18n.Tf("cmd.review.feedback.no_token", "oh mcp setup gitlab"))
	}
	glProject := resolveGitLabProject(a, project)
	if glProject == "" {
		return nil, errors.New(i18n.Tf("cmd.review.feedback.no_project", "tracker_project"))
	}
	gl := gitlabapi.NewClient(resolveGitLabURL(a), token)
	var (
		mr     *gitlabapi.MRInfo
		branch string
	)
	if iid, ok := mrIID(ref); ok {
		if mr, err = gl.GetMR(ctx, glProject, iid); err != nil {
			return nil, err
		}
		branch = mr.SourceBranch
	} else if mr, branch, err = resolveMRFromRef(ctx, gl, glProject, project.Path, ref); err != nil {
		return nil, err
	}
	if mr == nil {
		return nil, errors.New(i18n.Tf("cmd.review.feedback.no_mr", ref))
	}
	discussions, err := gl.ListMRDiscussions(ctx, glProject, mr.IID, true)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.T("cmd.review.feedback.discussions_error"), err)
	}
	return &mrFeedback{mr: mr, branch: branch, discussions: discussions}, nil
}
