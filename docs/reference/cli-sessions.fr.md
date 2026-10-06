> [Read in English](cli-sessions.en.md)

# Reference CLI — Sessions

> **v5 — alias dépréciés.** Ces commandes lancent leur workflow via [`oh run`](cli-workflows.fr.md#oh-run) et affichent un avertissement : `oh start` → `oh run feature` (`--prompt` = première entrée texte), `--agent <id>` → `oh run libre --agent <id>`, `--dev [-t <id>]` → `oh run ticket --tickets <id>` (une épopée choisie dans le sélecteur : une session pour toute l'épopée ou une par ticket, au choix), `--onboard` → `oh run onboarding`, `--parallel --tickets a,b` → `oh run ticket --tickets a,b` (sans `--tickets` : refusé), `--sweep <objectif>` → `oh run sweep -i goal=<objectif>`, `--worktree <branche>` → `--location new`, `--resume <id>` → `oh session attach <id> --how here`, `oh audit|review|debug` → `oh run audit|review|debug` (les options deviennent des entrées si le workflow les déclare), `oh review feedback` → `oh run review-feedback` (retours de la MR en entrée texte). Elles demandent opencode V2 et le workflow cible ; l'ancien lancement n'existe plus (voir le [guide de migration v5](../guides/migration-v5.fr.md)). Les sessions se suivent ensuite avec `oh session …` ou la vue **Sessions** de la TUI (voir [Sessions v5](../guides/sessions-v5.fr.md)).

## Sessions

### oh start

Alias déprécié de `oh run` (voir ci-dessus).

```
oh start [options]
```

| Flag | Court | Description | Équivalent v5 |
|------|-------|-------------|---------------|
| `--agent` | `-a` | Agent d'entrée | `oh run libre --agent <id>` (le prompt devient l'entrée `request`). Refusé avec `--dev`, `--onboard`, `--parallel` ou `--sweep` |
| `--assignee` | `-A` | Filtrer les tickets du sélecteur par assignee (requiert `--dev`, exclusif avec `--label`) | Pas d'option : passer les tickets avec `oh run ticket --tickets` |
| `--dev` | | Sélecteur épopées/tickets, puis `ticket` | `oh run ticket --tickets <id>` (épopée en une session : `--one-session`) |
| `--label` | `-l` | Filtrer les tickets du sélecteur par label (requiert `--dev`, exclusif avec `--assignee`) | Pas d'option : passer les tickets avec `oh run ticket --tickets` |
| `--onboard` | | Créer ou enrichir le wiki projet | `oh run onboarding` |
| `--parallel` | | Une session par ticket, exige `--tickets` (sinon refusé) | `oh run ticket --tickets a,b` |
| `--project` | `-p` | ID du projet (détection auto sinon) | `-p` |
| `--prompt` | `-m` | Prompt initial | Première entrée texte (`-i request=…` pour `feature`) |
| `--provider` | `-P` | Provider LLM (bedrock, anthropic, openai) | `-P` |
| `--recap` | | Afficher le récap et demander confirmation | `--recap` |
| `--refresh` | | Re-découvrir le wiki (requiert `--onboard`) | `oh run onboarding -i refresh=true` |
| `--resume` | `-r` | Ouvrir une session existante dans ce terminal | `oh session attach <id> --how here` |
| `--sweep` | | Objectif sweep haut niveau | `oh run sweep -i goal=<objectif>` |
| `--sweep-dry-run` | | Afficher le découpage sans exécuter | `-i dry_run=true` |
| `--sweep-exclude` | | Motifs glob à exclure | `-i exclude=<motifs>` |
| `--sweep-include` | | Motifs glob à inclure | `-i include=<motifs>` |
| `--sweep-strategy` | | Découpage : `manual`, `by-file`, `by-package`, `llm` | `-i strategy=<stratégie>` (défaut du workflow : `llm`) |
| `--sweep-tasks` | | Liste manuelle de tâches (`--sweep-strategy=manual`) | `-i tasks=<tâches>` (une par ligne) |
| `--sweep-verify` | | Vérification finale : `none`, `tests`, `lint`, `build`, `all`, `custom` | `-i verify=<valeur>` |
| `--sweep-verify-cmd` | | Commande de vérification (`--sweep-verify=custom`) | `-i verify_cmd=<commande>` |
| `--ticket` | `-t` | Ticket à travailler directement (saute le sélecteur, requiert `--dev`) | `oh run ticket --tickets <id>` |
| `--tickets` | | Tickets, séparés par des virgules (avec `--parallel`) | `--tickets` |
| `--worktree` | `-w` | Branche pour lancer dans un git worktree | `--location new` |

