# Référence TUI — OpenHub

> Documentation complète de l'interface terminal interactive (TUI) d'OpenHub.

## Lancement

```bash
oh          # Lance le TUI (terminal interactif détecté)
oh --no-tui # Force le mode CLI classique
```

Le TUI se lance automatiquement quand `oh` est exécuté sans sous-commande dans un terminal interactif. Variables qui désactivent le TUI : `CI=true`, `TERM=dumb`, `OH_RICH_TUI=0`.

Si aucun provider n'est configuré (premier lancement), un wizard inline de configuration initiale se lance automatiquement dans le shell. Voir [InlineWizardView](tui-inline-wizard.fr.md) pour les détails du composant.

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

### Workflows et sessions

| Commande | Alias | Description |
|----------|-------|-------------|
| `run <workflow>` | id du workflow ; anciens noms : `dev` → `run ticket`, `start` → `run feature`, `onboard` → `run onboarding`, `audit`, `review` (`rev`, `cr`), `debug` (`dbg`), `feedback` → `run review-feedback` | Ouvre la fiche de lancement du workflow (générée depuis le catalogue) |
| `run <workflow> ⟨ticket⟩` | — | Sur le board : workflow lancé sur le ticket sélectionné |
| `workflows` | catalogue, wf | Catalogue des workflows (brouillons, publication, historique) |
| `review.publish` | publish, mr | Créer la MR de la branche courante (API GitLab, terminal suspendu ; si l'écriture GitLab est activée) |
| `coder` | session, free, libre | Session libre (sans workflow ; opencode V1 ou catalogue vide) |
| `sessions` | parallel, inbox | Vue Sessions |

### Fiche de lancement

Générée depuis le YAML du workflow, en trois étapes : **Entrées** (une ligne par entrée : ticket Beads avec sélecteur `Choisir…`, case pour `bool`, liste pour `enum`, zone de texte pour `text`), **Options** (mode, exécution — les environnements indisponibles affichent la raison —, emplacement : base, worktrees existants, nouveau worktree ; ouverture), **Récap** (agents, budget du 1er tour, isolation, sessions et emplacements, avertissements). `Ctrl+S` lance depuis n'importe quelle étape, `Ctrl+B` revient, `Esc` ferme. Un second lancement pendant la préparation est ignoré. Avec plusieurs tickets, une case « Une seule session pour tous les tickets » est proposée. Quand une précondition suggère un autre workflow, le récap propose un bouton « Lancer <wf> d'abord (puis revenir) ».

### Démarrer, board, catalogue

- **Démarrer** (accueil, projet, équipe) : ★ épinglés (5 max), récents (3), suggestions si vide ; en mode projet/équipe, catégories repliées (Entrée = choix du workflow). `*` épingle ou désépingle (portée : hub, projet ou équipe selon l'accueil). « Tous les workflows (N) » ouvre le catalogue.
- **Board** : `a` sur un ticket liste les workflows qui prennent un ticket Beads ; la fiche s'ouvre à l'étape Options, ticket prérempli.
- **Catalogue** : workflows par couche (version, risque, ⌂ ▣ ☁, validité), détail à droite ; Entrée lance, `*` épingle. Avec un team-state (équipe ou espace solo), le catalogue est **modifiable** :
  - sections Hub (lecture seule), Équipe, Projet, **Mes brouillons** (`✎`, nombre d'erreurs, badge « + nouvelle brique » pour une brique d'équipe utilisée pour la première fois) et **Intégrité** (fichiers publiés ignorés, briques refusées) ; `✎` sur un workflow publié = vous en avez un brouillon, `⏳` = publication en attente du réseau ;
  - `n` nouveau (vide, étendre ou dupliquer le workflow sélectionné ; couche équipe ou projet ; identifiant), `e` éditer (sur un workflow du hub : l'étendre), `v` valider, `t` tester le brouillon (fiche de lancement « ✎ brouillon », en local), `p` publier, `D` diff du brouillon avec l'impact, `h` historique, `x` archiver (raison facultative ; sur un brouillon : l'abandonner), `r` recharger ; Entrée sur un brouillon = le tester ;
  - **Publier** : version actuelle → suivante, validation, impact (les élargissements sont marqués ⚠), nouvelles briques, diff du document et du gabarit, gouvernance (« Publication : tout membre ») ; message obligatoire, `Ctrl+S` publie une seule fois ; hors ligne, la publication est mise en attente et rejouée à la synchronisation suivante du team-state ;
  - **Historique** : versions (auteur, date, message, actuelle) ; Entrée = diff avec la version actuelle, `r` = restaurer (republiée comme nouvelle version).
- **Vue Sessions** : `e` « Enchaîner avec… » propose les workflows qui prennent une sortie de la session (branche, tickets), fiche préremplie ; un lancement mis en attente par une précondition (« lancer onboarding puis revenir ») est proposé en premier. Quand une session de workflow se termine (ou déclare ses sorties), un toast annonce la suite proposée et le détail de la session l'affiche (« ↪ Enchaîner avec review (e) »).

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

| Commande | Alias | Description | Disponibilité |
|----------|-------|-------------|---------------|
| `home` | accueil, welcome | Retour au splash | Global |
| `project.mode` | project mode | Mode Projet | `Ctrl+T` (Hub, Team modes) |
| `hub.mode` | hub mode | Mode Hub | `Ctrl+T` (Team, Project modes) |
| `workflow` | wf | Configuration du workflow | Global |
| `quit` | exit, q | Quitter le TUI | Global |

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

### Vue Board (projet)

| Touche | Action |
|--------|--------|
| `h` / `←` | Colonne précédente |
| `l` / `→` | Colonne suivante |
| `j` / `↑` | Item précédent |
| `k` / `↓` | Item suivant |
| `r` | Rafraîchir |
| `g` | Aller au premier item |
| `G` | Aller au dernier item |
| `Enter` | Ouvrir le détail |

### Vue Team Board

Le tableau d'équipe affiche **5 colonnes** : TODO (`planned`), IN PROGRESS (`in_progress`), REVIEW (`review`), BLOCKED (`blocked`), DONE (`done`). Seuls les tickets actifs apparaissent — les membres sans ticket en cours ne sont pas listés.

Les tickets affichent des étiquettes compactes : `[AI]` (vert) pour `agent-reviewed`, `[!]` (jaune) pour `needs-human-review`.

| Touche | Action |
|--------|--------|
| `h` / `←` | Colonne précédente |
| `l` / `→` | Colonne suivante |
| `j` / `↑` | Item précédent |
| `k` / `↓` | Item suivant |
| `c` | Prendre le ticket sélectionné (s'assigner) |
| `x` | Libérer le ticket sélectionné |
| `t` | Transférer le ticket à un autre membre |
| `s` | Changer le statut du ticket sélectionné |
| `r` | Rafraîchir (tire les dernières données git + tracker si configuré) |
| `g` | Aller au premier item |
| `G` | Aller au dernier item |
| `q` / `Esc` | Retour au hub |

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

## Wizards inline

Les flows de configuration multi-step (`team init`, `init`, `project add`) utilisent un composant **InlineWizardView** qui tourne dans le shell TUI sans switch alt-screen. Le wizard est pushé sur le router stack via `shell.PushView(v)` et pop automatiquement à la fin.

Raccourcis : `Ctrl+S` valider, `Ctrl+B` retour, double `Esc` passer.

Voir [Référence InlineWizardView](tui-inline-wizard.fr.md) pour l'API complète et les patterns de création de nouveaux wizards.

### Architecture — PushView

La méthode `PushView(v View)` sur `ShellAccess` permet de pusher une vue éphémère sur le router stack sans pré-enregistrement. Le wizard reçoit automatiquement `ShellAccess` via l'interface `shellAware`. Cette méthode est utilisée par les actions omnibar `team init`, `init`, et `project add`.

## Sessions opencode

Quand une session est lancée (Start, Audit, Review, Debug, Quick) :
1. Le TUI se met en pause
2. opencode prend le contrôle du terminal
3. À la fermeture d'opencode, le TUI reprend exactement où il en était
4. Un toast confirme le résultat de la session

## Synchronisation des vues d'équipe

Toutes les vues d'équipe (board, status, activity, takeover, patterns, policies) tirent automatiquement les dernières données git d'équipe à l'ouverture. Appuyer sur `r` déclenche également un pull avant le rafraîchissement.

- Un toast "Synchronisation..." s'affiche uniquement si le pull dépasse 1 seconde
- En cas d'erreur réseau : un toast d'avertissement est affiché et les données locales sont utilisées (aucun blocage)
