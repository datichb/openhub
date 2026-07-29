# ADR-031 : Consolidation des Vues de Configuration

## Statut

Accepted

## Date

2026-07-29

## Contexte

La TUI a actuellement 10 vues liees a la configuration, construites a differents moments avec des patterns d'interaction inconsistants :

| Probleme | Impact |
|---|---|
| ConfigView et HubConfigView editent toutes les deux hub.toml avec 8 champs en doublon | Bugs de lecture stale ; confusion utilisateur sur quelle vue est canonique |
| MCPView et MCPConfigView adressent le meme domaine (services MCP) mais separent lecture/ecriture | L'utilisateur doit naviguer deux vues pour gerer un seul sujet |
| Le panel projet de MCPView et la section MCP de ProjectConfigView editent les memes overrides MCP par projet | Chemins de mutation dupliques ; les changements dans l'un sont invisibles dans l'autre |
| 4 widgets de base differents sur 10 vues (TextView, List, Table, dual-List) | Pas de memoire musculaire coherente |
| Auto-save (5 vues) vs. save explicite avec `w` (3 vues) vs. lecture seule (2 vues) | Semantique de persistance imprevisible |
| Les keybindings Space/Enter/e varient par vue | Les utilisateurs ne peuvent pas former un modele mental universel |
| Seules 4/10 vues implementent CommandProvider pour l'omnibar | Decouverte inconsistante |
| HubConfigView utilise `term.ReadPassword(syscall.Stdin)` brut | Corrompt le rendu TUI |

## Decision

Consolider les vues de configuration de 10 a 8, avec un contrat d'interaction unifie :

### Inventaire des vues (apres)

| Vue | ViewID | Objectif |
|---|---|---|
| SettingsView (HubConfigView renommee) | `settings` | Defauts personnels hub |
| TeamsView (nouvelle) | `teams` | Gestion multi-equipe : liste, ajout, retrait, sync |
| TeamDetailView (remplace MCPConfigView) | `team.detail` | Config d'une equipe avec affichage resolution (enforced/recommended/effective) |
| ProjectConfigView (enrichie) | `project.config` | Overrides par projet avec annotations source/enforced |
| MCPView (simplifiee) | `mcp` | MCP hub : tokens, write, enabled (panel projet retire) |
| ProviderView | `provider` | Wizard credentials provider |
| ModelsView (enrichie) | `models` | Cascade models avec colonne source (equipe/hub/projet) |
| SecretsView | `secrets` | Gestion credentials keychain |

PluginsView et PoliciesView restent inchangees.

### Supprimees
- **ConfigView** : entierement redondante avec SettingsView (ses champs exclusifs migres vers SettingsView ou ProviderView)
- **MCPConfigView** : absorbee dans TeamDetailView (meme sujet, meilleure integration)
- **Panel projet de MCPView** : absorbe dans ProjectConfigView (chemin de mutation unique pour les overrides MCP projet)

### Contrat d'interaction unifie

| Touche | Action | Toutes les vues |
|---|---|---|
| j/k, fleches | Naviguer | Oui |
| Enter | Editer l'item selectionne | Oui (si editable) |
| Space | Toggle (bool/tri-state/cycle) | Oui (si toggleable) |
| a | Ajouter | Vues avec CRUD |
| d | Supprimer (avec confirmation modale) | Vues avec CRUD |
| u | Undo derniere action | Toutes (via UndoStack) |
| r | Refresh depuis la source | Toutes |
| v | Changer mode vue (simple/detaille) | Vues avec modes |
| ? | Aide (keybindings contextuels) | Toutes |

### Modele de persistance : Auto-save + Undo

Toutes les vues utilisent l'auto-save (persistance immediate a chaque mutation) avec un undo stack borne (profondeur 10). Plus de `w` explicite pour sauver, plus de dirty tracking, plus d'avertissements "modifications non sauvegardees". La touche undo (`u`) revient sur la derniere mutation auto-sauvee.

### Standardisation du widget de base

- `tview.List` (via widget `SectionedList`) : defaut pour toutes les vues d'edition de config
- `tview.Table` : exception pour ModelsView (donnees tabulaires multi-colonnes)
- `tview.TextView` : uniquement pour les pages statut en lecture seule (PluginsView)

### Toutes les vues implementent CommandProvider

Commandes omnibar contextuelles sur les 8 vues de config (etait 4/10 avant).

## Alternatives Rejetees

| Alternative | Raison du rejet |
|---|---|
| Vue "Settings" unique unifiee avec onglets pour tout | Trop monolithique ; perd le benefice des vues domaine focalisees ; la tab bar mange de l'espace vertical en TUI |
| Garder les 10 vues mais juste harmoniser les keybindings | Ne resout pas les problemes de mutation dupliquee et de lecture stale |
| Save explicite partout (avec dirty tracking) | Plus de friction ; les utilisateurs oublient de sauvegarder ; auto-save + undo est un modele mental plus simple |
| Auto-save sans undo | Trop risque pour les operations destructives (suppression, toggle de settings production) |

## Consequences

### Positives
- Chemin de mutation unique par setting — plus de lectures stale entre vues
- Memoire musculaire coherente sur toutes les vues de config
- Auto-save + undo : edition zero-friction avec filet de securite
- `term.ReadPassword` elimine (remplace par `ShowPasswordModal`)
- La nouvelle TeamsView supporte les workflows multi-equipe (ADR-029)
- Les annotations de resolution (ADR-030) visibles dans ProjectConfigView et TeamDetailView

### Negatives / Compromis
- Les utilisateurs familiers avec le raccourci ConfigView actuel doivent apprendre la nouvelle SettingsView
- L'affichage 3 colonnes de MCPConfigView est maintenant un mode (`v`) dans TeamDetailView — legerement moins immediat
- PluginsView garde le TextView (pas List) — acceptable car c'est une page statut avec interaction minimale
- Le widget SectionedList est une nouvelle abstraction a maintenir
