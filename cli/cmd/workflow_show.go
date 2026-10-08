package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/workflow"
)

func init() {
	workflowCmd.AddCommand(workflowShowCmd())
}

func workflowShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <id>|<layer>:<id>",
		Short: "Affiche un workflow résolu (avec l'origine des valeurs)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			origin, _ := cmd.Flags().GetBool("origin")
			c, err := workflowReadContext(cmd)
			if err != nil {
				return err
			}
			res, err := newWorkflowService(cmd.Context()).Resolve(cmd.Context(), c, args[0], workflowsvc.ResolveOpts{})
			var invalid *workflowsvc.InvalidError
			_ = errors.As(err, &invalid)
			if res == nil {
				if errors.Is(err, workflowsvc.ErrUnknownWorkflow) {
					return unknownWorkflowError(args[0], c)
				}
				if invalid != nil {
					printDiagnostics(cmd.OutOrStdout(), invalid.Diagnostics)
					return errors.New(i18n.Tf("cmd.workflow.show.invalid", len(invalid.Diagnostics.Errors())))
				}
				return err
			}
			if asJSON {
				if err := printWorkflowShowJSON(cmd.OutOrStdout(), res); err != nil {
					return err
				}
			} else {
				printWorkflowShow(cmd.OutOrStdout(), res, origin)
			}
			if invalid != nil {
				return errors.New(i18n.Tf("cmd.workflow.show.invalid", len(invalid.Diagnostics.Errors())))
			}
			return nil
		},
	}
	cmd.Flags().Bool("origin", false, "Affiche la couche qui a posé chaque valeur")
	cmd.Flags().Bool("json", false, "Sortie JSON")
	addWorkflowContextFlags(cmd)
	return cmd
}

// workflowShowJSON is the JSON form of `oh workflow show`.
type workflowShowJSON struct {
	Ref         string                     `json:"ref"`
	Chain       []string                   `json:"chain"`
	Mode        string                     `json:"mode"`
	Runtime     workflow.Runtime           `json:"runtime"`
	EntryAgent  string                     `json:"entry_agent"`
	Spec        map[string]any             `json:"spec"`
	Origins     map[string]workflow.Origin `json:"origins"`
	Diagnostics workflow.Diagnostics       `json:"diagnostics"`
}

