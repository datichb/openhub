package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/modelcheck"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

func init() {
	configCmd.AddCommand(configModelCmd())
}

func configModelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "model",
		Short: "Gestion de la cascade de modèles par agent",
		Long: `Configure les modèles IA par agent, par famille ou globalement.

La cascade de résolution (priorité décroissante) :
  1. Project agent override    (oh config model agent <id> <model> --project <p>)
  2. Project family override   (oh config model family <name> <model> --project <p>)
  3. Project global model      (oh config model default <model> --project <p>)
  4. Hub agent override        (oh config model agent <id> <model>)
  5. Hub family override       (oh config model family <name> <model>)
  6. Hub global model          (oh config model default <model>)
  7. Agent frontmatter floor   (model: dans le .md de l'agent)

Le modèle résolu est normalisé vers le provider du projet lors du deploy.`,
	}

	cmd.AddCommand(configModelDefaultCmd())
	cmd.AddCommand(configModelFamilyCmd())
	cmd.AddCommand(configModelAgentCmd())
	cmd.AddCommand(configModelShowCmd())
	cmd.AddCommand(configModelUnsetCmd())

	return cmd
}

func configModelDefaultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "default <model>",
		Short: "Définit le modèle global par défaut",
		Long:  "Sans --project: écrit dans hub.toml [models.default]. Avec --project: écrit dans la DB projet.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			model := args[0]
			projectID, _ := cmd.Flags().GetString("project")
			if err := modelcheck.CheckModel(model); err != nil {
				return err
			}

			if projectID != "" {
				return setProjectModel(cmd.Context(), projectID, model)
			}
			return setHubModelDefault(model)
		},
	}
	cmd.Flags().StringP("project", "p", "", "Nom du projet (hub-level si absent)")
	_ = cmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
	return cmd
}

func configModelFamilyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "family <name> <model>",
		Short: "Définit le modèle pour une famille d'agents",
		Long:  "Familles: les dossiers d'agents du catalogue (planning, developer, quality, auditor, design, documentation, infra, delivery, utility, équipe…).",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			family, model := args[0], args[1]
			projectID, _ := cmd.Flags().GetString("project")
			if err := checkModelTarget(cmd, func(c modelcheck.Catalog) error { return c.CheckFamily(family) }, family, model); err != nil {
				return err
			}

			if projectID != "" {
				return setProjectModelFamily(cmd.Context(), projectID, family, model)
			}
			return setHubModelFamily(family, model)
		},
	}
	cmd.Flags().StringP("project", "p", "", "Nom du projet (hub-level si absent)")
	_ = cmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
	return cmd
}

func configModelAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent <id> <model>",
		Short: "Définit le modèle pour un agent spécifique",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentID, model := args[0], args[1]
			projectID, _ := cmd.Flags().GetString("project")
			if err := checkModelTarget(cmd, func(c modelcheck.Catalog) error { return c.CheckAgent(agentID) }, agentID, model); err != nil {
				return err
			}

			if projectID != "" {
				return setProjectModelAgent(cmd.Context(), projectID, agentID, model)
			}
			return setHubModelAgent(agentID, model)
		},
	}
	cmd.Flags().StringP("project", "p", "", "Nom du projet (hub-level si absent)")
	_ = cmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
	return cmd
}

// checkModelTarget validates the agent or family (check) and the model of a
// cascade override. An unreadable agent catalogue only warns: the identifier
// cannot be verified, and refusing every override would be worse.
func checkModelTarget(cmd *cobra.Command, check func(modelcheck.Catalog) error, name, model string) error {
	cat := loadModelCatalog(cmd)
	if cat.Empty() {
		fmt.Fprintln(cmd.ErrOrStderr(), theme.WarningStyle.Render(theme.IconWarning)+" "+i18n.Tf("modelcheck.catalog_unavailable", name))
	} else if err := check(cat); err != nil {
		return err
	}
	return modelcheck.CheckModel(model)
}

// loadModelCatalog reads the effective agent catalogue (hub, team of the
// project, project) of the context of cmd; empty when it cannot be read.
func loadModelCatalog(cmd *cobra.Command) modelcheck.Catalog {
	c, err := workflowReadContext(cmd)
	if err != nil {
		c = workflowsvc.Context{} // an unknown project is reported by the writer
	}
	return catalogFor(ctxOf(cmd), c)
}

