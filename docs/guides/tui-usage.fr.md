> [Read in English](tui-usage.en.md)

# Guide -- Utiliser le TUI OpenHub

> Guide pratique pour naviguer et travailler dans l'interface TUI.

## Demarrage rapide

```bash
oh
```

Le TUI affiche un ecran d'accueil avec des hints de demarrage rapide et l'omnibar en bas. Le TUI organise le travail en trois modes : **mode Hub** (parametres globaux), **mode Projet** (vues par projet) et **mode Team** (collaboration equipe).

---

## Navigation

### Omnibar

L'omnibar est l'outil de navigation principal. Activez-le avec :
- **Taper n'importe quelle lettre** quand aucun champ n'a le focus
- **`Ctrl+P`** a tout moment

L'omnibar supporte le fuzzy matching -- tapez des mots partiels, abreviations ou alias :

```
> secu        → lance un audit securite
> dep         → deploie sur le projet actif
> doc         → ouvre le diagnostic doctor
> tst         → statut equipe
```

### Raccourcis clavier

| Touche | Action |
|--------|--------|
| `Ctrl+P` | Ouvrir l'omnibar |
| `Esc` | Retour (vue precedente / fermer l'omnibar) |
| `Ctrl+Q` | Quitter |
| `j` / `k` | Naviguer haut/bas dans les listes |
| `Enter` | Selectionner / confirmer |
| `Tab` | Basculer entre les panneaux (si applicable) |

### Toasts

Les resultats d'actions apparaissent en toasts dans le coin haut-droit (succes/erreur/info). Ils disparaissent automatiquement apres quelques secondes.

---

## Vues disponibles

### Sessions et actions

| Commande | Description |
|----------|-------------|
| `start` | Lancer une session (affiche le selecteur : Standard, Dev, Onboard) |
| `start dev` | Mode dev directement (workflow tickets) |
| `start onboard` | Mode onboard directement (decouverte projet) |
| `quick` | Lancement direct d'opencode (sans selection de mode) |
| `audit` | Lanceur d'audit (choisir le type) |
| `audit security` | Audit securite |
| `audit performance` | Audit performance |
| `audit architecture` | Audit architecture |
| `audit accessibility` | Audit accessibilite |
| `audit ecodesign` | Audit eco-conception |
| `audit observability` | Audit observabilite |
| `review` | Lanceur de review (choisir le mode) |
| `review standard` | Revue de code standard |
| `review adversarial` | Revue adversariale |
| `review edge` | Revue edge cases |
| `review complete` | Revue complete (tous les modes) |
| `debug` | Session debug avec description du probleme |
| `parallel` | Vue sessions paralleles (multi-tickets) |

Quand une session demarre, le TUI se suspend et opencode prend le relais. Quand vous quittez opencode, le TUI reprend.

### Gestion de projets

| Commande | Vue | Description |
|----------|-----|-------------|
| `projects` | Liste des projets | Tous les projets enregistres avec statut |
| `board` | Kanban | Board de tickets du projet (necessite bd) |
| `deploy` | - | Deployer agents/skills dans le projet courant |
| `sync` | - | Synchroniser agents/skills sur tous les projets |
| `project-config` | Config projet | Parametres par projet (provider, modele, MCP, agents) |

Dans la vue **Projets** :
- `a` pour ajouter un nouveau projet
- `d` pour supprimer
- `Enter` pour configurer
- `r` pour renommer

### Configuration

| Commande | Vue | Description |
|----------|-----|-------------|
| `models` | Config modeles | Configuration modeles par defaut et par agent |
| `provider` | Config provider | Parametres du fournisseur LLM |
| `mcp` | Serveurs MCP | Activer/desactiver/statut des serveurs MCP |
| `settings` | Parametres generaux | Langue, version opencode, auto-update |
| `secrets` | Secrets et tokens | Gerer les credentials stockes |
| `teams` | Liste des equipes | Gestion multi-equipes |

### Equipe (necessite un depot team-state)

| Commande | Vue | Description |
|----------|-----|-------------|
| `team-detail` | Detail equipe | Configuration et membres de l'equipe |
| `team board` | Board equipe | Kanban a travers tous les membres |
| `team status` | Statut | Resume de l'activite equipe |
| `team activity` | Activite | Evenements recents de l'equipe |
| `team briefs` | Takeover briefs | Contextes de reprise disponibles |
| `team patterns` | Patterns | Bibliotheque de patterns partages |
| `team policies` | Policies | Policies appliquees par l'equipe |
| `team sync` | - | Synchroniser les claims avec le tracker externe |
| `team init` | - | Initialiser les fonctionnalites equipe |

### Systeme

| Commande | Vue | Description |
|----------|-----|-------------|
| `status` | Statut systeme | Vue d'ensemble sante du hub et du projet |
| `doctor` | Diagnostics | Lancer les checks de sante (`r` pour relancer) |
| `metrics` | Metriques agents | Stats d'usage, cout et duree par agent |
| `plugins` | Gestionnaire plugins | Lister/installer/supprimer des plugins |
| `notifications` | Notifications | Evenements de notification recents |
| `worktrees` | Gestionnaire worktrees | Lister, ajouter, supprimer des Git worktrees |
| `upgrade` | - | Mettre a jour oh ou opencode |
| `help` | Aide | Raccourcis clavier et reference des commandes |

### Navigation

| Commande | Description |
|----------|-------------|
| `home` | Retourner a l'ecran d'accueil |
| `project mode` | Basculer vers la vue par projet |
| `hub mode` | Basculer vers la vue hub (globale) |
| `quit` | Quitter le TUI |

---

## Reference des types de sessions

### Types d'audit (6)

| Type | Domaine |
|------|---------|
| Security | OWASP, dependances, secrets, authentification |
| Performance | Charge, latence, memoire, CPU |
| Architecture | Patterns, couplage, complexite |
| Accessibility | WCAG, ARIA, navigation clavier |
| Eco-design | Empreinte carbone, usage des ressources |
| Observability | Logs, metriques, traces, alertes |

### Modes de review (4)

| Mode | Approche |
|------|----------|
| Standard | Revue equilibree sur toutes les dimensions |
| Adversarial | Tente activement de casser le code |
| Edge cases | Focus sur les conditions limites et les chemins d'erreur |
| Complete | Tous les modes en parallele, resultats fusionnes |

---

## Astuces

- **Fuzzy matching** -- tapez n'importe quelle partie d'un nom de commande. `sec` correspond a `audit security`, `rev` a `review`.
- **Changement de mode** -- le TUI adapte les commandes disponibles a votre mode courant. Les commandes specifiques a un projet n'apparaissent qu'en mode projet.
- **Edition de config** -- dans les vues de config, naviguez avec `j`/`k`, appuyez sur `Enter` pour modifier une valeur. Les changements sont sauvegardes automatiquement.
- **Raccourcis contextuels** -- chaque vue affiche les raccourcis disponibles dans le texte passif de l'omnibar en bas.
- **Detection automatique du projet** -- si vous avez lance `oh` depuis un repertoire de projet enregistre, il demarre en mode projet pour ce projet.
