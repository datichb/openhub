> [Read in English](README.en.md)

# Évaluations (`oh eval`)

Chaque dossier `evals/<workflow>/<cas>/` est un cas d'évaluation, lancé par [`oh eval`](../docs/reference/cli-tools.fr.md#oh-eval) avec un vrai modèle :

```
evals/<workflow>/<cas>/
  case.yaml       la session et ses assertions
  fixture/        le projet de départ (copié dans un clone git jetable, commité sur main)
  hidden_tests/   tests copiés sur le clone après la session (jamais vus par les agents)
```

## Format de `case.yaml`

```yaml
id: S2                      # identifiant unique (S1, S2…)
title: "Ticket : …"
workflow: ticket
active: true                # false : cas ignoré ; inactive_reason dit pourquoi
mode: semi-auto             # défaut : celui du workflow
inputs: { review_mode: standard }   # entrées du workflow
text: "…"                   # première entrée de texte libre
tickets: [ev-1]             # tickets passés au workflow
beads:                      # base Beads créée dans le clone
  prefix: ev
  tickets:
    - { id: ev-1, title: "…", type: bug, labels: [ai-delegated], description: "…", acceptance: "…" }
setup:                      # commandes shell lancées dans le clone, chacune commitée
  - git checkout -q -b feature/x
timeout: 20m                # défaut : 20m
budget_usd: 1.0             # défaut : celui du passage (--budget)
answers:
  checkpoints: { cp-2: approve, "*": approve }   # approve | fix | reject | other, « : message » en option
  permission: once          # once | reject
  question: first           # première option, ou une réponse libre
  wiki: accept              # propositions du wiki de la carte d'un checkpoint de commit : accept | refuse | pending (défaut : laissées en file)
assertions:
  - type: ran
    command: "git commit*"
    absent: true
    before: { checkpoint_passed: cp-2 }
hidden_tests: "go test ./..."   # défaut : selon la stack (go.mod, package.json, pyproject.toml, sinon make test)
reports: off                # le démon ne garde aucun rapport : workflow_report répond « non enregistré »
```

Les ancres `before:` désignent le premier événement de la trace qui correspond : `checkpoint` (atteint), `checkpoint_passed`, `subagent`, `tool` (avec `arg`, motif du chemin ou de la commande), `text` (expression régulière), `command` (motif shell), restreints à un agent avec `agent`. Les motifs shell acceptent `*` et vérifient aussi chaque partie d'une commande enchaînée (`cd x && go test ./...`).

