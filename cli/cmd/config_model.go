package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/common"
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

			if projectID != "" {
				return setProjectModel(cmd.Context(), projectID, model)
			}
			return setHubModelDefault(model)
		},
	}
	cmd.Flags().StringP("project", "j", "", "Nom du projet (hub-level si absent)")
	_ = cmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
	return cmd
}

func configModelFamilyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "family <name> <model>",
		Short: "Définit le modèle pour une famille d'agents",
		Long:  "Familles: planning, developer, quality, auditor, design, documentation.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			family, model := args[0], args[1]
			projectID, _ := cmd.Flags().GetString("project")

			if projectID != "" {
				return setProjectModelFamily(cmd.Context(), projectID, family, model)
			}
			return setHubModelFamily(family, model)
		},
	}
	cmd.Flags().StringP("project", "j", "", "Nom du projet (hub-level si absent)")
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

			if projectID != "" {
				return setProjectModelAgent(cmd.Context(), projectID, agentID, model)
			}
			return setHubModelAgent(agentID, model)
		},
	}
	cmd.Flags().StringP("project", "j", "", "Nom du projet (hub-level si absent)")
	_ = cmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
	return cmd
}

func configModelShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Affiche la configuration des modèles",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, _ := cmd.Flags().GetString("project")
			jsonOut, _ := cmd.Flags().GetBool("json")

			v := configViper()
			a := MustApp()

			hubDefault := v.GetString("models.default")
			hubFamilies := v.GetStringMapString("models.families")
			hubAgents := v.GetStringMapString("models.agents")

			// JSON mode: output structured data and exit early
			if jsonOut {
				output := map[string]interface{}{
					"hub": map[string]interface{}{
						"default":  hubDefault,
						"families": hubFamilies,
						"agents":   hubAgents,
					},
				}
				if projectID != "" {
					ctx := cmd.Context()
					project, err := a.Projects.Get(ctx, projectID)
					if err != nil {
						return fmt.Errorf("project %s: %w", projectID, err)
					}
					projectOut := map[string]interface{}{
						"default": project.Model,
					}
					if project.ModelOverrides != nil {
						projectOut["families"] = project.ModelOverrides.Families
						projectOut["agents"] = project.ModelOverrides.Agents
					}
					output["project"] = projectOut
				}
				return json.NewEncoder(a.IO.Out).Encode(output)
			}

			// Human-readable output
			fmt.Fprintln(a.IO.Out)
			fmt.Fprintf(a.IO.Out, "%s\n", common.Title.Render("  Model Configuration  "))
			fmt.Fprintln(a.IO.Out)

			// Hub-level
			fmt.Fprintf(a.IO.Out, "%s\n", common.Bold.Render("Hub-level (hub.toml):"))
			if hubDefault != "" {
				fmt.Fprintf(a.IO.Out, "  default: %s\n", hubDefault)
			} else {
				fmt.Fprintf(a.IO.Out, "  default: %s\n", common.Subtitle.Render("(not set)"))
			}

			if len(hubFamilies) > 0 {
				fmt.Fprintf(a.IO.Out, "  families:\n")
				for f, m := range hubFamilies {
					fmt.Fprintf(a.IO.Out, "    %s: %s\n", f, m)
				}
			}

			if len(hubAgents) > 0 {
				fmt.Fprintf(a.IO.Out, "  agents:\n")
				for id, m := range hubAgents {
					fmt.Fprintf(a.IO.Out, "    %s: %s\n", id, m)
				}
			}

			// Project-level (if requested)
			if projectID != "" {
				ctx := cmd.Context()
				project, err := a.Projects.Get(ctx, projectID)
				if err != nil {
					return fmt.Errorf("project %s: %w", projectID, err)
				}

				fmt.Fprintln(a.IO.Out)
				fmt.Fprintf(a.IO.Out, "%s\n", common.Bold.Render(fmt.Sprintf("Project-level (%s):", project.Name)))
				if project.Model != "" {
					fmt.Fprintf(a.IO.Out, "  default: %s\n", project.Model)
				} else {
					fmt.Fprintf(a.IO.Out, "  default: %s\n", common.Subtitle.Render("(not set)"))
				}

				if project.ModelOverrides != nil {
					if len(project.ModelOverrides.Families) > 0 {
						fmt.Fprintf(a.IO.Out, "  families:\n")
						for f, m := range project.ModelOverrides.Families {
							fmt.Fprintf(a.IO.Out, "    %s: %s\n", f, m)
						}
					}
					if len(project.ModelOverrides.Agents) > 0 {
						fmt.Fprintf(a.IO.Out, "  agents:\n")
						for id, m := range project.ModelOverrides.Agents {
							fmt.Fprintf(a.IO.Out, "    %s: %s\n", id, m)
						}
					}
				}
			}

			fmt.Fprintln(a.IO.Out)
			return nil
		},
	}
	cmd.Flags().StringP("project", "j", "", "Inclure la configuration projet")
	cmd.Flags().Bool("json", false, "Sortie JSON")
	_ = cmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
	return cmd
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
					return fmt.Errorf("usage: oh config model unset family <name>")
				}
				key := "models.families." + args[1]
				if projectID != "" {
					return unsetProjectModelFamily(cmd.Context(), projectID, args[1])
				}
				return unsetHubModel(key)
			case "agent":
				if len(args) < 2 {
					return fmt.Errorf("usage: oh config model unset agent <id>")
				}
				key := "models.agents." + args[1]
				if projectID != "" {
					return unsetProjectModelAgent(cmd.Context(), projectID, args[1])
				}
				return unsetHubModel(key)
			default:
				return fmt.Errorf("unknown scope %q — expected: default, family, agent", scope)
			}
		},
	}
	cmd.Flags().StringP("project", "j", "", "Nom du projet")
	_ = cmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
	return cmd
}

