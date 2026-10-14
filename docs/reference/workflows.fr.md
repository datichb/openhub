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
| `feature` | `orchestrator` | `pathfinder`, `planner`, `designer`, `orchestrator-dev`, `developer`, `reviewer`, `documentarian` (à la demande) | write | local | `oh start` (anciens modes A, B et E) |
| `quick` | `developer` | — | write | local | session rapide |
| `infra-change` | `operator` | — | write (infrastructure, aucun fichier) | local | ancien agent `infra` (partie application) |
| `spec` (alias `cadrage`) | `conductor` | `pathfinder` (si la demande compare des conceptions), `planner` (écrit `docs/wiki/specs/`), `designer` | plan (+ `writes:`) | local | `cadrage` (ancien mode A sans implémentation), avec une spec écrite et vérifiée |
| `implement-spec` | `orchestrator-dev` | `developer`, `reviewer`, `documentarian` (à la demande) | write | local | partie implémentation de `feature` |
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
| `benchmark` | `benchmarker` | — | write (`reports/benchmarks/`) | local | `oh run libre --agent benchmarker` |
| `sweep` | `conductor` | `pathfinder`, `developer`, `reviewer` | write | local | `oh start --sweep` |
| `brief-enrich` | `brief-enricher` | — | read | local | `oh takeover-brief enrich` |
| `libre` | au choix (`orchestrator` par défaut) | ceux que l'agent choisi peut appeler | write | local, conteneur | `oh start --agent`, session libre de la TUI |

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

`release` refuse de se lancer sur un arbre sale ou hors de `main`, `master` et `release/*`. `feature`, `spec` (et son alias `cadrage`) et `implement-spec` proposent `onboarding` quand le projet n'a ni wiki, ni `ONBOARDING.md`, ni `CONVENTIONS.md`.

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

`cp-2` déverrouille `commit` et `close` : avant sa validation, oh refuse à tous les agents `git commit` (et `revert`, `cherry-pick`, `merge`, `pull`, qui commitent aussi) et la fermeture du ticket. Le push revient à l'utilisateur (aucun workflow livré ne le déverrouille). Avant que `cp-2` passe, oh lance lui-même le build, les tests et le typecheck de la stack détectée (`verify: { from: stack, expect: pass }`) : la carte de décision affiche le résultat, un échec n'ouvre rien et l'utilisateur peut passer outre (choix tracé). Même vérification sur `cp-2` de `feature` et `review-feedback`, et sur `cp-commit` de `quick`, `debug` et `sweep` ; voir [Checkpoints](checkpoints.fr.md). Une fois `cp-2` validé, l'`orchestrator-dev` relance le `developer`, qui commite puis ferme le ticket ; oh refuse la fermeture tant que le travail n'est pas commité. Le `reviewer` lance le build et les tests du projet (et le lint, le typecheck) avant sa review, avec les commandes de la stack détectée, autorisées ici : un échec est un finding 🔴 (verdict `corriger`) ; les fichiers produits par le build et non ignorés sont signalés, la fermeture exigeant un arbre propre. Il en va de même dans `feature`, `sweep`, `review-feedback` et `libre`. Une question qui imite un checkpoint (« CP-2 — commit ? » posée avec l'outil `question`) est fermée par oh, qui rappelle à l'agent d'appeler `workflow_checkpoint`. Quand le ticket de la session est fermé (ou le dernier checkpoint passé) et que l'étape se termine, oh déclare les sorties manquantes (`branch`, `tickets`) et la session passe à **terminée** : « Enchaîner avec… » est proposé.

## `feature`

Réalise une feature de bout en bout. L'orchestrator choisit l'agent de planning : `pathfinder` pour une feature simple ou exploratoire, `planner` pour une feature à découper (création des tickets) ou pour classer des tickets existants. Il passe ensuite la main à `orchestrator-dev` pour l'implémentation.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `request` | `text` (8 000 caractères max) | non | La feature en langage naturel, ou l'identifiant d'un ticket ou d'une MR GitLab |
| `tickets` | `beads-ids` | non | Tickets existants à prendre en charge (ancien mode B) |
| `method` | `enum` (`standard`, `tdd`) | non (`standard`) | Méthode : `tdd` ajoute `cp-seams` et `cp-red` à chaque ticket |

