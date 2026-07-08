package prompt

import (
	"fmt"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
)

// BuildOnboardPrompt constructs the bootstrap prompt for the onboarder agent.
// It provides project identity and instructions for wiki creation/enrichment.
func BuildOnboardPrompt(project *domain.Project, hubDir string, refresh bool) string {
	var sb strings.Builder

	sb.WriteString("Lance l'onboarding de ce projet.\n\n")

	// Project identity
	fmt.Fprintf(&sb, "Projet : %s\n", project.ID)
	fmt.Fprintf(&sb, "Nom : %s\n", project.Name)
	fmt.Fprintf(&sb, "Chemin : %s\n", project.Path)
	if hubDir != "" {
		fmt.Fprintf(&sb, "Hub : %s\n", hubDir)
	}
	if project.Language != "" {
		fmt.Fprintf(&sb, "Langage : %s\n", project.Language)
	}
	sb.WriteString("\n")

	// Wiki instructions
	if refresh {
		sb.WriteString("Mode REFRESH : re-découvre le projet et enrichis le wiki existant.\n")
		sb.WriteString("Le wiki se trouve dans docs/wiki/.\n\n")
		sb.WriteString("Règles :\n")
		sb.WriteString("- Ne supprime aucune page existante\n")
		sb.WriteString("- Mets à jour les informations obsolètes\n")
		sb.WriteString("- Ajoute les nouvelles découvertes (nouveaux modules, conventions, dépendances)\n")
		sb.WriteString("- Corrige les inexactitudes\n")
		sb.WriteString("- Enrichis les god nodes (pages structurantes)\n")
		sb.WriteString("- Mets à jour les tags de confiance si nécessaire\n")
	} else {
		sb.WriteString("Crée le wiki documentaire vivant du projet dans docs/wiki/.\n\n")
		sb.WriteString("Le wiki doit couvrir :\n")
		sb.WriteString("- Architecture globale (modules, couches, patterns)\n")
		sb.WriteString("- Stack technique (langages, frameworks, dépendances clés)\n")
		sb.WriteString("- Conventions de code (nommage, structure fichiers, patterns récurrents)\n")
		sb.WriteString("- Points d'entrée (comment build, test, run)\n")
		sb.WriteString("- Décisions d'architecture (ADR si présents)\n")
		sb.WriteString("- Dépendances externes et intégrations\n")
		sb.WriteString("\n")
		sb.WriteString("Format : utilise le protocole doc-wiki-protocol (frontmatter YAML, tags de confiance, structure par heading).\n")
		sb.WriteString("Emplacement : docs/wiki/ (un fichier .md par sujet majeur + un index.md).\n")
	}

	return sb.String()
}

// WikiExists checks if a docs/wiki/ directory already exists in the project.
func WikiExists(projectPath string) bool {
	return dirExists(projectPath, "docs/wiki")
}

// WikiPath returns the path to the wiki directory.
func WikiPath(projectPath string) string {
	return strings.TrimRight(projectPath, "/") + "/docs/wiki"
}