// --- Hub-level writers (hub.toml) ---

func setHubModelDefault(model string) error {
	v := configViper()
	v.Set("models.default", model)
	return writeHubConfig(v, "models.default", model)
}

func setHubModelFamily(family, model string) error {
	v := configViper()
	v.Set("models.families."+family, model)
	return writeHubConfig(v, "models.families."+family, model)
}

func setHubModelAgent(agentID, model string) error {
	v := configViper()
	v.Set("models.agents."+agentID, model)
	return writeHubConfig(v, "models.agents."+agentID, model)
}

func unsetHubModel(key string) error {
	v := configViper()
	v.Set(key, nil)

	cfgPath := config.ConfigPath()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	if err := v.WriteConfigAs(cfgPath); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Fprintf(os.Stdout, "%s %s %s\n",
		common.SuccessStyle.Render(common.IconSuccess),
		i18n.T("cmd.config.unset_success"),
		common.Bold.Render(key))
	return nil
}

func writeHubConfig(v *viper.Viper, key, value string) error {
	cfgPath := config.ConfigPath()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	if err := v.WriteConfigAs(cfgPath); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Fprintf(os.Stdout, "%s %s = %s\n",
		common.SuccessStyle.Render(common.IconSuccess),
		common.Bold.Render(key), value)
	return nil
}

// --- Project-level writers (SQLite DB) ---

func setProjectModel(ctx context.Context, projectID, model string) error {
	a := MustApp()
	project, err := a.Projects.Get(ctx, projectID)
	if err != nil {
		return fmt.Errorf("project %s: %w", projectID, err)
	}

	project.Model = model
	if err := a.Projects.Update(ctx, project); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "%s project %s model.default = %s\n",
		common.SuccessStyle.Render(common.IconSuccess),
		common.Bold.Render(project.Name), model)
	return nil
}

func setProjectModelFamily(ctx context.Context, projectID, family, model string) error {
	a := MustApp()
	project, err := a.Projects.Get(ctx, projectID)
	if err != nil {
		return fmt.Errorf("project %s: %w", projectID, err)
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
		common.SuccessStyle.Render(common.IconSuccess),
		common.Bold.Render(project.Name), family, model)
	return nil
}

func setProjectModelAgent(ctx context.Context, projectID, agentID, model string) error {
	a := MustApp()
	project, err := a.Projects.Get(ctx, projectID)
	if err != nil {
		return fmt.Errorf("project %s: %w", projectID, err)
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
		common.SuccessStyle.Render(common.IconSuccess),
		common.Bold.Render(project.Name), agentID, model)
	return nil
}

func unsetProjectModelDefault(ctx context.Context, projectID string) error {
	a := MustApp()
	project, err := a.Projects.Get(ctx, projectID)
	if err != nil {
		return fmt.Errorf("project %s: %w", projectID, err)
	}

	project.Model = ""
	if err := a.Projects.Update(ctx, project); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "%s project %s model.default unset\n",
		common.SuccessStyle.Render(common.IconSuccess),
		common.Bold.Render(project.Name))
	return nil
}

func unsetProjectModelFamily(ctx context.Context, projectID, family string) error {
	a := MustApp()
	project, err := a.Projects.Get(ctx, projectID)
	if err != nil {
		return fmt.Errorf("project %s: %w", projectID, err)
	}

	if project.ModelOverrides != nil && project.ModelOverrides.Families != nil {
		delete(project.ModelOverrides.Families, family)
	}
	if err := a.Projects.Update(ctx, project); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "%s project %s model.families.%s unset\n",
		common.SuccessStyle.Render(common.IconSuccess),
		common.Bold.Render(project.Name), family)
	return nil
}

func unsetProjectModelAgent(ctx context.Context, projectID, agentID string) error {
	a := MustApp()
	project, err := a.Projects.Get(ctx, projectID)
	if err != nil {
		return fmt.Errorf("project %s: %w", projectID, err)
	}

	if project.ModelOverrides != nil && project.ModelOverrides.Agents != nil {
		delete(project.ModelOverrides.Agents, agentID)
	}
	if err := a.Projects.Update(ctx, project); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "%s project %s model.agents.%s unset\n",
		common.SuccessStyle.Render(common.IconSuccess),
		common.Bold.Render(project.Name), agentID)
	return nil
}
