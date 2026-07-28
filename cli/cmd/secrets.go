package cmd

import (
	"context"
	"fmt"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/storage/keychain"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ─────────────────────────────────────────────────────────────────────────────
// Command tree
// ─────────────────────────────────────────────────────────────────────────────

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Gérer les secrets (tokens, clés API)",
	Long: `Gérer les secrets stockés dans le keychain système (ou fichier chiffré en fallback).

Les secrets sont référencés par nom dans hub.toml (token_key) et résolus au runtime.
Ils peuvent être stockés en portée globale (partagés entre projets) ou par projet.

Résolution cascade lors de l'utilisation :
  1. Secret de portée projet (si dans un répertoire projet)
  2. Secret de portée globale
  3. Variable d'environnement correspondante`,
}

// ─────────────────────────────────────────────────────────────────────────────
// oh secrets set <key>
// ─────────────────────────────────────────────────────────────────────────────

var secretsSetCmd = &cobra.Command{
	Use:   "set <key>",
	Short: "Stocker un secret dans le keychain",
	Long: `Stocke une valeur secrète dans le keychain système sous le nom <key>.

La portée est déterminée automatiquement :
  - Si dans un répertoire projet enregistré → portée projet (prioritaire)
  - Sinon → portée globale

Utilisez --global pour forcer la portée globale.
Utilisez --project pour cibler un projet spécifique.`,
	Args: cobra.ExactArgs(1),
	RunE: runSecretsSet,
}

var secretsSetGlobal  bool
var secretsSetProject string

func init() {
	secretsSetCmd.Flags().BoolVar(&secretsSetGlobal, "global", false, "Forcer la portée globale")
	secretsSetCmd.Flags().StringVar(&secretsSetProject, "project", "", "ID du projet cible (ex: t-sru-b267fbf1)")
}

func runSecretsSet(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()
	key := args[0]

	scope, scopeLabel := resolveSecretScope(ctx, a, secretsSetGlobal, secretsSetProject)

	// Read value from stdin (masked)
	fmt.Fprintf(a.IO.Out, "Valeur pour %q (%s): ", key, scopeLabel)
	valueBytes, err := term.ReadPassword(syscall.Stdin)
	fmt.Fprintln(a.IO.Out) // newline after masked input
	if err != nil {
		return fmt.Errorf("lecture du secret: %w", err)
	}
	value := strings.TrimSpace(string(valueBytes))
	if value == "" {
		return fmt.Errorf("valeur vide — opération annulée")
	}

	if a.Secrets == nil {
		return fmt.Errorf("secret store non disponible (keychain inaccessible et OH_PASSPHRASE non défini)")
	}

	// Use scoped set if keychain supports it
	if ks, ok := a.Secrets.(*keychain.Store); ok {
		if err := ks.SetScoped(ctx, key, value, scope); err != nil {
			return fmt.Errorf("stockage du secret: %w", err)
		}
	} else {
		if err := a.Secrets.Set(ctx, key, value); err != nil {
			return fmt.Errorf("stockage du secret: %w", err)
		}
	}

	fmt.Fprintf(a.IO.Out, "%s Secret %q stocké (%s)\n",
		theme.SuccessStyle.Render(theme.IconSuccess), key, scopeLabel)
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// oh secrets get <key>
// ─────────────────────────────────────────────────────────────────────────────

var secretsGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Afficher un secret (masqué par défaut)",
	Args:  cobra.ExactArgs(1),
	RunE:  runSecretsGet,
}

var secretsGetReveal  bool
var secretsGetGlobal  bool
var secretsGetProject string

func init() {
	secretsGetCmd.Flags().BoolVar(&secretsGetReveal, "reveal", false, "Afficher la valeur en clair")
	secretsGetCmd.Flags().BoolVar(&secretsGetGlobal, "global", false, "Chercher uniquement en portée globale")
	secretsGetCmd.Flags().StringVar(&secretsGetProject, "project", "", "ID du projet cible")
}

func runSecretsGet(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()
	key := args[0]

	if a.Secrets == nil {
		return fmt.Errorf("secret store non disponible")
	}

	var value, sourceLabel string

	if ks, ok := a.Secrets.(*keychain.Store); ok {
		scope, scopeLabel := resolveSecretScope(ctx, a, secretsGetGlobal, secretsGetProject)

		// Try scoped first, fall back to global
		val, err := ks.GetScoped(ctx, key, scope)
		if err != nil {
			return fmt.Errorf("lecture du secret: %w", err)
		}
		if val != "" {
			value = val
			sourceLabel = scopeLabel
		} else if scope != "global" {
			// Fall back to global
			val, err = ks.GetScoped(ctx, key, "global")
			if err != nil {
				return fmt.Errorf("lecture du secret (global): %w", err)
			}
			value = val
			sourceLabel = "global"
		}
	} else {
		val, err := a.Secrets.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("lecture du secret: %w", err)
		}
		value = val
		sourceLabel = "store"
	}

	if value == "" {
		fmt.Fprintf(a.IO.Out, "%s Secret %q non trouvé\n",
			theme.WarningStyle.Render(theme.IconWarning), key)
		return nil
	}

	display := maskSecret(value)
	if secretsGetReveal {
		display = value
	}

	fmt.Fprintf(a.IO.Out, "%s %s  %s\n",
		theme.Bold.Render(key),
		display,
		theme.Subtitle.Render("("+sourceLabel+")"))
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// oh secrets list
// ─────────────────────────────────────────────────────────────────────────────

var secretsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lister tous les secrets connus",
	RunE:  runSecretsList,
}

func runSecretsList(cmd *cobra.Command, _ []string) error {
	a := MustApp()
	ctx := cmd.Context()

	if a.Secrets == nil {
		return fmt.Errorf("secret store non disponible")
	}

	if ks, ok := a.Secrets.(*keychain.Store); ok {
		entries, err := ks.ListAll(ctx)
		if err != nil {
			return fmt.Errorf("listage des secrets: %w", err)
		}
		if len(entries) == 0 {
			fmt.Fprintf(a.IO.Out, "  Aucun secret enregistré\n")
			return nil
		}
		fmt.Fprintf(a.IO.Out, "\n  %-30s %-20s %s\n",
			theme.Bold.Render("Clé"),
			theme.Bold.Render("Portée"),
			theme.Bold.Render("Présent"))
		fmt.Fprintf(a.IO.Out, "  %s\n", strings.Repeat("─", 60))
		for _, e := range entries {
			scope := e.Scope
			if scope == "" {
				scope = "global"
			}
			present := theme.ErrorStyle.Render("✗")
			if e.Present {
				present = theme.SuccessStyle.Render("✓")
			}
			fmt.Fprintf(a.IO.Out, "  %-30s %-20s %s\n", e.Key, scope, present)
		}
		fmt.Fprintln(a.IO.Out)
	} else {
		// filecrypt fallback — List returns key names only (all global)
		keys, err := a.Secrets.List(ctx)
		if err != nil {
			return fmt.Errorf("listage des secrets: %w", err)
		}
		if len(keys) == 0 {
			fmt.Fprintf(a.IO.Out, "  Aucun secret enregistré\n")
			return nil
		}
		fmt.Fprintf(a.IO.Out, "\n  %-30s %-20s %s\n",
			theme.Bold.Render("Clé"),
			theme.Bold.Render("Portée"),
			theme.Bold.Render("Présent"))
		fmt.Fprintf(a.IO.Out, "  %s\n", strings.Repeat("─", 60))
		for _, k := range keys {
			fmt.Fprintf(a.IO.Out, "  %-30s %-20s %s\n",
				k, "global", theme.SuccessStyle.Render("✓"))
		}
		fmt.Fprintln(a.IO.Out)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// oh secrets delete <key>
// ─────────────────────────────────────────────────────────────────────────────

var secretsDeleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Supprimer un secret du keychain",
	Args:  cobra.ExactArgs(1),
	RunE:  runSecretsDelete,
}

var secretsDeleteGlobal  bool
var secretsDeleteProject string

func init() {
	secretsDeleteCmd.Flags().BoolVar(&secretsDeleteGlobal, "global", false, "Supprimer de la portée globale")
	secretsDeleteCmd.Flags().StringVar(&secretsDeleteProject, "project", "", "ID du projet cible")
}

func runSecretsDelete(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()
	key := args[0]

	if a.Secrets == nil {
		return fmt.Errorf("secret store non disponible")
	}

	scope, scopeLabel := resolveSecretScope(ctx, a, secretsDeleteGlobal, secretsDeleteProject)

	// Confirmation
	fmt.Fprintf(a.IO.Out, "Supprimer %q (%s) ? [y/N] ", key, scopeLabel)
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
		fmt.Fprintf(a.IO.Out, "Annulé\n")
		return nil
	}

	if ks, ok := a.Secrets.(*keychain.Store); ok {
		if err := ks.DeleteScoped(ctx, key, scope); err != nil {
			return fmt.Errorf("suppression du secret: %w", err)
		}
	} else {
		if err := a.Secrets.Delete(ctx, key); err != nil {
			return fmt.Errorf("suppression du secret: %w", err)
		}
	}

	fmt.Fprintf(a.IO.Out, "%s Secret %q supprimé (%s)\n",
		theme.SuccessStyle.Render(theme.IconSuccess), key, scopeLabel)
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Registration
// ─────────────────────────────────────────────────────────────────────────────

func init() {
	secretsCmd.AddCommand(secretsSetCmd)
	secretsCmd.AddCommand(secretsGetCmd)
	secretsCmd.AddCommand(secretsListCmd)
	secretsCmd.AddCommand(secretsDeleteCmd)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// resolveSecretScope determines the scope (global or project ID) for a secret
// operation. Resolution order:
//  1. --project flag (explicit)
//  2. --global flag → "global"
//  3. Auto-detect from cwd (like oh claim)
//  4. Fallback: "global"
func resolveSecretScope(ctx context.Context, a *app.App, forceGlobal bool, forceProject string) (scope, label string) {
	if forceProject != "" {
		return forceProject, fmt.Sprintf("projet %s", forceProject)
	}
	if forceGlobal {
		return "global", "global"
	}
	// Auto-detect from current directory
	projectID := detectCurrentProject(ctx, a)
	if projectID != "" {
		if p, err := a.Projects.Get(ctx, projectID); err == nil {
			return projectID, fmt.Sprintf("projet %s", p.Name)
		}
		return projectID, fmt.Sprintf("projet %s", projectID)
	}
	return "global", "global"
}

// maskSecret returns "****<last4>" for display — never the full value.
func maskSecret(s string) string {
	if len(s) == 0 {
		return "(vide)"
	}
	if len(s) <= 4 {
		return "****"
	}
	return "****" + s[len(s)-4:]
}
