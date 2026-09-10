package teamstate

import (
	"fmt"
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// RenderTemplateBrief generates a human-readable Markdown summary from raw brief data.
// This is generated WITHOUT any LLM call — pure Go template.
func RenderTemplateBrief(brief *TakeoverBrief) string {
	var b strings.Builder

	// Header
	fmt.Fprintf(&b, "# Takeover Brief: %s\n\n", brief.Meta.TicketID)
	fmt.Fprintf(&b, "%s",
		i18n.Tf("takeover.template.transferred_from",
			brief.Meta.TransferredFrom,
			brief.Meta.TransferredTo,
			brief.Meta.TransferDate.Format("2006-01-02"),
			brief.Meta.Reason))

	if brief.Meta.StaleDays > 0 {
		fmt.Fprintf(&b, "> Ticket inactif depuis %d jours\n\n", brief.Meta.StaleDays)
	}

	// Activity
	b.WriteString(i18n.T("takeover.template.activity_header"))
	if brief.Activity.SessionsCount > 0 {
		fmt.Fprintf(&b, "- %d session(s)", brief.Activity.SessionsCount)
		if !brief.Activity.FirstSession.IsZero() && !brief.Activity.LastSession.IsZero() {
			fmt.Fprintf(&b, " (%s → %s)",
				brief.Activity.FirstSession.Format("02 Jan"),
				brief.Activity.LastSession.Format("02 Jan"))
		}
		b.WriteString("\n")
		if brief.Activity.TotalDurationMinutes > 0 {
			hours := brief.Activity.TotalDurationMinutes / 60
			mins := brief.Activity.TotalDurationMinutes % 60
			if hours > 0 {
				fmt.Fprintf(&b, "- Duree totale : ~%dh%02dm\n", hours, mins)
			} else {
				fmt.Fprintf(&b, "- Duree totale : ~%dm\n", mins)
			}
		}
	} else {
		b.WriteString("- Aucune session enregistree\n")
	}

	// Git
	if brief.Git.Branch != "" || brief.Git.CommitsCount > 0 {
		fmt.Fprintf(&b, "- %d commit(s)", brief.Git.CommitsCount)
		if brief.Git.Branch != "" {
			fmt.Fprintf(&b, " sur branche `%s`", brief.Git.Branch)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Files
	if len(brief.Git.FilesModified) > 0 || len(brief.Git.FilesCreated) > 0 {
		b.WriteString("## Fichiers principaux\n\n")
		b.WriteString("| Fichier | Action | Lignes |\n")
		b.WriteString("|---------|--------|--------|\n")

		for _, f := range brief.Git.FilesModified {
			fmt.Fprintf(&b, "| `%s` | modifie | +%d/-%d |\n",
				f.Path, f.Additions, f.Deletions)
		}
		for _, f := range brief.Git.FilesCreated {
			fmt.Fprintf(&b, "| `%s` | cree | +%d |\n",
				f.Path, f.Additions)
		}
		b.WriteString("\n")
	}

	// Last state
	if brief.Git.LastCommitMessage != "" {
		b.WriteString("## Dernier etat connu\n\n")
		fmt.Fprintf(&b, "- Dernier commit : \"%s\"", brief.Git.LastCommitMessage)
		if !brief.Git.LastCommitDate.IsZero() {
			fmt.Fprintf(&b, " (%s)", brief.Git.LastCommitDate.Format("02 Jan 15:04"))
		}
		b.WriteString("\n")
		if len(brief.Events) > 0 {
			fmt.Fprintf(&b, "- Derniere session : \"%s\"\n", brief.Events[0].Summary)
		}
		b.WriteString("\n")
	}

	// Event history
	if len(brief.Events) > 0 {
		b.WriteString("## Historique sessions\n\n")
		// Events are newest-first, reverse for chronological display
		for i := len(brief.Events) - 1; i >= 0; i-- {
			e := brief.Events[i]
			fmt.Fprintf(&b, "%d. **%s** — %s\n",
				len(brief.Events)-i,
				e.Timestamp.Format("02 Jan"),
				e.Summary)
		}
		b.WriteString("\n")
	}

	// Workflow context
	if brief.Workflow.ActiveMode != "" || brief.Workflow.CustomOverrides {
		b.WriteString("## Workflow\n\n")
		if brief.Workflow.ActiveMode != "" {
			b.WriteString(fmt.Sprintf("- **Mode actif** : %s\n", brief.Workflow.ActiveMode))
		}
		if brief.Workflow.CustomOverrides {
			b.WriteString("- **Workflow personnalise** : le projet utilise des overrides workflow\n")
		}
		b.WriteString("\n")
	}

	// Footer
	b.WriteString("---\n")
	b.WriteString("*Brief genere automatiquement par template. Utiliser `oh takeover-brief enrich ")
	b.WriteString(brief.Meta.TicketID)
	b.WriteString("` pour une version enrichie par IA.*\n")

	return b.String()
}