Sans demande ni ticket, la session commence par demander quelle feature réaliser.

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-tickets` Valider les tickets à créer (obligatoire, déverrouille `create`) | pause | pause | pause |
| `cp-0` Valider le plan (obligatoire) | pause | pause | pause |
| `cp-spec` Valider la spec UX/UI | si une spec a été produite | idem | idem |
| `cp-1` Démarrer le ticket | pause | auto | auto |
| `cp-seams` Points d'injection et tests prévus (méthode `tdd`) | pause | pause | auto |
| `cp-red` Tests en échec (méthode `tdd`, vérifié par oh) | auto | auto | auto |
| `cp-2` Commit ou correction (obligatoire) | pause | pause | pause |
| `cp-3` Ticket suivant | pause | auto | auto |
| `cp-feature` Récap de la feature | pause | auto | auto |

Aucun ticket n'est créé avant `cp-tickets` (avec des tickets existants seulement, `cp-tickets` confirme la liste). `orchestrator-dev` ne démarre qu'après `cp-0` ; il passe lui-même `cp-1`, `cp-2` et `cp-3` (en méthode `tdd`, le `developer` passe `cp-seams` et `cp-red`, une fois par ticket), avec **un `cp-2` par ticket** (chaque ticket est commité puis fermé avant le suivant ; pas de validation groupée). En fin de feature, l'`orchestrator` fait relire l'ensemble par le `reviewer` (review adversariale sur la branche de travail), avant `cp-feature`. Mode par défaut : `semi-auto`. Sorties : `tickets`, `branch`. Commandes Beads autorisées : `show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `lint`, `find-duplicates`, `status`, `graph`, `history`, `create`, `update`, `close`, `dep`, `label`, `duplicate`, `supersede`.

L'ancienne pré-phase d'onboarding (mode C) n'est plus dans `feature` : la précondition `project-context` propose le workflow `onboarding`, puis le retour à `feature`.

