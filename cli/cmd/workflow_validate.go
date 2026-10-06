package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/workflow"
	"github.com/datichb/openhub/cli/internal/workflow/hubcat"
)

func workflowValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [<file>|<id>|<layer>:<id>]",
		Short: "Valide un workflow (fichier ou identifiant du catalogue)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			layer, _ := cmd.Flags().GetString("layer")
			asJSON, _ := cmd.Flags().GetBool("json")
			all, _ := cmd.Flags().GetBool("all")
			if (len(args) == 0) == !all {
				return errors.New(i18n.T("cmd.workflow.validate.usage"))
			}
			target := ""
			if len(args) == 1 {
				target = args[0]
			}
			projectRef, _ := cmd.Flags().GetString("project")
			layers, err := resolveWorkflowTeamLayers(cmd.Context(), TryApp(), projectRef)
			if err != nil {
				return err
			}
			report, err := runWorkflowValidate(findHubDir(), target, workflow.Layer(layer), all, layers)
			if err != nil {
				return err
			}
			return printWorkflowValidate(cmd.OutOrStdout(), report, asJSON)
		},
	}
	cmd.Flags().String("layer", string(workflow.LayerHub), "Couche du fichier validé (hub, team, project)")
	cmd.Flags().Bool("json", false, "Sortie JSON")
	cmd.Flags().Bool("all", false, "Valide tous les workflows du hub")
	cmd.Flags().String("project", "", i18n.T("teamstate.workflow.flag_project"))
	return cmd
}

// workflowValidateReport is the result of `oh workflow validate`.
type workflowValidateReport struct {
	Workflows   []workflowValidateItem `json:"workflows"`
	Diagnostics workflow.Diagnostics   `json:"diagnostics"`
}

type workflowValidateItem struct {
	Ref   string `json:"ref"`
	Valid bool   `json:"valid"`
}

func (r *workflowValidateReport) counts() (errs, warns int) {
	for _, d := range r.Diagnostics {
		if d.Severity == workflow.SeverityError {
			errs++
		} else {
			warns++
		}
	}
	return errs, warns
}

// runWorkflowValidate validates a file, a catalogue id or (all) every
// workflow against the hub content in hubDir and, when layers is set, the
// published team and project workflows (integrity-checked).
func runWorkflowValidate(hubDir, target string, layer workflow.Layer, all bool, layers *workflowTeamLayers) (*workflowValidateReport, error) {
	if !layer.IsDocumentLayer() {
		return nil, fmt.Errorf("%s", i18n.Tf("cmd.workflow.validate.bad_layer", string(layer)))
	}
	cat := workflow.NewMemCatalog()
	var loadDiags workflow.Diagnostics
	env := workflow.Env{}
	if hubDir != "" {
		cat, loadDiags = hubcat.LoadWorkflows(hubDir)
		hc, err := hubcat.New(hubDir)
		if err != nil {
			return nil, err
		}
		env = hc.Env()
		env.Skills = bundle.NewSkillCatalog(hubDir)
	}
	if layers != nil {
		loadDiags = append(loadDiags, layers.Repo.LoadWorkflowLayers(cat, layers.Project)...)
		env.Prompts = teamstate.PromptSource{Repo: layers.Repo, Fallback: env.Prompts}
	}

	report := &workflowValidateReport{Diagnostics: workflow.Diagnostics{}}
	var refs []workflow.Ref
	switch {
	case all:
		report.Diagnostics = append(report.Diagnostics, loadDiags...)
		refs = cat.Refs()
		// Files that could not be loaded are listed as invalid.
		seen := map[string]bool{}
		for _, d := range loadDiags {
			name := strings.TrimSuffix(filepath.Base(d.Source), filepath.Ext(d.Source))
			if d.Severity == workflow.SeverityError && name != "" && !seen[name] {
				seen[name] = true
				report.Workflows = append(report.Workflows, workflowValidateItem{Ref: workflow.Ref{Layer: sourceLayer(d.Source, layers), ID: name}.String()})
			}
		}
	case isWorkflowFile(target):
		doc, diags := workflow.ParseFile(target, layer)
		report.Diagnostics = append(report.Diagnostics, diags...)
		if doc == nil || diags.HasErrors() {
			name := strings.TrimSuffix(filepath.Base(target), filepath.Ext(target))
			report.Workflows = append(report.Workflows, workflowValidateItem{Ref: string(layer) + ":" + name})
			return report, nil
		}
		cat.Put(doc)
		refs = []workflow.Ref{doc.Ref()}
	default:
		ref, err := workflow.ParseRef(target)
		if err != nil {
			ref = workflow.Ref{Layer: workflow.LayerHub, ID: target}
		}
		// Load errors of the requested file explain an unknown workflow.
		for _, d := range loadDiags {
			if base := filepath.Base(d.Source); base == ref.ID+".yaml" || base == ref.ID+".yml" {
				report.Diagnostics = append(report.Diagnostics, d)
			}
		}
		refs = []workflow.Ref{ref}
	}

	for _, ref := range refs {
		_, diags := workflow.Check(cat, ref, nil, env)
		report.Diagnostics = append(report.Diagnostics, diags...)
		report.Workflows = append(report.Workflows, workflowValidateItem{Ref: ref.String(), Valid: !diags.HasErrors()})
	}
	if !all && report.Diagnostics.HasErrors() {
		report.Workflows[0].Valid = false
	}
	report.Diagnostics.Sort()
	return report, nil
}

