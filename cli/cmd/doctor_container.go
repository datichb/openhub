package cmd

import (
	"context"
	"runtime"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/runtime/container"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/worktree"
)

// Doctor › container (P4-T12): engine, virtiofs, keep-id, folders shared
// with the VM, the tool in the project images (version, C++ libraries on
// musl), Linux second listener.

func init() { registerDoctorCheck(containerDoctorChecks) }

// doctorProjectImages is the number of project images checked (newest first).
const doctorProjectImages = 3

func containerDoctorChecks(ctx context.Context) []views.DoctorCheck {
	a := TryApp()
	rt := v5ContainerRuntime(a)
	e, av := rt.Engine(ctx)
	check := func(key string, ok bool, detail string) views.DoctorCheck {
		return views.DoctorCheck{Name: i18n.T("tui.doctor.container." + key), OK: ok, Detail: detail}
	}
	if !av.OK {
		// No engine at all, or Windows: the container runtime is optional.
		optional := av.Reason == "cmd.runtime.container.no_engine" || av.Reason == "cmd.runtime.container.unsupported_os"
		return []views.DoctorCheck{check("engine", optional, av.Message())}
	}
	engine := strings.TrimSpace(string(e.Kind) + " " + e.Version)
	if len(e.Details) > 0 {
		engine += " (" + strings.Join(e.Details, ", ") + ")"
	}
	out := []views.DoctorCheck{check("engine", true, engine)}

	client := v5ToolVersion()
	if pinned := executionConfig(a).ToolVersion; client != "" && !pinnedVersionOK(pinned, client) {
		out = append(out, check("pinned", false, i18n.Tf("tui.settings.exec.tool.mismatch", toolName(), pinned, client)))
	}

	if mount := engineDetail(e, "mount"); e.Kind == container.EngineColima && mount != "" {
		if mount == "virtiofs" {
			out = append(out, check("mount", true, mount))
		} else {
			out = append(out, check("mount", false, i18n.Tf("tui.doctor.container.mount_slow", mount)))
		}
	}

	if e.Kind == container.EnginePodman && e.Rootless {
		ok, err := rt.KeepID(ctx, e, container.ProbeImage)
		switch {
		case err != nil:
			out = append(out, check("keep_id", false, i18n.Tf("tui.doctor.container.probe_failed", err.Error())))
		case ok:
			out = append(out, check("keep_id", true, i18n.T("tui.doctor.container.keep_id_ok")))
		default:
			out = append(out, check("keep_id", false, i18n.T("tui.doctor.container.keep_id_bad")))
		}
	}

	if e.VM && a != nil {
		dirs := doctorProjectDirs(ctx, a.Projects)
		if missing := container.NotShared(e, dirs); len(missing) > 0 {
			out = append(out, check("shared", false, i18n.Tf("tui.doctor.container.not_shared", strings.Join(missing, ", "), strings.Join(e.Shared, ", "))))
		} else if len(dirs) > 0 {
			out = append(out, check("shared", true, i18n.Tf("tui.doctor.container.shared_ok", len(dirs))))
		}
	}

	out = append(out, imageDoctorChecks(ctx, rt, e, client, check)...)

	if runtime.GOOS == "linux" && !e.VM {
		if ip, err := rt.HostIP(ctx, e, container.ProbeImage); err != nil {
			out = append(out, check("listener", false, i18n.Tf("tui.doctor.container.listener_failed", err.Error())))
		} else {
			out = append(out, check("listener", true, i18n.Tf("tui.doctor.container.listener_ok", ip)))
		}
	}
	return out
}

// imageDoctorChecks runs the tool in the latest project images.
func imageDoctorChecks(ctx context.Context, rt *container.Runtime, e container.Engine, client string, check func(string, bool, string) views.DoctorCheck) []views.DoctorCheck {
	images, err := rt.ProjectImages(ctx, e)
	if err != nil {
		return []views.DoctorCheck{check("images", false, i18n.Tf("tui.doctor.container.probe_failed", err.Error()))}
	}
	if len(images) == 0 {
		return []views.DoctorCheck{check("images", true, i18n.T("tui.doctor.container.no_image"))}
	}
	var out []views.DoctorCheck
	for i, img := range images {
		if i == doctorProjectImages {
			break
		}
		tc := rt.CheckTool(ctx, e, img.Ref, v5Tool.Command)
		name := i18n.Tf("tui.doctor.container.image", img.ProjectID)
		switch {
		case len(tc.MissingLibs) > 0:
			out = append(out, views.DoctorCheck{Name: name, Detail: i18n.Tf("tui.doctor.container.musl_libs", strings.Join(tc.MissingLibs, ", "))})
		case tc.Version == "":
			detail := img.Ref
			if tc.Err != nil {
				detail += " : " + tc.Err.Error()
			}
			out = append(out, views.DoctorCheck{Name: name, Detail: i18n.Tf("tui.doctor.container.tool_failed", toolName(), detail)})
		case client != "" && tc.Version != strings.TrimPrefix(client, "v"):
			out = append(out, views.DoctorCheck{Name: name, OK: true, Detail: i18n.Tf("tui.doctor.container.tool_outdated", toolName(), tc.Version, client)})
		default:
			out = append(out, views.DoctorCheck{Name: name, OK: true, Detail: i18n.Tf("tui.doctor.container.tool_ok", toolName(), tc.Version, img.Ref)})
		}
	}
	return out
}

// engineDetail returns a detail of the engine (« mount=virtiofs » → virtiofs).
func engineDetail(e container.Engine, key string) string {
	for _, d := range e.Details {
		if v, ok := strings.CutPrefix(d, key+"="); ok {
			return v
		}
	}
	return ""
}

// doctorProjectDirs lists the active projects and their worktrees (the
// folders a container session mounts).
func doctorProjectDirs(ctx context.Context, projects domain.ProjectStore) []string {
	if projects == nil {
		return nil
	}
	list, err := projects.List(ctx, domain.ProjectStatusActive)
	if err != nil {
		return nil
	}
	var dirs []string
	seen := map[string]bool{}
	add := func(d string) {
		if d != "" && !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	for _, p := range list {
		add(p.Path)
		if worktree.IsGitRepo(p.Path) {
			if wts, err := worktree.List(p.Path); err == nil {
				for _, w := range wts {
					if !w.IsBare {
						add(w.Path)
					}
				}
			}
		}
	}
	return dirs
}
