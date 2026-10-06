> [Read in English](cli-team.en.md)

# Reference CLI — Equipe

## Analytique equipe

## Analytique

### oh team claim

Reclame un ticket et cree un claim dans la base de donnees equipe.

```
oh team claim <ticket-id> [options]
```

| Flag | Description |
|------|-------------|
| `--planned` | Cree le claim en statut `planned` (colonne TODO) au lieu de demarrer immediatement en `in_progress` |
| `--project` | Projet cible |
| `--worktree` | Creer un worktree pour le ticket reclame |

Sans `--planned`, le claim est cree directement en statut `in_progress`.

**Exemple :**

```bash
oh team claim TICKET-123
oh team claim TICKET-123 --planned
```

### oh team release

Libere un ticket reclame.

```
oh team release <ticket-id> [options]
```

| Flag | Description |
|------|-------------|
| `--project` | Projet cible |

```bash
oh team release TICKET-123
```

### oh team claim transfer

Transfere un claim a un autre membre de l'equipe.

```
oh team claim transfer <ticket-id> --to <member-id>
```

```bash
oh team claim transfer TICKET-123 --to bob
```

---

### oh team status

Affiche l'etat de l'equipe : claims actifs, membres, tickets en cours.

```
oh team status [options]
```

**Exemple :**

```bash
oh team status
```

---

### oh team activity

Affiche l'historique d'activite de l'equipe.

```
oh team activity [options]
```

**Exemple :**

```bash
oh team activity
```

---

### oh team board

Affiche le tableau kanban de l'equipe (claims par statut).

```
oh team board [options]
```

| Flag | Description |
|------|-------------|
| `--watch` | Rafraichissement auto toutes les 5s |

**Exemple :**

```bash
oh team board
oh team board --watch
```

---

### oh team sync-tracker

Synchronise les claims avec le tracker externe (GitLab/Jira).

```
oh team sync-tracker
```

Tire les etats des issues depuis le tracker, met a jour les statuts des claims, miroire les labels, et cree automatiquement des claims en statut `planned` pour les issues assignees non encore reclames. Utilise la configuration du `config.toml` team-state et la config hub MCP.

Pas de flags. La configuration est lue depuis `team-state/config.toml` et la config MCP du hub.

**Exemple :**

```bash
oh team sync-tracker
```

---


## Gestion d'equipes

## Gestion d'equipes

### oh teams list

Lister toutes les equipes configurees.

```bash
oh teams list
```

### oh teams add

Ajouter une nouvelle equipe.

```bash
oh teams add --repo git@github.com:org/team-state.git --member-id alice
```

| Flag | Description |
|------|-------------|
| `--repo` | URL du depot Git team-state |
| `--member-id` | Votre identifiant membre dans l'equipe |
| `--id` | Identifiant de l'equipe |
| `--name` | Nom d'affichage de l'equipe |

### oh teams remove

Supprimer une equipe.

```bash
oh teams remove <team-id>
```

### oh teams detach

Detacher un projet de son equipe.

```bash
oh teams detach <project-name>
```

### oh teams archive / restore

Archiver ou restaurer une equipe :

```bash
oh teams archive <team-id>
oh teams restore <team-id>
```

---

### oh team init

Initialiser les fonctionnalites equipe avec un assistant interactif.

```bash
oh team init
```

#### Espace solo (`--solo`)

Pour un projet sans équipe : crée un team-state local (dépôt git sans remote) dans `~/.oh/teams/<id>/`, qui accueille les workflows du projet. Le seul membre a le rôle `lead`. Les fonctions d'équipe (board, claims, notifications, serveur MCP `team`) restent désactivées ; l'équipe active n'est jamais un espace solo.

```bash
oh team init --solo --project web-app
```

| Flag | Description |
|------|-------------|
| `--solo` | Crée un espace solo au lieu de lancer l'assistant |
| `--id` | Identifiant de l'espace (défaut : `solo`) |
| `--name` | Nom affiché |
| `--member-id` | Identifiant du membre (défaut : celui d'une autre équipe, sinon l'utilisateur système) |
| `--project` | Projet à rattacher (id ou nom ; refusé s'il a déjà une équipe) |

Dans `hub.toml`, l'espace est ajouté aux équipes existantes avec `solo = true` (sans `state_repo`).

### oh team promote

Partage un espace solo : ajoute le remote et pousse tout l'historique. L'identifiant, le dossier, les projets rattachés et les workflows publiés sont conservés.

```bash
oh team promote --remote git@gitlab.com:acme/team-state.git
```

| Flag | Description |
|------|-------------|
| `--remote` | URL d'un dépôt git **vide** (obligatoire) |
| `--team` | Espace solo à partager, si vous en avez plusieurs |

En cas d'échec (dépôt absent, non vide, accès refusé), l'espace reste solo. Les autres membres rejoignent ensuite l'équipe avec `oh team init` en saisissant l'URL du dépôt.

### oh team config

Gerer la configuration equipe.

```bash
oh team config
oh team config status
```



### oh team notify test

Tester l'envoi de notifications.

```bash
oh team notify test
```

### oh team sync-tracker

Synchroniser les liens ExternalIID des claims vers le tracker externe (Jira, Linear, GitLab Issues).

```bash
oh team sync-tracker
```

---


---

### oh team wiki list

Lister les pages wiki et les propositions en attente.

```bash
oh team wiki list
```

### oh team wiki read

Lire une page wiki specifique.

```bash
oh team wiki read <nom-de-page>
```

### oh team wiki review

Reviser les propositions wiki en attente de maniere interactive.

```bash
oh team wiki review
oh team wiki review <proposal-id>
```

---

## Gouvernance

### oh conventions check

Valider le projet courant contre les conventions d'equipe.

```bash
oh conventions check
```

### oh patterns

Gerer la bibliotheque de patterns d'equipe.

```bash
oh patterns list               # lister tous les patterns
oh patterns show <nom>         # afficher un pattern
oh patterns add                # proposer un nouveau pattern
oh patterns validate           # valider tous les patterns
oh patterns remove <nom>       # supprimer un pattern
```

### oh policies

Gerer et verifier les policies d'equipe.

```bash
oh policies list               # lister les policies actives
oh policies check              # valider le projet contre les policies
oh policies add                # ajouter une nouvelle policy
```

---

## Takeover Briefs

### oh takeover-brief

Gerer les briefs de reprise pour les handoffs de tickets. Alias : `tb`.

```bash
oh takeover-brief show <ticket-id>    # voir le contexte de reprise
oh takeover-brief list                # lister les briefs disponibles
oh takeover-brief enrich <ticket-id>  # enrichir avec l'analyse du code
```

---

