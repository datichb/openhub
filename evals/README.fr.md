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

S16 à S18 ne sont pas encore dans la mesure de référence (`notInBaseline` dans `cli/internal/evalrun/evalrun_test.go`) : ils y entreront au prochain enregistrement.

La mesure de référence (Haiku, trois répétitions) est enregistrée dans `cli/internal/evalrun/testdata/evals/baseline.json` ; `make test-eval` la compare au passage courant.

## Lancer plus vite, sans dépasser un budget

```
cd cli
make test-eval OH_EVAL_REPS=3 OH_EVAL_PARALLEL=4 OH_EVAL_BUDGET_TOTAL=5
```

- `OH_EVAL_PARALLEL` (`--parallel`) : nombre de passages lancés en même temps. Chaque passage reste isolé (son `OH_HOME`, son `HOME`, son `TMPDIR`, son clone, son démon, son proxy) et le rapport est le même qu'en séquentiel. Mesuré sur S7 et S10 ×2 avec Haiku : 1 min 14 s à quatre en parallèle contre environ 3 min 6 s en séquentiel ; chaque passage dure un peu plus (57 s en moyenne contre 44 à 49 s).
- `OH_EVAL_BUDGET_TOTAL` (`--budget-total`) : plafond de toute l'évaluation ; une fois atteint, plus aucun passage ne démarre. Le plafond par passage (`OH_EVAL_BUDGET_USD`) reste appliqué : 1 $ par défaut, 2,50 $ avec un modèle Sonnet.
- Un passage arrêté par une limite de débit Bedrock (throttling) est relancé jusqu'à trois fois après une attente croissante ; la section « Relances » du rapport le signale. Si les relances se multiplient, baissez `OH_EVAL_PARALLEL`.
