> [Read in English](cli-sessions.en.md)

# Référence CLI — Sessions

Cette page couvre la **gestion des sessions v5** (`oh session …`), les **restrictions** (`oh budget`), l'**historique** (`oh history`), le proxy `oh beads` et la liste des **commandes de lancement retirées en 5.1** (`oh start`, `oh audit`, `oh review`, `oh debug`) avec leur remplaçant. Le lancement lui-même se fait avec [`oh run`](cli-workflows.fr.md#oh-run). Guide : [Sessions v5](../guides/sessions-v5.fr.md) ; décisions : [ADR-042](../architecture/adr/042-checkpoints-headless-decisions.fr.md), [ADR-047](../architecture/adr/047-session-interaction-daemon.fr.md).

## Gestion des sessions (v5)

« oh = tour de contrôle, opencode = cabine » : une session tourne sur un serveur `opencode serve` (un par **groupe** : version du paquet, projet, environnement d'exécution) ; fermer l'interface d'opencode ne coupe pas la session. Une session inactive depuis 5 min sans décision en attente passe **en veille** (serveur arrêté) ; elle reprend avec le même paquet (`attach`, `open` ou `resume`).

- **Désigner une session** : son identifiant complet ou un **préfixe unique** (avec ou sans `ses_`) ; `oh session list` les affiche.
- **Désigner une décision** (`approve`, `answer`, `dismiss`, `oh budget raise`) : l'identifiant de la décision (`oh session inbox`) ou celui de la session ; si la session a plusieurs décisions en attente du type visé, oh refuse et les liste.
- **Types de décisions** : ⏸ checkpoint, ? question, ! permission, $ budget, ✗ erreur, ⊘ coupe-circuit. La **première réponse gagne** (oh, interface d'opencode, navigateur, CLI) ; une réponse arrivée trop tard affiche qui a déjà décidé.
- **Annuler une session** : `oh session delete <session>` (elle est arrêtée, puis son worktree, sa branche locale et ses fichiers sont supprimés, après confirmation). `oh session stop` arrête sans rien supprimer : la session et son travail restent. Pour voir un lancement sans le démarrer : `oh run --dry-run`.
- Le suivi en direct, les notifications et les décisions passent par le démon `ohd` ([`oh daemon`](cli-infra.fr.md#oh-daemon)).

| Commande | Rôle |
|----------|------|
| [`oh session list`](#oh-session-list) | Sessions en cours, en attente, en veille (`--all` : aussi les terminées) |
| [`oh session inbox`](#oh-session-inbox) | Décisions en attente de toutes les sessions |
| [`oh session attach`](#oh-session-attach) | Ouvrir l'interface d'une session (reprend une session en veille) |
| [`oh session follow`](#oh-session-follow) | Suivre une session en direct, en lecture seule |
| [`oh session approve`](#oh-session-approve) | Répondre à une permission ou à un checkpoint ; `--batch` : plusieurs checkpoints d'un coup |
| [`oh fanout`](#oh-fanout) | Fan-out (expérimental) : état et pilote des sessions enfants |
| [`oh session answer`](#oh-session-answer) | Répondre à une question de l'agent |
| [`oh session dismiss`](#oh-session-dismiss) | Classer une alerte (erreur, coupe-circuit, budget) |
| [`oh session send`](#oh-session-send) | Envoyer une consigne courte |
| [`oh session interrupt`](#oh-session-interrupt) | Interrompre l'étape en cours |
| [`oh session compact`](#oh-session-compact) | Compacter l'historique |
| [`oh session model`](#oh-session-model) | Changer le modèle des prochaines étapes |
| [`oh session fork`](#oh-session-fork) | Créer une variante (copie de l'historique) |
| [`oh session results`](#oh-session-results) | Fichiers modifiés, branche, coût, description de MR, diff |
| [`oh session report`](#oh-session-report) | Rapports déposés par les agents (`workflow_report`) |
| [`oh session resume`](#oh-session-resume) | Reprendre une session en veille sans interface |
| [`oh session stop`](#oh-session-stop) | Arrêter une session (sans rien supprimer ; pour l'annuler : `oh session delete`) |
| [`oh session delete`](#oh-session-delete) | Supprimer définitivement une session (worktree, branche locale, fichiers) |
| [`oh session open`](#oh-session-open) | Ouvrir une session dans le navigateur |
| [`oh session fetch`](#oh-session-fetch) | Récupérer une session distante terminée |
| [`oh session resolve`](#oh-session-resolve) | Rejouer le journal Beads d'une session distante |

### oh session list

```
oh session list [--all] [--json]
```

Liste les sessions v5 : identifiant, projet, agent d'entrée, état, décisions en attente (badges), coût, début. Le coût est celui de **toute la session** (agent d'entrée et sous-agents), celui que compte le budget ; de même dans la vue Sessions, le flux en direct, les résultats et le récapitulatif « pendant votre absence ».

| Flag | Type | Défaut | Description |
|------|------|--------|-------------|
| `--all` | bool | `false` | Inclure les sessions terminées |
| `--json` | bool | `false` | Sortie JSON (sessions et leurs décisions en attente) |

### oh session inbox

```
oh session inbox [--json]
```

Décisions en attente de toutes les sessions : type (⏸ ? ! $ ✗ ⊘), identifiant de la décision, agent, résumé, ancienneté.

| Flag | Type | Défaut | Description |
|------|------|--------|-------------|
| `--json` | bool | `false` | Sortie JSON |

### oh session attach

```
oh session attach <session> [--attach <ouverture>] [--iterm-style tab|split|window]
```

Ouvre l'interface d'opencode sur la session (nouvel onglet ou fenêtre, tmux, navigateur ou terminal courant). Une session en veille est d'abord reprise (son serveur redémarre). Si aucun terminal ne peut être ouvert, l'interface s'ouvre dans le terminal courant. Si un client est déjà ouvert sur la session, sa fenêtre est ramenée au premier plan (rien n'est ouvert).

| Flag | Type | Défaut | Description |
|------|------|--------|-------------|
| `--attach` | string | `auto` | Ouverture, comme `oh run --attach` : `auto` (terminal courant d'abord : iTerm2 ou Terminal.app, puis l'autre, puis tmux si oh tourne dans tmux), `iterm`, `terminal`, `tmux`, `browser`, `suspend` (dans le terminal courant). `--how` (ancien nom) et la valeur `here` ont été retirés en 5.1 |
| `--iterm-style` | string | `tab` | Avec iTerm2 : `tab`, `split` ou `window` |
| `--exec` | bool | `false` | Interne, caché de l'aide : lance l'interface dans le processus courant (commande des fenêtres ouvertes par oh) |

```bash
oh session attach 7f3a
oh session attach 7f3a --attach suspend      # reprend une session (remplace oh start --resume <id>, retirée en 5.1)
oh session attach 7f3a --attach tmux
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

**Valideur** (`approver:`, CP-R9) : pour un checkpoint qui nomme ses valideurs, la commande affiche « Valideur attendu : … » ; si la personne qui valide (`user.name <user.email>` de git dans l'emplacement de la session) n'en fait pas partie, un avertissement sur la sortie d'erreur, et la validation est acceptée, tracée « hors valideur ». Avec `--batch`, une ligne d'avertissement par élément concerné, après le tableau (aussi avec `--dry-run`). Voir [Checkpoints](checkpoints.fr.md#valideur-humain-approver-cp-r9).

**Validation par lot** (CP-R6) : `oh session approve --batch` liste les checkpoints en attente (décision, session, checkpoint, résumé, résultat de la vérification lancée par oh) puis les valide un par un, chacun par le même chemin qu'une validation seule (une vérification échouée refuse celui-là seulement, qui reste en attente ; la commande finit en erreur et le dit).

| Flag | Type | Description |
|------|------|-------------|
| `--batch` | bool | Valider d'un coup les checkpoints en attente de la sélection |
| `--session` | string (répétable) | Seulement ces sessions |
| `--fanout` | string | Le fan-out d'une session (parente ou enfant) : la session parente et ses sessions enfants, merger et correction |
| `--checkpoint` | liste | Seulement ces checkpoints (`cp-2,cp-merge`) |
| `--except` | string (répétable) | Décision laissée en attente |
| `--reject` | string (répétable) | Décision refusée : `id` ou `id=consigne` |
| `--dry-run` | bool | Lister sans répondre |

```bash
oh session approve --batch --fanout ses_ab12 --dry-run
oh session approve --batch --checkpoint cp-2 --except checkpoint:ses_x:per_3 --reject checkpoint:ses_y:per_7="Ajoute le test d'erreur"
```

### oh fanout

```
oh fanout status <session> [--json]
oh fanout drive <session>
oh fanout clean <session> [--dry-run] [--discard] [--json]
```

Fan-out expérimental (`[fanout] experimental = true`, [guide](../guides/fan-out.fr.md)). `status` : branche d'intégration et son worktree, chaque session enfant (ticket, état `pending`, `running`, `merged`, `conflict`, `failed`, `budget`, session, commit de merge, détail), budget global dépensé et derniers événements ; pour une session enfant ou un merger, sa session parente, son ticket et sa branche. `drive` : pilote le fan-out d'une session parente (lance les sessions enfants une fois `cp-plan` passé, les suit, les merge, lance le merger sur un conflit, puis prévient la session parente) ; `oh run` le démarre en arrière-plan (journal `~/.oh/sessions/<session>/fanout.log`), il reprend où il en était, et suit le fan-out jusqu'au passage de `cp-merge` (tours de correction). Arrêté avant `cp-merge` (la session parente a fini son tour, machine redémarrée), le démon d'oh le relance de lui-même quand la session parente reprend ou finit une étape (au plus une fois par minute, événement `driver` du fan-out) ; `oh fanout drive <session>` le relance à la main. Un seul pilote par fan-out. `clean` : supprime les worktrees et branches `oh/fanout/…` d'un fan-out mergé ou dont la session parente est arrêtée (ce qu'oh fait déjà de lui-même, et `oh gc` pour les restes) : un worktree où une session travaille encore, ou avec des modifications non commitées, reste ; une branche ne part que si chacun de ses commits est sur une autre branche, sauf avec `--discard`, qui note le commit de chaque branche supprimée dans les événements du fan-out (`discarded`).

Dans `oh session list` et la TUI, une session enfant est marquée `↳` (avec son ticket), un merger `↳ … (merge <ticket>)`, une session de correction `↳ … (correction)`, une session parente `⑂` ; `oh session list --json` donne l'enregistrement `fanout`. `oh session trace` de la parente contient les événements du fan-out et les checkpoints passés dans chaque enfant (événements `fanout`, `child_checkpoint` avec `by: delegated` pour un `cp-2` délégué).

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

Envoie une consigne courte, prise en compte à la prochaine étape. Les mots après l'identifiant sont joints. L'outil admet la consigne sans garantir qu'elle sera lue : une session terminée (`completed`, `failed`, `stopped`) la refuse (« la consigne ne sera pas remise », code 1 ; pour continuer, `oh session attach`), et une session qui se termine pendant l'envoi est signalée (« la consigne n'a peut-être pas été prise en compte », `oh session trace`). Une session qui n'a pas encore démarré (`queued`, en attente d'un créneau libre, ou `preparing`) la refuse aussi (« la session n'a pas encore démarré… », code 1) : il n'y a pas encore de premier prompt à suivre ; réessayez quand elle tourne. La vue Sessions (`m`) refuse de même sur une session terminée ou pas encore démarrée.

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

Résultats de **toute la session**, sous la forme d'une description de MR prête à coller : titre, récapitulatif, fichiers modifiés (`+ajouts −suppressions`), coût total (sous-agents compris), branche. Les changements sont lus avec git dans le dossier de la session : commits et modifications depuis son départ (le commit de départ est enregistré au lancement ; pour une session plus ancienne, le dernier commit antérieur à son départ), sur sa branche même si le dossier est passé sur une autre, fichiers non suivis compris, quel que soit l'agent qui les a faits. Si le dossier n'existe plus, oh affiche le dernier instantané.

| Flag | Type | Description |
|------|------|-------------|
| `--mr` | bool | Description de MR seule (sans la note d'instantané) |
| `--patch` | bool | Diff complet |
| `--json` | bool | Sortie JSON (`results`, `live`) |

### oh session trace

```
oh session trace <session> [--json] [-o <fichier>]
```

Trace complète d'une session, dans l'ordre : prompts reçus, messages et appels d'outils de l'agent d'entrée et de ses sous-agents (chaque événement d'un sous-agent porte `in_subagent`), skills chargés, délégations, décisions (permissions accordées ou refusées avec `granted`, questions, checkpoints, budget) et leur réponse, chronologie des checkpoints (avec, pour chaque validation ou refus d'un humain, qui — `identity` — et, si le checkpoint nomme ses valideurs, `approver` et `approver_mismatch` : CP-R9), vérifications (les commandes de build, de tests et de lint de la stack détectée portent `verification`), appels refusés par les règles (`denied`), sorties déclarées ou relevées par oh, workflows proposés ensuite (`next`), coûts et tokens par tour, puis un résumé (totaux, par agent, registre d'usage d'oh).

Sources : la session et ses décisions enregistrées par oh, la chronologie des checkpoints tenue par le démon, et l'historique de l'outil, lu en direct tant que le serveur tourne (un instantané masqué est alors gardé dans `~/.oh/sessions/<id>/transcript.json`), sinon depuis ce dernier instantané. Les secrets sont masqués partout (jetons d'oh, clés AWS, Bedrock, GitHub, GitLab, Slack, JWT, clés privées, valeurs de `password`, `token`, `api_key`…, mots de passe des URL).

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Un événement JSON par ligne (JSONL) : `seq`, `time`, `kind` (`session`, `prompt`, `text`, `tool`, `skill`, `subagent`, `decision`, `checkpoint`, `usage`, `end`, `output`, `next`, `summary`) et les champs du type |
| `-o`, `--output` | string | Écrire la trace JSONL dans ce fichier |

C'est l'entrée des évaluations ([`oh eval`](cli-tools.fr.md#oh-eval)) et des rétrospectives.

### oh session next

```
oh session next <session>
```

Ce qui suit une session (R5) : ses sorties (déclarées par les agents ou relevées par oh) et les étapes [`next`](workflow-schema.fr.md#next--ce-que-oh-propose-ensuite-e4) de son workflow dont la condition est remplie, chacune avec la commande qui la lance (`oh run implement-spec --parent <session> --tickets … -i spec=…`). Rien n'est lancé. `oh run` affiche la même chose quand le client d'une session qu'il a ouverte se ferme ; la vue Sessions de la TUI met ces étapes en tête de « Enchaîner avec… ».

### oh session report

```
oh session report <session> [rp-id] [--json]
```

Rapports que les agents de la session ont déposés avec l'outil `workflow_report` ([serveur MCP `workflow`](cli-mcp.fr.md#serveur-mcp-workflow)). Sans identifiant : la liste (identifiant `rp-<n>`, auteur, titre, taille, heure, sections). Avec un identifiant : le rapport complet.

L'auteur est l'agent qu'oh a vu appeler l'outil, jamais une valeur donnée par l'agent. Les corps sont gardés dans `~/.oh/sessions/<session>/reports/` (fichiers 0600, dossier 0700) avec leur empreinte SHA-256 : un rapport modifié hors d'oh n'est pas affiché. Limites : 64 Ko par rapport, 100 rapports et 4 Mo par session ; les séquences de terminal et les caractères de contrôle sont retirés. Les rapports d'une session distante reviennent avec elle ([`oh session fetch`](#oh-session-fetch)). `oh gc` les garde aussi longtemps que l'historique de la session (comme ses résultats).

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Sortie JSON : la liste, ou le rapport avec son corps (`body`) |

Dans la TUI, la carte d'un checkpoint liste les rapports déposés depuis le checkpoint précédent (et ceux que l'agent cite) ; chacun s'ouvre dans un lecteur défilant.

### oh session resume

```
oh session resume <session>
```

Reprend une session en veille (redémarre son serveur, même paquet) sans ouvrir d'interface. Sans effet si elle tourne déjà.

### oh session stop

```
oh session stop <session>
```

Arrête la session, et son serveur si plus aucune autre session du groupe ne l'utilise. Le worktree temporaire d'une [review d'une autre branche](workflows.fr.md#review) est supprimé (la branche relue n'est pas touchée) ; tout autre worktree reste.

### oh session delete

```
oh session delete <session> [--yes] [--discard-base-changes]
```

Supprime définitivement une session, après la liste de ce qui part : la session est arrêtée (réservations d'équipe libérées), son worktree séparé supprimé (gardé, avec sa branche, si une autre session ouverte y travaille) et sa branche locale supprimée (le worktree temporaire d'une [review d'une autre branche](workflows.fr.md#review) part avec l'arrêt de la session, la branche relue n'est jamais supprimée), puis son enregistrement, ses décisions, son dossier `~/.oh/sessions/<id>` et, si aucune autre session n'utilise son groupe de serveurs, les données de l'outil du groupe. Les commits poussés nulle part et les fichiers non commités perdus sont listés avant la confirmation. **Jamais touchés** : la branche distante, le registre d'usage (dépenses). Une session qui travaillait dans le dossier du projet y laisse ses modifications non commitées, sauf `--discard-base-changes` (seconde question dans un terminal). Aussi proposé au lancement (`oh run --on-existing=delete`) et dans la vue Sessions de la TUI (`D`).

| Flag | Court | Type | Défaut | Description |
|------|-------|------|--------|-------------|
| `--yes` | `-y` | bool | `false` | Sans confirmation (obligatoire sans terminal : sinon le plan est affiché et la suppression refusée) |
| `--discard-base-changes` | | bool | `false` | Abandonner aussi les modifications non commitées d'une session qui travaillait dans le dossier du projet |

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
| `report` | `oh budget report [--by project\|workflow] [--since <durée>] [--json]` | Dépenses (USD, jetons) par projet ou par workflow depuis une durée (`7d`, `12h` ; défaut `30d`, `0` = depuis toujours ; la période compte en jours). Lit le registre d'usage joint aux sessions : une session supprimée (`oh session delete`, `oh gc`) garde ses dépenses, sous « inconnu » pour le workflow (le projet vient du registre) ; une session d'avant les workflows aussi |
| `raise` | `oh budget raise <session\|décision> [montant]` | Répond à une décision $ : ajoute le montant donné en USD au budget atteint, de la session ou du jour (défaut : le budget configuré une fois de plus) ; la session reprend dès que le budget couvre sa dépense : son état n'est plus « en attente » et une étape interrompue pendant la décision est relancée par une consigne de reprise d'oh |

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
oh budget report --by workflow --since 7d
```

---

## Historique

### oh history

Historique des sessions enregistrées par oh (`--team` : sessions terminées de votre membre, lues dans le team-state).

```
oh history [--limit 20] [--team]
oh history export [--output <fichier>] [--member-id <id>]
oh history import <fichier> [--member-id <id>]
```

| Commande / flag | Type | Défaut | Description |
|-----------------|------|--------|-------------|
| `--limit` | int | `20` | Nombre maximal d'entrées |
| `--team` | bool | `false` | Historique d'équipe (team-state) |
| `export --output` | string | `oh-history-<date>.json` | Fichier d'export (JSON) |
| `export --member-id` | string | membre de l'équipe active | `member_id` écrit dans le fichier |
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

## Commandes retirées en 5.1

`oh start`, `oh audit`, `oh review` (et `oh review feedback`), `oh debug` étaient des alias dépréciés de [`oh run`](cli-workflows.fr.md#oh-run) depuis la 5.0 ; ils sont **retirés en 5.1**. Le nom reste reconnu seulement pour afficher le remplaçant (message sur la sortie d'erreur, code de sortie 2) : `oh start -a debugger` répond « `oh start` a été retirée en 5.1 » et indique `oh run libre --agent <agent>`. Leurs options (`-p`, `-t`, `-m`, `-b`, `-i`, `-w`, `-l`…) disparaissent avec eux. Les entrées des workflows (types d'audit, modes de review) sont celles du workflow : voir [Workflows livrés](workflows.fr.md).

| Retirée | Remplacement |
|---------|--------------|
| `oh start [-m "…"]` | `oh run feature -m "…"` (le message est la première entrée texte) |
| `oh start --agent <id> -m "…"` | `oh run libre --agent <id> -m "…"` |
| `oh start --dev [-t <id>]` | `oh run ticket --tickets <id>` (sans `--tickets`, le sélecteur de la TUI) |
| `oh start --onboard [--refresh]` | `oh run onboarding [-i refresh=true]` |
| `oh start --parallel --tickets a,b` | `oh run ticket --tickets a,b` |
| `oh start --sweep "<objectif>"` | `oh run sweep -i goal="<objectif>"` |
| `oh start --worktree <branche>` | `oh run … --location new` |
| `oh start --resume <id>` | `oh session attach <id> --attach suspend` |
| `oh audit --type <type>` | `oh run audit -i type=<type>` |
| `oh review [--mode <mode>] [--branch <b>]` | `oh run review -i review_mode=<mode> -i branch=<b>` |
| `oh review --publish [--reviewer <id>]` | `oh mr create [--reviewer <id>]` |
| `oh review feedback <réf>` | `oh run review-feedback -i mr=<réf>` |
| `oh debug --issue "…"` | `oh run debug -i issue="…"` |
| `oh service` | `oh mcp status` |
| `oh service setup` | `oh mcp setup` |
| `oh service remove <service>` | `oh mcp disable <service>` (le jeton du trousseau se supprime avec `oh secrets delete`) |
| `oh takeover-brief enrich <ticket>` | `oh run brief-enrich --headless -i ticket=<ticket>` (l'enrichissement avec enregistrement du brief reste dans la vue Reprise de la TUI, touche `e`) |

Pas de remplaçant pour les options des anciens lancements parallèle et sweep qui n'avaient plus d'effet depuis la 5.0 (`--max-sessions`, `--priority`, `--sweep-branch-prefix`).

---

> **Voir aussi :** [Workflows livrés](workflows.fr.md) | [Sessions v5](../guides/sessions-v5.fr.md) | [Guide Review & Feedback](../guides/review-feedback.fr.md) | [Migration depuis la 5.0](../guides/migration-v5.fr.md#depuis-la-50-vers-la-51)
