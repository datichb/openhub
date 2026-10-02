> [Read in English](parallel-mode.en.md)

# Mode parallele — Guide

## Vue d'ensemble

Le mode parallele execute plusieurs sessions de codage IA simultanement, chacune dans un worktree git isole. Il est concu pour traiter des lots de tickets independants en meme temps, reduisant considerablement le temps total d'implementation.

---

## Quand l'utiliser

- **Lot de tickets independants** — Plusieurs correctifs ou fonctionnalites qui touchent des fichiers differents
- **Taches de migration** — Appliquer des changements sur plusieurs packages ou modules
- **Velocite de sprint** — Traiter un backlog de sprint entier en parallele

Le mode parallele n'est PAS recommande lorsque les tickets ont de fortes interdependances ou modifient les memes fichiers de maniere intensive.

---

## Demarrage rapide

```bash
oh start --parallel --tickets BD-42,BD-43,BD-44
```

Cela va :
1. Valider les tickets et verifier l'admission budgetaire
2. Creer un repertoire worktree frere pour chaque ticket
3. Lancer des sessions `opencode serve` isolees (ports 4100+)
4. Ouvrir le moniteur parallele TUI
5. Une fois toutes les sessions terminees, proposer une fusion interactive

---

## Reference des commandes

```bash
oh start --parallel --tickets BD-42,BD-43,BD-44 [options]
```

| Flag | Description | Defaut |
|------|-------------|--------|
| `--parallel` | Activer le mode parallele | — |
| `--tickets` | Identifiants de tickets separes par des virgules (requis) | — |
| `--max-sessions` | Nombre maximal de sessions simultanees (plafond : 10) | Defaut de la config (5) |
| `--priority` | Identifiant du ticket prioritaire (fusionne en premier) | — |
| `--project` / `-p` | Identifiant du projet | Detection automatique |
| `--yes` / `-y` | Ignorer les confirmations interactives | false |

---

## Flux de travail

### 1. Controle d'admission

Avant le lancement, le coordinateur valide :
- Le total des minutes estimees pour tous les tickets ne depasse pas `MaxBudgetMinutes` (defaut : 180)
- Les tickets sans estimation utilisent `DefaultTicketWeightMin` (defaut : 60 min) comme valeur de repli
- Le nombre de tickets ne depasse pas `MaxSessions`

### 2. Creation des worktrees

Chaque ticket obtient un **repertoire frere** a cote du projet principal :

```
/home/user/myrepo/                  <- projet principal
/home/user/myrepo-feat-bd-42/       <- worktree pour BD-42
/home/user/myrepo-feat-bd-43/       <- worktree pour BD-43
```

Le nommage des branches suit `[worktree].branch_pattern` de `hub.toml` (defaut : `feat/%s`).

### 3. Execution des sessions

Chaque session s'execute comme un sous-processus `opencode serve` independant :
- Ports isoles a partir de 4100 (configurable via `port_range_start`)
- Pile d'agents complete deployee par worktree
- Les sessions ne peuvent pas interferer avec le systeme de fichiers des autres

### 4. Moniteur TUI

Le moniteur parallele affiche toutes les sessions en temps reel :

| Touche | Action |
|--------|--------|
| Fleches Haut/Bas | Naviguer entre les sessions |
| Entree | Se connecter a une session (interactif) |
| `r` | Rafraichir le statut |
| `q` | Quitter le moniteur (les sessions continuent en arriere-plan) |

Informations affichees par session : statut, duree, fichiers modifies, severite des conflits.

### 5. Detection des conflits

Le coordinateur suit en temps reel les fichiers modifies par chaque session :
- **Faible** — Fichiers differents dans le meme repertoire
- **Moyen** — Meme fichier modifie par plusieurs sessions
- **Eleve** — Memes lignes modifiees par plusieurs sessions

Les sessions en cours sont notifiees lorsqu'un conflit est detecte.

### 6. Recuperation automatique

Les sessions echouees sont automatiquement relancees :
- Jusqu'a `max_retries` tentatives (defaut : 2, plafond : 5)
- Delai entre les tentatives : `retry_delay_seconds` (defaut : 5, plafond : 60)
- Les prompts de recuperation incluent le contexte de l'echec precedent

### 7. Fusion

Une fois toutes les sessions terminees, une vue de fusion interactive permet :
- La fusion sequentielle des branches completees dans la branche de base
- L'apercu du diff avant chaque fusion
- La resolution manuelle des conflits si necessaire
- Le ticket prioritaire (si specifie) est fusionne en premier

---

## Configuration d'equipe

Configurez les valeurs par defaut du mode parallele dans le `config.toml` du team-state :

```toml
[parallel]
max_sessions = 5                    # Sessions simultanees max (plafond : 10)
max_budget_minutes = 180            # Plafond budgetaire (0 = desactive)
default_ticket_weight_min = 60      # Poids de repli pour les tickets non estimes
port_range_start = 4100             # Port de depart pour opencode serve
auto_merge_beads = true             # Proposer l'auto-merge pour les tickets Beads
max_retries = 2                     # Plafond de tentatives (max : 5)
retry_delay_seconds = 5             # Delai entre les tentatives (max : 60)
```

---

## API REST et SSE

Lors de l'execution de `oh serve`, l'etat d'execution parallele est expose :
- **API REST** — Interroger le statut des sessions par programmation
- **Server-Sent Events (SSE)** — Flux de statut en temps reel pour les tableaux de bord

---

## Exemples

**Traiter 3 tickets avec priorite :**
```bash
oh start --parallel --tickets BD-42,BD-43,BD-44 --priority BD-42
```

**Limiter les sessions simultanees :**
```bash
oh start --parallel --tickets BD-42,BD-43,BD-44,BD-45 --max-sessions 2
```

**Ignorer la confirmation :**
```bash
oh start --parallel --tickets BD-42,BD-43 --yes
```

---

## Depannage

### Conflit de port

```
Error: port 4100 already in use
```

Changez le port de depart dans la configuration d'equipe (`port_range_start`) ou arretez le processus occupant le port.

### Budget depasse

```
Error: total estimated time (240 min) exceeds budget (180 min)
```

Reduisez le nombre de tickets, augmentez `max_budget_minutes`, ou definissez `max_budget_minutes = 0` pour desactiver la verification.

### Worktrees orphelins

Apres un crash, des worktrees peuvent rester en suspens :
```bash
oh worktree list               # Lister tous les worktrees
oh worktree cleanup            # Supprimer les worktrees fusionnes
oh worktree cleanup -f         # Suppression forcee (y compris les dirty)
```

### Conflits de fusion

Si deux sessions ont modifie les memes fichiers, la vue de fusion signalera le conflit. Resolvez manuellement dans le worktree avant de completer la fusion.

---

## Ressources

- [Documentation des worktrees](../worktree.md)
- [ADR-012 — Git Worktrees](../architecture/adr/012-git-worktree.fr.md)
- [ADR-036 — Abstraction de plateforme](../architecture/adr/036-platform-abstraction.fr.md)
- [Reference CLI](../reference/cli.fr.md)