func printWorkflowShowJSON(w io.Writer, res *workflowsvc.Resolution) error {
	raw, err := yaml.Marshal(res.Spec)
	if err != nil {
		return err
	}
	spec := map[string]any{}
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return err
	}
	out := workflowShowJSON{Ref: res.Ref.String(), Mode: res.Mode, Runtime: res.Runtime, EntryAgent: res.Spec.EntryAgent(),
		Spec: spec, Origins: res.Origins, Diagnostics: res.Diagnostics}
	for _, c := range res.Chain {
		out.Chain = append(out.Chain, c.String())
	}
	if out.Diagnostics == nil {
		out.Diagnostics = workflow.Diagnostics{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func printWorkflowShow(w io.Writer, res *workflowsvc.Resolution, withOrigin bool) {
	sp := res.Spec
	lang := i18n.Locale()
	orig := func(path string) string {
		if !withOrigin || path == "" {
			return ""
		}
		if o, ok := res.Origins.Of(path); ok {
			return "  " + theme.Subtitle.Render("← "+o.String())
		}
		return "  " + theme.Subtitle.Render("← "+i18n.T("cmd.workflow.show.default"))
	}
	line := func(label, value, path string) {
		if value == "" {
			value = "—"
		}
		fmt.Fprintf(w, "  %-16s %s%s\n", label, value, orig(path))
	}
	section := func(key string) { fmt.Fprintf(w, "\n%s\n", theme.Title.Render(i18n.T(key))) }

	title := sp.ID
	if l := sp.Label.Text(lang); l != "" && l != sp.ID {
		title += " · " + l
	}
	fmt.Fprintln(w, theme.Title.Render(title))
	if d := sp.Description.Text(lang); d != "" {
		fmt.Fprintln(w, theme.Subtitle.Render(d))
	}
	chain := make([]string, len(res.Chain))
	for i, c := range res.Chain {
		chain[i] = c.String()
	}
	line(i18n.T("cmd.workflow.show.chain"), strings.Join(chain, " → "), "")
	line(i18n.T("cmd.workflow.show.version"), versionLabel(sp.Version), "version")
	line(i18n.T("cmd.workflow.show.category"), string(sp.Category), "category")
	line(i18n.T("cmd.workflow.show.risk"), string(sp.Risk), "risk")
	isolation := sp.Isolation
	if isolation == "" {
		isolation = workflow.IsolationStandard
	}
	line(i18n.T("cmd.workflow.show.isolation"), string(isolation), "isolation")
	line(i18n.T("cmd.workflow.show.entry"), sp.EntryAgent(), "entry.agent")
	line(i18n.T("cmd.workflow.show.modes"), markDefault(sp.AllowedModes(), sp.DefaultMode()), "modes")
	rts := make([]string, 0, len(sp.AllowedRuntimes()))
	for _, r := range sp.AllowedRuntimes() {
		rts = append(rts, string(r))
	}
	line(i18n.T("cmd.workflow.show.runtimes"), markDefault(rts, string(sp.DefaultRuntime())), "runtime")
	codeMode := "off"
	if sp.CodeMode != nil && *sp.CodeMode {
		codeMode = "on"
	}
	line(i18n.T("cmd.workflow.show.code_mode"), codeMode, "code_mode")

	if sp.Inputs.Len() > 0 {
		section("cmd.workflow.show.inputs")
		for _, k := range sp.Inputs.Keys() {
			in, _ := sp.Inputs.Get(k)
			v := string(in.Type)
			if in.Required {
				v += " *"
			}
			if in.Picker != nil && in.Picker.Multi {
				v += " · multi"
			}
			if in.Default != nil {
				v += " · " + i18n.Tf("cmd.workflow.show.input_default", fmt.Sprint(in.Default))
			}
			if h := in.Help.Text(lang); h != "" {
				v += " — " + h
			}
			line(k, v, "inputs."+k)
		}
	}
	if sp.Agents.Len() > 0 || sp.Entry != nil {
		section("cmd.workflow.show.agents")
		for _, k := range sp.Agents.Keys() {
			a, _ := sp.Agents.Get(k)
			v := string(a.Role)
			if a.Mode != "" {
				v += " · " + string(a.Mode)
			}
			if a.After != "" {
				v += " · " + i18n.Tf("cmd.workflow.show.after", a.After)
			}
			if len(a.Calls) > 0 {
				v += " · → " + strings.Join(a.Calls, ", ")
			}
			line(k, v, "agents."+k)
		}
	}
	if sp.Checkpoints.Len() > 0 {
		section("cmd.workflow.show.checkpoints")
		for _, k := range sp.Checkpoints.Keys() {
			c, _ := sp.Checkpoints.Get(k)
			var parts []string
			if l := c.Label.Text(lang); l != "" {
				parts = append(parts, l)
			}
			var modes []string
			for _, m := range sp.AllowedModes() {
				modes = append(modes, m+": "+string(c.Behavior(m)))
			}
			parts = append(parts, strings.Join(modes, ", "))
			if c.IsMandatory() {
				parts = append(parts, i18n.T("cmd.workflow.show.mandatory"))
			}
			line(k, strings.Join(parts, " · "), "checkpoints."+k)
		}
	}
	section("cmd.workflow.show.resources")
	if sp.Skills != nil {
		line(i18n.T("cmd.workflow.show.skills_extra"), strings.Join(sp.Skills.Extra, ", "), "skills.extra")
		line(i18n.T("cmd.workflow.show.skills_deny"), strings.Join(sp.Skills.Deny, ", "), "skills.deny")
	}
	line("MCP", strings.Join(sp.MCP, ", "), "mcp")
	beads := i18n.T("cmd.workflow.show.beads_all")
	if sp.Beads != nil {
		beads = strings.Join(sp.Beads.Allow, ", ")
	}
	line("Beads", beads, "beads")
	var plugins []string
	for _, p := range sp.Plugins {
		plugins = append(plugins, p.ID)
	}
	line(i18n.T("cmd.workflow.show.plugins"), strings.Join(plugins, ", "), "plugins")
	var outputs []string
	for _, o := range sp.Outputs {
		outputs = append(outputs, o.ID+" ("+string(o.Type)+")")
	}
	line(i18n.T("cmd.workflow.show.outputs"), strings.Join(outputs, ", "), "outputs")
	if sp.Models != nil {
		models := []string{}
		if sp.Models.Default != "" {
			models = append(models, sp.Models.Default)
		}
		keys := make([]string, 0, len(sp.Models.Agents))
		for k := range sp.Models.Agents {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			models = append(models, k+"="+sp.Models.Agents[k])
		}
		line(i18n.T("cmd.workflow.show.models"), strings.Join(models, ", "), "models")
	}
	if len(res.Diagnostics) > 0 {
		fmt.Fprintln(w)
		printDiagnostics(w, res.Diagnostics)
	}
}

func markDefault(values []string, def string) string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = v
		if v == def {
			out[i] = v + " (" + i18n.T("cmd.workflow.show.default") + ")"
		}
	}
	return strings.Join(out, ", ")
}

// printDiagnostics prints workflow findings (same layout as validate).
func printDiagnostics(w io.Writer, ds workflow.Diagnostics) {
	for _, d := range ds {
		icon, style := theme.IconError, theme.ErrorStyle
		if d.Severity == workflow.SeverityWarning {
			icon, style = theme.IconWarning, theme.WarningStyle
		}
		loc := d.Source
		if p := d.Pos.String(); p != "" {
			loc += ":" + p
		}
		if d.Path != "" {
			loc += "  " + d.Path
		}
		fmt.Fprintf(w, "%s %s\n    %s\n", style.Render(icon), loc, d.Message)
		if d.Hint != "" {
			fmt.Fprintf(w, "    %s\n", theme.Subtitle.Render(d.Hint))
		}
	}
}