// sourceLayer guesses the layer of a file that failed to load.
func sourceLayer(source string, layers *workflowTeamLayers) workflow.Layer {
	if layers == nil {
		return workflow.LayerHub
	}
	root := layers.Repo.Path() + string(filepath.Separator)
	switch {
	case strings.HasPrefix(source, filepath.Join(root, "projects")+string(filepath.Separator)):
		return workflow.LayerProject
	case strings.HasPrefix(source, root):
		return workflow.LayerTeam
	}
	return workflow.LayerHub
}

func isWorkflowFile(target string) bool {
	if ext := filepath.Ext(target); ext == ".yaml" || ext == ".yml" {
		return true
	}
	st, err := os.Stat(target)
	return err == nil && !st.IsDir()
}

func printWorkflowValidate(w io.Writer, r *workflowValidateReport, asJSON bool) error {
	errs, warns := r.counts()
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(r); err != nil {
			return err
		}
	} else {
		for _, d := range r.Diagnostics {
			icon, style := theme.IconError, theme.ErrorStyle
			if d.Severity == workflow.SeverityWarning {
				icon, style = theme.IconWarning, theme.WarningStyle
			}
			loc := d.Source
			if p := d.Pos.String(); p != "" {
				loc += ":" + p
			}
			head := loc
			if d.Path != "" {
				head += "  " + d.Path
			}
			fmt.Fprintf(w, "%s %s\n    %s\n", style.Render(icon), head, d.Message)
			if d.Hint != "" {
				fmt.Fprintf(w, "    %s\n", theme.Subtitle.Render(d.Hint))
			}
		}
		for _, wf := range r.Workflows {
			if wf.Valid {
				fmt.Fprintf(w, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.workflow.validate.valid", wf.Ref))
			} else {
				fmt.Fprintf(w, "%s %s\n", theme.ErrorStyle.Render(theme.IconError), i18n.Tf("cmd.workflow.validate.invalid", wf.Ref))
			}
		}
		if len(r.Workflows) == 0 {
			fmt.Fprintln(w, i18n.T("cmd.workflow.validate.none"))
		}
		if warns > 0 && errs == 0 {
			fmt.Fprintln(w, i18n.Tf("cmd.workflow.validate.warnings", warns))
		}
	}
	if errs > 0 {
		return errors.New(i18n.Tf("cmd.workflow.validate.failed", errs, warns))
	}
	return nil
}
