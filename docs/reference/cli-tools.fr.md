> [Read in English](cli-tools.en.md)

# Reference CLI — Outils

## Skills

`oh skill` vérifie le catalogue des skills. Les skills d'une session sont celles du hub et du catalogue d'équipe (`catalog/skills/` du team-state), livrées par son paquet de session (voir [`oh bundle show`](cli-workflows.fr.md#oh-bundle-build--show), qui donne aussi le budget avec `--budget`).

> **Déconnecté en v5** ([ADR-051](../architecture/adr/051-community-skills-disconnection.fr.md)) : le registre de skills communautaires (`oh skill add`, `list`, `remove`, `search`) et `oh skill budget` sont supprimés, et `~/.oh/skills` n'est plus lu. `oh doctor` signale les paquets qui y restent.

### oh skill check

Vérifie les skills du hub (`skills/**/*.md`, hors annexes de `skills/templates/`) et les skills nommées par les agents du hub. Sort avec le code 1 en cas d'erreur.

Avec une équipe active (espace solo compris), il applique aussi la règle `unknown_key` aux skills du catalogue d'équipe (`catalog/skills/**/*.md` du team-state, hors `templates/`) : leurs problèmes portent le préfixe `team:` (`team:<chemin>`), et la clé `extends` (remplacement d'une skill du hub, `extends: hub:<ref>`) y est permise. Un frontmatter illisible y est un `invalid_frontmatter`. Les règles de description s'y appliquent en avertissement : `description_not_trigger`, et `long_description` au-delà de 200 caractères (erreur au-delà de 1024, limite du standard).

| Type | Niveau | Problème |
|------|--------|----------|
| `duplicate_id` | erreur | Deux skills de même identifiant (dernier élément du chemin) |
| `missing_require` | erreur | `requires:` nomme une skill introuvable |
| `require_cycle` | erreur | Cycle de `requires:` |
| `invalid_frontmatter` | erreur | Frontmatter qui n'est pas du YAML valide |
| `name_mismatch` | erreur | `name:` différent du nom du fichier |
| `invalid_name` | erreur | `name:` de plus de 64 caractères ou pas en kebab-case (minuscules, chiffres, tirets simples) |
| `long_description` | erreur | `description:` de plus de 200 caractères (limite du catalogue du hub ; avertissement pour une skill du catalogue d'équipe, erreur au-delà de 1024, limite du standard Agent Skills) |
| `unknown_key` | erreur | Clé de frontmatter qui n'est ni une clé Agent Skills (`name`, `description`, `license`, `compatibility`, `metadata`, `allowed-tools`) ni une clé du hub (`requires`, `annexes`, `plugin`, `mcp`, `when`, `statuses`, `versions`, `reviewed`) |
| `invalid_when` | erreur | Condition `when:` ou marqueurs `<!-- oh:when … -->` invalides : clé ou rôle inconnu, marqueur qui n'est pas seul sur sa ligne, blocs non équilibrés |
| `missing_annex` | erreur | `annexes:` nomme un fichier introuvable |
| `missing_freshness` | erreur | Skill de stack (`developer/stacks/`) sans `versions:` ou sans `reviewed:` (voir [Fraîcheur des skills de stack](../guides/skill-refresh.fr.md)) |
| `invalid_freshness` | erreur | `versions:` vide, clé qui n'est pas en minuscules (`react`, `spring-boot`) ou version vide ; `reviewed:` qui n'est pas une date `AAAA-MM-JJ` ou qui est dans le futur |
| `no_description` | avertissement | `description:` absente |
| `legacy_bucket` | avertissement | Champ `bucket:` obsolète |
| `missing_agent_skill` | avertissement | Skill nommée dans les `skills:` ou `native_skills:` d'un agent mais introuvable |
| `undeclared_annex` | avertissement | Gabarit cité dans le texte mais absent d'`annexes:` |
| `orphan_annex` | avertissement | Fichier de `skills/templates/` utilisé par aucune skill |
| `description_not_trigger` | avertissement | `description:` qui ne dit pas quand charger la skill : elle doit commencer par « Utiliser … » (« Utiliser quand… », « Utiliser pour… ») ; les skills générées depuis le workflow sont exclues |

Seules les clés Agent Skills arrivent dans le `SKILL.md` livré ; les clés du hub sont lues par oh (`plugin:` et `mcp:` ne livrent la skill que si le workflow charge ce plugin ou si la session a ce serveur MCP). Voir [Format d'un skill](../architecture/skills.fr.md#format-dun-skill).

```
oh skill check [--json] [--hub <dossier>]
oh skill check --stale [--max-age 90d] [--strict] [--json] [--hub <dossier>]
```

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Problèmes au format JSON (`kind`, `error`, `ref`, `detail`) ; avec `--stale`, le rapport de fraîcheur |
| `--hub` | chemin | Dossier du hub à vérifier, par exemple une copie de travail du dépôt (`--hub .`) ; par défaut, le hub installé (`~/.oh/hub`) |
| `--stale` | bool | Liste les skills de stack à revoir au lieu de vérifier le catalogue |
| `--max-age` | durée | Avec `--stale` : âge maximal d'une revue, en jours (`90`, `90d`) ou en semaines (`12w`) ; défaut `90d` |
| `--strict` | bool | Avec `--stale` : code de sortie 1 quand une skill est à revoir (tâche planifiée, CI) |

#### oh skill check --stale

Liste les skills de stack (`developer/stacks/`, et toute autre skill qui porte `versions:` ou `reviewed:`) dont la dernière revue (`reviewed:`) date de plus de `--max-age` jours, et celles dont les métadonnées manquent ou sont illisibles : les secondes d'abord, puis la revue la plus ancienne d'abord, avec les versions couvertes. Sans `--strict`, le code de sortie reste 0. Aucun appel réseau : la commande compare des dates, pas les versions publiées.

```
$ oh skill check --stale --hub .
Skills de stack à revoir (41 vérifiées, revue tous les 90 jours au plus, aujourd'hui 2026-10-13) :

  ! developer/stacks/dev-standards-cypress : revue le 2026-04-30 (il y a 166 jours), couvre cypress 12+
  ! developer/stacks/dev-standards-kubernetes : revue le 2026-04-30 (il y a 166 jours), couvre kubernetes 1.21+

  2 skill(s) à revoir : oh run skill-refresh (docs/guides/skill-refresh.fr.md)
```

Avec `--json` : `today`, `max_age_days`, `checked` (skills vérifiées) et `skills` (les skills à revoir : `ref`, `status` `stale` ou `missing`, `reviewed`, `age_days`, `versions`, `detail`). Le défaut de 90 jours correspond à une revue par trimestre ; la mise à jour se fait avec le workflow [`skill-refresh`](../guides/skill-refresh.fr.md).

---

## Évaluations

### oh eval

Mesure le comportement des workflows livrés avec un vrai modèle, avant et après un changement de contenu. Chaque cas de `evals/<workflow>/<cas>/` décrit une session (workflow, mode, entrées, tickets Beads), une fixture (le projet de départ), d'éventuels tests cachés et les assertions à vérifier sur la trace de la session ([`oh session trace`](cli-sessions.fr.md#oh-session-trace)). Le format des cas est décrit dans [`evals/README.fr.md`](../../evals/README.fr.md).

```
oh eval [--case S1,S2] [--reps N] [--models m1,m2] [--budget USD] [--parallel N] [--budget-total USD] [--out <dossier>] [--baseline <fichier>] [--record-baseline <fichier>] [--variant <nom>[=<définition>]]… [--rescore <dossier>] [--list] [--strict]
        [--tiered [--stable-measures N] [--history <report.json>]…] [--reps-case S1=1,S5=3] [--escalate] [--plan [--costs <report.json>]…]
```

Chaque passage tourne isolé : un `OH_HOME` et un `HOME` temporaires (vos sessions, votre base, votre trousseau et votre démon ne sont jamais touchés), un clone jetable de la fixture, un démon oh dédié. La session est lancée sans interface ; ses checkpoints, permissions et questions reçoivent les réponses écrites dans le cas, par l'API de décisions d'oh (`answers.wiki` : `accept`, `refuse` ou `pending` décide d'abord les propositions du wiki de la carte d'un checkpoint de commit ; `answers.sequence` donne, par checkpoint, les réponses de ses premières demandes, dans l'ordre). `previous:` enregistre avant le lancement des sessions terminées du projet avec leurs sorties (`{ workflow: spec, outputs: { tickets: [ev-1] } }`), lues par les sources `session.output(…)`. Une validation refusée parce que la vérification d'oh a échoué devient un refus avec consigne, comme le ferait l'utilisateur : une évaluation ne passe jamais outre. Une fois la session terminée (inactive sans décision en attente), oh lit sa trace, copie les tests cachés sur le clone et vérifie les assertions.

| Assertion | Vérifie |
|-----------|---------|
| `ran` | Une commande shell a tourné (avant un événement avec `before`, ou jamais avec `absent`) ; les appels refusés ne comptent pas |
| `checkpoint_order` | Les checkpoints sont atteints dans cet ordre |
| `no_question_in_subagent` | Aucun sous-agent (ou ceux d'`agents`) n'appelle l'outil `question` ; `include_entry` vérifie aussi l'agent d'entrée |
| `skill_loaded_before` | Un skill est chargé (avant un événement avec `before`) ; un chargement en échec (skill absent du bundle) ne compte pas |
| `changed_within` | Le clone n'a changé que dans `paths` (`max_files`, `min_files` ; chaque motif d'`include` correspond à au moins un fichier changé ; rien du tout sans paramètre) |
| `called` | Un outil a été appelé (motif de `tool`, par exemple `*wiki/propose` ; avant un événement avec `before`, ou jamais avec `absent`, restreint à `agent`) ; les appels refusés ne comptent pas |
| `hidden_tests` | Les tests cachés passent sur le clone |
| `no_denied_loop` | Un appel refusé (règle, permission refusée, skill inconnu) n'est pas répété plus de `max_repeats` fois |
| `text_matches` | Un agent a écrit (ou jamais, avec `absent`) un texte ; `questions` regarde aussi les questions posées |
| `file_matches` | Un fichier du clone modifié depuis la fixture et couvert par `paths` contient `pattern` (ou aucun, avec `absent`) : la spec écrite sous `docs/wiki/specs/`, par exemple |
| `output` | La session a la sortie `output` (déclarée, ou relevée par oh) et sa valeur correspond à `pattern` (facultatif ; listes jointes par des virgules), ou ne l'a pas avec `absent` |
| `fanout` | La trace contient au moins `count` (1 par défaut) événements du fan-out de genre `event` (`child`, `merged`, `conflict`, `merger`, `resolved`, `ready`, `integrated`, `closed`, `budget`, `child_checkpoint`…), restreints par `checkpoint` et `by` pour `child_checkpoint` (`by: delegated` : `cp-2` passé par délégation) ; aucun avec `absent`. Un cas en fan-out porte `fanout: { parallel: N }` : le passage active le fan-out, son pilote tourne dans le passage, les décisions des enfants reçoivent les réponses du cas, leur coût compte, et leurs traces sont gardées (`trace-<session>.jsonl`) |
| `next` | oh propose `workflow` après la session (`next:` de son workflow, événement `next` ajouté à la trace), avec des entrées qui correspondent à `pattern` (`tickets=ev-1,ev-2 spec=…`) ; jamais avec `absent` |

Le modèle est Haiku par défaut ; `OH_EVAL_MODELS` (séparés par des virgules) en ajoute d'autres, par exemple `claude-sonnet-4-6`. Chaque passage est plafonné par le budget de session qu'applique le proxy d'identifiants (`--budget` ou `OH_EVAL_BUDGET_USD` ; 1 $ par défaut, 2,50 $ dès qu'un modèle Sonnet fait partie des modèles) : au-delà, la session est arrêtée et le passage compte comme `budget`. `--budget-total` (ou `OH_EVAL_BUDGET_TOTAL`) plafonne toute l'évaluation : une fois ce montant dépensé, aucun nouveau passage ne démarre, ceux en cours finissent, et le rapport liste les passages non lancés. La clé Bedrock vient de `OH_EVAL_BEDROCK_TOKEN`, sinon d'`opencode auth`.

`--parallel N` (ou `OH_EVAL_PARALLEL`) lance jusqu'à N passages en même temps (1 par défaut). Chaque passage est un processus `oh` à part, avec ses propres `OH_HOME`, `HOME` et `TMPDIR`, son clone, son démon (socket dans son `OH_HOME`), son proxy d'identifiants et son serveur de l'outil sur des ports choisis par le système ; rien n'est partagé entre passages hormis les caches Go. Les rapports sont les mêmes qu'en séquentiel : passages triés par cas, modèle et répétition. Sur la console, `→` signale un passage lancé et `[k/N]` chaque passage terminé.

Un passage arrêté par une limite de débit du fournisseur (throttling Bedrock : `ThrottlingException`, « Too many requests/tokens », HTTP 429 venu du fournisseur et non du plafond du proxy) est relancé sur un clone neuf, jusqu'à trois fois, après 30 s, 1 min puis 2 min. Les fichiers des tentatives abandonnées restent dans `passes/<cas>/<modèle>/<n>/attempt-<k>/` ; le rapport les liste (« Relances ») et compte leur coût dans le coût total. Tout autre échec n'est pas relancé.

| Flag | Type | Description |
|------|------|-------------|
| `--cases` | string | Dossier des cas (défaut : `evals/` ou `../evals`) |
| `--case` | strings | Cas à lancer (défaut : tous les cas actifs) |
| `--models` | strings | Modèles (défaut : `OH_EVAL_MODELS`, sinon Haiku) |
| `--reps` | int | Répétitions par cas et par modèle (défaut : `OH_EVAL_REPS`, sinon 3 avec `--tiered`, 1 sinon) |
| `--budget` | float | Plafond de coût d'un passage, en dollars |
| `--parallel` | int | Nombre de passages lancés en même temps (défaut : `OH_EVAL_PARALLEL`, sinon 1) |
| `--budget-total` | float | Plafond de coût de toute l'évaluation, en dollars (défaut : `OH_EVAL_BUDGET_TOTAL`, sinon aucun) |
| `--out` | string | Dossier des traces (`passes/<cas>/<modèle>/<n>/trace.jsonl`) et des rapports `report.json` et `report.md` |
| `--baseline` | string | Mesure de référence à comparer (colonne « Référence » du rapport) |
| `--record-baseline` | string | Enregistrer ce passage comme mesure de référence |
| `--note` | string | Note enregistrée avec le rapport |
| `--keep` | bool | Garder les dossiers de travail des passages |
| `--list` | bool | Lister les cas sans rien lancer |
| `--strict` | bool | Sortir avec le code 1 si un passage échoue |
| `--rescore` | string | Vérifier de nouveau les assertions sur les traces d'un passage précédent (son dossier `--out`), sans appeler de modèle ; `changed_within`, `hidden_tests` et `file_matches` gardent leur résultat ; les variantes et leur comparaison sont recalculées |
| `--variant` | string, répétable | Variante du hub : `nom` (hub embarqué), `nom=-skill:<id>` (sans ce skill), `nom=<dossier de hub>[,-skill:<id>…]` ; la première est la référence du verdict. Incompatible avec `--record-baseline` |
| `--tiered` | bool | Référence par niveaux : répétitions de chaque cas calculées depuis l'historique de `--baseline` (obligatoire) ; ×1 pour un cas à 100 % sur ses deux dernières mesures, ×`--reps` sinon ; escalade active |
| `--stable-measures` | int | Avec `--tiered`, nombre de dernières mesures à 100 % qui font passer un cas ×1 (défaut : 2 ; 0 : tous les cas ×1, l'escalade décide) |
| `--history` | string, répétable | Autre `report.json` (motif glob accepté) dont les mesures comptent pour `--tiered` après celles de la référence, par date (passages des lots depuis la dernière référence), et dont les coûts servent à `--plan` |
| `--reps-case` | strings | Répétitions imposées à des cas (`S5=3,S1=1`), au-dessus de `--reps` et de `--tiered` |
| `--escalate` | bool | Un cas lancé moins de ×`--reps` dont un passage échoue est relancé jusqu'à ×`--reps` dans le même passage (défaut : oui avec `--tiered`, `--escalate=false` pour couper) |
| `--plan` | bool | Afficher les passages prévus par cas et le coût estimé, sans rien lancer ni demander de clé |
| `--costs` | string, répétable | Autre `report.json` (motif glob accepté) dont les coûts moyens complètent ceux de `--baseline` pour `--plan` |

**Variantes du hub** (`--variant`, deux ou plus) : chaque cas tourne avec chaque variante, pour mesurer l'effet d'un changement de contenu, par exemple un skill. `-skill:<id>` (nom ou catégorie/nom) retire le fichier du skill et ses mentions dans `skills` et `native_skills` du frontmatter des agents ; le corps des agents et des skills n'est pas réécrit. Un `<dossier de hub>` contient `agents/`, `skills/`, `workflows/` (et `permissions/`), par exemple la racine du dépôt (`--variant depot=..`, sans `make sync-content`). Avant tout passage, le hub de chaque variante est vérifié : catalogue des skills (erreurs d'`oh skill check`), validation des workflows des cas et composition de leur bundle ; une variante qui casse le hub arrête l'évaluation avec la liste des erreurs. Chaque passage installe le hub de sa variante dans son `OH_HOME` temporaire. Les répétitions alternent les variantes (cas, modèle, répétition, variante), `--parallel` et `--budget-total` portent sur l'ensemble, et les traces vont dans `passes/<cas>/<variante>/<modèle>/<n>/`. Le rapport ajoute une colonne « Variante » et une section « Comparaison avec la référence » : par cas, réussite, coût moyen et durée de chaque variante, écart avec la référence et verdict (`gain`, `no_gain`, `ambiguous`, `incomplete` dans `report.json`) ; la même comparaison sans les assertions `skill_loaded_before` (« Résultat hors chargement des skills », `outcome` dans `report.json`), puis un verdict par variante sur tous les cas et la réussite de chaque assertion dans les deux variantes. Le critère est décrit dans [`evals/README.fr.md`](../../evals/README.fr.md#comparer-avec-et-sans-un-skill).

**Référence par niveaux** (`--tiered`) : un cas resté à 100 % sur ses deux dernières mesures passe ×1, les autres ×`--reps` (3 par défaut). Les mesures sont l'historique de la référence (`history` de `baseline.json` : les quatre dernières références de chaque cas, avec leur réussite x/n, leur coût moyen et l'empreinte du dossier du cas), puis celles des rapports de `--history`, par date. Une mesure ne compte que si elle a été prise sur le cas tel qu'il est : un cas dont le dossier a changé (empreinte différente, ou assertions différentes pour une référence enregistrée sans empreinte) repasse ×`--reps`, comme un cas absent de la référence ou mesuré une seule fois. `--reps-case` impose ensuite les répétitions de certains cas (assertions changées, cas à confirmer). **Escalade** : quand un passage d'un cas lancé moins de ×`--reps` échoue, ses répétitions manquantes sont ajoutées tout de suite en tête de file (dans le même passage, toutes variantes comprises) ; `--parallel` et `--budget-total` s'appliquent à elles aussi, et une répétition non lancée faute de budget apparaît dans « Passages non lancés ». Le rapport donne les répétitions de chaque cas (« Répétitions par cas » : prévu, lancé, raison, escalade) ; `report.json` les garde dans `plan` et `escalate_to`. Le taux de réussite reste un ratio x/n : la colonne « Référence » compare des taux, quel que soit le nombre de passages de chaque côté. `--record-baseline` reprend l'historique de la référence qu'il remplace et y ajoute la nouvelle mesure (un `--rescore` de la même mesure la remplace). `--plan` affiche, sans rien lancer, les passages de chaque cas, la raison, le coût moyen d'un passage (dernière mesure de la référence, sinon le rapport le plus récent de `--history`/`--costs`, sinon la moyenne du workflow, sinon celle de la référence) et le total, avec les escalades probables (taux d'échec de la dernière mesure, une chance sur deux pour un cas jamais mesuré), le pire cas (toutes les escalades) et la comparaison avec `--budget-total`.

Depuis `cli/` : `make test-eval` (une répétition), `make test-eval OH_EVAL_REPS=3`, `make test-eval OH_EVAL_REPS=3 OH_EVAL_PARALLEL=4 OH_EVAL_BUDGET_TOTAL=5` (quatre passages à la fois, 5 $ au plus), `make test-eval OH_EVAL_REPS=3 OH_EVAL_CASES=S6 OH_EVAL_VARIANTS="base sans-23=-skill:debugger-phase-2-3"` (avec et sans un skill), `make test-eval OH_EVAL_REPS=3 OH_EVAL_RECORD=1` pour réenregistrer la mesure de référence (`cli/internal/evalrun/testdata/evals/baseline.json`), ou `make test-eval OH_EVAL_TIERED=1 OH_EVAL_PLAN=1 OH_EVAL_BUDGET_TOTAL=14` puis `make test-eval OH_EVAL_TIERED=1 OH_EVAL_RECORD=1 OH_EVAL_PARALLEL=4 OH_EVAL_BUDGET_TOTAL=14` pour une référence par niveaux (`OH_EVAL_REPS_CASE`, `OH_EVAL_ESCALATE=0`, `OH_EVAL_STABLE_MEASURES`, `OH_EVAL_HISTORY` et `OH_EVAL_COSTS` donnent `--reps-case`, `--escalate=false`, `--stable-measures`, `--history` et `--costs`). Les tests de `go test ./...` n'appellent jamais de modèle.

---

## Git Worktree

`oh worktree` gere les git worktrees du projet. **Alias :** `oh wt`. `oh run --location new` cree aussi un worktree par session (voir [`oh run`](cli-workflows.fr.md#oh-run)).

### oh worktree list

Liste les worktrees du projet.

**Alias :** `oh worktree ls`

```
oh worktree list [options]
```

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Sortie au format JSON |

**Exemple :**

```bash
oh worktree list
oh worktree ls --json
```

---

### oh worktree add

Cree un worktree pour une branche. Interactif si la branche est omise.

```
oh worktree add [branch]
```

**Exemple :**

```bash
oh worktree add feat/new-feature
oh worktree add fix/bug-123
```

---

### oh worktree remove

Supprime un worktree.

**Alias :** `oh worktree rm`

```
oh worktree remove [path]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--force` | `-f` | bool | Forcer la suppression |

**Exemple :**

```bash
oh worktree remove ../project-feat-auth
oh worktree rm -f ../project-fix-old
```

---

### oh worktree cleanup

Supprime les worktrees dont la branche est entierement mergee dans la branche de base (detection par `git branch --merged`).

```
oh worktree cleanup [options]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--base` | `-b` | string | Branche de base (defaut : detection automatique) |
| `--force` | `-f` | bool | Supprimer sans confirmation |

**Exemple :**

```bash
oh worktree cleanup
oh worktree cleanup -b develop --force
```

---

## Metriques & Dashboard

### oh metrics

Metriques d'utilisation : sessions, tokens, couts et economies.

```
oh metrics [options]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--period` | `-d` | string | Periode d'analyse : `7d`, `30d` ou `all` (defaut : `all`) |

**Exemple :**

```bash
oh metrics
oh metrics -d 30d
oh metrics -d 7d
```

---

### oh dashboard

Tableau de bord interactif (TUI) : projets, sessions, tokens. Sans flag.

```
oh dashboard
```

---

### oh board

Tableau kanban des tickets en plein ecran.

```
oh board [options]
```

| Flag | Type | Description |
|------|------|-------------|
| `--watch` | bool | Rafraichissement automatique toutes les 5 s |

**Exemple :**

```bash
oh board
oh board --watch
```

`oh optimize` (analyse d'usage et suggestions) et `oh yield` (rapport sessions ↔ commits) existent encore mais sont masques de l'aide, sans flag.

---

## Gestion des secrets

### oh secrets

Secrets stockes dans le trousseau du systeme (ou, a defaut, dans un fichier chiffre `~/.oh/secrets.enc`, phrase de passe demandee ou `OH_PASSPHRASE`). Ils sont references par nom dans `hub.toml` (`token_key`) et resolus a l'execution, en portee projet ou globale. Ordre de resolution : secret du projet (dans un dossier de projet), puis secret global, puis variable d'environnement correspondante.

| Commande | Flags | Description |
|----------|-------|-------------|
| `oh secrets set <cle>` | `--global`, `--project <id>` | Stocke un secret ; la valeur est saisie au terminal (masquee), ou lue sur l'entree standard sans terminal (`printf %s "$TOKEN" \| oh secrets set <cle>`). Portee : projet si le dossier courant est un projet enregistre, sinon globale ; `--global` force la portee globale, `--project` cible un projet |
| `oh secrets get <cle>` | `--global`, `--project <id>`, `--reveal` | Affiche un secret, masque sauf avec `--reveal` ; `--global` : portee globale uniquement |
| `oh secrets list` | | Liste les secrets connus |
| `oh secrets delete <cle>` | `--global`, `--project <id>` | Supprime un secret du trousseau |
| `oh secrets cleanup` | `--dry-run` | Supprime les entrees orphelines du trousseau (absentes de l'index des secrets) ; `--dry-run` : les afficher sans supprimer |

```bash
oh secrets set openhub.mcp.gitlab.token
oh secrets set openhub.mcp.gitlab.token --project t-sru-b267fbf1
oh secrets get openhub.mcp.gitlab.token --reveal
oh secrets list
oh secrets delete openhub.mcp.gitlab.token --global
oh secrets cleanup --dry-run
```

---

## Utilitaires

### oh version

Affiche la version du binaire `oh`.

```
oh version
```

---

### oh completion

Genere le script d'autocompletion pour le shell indique.

```
oh completion bash|zsh|fish|powershell
```

**Exemple :**

```bash
source <(oh completion zsh)
oh completion zsh > "${fpath[1]}/_oh"
oh completion bash > /etc/bash_completion.d/oh
oh completion fish > ~/.config/fish/completions/oh.fish
oh completion powershell | Out-String | Invoke-Expression
```
