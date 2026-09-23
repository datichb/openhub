package sweep

import (
	"fmt"
	"strings"

	"github.com/datichb/openhub/cli/internal/task"
)

// BuildSweepPrompt generates the agent prompt for a sweep sub-task.
// It is intended to be used as the TaskPromptFunc passed to the coordinator.
func BuildSweepPrompt(t task.Task, goal string) string {
	var sb strings.Builder

	sb.WriteString("[SWEEP MODE] Tu fais partie d'un sweep parallèle.\n\n")
	sb.WriteString(fmt.Sprintf("Objectif global : %s\n\n", goal))

	if t.Description != "" {
		sb.WriteString(fmt.Sprintf("Ta sous-tâche : %s\n\n", t.Description))
	}

	if scope, ok := t.Metadata["scope"]; ok && scope != "" {
		sb.WriteString(fmt.Sprintf("Scope (fichiers/packages autorisés) : %s\n\n", scope))
	}

	sb.WriteString("Règles :\n")
	sb.WriteString("1. Ne modifie QUE les fichiers dans ton scope assigné.\n")
	sb.WriteString("2. Minimise les changements sur les fichiers partagés (go.mod, go.sum, package.json).\n")
	sb.WriteString("3. Lance les tests dans ton scope avant de terminer si possible.\n")
	sb.WriteString("4. Utilise des messages de commit au format conventionnel (feat/fix/refactor).\n")
	sb.WriteString("5. Termine par un résumé des fichiers modifiés.\n")

	if strategy, ok := t.Metadata["strategy"]; ok {
		sb.WriteString(fmt.Sprintf("\nStratégie de décomposition : %s\n", strategy))
	}

	return sb.String()
}

// buildPlannerPrompt generates the prompt sent to the LLM for task decomposition.
// The LLM is expected to return a JSON array of task definitions.
func buildPlannerPrompt(goal, projectPath string, maxTasks int, hints string) string {
	var sb strings.Builder

	sb.WriteString("Tu es un planificateur de tâches. Ton rôle est de décomposer un objectif ")
	sb.WriteString("en sous-tâches concrètes et indépendantes qui peuvent être exécutées en parallèle.\n\n")

	sb.WriteString(fmt.Sprintf("Objectif : %s\n", goal))
	sb.WriteString(fmt.Sprintf("Répertoire projet : %s\n", projectPath))
	sb.WriteString(fmt.Sprintf("Nombre maximum de tâches : %d\n", maxTasks))

	if hints != "" {
		sb.WriteString(fmt.Sprintf("\nIndications supplémentaires : %s\n", hints))
	}

	sb.WriteString("\nRéponds UNIQUEMENT avec un objet JSON valide au format suivant :\n")
	sb.WriteString("```json\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"tasks\": [\n")
	sb.WriteString("    {\n")
	sb.WriteString("      \"id\": \"sweep-nom-court\",\n")
	sb.WriteString("      \"label\": \"Description courte pour le TUI\",\n")
	sb.WriteString("      \"scope\": \"chemin/du/package/ou/fichiers\",\n")
	sb.WriteString("      \"description\": \"Description complète de ce qu'il faut faire\"\n")
	sb.WriteString("    }\n")
	sb.WriteString("  ]\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n\n")

	sb.WriteString("Contraintes :\n")
	sb.WriteString("- Chaque tâche doit être indépendante (exécutable en parallèle).\n")
	sb.WriteString("- Les IDs doivent être uniques, en kebab-case, préfixés par \"sweep-\".\n")
	sb.WriteString("- Le scope doit correspondre à des chemins réels du projet.\n")
	sb.WriteString("- Minimise le chevauchement de scope entre tâches.\n")

	return sb.String()
}
