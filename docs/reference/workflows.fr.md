> [Read in English](workflows.en.md)

# Workflows livrés par le hub

> v5 : chaque cas d'usage est un workflow déclaratif (`apiVersion: oh/v1`) livré par le hub. Les fichiers sont dans `workflows/` à la racine du dépôt, embarqués dans le binaire et extraits dans `~/.oh/hub/workflows/`. Une équipe ou un projet peut les étendre (`extends`) dans le team-state.

Voir aussi : [schéma `oh/v1`](workflow-schema.fr.md) (référence de chaque champ) · [workflows d'équipe](../guides/team-workflows.fr.md).

Vérifier les workflows du hub :

```bash
oh workflow validate --all          # tous les workflows du hub
oh workflow validate ticket         # un workflow du catalogue
oh workflow validate ./mon-wf.yaml  # un fichier
```

On lance un workflow avec `oh run <workflow>` ou la fiche de lancement de la TUI ; les anciennes commandes (`oh start`, `oh audit`…) en sont des alias dépréciés.

---

## Vue d'ensemble

| Workflow | Agent d'entrée | Agents | Risque | Exécution | Remplace |
|---|---|---|---|---|---|
| `ticket` | `orchestrator-dev` | `developer`, `reviewer`, `documentarian` (à la demande) | write | local, conteneur, distant | `oh start --dev`, `-t` |
| `quick` | `developer` | — | write | local | session rapide |
| `infra-change` | `operator` | — | write (infrastructure, aucun fichier) | local | ancien agent `infra` (partie application) |
| `spec` (alias `cadrage`, `feature`) | `conductor` | `pathfinder` (si la demande compare des conceptions), `planner` (écrit `docs/wiki/specs/`), `designer` | plan (+ `writes:`) | local | `cadrage` (ancien mode A sans implémentation), avec une spec écrite et vérifiée |
| `implement-spec` (alias `feature` avec `--tickets`) | `orchestrator-dev` | `developer`, `reviewer`, `documentarian` (à la demande) ; en fan-out : `reviewer` et une session enfant `ticket` par ticket | write | local | partie implémentation de l'ancien `feature` (mode B : tickets existants) |
| `merge-conflicts` | `merger` | — | write (conclut un merge en cours ; jamais de push) | local | — (lancé par oh dans un fan-out) |
| `onboarding` | `onboarder` | — | write (`docs/wiki/`) | local, distant | `oh start --onboard` (ancien mode C) |
| `doc-sync` | `documentarian` | — | write (`docs/wiki/`) | local | — |
| `skill-refresh` | `documentarian` | — | write (skills de stack du hub) | local | — |
| `release` | `release-manager` | — | publish (CHANGELOG, commit et tag de release ; jamais de push) | local | — |
| `review` | `reviewer` | — | read | local, distant | `oh review` |
| `review-feedback` | `orchestrator-dev` | `developer`, `reviewer` | write | local | `oh review feedback` |
| `audit` | `auditor` | `auditor-subagent` | read | local, distant | `oh audit`, anciens agents `infra` et `database` (partie analyse) |
| `debug` | `debugger` | `developer` (à la demande) | write | local | `oh debug` (ancien mode D) |
| `incident` | `debugger` (écrit `docs/wiki/postmortems/`) | `developer` | write | local | — |
| `security-fix` | `conductor` | `developer`, `auditor-subagent` | write | local | — |
| `spike` | `pathfinder` (écrit `docs/wiki/decisions/`) | `developer` (prototype jetable) | write | local | — |
| `retro` | `debugger` (écrit `docs/wiki/retros/`) | — | write | local | — |
| `triage` | `planner` | — | plan | local | — |
| `benchmark` | `benchmarker` | — | write (`reports/benchmarks/`) | local | `oh run libre --agent benchmarker` |
| `sweep` | `conductor` | `pathfinder`, `developer`, `reviewer` ; en fan-out : `pathfinder`, `reviewer` et une session enfant `sweep-task` par sous-tâche | write | local | `oh start --sweep` |
| `sweep-task` | `developer` | — | write | local | — (lancé par oh dans un sweep en fan-out) |
| `brief-enrich` | `brief-enricher` | — | read | local | `oh takeover-brief enrich` |
| `libre` | au choix (`orchestrator` par défaut, son seul workflow) | ceux que l'agent choisi peut appeler | write | local, conteneur | `oh start --agent`, session libre de la TUI |

### Niveaux de risque

| `risk` | Fichiers | Beads |
|---|---|---|
| `read` | aucune modification, shell restreint | lecture seule ; `beads.allow` obligatoire, sans commande d'écriture |
| `plan` | aucune modification, shell restreint ; seuls les chemins de documentation de `writes:` (et leur commit) sont ouverts aux agents qui les déclarent | `beads.allow` obligatoire ; écritures permises sauf `delete` |
| `write` | modifiés | `beads.allow` du workflow ; lecture seule si `beads` est absent |
| `publish` | modifiés, tags créés, MR ouvertes ; jamais en distant | idem |

Une couche supérieure (équipe, projet) peut seulement durcir le risque : `read < plan < write < publish`.

### Préconditions

Un workflow peut déclarer des tests faits avant le lancement, dans l'emplacement de la session :

```yaml
preconditions:
  project-context:
    label: { fr: Aucun contexte projet trouvé, en: No project context found }
    check: { path_exists: [docs/wiki/index.md, ONBOARDING.md, CONVENTIONS.md] }  # l'un des chemins suffit
    on_fail: suggest          # suggest (défaut) | block
    suggest: { workflow: onboarding, resume: true }
```

- `on_fail: suggest` : oh affiche le message et propose le workflow `suggest.workflow` ; l'utilisateur peut aussi continuer. Avec `resume: true`, oh propose de relancer le workflow d'origine (mêmes entrées) une fois le workflow proposé terminé.
- `on_fail: block` : le lancement est refusé.
- En distant ou sans interface : un échec `suggest` devient un avertissement, un échec `block` arrête la session.
- Autres tests : `git_clean: true` (aucune modification non commitée, `.beads/` exclu), `branch: [main, "release/*"]` (branche courante), `tools: [goreleaser]` (commandes dans le `PATH`) ; en cas d'échec, le message dit ce qui a été trouvé (modifications, branche, outils manquants). Voir le [schéma](workflow-schema.fr.md#préconditions).
- Chemins relatifs, sans `..`. Un patch (`extends`) modifie une précondition par son identifiant ou la retire (`disabled: true`).

`release` refuse de se lancer sur un arbre sale ou hors de `main`, `master` et `release/*`. `spec` (et ses alias `cadrage` et `feature`) et `implement-spec` proposent `onboarding` quand le projet n'a ni wiki, ni `ONBOARDING.md`, ni `CONVENTIONS.md`.

---

## `ticket`

Implémente un ou plusieurs tickets Beads déjà détaillés : routage vers le bon développeur, pre-review, review, commit.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `ticket` | `beads-id` (sélecteur : `ai-delegated`, multi-sélection) | oui | Ticket à implémenter ; plusieurs tickets sélectionnés = une session par ticket |
| `method` | `enum` (`standard`, `tdd`) | non (`standard`) | Méthode : `tdd` ajoute `cp-seams` et `cp-red` (tests écrits d'abord) |
| `instructions` | `text` (4 000 caractères max) | non | Précisions pour la session |

| Checkpoint | manuel | semi-auto | auto | distant |
|---|---|---|---|---|
| `cp-1` Démarrer le ticket | pause | auto | auto | auto |
| `cp-seams` Points d'injection et tests prévus (méthode `tdd`) | pause | pause | auto | auto |
| `cp-red` Tests en échec (méthode `tdd`, vérifié par oh) | auto | auto | auto | auto |
| `cp-2` Commit ou correction (obligatoire) | pause | pause | pause | attend l'utilisateur |
| `cp-3` Ticket suivant | pause | auto | auto | auto |

Mode par défaut : `semi-auto`. Commandes Beads autorisées : `show`, `list`, `ready`, `children`, `dep`, `update`, `close`, `comments`, `label`. Sorties : `branch`, `tickets`.

En méthode `tdd` (`-i method=tdd`), le `developer` passe lui-même `cp-seams` (il présente les points d'injection, les interfaces et les tests prévus) puis `cp-red`, où oh vérifie que les tests écrits échouent pour la bonne raison (le code construit, les tests compilent et échouent) et qu'un fichier de test a changé depuis `cp-seams` ; les autres fichiers modifiés sont signalés. Sans cette méthode, rien ne change : ni checkpoint, ni texte. Voir [Méthode TDD](checkpoints.fr.md#méthode-tdd--cp-seams-et-cp-red-r7).

`cp-2` déverrouille `commit` et `close` : avant sa validation, oh refuse à tous les agents `git commit` (et `revert`, `cherry-pick`, `merge`, `pull`, qui commitent aussi) et la fermeture du ticket. Le push revient à l'utilisateur (aucun workflow livré ne le déverrouille). Avant que `cp-2` passe, oh lance lui-même le build, les tests et le typecheck de la stack détectée (`verify: { from: stack, expect: pass }`) : la carte de décision affiche le résultat, un échec n'ouvre rien et l'utilisateur peut passer outre (choix tracé). Même vérification sur `cp-2` d'`implement-spec` et `review-feedback`, et sur `cp-commit` de `quick`, `debug`, `sweep` et `sweep-task` ; voir [Checkpoints](checkpoints.fr.md). Une fois `cp-2` validé, l'`orchestrator-dev` relance le `developer`, qui commite puis ferme le ticket ; oh refuse la fermeture tant que le travail n'est pas commité. Le `reviewer` lance le build et les tests du projet (et le lint, le typecheck) avant sa review, avec les commandes de la stack détectée, autorisées ici : un échec est un finding 🔴 (verdict `corriger`) ; les fichiers produits par le build et non ignorés sont signalés, la fermeture exigeant un arbre propre. Il en va de même dans `implement-spec`, `sweep`, `review-feedback` et `libre`. Une question qui imite un checkpoint (« CP-2 — commit ? » posée avec l'outil `question`) est fermée par oh, qui rappelle à l'agent d'appeler `workflow_checkpoint`. Quand le ticket de la session est fermé (ou le dernier checkpoint passé) et que l'étape se termine, oh déclare les sorties manquantes (`branch`, `tickets`) et la session passe à **terminée** : « Enchaîner avec… » est proposé.

## `feature` (alias de `spec` → `implement-spec`)

`feature` n'est plus un workflow (`workflows/feature.yaml` est retiré) : c'est l'enchaînement [`spec`](#spec) puis [`implement-spec`](#implement-spec), routé par les entrées (voir [`aliases`](workflow-schema.fr.md#aliases--autres-noms-du-workflow)) :

| Lancement | Workflow lancé | Ancien mode |
|---|---|---|
| `oh run feature -i request="…"` (et `oh start -m "…"`) ; sans demande, la session la demande | `spec` : `aliases: [cadrage, feature]` | A (feature en langage naturel) |
| `oh run feature --tickets bd-1,bd-2` | `implement-spec` : `aliases: [{ id: feature, with: tickets }]` ; `-m` va à son entrée `instructions` | B (tickets existants) |

oh affiche la redirection (`feature → spec`). À la fin de `spec`, oh propose `implement-spec` sur les tickets créés et la spec écrite, sans le lancer (`oh session next <id>`, fermeture du client, « Enchaîner avec… » dans la TUI).

**Méthode TDD transmise.** `spec` accepte l'entrée `method` (`standard` ou `tdd`) sans l'utiliser lui-même : oh la relève au lancement (sortie `method`, `record: input`) et la passe à `implement-spec` dans la suite proposée (`-i method=tdd`). `oh run feature -i method=tdd -i request="…"` donne donc une spec, puis une implémentation en TDD.

Ce qui change par rapport à l'ancien `feature` (⚠) :

- sans demande (`oh run feature`, `oh start`), la session la demande d'abord, comme l'ancien `feature` : le `conductor` la pose avec `question` ;
- la spec est écrite et vérifiée dans le wiki (`docs/wiki/specs/`), les tickets créés à `cp-tickets` ; l'implémentation est une **seconde session**, proposée par oh ;
- checkpoints : ceux de `spec` (`cp-understanding`, `cp-converge`, `cp-spec`, `cp-tickets`), puis ceux d'`implement-spec` (`cp-plan` remplace `cp-0`, `cp-final` remplace `cp-feature` et sa revue de l'ensemble) ; `cp-spec` (spec UX/UI conditionnelle) disparaît : le designer est appelé par le planner de `spec` ;
- l'`orchestrator` n'est plus l'agent d'entrée que de [`libre`](#libre) ;
- les commandes Beads, la vérification de `cp-2` et le verrou de commit sont ceux d'`implement-spec`.

**Couches d'équipe et de projet.** Un document qui étend `hub:feature` (ou `hub:cadrage`) ne se résout plus : oh refuse avec une erreur explicite, `extends_alias` (« hub:feature n'est plus un workflow : c'est un alias de implement-spec, spec… »), au lancement comme à `oh workflow validate`, et `oh doctor` liste les workflows publiés concernés (« Workflows retirés »). Il n'y a pas de migration automatique : les patchs de `feature` visent des agents et des checkpoints (`orchestrator`, `cp-0`, `cp-spec`, `cp-feature`) qui n'existent ni dans `spec` ni dans `implement-spec`, et les rediriger en silence pourrait perdre une règle de l'équipe (`enforce`, checkpoint rendu obligatoire). Pour migrer, au choix :

- **supprimer** le document : `oh run feature` reprend l'alias ;
- le **réécrire** comme couche de `spec` ou d'`implement-spec` (`extends: hub:spec`, `extends: hub:implement-spec`), en portant les changements sur leurs agents et leurs checkpoints ; un document qui garde l'id `feature` masque l'alias (avertissement `alias_shadowed`).

Un document `feature` complet, sans `extends`, reste un workflow d'équipe à part entière (il masque aussi l'alias). La migration des anciennes configurations (v38) écrit désormais un patch d'`implement-spec` (`cp-0` devient `cp-plan` ; les agents absents d'`implement-spec` sont listés en commentaire, non traduits).

## `quick`

Session de développement directe avec l'agent `developer`, sans planification. Un seul checkpoint, `cp-commit` (obligatoire, en pause dans tous les modes) : il déverrouille `commit` et `close` ; avant lui, oh refuse tout `git commit` et la fermeture d'un ticket. Le push revient à l'utilisateur. Le développeur déduit le domaine de la demande et charge les standards correspondants. Il propose `feature` si la demande s'avère importante. Commandes Beads autorisées : `show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`, `update`, `close`, `dep`, `label`.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `request` | `text` (8 000 caractères max) | non | Ce qu'il faut faire ; sinon, la session attend la demande |

## `spec`

Comprend une demande, écrit sa spec dans le wiki et la découpe en tickets Beads, sans l'implémenter (`risk: plan`). Le `conductor` relance le `planner` à chaque étape ; le planner reste en lecture seule, sauf sa spec et sa ligne dans l'index du wiki : `agents.planner.writes: ["docs/wiki/specs/**", "docs/wiki/index.md"]` ([`writes`](workflow-schema.fr.md#writes-chemins-de-documentation)).

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `request` | `text` (8 000 caractères max) | non | La feature à spécifier ; sans elle (ni `source`), le `conductor` la demande à l'utilisateur avec `question` avant toute étape |
| `source` | `path` | non | Page du wiki à l'origine de la demande (décision d'un `spike`, postmortem d'un `incident`), donnée par leur `next:` ; le planner la lit à l'entretien |
| `method` | `enum` (`standard`, `tdd`) | non (`standard`) | Méthode de l'implémentation proposée ensuite : `spec` ne s'en sert pas, oh la relève et la passe à `implement-spec` |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-understanding` Valider la compréhension (reformulation, vocabulaire, réponses aux questions) | pause | pause | auto |
| `cp-converge` Convergence de la spec et du découpage (vérifié par oh) | auto | auto | auto |
| `cp-spec` Valider la spec (obligatoire, vérifié par oh, déverrouille `commit`) | pause | pause | pause |
| `cp-tickets` Valider le découpage (obligatoire, vérifié par oh, déverrouille `create`) | pause | pause | pause |

Déroulé :

1. **Entretien** : le planner explore le code et charge `grilling` ; ses questions bloquantes (une par décision) sont posées par le `conductor`. Il rend sa reformulation et le vocabulaire (chaque mot de la demande en face des noms du code), présentés à `cp-understanding`.
2. **Spec** : une fois `cp-understanding` passé (avant, oh refuse l'écriture de la spec et dit au planner d'attendre ce checkpoint), le planner écrit `docs/wiki/specs/<slug>.md` : objectif, périmètre, **glossaire**, **critères d'acceptation** (`CA-n`, chacun avec son point de test sur sa ligne : `· Test : TestPercent_ZeroTotal`), décisions et **découpage** en tranches verticales (`Couvre : CA-…`). Le format est dans le prompt du planner (section « Spec écrite »), pas dans une skill à charger.
3. **Passe de convergence** (`cp-converge`, sans pause) : oh lit la spec et vérifie le glossaire, le point de test de chaque critère, que chaque critère est couvert par une tranche et qu'aucune tranche n'est hors spec. Un écart refuse le checkpoint avec la liste des problèmes : le planner corrige (ou pose une question si l'écart demande une décision), puis le checkpoint est redemandé.
4. **`cp-spec`** : l'utilisateur valide la spec (même vérification) ; le planner la commite avec sa ligne dans `docs/wiki/index.md` (s'il existe) : `git add docs/wiki/specs/<slug>.md docs/wiki/index.md`, puis `git commit -m "docs(spec): <slug>" -- docs/wiki/specs/<slug>.md docs/wiki/index.md`.
5. **`cp-tickets`** : l'utilisateur valide le découpage ; le planner crée un ticket par tranche, dont la description cite la spec et ses critères. Une spec ou un découpage modifiés à un checkpoint repassent par `cp-converge`, `cp-spec` (nouveau commit) et `cp-tickets`.

Ordre imposé par oh : `cp-converge` est refusé avant `cp-understanding`, `cp-spec` avant `cp-converge`, `cp-tickets` avant `cp-spec` (`after:`) ; le refus nomme le checkpoint à passer d'abord.

Exploration : quand la demande compare des conceptions ou demande d'analyser le code avant de spécifier, le `conductor` lance d'abord le `pathfinder` (skill `codebase-design`) et passe son rapport au planner ; sinon l'entretien commence directement.

Budget de pauses (CP-R2) en semi-auto : 3 pauses (1 décision, 2 actions irréversibles), pour un budget de 4. Mode par défaut : `semi-auto`. Commandes Beads autorisées : `show`, `list`, `ready`, `children`, `search`, `query`, `count`, `lint`, `find-duplicates`, `label`, `dep`, `create`, `update`, `comments`, `duplicate`, `supersede`.

Sorties **relevées par oh** (aucun agent ne les déclare) : `spec` (la spec vérifiée à `cp-converge` et `cp-spec`), `tickets` (les tickets créés après `cp-tickets`) et `method` (l'entrée du lancement, `record: input`). Suite proposée (`next:`) quand des tickets ont été créés : **`implement-spec`** sur ces tickets, cette spec et cette méthode ; oh l'affiche à la fermeture du client (`oh run`), dans `oh session next <id>` et dans « Enchaîner avec… » de la TUI, sans la lancer.

**`cadrage` et `feature` sont des alias de `spec`** (R5, feature-migration) : `oh run cadrage` et `oh run feature` lancent `spec` (`aliases: [cadrage, feature]` ; `feature` avec `--tickets` lance `implement-spec`, voir [`feature`](#feature-alias-de-spec--implement-spec)). L'ancien `cadrage` (exploration systématique, `cp-scope`, `cp-recap`, aucun fichier écrit) n'existe plus ; une couche d'équipe ou de projet qui définit un workflow `cadrage` l'emporte sur l'alias, une couche qui étend `hub:cadrage` est refusée (`extends_alias`).

## `implement-spec`

Implémente les tickets d'une spec l'un après l'autre, dans une seule session (`risk: write`), puis fait revoir l'ensemble. C'est la partie implémentation de l'ancien `feature` (alias `feature` avec `--tickets`), sans planification : l'`orchestrator-dev` lit lui-même les tickets (`bd show`) et fixe l'ordre (dépendances, puis priorité). Par défaut, version **séquentielle** dans la session ; avec `fanout=true` (expérimental), **sessions enfants** (voir plus bas).

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `tickets` | `beads-ids` | oui | Tickets à implémenter ; par défaut `session.output(spec.tickets)` : ceux de la session `spec` dont le lancement découle (`--parent`), sinon de la dernière session `spec` du projet |
| `spec` | `path` | non | Spec des tickets ; par défaut `session.output(spec.spec)` |
| `method` | `enum` : `standard`, `tdd` | non (`standard`) | Méthode TDD : `cp-seams` et `cp-red` pour chaque ticket |
| `fanout` | `bool` (défaut : non) | non | Expérimental (`[fanout] experimental = true`) : une session enfant par ticket, chacune dans son worktree, mergées par oh |
| `instructions` | `text` (4 000 caractères max) | non | Précisions pour la session |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-plan` Valider le plan (obligatoire) : ordre des tickets, agent prévu, tickets écartés | pause | pause | pause |
| `cp-1` Démarrer le ticket | pause | auto | auto |
| `cp-seams`, `cp-red` (méthode `tdd`) | comme dans `ticket` | | |
| `cp-2` Commit ou correction (obligatoire, vérifié par oh, déverrouille `commit` et `close`) | pause | pause | pause |
| `cp-3` Ticket suivant | pause | auto | auto |
| `cp-final` Revue de l'ensemble et récap | pause | pause | auto |

Aucune délégation avant `cp-plan`. Chaque ticket passe par son `cp-2` (commit, puis fermeture, avant le suivant). Tous les tickets traités, le `reviewer` fait une review adversariale de tous les commits de la session ; `cp-final` présente son verdict, les tickets et leurs commits, les points ouverts : l'utilisateur décide d'un tour de correction (un seul, repassé par `cp-2`) ou de s'arrêter.

Budget de pauses (CP-R2) en semi-auto : `cp-plan`, `cp-2`, `cp-final`, soit 3 pauses pour un budget de 3 (2 décisions, 1 action irréversible) ; la méthode `tdd` en ajoute une (`cp-seams`). Une feature de N tickets coûte donc N + 2 pauses (au lieu de N + 3 dans l'ancien `feature`). Mode par défaut : `semi-auto`. Exécution : locale. Commandes Beads autorisées : `show`, `list`, `ready`, `children`, `dep`, `update`, `close`, `comments`, `label`. Sorties : `branch` (déclarée), `tickets` (tickets fermés, relevés par oh).

### Fan-out (`fanout=true`, expérimental)

`oh run implement-spec -i fanout=true` (avec `[fanout] experimental = true`) : la session parente garde `cp-plan`, `cp-merge` et `cp-final` ; `cp-1`, `cp-2`, `cp-3`, `cp-seams` et `cp-red` passent dans les enfants, et le developer n'est pas dans le paquet de la parente.

1. `cp-plan` validé, oh crée la branche d'intégration `oh/fanout/<session>/integration` (worktree voisin) depuis la branche de la session, puis lance une session enfant `ticket` par ticket (sa branche `oh/fanout/<session>/<ticket>` depuis la branche d'intégration, son worktree), 1 à 3 à la fois (`fanout.parallel` du workflow et `[fanout] parallel`), un ticket après ceux dont il dépend, aucun pour un ticket déjà fermé ; les enfants héritent du mode de la parente et des entrées déclarées par `fanout.inputs` (`method`, `instructions`).
2. Chaque enfant implémente, fait relire et passe son `cp-2` **sans pause** (délégué à `cp-merge`, CP-R3), vérifié par oh dans son worktree, puis commite ; il ne ferme pas son ticket, ne merge pas et ne pousse pas (oh le refuse).
3. Un enfant terminé (session arrêtée, ou fin de travail constatée par oh, son tour fini sans décision en attente : checkpoint de commit passé, un commit sur sa branche, rien de non commité ; garde-fou : 10 minutes d'inactivité sans ce signal), oh le merge dans la branche d'intégration (`--no-ff`). Un conflit : oh lance [`merge-conflicts`](#merge-conflicts) dans le worktree d'intégration (garde-fou du merger : 2 minutes d'inactivité sans merge conclu), puis vérifie le résultat (merge commité, les deux côtés dans l'historique, aucun marqueur) ; sinon il remet la branche d'intégration où elle était et l'enfant reste non mergé. Sans commit, avec des modifications non commitées, ou arrêté par le budget global : non mergé.
4. Tous les enfants terminés, oh écrit à la session parente (tableau des enfants) ; le `reviewer` revoit l'ensemble de la branche d'intégration, puis `cp-merge` (obligatoire, pause dans tous les modes) : oh y lance la vérification **sur la branche d'intégration**, jamais forcée ; validé, oh merge la branche d'intégration dans celle de la session et ferme lui-même les tickets mergés (sortie `tickets`).
5. `cp-merge` refusé avec une consigne : **tour de correction** (`fanout.fix`) ; oh lance une session [`quick`](#quick) dans le worktree d'intégration (rôle `fixer` : ni push, ni merge, ni fermeture, ni déplacement de branche) avec la consigne et ce qui a échoué, son `cp-commit` validé par l'utilisateur (jamais délégué) ; oh reprend ses commits s'ils sont par-dessus la branche d'intégration, sans merge et sans rien de non commité, sinon il remet la branche où elle était ; la parente fait relire la correction et repasse `cp-merge`, revérifié sur la branche d'intégration. Au plus `limits.max_fix_rounds` tours (3).
6. `cp-final` : récap ; un nouvel `implement-spec` pour les tickets non mergés. oh supprime ensuite les worktrees et branches `oh/fanout/…` (après `cp-merge`, ou à l'arrêt de la parente ; jamais une branche dont les commits ne sont sur aucune autre : `oh fanout clean --discard`).

Budget de pauses en semi-auto : `cp-plan`, `cp-merge`, `cp-final`, soit **3 pauses quel que soit le nombre de tickets** (2 de plus par tour de correction). Budget global (F-04) : la parente et ses enfants ne dépassent pas `[fanout] budget_usd` (10 $ par défaut) ; au-delà, oh ne lance plus d'enfant et arrête celui qui tourne (état `budget`), et chaque enfant reçoit ce qui reste comme budget de session. État : `oh fanout status <session>` ; dans `oh session list` et la TUI, un enfant est marqué `↳` (avec son ticket), la parente `⑂`. Guide : [Fan-out](../guides/fan-out.fr.md).

## `merge-conflicts`

Termine un merge arrêté sur des conflits (`risk: write`, mode `auto` seulement, aucun checkpoint) : le `merger` charge `resolving-merge-conflicts`, garde l'intention des deux côtés, cherche les conflits sans marqueur, vérifie (build, tests), `git add` puis `git commit --no-edit`. Il ne lance, n'annule ni ne pousse aucun merge, ne change pas de branche et ne ferme aucun ticket ; s'il ne peut pas établir l'intention d'un côté, il laisse le merge en cours et l'explique. Entrées : `branch` (obligatoire, la branche mergée), `into`, `files` (fichiers en conflit). Lancé par oh dans un fan-out ; utilisable à la main sur un merge en cours (`oh run merge-conflicts -i branch=<branche>`).

## `onboarding`

Découvre le projet et crée ou enrichit le wiki `docs/wiki/` (protocole `doc-wiki-protocol`).

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `refresh` | `bool` (défaut : non) | non | Re-découvrir le projet et proposer les mises à jour du wiki existant (validées une à une à la carte de `cp-wiki`), sans rien supprimer |
| `focus` | `text` (4 000 caractères max) | non | Modules ou sujets à approfondir |

L'onboarder n'écrit que dans `docs/wiki/`, `ONBOARDING.md` et `CONVENTIONS.md` : c'est une permission (toute autre écriture est refusée, `.gitignore` et `.git/` compris). Sans wiki, il crée les pages ; avec un wiki (`docs/wiki/index.md`), il n'en modifie aucune et propose chaque mise à jour avec `wiki_propose` (rien si le wiki est fidèle au code). Il passe ensuite `cp-wiki` (obligatoire, déverrouille le commit, `remote: defer`) : la carte montre ses propositions, l'utilisateur accepte, modifie ou refuse chacune, oh applique les acceptées ; puis l'onboarder commite en un commit `docs/wiki/`, `ONBOARDING.md` et `CONVENTIONS.md` (`writes: ["docs/wiki/**", ONBOARDING.md, CONVENTIONS.md]`), rien d'autre. Le push revient à l'utilisateur. Une pause en semi-auto (budget CP-R2 : une action irréversible). Sortie : `wiki`. Commandes Beads autorisées (lecture seule) : `show`, `list`, `search`, `query`.

## `doc-sync`

Met le wiki à jour avec les commits depuis la dernière synchronisation, puis traite les propositions en file (celles des workflows sans commit : `review`, `audit`…). Précondition : `docs/wiki/index.md` existe (sinon, oh propose `onboarding`).

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `focus` | `text` (4 000 caractères max) | non | Pages ou sujets à revérifier en priorité |

| Checkpoint | manuel | semi-auto | auto | distant |
|---|---|---|---|---|
| `cp-sync` Commit du wiki (obligatoire, déverrouille `commit`, `wiki: sync`) | pause | pause | pause | attend l'utilisateur |

Le `documentarian` appelle `wiki_check` avec `since_sync` (commits, fichiers modifiés, pages concernées et dérives calculés par oh), met à jour les pages, vérifie chaque proposition en file, puis présente `cp-sync`. La carte montre toute la file du projet : l'utilisateur accepte, modifie ou refuse chaque proposition. Au passage, oh applique les acceptées et note la synchronisation (`synced:` dans `docs/wiki/index.md`) ; le `documentarian` commite le wiki seul. Une pause en semi-auto (budget CP-R2 : une action irréversible). Commandes Beads autorisées (lecture seule) : `show`, `list`, `ready`, `search`, `query`. Détails : [Wiki documentaire vivant](../architecture/living-wiki.fr.md#synchronisation--workflow-doc-sync-ld-d).

## `skill-refresh`

Revoit les skills de stack du hub dont la dernière revue est ancienne, avec les nouveautés de leurs technologies, puis met à jour leur contenu, `versions:` et `reviewed:`. Se lance dans une copie de travail du hub (précondition : `skills/developer/stacks/` existe). Guide : [Fraîcheur des skills de stack](../guides/skill-refresh.fr.md).

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `skills` | `text` (2 000 caractères max) | non | Skills à revoir (noms ou chemins, par exemple la sortie d'`oh skill check --stale`) ; vide : les skills de stack dont la revue dépasse `max_age_days` (5 au plus, les plus anciennes d'abord) |
| `notes` | `text` (8 000 caractères max) | non | Notes de version ou changelogs fournis par l'utilisateur, pris comme source en plus de la recherche web |
| `max_age_days` | `int` (défaut 90) | non | Âge maximal d'une revue, en jours |

| Checkpoint | manuel | semi-auto | auto | distant |
|---|---|---|---|---|
| `cp-refresh` Mise à jour des skills (obligatoire, déverrouille `commit`) | pause | pause | pause | — (local seulement) |

Le `documentarian` lit les skills retenues, fait la veille de chaque technologie de `versions:` (notes de l'utilisateur, puis recherche web sur les sources officielles), et présente à `cp-refresh` sa proposition : versions avant → après, changements avec leurs sources, nouvelle date `reviewed`. **Rien n'est écrit avant la validation** ; un refus avec une consigne porte sur la proposition, jamais sur les fichiers. Après la validation, il applique la proposition et commite les skills seules. Sans skill à revoir, la session s'arrête sans checkpoint. Le contenu ajouté est en français, inspiré des sources et jamais recopié. Une pause en semi-auto (budget CP-R2 : une action irréversible). Commandes Beads autorisées (lecture seule) : `show`, `list`, `ready`, `search`, `query`.

## `release`

Prépare une release : version SemVer déduite des commits depuis le dernier tag et du CHANGELOG (ou imposée), section `[Unreleased]` datée du jour, notes de release, commit de release, tag annoté et, sur demande, description de la merge request de release. Risque `publish`, local seulement. Guide : [Préparer une release](../guides/release.fr.md).

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `version` | `string` (100 caractères max) | non | Version à publier ; vide : déduite des changements |
| `notes` | `text` (4 000 caractères max) | non | Points à mettre en avant, éléments à exclure |
| `mr` | `bool` (défaut : non) | non | Préparer la description d'une MR/PR de release, et l'ouvrir si la branche est déjà sur le dépôt distant |
| `target` | `string` (défaut : `main`) | non | Branche cible de la MR |

| Précondition | Test | Échec |
|---|---|---|
| `clean-tree` | `git_clean` | lancement refusé, avec les modifications trouvées |
| `release-branch` | `branch: [main, master, "release/*"]` | lancement refusé, avec la branche courante |
| `changelog` | `path_exists: [CHANGELOG.md]` | avertissement (le CHANGELOG sera créé) |

| Checkpoint | manuel | semi-auto | distant |
|---|---|---|---|
| `cp-release` Release (obligatoire, déverrouille `commit`, `tag`, `mr`) | pause | pause | interdit (`remote: forbid`) |

Le `release-manager` prépare tout **sans rien écrire** et le présente à `cp-release` : version et raison, section du CHANGELOG, message du tag, notes, description de la MR. Après la validation : CHANGELOG (et fichiers de version, chacun demandé), commit de release, puis tag annoté — oh refuse le tag tant que la release n'est pas commitée —, puis la MR si elle est demandée et que la branche existe sur le dépôt distant. **Aucun push** : le rapport donne les commandes (`git push origin <branche>`, puis `git push origin <tag>`) et la façon d'annuler avant le push. Une pause en semi-auto. Skills à la demande : `release-management`, `pr-description`.

## `review`

Review en lecture seule d'une branche ou des modifications récentes. Commande Beads autorisée : `show`. Pas de checkpoint.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `review_mode` | `enum` : `standard`, `adversarial`, `edge-case`, `standard+adversarial`, `all` | non | Vide : le reviewer propose le choix au démarrage |
| `branch` | `branch` | non | Branche à reviewer (vide : modifications récentes) |
| `base` | `branch` (défaut : `main`) | non | Branche de base |

Les modes combinés (`standard+adversarial`, `all`) lancent des sessions `reviewer` en parallèle : le workflow déclare l'auto-délégation `reviewer: { calls: [reviewer] }`. Une auto-délégation n'est retenue que si elle est écrite dans `calls` (jamais déduite des permissions) ; elle compte pour un niveau de profondeur.

Le reviewer lance une fois le build et les tests de la branche, avant les sessions parallèles : dans ce workflow `risk: read` (souvent la merge request d'un tiers, dont les tests exécutent le code), chaque commande est demandée à l'utilisateur ; refusée ou impossible à lancer, elle est signalée en finding 🟡 « non vérifiable ». Rien n'est installé.

La publication d'une MR (`oh review --publish`) n'est pas un workflow : elle reste une commande d'oh.

## `review-feedback`

Applique les commentaires non résolus d'une merge request. `oh run review-feedback -i mr=<url|!iid|branche>` suffit : oh lit la MR sur GitLab au lancement et calcule les entrées laissées vides (`branch`, `base`, `feedback` : entrées `from:`, voir le [schéma](workflow-schema.fr.md#entrées-inputs)). `oh review feedback <ticket|branche>` affiche d'abord un aperçu des discussions et demande confirmation. Sans discussion non résolue, le lancement est refusé.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `mr` | `string` (500 caractères max) | oui | URL ou référence de la MR |
| `branch` | `branch` | oui (calculée) | Branche de la MR ; vide : branche source de la MR |
| `base` | `branch` (défaut : `main`) | non | Branche cible ; vide : branche cible de la MR |
| `feedback` | `text` (70 000 caractères max) | oui (calculée) | Discussions non résolues ; vide : lues sur GitLab |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-fix` Corrections à appliquer | pause | pause | auto |
| `cp-2` Commit ou correction (obligatoire) | pause | pause | pause |

Déroulé : classement des retours → `cp-fix` → correction par le `developer`, **sans commit**, tests du projet compris → nouvelle review des corrections par le `reviewer` → `cp-2` (déverrouille `commit`) → commit `fix(review): address reviewer feedback` → récap. Le push de la branche et les réponses aux fils de la MR **reviennent à l'utilisateur** (aucun agent ne pousse ni ne répond sur GitLab). `developer` et `reviewer` ne démarrent qu'après `cp-fix`. Mode par défaut : `semi-auto`. Sortie : `branch`. Commandes Beads autorisées : `show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `status`, `graph`, `history` (lecture seule : pas de ticket dans ce workflow).

## `audit`

Audit en lecture seule : l'`auditor` coordonne des `auditor-subagent`, un par domaine (skill `auditor/audit-<type>`). Commandes Beads autorisées : `show`, `list`. Pas de checkpoint.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `type` | `enum` : `security`, `performance`, `architecture`, `accessibility`, `ecodesign`, `observability`, `privacy`, `infra`, `database` (défaut : `security`) | oui | Type d'audit |
| `focus` | `text` (4 000 caractères max) | non | Modules, fichiers ou questions à cibler |

- `infra` : revue IaC (Terraform/OpenTofu, Kubernetes, Helm, ArgoCD), sécurité IaC, coût cloud, drift. L'`auditor-subagent` peut lancer des outils d'analyse statique (`tflint`, `tfsec`, `checkov`, `trivy config`, `kube-score`, `kubesec`, `helm lint`, `helm template`, `infracost`, `terraform fmt -check`, `terraform validate`, `terraform plan`), chacun demandé ; toute autre commande, modifications comprises, est refusée. L'application d'un changement relève d'`infra-change`.
- `database` : rôles et privilèges, Row-Level Security, surfaces d'injection, chiffrement au repos et en transit, sauvegardes, sûreté des migrations, exposition des données sensibles. Lecture statique uniquement : les requêtes sur une base réelle sont fournies à l'utilisateur.

Sorties **relevées par oh** : `findings` (le dernier rapport déposé, `record: report`) et `type` (l'entrée du lancement, `record: input`). Suite proposée (`next:`) après un audit `security` qui a déposé son rapport : **`security-fix`**, qui lit les constats dans cette session (`oh run security-fix --parent <session>`), sans la lancer.

## `infra-change`

Applique un changement d'infrastructure déjà écrit dans le dépôt (Terraform/OpenTofu, Kubernetes, Helm, Pulumi) : l'`operator` vérifie le contexte réel par rapport à la cible, produit le plan (`terraform plan -out`, `kubectl diff`, `helm diff upgrade`, `pulumi preview`), le delta de coût (`infracost` s'il est installé), les risques et le retour arrière, puis applique **exactement** le plan validé (`terraform apply <fichier de plan>`) et vérifie le résultat. Il ne modifie aucun fichier (base `readonly-ops`) et ne délègue à aucun agent : l'écriture du code IaC revient au `developer` (`quick`, `ticket`), l'analyse en lecture seule à `audit` (`type=infra`). `isolation: strict`, exécution locale uniquement. Commandes Beads autorisées : lecture seule.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `request` | `text` (8 000 caractères max) | oui | Le changement à appliquer (commit, dossier, module, release) et son objectif |
| `target` | `string` (200 caractères max) | oui | Environnement visé tel que le projet le nomme (`staging`, `prod`, workspace Terraform, contexte kube, stack Pulumi) |

| Checkpoint | manuel | distant |
|---|---|---|
| `cp-plan` Plan | pause | — |
| `cp-apply` Application (obligatoire, déverrouille `apply`) | pause | interdit |

Mode `manuel` uniquement (`modes: { default: manuel, allowed: [manuel] }`) : `--mode semi-auto` et `--mode auto` sont refusés, chaque étape d'une modification d'infrastructure reste validée par l'utilisateur.

`target` est un texte libre plutôt qu'une énumération : chaque projet nomme ses environnements à sa façon (workspaces, contextes, stacks) ; l'`operator` la confronte au contexte réel avant tout plan et s'arrête en cas d'écart. Les lectures d'infrastructure sont autorisées, les lectures de secrets demandées, et toute modification (`terraform apply`, `kubectl apply`, `helm upgrade`, `pulumi up`, commandes create/delete/update des clouds…) est refusée jusqu'à `cp-apply`, puis demandée une à une ([garde-fou infra](../../SECURITY.fr.md)).

## `debug`

Diagnostic d'un bug ou d'un problème isolé par le `debugger` : rapport de diagnostic (actions d'urgence en premier) et ticket de correction. Le `developer` est disponible à la demande dans la session ; les enrichissements du wiki passent par `wiki_propose`.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `issue` | `text` (8 000 caractères max) | non | Le problème observé ; vide : la session le demande |

Le ticket de correction n'est créé qu'après `cp-ticket` (« Créer le ticket de correction », en pause dans tous les modes, déverrouille `create`) : le `debugger` y présente le diagnostic et le ticket, au lieu de demander la confirmation avec l'outil `question` (qu'il garde dans une session `libre`). Une correction faite dans la session se termine par `cp-commit` (obligatoire, en pause dans tous les modes, déverrouille `commit` et `close`). Sortie : `tickets`. Commandes Beads autorisées : `show`, `list`, `ready`, `search`, `query`, `lint`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`, `update`, `close`, `dep`, `label`.

**Brouillons de session** (`debugger: { scratch: { readers: [developer] } }`, voir [`scratch`](workflow-schema.fr.md#scratch-brouillons-de-session)) : en phase de reproduction, le `debugger` écrit son test ou script de reproduction dans `~/.oh/sessions/<session>/scratch/debugger/` (chemin donné dans l'entrée `oh.scratch` de l'état de session), hors dépôt et supprimé avec la session ; il décrit ce test en texte dans le ticket de correction. Le `developer`, lecteur déclaré, peut lire ce brouillon dans la même session, sans le modifier, et en fait le test définitif du projet. Aucun agent d'une autre session n'y a accès.

## `incident`

Gère un incident en cours : **atténuer d'abord**, puis écrire le postmortem sans blâme dans le wiki et créer les actions en tickets. Le `debugger` mène la session ; il reste en lecture seule, sauf son postmortem (`agents.debugger.writes: ["docs/wiki/postmortems/**"]`) ; le code d'une atténuation revient au `developer`, qu'il lance. Guide : [Incident, correction de sécurité et spike](../guides/incident-security-spike.fr.md).

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `incident` | `text` (16 000 caractères max) | oui | Ce qui se passe : symptômes, depuis quand, impact, alertes, logs, dernier déploiement |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-mitigated` Atténuation (obligatoire, vérifié par oh : tests de la stack, déverrouille `commit` : un commit) | pause | pause | pause |
| `cp-postmortem` Postmortem (obligatoire, déverrouille de nouveau `commit`, `unlock_again`) | pause | pause | pause |
| `cp-actions` Actions (obligatoire, déverrouille `create`) | pause | pause | pause |

Déroulé :

1. **Diagnostic rapide** : impact, heure de début, changement déclencheur (`git log` autour de l'heure de début, puis le diff du commit suspect).
2. **Atténuation** : la mesure la plus sûre et réversible (annuler le changement déclencheur, désactiver la fonction, configuration). Une mesure de code est appliquée par le `developer` **dans l'arbre de travail**, sans commit ; une mesure hors du dépôt (déploiement, variable d'environnement) est donnée telle quelle à l'utilisateur. À `cp-mitigated`, oh lance les tests de la stack, puis l'utilisateur choisit ou confirme la mesure ; validé, le correctif de code (s'il y en a un) est commité aussitôt par le `developer`, seul (`fix(incident): …`).
3. **Postmortem** : `docs/wiki/postmortems/<AAAA-MM-JJ>-<slug>.md` (frontmatter du wiki ; résumé, chronologie, cause racine, atténuation, actions), **sans blâme** : aucun nom de personne, pas même celui que cite le signalement. Skill à la demande : `postmortem` (chronologie sourcée, impact et délai de détection chiffrés, déclencheur et cause, actions vérifiables).
4. **`cp-postmortem`** : une fois validé, le postmortem est commité seul par le debugger (`git commit -m "docs(postmortem): <slug>" -- <fichier>`).
5. **`cp-actions`** : un ticket par action validée (`bd create`, description qui cite le postmortem) ; un refus : aucun ticket.

Sorties **relevées par oh** : `postmortem` (le fichier écrit sous `docs/wiki/postmortems/`, relevé quand un checkpoint passe, `record: written`) et `actions` (les tickets créés). Suites proposées (`next:`), jamais lancées : **`ticket`** sur les tickets des actions, s'il y en a ; **`spec`** avec le postmortem pour page d'origine (`-i source=…`), pour des actions qui demandent d'abord une spec.

Deux checkpoints de commit (`unlock_again`, voir le [schéma](workflow-schema.fr.md#checkpoints)) : la fenêtre de `cp-mitigated` ne tient qu'un commit, celui du correctif (oh ne l'ouvre pas s'il n'y a rien à commiter, la referme après ce commit et au plus tard quand `cp-postmortem` est demandé) ; le postmortem attend `cp-postmortem`. Le correctif peut ainsi être livré dès l'atténuation validée. Budget de pauses (CP-R2) en semi-auto : 3 pauses (1 décision, 2 actions irréversibles : le commit répété ne compte pas), pour un budget de 4. Commandes Beads autorisées : `show`, `list`, `ready`, `search`, `query`, `lint`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`, `update`, `dep`, `label`.

## `security-fix`

Corrige les constats d'un audit de sécurité : tri, correction avec test de non-régression, nouvelle analyse ciblée **avant** le commit. Le `conductor` enchaîne le `developer` et l'`auditor-subagent`.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `findings` | `text` (65 536 caractères max) | non | Les constats : rapport d'audit ou extrait, un constat par entrée avec sa gravité et son emplacement ; par défaut, le rapport de l'audit de sécurité dont le lancement est chaîné (`--parent`), sinon du dernier audit de sécurité du projet (`from: session.output(audit.findings)`, `from_equals: { type: security }`) ; aucun : la session les demande avant toute étape (outil `question`) |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-triage` Constats à corriger (déverrouille `create`) | pause | pause | auto |
| `cp-rescan` Nouvelle analyse (vérifié par oh : tests de la stack, au moins un fichier de test modifié) | pause | auto | auto |
| `cp-2` Commit ou correction (obligatoire, même vérification, déverrouille `commit`) | pause | pause | pause |

Déroulé :

1. **`cp-triage`** : les constats sont présentés tels que reçus, numérotés `F1`, `F2`… ; la réponse dit lesquels corriger, les autres sont écartés avec leur raison (plus tard, risque accepté, faux positif…).
2. **Tickets des constats écartés** (`developer`) : un ticket par constat écarté (`bd create … -t bug -l security`), avec le constat tel que reçu et la raison du tri ; avant `cp-triage`, oh refuse la création. oh relève ces tickets (sortie `deferred`, `record: created`).
3. **Corrections** (`developer`, domaine `security`) : seulement les constats retenus, à la cause et pour toutes les entrées concernées, chacun avec un test de non-régression ; aucun commit. Un faux positif est signalé, sans changement. Le developer rend un contrat précis, dans la section `### Critères d'acceptance couverts` de son rapport : une ligne par constat (`F<n>`, corrigé ou faux positif, test de non-régression, fichiers modifiés).
4. **Nouvelle analyse ciblée** (`auditor-subagent`) : il reçoit le rapport du developer (`rp-…`) et ce contrat ; pour chaque constat, `corrigé`, `encore présent` ou `contournable` (autre chemin, préfixe voisin, lien symbolique, autre encodage, autre appelant).
5. **`cp-rescan`** : sans pause en semi-auto ; oh lance les tests et refuse un changement sans test. Un constat encore présent repart au developer (au plus 2 tours, `limits.max_fix_rounds`).
6. **`cp-2`** : corrections, tests et verdict de la nouvelle analyse ; une fois validé, le developer commite (`fix(security): …`).

Pourquoi la nouvelle analyse précède le commit : une correction que l'auditeur juge contournable ne doit pas être commitée ; `cp-2` décide avec les tests (oh) et le verdict sous les yeux. Budget de pauses en semi-auto : 2 pauses (2 actions irréversibles : la création des tickets à `cp-triage`, le commit à `cp-2`), pour un budget de 4. Commandes Beads autorisées : `show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`.

**Depuis un audit** : un audit `security` propose `security-fix` (`next:`) ; `oh run security-fix --parent <session d'audit>` lit le rapport de cet audit (son contenu, pas son identifiant). Sans `--parent` ni `-i findings=…`, oh prend le rapport du dernier audit `security` du projet (jamais celui d'un audit d'un autre type) et l'annonce au lancement ; sans audit de sécurité, il le dit et la session demande les constats. Un extrait se donne toujours en entrée : `oh run security-fix -i findings="…"`.

## `spike`

Tranche une question technique dans un temps borné : recherche sourcée, prototype jetable au besoin, décision **go**, **clarify** ou **kill** écrite dans le wiki. Le `pathfinder` mène la session ; il reste en lecture seule, sauf sa décision (`agents.pathfinder.writes: ["docs/wiki/decisions/**"]`) ; le `developer` n'écrit que le prototype jetable.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `question` | `text` (8 000 caractères max) | oui | La question, les critères de décision (ce qui ferait dire go ou kill) et la durée accordée |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-question` Question et critères | pause | pause | auto |
| `cp-decision` Décision du spike (obligatoire, vérifié par oh : `.oh-spike/` supprimé, déverrouille `commit`) | pause | pause | pause |

Déroulé :

1. **`cp-question`** : question fermée, critères vérifiables, durée. Aucune recherche ni prototype avant (le `developer` est verrouillé jusqu'à `cp-question`, `after:`).
2. **Recherche sourcée**, critère par critère : le dépôt d'abord, puis la documentation officielle (skill `research`) ; chaque fait cite `fichier:ligne` ou son URL.
3. **Prototype jetable**, seulement si un critère ne se tranche qu'en exécutant du code : le `developer` écrit le plus petit programme qui répond, dans `.oh-spike/<slug>/` seulement, sans dépendance ajoutée, le lance, relève sa sortie, puis supprime `.oh-spike/`. Jamais de commit.
4. **Décision** : `docs/wiki/decisions/<slug>.md` (`**Décision : go | clarify | kill**`, constats par critère avec leurs sources, prototype, suites). `go` : tous les critères tiennent ; `kill` : l'un ne tient pas ; `clarify` : l'un ne se tranche pas sans une décision ou une information de l'utilisateur.
5. **`cp-decision`** : oh vérifie que `.oh-spike/` n'existe plus (`verify.cmd: ["test ! -e .oh-spike"]`) ; une fois validée, la décision est commitée seule.

Sorties **relevées par oh** quand un checkpoint passe (`record: written`) : `decision` (le fichier écrit sous `docs/wiki/decisions/`) et `verdict` (`go`, `clarify` ou `kill`, lu sur la ligne `**Décision : …**`, `match:`). Suite proposée (`next:`) après un `go` seulement : **`spec`**, avec la décision pour page d'origine (`-i source=…`), sans la lancer.

Budget de pauses en semi-auto : 2 pauses (1 décision, 1 action irréversible), pour un budget de 3. Commandes Beads autorisées (lecture seule) : `show`, `list`, `search`, `query`, `count`.

## `retro`

Rétrospective de sessions passées, à partir de leur trace : ce qui a ralenti, coûté ou bloqué le travail, et des actions vérifiables. Le `debugger` mène la session ; il reste en lecture seule, sauf la page de la rétrospective (`agents.debugger.writes: ["docs/wiki/retros/**"]`) ; il ne lit pas les traces lui-même, oh les lui donne. Guide : [Rétrospective et tri](../guides/retro-triage.fr.md).

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `sessions` | `string` (400 caractères max) | non | Les sessions à examiner : identifiants (`ses_…`), une période (`7d`, `24h`, `2w`, ou une date `AAAA-MM-JJ` : depuis ce jour) et des workflows (`ticket`), séparés par des espaces ou des virgules ; vide : la session dont le lancement est chaîné (`--parent`), sinon la dernière session terminée du projet |
| `trace` | `text` (20 000 caractères max) | non | Les résumés des traces, **calculés par oh** (`from: session.traces(sessions)`) ; donnés au lancement seulement pour rejouer une rétrospective |
| `focus` | `text` (2 000 caractères max) | non | Ce que l'utilisateur veut comprendre |

Sessions retenues avec une période ou des workflows : les plus récentes d'abord, terminées, 6 au plus, sans les sessions `retro` elles-mêmes (sauf si on les nomme). Chaque résumé (celui de [`oh session trace`](cli-sessions.fr.md), raccourci) donne le titre, l'état, la durée, les totaux et le coût par agent, les checkpoints et ceux demandés plusieurs fois (relances), les décisions refusées avec la consigne de l'utilisateur, les appels refusés, les skills, les délégations et les sorties ; 16 000 caractères au plus pour l'ensemble.

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-retro` Rétrospective (obligatoire, déverrouille `commit`) | pause | pause | pause |
| `cp-actions` Actions (obligatoire, après `cp-retro`, déverrouille `create`) | pause | pause | pause |

Déroulé :

1. **Relevé** : session par session, les faits du résumé, chacun avec sa source (`<session> · <section> · <ligne>`) ; rien d'autre : un fait absent du résumé n'existe pas.
2. **Constats** : les faits regroupés par cause (une commande refusée dans deux sessions, la même consigne à deux checkpoints), avec leur nombre, leur coût (relances, durée, dollars) et leur cause dans le système (prompt, permission, workflow, convention, vérification absente), **sans blâme** ; une cause non prouvée est une hypothèse. Le dépôt n'est lu que pour éclairer une cause.
3. **Actions** : au plus une par constat, chacune avec le signe qui dira, dans une prochaine trace, qu'elle est faite.
4. **Page** : `docs/wiki/retros/<AAAA-MM-JJ>-<slug>.md` (frontmatter du wiki ; sessions examinées, ce qui a bien marché, constats `C<n>`, actions `- [ ] A<n> … — vérification : …`).
5. **`cp-retro`** : une fois validée, la page est commitée seule (`git commit -m "docs(retro): <slug>" -- <fichier>`).
6. **`cp-actions`** : un ticket par action validée ; un refus : aucun ticket.

Sans session à examiner, le debugger le dit, donne la commande pour relancer (`oh run retro -i sessions="…"`) et s'arrête : ni checkpoint, ni écriture.

Pourquoi `writes:` et pas `wiki_propose` : une rétrospective est une page nouvelle et datée, écrite d'un seul tenant (comme le postmortem d'`incident` et la décision de `spike`) ; `wiki_propose` sert à ajouter ou remplacer des lignes dans une page existante, une proposition de 60 lignes au plus validée une à une à la carte du commit. Avec `writes:`, oh n'ouvre que `docs/wiki/retros/**` au debugger, ne laisse commiter que ce chemin après `cp-retro` et relève la page (`record: written`).

Sorties **relevées par oh** : `retro` (la page, `record: written`) et `actions` (les tickets créés). Suite proposée (`next:`), jamais lancée : **`triage`** sur les tickets des actions. Budget de pauses en semi-auto : 2 pauses (2 actions irréversibles), pour un budget de 4. Commandes Beads autorisées : `show`, `list`, `search`, `query`, `count`, `create`.

## `triage`

Tri d'un lot de tickets Beads ou de constats : priorités, doublons, étiquettes, ticket trop gros à découper. Le `planner` mène la session ; il ne modifie aucun fichier et ne lance aucun agent. **Rien n'est écrit dans Beads avant `cp-triage`**, qui montre tous les changements proposés. Guide : [Rétrospective et tri](../guides/retro-triage.fr.md).

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `tickets` | `beads-ids` | non | Les tickets à trier |
| `query` | `string` (400 caractères max) | non | Un filtre `bd query` des tickets à trier ; sans tickets ni filtre : les tickets ouverts du projet, 30 au plus |
| `findings` | `text` (16 000 caractères max) | non | Des constats (revue, audit, rétrospective…) : chacun est rattaché au ticket qui le couvre déjà (un commentaire) ou devient un ticket |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-triage` Changements du tri (obligatoire, déverrouille `create` et `update`) | pause | pause | pause |

Déroulé :

1. **Lecture** : `bd show`, `bd label list-all`, `bd find-duplicates --status open` (des candidats), `bd search` pour chaque constat.
2. **Tri** : priorité changée seulement si le ticket le justifie ; doublon seulement si les deux tickets décrivent le même problème au même endroit (on garde le plus complet), sinon un lien `related` ; étiquettes du projet seulement ; un ticket qui mêle des livrables indépendants reçoit un ticket enfant par livrable ; ni statut, ni assignation, ni description ne changent.
3. **`cp-triage`** : le tableau des changements (ticket, champ, avant → après, raison), les tickets à créer, les inchangés. Avant son passage, oh refuse **toute écriture dans Beads** : création (`create`), et modification (`update` : `bd update`, `label add|remove`, `dep add|remove`, `comments add`, `duplicate`, `supersede`, `reopen`…), dans le shell comme dans la passerelle Beads. Un refus : rien n'est écrit.
4. **Écritures** : exactement les lignes validées, une commande par ligne.

Sorties **relevées par oh** à partir de ce que la passerelle Beads a vu : `updated` (les tickets modifiés, `record: updated`), `created` (les tickets créés) et `duplicates` (les doublons fermés, `record: closed`). Suite proposée : **`ticket`** sur les tickets créés. Budget de pauses en semi-auto : 1 pause (1 action irréversible), pour un budget de 3. Commandes Beads autorisées : `show`, `list`, `ready`, `search`, `query`, `count`, `children`, `lint`, `find-duplicates`, `label`, `dep`, `comments`, `create`, `update`, `duplicate`, `supersede`.

Préalables de l'audit v5 (S-13, S-31, S-32, toujours ouverts) : ils touchent la passerelle MCP et les serveurs MCP Jira et GitHub, que `triage` n'utilise pas : il ne trie que des tickets Beads (passerelle Beads, verrous `create` et `update`) et des constats donnés en entrée.

## `benchmark`

Mesure des performances d'une cible par le `benchmarker` (modes `frontend` : Lighthouse, `api` : k6, `go` : go bench et pprof, `python` : py-spy), comparée à une référence. Le code n'est pas modifié : le `benchmarker` n'écrit que dans `reports/benchmarks/` (permission) ; les optimisations deviennent des recommandations pour le `developer`.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `target` | `string` (1 000 caractères max) | oui | Ce qu'il faut mesurer : URL, endpoint, package Go, script ou commande |
| `mode` | `enum` : `frontend`, `api`, `go`, `python` | non | Vide : déduit de la cible et de la stack |
| `baseline` | `string` (500 caractères max) | non | Ref git ou rapport précédent à comparer (mesure avant/après) ; vide : dernier rapport de `reports/benchmarks/` |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-target` Valider la cible et le profil de charge (obligatoire, avant toute mesure) | pause | pause | pause |
| `cp-recap` Récap du benchmark | pause | auto | auto |

À `cp-target`, le `benchmarker` présente la cible, l'environnement (la production demande une confirmation explicite), le profil de charge et la durée ; dans une session `libre`, il confirme avec l'outil `question`. Les outils de charge (`k6 run`, `wrk`, `ab`, `hey`) et `curl` demandent en plus une confirmation à chaque appel. Le `documentarian` n'est pas membre : les enrichissements du wiki sont listés dans le rapport. Sortie : `report` (chemin du rapport). Commandes Beads autorisées : `show`, `list`, `ready`, `search`, `query`.

## `sweep`

Atteint un objectif transverse en le découpant en sous-tâches indépendantes : le `pathfinder` (lecture seule) découpe, le `conductor` fait valider le découpage, **relevé et vérifié par oh**, puis les sous-tâches sont réalisées, vérifiées et relues. Aucun ticket Beads. Par défaut, version **séquentielle** dans la session ; avec `fanout=true` (expérimental), une **session enfant par sous-tâche** (voir plus bas).

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `goal` | `text` (4 000 caractères max) | oui | Objectif de haut niveau |
| `strategy` | `enum` : `llm` (défaut), `manual`, `by-file`, `by-package` | non | Découpage |
| `tasks` | `text` (8 000 caractères max) | non | Tâches, une par ligne (`manual`) |
| `include`, `exclude` | `string` (1 000 caractères max) | non | Motifs glob, séparés par des virgules |
| `verify` | `enum` : `none` (défaut), `tests`, `lint`, `build`, `all`, `custom` | non | Vérification demandée en plus de celle d'oh |
| `verify_cmd` | `string` (500 caractères max) | avec `custom` | Commande de vérification ; `verify: custom` sans elle est refusé au lancement (`verify_cmd_missing`). Lancée par un `developer` : elle peut être refusée par ses permissions |
| `dry_run` | `bool` (défaut : non) | non | Afficher le découpage sans rien exécuter : `cp-plan` n'est pas appelé, la session se termine par `cp-recap` (aucune session enfant en fan-out) |
| `fanout` | `bool` (défaut : non) | non | Expérimental (`[fanout] experimental = true`) : une session enfant `sweep-task` par sous-tâche validée, chacune dans son worktree, mergées par oh |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-plan` Valider le découpage (obligatoire, relevé et vérifié par oh, avant toute exécution) | pause | pause | pause |
| `cp-commit` Commit du sweep (obligatoire, vérifié par oh ; hors fan-out) | pause | pause | pause |
| `cp-merge` Valider les merges (obligatoire, vérifié par oh sur la branche d'intégration ; fan-out) | pause | pause | pause |
| `cp-recap` Récap du sweep | pause | pause | auto |

**Découpage relevé par oh** (`checkpoints.cp-plan.tasks`, [schéma](workflow-schema.fr.md#tasks--découpage-relevé-par-oh)). Le `conductor` passe le découpage dans l'argument `tasks` de `workflow_checkpoint` : une entrée `{ title, description, files, domain, parallel }` par sous-tâche. oh le vérifie **avant toute carte** : de 1 à 12 sous-tâches ; un titre sur une ligne ; un domaine parmi `frontend`, `backend`, `fullstack`, `api`, `data`, `mobile`, `devops`, `platform`, `security`, `refactor`, `migration`, `tests` ; un périmètre dont chaque entrée (chemin, dossier ou motif `*`, `**`) désigne des fichiers du dépôt, ou un fichier à créer dans un dossier existant (jamais absolu, hors du dépôt, une option ou `.git`) ; deux sous-tâches qui peuvent tourner en même temps (les parallélisables, ou toutes en fan-out) ne partagent aucun fichier. Un découpage refusé ne crée pas de carte : l'appel est refusé avec la liste des problèmes, et le `conductor` le corrige. Accepté, la carte montre le découpage **tel qu'oh l'a lu** (`t1`, `t2`… numérotés par oh) ; l'appel qui passe le checkpoint doit apporter le même, sinon il est refusé. oh l'enregistre une fois `cp-plan` passé : sortie `tasks` (`record: tasks`, une ligne par sous-tâche), et, en fan-out, les sessions enfants.

Version séquentielle : les agents développeurs et le `reviewer` ne démarrent qu'après `cp-plan` (en parallèle seulement pour des sous-tâches parallélisables aux périmètres disjoints) ; `cp-commit` déverrouille `commit` (avant lui, oh refuse tout `git commit`) et oh y lance la vérification de la stack. Mode par défaut : `semi-auto` ; coupe-circuit à 12 délégations consécutives. Commandes Beads autorisées : `show`, `list`, `search`, `query`. Sorties : `branch`, `tasks` (relevée par oh). Différence avec l'ancien `--sweep` : sans fan-out, les sous-tâches s'exécutent dans la même session et le même emplacement ; `--sweep-branch-prefix` et `--max-sessions` n'ont pas d'équivalent.

### Sweep en fan-out (`fanout=true`, expérimental)

`oh run sweep -i goal="…" -i fanout=true` (avec `[fanout] experimental = true`) : `sweep` utilise le mécanisme du fan-out d'[`implement-spec`](#fan-out-fanouttrue-expérimental), ses enfants étant les sous-tâches relevées à `cp-plan` (`fanout.source: tasks`) au lieu de tickets. La session parente garde `cp-plan`, `cp-merge` et `cp-recap` ; `cp-commit` passe dans les enfants et le `developer` n'est pas dans le paquet de la parente (`agents.developer.fanout: never`).

1. `cp-plan` validé, oh crée la branche d'intégration `oh/fanout/<session>/integration`, puis lance une session [`sweep-task`](#sweep-task) par sous-tâche **enregistrée par oh** (jamais une liste déclarée par un agent), sur `oh/fanout/<session>/t<n>` dans son worktree, 1 à 3 à la fois. oh lui transmet sa sous-tâche (entrée `task` : titre et description), son périmètre (`files`) et son domaine (`domain`), plus `goal`, `verify` et `verify_cmd` quand la session les a reçus (`fanout.inputs`), rien d'autre.
2. Chaque enfant fait sa sous-tâche, lance la vérification demandée, passe son `cp-commit` **sans pause** (délégué à `cp-merge`, CP-R3), vérifié par oh dans son worktree, puis commite. oh refuse ce commit si un changement sort du **périmètre de la sous-tâche** (ce qu'il a enregistré) : les périmètres étant disjoints, un enfant ne touche pas au travail d'un autre. Il ne merge pas et ne pousse pas (oh le refuse) ; aucun ticket à fermer.
3. Merge par oh dans la branche d'intégration, `merger` sur un conflit, tableau des sous-tâches à la parente, revue de l'ensemble par le `reviewer`, puis **`cp-merge`** (pause dans tous les modes) : oh y lance la vérification de la stack **sur la branche d'intégration**, jamais forcée ; validé, oh merge la branche d'intégration dans celle de la session. Rien n'est fermé : la sortie `tickets` n'existe pas.
4. `cp-merge` refusé avec une consigne : tour de correction (`fanout.fix`, session `quick` sur la branche d'intégration, son `cp-commit` validé par l'utilisateur), un tour (`limits.max_fix_rounds` absent), puis `cp-merge` revérifié.
5. `cp-recap` : sous-tâches mergées ou non, et pourquoi. oh nettoie ensuite les worktrees et branches `oh/fanout/…` comme pour `implement-spec`.

Budget de pauses en semi-auto : `cp-plan`, `cp-merge`, `cp-recap`, soit 3 pauses quel que soit le nombre de sous-tâches (comme la version séquentielle ; deux de plus par tour de correction). `dry_run` : `cp-plan` n'est pas appelé, aucune session enfant ne démarre.

**Fan-out désactivé** (`[fanout] experimental` à `false`, par défaut) : `sweep` garde sa version séquentielle, inchangée hormis le découpage relevé par oh ; `-i fanout=true` est refusé au lancement, comme pour `implement-spec`. Le fan-out n'est jamais choisi d'office.

**Couches d'équipe qui étendent `hub:sweep`** : rien à migrer, les identifiants (`cp-plan`, `cp-commit`, `cp-recap`, agents, entrées) ne changent pas. Une couche peut baisser `tasks.max`, restreindre `tasks.domains` ou `fanout.parallel`, jamais l'inverse ni retirer `tasks:`, changer `fanout.source`, ou garder le `developer` en fan-out (`loosening`). Une couche qui modifie `cp-commit` (vérification durcie, pause) le garde hors fan-out, mais une session en fan-out n'a pas ce checkpoint : avertissement `fanout_never_patched` à la validation, et `oh doctor` liste ces couches (« Couches et fan-out ») ; pour durcir aussi le fan-out, modifier `cp-merge`.

## `sweep-task`

Une sous-tâche d'un sweep, sans ticket (`risk: write`) : le `developer` (agent d'entrée) la fait dans son périmètre de fichiers, lance la vérification demandée, passe `cp-commit` (obligatoire, vérifié par oh, déverrouille `commit`) et commite. Lancé par oh pour chaque sous-tâche d'un `sweep` en fan-out, où son `cp-commit` est délégué au `cp-merge` de la parente et où oh refuse un changement hors du périmètre enregistré ; utilisable à la main (`oh run sweep-task -i task="…" -i files="…"`) avec le même contrôle : oh vérifie le périmètre au lancement (chaque entrée est un chemin, un dossier ou un motif du dépôt, ou un nouveau fichier d'un dossier existant ; sinon le lancement est refusé avec la liste des problèmes), le relève (sortie `files`) et refuse `cp-commit` sur un changement hors du périmètre (`inputs.files.scope`). Entrées : `task` et `files` (obligatoires, un chemin, dossier ou motif par ligne), `domain` (même liste que `sweep`), `goal`, `verify`, `verify_cmd`. Commandes Beads autorisées : `show`, `list`, `search`, `query`. Mode par défaut : `semi-auto` (dans un fan-out, celui de la parente).

## `brief-enrich`

Enrichit un brief de reprise de ticket, sans interaction (session sans interface : `oh run brief-enrich --headless`). L'entrée `brief` est lue dans l'espace d'équipe du projet (`from: ticket.brief(ticket)`) : `oh run brief-enrich --headless -i ticket=<id>` suffit. `oh takeover-brief enrich` enregistre en plus le résultat. Aucune commande Beads (`beads.allow: []`).

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `ticket` | `beads-id` | oui | Ticket du brief |
| `brief` | `text` (40 000 caractères max) | oui | Contenu du brief existant |

## `libre`

Session avec l'agent de ton choix, sans checkpoint ni verrou de commit (l'agent demande confirmation avant toute action irréversible) : `oh run libre --agent debugger -i request="…"` (sans `--agent` : `orchestrator`, l'agent d'entrée de ce seul workflow depuis que `feature` est un alias : il coordonne sans lire ni modifier de fichier, route vers le `pathfinder` ou le `planner` puis l'`orchestrator-dev`, et fait valider le plan avec `question`, faute de checkpoint). L'entrée du workflow est **au choix** (`entry.selectable: true`) : oh calcule les membres à partir de l'agent choisi et des agents qu'il peut appeler (permission `task` du catalogue, de proche en proche). Le monde reste fermé : rien d'autre n'est visible. Dans la TUI, la session libre (commande `coder`, ou « Démarrer » quand le catalogue est vide) ouvre la fiche avec l'agent par défaut. Commandes Beads autorisées : `show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `lint`, `find-duplicates`, `status`, `graph`, `history`, `create`, `update`, `close`, `reopen`, `edit`, `comment`, `dep`, `label`, `duplicate`, `supersede` (tout sauf `delete`).

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `request` | `text` (8 000 caractères max) | non | Ce qu'il faut faire ; sinon, la session attend la demande |

`entry.selectable` est un ajout additif au schéma `oh/v1` : un autre workflow peut le déclarer ; `--agent` est refusé sur un workflow dont l'entrée n'est pas au choix.

---

## Limites connues

Acceptées en l'état (p6-debts) ; elles ne changent ni les verrous ni les checkpoints :

- **Gardes sur le texte des commandes** : les règles shell (verrous `unlocks:`, `writes:`, fenêtre du premier commit d'un `unlock_again`, garde-fou global) comparent le texte brut de chaque commande, sans le normaliser : `git 'clean'` ou deux espaces ne correspondent pas au motif. Ce sont des garde-fous contre les erreurs d'un modèle, pas une barrière ([SECURITY](../../SECURITY.fr.md#ce-que-les-règles-shell-ne-protègent-pas)). Ce qui ne dépend pas du texte : la validation des checkpoints, la passerelle Beads, les vérifications d'oh, et les lignes `git add`/`git commit` de plusieurs chemins `writes:`, lues mot à mot par oh.
- **Pas de message de refus personnalisé** (opencode 2.0.20) : une commande refusée par une règle reçoit le message générique de l'outil. oh ne donne sa raison que pour ce que le plugin oh lui relaie (écriture tenue par `writes_after`, opération verrouillée tant qu'oh n'a pas confirmé les règles de la session, ligne `writes:` lue par oh) ; une `question` ne peut pas être refusée avec un message.
- **MR seulement si la branche est poussée** : `release` (`mr=true`) n'ouvre la merge request que si la branche est déjà sur le dépôt distant ; oh ne pousse jamais, le rapport donne les commandes.
- **2 pauses par tour de correction** du fan-out (`fanout.fix`) : le `cp-commit` de la session de correction, puis `cp-merge` repassé.
- **Éditeur de workflow de la TUI** : il propose `risk: publish` sans message propre à ce niveau (hub seulement, jamais en distant, `tag` et `mr`, préconditions durcies seulement) ; les refus apparaissent à la validation.
- **Sessions distantes** : les identifiants `wp-…` des propositions du wiki cités dans la transcription importée ne sont pas renumérotés (les rapports le sont).

## Restrictions

```yaml
limits:
  budget_usd: 5                          # budget de chaque session (USD)
  models: ["eu.anthropic.claude-*"]      # modèles autorisés (motifs)
```

- Les deux sont facultatifs et s'ajoutent aux restrictions du hub, de l'équipe et du projet (`oh budget show`, guide [Sessions v5](../guides/sessions-v5.fr.md#restrictions)). La valeur la plus précise l'emporte ; une valeur imposée par l'équipe est un plafond.
- `budget_usd` est un plafond souple sur le coût que l'outil indique pour la session et ses sous-agents : l'étape qui le dépasse se termine, puis une décision `$` demande de relever le budget ou d'arrêter.
- `models` porte sur l'identifiant envoyé au fournisseur (Bedrock : `eu.anthropic.…`) ; le proxy d'identifiants refuse les autres.
- Avec `extends`, un workflow peut seulement les durcir : budget plus bas, modèles couverts par les motifs du parent.

---

## Plugins et code mode

```yaml
code_mode: true                  # défaut : false
plugins:
  - context-mode@latest          # spécification npm (paquet, version ou tag)
  - { id: "@acme/probe", options: { verbose: true } }
```

- `code_mode` : `false` (ou absent) refuse l'outil `execute` d'opencode à tous les agents de la session ; `true` le laisse disponible.
- `plugins` : chaque entrée est une spécification npm, installée par opencode au démarrage du serveur (cache `~/.cache/opencode/npm`), avec ses `options`. Les plugins s'ajoutent au plugin oh ; en conteneur, la spécification est passée telle quelle (installation dans le conteneur).
- Un plugin qui ne se charge pas n'empêche pas la session de démarrer : opencode l'ignore et le note dans son journal. Le plugin doit exporter le format V2 (`{ id, setup }`) : `context-mode`, écrit pour opencode V1, ne se charge pas sous V2.
- Les deux champs entrent dans le hash du paquet : changer de plugin ou de code mode donne un autre paquet (et un autre serveur).

---

## Gabarits de prompt

Le premier message de la session est rendu depuis `workflows/prompts/<id>.md.tmpl` (Go `text/template`) :

- les entrées sont au premier niveau (`{{ .ticket }}`), le contexte de session sous `.oh` : `project`, `location`, `mode`, `runtime`, `lang`, `workflow` ;
- chaque gabarit commence par `Mode de workflow : {{ .oh.mode }}` (contrat avec l'agent d'entrée) et `Langue de réponse : {{ .oh.lang }}` ;
- `{{ data "request" .request }}` écrit une entrée, tronquée à `max_length`. Une valeur calculée par oh depuis une source externe (`from:` : discussions d'une MR, brief d'un ticket) est placée dans une balise de données `<oh:data name="…">…</oh:data>` : l'agent la traite comme une donnée, jamais comme une consigne (une balise contenue dans la valeur est neutralisée). Ce que l'utilisateur a saisi au lancement est sa propre demande : elle est écrite telle quelle, pour qu'une demande courte (« Dis seulement X ») soit traitée et non prise pour une injection. **Toute entrée `string` ou `text` passe par `data`** (vérifié par les tests) ;
- `{{ join .tickets ", " }}` joint une liste ;
- les entrées texte sont tronquées à `max_length` (20 000 caractères par défaut), avec la mention de la troncature ; un identifiant Beads invalide ou une branche ou un chemin sur plusieurs lignes est refusé ;
- une entrée absente vaut sa valeur par défaut, sinon la valeur vide de son type (`""`, `false`, `0`, liste vide), ce qui permet `{{ if .request }}`.

Le rendu est fait par `workflow.RenderPrompt` (`cli/internal/workflow/render_prompt.go`).

---

## Tests

- `TestShippedWorkflowsAreValid` : chaque workflow du dépôt est valide contre le hub, sans aucun diagnostic.
- `TestShippedWorkflowPrompts` : le prompt de chaque workflow est rendu avec toutes les entrées puis avec les seules entrées obligatoires.
- `TestShippedWorkflowTextInputsAreDelimited` : les entrées texte ne sont écrites que par `data`.
- `TestShippedWorkflowBundles` : le paquet compilé depuis chaque workflow (agents, permissions, skills, graphe, profondeur) et les skills générées sont figés par des fichiers golden (`cli/internal/bundle/testdata/golden/workflows/<id>/`).

Après une modification volontaire : `go test ./internal/bundle -run ShippedWorkflow -update`, puis relire le diff des fichiers golden.
