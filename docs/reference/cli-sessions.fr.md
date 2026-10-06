> [Read in English](cli-sessions.en.md)

# Reference CLI — Sessions

> **v5 — alias dépréciés.** Avec opencode V2 et les workflows du hub, ces commandes lancent leur workflow via [`oh run`](cli-workflows.fr.md#oh-run) et affichent un avertissement : `oh start` → `oh run feature` (`--prompt` = première entrée texte), `--dev [-t <id>]` → `oh run ticket --tickets <id>` (une épopée choisie dans le sélecteur : une session pour toute l'épopée ou une par ticket, au choix), `--onboard` → `oh run onboarding`, `--parallel --tickets` → `oh run ticket --tickets`, `--sweep` → `oh run sweep`, `--worktree <branche>` → `--location new`, `oh audit|review|debug` → `oh run audit|review|debug` (les options deviennent des entrées si le workflow les déclare), `oh review feedback` → `oh run review-feedback` (retours de la MR en entrée texte). `--agent` et opencode V1 gardent l'ancien lancement, de même qu'un workflow absent du catalogue.

## Sessions

### oh start

Lance une session opencode.

```
oh start [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--agent` | `-a` | Agent a utiliser |
| `--prompt` | `-m` | Prompt initial |
| `--provider` | `-P` | Provider LLM (bedrock, anthropic, openai) |
| `--project` | `-p` | ID du projet (detection auto sinon) |
| `--resume` | `-r` | Reprendre une session existante (ID de session) |
| `--worktree` | `-w` | Branche pour lancer dans un git worktree |
| `--dev` | | Mode dev : picker epics/tickets + orchestrator-dev |
| `--ticket` | `-t` | ID du ticket a travailler directement (skip le picker, requiert --dev) |
| `--label` | `-l` | Filtrer tickets par label (requiert --dev) |
| `--assignee` | `-A` | Filtrer tickets par assignee (requiert --dev) |
| `--onboard` | | Mode onboarding : cree/enrichit le wiki projet |
| `--refresh` | | Force la re-decouverte du wiki (requiert --onboard) |
| `--recap` | | Afficher le recap et demander confirmation |
| `--parallel` | | Lance N sessions en parallele sur des tickets differents |
| `--tickets` | | Liste des tickets a traiter en parallele (separes par des virgules) |
| `--max-sessions` | | Nombre max de sessions paralleles (0 = valeur config, defaut : 3, cap : 10) |
| `--priority` | | Ticket prioritaire (merge en premier) |
| `--sweep` | | Objectif sweep haut niveau (active le mode sweep) |
| `--sweep-strategy` | | Strategie de decomposition : `manual`, `by-file`, `by-package`, `llm` |
| `--sweep-tasks` | | Liste manuelle de taches (requiert `--sweep-strategy=manual`) |
| `--sweep-include` | | Glob patterns a inclure |
| `--sweep-exclude` | | Glob patterns a exclure |
| `--sweep-verify` | | Verification post-sweep : `none`, `tests`, `lint`, `build`, `all`, `custom` |
| `--sweep-verify-cmd` | | Commande de verification custom (requiert `--sweep-verify=custom`) |
| `--sweep-dry-run` | | Afficher le plan decompose sans executer |
| `--sweep-branch-prefix` | | Prefixe des branches sweep (defaut : `sweep/`) |

**Exemple :**

```bash
oh start -p mon-projet -a coder -m "Ajoute un endpoint /health"
oh start --resume abc123-def456
oh start --worktree feat/auth --dev -l "priority:high"
oh start --dev -t TICKET-123
oh start --onboard --refresh
oh start --parallel --tickets bd-42,bd-43,bd-44
oh start --parallel --tickets T-1,T-2,T-3 --priority T-1 --max-sessions 2
oh start --sweep "Migrer les appels API depreces" --sweep-strategy llm --sweep-verify tests
oh start --sweep "Corriger les warnings lint" --sweep-strategy by-package --sweep-dry-run
```

> **Voir aussi :** [Guide Mode parallele](../guides/parallel-mode.fr.md) | [Guide Mode sweep](../guides/sweep-mode.fr.md)

---

### oh audit

Lance un audit de code via opencode.

```
oh audit [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-p` | ID du projet |
| `--type` | `-t` | Type d'audit (defaut : security) |

Types disponibles : `security`, `performance`, `architecture`, `accessibility`, `ecodesign`, `observability`, `privacy`.

**Exemple :**

```bash
oh audit -p api-gateway -t security
oh audit --type performance
oh audit -t ecodesign
```

---

### oh review

Lance une review de code via opencode avec sélection du mode.

```
oh review [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-p` | ID du projet |
| `--mode` | `-m` | Mode de review (voir ci-dessous) |
| `--branch` | `-b` | Branche a reviewer (diff vs main). Defaut : branche courante si feature branch |
| `--publish` | | Creer une MR sur GitLab et optionnellement assigner un reviewer (requiert write_enabled) |
| `--reviewer` | | Member ID du reviewer a assigner sur la MR (utilise avec --publish) |

**Modes disponibles :**

| Mode | Description |
|------|-------------|
| `standard` | Review classique — checklist 6 catégories |
| `adversarial` | Critique approfondie — scepticisme maximal, min. 10 findings, hypothèses dangereuses |
| `edge-case` | Chasse aux chemins d'exécution non gérés |
| `standard+adversarial` | Les deux en parallèle (sessions indépendantes) + rapport unifié |
| `all` | Standard + Adversarial + Edge-case — couverture maximale |

Sans `--mode`, un prompt interactif propose le choix du mode au démarrage de la session.

**Exemple :**

```bash
oh review -p frontend
oh review -m adversarial
oh review -m standard+adversarial -p backend
oh review -m all
oh review --publish --reviewer alice
oh review --publish -b feat/auth
```

> **Voir aussi :** [Guide Review & Feedback](../guides/review-feedback.fr.md)

---

### oh review feedback

Lance une session de correction a partir des discussions de review MR. Recupere les discussions GitLab non resolues et ouvre une session IA pour traiter chaque commentaire.

```
oh review feedback <ticket-ou-branche>
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | ID du projet |
| `--yes` | | bool | Ignorer la confirmation |

```bash
oh review feedback TICKET-123
oh review feedback feat/ma-branche -p backend
oh review feedback TICKET-123 --yes
```

> **Limites :** Max 30 discussions par session, 2000 caracteres par note.

---

### oh debug

Lance une session de debug via opencode.

```
oh debug [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-p` | ID du projet |
| `--issue` | `-i` | Description du probleme |

**Exemple :**

```bash
oh debug -p backend -i "Timeout sur les requetes POST /api/users"
oh debug --issue "Memory leak dans le worker pool"
```

---

### oh beads

Proxy vers `bd` (Beads CLI). Tous les arguments sont passes directement a `bd`.

```
oh beads [arguments...]
```

Necessite `bd` installe et accessible dans le PATH.

**Exemple :**

```bash
oh beads list
oh beads run mon-bead
oh beads status
```

---

---

> **Voir aussi :** [Guide Mode parallele](../guides/parallel-mode.fr.md) | [Guide Mode sweep](../guides/sweep-mode.fr.md) | [Guide Review & Feedback](../guides/review-feedback.fr.md)
