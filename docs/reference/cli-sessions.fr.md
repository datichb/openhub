> [Read in English](cli-sessions.en.md)

# Reference CLI — Sessions

Cette page couvre la **gestion des sessions v5** (`oh session …`), les **restrictions** (`oh budget`), l'**historique** (`oh history`), le proxy `oh beads` et les **alias dépréciés** de lancement (`oh start`, `oh audit`, `oh review`, `oh debug`). Le lancement lui-même se fait avec [`oh run`](cli-workflows.fr.md#oh-run). Guide : [Sessions v5](../guides/sessions-v5.fr.md) ; décisions : [ADR-042](../architecture/adr/042-checkpoints-headless-decisions.fr.md), [ADR-047](../architecture/adr/047-session-interaction-daemon.fr.md).

## Gestion des sessions (v5)

« oh = tour de contrôle, opencode = cabine » : une session tourne sur un serveur `opencode serve` (un par **groupe** : version du paquet, projet, environnement d'exécution) ; fermer l'interface d'opencode ne coupe pas la session. Une session inactive depuis 5 min sans décision en attente passe **en veille** (serveur arrêté) ; elle reprend avec le même paquet (`attach`, `open` ou `resume`).

- **Désigner une session** : son identifiant complet ou un **préfixe unique** (avec ou sans `ses_`) ; `oh session list` les affiche.
- **Désigner une décision** (`approve`, `answer`, `dismiss`, `oh budget raise`) : l'identifiant de la décision (`oh session inbox`) ou celui de la session ; si la session a plusieurs décisions en attente du type visé, oh refuse et les liste.
- **Types de décisions** : ⏸ checkpoint, ? question, ! permission, $ budget, ✗ erreur ou coupe-circuit. La **première réponse gagne** (oh, interface d'opencode, navigateur, CLI) ; une réponse arrivée trop tard affiche qui a déjà décidé.
- Le suivi en direct, les notifications et les décisions passent par le démon `ohd` ([`oh daemon`](cli-infra.fr.md#oh-daemon)).

| Commande | Rôle |
|----------|------|
| [`oh session list`](#oh-session-list) | Sessions en cours, en attente, en veille (`--all` : aussi les terminées) |
| [`oh session inbox`](#oh-session-inbox) | Décisions en attente de toutes les sessions |
| [`oh session attach`](#oh-session-attach) | Ouvrir l'interface d'une session (reprend une session en veille) |
| [`oh session follow`](#oh-session-follow) | Suivre une session en direct, en lecture seule |
| [`oh session approve`](#oh-session-approve) | Répondre à une permission ou à un checkpoint |
| [`oh session answer`](#oh-session-answer) | Répondre à une question de l'agent |
| [`oh session dismiss`](#oh-session-dismiss) | Classer une alerte (erreur, coupe-circuit, budget) |
| [`oh session send`](#oh-session-send) | Envoyer une consigne courte |
| [`oh session interrupt`](#oh-session-interrupt) | Interrompre l'étape en cours |
| [`oh session compact`](#oh-session-compact) | Compacter l'historique |
| [`oh session model`](#oh-session-model) | Changer le modèle des prochaines étapes |
| [`oh session fork`](#oh-session-fork) | Créer une variante (copie de l'historique) |
| [`oh session results`](#oh-session-results) | Fichiers modifiés, branche, coût, description de MR, diff |
| [`oh session resume`](#oh-session-resume) | Reprendre une session en veille sans interface |
| [`oh session stop`](#oh-session-stop) | Arrêter une session |
| [`oh session open`](#oh-session-open) | Ouvrir une session dans le navigateur |
| [`oh session fetch`](#oh-session-fetch) | Récupérer une session distante terminée |
| [`oh session resolve`](#oh-session-resolve) | Rejouer le journal Beads d'une session distante |

### oh session list

```
oh session list [--all] [--json]
```

Liste les sessions v5 : identifiant, projet, agent d'entrée, état, décisions en attente (badges), coût, début.

| Flag | Type | Défaut | Description |
|------|------|--------|-------------|
| `--all` | bool | `false` | Inclure les sessions terminées |
| `--json` | bool | `false` | Sortie JSON (sessions et leurs décisions en attente) |

### oh session inbox

```
oh session inbox [--json]
```

Décisions en attente de toutes les sessions : type (⏸ ? ! $ ✗), identifiant de la décision, agent, résumé, ancienneté.

| Flag | Type | Défaut | Description |
|------|------|--------|-------------|
| `--json` | bool | `false` | Sortie JSON |

### oh session attach

```
oh session attach <session> [--how <ouverture>] [--iterm-style tab|split|window]
```

Ouvre l'interface d'opencode sur la session (nouvel onglet ou fenêtre, tmux, navigateur ou terminal courant). Une session en veille est d'abord reprise (son serveur redémarre). Si aucun terminal ne peut être ouvert, l'interface s'ouvre dans le terminal courant.

| Flag | Type | Défaut | Description |
|------|------|--------|-------------|
| `--how` | string | `auto` | `auto` (terminal courant d'abord : iTerm2 ou Terminal.app, puis l'autre, puis tmux si oh tourne dans tmux), `iterm`, `terminal`, `tmux`, `browser`, `suspend` (dans le terminal courant ; `here` est accepté comme synonyme) |
| `--iterm-style` | string | `tab` | Avec iTerm2 : `tab`, `split` ou `window` |

```bash
oh session attach 7f3a
oh session attach 7f3a --how here      # remplace oh start --resume <id>
oh session attach 7f3a --how tmux
```

### oh session follow

```
oh session follow <session>
```

Suit une session en direct, en lecture seule : agent courant, outils, messages ; le coût s'affiche à la fin. `Ctrl+C` pour quitter (la session continue). Exige le démon oh (sinon : « suivi en direct indisponible »).

### oh session approve

```
oh session approve <session|décision> [--decision <choix>] [-m "<message>"]
```

Répond à une **permission** (!) ou à un **checkpoint** (⏸) en attente, sans ouvrir la session.

| Flag | Court | Type | Défaut | Description |
|------|-------|------|--------|-------------|
| `--decision` | | string | `once` | Permission : `once` (autoriser cette fois), `always` (toujours, refusé si la session est en isolation stricte), `reject`. Checkpoint : `once` (valider), `fix` (corriger d'abord), `other` (autre consigne), `reject` |
| `--message` | `-m` | string | | Message transmis à l'agent ; **obligatoire** avec `fix` et `other` |

```bash
oh session approve 7f3a                                  # permission ou checkpoint : once
oh session approve dec_91 --decision reject
oh session approve 7f3a --decision fix -m "Ajoute d'abord les tests"
```

### oh session answer

```
oh session answer <session|décision> --field clé=valeur…
```

Répond à une **question** (?) de l'agent (formulaire). Sans `--field`, oh affiche les champs attendus (type, obligatoire, choix possibles) et sort en erreur.

| Flag | Type | Description |
|------|------|-------------|
| `--field` | string (répétable) | Réponse `clé=valeur` ; pour une liste : `clé=a,b` |

```bash
oh session answer 7f3a --field scope=api --field targets=auth,users
```

### oh session dismiss

```
oh session dismiss <session|décision>
```

Classe une alerte : ✗ erreur, ✗ coupe-circuit (remet le compteur de délégations à zéro), $ budget (une étape de plus est permise, puis la décision revient ; pour relever le budget : [`oh budget raise`](#oh-budget)).

### oh session send

```
oh session send <session> "consigne…" [--queue] [--synthetic]
```

Envoie une consigne courte, prise en compte à la prochaine étape. Les mots après l'identifiant sont joints.

| Flag | Type | Défaut | Description |
|------|------|--------|-------------|
| `--queue` | bool | `false` | Après l'étape en cours (au lieu de la prochaine étape) |
| `--synthetic` | bool | `false` | Message d'oh (non utilisateur) |

### oh session interrupt

```
oh session interrupt <session>
```

Interrompt l'étape en cours ; la session reste ouverte.

### oh session compact

```
oh session compact <session>
```

Compacte l'historique de la session.

### oh session model

```
oh session model <session> <fournisseur/modèle>
```

Change le modèle des prochaines étapes (ex. `amazon-bedrock/eu.anthropic.claude-sonnet-4-5`). Les restrictions de modèles (`limits.models`) s'appliquent.

### oh session fork

```
oh session fork <session>
```

Crée une variante de la session : copie de son historique, sur le même serveur. Affiche l'identifiant de la nouvelle session.

### oh session results

```
oh session results <session> [--mr | --patch | --json]
```

Résultats : récapitulatif, branche, fichiers modifiés (`+ajouts −suppressions`), coût. Si le serveur ne tourne plus, oh affiche le dernier instantané.

| Flag | Type | Description |
|------|------|-------------|
| `--mr` | bool | Description de MR (Markdown) |
| `--patch` | bool | Diff complet |
| `--json` | bool | Sortie JSON (`results`, `live`) |

### oh session resume

```
oh session resume <session>
```

Reprend une session en veille (redémarre son serveur, même paquet) sans ouvrir d'interface. Sans effet si elle tourne déjà.

### oh session stop

```
oh session stop <session>
```

Arrête la session, et son serveur si plus aucune autre session du groupe ne l'utilise.

### oh session open

```
oh session open <session> [--browser] [--print]
```

Ouvre la session dans le navigateur avec un code d'appairage à usage unique (valable 5 min) ; l'URL est aussi affichée. Une session en veille est d'abord reprise.

| Flag | Type | Défaut | Description |
|------|------|--------|-------------|
| `--browser` | bool | `true` | Ouvrir dans le navigateur (seul mode disponible) |
| `--print` | bool | `false` | Afficher l'URL sans ouvrir le navigateur |

### oh session fetch

```
oh session fetch <session> [--no-import]
```

Session distante terminée ([exécution distante](../guides/remote-runners.fr.md)) : télécharge les artefacts du pipeline `oh-runner` (journal Beads, résumé, export de la session) et importe la session dans un serveur local (worktree de la branche poussée par le job), pour la reprendre avec `oh session attach`. Le journal Beads n'est pas rejoué ici : voir `oh session resolve`.

| Flag | Type | Défaut | Description |
|------|------|--------|-------------|
| `--no-import` | bool | `false` | Télécharger les artefacts sans importer la session |

### oh session resolve

```
oh session resolve <session> [--dry-run] [--yes] [--all keep-local|apply-remote|merge-notes]
                             [--keep-local a,b] [--apply-remote a,b] [--merge-notes a,b]
```

Rejoue sur la machine le journal Beads d'une session distante récupérée : chaque écriture est revérifiée (`beads.allow` du workflow, options refusées, tickets de la session) puis appliquée après confirmation. Un ticket modifié sur la machine depuis l'envoi est un **conflit** : garder la version locale, appliquer la version distante ou fusionner seulement les notes. Relançable : seules les écritures restantes sont appliquées.

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--dry-run` | | bool | Afficher le rejeu sans rien appliquer |
| `--yes` | `-y` | bool | Appliquer sans confirmation |
| `--all` | | string | Résolution de tous les conflits : `keep-local`, `apply-remote` ou `merge-notes` |
| `--keep-local` | | liste | Tickets en conflit dont la version locale est gardée |
| `--apply-remote` | | liste | Tickets en conflit dont les écritures distantes sont appliquées |
| `--merge-notes` | | liste | Tickets en conflit dont seules les notes sont fusionnées |

---

## Restrictions

### oh budget

Restrictions des sessions, **désactivées par défaut** : sessions actives max, budget par session et journalier (USD), plafond mémoire, modèles autorisés. Cascade hub → équipe (recommandé ou imposé) → projet → workflow (`limits:`). Voir [Sessions v5 › Restrictions](../guides/sessions-v5.fr.md#restrictions) et l'[ADR-044](../architecture/adr/044-credential-proxy-session-limits.fr.md).

| Sous-commande | Usage | Description |
|---------------|-------|-------------|
| `show` | `oh budget show [-p <projet>] [--json]` | Restrictions effectives avec leur origine, et dépenses du jour. `-p` : projet (défaut : celui du dossier courant) |
| `set` | `oh budget set <restriction> <valeur> [-p <projet>]` | Règle une restriction du hub (`hub.toml [limits]`) ou, avec `-p`, d'un projet |
| `unset` | `oh budget unset <restriction> [-p <projet>]` | Retire une restriction du hub ou d'un projet |
| `raise` | `oh budget raise <session\|décision> [montant]` | Répond à une décision $ : ajoute le montant donné en USD au budget atteint, de la session ou du jour (défaut : le budget configuré une fois de plus) ; la session reprend dès que le budget couvre sa dépense |

| Restriction | Valeur | Note |
|-------------|--------|------|
| `max_active_sessions` | entier | Sessions actives en même temps |
| `session_budget_usd` | montant USD | Budget par session |
| `daily_budget_usd` | montant USD | Budget journalier |
| `memory_mb` | entier (Mo) | Plafond mémoire ; **hub seulement** (refusé avec `-p`) |
| `models` | liste séparée par des virgules | Modèles autorisés |

`0`, `off` ou une valeur vide désactivent une restriction.

```bash
oh budget show --json
oh budget set session_budget_usd 5
oh budget set max_active_sessions 2 -p mon-app
oh budget set models amazon-bedrock/eu.anthropic.claude-sonnet-4-5,amazon-bedrock/eu.anthropic.claude-haiku-4-5
oh budget unset daily_budget_usd
oh budget raise 7f3a 3
```

---

## Historique

### oh history

Historique des sessions enregistrées par oh (`--team` : sessions terminées de votre membre, lues dans le team-state).

```
oh history [--limit 20] [--team]
oh history export [--output <fichier>]
oh history import <fichier> [--member-id <id>]
```

| Commande / flag | Type | Défaut | Description |
|-----------------|------|--------|-------------|
| `--limit` | int | `20` | Nombre maximal d'entrées |
| `--team` | bool | `false` | Historique d'équipe (team-state) |
| `export --output` | string | `oh-history-<date>.json` | Fichier d'export (JSON) |
| `import --member-id` | string | | Remplace le `member_id` des sessions importées |

---

## Beads

### oh beads

Proxy vers `bd` (Beads CLI) : tous les arguments sont passés tels quels à `bd`. Pour `bd init`, oh ajoute les options « zéro impact » (pas de hooks, pas de fichiers d'agent, `.gitignore` inchangé).

```
oh beads [arguments...]
```

Nécessite `bd` installé et accessible dans le PATH. Si `bd` échoue, oh sort avec le code 1.

```bash
oh beads list
oh beads show bd-42
oh beads ready
```

---

## Alias dépréciés

> **v5 — alias dépréciés.** Ces commandes lancent leur workflow via [`oh run`](cli-workflows.fr.md#oh-run) et affichent un avertissement : `oh start` → `oh run feature` (`--prompt` = première entrée texte), `--agent <id>` → `oh run libre --agent <id>`, `--dev [-t <id>]` → `oh run ticket --tickets <id>` (une épopée choisie dans le sélecteur : une session pour toute l'épopée ou une par ticket, au choix), `--onboard` → `oh run onboarding`, `--parallel --tickets a,b` → `oh run ticket --tickets a,b` (sans `--tickets` : refusé), `--sweep <objectif>` → `oh run sweep -i goal=<objectif>`, `--worktree <branche>` → `--location new`, `--resume <id>` → `oh session attach <id> --how here`, `oh audit|review|debug` → `oh run audit|review|debug` (les options deviennent des entrées si le workflow les déclare), `oh review feedback` → `oh run review-feedback` (retours de la MR en entrée texte). Elles demandent opencode V2 et le workflow cible ; l'ancien lancement n'existe plus (voir le [guide de migration v5](../guides/migration-v5.fr.md)). Les sessions se suivent ensuite avec `oh session …` ou la vue **Sessions** de la TUI (voir [Sessions v5](../guides/sessions-v5.fr.md)).

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

`--max-sessions`, `--priority` et `--sweep-branch-prefix` sont encore acceptés mais sans effet (masqués de l'aide), comme `-y, --yes` (le lancement direct est le défaut ; `--recap` pour confirmer). L'ancien sweep (un worktree par sous-tâche) et l'ancien mode parallèle (moniteur, vue de fusion) n'existent plus.

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

> **Voir aussi :** [Workflows livrés](workflows.fr.md) | [Sessions v5](../guides/sessions-v5.fr.md) | [Guide Review & Feedback](../guides/review-feedback.fr.md)