La même chaîne existe en deux workflows composés (R5) : [`spec`](#spec), qui propose à sa fin [`implement-spec`](#implement-spec) sur les tickets créés. `feature` reste inchangé dans cette version ; il deviendra l'enchaînement `spec` → `implement-spec` (un alias routé par les entrées : `aliases: [{ id: feature, with: tickets }]` dans `implement-spec`, `feature` dans `spec`) une fois les couches d'équipe qui l'étendent migrées.

## `quick`

Session de développement directe avec l'agent `developer`, sans planification. Un seul checkpoint, `cp-commit` (obligatoire, en pause dans tous les modes) : il déverrouille `commit` et `close` ; avant lui, oh refuse tout `git commit` et la fermeture d'un ticket. Le push revient à l'utilisateur. Le développeur déduit le domaine de la demande et charge les standards correspondants. Il propose `feature` si la demande s'avère importante. Commandes Beads autorisées : `show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`, `update`, `close`, `dep`, `label`.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `request` | `text` (8 000 caractères max) | non | Ce qu'il faut faire ; sinon, la session attend la demande |

## `spec`

Comprend une demande, écrit sa spec dans le wiki et la découpe en tickets Beads, sans l'implémenter (`risk: plan`). Le `conductor` relance le `planner` à chaque étape ; le planner reste en lecture seule, sauf sa spec : `agents.planner.writes: ["docs/wiki/specs/**"]` ([`writes`](workflow-schema.fr.md#writes-chemins-de-documentation)).

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `request` | `text` (8 000 caractères max) | oui | La feature à spécifier |

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
4. **`cp-spec`** : l'utilisateur valide la spec (même vérification) ; le planner la commite seule : `git add docs/wiki/specs/<slug>.md`, puis `git commit -m "docs(spec): <slug>" -- docs/wiki/specs/<slug>.md`.
5. **`cp-tickets`** : l'utilisateur valide le découpage ; le planner crée un ticket par tranche, dont la description cite la spec et ses critères. Une spec ou un découpage modifiés à un checkpoint repassent par `cp-converge`, `cp-spec` (nouveau commit) et `cp-tickets`.

Ordre imposé par oh : `cp-converge` est refusé avant `cp-understanding`, `cp-spec` avant `cp-converge`, `cp-tickets` avant `cp-spec` (`after:`) ; le refus nomme le checkpoint à passer d'abord.

Exploration : quand la demande compare des conceptions ou demande d'analyser le code avant de spécifier, le `conductor` lance d'abord le `pathfinder` (skill `codebase-design`) et passe son rapport au planner ; sinon l'entretien commence directement.

Budget de pauses (CP-R2) en semi-auto : 3 pauses (1 décision, 2 actions irréversibles), pour un budget de 4. Mode par défaut : `semi-auto`. Commandes Beads autorisées : `show`, `list`, `ready`, `children`, `search`, `query`, `count`, `lint`, `find-duplicates`, `label`, `dep`, `create`, `update`, `comments`, `duplicate`, `supersede`.

Sorties **relevées par oh** (aucun agent ne les déclare) : `spec` (la spec vérifiée à `cp-converge` et `cp-spec`) et `tickets` (les tickets créés après `cp-tickets`). Suite proposée (`next:`) quand des tickets ont été créés : **`implement-spec`** sur ces tickets et cette spec ; oh l'affiche à la fermeture du client (`oh run`), dans `oh session next <id>` et dans « Enchaîner avec… » de la TUI, sans la lancer.

**`cadrage` est un alias de `spec`** (R5) : `oh run cadrage` lance `spec` (`aliases: [cadrage]`). L'ancien `cadrage` (exploration systématique, `cp-scope`, `cp-recap`, aucun fichier écrit) n'existe plus ; une couche d'équipe ou de projet qui définit un workflow `cadrage` l'emporte sur l'alias.

## `implement-spec`

Implémente les tickets d'une spec l'un après l'autre, dans une seule session (`risk: write`), puis fait revoir l'ensemble. C'est la partie implémentation de `feature`, sans planification : l'`orchestrator-dev` lit lui-même les tickets (`bd show`) et fixe l'ordre (dépendances, puis priorité). **Version séquentielle** : les sessions enfants dans des worktrees, le parallélisme et le merge par oh viendront avec R6.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `tickets` | `beads-ids` | oui | Tickets à implémenter ; par défaut `session.output(spec.tickets)` : ceux de la session `spec` dont le lancement découle (`--parent`), sinon de la dernière session `spec` du projet |
| `spec` | `path` | non | Spec des tickets ; par défaut `session.output(spec.spec)` |
| `method` | `enum` : `standard`, `tdd` | non (`standard`) | Méthode TDD : `cp-seams` et `cp-red` pour chaque ticket |
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

Budget de pauses (CP-R2) en semi-auto : `cp-plan`, `cp-2`, `cp-final`, soit 3 pauses pour un budget de 3 (2 décisions, 1 action irréversible) ; la méthode `tdd` en ajoute une (`cp-seams`). Une feature de N tickets coûte donc N + 2 pauses (au lieu de N + 3 dans `feature`). Mode par défaut : `semi-auto`. Exécution : locale. Commandes Beads autorisées : `show`, `list`, `ready`, `children`, `dep`, `update`, `close`, `comments`, `label`. Sorties : `branch` (déclarée), `tickets` (tickets fermés, relevés par oh).

## `onboarding`

Découvre le projet et crée ou enrichit le wiki `docs/wiki/` (protocole `doc-wiki-protocol`).

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `refresh` | `bool` (défaut : non) | non | Re-découvrir le projet et proposer les mises à jour du wiki existant (validées une à une à la carte de `cp-wiki`), sans rien supprimer |
| `focus` | `text` (4 000 caractères max) | non | Modules ou sujets à approfondir |

L'onboarder n'écrit que dans `docs/wiki/`, `ONBOARDING.md` et `CONVENTIONS.md` : c'est une permission (toute autre écriture est refusée, `.gitignore` et `.git/` compris). Sans wiki, il crée les pages ; avec un wiki (`docs/wiki/index.md`), il n'en modifie aucune et propose chaque mise à jour avec `wiki_propose` (rien si le wiki est fidèle au code). Il passe ensuite `cp-wiki` (obligatoire, déverrouille le commit, `remote: defer`) : la carte montre ses propositions, l'utilisateur accepte, modifie ou refuse chacune, oh applique les acceptées ; puis l'onboarder commite `docs/wiki/` seul (`writes: ["docs/wiki/**"]`). `ONBOARDING.md`, `CONVENTIONS.md` et le push reviennent à l'utilisateur. Une pause en semi-auto (budget CP-R2 : une action irréversible). Sortie : `wiki`. Commandes Beads autorisées (lecture seule) : `show`, `list`, `search`, `query`.

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
| `cp-mitigated` Atténuation (obligatoire, vérifié par oh : tests de la stack) | pause | pause | pause |
| `cp-postmortem` Postmortem (obligatoire, déverrouille `commit`) | pause | pause | pause |
| `cp-actions` Actions (obligatoire, déverrouille `create`) | pause | pause | pause |

Déroulé :

1. **Diagnostic rapide** : impact, heure de début, changement déclencheur (`git log` autour de l'heure de début, puis le diff du commit suspect).
2. **Atténuation** : la mesure la plus sûre et réversible (annuler le changement déclencheur, désactiver la fonction, configuration). Une mesure de code est appliquée par le `developer` **dans l'arbre de travail**, sans commit ; une mesure hors du dépôt (déploiement, variable d'environnement) est donnée telle quelle à l'utilisateur. À `cp-mitigated`, oh lance les tests de la stack, puis l'utilisateur choisit ou confirme la mesure.
3. **Postmortem** : `docs/wiki/postmortems/<AAAA-MM-JJ>-<slug>.md` (frontmatter du wiki ; résumé, chronologie, cause racine, atténuation, actions), **sans blâme** : aucun nom de personne, pas même celui que cite le signalement. Skill à la demande : `postmortem` (chronologie sourcée, impact et délai de détection chiffrés, déclencheur et cause, actions vérifiables).
4. **`cp-postmortem`** : une fois validé, le correctif d'atténuation (s'il y en a un) est commité par le `developer`, puis le postmortem seul par le debugger (`git commit -m "docs(postmortem): <slug>" -- <fichier>`).
5. **`cp-actions`** : un ticket par action validée (`bd create`, description qui cite le postmortem) ; un refus : aucun ticket.

