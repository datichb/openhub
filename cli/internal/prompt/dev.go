package prompt

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/datichb/openhub/cli/internal/beads"
)

// BuildDevPrompt constructs the bootstrap prompt for orchestrator-dev.
// It includes the ticket list and mandatory workflow instructions.
func BuildDevPrompt(tickets []beads.Ticket) string {
	var sb strings.Builder

	if len(tickets) == 1 {
		sb.WriteString("Voici le ticket à implémenter :\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("Voici les %d tickets à implémenter :\n\n", len(tickets)))
	}

	// JSON-encoded ticket list for structured parsing by the agent
	ticketJSON, _ := json.MarshalIndent(tickets, "", "  ")
	sb.Write(ticketJSON)
	sb.WriteString("\n\n")

	// Mandatory workflow
	sb.WriteString("Workflow obligatoire pour chaque ticket :\n")
	sb.WriteString("1. bd show <ID>                  — lire le détail complet avant tout\n")
	sb.WriteString("2. bd update <ID> --claim        — clamer le ticket (atomique)\n")
	sb.WriteString("3. Implémenter + tester\n")
	sb.WriteString("4. bd update <ID> -s review      — passer en review\n")
	sb.WriteString("5. bd close <ID> --suggest-next  — clore après validation et passer au suivant\n")
	sb.WriteString("\n")

	if len(tickets) > 1 {
		sb.WriteString("Stratégie : traite les tickets par ordre de priorité (P0 > P1 > P2 > P3).\n")
		sb.WriteString("Si des tickets sont indépendants, tu peux les paralléliser via des sous-agents.\n")
	} else {
		sb.WriteString("Commence par lire le détail du ticket avec bd show.\n")
	}

	return sb.String()
}
