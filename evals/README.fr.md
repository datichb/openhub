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

La mesure de référence (Haiku, trois répétitions) est enregistrée dans `cli/internal/evalrun/testdata/evals/baseline.json` ; `make test-eval` la compare au passage courant.