// newModelsView wires the Models view with the agent catalogue of the active
// project (hub, team, project) to check the identifiers it writes.
func newModelsView(a *app.App) *views.ModelsView {
	v := views.NewModelsView(a)
	v.SetCatalog(func(ctx context.Context) modelcheck.Catalog {
		c := workflowsvc.Context{}
		if p, err := resolveActiveProject(a); err == nil && p != nil {
			c.ProjectID = p.ID
		}
		return catalogFor(ctx, c)
	})
	return v
}

func catalogFor(ctx context.Context, c workflowsvc.Context) modelcheck.Catalog {
	b, err := newWorkflowService(ctx).Bricks(ctx, c)
	if err != nil || b.Dir == "" {
		return modelcheck.Catalog{}
	}
	files, err := bricks.FindAgentFiles(b.Dir)
	if err != nil {
		return modelcheck.Catalog{}
	}
	agents := make(map[string]string, len(files))
	for id, path := range files {
		rel, _ := filepath.Rel(filepath.Join(b.Dir, "agents"), path)
		agents[id] = bricks.AgentFamily(filepath.ToSlash(rel))
	}
	return modelcheck.NewCatalog(agents)
}

func configModelShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Affiche la configuration des modèles",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectFlag, _ := cmd.Flags().GetString("project")
			workflowID, _ := cmd.Flags().GetString("workflow")
			jsonOut, _ := cmd.Flags().GetBool("json")
			ctx, a := cmd.Context(), MustApp()

			project, err := budgetProject(ctx, a, projectFlag)
			if err != nil {
				return err
			}
			levels, err := modelCascadeLevels(ctx, a, project, workflowID)
			if err != nil {
				return err
			}
			if jsonOut {
				out := map[string]any{}
				for _, l := range levels {
					out[l.ID] = l.JSON()
				}
				return json.NewEncoder(a.IO.Out).Encode(out)
			}
			printModelCascade(a.IO.Out, levels)
			return nil
		},
	}
	cmd.Flags().StringP("project", "p", "", "Projet (ID ou nom ; défaut : projet du dossier courant)")
	cmd.Flags().StringP("workflow", "w", "", "Inclure le niveau d'un workflow (id ou <couche>:<id>)")
	cmd.Flags().Bool("json", false, "Sortie JSON")
	_ = cmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
	return cmd
}

// modelLevel is one level of the model cascade shown by `oh config model show`.
type modelLevel struct {
	ID     string // workflow | project | hub | team
	Title  string
	Models *bricks.ModelOverrides // nil: nothing set at this level
}

func (l modelLevel) JSON() map[string]any {
	out := map[string]any{"default": "", "families": map[string]string{}, "agents": map[string]string{}}
	if l.Models != nil {
		out["default"] = l.Models.Default
		if l.Models.Families != nil {
			out["families"] = l.Models.Families
		}
		if l.Models.Agents != nil {
			out["agents"] = l.Models.Agents
		}
	}
	return out
}

// modelCascadeLevels returns the levels of the model cascade in priority
// order (O9): workflow > project > hub > team (A11). The frontmatter of the
// agents is the last level.
func modelCascadeLevels(ctx context.Context, a *app.App, project *domain.Project, workflowID string) ([]modelLevel, error) {
	var levels []modelLevel
	if workflowID != "" {
		c := workflowsvc.Context{}
		if project != nil {
			c.ProjectID = project.ID
		}
		res, err := newWorkflowService(ctx).Resolve(ctx, c, workflowID, workflowsvc.ResolveOpts{})
		if res == nil {
			if errors.Is(err, workflowsvc.ErrUnknownWorkflow) {
				return nil, errors.New(i18n.Tf("cmd.workflow.show.unknown", workflowID))
			}
			return nil, err
		}
		levels = append(levels, modelLevel{ID: "workflow", Title: i18n.Tf("cmd.config.model.show.level_workflow", res.Spec.ID),
			Models: bundle.WorkflowModels(res.Spec.Models)})
	}
	hub, proj := modelOverridesFor(a, project)
	if project != nil {
		levels = append(levels, modelLevel{ID: "project", Title: i18n.Tf("cmd.config.model.show.level_project", project.Name), Models: proj})
	}
	levels = append(levels, modelLevel{ID: "hub", Title: i18n.T("cmd.config.model.show.level_hub"), Models: hub})
	if project != nil {
		if tc := config.ResolveTeamForProject(a.Config, project); tc.Enabled {
			levels = append(levels, modelLevel{ID: "team", Title: i18n.Tf("cmd.config.model.show.level_team", tc.TeamID),
				Models: teamModelOverrides(tc)})
		}
	}
	return levels, nil
}