Les assertions sont décrites dans la [référence d'`oh eval`](../docs/reference/cli-tools.fr.md#oh-eval). Après une retouche d'assertion, `oh eval --rescore <dossier>` revérifie les traces déjà obtenues sans repayer les passages.

## Cas livrés

| Cas | Workflow | Ce qui est vérifié |
|-----|----------|--------------------|
| S1 | review | La vérification passe en premier ; verdict « corriger » sur un bug semé |
| S2 | ticket | Developer et reviewer n'appellent jamais `question` ; pas de commit avant `cp-2` |
| S3 | feature | Le pathfinder renvoie son bloc sans poser de question ; une question bloquante est relayée |
| S4 | audit | Chaque skill de phase est chargé avant sa phase |
| S5 | cadrage | Le planner suit la variante sous-agent |
| S6 | debug | Skill de phase chargé avant chaque phase |
| S7 | quick | Un skill hors périmètre est signalé absent, sans boucle |
| S8 | ticket | Rapports non enregistrés (`reports: off`) : developer et reviewer incluent leur rapport complet dans le bloc |
| S9 | sweep | Les retours de developer et reviewer s'adressent à `conductor` |
| S10 | quick | Une décision sur les données est posée en « Questions bloquantes », pas via `question` |
| S11 | ticket | Projet avec wiki : une proposition `wiki_propose` est acceptée à la carte de `cp-2`, appliquée par oh et commitée avec le code |
| S12 | doc-sync | Une page `CONFIRMÉ` en dérive est corrigée ; oh note la synchronisation ; seul le wiki est commité, après `cp-sync` |
| S13 | onboarding | Skills de phase 1 et 5 chargés avant l'exploration et avant l'écriture du wiki ; écritures limitées à `docs/wiki/` et `ONBOARDING.md` |
| S14 | libre | L'onboarder lancé en sous-agent ne pose pas de question et rend son enveloppe à `orchestrator` |
| S15 | cadrage | Une demande d'interface fait invoquer le designer : `ui-protocol` chargé, pas de question, enveloppe vers son appelant |
| S16 | skill-refresh | Hub réduit : seule la skill périmée est retenue, proposée à `cp-refresh` sans écriture avant, puis mise à jour (`versions`, `reviewed`) et commitée ; les notes de veille rendent le cas déterministe sans recherche web |
| S17 | skill-refresh | Proposition refusée à `cp-refresh` : aucune écriture ni commit, avant comme après |
| S18 | skill-refresh | Toutes les skills de stack sont récentes : ni checkpoint, ni écriture, ni commit |
| S19 | cadrage | « Toutes les tâches » heurte deux règles du code (tâches privées, corbeille) : chacune devient une question posée à l'utilisateur |
| S20 | cadrage | « Réservation » désigne un prêt dans le code et une file d'attente dans un type : le sens est demandé à l'utilisateur, la spec du planner porte un glossaire qui le définit |
| S21 | cadrage | Chaque critère d'acceptation de la spec porte, sur sa ligne, le test qui le vérifie |
| S22 | ticket | Méthode `tdd` : `cp-seams` puis `cp-red` passé (tests qui échouent pour la bonne raison) avant le commit, `cp-2` vert, tests cachés |
| S23 | quick | Mutex demandé sans changer l'API : `go vet ./...` (étape de la CI, hors du paquet modifié) et tests cachés passent ; une implémentation naïve passe `go test` |
| S24 | debug | Piste trompeuse : la pile pointe sur `report/format.go`, le ticket de correction nomme la vraie cause (`splitRecord`, `ledger/parse.go`) |
| S25 | quick | Merge commité avec ses marqueurs de conflit : les deux côtés survivent (dont un conflit sémantique sans marqueur), aucun marqueur ne reste |
| S26 | review | Revue sur deux axes : le critère oublié du ticket (`ErrDivByZero`) est relevé dans l'axe « conformité à la demande », l'affichage de débogage dans l'axe « qualité du code » ; rapports non enregistrés (`reports: off`) |
| S27 | cadrage | Deux conceptions, une seule respecte la règle `depguard` de `.golangci.yml` (le domaine n'importe aucun autre paquet) : l'option qui la respecte est recommandée, contrainte citée, malgré la préférence de l'utilisateur |
| S28 | skill-refresh | Skill de stack sans métadonnées réécrite : `oh skill check --hub .` sans erreur (si `oh` est dans le `PATH` des tests cachés), description « Utiliser quand… », locators, et rien de l'extrait fourni dans les notes recopié tel quel |

S16 à S28 ne sont pas encore dans la mesure de référence (`notInBaseline` dans `cli/internal/evalrun/evalrun_test.go`) : ils y entreront au prochain enregistrement.

La mesure de référence (Haiku, trois répétitions) est enregistrée dans `cli/internal/evalrun/testdata/evals/baseline.json` ; `make test-eval` la compare au passage courant.

## Lancer plus vite, sans dépasser un budget

```
cd cli
make test-eval OH_EVAL_REPS=3 OH_EVAL_PARALLEL=4 OH_EVAL_BUDGET_TOTAL=5
```

- `OH_EVAL_PARALLEL` (`--parallel`) : nombre de passages lancés en même temps. Chaque passage reste isolé (son `OH_HOME`, son `HOME`, son `TMPDIR`, son clone, son démon, son proxy) et le rapport est le même qu'en séquentiel. Mesuré sur S7 et S10 ×2 avec Haiku : 1 min 14 s à quatre en parallèle contre environ 3 min 6 s en séquentiel ; chaque passage dure un peu plus (57 s en moyenne contre 44 à 49 s).
- `OH_EVAL_BUDGET_TOTAL` (`--budget-total`) : plafond de toute l'évaluation ; une fois atteint, plus aucun passage ne démarre. Le plafond par passage (`OH_EVAL_BUDGET_USD`) reste appliqué : 1 $ par défaut, 2,50 $ avec un modèle Sonnet.
- Un passage arrêté par une limite de débit Bedrock (throttling) est relancé jusqu'à trois fois après une attente croissante ; la section « Relances » du rapport le signale. Si les relances se multiplient, baissez `OH_EVAL_PARALLEL`.

## Comparer avec et sans un skill

Un nouveau skill n'est gardé que si une évaluation avec et sans lui montre un gain. `--variant` (ou `OH_EVAL_VARIANTS` avec `make test-eval`, variantes séparées par des espaces) lance chaque cas avec plusieurs versions du hub :

```
cd cli
make test-eval OH_EVAL_REPS=3 OH_EVAL_PARALLEL=4 OH_EVAL_BUDGET_TOTAL=5 OH_EVAL_CASES=S6 \
  OH_EVAL_VARIANTS="base sans-23=-skill:debugger-phase-2-3"
```

| Variante | Hub des passages |
|---|---|
| `base` | le hub embarqué dans oh, tel quel |
| `sans-x=-skill:<id>` | le hub embarqué sans ce skill (nom ou catégorie/nom ; plusieurs `-skill:` séparés par des virgules) |
| `autre=<dossier>` | un autre hub (`agents/`, `skills/`, `workflows/`, `permissions/`), par exemple `..` pour la racine du dépôt sans `make sync-content` ; `autre=<dossier>,-skill:<id>` le combine avec un retrait |

- **La première variante est la référence** : celle qui contient ce qu'on évalue (le hub avec le skill). Les autres lui sont comparées.
- **Retrait d'un skill** : le fichier du skill, et ses mentions dans `skills` et `native_skills` du frontmatter des agents. Le corps des agents et des skills n'est pas réécrit : une consigne « charge `x` » reste, et le modèle tente de charger un skill absent (`no_denied_loop` le voit s'il insiste). Si le hub ne se compose plus (un autre skill le demande dans `requires:`, un workflow le cite dans `skills:`), l'évaluation s'arrête avant tout passage et liste les erreurs.
- **Vérification** : avant tout passage, le hub de chaque variante passe le contrôle des skills (erreurs d'`oh skill check`), la validation des workflows des cas lancés et la composition de leur bundle.
- **Passages** : chaque cas × modèle × variante × répétition est un passage ; les variantes d'une même répétition sont lancées côte à côte, `--parallel` et `--budget-total` portent sur l'ensemble. Les traces vont dans `passes/<cas>/<variante>/<modèle>/<n>/` ; `--rescore` recalcule la comparaison.
- **Mesure de référence** : `--record-baseline` refuse les variantes ; la colonne « Référence » n'est remplie que pour une variante qui est le hub embarqué tel quel.

### Critère de gain

Pour chaque cas et chaque modèle, `report.md` (section « Comparaison avec la référence ») et `report.json` (`comparisons`, `verdicts`) donnent la réussite, le coût moyen et la durée moyenne de chaque variante, l'écart avec la référence et un verdict :

| Verdict | Quand |
|---|---|
| `gain` | la référence réussit au moins 1/3 de plus (par exemple 3/3 contre 1/3), ou réussit autant et coûte en moyenne au moins 10 % de moins |
| `ambiguous` | l'écart n'est que d'un seul passage (2/3 contre 1/3) : relancer ×3 (`OH_EVAL_REPS=6`, l'écart d'1/3 devient alors deux passages) |
| `no_gain` | écart plus petit, recul, ou réussite égale sans baisse de coût suffisante |
| `incomplete` | aucun passage d'un côté (plafond global atteint) |

Le verdict d'une variante sur tous les cas lancés est un gain s'il y a un gain sur au moins un cas sans recul ailleurs ; un recul d'un seul passage à côté d'un gain le rend ambigu.

Retirer un skill fait échouer par construction les assertions `skill_loaded_before` qui le visent (un chargement en échec ne compte pas). La section « Résultat hors chargement des skills » refait donc la comparaison et le verdict sans ces assertions : c'est le résultat métier (diagnostic, faille relevée, tests cachés) qui dit si le skill apporte quelque chose. Le tableau « Assertions par variante » détaille chaque assertion (◆ : réussite différente).

Première mesure (13 octobre 2026, Haiku ×3, S4 et S6 avec `base`, `sans-23=-skill:debugger-phase-2-3` et `sans-a3=-skill:auditor-phase-3`, 1,82 $) : sans son skill, chaque cas échoue sur l'assertion de chargement (S6 0/3, S4 0/3), mais le résultat métier ne change pas (S6 : cause trouvée et ticket créé 3/3 ; S4 : injection relevée 3/3) et les passages coûtent 17 à 19 % de moins. Hors chargement des skills, le verdict est donc « pas de gain » pour les deux skills sur ces cas simples ; le cas témoin de l'autre variante (S4 sans `debugger-phase-2-3`) montre un recul d'un passage, dû au bruit de `base` (`auditor-phase-4` chargé un événement trop tard), qui rend le verdict de `sans-23` ambigu.