Pourquoi le correctif d'atténuation n'est commité qu'à `cp-postmortem` : une opération n'est déverrouillée que par un seul checkpoint (`unlock_duplicate`) ; `cp-mitigated` reste la décision sur l'atténuation, vérifiée par oh, et le correctif est en place dans l'arbre de travail dès ce moment (l'utilisateur peut le livrer lui-même s'il ne peut pas attendre). Budget de pauses (CP-R2) en semi-auto : 3 pauses (1 décision, 2 actions irréversibles), pour un budget de 4. Commandes Beads autorisées : `show`, `list`, `ready`, `search`, `query`, `lint`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`, `update`, `dep`, `label`.

## `security-fix`

Corrige les constats d'un audit de sécurité : tri, correction avec test de non-régression, nouvelle analyse ciblée **avant** le commit. Le `conductor` enchaîne le `developer` et l'`auditor-subagent`.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `findings` | `text` (30 000 caractères max) | oui | Les constats : rapport d'audit ou extrait, un constat par entrée avec sa gravité et son emplacement |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-triage` Constats à corriger | pause | pause | auto |
| `cp-rescan` Nouvelle analyse (vérifié par oh : tests de la stack, au moins un fichier de test modifié) | pause | auto | auto |
| `cp-2` Commit ou correction (obligatoire, même vérification, déverrouille `commit`) | pause | pause | pause |

Déroulé :

1. **`cp-triage`** : les constats sont présentés tels que reçus, numérotés `F1`, `F2`… ; la réponse dit lesquels corriger.
2. **Corrections** (`developer`, domaine `security`) : seulement les constats retenus, à la cause et pour toutes les entrées concernées, chacun avec un test de non-régression ; aucun commit. Un faux positif est signalé, sans changement.
3. **Nouvelle analyse ciblée** (`auditor-subagent`) : pour chaque constat, `corrigé`, `encore présent` ou `contournable` (autre chemin, préfixe voisin, lien symbolique, autre encodage, autre appelant).
4. **`cp-rescan`** : sans pause en semi-auto ; oh lance les tests et refuse un changement sans test. Un constat encore présent repart au developer (au plus 2 tours, `limits.max_fix_rounds`).
5. **`cp-2`** : corrections, tests et verdict de la nouvelle analyse ; une fois validé, le developer commite (`fix(security): …`).

Pourquoi la nouvelle analyse précède le commit : une correction que l'auditeur juge contournable ne doit pas être commitée ; `cp-2` décide avec les tests (oh) et le verdict sous les yeux. Budget de pauses en semi-auto : 2 pauses (1 décision, 1 action irréversible), pour un budget de 3. Commandes Beads autorisées (lecture seule) : `show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `status`, `graph`, `history`.