`--max-sessions`, `--priority` et `--sweep-branch-prefix` sont encore acceptés mais sans effet (masqués de l'aide). L'ancien sweep (un worktree par sous-tâche) et l'ancien mode parallèle (moniteur, vue de fusion) n'existent plus.

**Exemple :**

```bash
oh run feature -p mon-projet -i request="Ajoute un endpoint /health"
oh run libre --agent debugger -i request="Le test X échoue depuis hier"
oh session attach abc123-def456 --how here
oh run ticket --tickets TICKET-123 --location new
oh run onboarding -i refresh=true
oh run feature -i request="Refactorer le module auth" --recap
oh run ticket --tickets bd-42,bd-43,bd-44
oh run sweep -i goal="Migrer les appels API dépréciés" -i strategy=llm -i verify=tests
oh run sweep -i goal="Corriger les warnings lint" -i strategy=by-package -i dry_run=true

# Alias dépréciés équivalents
oh start -p mon-projet -m "Ajoute un endpoint /health"
oh start -a debugger -m "Le test X échoue depuis hier"
oh start --resume abc123-def456
oh start --dev -t TICKET-123 -w feat/ticket-123
oh start --onboard --refresh
oh start -m "Refactorer le module auth" --recap
oh start --parallel --tickets bd-42,bd-43,bd-44
oh start --sweep "Migrer les appels API dépréciés" --sweep-strategy llm --sweep-verify tests
oh start --sweep "Corriger les warnings lint" --sweep-strategy by-package --sweep-dry-run
```

> **Voir aussi :** [Workflows livrés](workflows.fr.md) | [Sessions v5](../guides/sessions-v5.fr.md) | [Mode parallèle (remplacé)](../guides/parallel-mode.fr.md) | [Mode sweep (remplacé)](../guides/sweep-mode.fr.md)

---

### oh audit

Alias déprécié de `oh run audit` (audit de code en lecture seule).

```
oh audit [options]
```

| Flag | Court | Description | Équivalent v5 |
|------|-------|-------------|---------------|
| `--project` | `-p` | ID du projet | `-p` |
| `--type` | `-t` | Type d'audit (défaut : security) | `-i type=<type>` |

Types disponibles : `security`, `performance`, `architecture`, `accessibility`, `ecodesign`, `observability`, `privacy`.

**Exemple :**

```bash
oh run audit -p api-gateway -i type=security
oh run audit -i type=performance
oh run audit -i type=ecodesign

# Alias déprécié équivalent
oh audit -p api-gateway -t security
```

---

### oh review

Alias déprécié de `oh run review` (review de code en lecture seule). `--publish` n'est pas un workflow : il reste une commande d'oh.

```
oh review [options]
```

| Flag | Court | Description | Équivalent v5 |
|------|-------|-------------|---------------|
| `--branch` | `-b` | Branche à reviewer (diff vs main). Défaut : branche courante si feature branch | `-i branch=<branche>` |
| `--mode` | `-m` | Mode de review (voir ci-dessous) | `-i review_mode=<mode>` |
| `--project` | `-p` | ID du projet | `-p` |
| `--publish` | | Créer une MR sur GitLab et optionnellement assigner un reviewer (requiert write_enabled) | Inchangé : `oh review --publish` |
| `--reviewer` | | Member ID du reviewer à assigner sur la MR (avec `--publish`) | Inchangé |

**Modes disponibles :**

| Mode | Description |
|------|-------------|
| `standard` | Review classique — checklist 6 catégories |
| `adversarial` | Critique approfondie — scepticisme maximal, min. 10 findings, hypothèses dangereuses |
| `edge-case` | Chasse aux chemins d'exécution non gérés |
| `standard+adversarial` | Les deux en parallèle (sessions indépendantes) + rapport unifié |
| `all` | Standard + Adversarial + Edge-case — couverture maximale |

Sans mode, le reviewer propose le choix au démarrage de la session.

**Exemple :**

```bash
oh run review -p frontend
oh run review -i review_mode=adversarial
oh run review -i review_mode=standard+adversarial -p backend
oh run review -i review_mode=all -i branch=feat/auth
oh review --publish --reviewer alice
oh review --publish -b feat/auth

# Alias déprécié équivalent
oh review -m adversarial
```

> **Voir aussi :** [Guide Review & Feedback](../guides/review-feedback.fr.md)

---

### oh review feedback

Récupère les discussions GitLab non résolues d'une MR, affiche un aperçu, demande confirmation, puis lance le workflow `review-feedback` (entrées `mr`, `branch` et `feedback` remplies par oh). Avertit que l'alias est déprécié.

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

> **Limites :** Max 30 discussions par session, 2000 caractères par note.

---

### oh debug

Alias déprécié de `oh run debug` (diagnostic d'un bug).

```
oh debug [options]
```

| Flag | Court | Description | Équivalent v5 |
|------|-------|-------------|---------------|
| `--issue` | `-i` | Description du problème | `-i issue=<description>` |
| `--project` | `-p` | ID du projet | `-p` |

**Exemple :**

```bash
oh run debug -p backend -i issue="Timeout sur les requêtes POST /api/users"
oh run debug -i issue="Memory leak dans le worker pool"

# Alias déprécié équivalent
oh debug -p backend -i "Timeout sur les requêtes POST /api/users"
```

---

### oh budget

Restrictions des sessions (désactivées par défaut) : sessions actives max, budget par session et journalier, plafond mémoire, modèles autorisés. Voir [Sessions v5 › Restrictions](../guides/sessions-v5.fr.md#restrictions).

```bash
oh budget show [-p <projet>] [--json]           # valeurs effectives, origine, dépenses du jour
oh budget set session_budget_usd 5              # hub.toml [limits]
oh budget set max_active_sessions 2 -p mon-app  # niveau projet
oh budget unset daily_budget_usd
oh budget raise <session> [montant]             # répondre à une décision $
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

> **Voir aussi :** [Workflows livrés](workflows.fr.md) | [Sessions v5](../guides/sessions-v5.fr.md) | [Guide Review & Feedback](../guides/review-feedback.fr.md)
