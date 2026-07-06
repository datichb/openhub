# 022 — Migration du CLI de Bash vers Go

## Statut

accepted

## Contexte

Le CLI original (`oc`) était un ensemble de ~60 scripts shell (bash) totalisant ~15 000 lignes de code, orchestré par un dispatcher central (`oc.sh`). Il dépendait d'outils externes : jq, Node.js, sqlite3, bun, curl, et GNU parallel.

Une analyse menée en juin 2026 (`docs/dev/cli-migration-analysis.md`) a identifié 17 problèmes structurels :

- **Fragilité** : parsing texte avec awk/sed/grep, pas de typage, erreurs silencieuses
- **Dépendances externes** : 6+ outils runtime requis, comportement variable selon la plateforme
- **Tests** : tests BATS lents, instables et incomplets (pas d'isolation unitaire)
- **Performance** : 30+ spawns de sous-processus par deploy, lectures répétées (api-keys.local.md lu 30+ fois)
- **Distribution** : pas de binaire — les utilisateurs devaient cloner le repo et créer des alias shell
- **Secrets** : clés API stockées en clair dans des fichiers markdown
- **i18n** : non supporté — toutes les chaînes codées en dur en français
- **Configuration** : fichiers JSON parsés avec jq, pas de validation de schéma, pas de migration
- **Serveurs MCP** : processus TypeScript nécessitant le runtime Node.js

Ces problèmes étaient intrinsèques à l'architecture shell et ne pouvaient pas être résolus par un refactoring incrémental.

## Décision

Nous avons décidé de réécrire le CLI entièrement en Go, sous le nouveau nom `oh` :

- **Architecture** : monorepo — code Go dans `cli/` coexistant avec `agents/` et `skills/`
- **Stack** : Go 1.26 + Cobra (commandes) + Viper (config) + BubbleTea (TUI) + huh (formulaires) + lipgloss (styling)
- **Distribution** : binaire statique unique via GoReleaser + Homebrew tap (`datichb/openhub/oh`)
- **Configuration** : TOML (`~/.oh/hub.toml`) remplaçant JSON (`config/hub.json`)
- **Secrets** : keychain OS (go-keyring) avec fallback fichier chiffré AES-256-GCM
- **Projets & Sessions** : base SQLite (remplaçant les fichiers markdown)
- **Serveurs MCP** : implémentations Go natives (remplaçant TypeScript dans `servers/`)
- **i18n** : système bilingue JSON (fr/en), 474 clés avec test de parité
- **Stratégie de migration** : Strangler Fig — CLI Go développé en parallèle, puis bash supprimé

La migration a été exécutée en 7 phases de développement plus 3 passes prioritaires (P0, P1, P2 avec 5 blocs), un cycle d'audit (33 findings, 6 phases de remédiation A-F), et un bloc final de nettoyage. Effort total : ~12 jours répartis sur juin-juillet 2026.

Documents de référence :
- `docs/dev/cli-migration-plan-v2.md` — plan technique complet
- `docs/dev/cli-remaining-work.md` — suivi du backlog (100% terminé)
- `docs/dev/cli-migration-analysis.md` — analyse initiale des problèmes

## Conséquences

### Positives

- **Zéro dépendance runtime** — binaire unique de ~5,5 MB, ne requiert que git
- **Cross-platform** — darwin/amd64, darwin/arm64, linux/amd64, linux/arm64
- **Suite de tests robuste** — 213 tests sur 24 packages, vérification race-condition
- **Démarrage instantané** — pas de boot interpréteur, pas de résolution de modules
- **Distribution Homebrew** — `brew install datichb/openhub/oh`
- **TUI interactif** — dashboard, tableau kanban, picker, formulaires (BubbleTea)
- **i18n complète** — 474 clés, bilingue (fr/en), test de parité imposé
- **Secrets sécurisés** — keychain OS avec fallback chiffré (Argon2id + AES-256-GCM)
- **MCP natif** — serveurs Go sans dépendance Node.js
- **Logging structuré** — `log/slog` avec flag `--verbose`
- **Complétion shell** — bash, zsh, fish, powershell

### Négatives / Compromis

- **Migration breaking** pour les utilisateurs existants — mitigé par le guide `MIGRATION.md` avec table d'équivalence complète
- **Features supprimées** — `oh conventions`, `oh agent create/edit`, `oh skills install/remove`, machine à états de session, installeur Node.js (jugées non-essentielles ou gérées nativement par opencode)
- **Toolchain Go requise** pour contribuer au code CLI (vs. éditer des scripts shell)
- **Taille du binaire** — ~5,5 MB vs. ~0 pour des scripts shell (acceptable pour les gains)

## Alternatives rejetées

| Alternative | Raison du rejet |
|-------------|----------------|
| TypeScript + Bun | Recommandé initialement (analyse juin 2026). Rejeté : dépendance runtime (Node/Bun), pas de binaire natif sans bundling, écosystème CLI moins mature que Go (Cobra/Viper/BubbleTea) |
| Python (Click/Typer) | Dépendance runtime, distribution complexe (pyinstaller/shiv), pas de TUI natif comparable à BubbleTea |
| Rust (clap + ratatui) | Temps de développement plus long, courbe d'apprentissage plus raide, écosystème plus restreint pour les formulaires CLI interactifs |
| Refactoring incrémental du bash | Structurellement impossible — les 17 problèmes identifiés sont inhérents au scripting shell (pas de types, pas de packages, pas de testabilité) |
| Approche hybride (garder du bash) | Charge de maintenance de deux systèmes, confusion utilisateur entre les entry points `oc` et `oh` |
