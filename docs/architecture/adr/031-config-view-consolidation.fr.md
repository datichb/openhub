# ADR-031 : Consolidation des Vues de Configuration

## Statut

Accepted

## Date

2026-07-29

## Contexte

La TUI a actuellement 10 vues liées à la configuration, construites à différents moments avec des patterns d'interaction inconsistants :

| Problème | Impact |
|---|---|
| ConfigView et HubConfigView éditent toutes les deux hub.toml avec 8 champs en doublon | Bugs de lecture stale ; confusion utilisateur sur quelle vue est canonique |
| MCPView et MCPConfigView adressent le même domaine (services MCP) mais separent lecture/écriture | L'utilisateur doit naviguer deux vues pour gérer un seul sujet |
| Le panel projet de MCPView et la section MCP de ProjectConfigView éditent les mêmes overrides MCP par projet | Chemins de mutation dupliqués ; les changements dans l'un sont invisibles dans l'autre |
| 4 widgets de base différents sur 10 vues (TextView, List, Table, dual-List) | Pas de mémoire musculaire cohérente |
| Auto-save (5 vues) vs. save explicite avec `w` (3 vues) vs. lecture seule (2 vues) | Sémantique de persistance imprevisible |
| Les keybindings Space/Enter/e varient par vue | Les utilisateurs ne peuvent pas former un modèle mental universel |
| Seules 4/10 vues implémentent CommandProvider pour l'omnibar | Découverte inconsistante |
| HubConfigView utilise `term.ReadPassword(syscall.Stdin)` brut | Corrompt le rendu TUI |

## Décision

Consolider les vues de configuration de 10 à 8, avec un contrat d'interaction unifié :

### Inventaire des vues (après)

| Vue | ViewID | Objectif |
|---|---|---|
| SettingsView (HubConfigView renommée) | `settings` | Defauts personnels hub |
| TeamsView (nouvelle) | `teams` | Gestion multi-equipe : liste, ajout, retrait, sync |
| TeamDetailView (remplace MCPConfigView) | `team.detail` | Config d'une équipe avec affichage résolution (enforced/recommended/effective) |
| ProjectConfigView (enrichie) | `project.config` | Overrides par projet avec annotations source/enforced |
| MCPView (simplifiée) | `mcp` | MCP hub : tokens, write, enabled (panel projet retiré) |
| ProviderView | `provider` | Wizard credentials provider |
| ModelsView (enrichie) | `models` | Cascade models avec colonne source (équipe/hub/projet) |
| SecretsView | `secrets` | Gestion credentials keychain |

PluginsView et PoliciesView restent inchangées.

### Supprimées
- **ConfigView** : entièrement redondante avec SettingsView (ses champs exclusifs migrés vers SettingsView ou ProviderView)
- **MCPConfigView** : absorbee dans TeamDetailView (même sujet, meilleure intégration)
- **Panel projet de MCPView** : absorbé dans ProjectConfigView (chemin de mutation unique pour les overrides MCP projet)

### Contrat d'interaction unifié

| Touche | Action | Toutes les vues |
|---|---|---|
| j/k, flèches | Naviguer | Oui |
| Enter | Éditer l'item sélectionné | Oui (si éditable) |
| Space | Toggle (bool/tri-state/cycle) | Oui (si toggleable) |
| a | Ajouter | Vues avec CRUD |
| d | Supprimer (avec confirmation modale) | Vues avec CRUD |
| u | Undo dernière action | Toutes (via UndoStack) |
| r | Refresh depuis la source | Toutes |
| v | Changer mode vue (simple/détaillé) | Vues avec modes |
| ? | Aide (keybindings contextuels) | Toutes |

### Modèle de persistance : Auto-save + Undo

Toutes les vues utilisent l'auto-save (persistance immédiate à chaque mutation) avec un undo stack borné (profondeur 10). Plus de `w` explicite pour sauver, plus de dirty tracking, plus d'avertissements "modifications non sauvegardees". La touche undo (`u`) revient sur la dernière mutation auto-sauvee.

### Standardisation du widget de base

- `tview.List` (via widget `SectionedList`) : défaut pour toutes les vues d'édition de config
- `tview.Table` : exception pour ModelsView (données tabulaires multi-colonnes)
- `tview.TextView` : uniquement pour les pages statut en lecture seule (PluginsView)

### Toutes les vues implémentent CommandProvider

Commandes omnibar contextuelles sur les 8 vues de config (était 4/10 avant).

## Alternatives Rejetées

| Alternative | Raison du rejet |
|---|---|
| Vue "Settings" unique unifiée avec onglets pour tout | Trop monolithique ; perd le bénéfice des vues domaine focalisees ; la tab bar mange de l'espace vertical en TUI |
| Garder les 10 vues mais juste harmoniser les keybindings | Ne résout pas les problèmes de mutation dupliquée et de lecture stale |
| Save explicite partout (avec dirty tracking) | Plus de friction ; les utilisateurs oublient de sauvegarder ; auto-save + undo est un modèle mental plus simple |
| Auto-save sans undo | Trop risqué pour les opérations destructives (suppression, toggle de settings production) |

## Conséquences

### Positives
- Chemin de mutation unique par setting — plus de lectures stale entre vues
- Mémoire musculaire cohérente sur toutes les vues de config
- Auto-save + undo : édition zéro-friction avec filet de sécurité
- `term.ReadPassword` éliminé (remplacé par `ShowPasswordModal`)
- La nouvelle TeamsView supporte les workflows multi-equipe (ADR-029)
- Les annotations de résolution (ADR-030) visibles dans ProjectConfigView et TeamDetailView

### Négatives / Compromis
- Les utilisateurs familiers avec le raccourci ConfigView actuel doivent apprendre la nouvelle SettingsView
- L'affichage 3 colonnes de MCPConfigView est maintenant un mode (`v`) dans TeamDetailView — légèrement moins immédiat
- PluginsView garde le TextView (pas List) — acceptable car c'est une page statut avec interaction minimale
- Le widget SectionedList est une nouvelle abstraction à maintenir
