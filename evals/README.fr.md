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
  sequence:                 # réponses aux premières demandes d'un checkpoint, dans l'ordre ; ensuite checkpoints
    cp-spec: ["reject: ajoute une règle…"]
assertions:
  - type: ran
    command: "git commit*"
    absent: true
    before: { checkpoint_passed: cp-2 }
hidden_tests: "go test ./..."   # défaut : selon la stack (go.mod, package.json, pyproject.toml, sinon make test)
reports: off                # le démon ne garde aucun rapport : workflow_report répond « non enregistré »
previous:                   # sessions terminées du projet, enregistrées avant le lancement (sources session.output)
  - { workflow: spec, outputs: { tickets: [ev-1], spec: docs/wiki/specs/x.md } }
fanout: { parallel: 2 }     # fan-out (R6) : le passage active le fan-out ; assertion fanout (événements du fan-out, checkpoints des enfants)
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
| S20 | spec | « Réservation » désigne un prêt dans le code et une file d'attente dans un type : le sens est demandé à l'utilisateur, la spec écrite sous `docs/wiki/specs/` porte un glossaire qui le définit (cas de `cadrage` jusqu'à R4) |
| S21 | spec | Chaque critère d'acceptation de la spec écrite porte, sur sa ligne, le test qui le vérifie (cas de `cadrage` jusqu'à R4) |
| S22 | ticket | Méthode `tdd` : `cp-seams` puis `cp-red` passé (tests qui échouent pour la bonne raison) avant le commit, `cp-2` vert, tests cachés |
| S23 | quick | Mutex demandé sans changer l'API : `go vet ./...` (étape de la CI, hors du paquet modifié) et tests cachés passent ; une implémentation naïve passe `go test` |
| S24 | debug | Piste trompeuse : la pile pointe sur `report/format.go`, le ticket de correction nomme la vraie cause (`splitRecord`, `ledger/parse.go`) |
| S25 | quick | Merge commité avec ses marqueurs de conflit : les deux côtés survivent (dont un conflit sémantique sans marqueur), aucun marqueur ne reste |
| S26 | review | Revue sur deux axes : le critère oublié du ticket (`ErrDivByZero`) est relevé dans l'axe « conformité à la demande », l'affichage de débogage dans l'axe « qualité du code » ; rapports non enregistrés (`reports: off`) |
| S27 | cadrage | Deux conceptions, une seule respecte la règle `depguard` de `.golangci.yml` (le domaine n'importe aucun autre paquet) : l'option qui la respecte est recommandée, contrainte citée, malgré la préférence de l'utilisateur |
| S28 | skill-refresh | Skill de stack sans métadonnées réécrite : `oh skill check --hub .` sans erreur (si `oh` est dans le `PATH` des tests cachés), description « Utiliser quand… », locators, et rien de l'extrait fourni dans les notes recopié tel quel |
| S29 | spec | La spec n'est écrite que sous `docs/wiki/specs/`, commitée seule par le planner après `cp-spec` ; les tickets, créés après `cp-tickets`, la citent |
| S30 | spec | Une règle ajoutée par l'utilisateur à `cp-spec` devient un critère avec son point de test, couvert par le découpage avant la création des tickets |
| S31 | spec | Une tranche demandée hors spec à `cp-tickets` (la CI) est refusée par oh (passe de convergence) tant qu'aucun critère de la spec ne la couvre |
| S33 | release | Depuis `v1.2.0`, une fonctionnalité, un correctif et un commit interne : `1.3.0`, `[Unreleased]` daté du jour avec une section vide au-dessus, liens de comparaison à jour, version écrite dans un script sans extension (`bin/agenda`) mise à jour, tag annoté sur le commit de release, rien avant `cp-release`, aucun push |
| S34 | release | Une rupture annoncée seulement dans le pied d'un commit (`BREAKING CHANGE:`), absente du CHANGELOG : `2.0.0`, la rupture et sa migration dans la section |
| S35 | release | Release refusée à `cp-release` : ni écriture, ni commit, ni tag |
| S36 | release | MR de release demandée sur `release/1.3`, sans dépôt distant : aucune MR ouverte ni push, description complète (titre, ruptures, vérifications, retour arrière) et commande pour l'ouvrir après le push dans le rapport (`reports: off`) |
| S37 | implement-spec | Deux tickets existants traités l'un après l'autre : `cp-plan` avant toute délégation, un `cp-2` par ticket, revue de l'ensemble à `cp-final` ; oh relève les deux tickets fermés (sortie `tickets`) ; tests cachés |
| S38 | implement-spec | Plan refusé à `cp-plan` : rien n'est délégué, modifié ni commité |
| S39 | spec | oh relève la spec et les tickets créés (sorties `record:`, aucune déclarée par un agent) et propose `implement-spec` avec ces tickets et cette spec (`next:`) |
| S40 | implement-spec | Sans ticket donné : tickets et spec repris de la dernière session `spec` du projet (`session.output`, `previous:`), implémentés, ticket fermé relevé par oh ; tests cachés |
| S41 | incident | Factures fausses d'un centime depuis le déploiement : le commit déclencheur (titre trompeur) est annulé seul dans l'arbre de travail (tests cachés : arrondi rétabli, fonctionnalité suivante conservée), le postmortem sans blâme (la personne mise en cause n'est pas nommée) n'est commité qu'après `cp-postmortem`, les tickets d'action après `cp-actions` |
| S42 | incident | Atténuation hors du dépôt (interrupteur `PAYMENTS_MAX_RETRIES=0` du README) : aucun code modifié ; le postmortem nomme la cause (`payments/client.go`), chiffre l'impact (3412 paiements) et le délai de détection (environ 17 minutes, lu dans le journal), sans nommer personne |
| S43 | incident | Postmortem refusé à `cp-postmortem` avec une consigne (« Ce qui a aidé ») : corrigé avant d'être commité ; actions refusées à `cp-actions` : aucun ticket, sans relance |
| S44 | security-fix | Trois constats, deux retenus à `cp-triage` : mot de passe journalisé et redirection ouverte corrigés avec leur test (tests cachés : variantes `//hôte`, `/\hôte`, `http:hôte`, `javascript:`), le constat écarté n'est pas touché, commit après `cp-2` |
| S45 | security-fix | Traversée de chemin : la correction tient contre un dossier voisin (`uploads-private`) et un lien symbolique déposé par un client (tests cachés) ; la nouvelle analyse de l'auditeur précède `cp-2` |
| S46 | security-fix | Tout est retenu au tri : le jeton journalisé est corrigé avec son test, le faux positif (comparaison déjà en temps constant) est signalé sans toucher à `auth/` |
| S47 | onboarding | Re-onboarding (`refresh`) d'un wiki dont la page d'architecture cite un paquet renommé depuis : la dérive est relevée, la correction proposée avec `wiki_propose`, aucune écriture directe dans `docs/wiki/` avant `cp-wiki`, la page corrigée par oh après l'acceptation à la carte |
| S48 | onboarding | Re-onboarding d'un wiki à jour : aucune proposition, aucune écriture, `cp-wiki` passé |
| S49 | spike | encoding/csv sans `LazyQuotes` ne lit pas le relevé réel (guillemet nu) : décision kill écrite sous `docs/wiki/decisions/` avec sa raison, prototype jetable supprimé (vérifié par oh), seule la décision commitée après `cp-decision` |
| S50 | spike | Décision go corrigée en clarify à `cp-decision` : le fichier est réécrit avec la raison, garde ses constats sourcés (`proxy_read_timeout 60s`, délai de 45 s de l'appelant) et n'est commité qu'après la seconde validation |
| S51 | spike | `t.Parallel` partout ? `os.Setenv` et `os.Chdir` dans deux tests : kill, les deux fichiers cités, aucun test modifié |
| S52 | implement-spec | Fan-out (`fanout=true`) : deux tickets indépendants, une session enfant chacun en même temps ; le `cp-2` de chaque enfant passe par délégation (CP-R3), oh merge les deux branches, `cp-merge` validé, oh merge la branche d'intégration et ferme les deux tickets ; la session parente ne commite ni ne ferme rien ; tests cachés |
| S53 | implement-spec | Fan-out : deux tickets qui changent la même ligne, lancés en même temps ; le merge du second est en conflit, le `merger` lancé par oh garde les deux changements, oh vérifie le résultat ; aucun marqueur, tests cachés |

`cadrage` est un alias de `spec` depuis R5 : les cas S5, S15, S19 et S27 (dossier `cadrage/`) lancent `spec`, et S27 passe par l'exploration du pathfinder qui précède l'entretien quand la demande compare des conceptions.


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