**Depuis un audit** : les constats sont donnés en entrée, par exemple le rapport de l'audit (`oh session report <session> <rp-…>`) : `oh run security-fix -i findings="$(oh session report <session> rp-2)"`. L'enchaînement direct `audit → security-fix` (sortie `findings` de l'audit, `next:`) viendra avec la composition des workflows (R5).

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

Budget de pauses en semi-auto : 2 pauses (1 décision, 1 action irréversible), pour un budget de 3. Commandes Beads autorisées (lecture seule) : `show`, `list`, `search`, `query`, `count`.

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

Atteint un objectif transverse en le découpant en sous-tâches indépendantes : le `pathfinder` (lecture seule) découpe, le `conductor` fait valider le plan, lance les agents développeurs (en parallèle seulement pour des périmètres de fichiers disjoints), fait vérifier et relire le résultat par le `reviewer`, puis fait commiter après validation. Aucun ticket Beads.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `goal` | `text` (4 000 caractères max) | oui | Objectif de haut niveau |
| `strategy` | `enum` : `llm` (défaut), `manual`, `by-file`, `by-package` | non | Découpage |
| `tasks` | `text` (8 000 caractères max) | non | Tâches, une par ligne (`manual`) |
| `include`, `exclude` | `string` (1 000 caractères max) | non | Motifs glob, séparés par des virgules |
| `verify` | `enum` : `none` (défaut), `tests`, `lint`, `build`, `all`, `custom` | non | Vérification finale |
| `verify_cmd` | `string` (500 caractères max) | avec `custom` | Commande de vérification ; `verify: custom` sans elle est refusé au lancement (`verify_cmd_missing`). Lancée par un `developer` : elle peut être refusée par ses permissions |
| `dry_run` | `bool` (défaut : non) | non | Afficher le découpage sans rien exécuter : `cp-plan` n'est pas appelé, la session se termine par `cp-recap` |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-plan` Valider le découpage (obligatoire, avant toute exécution) | pause | pause | pause |
| `cp-commit` Commit du sweep (obligatoire) | pause | pause | pause |
| `cp-recap` Récap du sweep | pause | pause | auto |

Les agents développeurs et le `reviewer` ne démarrent qu'après `cp-plan` ; `cp-commit` déverrouille `commit` (avant lui, oh refuse tout `git commit`). Mode par défaut : `semi-auto` ; coupe-circuit à 12 délégations consécutives. Commandes Beads autorisées : `show`, `list`, `search`, `query`. Sortie : `branch`. Différence avec l'ancien `--sweep` : les sous-tâches s'exécutent dans la même session et le même emplacement (pas un worktree par tâche) ; `--sweep-branch-prefix` et `--max-sessions` n'ont pas d'équivalent.

## `brief-enrich`

Enrichit un brief de reprise de ticket, sans interaction (session sans interface : `oh run brief-enrich --headless`). L'entrée `brief` est lue dans l'espace d'équipe du projet (`from: ticket.brief(ticket)`) : `oh run brief-enrich --headless -i ticket=<id>` suffit. `oh takeover-brief enrich` enregistre en plus le résultat. Aucune commande Beads (`beads.allow: []`).

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `ticket` | `beads-id` | oui | Ticket du brief |
| `brief` | `text` (40 000 caractères max) | oui | Contenu du brief existant |

## `libre`

Session avec l'agent de ton choix, sans checkpoint ni verrou de commit (l'agent demande confirmation avant toute action irréversible) : `oh run libre --agent debugger -i request="…"` (sans `--agent` : `orchestrator`). L'entrée du workflow est **au choix** (`entry.selectable: true`) : oh calcule les membres à partir de l'agent choisi et des agents qu'il peut appeler (permission `task` du catalogue, de proche en proche). Le monde reste fermé : rien d'autre n'est visible. Dans la TUI, la session libre (commande `coder`, ou « Démarrer » quand le catalogue est vide) ouvre la fiche avec l'agent par défaut. Commandes Beads autorisées : `show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `lint`, `find-duplicates`, `status`, `graph`, `history`, `create`, `update`, `close`, `reopen`, `edit`, `comment`, `dep`, `label`, `duplicate`, `supersede` (tout sauf `delete`).

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `request` | `text` (8 000 caractères max) | non | Ce qu'il faut faire ; sinon, la session attend la demande |

`entry.selectable` est un ajout additif au schéma `oh/v1` : un autre workflow peut le déclarer ; `--agent` est refusé sur un workflow dont l'entrée n'est pas au choix.

---

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
