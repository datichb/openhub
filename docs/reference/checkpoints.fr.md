> [Read in English](checkpoints.en.md)

# Référence — Checkpoints, modes et vérification par oh

Un checkpoint est une pause du workflow : l'agent appelle `workflow_checkpoint`, oh applique le comportement du mode (attendre l'utilisateur ou laisser passer) et ouvre les opérations que le checkpoint déverrouille (`commit`, `close`, `create`, `update`, `apply`, `push`, `tag`, `mr`). Le schéma des champs est dans [Schéma des workflows](workflow-schema.fr.md#checkpoints) ; l'exécution à trois niveaux (prompt, outil MCP, plugin oh) dans l'[ADR-042](../architecture/adr/042-checkpoints-headless-decisions.fr.md).

Chaque pause coûte du temps humain et, à force, une validation machinale : le nombre de pauses est un paramètre de sécurité. Les règles ci-dessous (rapport de modernisation d'octobre 2026, §9, règles CP-R1 à CP-R11) gardent peu de pauses, et mieux informées.

---

## Modes

| Mode | Ce qui met en pause |
|---|---|
| `manuel` | tous les checkpoints, sauf ceux en `skip` |
| `semi-auto` | les décisions humaines (périmètre, plan) et les checkpoints obligatoires ; les étapes intermédiaires passent seules |
| `auto` | **tout sauf les décisions obligatoires** : un commit, une fermeture de ticket, une création de tickets ou une modification d'infrastructure demandent toujours l'utilisateur |

Le mode `auto` n'est donc pas une autonomie complète : un checkpoint obligatoire (ou qui déverrouille une opération) n'est jamais sauté. Au lancement, `oh run` et le récapitulatif de la TUI affichent les **pauses prévues** pour le mode choisi, par exemple :

```
  Pauses prévues 2 par passage : cp-plan (obligatoire) · cp-2 (obligatoire) — auto = tout sauf les décisions obligatoires
```

Un checkpoint conditionnel y figure avec la mention « si sa condition est vraie ». Les checkpoints répétés (un `cp-2` par ticket) comptent une fois par passage.

### Pauses des workflows livrés (semi-auto)

| Workflow | Pauses en semi-auto | Dont irréversibles | Budget |
|---|---|---|---|
| `ticket` | `cp-2` | `cp-2` | 3 |
| `ticket` en méthode `tdd` | `cp-seams`, `cp-2` | `cp-2` | 3 + 1 |
| `implement-spec` | `cp-plan`, `cp-2`, `cp-final` | `cp-2` | 3 |
| `implement-spec` en méthode `tdd` | les mêmes et `cp-seams` | `cp-2` | 3 + 1 |
| `implement-spec` en fan-out (expérimental) | `cp-plan`, `cp-merge`, `cp-final` (les `cp-2` des enfants sont délégués, CP-R3) | `cp-merge` | 3 |
| `merge-conflicts` (lancé par oh dans un fan-out) | aucune | — | 2 |
| `spec` | `cp-understanding`, `cp-spec`, `cp-tickets` (`cp-converge` est vérifié par oh, sans pause) | `cp-spec`, `cp-tickets` | 4 |
| `sweep` | `cp-plan` (découpage relevé et vérifié par oh), `cp-commit`, `cp-recap` | `cp-commit` | 3 |
| `sweep` en fan-out (expérimental) | `cp-plan`, `cp-merge`, `cp-recap` (les `cp-commit` des enfants `sweep-task` sont délégués, CP-R3) | `cp-merge` | 3 |
| `sweep-task` (hors fan-out) | `cp-commit` | `cp-commit` | 3 |
| `review-feedback` | `cp-fix`, `cp-2` | `cp-2` | 3 |
| `quick` | `cp-commit` | `cp-commit` | 3 |
| `debug` | `cp-ticket`, `cp-commit` | `cp-ticket`, `cp-commit` | 4 |
| `infra-change` (manuel seulement) | `cp-plan`, `cp-apply` | `cp-apply` | 3 |
| `release` | `cp-release` | `cp-release` | 3 |
| `benchmark` | `cp-target` | — | 2 |

`feature` est un alias (`spec`, ou `implement-spec` avec `--tickets`) : ses pauses sont celles du workflow lancé.

---

## Budget de pauses (CP-R2)

En semi-auto (ou dans le mode par défaut quand semi-auto n'est pas autorisé), un workflow met en pause au plus :

> **2 décisions humaines** (périmètre, compréhension, arbitrage) **+ 1 par checkpoint qui déverrouille une action irréversible** (`unlocks:` : commit, close, create, apply, push, tag, mr).

Un checkpoint `conditional`, ou qui porte un `when:`, compte comme une pause (le pire cas). Le budget est vérifié :

- sur les workflows livrés par le hub, par le test `TestCheckpointPauseBudget` (bloquant) ;
- sur les workflows d'équipe et de projet, par `oh workflow validate` : avertissement `pause_budget_exceeded` (le workflow reste utilisable).

Une **méthode** choisie au lancement (`method: tdd`, voir [Méthode TDD](#méthode-tdd--cp-seams-et-cp-red-r7)) ajoute au plus **1 pause**, pour ses seules sessions : le budget est vérifié pour une session sans méthode et pour une session de chaque méthode (rapport §9.2 : « +1 pause par ticket TDD »).

Pour rentrer dans le budget : passer en `auto` les étapes qui ne protègent rien, ou les faire vérifier par oh (`verify:`) plutôt que par une pause.

---

## Vérification lancée par oh (`verify:`)

```yaml
checkpoints:
  cp-2:
    mandatory: true
    unlocks: [commit, close]
    verify: { from: stack, expect: pass }
```

Avant qu'un checkpoint passe, et donc avant d'ouvrir ses déverrouillages, oh lance lui-même la vérification :

1. **Commandes** : `from: stack` prend celles de la stack détectée (`bricks.VerifyRuns`, dérivées des motifs de `bricks.VerifyCommands` : une cible `make test|build|lint|typecheck` d'abord, sinon les scripts de `package.json` avec le gestionnaire du lockfile, `tsc --noEmit` pour TypeScript, `go build ./...` et `go test ./...`, `cargo`, `pytest`, `mvn`, `./gradlew`). `kinds` choisit les types (par défaut `build`, `test`, `typecheck` ; `lint` sur demande). `cmd` ajoute des commandes **écrites dans le document du workflow**, jamais fournies par l'agent ni par une entrée (pas de gabarit). Jamais d'installation ni de publication.
2. **Exécution** : sans shell, dans l'emplacement de la session (ou le sous-dossier `in`), avec un environnement nettoyé (chemins, locale, caches des outils ; ni jetons, ni identifiants, ni variables d'oh), une entrée vide, un délai (`timeout`, 10 min par défaut, 30 min au plus ; le groupe de processus est tué à l'échéance) et une sortie bornée (fin de sortie conservée, extrait de 30 lignes).
3. **Résultat** : `expect: pass` (défaut) exige que toutes les commandes réussissent ; `expect: fail` que les commandes qui construisent le code réussissent et qu'un test échoue à l'exécution (test écrit d'abord, voir [Méthode TDD](#méthode-tdd--cp-seams-et-cp-red-r7)) ; un dépassement de délai est toujours un échec. `must_touch` exige en plus que les changements de la session (commits depuis le début, modifications non commitées, fichiers nouveaux) touchent un des chemins donnés.
4. **Réutilisation** : le résultat est lié à l'état du travail (HEAD, modifications, fichiers non suivis). Il est réutilisé tant que cet état ne change pas ; sinon la vérification est relancée.

**Au checkpoint en pause**, la vérification démarre dès que l'agent demande le checkpoint : la carte de décision affiche son résultat (en cours, réussie, échouée avec un extrait). **Si elle échoue** :

- « Valider » est refusé ; la carte propose **« Passer outre la vérification »**, avec une consigne obligatoire dans la TUI. Le passage est tracé dans la frise (`⚑ cp-2`) et dans la trace de la session (événement `overridden`, auteur et message) ;
- une validation donnée ailleurs (dans l'interface de l'outil) ne passe pas outre : l'appel de l'agent échoue avec `ErrVerifyFailed`, un résumé et l'extrait de la sortie, et rien n'est déverrouillé. L'agent corrige, puis redemande le checkpoint.

**Au checkpoint automatique**, la vérification tourne pendant l'appel de l'agent ; au-delà de 20 secondes, l'appel répond « vérification en cours » et l'agent le rappelle un peu plus tard (le résultat est conservé).

Projet sans stack détectée : « rien à vérifier », le checkpoint passe et la carte l'indique.

**Vérification impossible : jamais de passage en silence.** Si l'emplacement de travail est introuvable (dossier supprimé, sous-dossier `in` absent), la vérification ne peut pas tourner : le checkpoint **se met en pause dans tous les modes**, `auto` compris, même s'il passerait seul d'ordinaire. La carte affiche « impossible ici » ; « Valider » est refusé et seul **« Passer outre la vérification »** le laisse passer, tracé comme un échec (`⚑`, événement `overridden`). L'appel de l'agent échoue avec `ErrVerifyUnavailable` et rien n'est déverrouillé. Il en va de même quand l'emplacement est un dépôt git que git refuse de lire (dépôt d'un autre propriétaire, par exemple) : sans état du travail connu, aucun résultat ne peut y être rattaché, la carte affiche « git ne peut pas lire l'état du travail » et aucune commande ne tourne.

Dans un workflow en lecture seule (`risk: read`), un checkpoint vérifié demande toujours l'utilisateur : les commandes ne tournent qu'après sa validation.

**Assouplissement** : une couche d'équipe ou de projet peut renforcer une vérification héritée (ajouter des types, des commandes, des chemins `must_touch`, allonger le délai), jamais la retirer ni l'affaiblir (`loosening`) ; retirer un checkpoint vérifié est refusé (`verified_checkpoint_removed`).

Les workflows livrés vérifient ainsi `cp-2` (`ticket`, `implement-spec`, `review-feedback`) et `cp-commit` (`quick`, `debug`, `sweep`, `sweep-task`).

**Specs écrites (`verify.spec`, R4)** : au lieu de commandes, oh lit les specs que la session a modifiées sous les chemins donnés et vérifie leur frontmatter (format du wiki), leur glossaire, le point de test sur la ligne de chaque critère (`CA-n · Test : …`) et, avec `breakdown: true`, que le découpage couvre chaque critère sans tranche hors spec ([format](workflow-schema.fr.md#verify--vérification-lancée-par-oh)). Le workflow `spec` s'en sert trois fois : `cp-converge` (automatique dans tous les modes : la passe de convergence, qui renvoie un écart au planner sans pause), puis `cp-spec` et `cp-tickets` (même vérification avant le commit et la création des tickets).

### Sessions distantes

Dans une [session distante](../guides/remote-runners.fr.md), la vérification tourne **là où est le travail** : dans le job, sur le clone du projet, avec les mêmes commandes et les mêmes garanties (environnement filtré, délai, sortie bornée, groupe de processus tué à l'échéance), sous le compte `oh` des agents (celui du serveur de l'outil), jamais sous root ; les lectures git de la vérification aussi, comme toutes les autres lectures git d'oh dans le clone (fenêtre `commit`, garde de fermeture, contrôle de la configuration git…, voir [Exécution distante › Lectures git d'oh](../guides/remote-runners.fr.md#6-dans-le-job--oh-runner)). Sans compte `oh` dans l'image, elle tourne sous l'utilisateur du job, comme le serveur.

| Situation dans le job | Ce que fait oh |
|---|---|
| checkpoint `remote: auto`, vérification réussie | validé par le répondeur de politique, puis passé |
| checkpoint `remote: auto`, vérification échouée | jamais passé outre dans le job : l'appel de l'agent est refusé avec l'échec et l'extrait, rien n'est déverrouillé (si la vérification est déjà finie quand le répondeur répond, la décision est refusée : « corriger d'abord ») ; l'agent corrige et redemande |
| vérification impossible (emplacement introuvable, dépôt que git ne peut pas lire) | le checkpoint est **différé** : la session s'arrête proprement, la suite (et le choix de passer outre) se fait sur la machine |
| checkpoint `remote: defer` | différé, comme avant ; la vérification lancée à la demande se termine avant le push, et son résultat remonte |

Le job attend la fin des vérifications en cours avant d'arrêter les processus des agents et de pousser la branche. Le dernier résultat de chaque checkpoint voyage dans `summary.json` (`verifications`) ; `oh session fetch` l'affiche et l'importe, nettoyé et borné : la carte du checkpoint montre « Dans le job distant : … » et la frise garde l'entrée (auteur `remote`). Ce résultat **ne vaut jamais** vérification sur la machine : quand la session reprend en local et redemande le checkpoint, oh vérifie de nouveau, dans le worktree de la branche poussée.

---

## Preuves de l'agent (`evidence`)

`workflow_checkpoint` accepte `evidence: [{cmd, exit}]` : les commandes que l'agent dit avoir lancées et leur code de sortie. oh les recoupe avec les appels shell qu'il a observés dans la session (et ses sous-agents) :

- **observée** : un appel shell a lancé cette commande ;
- **preuve non observée** : aucun appel ne l'a lancée (avertissement sur la carte, dans la frise et dans la réponse à l'agent) ;
- **observée en échec** : l'outil a signalé l'échec de l'appel alors que l'agent annonce 0.

L'outil ne transmet pas le code de sortie d'une commande qui réussit : oh recoupe la commande, et l'échec quand l'outil le signale. La vérification lancée par oh reste la preuve de référence.

---

## Limite des tours de correction (`limits.max_fix_rounds`)

oh compte les paires developer → reviewer par ticket (le ticket pris et pas encore fermé). La première paire est l'implémentation ; au-delà de `max_fix_rounds` tours de correction, quand le reviewer est lancé pour le dernier tour permis :

- les délégations aux agents `developer*` sont refusées ;
- le checkpoint de commit est marqué **« non résolu »** sur la carte de décision ;
- la frise affiche `⛔ <ticket>`.

Le verrou se lève quand l'utilisateur demande une correction (ou donne une autre consigne) au checkpoint, ou quand le ticket est fermé.

---

## Conditions évaluées par oh (`when:`)

```yaml
  cp-spec:
    mode: { manuel: pause, semi-auto: conditional, auto: conditional }
    when: { paths: ["docs/specs/**"], labels: [ux] }
```

Conditions (une suffit) : `entry` (agent d'entrée de la session), `labels` (étiquette d'un ticket de la session), `paths` (chemin modifié par la session, motifs `*` et `**`), `outputs` (sortie déclarée par `workflow_outputs`). Effet :

| Comportement du mode | Condition vraie | Condition fausse |
|---|---|---|
| `auto` | pause | passe |
| `conditional` | pause | passe |
| `pause`, checkpoint obligatoire ou qui déverrouille | pause | **pause** |
| `skip` | sauté | sauté |

Si les chemins modifiés ne peuvent pas être lus (git refuse le dépôt), une condition `paths` est tenue pour vraie : la pause est ajoutée plutôt que perdue. `when:` ne peut qu'**ajouter** une pause, jamais retirer une pause obligatoire. Sans `when:`, un checkpoint `conditional` demande toujours l'utilisateur (oh ne sait pas évaluer la phrase `condition`). Une couche ne peut pas retirer une condition héritée (`loosening`).

---

## Méthode TDD : `cp-seams` et `cp-red` (R7)

`ticket` et `implement-spec` ont une entrée **Méthode** (`method`, `standard` par défaut). Lancés avec `tdd` (`oh run ticket -t <id> -i method=tdd`, ou la fiche de lancement), ils ajoutent deux checkpoints, passés par le `developer`, une fois par ticket, avant l'implémentation :

```
cp-1 → cp-seams (pause) → cp-red (vérifié par oh) → implémentation → review → cp-2
```

| Checkpoint | manuel | semi-auto | auto | distant | Rôle |
|---|---|---|---|---|---|
| `cp-seams` — Points d'injection et tests prévus | pause | pause | auto | auto | le developer présente les points d'injection, les interfaces et les tests prévus, avant d'écrire un test ; oh relève le commit de l'emplacement |
| `cp-red` — Tests en échec | auto | auto | auto | auto | oh vérifie que les tests écrits échouent, pour la bonne raison |

La méthode est évaluée **au lancement** (CP-R7) : le paquet d'une session `tdd` contient ces deux checkpoints, leurs descriptions et ce qu'oh vérifie (contrat de session du developer, ligne « Méthode de la session » pour tous les agents) ; le paquet d'une session `standard` est exactement celui d'avant, sans eux. Les deux checkpoints ne sont ni obligatoires ni déverrouillants : ils n'ajoutent qu'une étape. Une session `tdd` compte **une pause de plus** en `manuel` et en semi-auto (`cp-seams`, rapport §9.2 « +1 pause par ticket TDD ») ; en `auto`, aucune.

**Vérification de `cp-red`** (`verify: { from: stack, expect: fail, must_touch_tests: true, outside_tests: warn, since: cp-seams }`), dans cet ordre :

1. **Changements depuis `cp-seams`** : les fichiers modifiés depuis le commit relevé au passage de `cp-seams` (commits, modifications non commitées, fichiers nouveaux ; depuis le début de la session si `cp-seams` n'a pas été passé) doivent comprendre **au moins un fichier de test** de la stack (`*_test.go`, `**/testdata/**` ; `*.test.*`, `*.spec.*`, `**/__tests__/**` ; `test_*.py`, `*_test.py`, `conftest.py` ; `**/src/test/**` ; dossiers `test/`, `tests/`, `spec/`, `fixtures/` ; ceux de toutes les stacks quand aucune n'est détectée). Sinon : refusé, rien n'est lancé.
2. **Fichiers hors tests** : ils sont **signalés**, pas refusés (`outside_tests: warn`) : le résultat (réponse à l'agent, frise) les nomme (5 au plus). Pourquoi pas un refus : pour qu'un test échoue « pour la bonne raison », le code doit construire, donc les amorces (interfaces, signatures, fonctions qui ne font rien encore) changent des fichiers hors tests ; et les tests unitaires Rust vivent dans les fichiers du code. Le refus reste possible dans un workflow d'équipe (`outside_tests: refuse`). L'implémentation complète écrite trop tôt est, elle, attrapée par l'étape 4.
3. **Le code construit** : oh lance d'abord les commandes qui construisent (`build`, `typecheck`, `lint` si demandé) ; elles doivent réussir. Un échec est refusé (« le code doit construire, seuls les tests doivent échouer ») et les tests ne sont pas lancés.
4. **Les tests échouent à l'exécution** : puis les tests. Ils doivent échouer, et pour la bonne raison ; sont refusés : des tests qui ne compilent pas (`go test` : `[build failed]`, `[setup failed]` ; `cargo` : `could not compile` ; `tsc` : `error TS…`), des tests que pytest ne peut pas charger (code de sortie 2, `ERROR collecting`) ou aucun test lancé (code 5) ; une suite qui passe (« un échec était attendu ») ; un dépassement de délai. Ce qu'un outil ne distingue pas (jest, vitest, `mvn`, un script) compte comme un échec de test : la détection est faite sur la sortie et le code de sortie, sans analyse du langage.

Refusé, l'appel de l'agent échoue avec `ErrVerifyFailed`, la raison et l'extrait de la sortie ; il corrige et rappelle `cp-red`. Le checkpoint étant automatique, aucune carte ne s'ouvre : en `manuel`, `cp-seams` reste la décision humaine. En session distante, la vérification tourne dans le job, comme celle de `cp-2` (vérification impossible : checkpoint différé, jamais de passage en silence). Projet sans stack détectée : « rien à vérifier », seule l'étape 1 s'applique.

---

## Fan-out : substitution de contrôle (CP-R3)

Dans un fan-out (`implement-spec -i fanout=true` ou `sweep -i fanout=true`, expérimental, [guide](../guides/fan-out.fr.md)), chaque ticket a sa session enfant (`ticket`), chaque sous-tâche d'un sweep la sienne (`sweep-task`), et le `cp-2` (ou le `cp-commit`) de chaque enfant **ne fait pas de pause** : il est remplacé par le `cp-merge` de la session parente. Sans cette substitution, N tickets donneraient N pauses ; avec elle, une seule validation, mieux informée (tableau des enfants, revue de l'ensemble, vérification de la branche d'intégration). Elle ne relâche rien, grâce à ces garde-fous, chacun couvert par des tests de contournement :

| Garde-fou | Où |
|---|---|
| Déclarée par la couche hub seulement (`fanout.delegates`) ; une couche d'équipe ou de projet, ou un brouillon, qui l'écrit : refusée (`fanout_delegates_layer`) ; un patch ne peut que baisser `parallel` | schéma (`TestFanout_DelegatesHubOnly`, `TestFanout_PatchMayOnlyHarden`, `TestShippedImplementSpecFanoutTeamLayer`) |
| Refusée sans `cp-merge` obligatoire, `fanout: only`, qui déverrouille `merge` et dont la vérification lancée par oh tourne sur la branche d'intégration (`verify.on: integration`, `expect: pass`) | schéma (`TestFanout_MergeCheckpointGuarantees`) et démon (`Substitutes`, `TestFanoutDelegationBypass`) |
| Le démon relit tout à chaque appel, dans les enregistrements écrits par oh et jamais par un agent : l'enfant est enregistré comme enfant (pas seulement lancé avec `--parent`), la session parente le liste, en cours, du même projet, lancée avec le drapeau expérimental, et son paquet délègue ce checkpoint | `services/checkpoint/fanout.go` (`TestFanoutDelegationBypass` : 17 tentatives) |
| La vérification du `cp-2` délégué tourne toujours dans le worktree de l'enfant : un échec le refuse | `TestFanoutDelegatedVerificationStillRefuses` |
| Aucune session du fan-out (parente, enfant, merger) ne pousse, ne merge, ne ferme de ticket ni ne déplace les branches `oh/fanout/…` ; la parente ne commite pas | règles de session (`FanoutRoleRules`), passerelle Beads (`GuardClose`) ; `TestFanoutRoleRules`, `TestFanoutChildRestrictions` |
| oh merge lui-même : chaque enfant dans la branche d'intégration (`--no-ff`), puis, au passage de `cp-merge`, la branche d'intégration dans celle de la session ; oh ferme les tickets mergés | `services/fanout` |
| `cp-merge` ne passe que quand tous les enfants sont terminés, sur une branche d'intégration où oh l'a laissée (sinon le fan-out s'arrête : `tampered`), après sa vérification, **jamais forcée** (« passer outre » refusé) | `TestFanoutMergeCheckpoint`, `TestFanoutMergeCheckpointGuards`, `TestDriverTamperedIntegration` |
| Traçabilité : le passage délégué est écrit dans la frise de l'enfant (`delegated`, avec le checkpoint et la session parente) et dans la trace de la parente (`fanout`, `child_checkpoint`), avec chaque étape du fan-out | `oh session trace`, `oh fanout status` |
| Drapeau expérimental, désactivé par défaut : sans lui, le lancement est refusé et le démon ne délègue rien | `[fanout] experimental`, `TestDriverRefusals` |
| Tour de correction (`cp-merge` refusé avec une consigne) : la session de correction n'est jamais un enfant (rôle `fixer`, son `cp-commit` validé par l'utilisateur, aucune délégation), sans push, merge, fermeture ni déplacement de branche ; oh ne reprend que des commits par-dessus la branche d'intégration, sans merge ni rien de non commité ; `cp-merge` se repasse et se revérifie sur la nouvelle tête | `TestFanoutFixerNotDelegated`, `TestFanoutMergeRefusedAsksForAFix`, `TestFixRound`, `TestFixRoundRefused` |
| Enfants issus du découpage (`sweep`, `fanout.source: tasks`) : les enfants sont les sous-tâches **enregistrées par oh** à `cp-plan` (`tasks:`), jamais une liste déclarée par un agent ; ce découpage est vérifié par oh avant la carte (fichiers du dépôt, périmètres disjoints, au plus `max`), et l'appel qui passe `cp-plan` doit apporter celui de la carte ; la vérification du `cp-commit` délégué d'un enfant échoue sur un changement hors du périmètre de sa sous-tâche ; aucun ticket, rien à fermer ; le `developer` n'est pas dans la parente (`agents.developer.fanout: never`) ; une couche ne change ni `source` ni ce scope, et ne retire pas `tasks:` | `TestFanoutTasks_*`, `TestShippedSweepFanout`, `TestShippedSweepFanoutTeamLayer`, `TestTasksRefusedBeforeAnyCard`, `TestTasksChangedAfterTheCardRefused`, `TestTaskScopeOfTheChild`, `TestDriverTasksBecomeChildren`, `TestDriverTasksNoneRecorded` |

Une couche qui modifie le checkpoint délégué (`fanout: never` : `cp-2` d'`implement-spec`, `cp-commit` de `sweep`) le garde hors fan-out seulement : avertissement `fanout_never_patched`, listé par `oh doctor` ; pour durcir le fan-out, c'est `cp-merge` qu'elle modifie.

Le `cp-2` des enfants n'est délégué que si la session parente porte le checkpoint de merge : dans une couche d'équipe qui déclare son propre fan-out sans `delegates`, chaque enfant garde sa pause, et ces pauses se valident par lot (ci-dessous).

## Validation par lot (CP-R6)

Plusieurs checkpoints en attente (enfants d'un fan-out sans délégation, sessions distantes, `cp-merge` de plusieurs fan-outs) se valident en une fois, chacun restant listé et refusable :

```
oh session approve --batch --fanout <session parente> --dry-run     # liste : décision, session, checkpoint, résumé, vérification
oh session approve --batch --fanout <session parente> --except <décision> --reject <décision>=<consigne>
oh session approve --batch --checkpoint cp-2,cp-merge
```

Chaque validation passe par le même chemin qu'une validation seule : une vérification échouée refuse celle-là seulement (elle reste en attente, la commande le signale). Dans la TUI, touche `B` de la vue Sessions : les checkpoints en attente du fan-out de la session sélectionnée (sinon tous), chacun à valider, laisser en attente ou refuser avec sa consigne, puis une confirmation ; même service que la commande.

---

## Fenêtres de déverrouillage bornées (CP-R8)

| Opération | Ouverte par | Refermée |
|---|---|---|
| `create` | son checkpoint (`cp-tickets`, `cp-ticket`) | au **checkpoint obligatoire suivant** (`cp-spec`, `cp-commit`…) ou quand le checkpoint est redemandé ; pas à la fermeture d'un ticket |
| `update` | son checkpoint (`cp-triage`) | comme `create` : au checkpoint obligatoire suivant ou quand le checkpoint est redemandé ; pas à la fermeture d'un ticket |
| `commit` | son checkpoint (`cp-2`, `cp-commit`) | **une fois le travail commité** (HEAD a avancé et il ne reste rien à committer : un commit, ou plusieurs à la suite), quand le checkpoint est redemandé, ou à la fermeture du ticket |
| `close` | son checkpoint | à la fermeture d'un ticket, ou quand le checkpoint est redemandé |
| `tag` | son checkpoint (`cp-release`) ; avec `commit` au même checkpoint, seulement **une fois la release commitée** | après le premier tag, au checkpoint obligatoire suivant ou quand le checkpoint est redemandé |
| `mr` | son checkpoint (`cp-release`) | au checkpoint obligatoire suivant ou quand le checkpoint est redemandé |
| `apply`, `push` | leur checkpoint | quand le checkpoint est redemandé |

`apply`, `tag` et `mr` sont refusés dans tout workflow qui ne déclare aucun checkpoint pour les déverrouiller. Seul un workflow `risk: publish` déverrouille `tag` et `mr`, à un checkpoint qui fait une pause dans tous les modes et ne tourne jamais en distant (`remote: forbid`).

En cas d'échec de lecture git, ces protections restent **fermées** : si l'état du travail ne peut pas être lu après un appel shell, la fenêtre `commit` se referme (la frise en garde la raison ; redemander le checkpoint la rouvre) au lieu de rester ouverte ; la fermeture d'un ticket (`bd close`) est refusée tant qu'oh ne peut pas constater un commit non vide depuis le checkpoint et un arbre propre.

---

## Qui passe un checkpoint (`by:`, CP-R11)

Chaque checkpoint nomme les agents qui le passent (`by:` ; sinon déduit du graphe de délégation). Chaque agent ne voit que les siens : dans son contrat de session et dans la table générée des checkpoints. Un checkpoint passé par un autre agent n'est pas refusé : oh l'écrit dans son journal et la réponse de `workflow_checkpoint` porte un avertissement (« vérifie que c'est bien ton checkpoint »).

---

## Valideur humain (`approver:`, CP-R9)

`by:` dit quel agent passe un checkpoint ; `approver:` dit quel **humain** doit le valider (étape 1, [ADR-055](../architecture/adr/055-checkpoint-approver.fr.md)) :

```yaml
checkpoints:
  cp-apply:
    approver: [Grace Hopper, lead@example.com]
```

- **Qui le déclare** : une couche d'équipe seulement (team-state). Ni le hub, ni une couche de projet, ni les options de lancement ; une couche de projet ne le retire ni ne le change, ne retire pas le checkpoint et ne le fait pas moins mettre en pause. Format et diagnostics : [Schéma des workflows](workflow-schema.fr.md#approver--valideur-humain).
- **Identité** : celle de l'[ADR-052](../architecture/adr/052-beads-identity-provenance.fr.md), `user.name` (et `user.email`) de git dans l'emplacement de la session, sinon le nom de connexion. Une entrée avec `@` se compare à l'e-mail, les autres au nom (sans casse ni espaces en trop).
- **En local, oh ne bloque pas** : il ne peut pas savoir qui est devant la machine. La carte de la TUI et `oh session approve` affichent « Valideur attendu : … » ; si la personne qui valide n'en fait pas partie, un avertissement le dit (la TUI demande une confirmation sur `y`) et la validation est acceptée, tracée « hors valideur ». La validation par lot (`--batch`, TUI `B`) l'affiche pour chaque élément.
- **Traçage** : chaque validation ou refus d'un humain, avec ou sans valideur, enregistre dans les événements du checkpoint `identity` (qui), `at` (quand), `approver` (valideurs attendus) et `approver_mismatch` ; `oh session trace` les montre (« par … », « ⚠ hors valideur »). Le mode (`auto`) et une délégation (CP-R3) passent sans identité ; dans un job distant, `remote: auto` valide sous l'identité `remote` (hors valideur, avertissement `checkpoint_approver_remote_auto` : préférez `remote: defer`).
- **Étape 2, non faite** : en session distante, un checkpoint avec valideur ne passera que sur la réponse d'un valideur à l'identité attestée (compte de la forge ou signature) ; voir l'ADR.

---

## Carte de décision (CP-R5, partie vérification)

Pour un checkpoint en pause, la carte de la TUI affiche, en plus du résumé de l'agent, des changements, des derniers messages et de la frise :

- **Valideur attendu** (`approver:`, CP-R9) : les valideurs du checkpoint et, si la personne qui valide n'en fait pas partie, l'avertissement « hors valideur » ;
- **Vérification lancée par oh** : statut, commandes et code de sortie, extrait de la sortie en cas d'échec ; pour une session distante récupérée, le résultat obtenu dans le job ;
- **Preuves de l'agent** : chaque commande annoncée, observée ou non ;
- **Non résolu** : le ticket dont les tours de correction ont atteint la limite ;
- **Propositions du wiki** (CP-R4, checkpoint qui déverrouille le commit seulement) : les propositions `wp-…` de la session (auteur, page, section, tag, début du contenu, échappé), avec une action par proposition (accepter, modifier — sur une ligne dans le champ, `⏎` pour un retour à la ligne, ou sur plusieurs lignes avec « Modifier dans l'éditeur », qui ouvre `$EDITOR` —, refuser, laisser en file) et « Accepter les N propositions ». Hors de la TUI : [`oh wiki decide`](cli-projects.fr.md#oh-wiki-decide). Les acceptées sont appliquées par oh au passage du checkpoint, avant le commit ; voir [Serveur MCP wiki](cli-mcp.fr.md#serveur-mcp-wiki). Sur un checkpoint `wiki: sync` (`doc-sync`), la carte montre toute la file du projet, propositions des autres sessions comprises.

Choix : Valider · Corriger d'abord · Autre consigne · **Passer outre la vérification** (seulement si elle a échoué ou ne peut pas tourner).