func printModelCascade(w io.Writer, levels []modelLevel) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s\n", theme.Title.Render("  "+i18n.T("cmd.config.model.show.title")+"  "))
	fmt.Fprintln(w)
	notSet := theme.Subtitle.Render(i18n.T("cmd.config.model.show.not_set"))
	printMap := func(label string, m map[string]string) {
		if len(m) == 0 {
			return
		}
		fmt.Fprintf(w, "  %s\n", label)
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(w, "    %s: %s\n", k, m[k])
		}
	}
	for i, l := range levels {
		fmt.Fprintf(w, "%s\n", theme.Bold.Render(fmt.Sprintf("%d. %s", i+1, l.Title)))
		def := notSet
		if l.Models != nil && l.Models.Default != "" {
			def = l.Models.Default
		}
		fmt.Fprintf(w, "  %s %s\n", i18n.T("cmd.config.model.show.default"), def)
		if l.Models != nil {
			printMap(i18n.T("cmd.config.model.show.families"), l.Models.Families)
			printMap(i18n.T("cmd.config.model.show.agents"), l.Models.Agents)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "%s\n", theme.Bold.Render(fmt.Sprintf("%d. %s", len(levels)+1, i18n.T("cmd.config.model.show.level_frontmatter"))))
	fmt.Fprintf(w, "  %s\n\n", theme.Subtitle.Render(i18n.T("cmd.config.model.show.order")))
}

func configModelUnsetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unset [default|family <name>|agent <id>]",
		Short: "Supprime un override de modèle",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, _ := cmd.Flags().GetString("project")
			scope := args[0]

			switch scope {
			case "default":
				if projectID != "" {
					return unsetProjectModelDefault(cmd.Context(), projectID)
				}
				return unsetHubModel("models.default")
			case "family":
				if len(args) < 2 {
					return errors.New(i18n.T("cmd.config.model.unset.usage_family"))
				}
				key := "models.families." + args[1]
				if projectID != "" {
					return unsetProjectModelFamily(cmd.Context(), projectID, args[1])
				}
				return unsetHubModel(key)
			case "agent":
				if len(args) < 2 {
					return errors.New(i18n.T("cmd.config.model.unset.usage_agent"))
				}
				key := "models.agents." + args[1]
				if projectID != "" {
					return unsetProjectModelAgent(cmd.Context(), projectID, args[1])
				}
				return unsetHubModel(key)
			default:
				return errors.New(i18n.Tf("cmd.config.model.unset.bad_scope", scope))
			}
		},
	}
	cmd.Flags().StringP("project", "p", "", "Nom du projet")
	_ = cmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
	return cmd
}

// --- Hub-level writers (hub.toml) ---

func setHubModelDefault(model string) error {
	if err := config.Update(func(c *config.Config) error {
		c.Models.Default = model
		return nil
	}); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "%s %s = %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		theme.Bold.Render("models.default"), model)
	return nil
}

func setHubModelFamily(family, model string) error {
	if err := config.Update(func(c *config.Config) error {
		if c.Models.Families == nil {
			c.Models.Families = make(map[string]string)
		}
		c.Models.Families[family] = model
		return nil
	}); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "%s %s = %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		theme.Bold.Render("models.families."+family), model)
	return nil
}

func setHubModelAgent(agentID, model string) error {
	if err := config.Update(func(c *config.Config) error {
		if c.Models.Agents == nil {
			c.Models.Agents = make(map[string]string)
		}
		c.Models.Agents[agentID] = model
		return nil
	}); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "%s %s = %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		theme.Bold.Render("models.agents."+agentID), model)
	return nil
}

