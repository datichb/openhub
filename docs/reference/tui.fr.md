# Référence TUI — OpenHub

> Documentation complète de l'interface terminal interactive (TUI) d'OpenHub.

## Lancement

```bash
oh          # Lance le TUI (terminal interactif détecté)
oh --no-tui # Force le mode CLI classique
```

Le TUI se lance automatiquement quand `oh` est exécuté sans sous-commande dans un terminal interactif. Variables qui désactivent le TUI : `CI=true`, `TERM=dumb`, `OH_RICH_TUI=0`.

## Principes de design

Le TUI suit un design **omnibar-first** inspiré des lanceurs fuzzy (fzf, Telescope) et de l'approche minimaliste d'opencode :

- **Point d'interaction unique** : l'omnibar gère toutes les commandes
- **Contenu d'abord** : espace maximum dédié au contenu
- **Contextuel** : l'interface s'adapte à la vue active
- **Raccourcis minimaux** : seulement 4 raccourcis globaux à mémoriser

## Layout

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                   │
│                    ZONE DE CONTENU                                │
│              (s'adapte à la vue active)                           │
│                                                                   │
├───────────────────────────────────────────────────────────────────┤
│  > _  Ctrl+P commande · Esc retour · Ctrl+Q quitter             │ Omnibar (1 row)
└───────────────────────────────────────────────────────────────────┘
```

## Raccourcis globaux

| Touche | Action |
|--------|--------|
| `Ctrl+P` | Activer l'omnibar |
| `Esc` | Retour (vue précédente) |
| `Ctrl+Q` | Quitter le TUI |
| Toute lettre | Active l'omnibar avec ce caractère (si non consommé par la vue) |

C'est tout. Quatre raccourcis. Tout le reste passe par l'omnibar.

## L'Omnibar

L'omnibar est toujours visible en bas de l'écran. Il a deux modes :

### Mode passif (par défaut)

Affiche les hints contextuels de la vue active :

```
│  Ctrl+P commande · j/k nav · Enter ouvrir · h/l colonnes        │
```

### Mode actif (saisie)

Accepte la saisie avec suggestions fuzzy au-dessus :

```
│  ● Audit Sécurité      Audit de sécurité (OWASP, injections)    │
│  ○ Audit Performance   Audit performance (N+1, mémoire, CPU)    │
│  ○ Audit Architecture  Audit d'architecture (couplage, patterns) │
├───────────────────────────────────────────────────────────────────┤
│  > audit_                                                         │
```

### Contrôles de l'omnibar

| Touche | Action |
|--------|--------|
| Saisie | Filtre les commandes (fuzzy) |
| `↓` / `Tab` | Descendre dans les suggestions |
| `↑` / `Shift+Tab` | Monter dans les suggestions |
| `Enter` | Exécuter la commande sélectionnée |
| `Esc` | Fermer et retourner au contenu |

## Commandes disponibles

### Sessions

| Commande | Alias | Description |
|----------|-------|-------------|
| `start` | session, code, launch | Lancer une session opencode (avec sélecteur de mode) |
| `start dev` | dev, ticket | Session orientée développement |
| `start onboard` | onboard | Session d'onboarding projet |
| `audit` | — | Lancer un audit (sélecteur de type) |
| `audit security` | secu | Audit de sécurité (OWASP) |
| `audit performance` | perf | Audit de performance |
| `audit architecture` | archi | Audit d'architecture |
| `audit accessibility` | a11y | Audit d'accessibilité |
| `audit ecodesign` | eco | Audit éco-conception |
| `audit observability` | obs | Audit d'observabilité |
| `review` | rev, cr | Lancer une code review |
| `review standard` | — | Review classique |
| `review adversarial` | adversarial | Review adversariale |
| `review edge` | edge | Review cas limites |
| `review complete` | complete, all | Review complète (tous les modes) |
| `debug` | dbg | Session debug (avec prompt de description) |
| `quick` | q, fast | Lancer opencode directement |
| `parallel` | par, multi | Vue sessions parallèles |

### Projets

| Commande | Alias | Description |
|----------|-------|-------------|
| `board` | kanban, tasks | Kanban du projet actif |
| `projects` | proj, list | Liste des projets |
| `deploy` | dep, push | Déployer agents/skills sur le projet actif |
| `sync` | synchronize | Synchroniser tous les projets |

### Configuration

| Commande | Alias | Description |
|----------|-------|-------------|
| `config` | cfg, settings, hub | Configuration du hub |
| `models` | mod, model, llm | Configuration des modèles |
| `provider` | prov, api | Configuration du provider LLM |
| `mcp` | servers | Serveurs MCP |

### Système

| Commande | Alias | Description |
|----------|-------|-------------|
| `status` | stat, info | État du système |
| `doctor` | health, check | Diagnostic de santé |
| `metrics` | met, stats, tokens | Statistiques d'usage |
| `plugins` | plug, extensions | Gestion des plugins |
| `upgrade` | up, update | Mettre à jour opencode |
| `help` | ?, aide, shortcuts | Aide et raccourcis |

### Navigation

| Commande | Alias | Description |
|----------|-------|-------------|
| `home` | accueil, welcome | Retour au splash |
| `quit` | exit, q | Quitter le TUI |

### Team (si activé)

| Commande | Alias | Description |
|----------|-------|-------------|
| `team board` | team kanban | Kanban d'équipe |
| `team status` | team stat | Statut d'équipe |
| `team activity` | activite, feed | Activité récente |
| `takeover briefs` | takeover | Briefs de reprise de contexte |
| `worktrees` | wt | Gestion git worktrees |
| `patterns` | pat | Patterns d'équipe |
| `policies` | pol, rules | Politiques d'équipe |

## Raccourcis contextuels par vue

Quand une vue est active, des raccourcis additionnels fonctionnent directement sans activer l'omnibar. Ils sont affichés dans le texte passif de l'omnibar.

### Vue Board

| Touche | Action |
|--------|--------|
| `h` / `l` | Changer de colonne |
| `j` / `k` | Naviguer les items |
| `r` | Rafraîchir |
| `Enter` | Ouvrir le détail |

### Vue Projets

| Touche | Action |
|--------|--------|
| `a` | Ajouter un projet |
| `d` | Supprimer un projet |
| `r` | Renommer |
| `Enter` | Configurer |

### Vue Config

| Touche | Action |
|--------|--------|
| `j` / `k` | Naviguer les clés |
| `Enter` | Modifier la valeur |

## Sessions opencode

Quand une session est lancée (Start, Audit, Review, Debug, Quick) :
1. Le TUI se met en pause
2. opencode prend le contrôle du terminal
3. À la fermeture d'opencode, le TUI reprend exactement où il en était
4. Un toast confirme le résultat de la session