func unsetHubModel(key string) error {
	if err := config.Update(func(c *config.Config) error {
		if key == "models.default" {
			c.Models.Default = ""
		} else if family, ok := strings.CutPrefix(key, "models.families."); ok {
			delete(c.Models.Families, family)
		} else if agent, ok := strings.CutPrefix(key, "models.agents."); ok {
			delete(c.Models.Agents, agent)
		}
		return nil
	}); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "%s %s %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		i18n.T("cmd.config.unset_success"),
		theme.Bold.Render(key))
	return nil
}

// --- Project-level writers (SQLite DB) ---

func setProjectModel(ctx context.Context, projectID, model string) error {
	a := MustApp()
	project, err := a.Projects.Get(ctx, projectID)
	if err != nil {
		return projectLoadError(projectID, err)
	}

	project.Model = model
	if err := a.Projects.Update(ctx, project); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "%s project %s model.default = %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		theme.Bold.Render(project.Name), model)
	return nil
}

func setProjectModelFamily(ctx context.Context, projectID, family, model string) error {
	a := MustApp()
	project, err := a.Projects.Get(ctx, projectID)
	if err != nil {
		return projectLoadError(projectID, err)
	}

	if project.ModelOverrides == nil {
		project.ModelOverrides = &domain.ProjectModelOverrides{}
	}
	if project.ModelOverrides.Families == nil {
		project.ModelOverrides.Families = make(map[string]string)
	}
	project.ModelOverrides.Families[family] = model

	if err := a.Projects.Update(ctx, project); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "%s project %s model.families.%s = %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		theme.Bold.Render(project.Name), family, model)
	return nil
}

func setProjectModelAgent(ctx context.Context, projectID, agentID, model string) error {
	a := MustApp()
	project, err := a.Projects.Get(ctx, projectID)
	if err != nil {
		return projectLoadError(projectID, err)
	}

	if project.ModelOverrides == nil {
		project.ModelOverrides = &domain.ProjectModelOverrides{}
	}
	if project.ModelOverrides.Agents == nil {
		project.ModelOverrides.Agents = make(map[string]string)
	}
	project.ModelOverrides.Agents[agentID] = model

	if err := a.Projects.Update(ctx, project); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "%s project %s model.agents.%s = %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		theme.Bold.Render(project.Name), agentID, model)
	return nil
}

func unsetProjectModelDefault(ctx context.Context, projectID string) error {
	a := MustApp()
	project, err := a.Projects.Get(ctx, projectID)
	if err != nil {
		return projectLoadError(projectID, err)
	}

	project.Model = ""
	if err := a.Projects.Update(ctx, project); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "%s project %s model.default unset\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		theme.Bold.Render(project.Name))
	return nil
}

func unsetProjectModelFamily(ctx context.Context, projectID, family string) error {
	a := MustApp()
	project, err := a.Projects.Get(ctx, projectID)
	if err != nil {
		return projectLoadError(projectID, err)
	}

	if project.ModelOverrides != nil && project.ModelOverrides.Families != nil {
		delete(project.ModelOverrides.Families, family)
	}
	if err := a.Projects.Update(ctx, project); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "%s project %s model.families.%s unset\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		theme.Bold.Render(project.Name), family)
	return nil
}

func unsetProjectModelAgent(ctx context.Context, projectID, agentID string) error {
	a := MustApp()
	project, err := a.Projects.Get(ctx, projectID)
	if err != nil {
		return projectLoadError(projectID, err)
	}

	if project.ModelOverrides != nil && project.ModelOverrides.Agents != nil {
		delete(project.ModelOverrides.Agents, agentID)
	}
	if err := a.Projects.Update(ctx, project); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "%s project %s model.agents.%s unset\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		theme.Bold.Render(project.Name), agentID)
	return nil
}

// projectLoadError explains a project that cannot be read (unknown: the
// message of oh project list).
func projectLoadError(projectID string, err error) error {
	if errors.Is(err, domain.ErrNotFound) {
		return errors.New(i18n.Tf("cmd.config.model.unknown_project", projectID))
	}
	return fmt.Errorf("%s: %w", i18n.Tf("cmd.config.model.project_failed", projectID), err)
}
