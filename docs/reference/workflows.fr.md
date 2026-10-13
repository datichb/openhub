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
| `cadrage` | `conductor` | `pathfinder`, `planner`, `designer` | plan | local | ancien mode A sans implémentation |
| `onboarding` | `onboarder` | — | write (`docs/wiki/`) | local, distant | `oh start --onboard` (ancien mode C) |
| `review` | `reviewer` | — | read | local, distant | `oh review` |
| `review-feedback` | `orchestrator-dev` | `developer`, `reviewer` | write | local | `oh review feedback` |
| `audit` | `auditor` | `auditor-subagent` | read | local, distant | `oh audit`, anciens agents `infra` et `database` (partie analyse) |
| `debug` | `debugger` | `developer`, `documentarian` (à la demande) | write | local | `oh debug` (ancien mode D) |
| `benchmark` | `benchmarker` | — | write (`reports/benchmarks/`) | local | `oh run libre --agent benchmarker` |
| `sweep` | `conductor` | `pathfinder`, `developer`, `reviewer` | write | local | `oh start --sweep` |
| `brief-enrich` | `brief-enricher` | — | read | local | `oh takeover-brief enrich` |
| `libre` | au choix (`orchestrator` par défaut) | ceux que l'agent choisi peut appeler | write | local, conteneur | `oh start --agent`, session libre de la TUI |

### Niveaux de risque

| `risk` | Fichiers | Beads |
|---|---|---|
| `read` | aucune modification, shell restreint | lecture seule ; `beads.allow` obligatoire, sans commande d'écriture |
| `plan` | aucune modification, shell restreint | `beads.allow` obligatoire ; écritures permises sauf `delete` |
| `write` | modifiés | `beads.allow` du workflow ; lecture seule si `beads` est absent |
| `publish` | modifiés, branches poussées, MR ouvertes | idem |

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
- Chemins relatifs, sans `..`. Un patch (`extends`) modifie une précondition par son identifiant ou la retire (`disabled: true`).

`feature` et `cadrage` proposent `onboarding` quand le projet n'a ni wiki, ni `ONBOARDING.md`, ni `CONVENTIONS.md`.

---

## `ticket`

Implémente un ou plusieurs tickets Beads déjà détaillés : routage vers le bon développeur, pre-review, review, commit.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `ticket` | `beads-id` (sélecteur : `ai-delegated`, multi-sélection) | oui | Ticket à implémenter ; plusieurs tickets sélectionnés = une session par ticket |
| `instructions` | `text` (4 000 caractères max) | non | Précisions pour la session |

| Checkpoint | manuel | semi-auto | auto | distant |
|---|---|---|---|---|
| `cp-1` Démarrer le ticket | pause | auto | auto | auto |
| `cp-2` Commit ou correction (obligatoire) | pause | pause | pause | attend l'utilisateur |
| `cp-3` Ticket suivant | pause | auto | auto | auto |

Mode par défaut : `semi-auto`. Commandes Beads autorisées : `show`, `list`, `ready`, `children`, `dep`, `update`, `close`, `comments`, `label`. Sorties : `branch`, `tickets`.

`cp-2` déverrouille `commit` et `close` : avant sa validation, oh refuse à tous les agents `git commit` (et `revert`, `cherry-pick`, `merge`, `pull`, qui commitent aussi) et la fermeture du ticket. Le push revient à l'utilisateur (aucun workflow livré ne le déverrouille). Une fois `cp-2` validé, l'`orchestrator-dev` relance le `developer`, qui commite puis ferme le ticket ; oh refuse la fermeture tant que le travail n'est pas commité. Le `reviewer` lance le build et les tests du projet (et le lint, le typecheck) avant sa review, avec les commandes de la stack détectée, autorisées ici : un échec est un finding 🔴 (verdict `corriger`) ; les fichiers produits par le build et non ignorés sont signalés, la fermeture exigeant un arbre propre. Il en va de même dans `feature`, `sweep`, `review-feedback` et `libre`. Une question qui imite un checkpoint (« CP-2 — commit ? » posée avec l'outil `question`) est fermée par oh, qui rappelle à l'agent d'appeler `workflow_checkpoint`. Quand le ticket de la session est fermé (ou le dernier checkpoint passé) et que l'étape se termine, oh déclare les sorties manquantes (`branch`, `tickets`) et la session passe à **terminée** : « Enchaîner avec… » est proposé.

## `feature`

Réalise une feature de bout en bout. L'orchestrator choisit l'agent de planning : `pathfinder` pour une feature simple ou exploratoire, `planner` pour une feature à découper (création des tickets) ou pour classer des tickets existants. Il passe ensuite la main à `orchestrator-dev` pour l'implémentation.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `request` | `text` (8 000 caractères max) | non | La feature en langage naturel, ou l'identifiant d'un ticket ou d'une MR GitLab |
| `tickets` | `beads-ids` | non | Tickets existants à prendre en charge (ancien mode B) |

Sans demande ni ticket, la session commence par demander quelle feature réaliser.

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-tickets` Valider les tickets à créer (obligatoire, déverrouille `create`) | pause | pause | pause |
| `cp-0` Valider le plan (obligatoire) | pause | pause | pause |
| `cp-spec` Valider la spec UX/UI | si une spec a été produite | idem | idem |
| `cp-1` Démarrer le ticket | pause | auto | auto |
| `cp-2` Commit ou correction (obligatoire) | pause | pause | pause |
| `cp-3` Ticket suivant | pause | auto | auto |
| `cp-feature` Récap de la feature | pause | pause | auto |

Aucun ticket n'est créé avant `cp-tickets` (avec des tickets existants seulement, `cp-tickets` confirme la liste). `orchestrator-dev` ne démarre qu'après `cp-0` ; il passe lui-même `cp-1`, `cp-2` et `cp-3`, avec **un `cp-2` par ticket** (chaque ticket est commité puis fermé avant le suivant ; pas de validation groupée). En fin de feature, l'`orchestrator` fait relire l'ensemble par le `reviewer` (review adversariale sur la branche de travail), avant `cp-feature`. Mode par défaut : `semi-auto`. Sorties : `tickets`, `branch`. Commandes Beads autorisées : `show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `lint`, `find-duplicates`, `status`, `graph`, `history`, `create`, `update`, `close`, `dep`, `label`, `duplicate`, `supersede`.

L'ancienne pré-phase d'onboarding (mode C) n'est plus dans `feature` : la précondition `project-context` propose le workflow `onboarding`, puis le retour à `feature`.

## `quick`

Session de développement directe avec l'agent `developer`, sans planification. Un seul checkpoint, `cp-commit` (obligatoire, en pause dans tous les modes) : il déverrouille `commit` et `close` ; avant lui, oh refuse tout `git commit` et la fermeture d'un ticket. Le push revient à l'utilisateur. Le développeur déduit le domaine de la demande et charge les standards correspondants. Il propose `feature` si la demande s'avère importante. Commandes Beads autorisées : `show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`, `update`, `close`, `dep`, `label`.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `request` | `text` (8 000 caractères max) | non | Ce qu'il faut faire ; sinon, la session attend la demande |

## `cadrage`

Explore, planifie et spécifie une feature sans l'implémenter : seuls des tickets Beads sont créés (`risk: plan`). Le `conductor` enchaîne `pathfinder` (exploration), `planner` (découpage et création des tickets) et `designer` (spécification UX/UI si besoin).

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `request` | `text` (8 000 caractères max) | oui | La feature à cadrer |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-scope` Valider le périmètre (avant la planification) | pause | pause | auto |
| `cp-tickets` Valider le découpage (obligatoire, déverrouille `create`) | pause | pause | pause |
| `cp-recap` Récap du cadrage | pause | pause | auto |

`planner` ne démarre qu'après `cp-scope`. Avant `cp-tickets`, oh refuse toute création de ticket (règles shell et passerelle Beads) : le planner propose le découpage, le `conductor` passe `cp-tickets`, puis relance le planner qui crée les tickets. Mode par défaut : `semi-auto`. Commandes Beads autorisées : `show`, `list`, `ready`, `children`, `search`, `query`, `count`, `lint`, `find-duplicates`, `label`, `dep`, `create`, `update`, `comments`, `duplicate`, `supersede`. Sortie : `tickets`. Pour implémenter ensuite : `ticket` ou `feature` sur les tickets créés.

## `onboarding`

Découvre le projet et crée ou enrichit le wiki `docs/wiki/` (protocole `doc-wiki-protocol`).

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `refresh` | `bool` (défaut : non) | non | Re-découvrir le projet et enrichir le wiki existant, sans rien supprimer |
| `focus` | `text` (4 000 caractères max) | non | Modules ou sujets à approfondir |

L'onboarder n'écrit que dans `docs/wiki/`, `ONBOARDING.md` et `CONVENTIONS.md` : c'est une permission (toute autre écriture est refusée, `.gitignore` et `.git/` compris). Il ne commite pas : le commit du wiki revient à l'utilisateur. Sortie : `wiki`. Commandes Beads autorisées (lecture seule) : `show`, `list`, `search`, `query`.

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

Diagnostic d'un bug ou d'un problème isolé par le `debugger` : rapport de diagnostic (actions d'urgence en premier) et ticket de correction. Le `developer` et le `documentarian` (enrichissement du wiki demandé par le `debugger`) sont disponibles à la demande dans la session.

| Entrée | Type | Obligatoire | Rôle |
|---|---|---|---|
| `issue` | `text` (8 000 caractères max) | non | Le problème observé ; vide : la session le demande |

Le ticket de correction n'est créé qu'après `cp-ticket` (« Créer le ticket de correction », en pause dans tous les modes, déverrouille `create`) : le `debugger` y présente le diagnostic et le ticket, au lieu de demander la confirmation avec l'outil `question` (qu'il garde dans une session `libre`). Une correction faite dans la session se termine par `cp-commit` (obligatoire, en pause dans tous les modes, déverrouille `commit` et `close`). Sortie : `tickets`. Commandes Beads autorisées : `show`, `list`, `ready`, `search`, `query`, `lint`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`, `update`, `close`, `dep`, `label`.

**Brouillons de session** (`debugger: { scratch: { readers: [developer] } }`, voir [`scratch`](workflow-schema.fr.md#scratch-brouillons-de-session)) : en phase de reproduction, le `debugger` écrit son test ou script de reproduction dans `~/.oh/sessions/<session>/scratch/debugger/` (chemin donné dans l'entrée `oh.scratch` de l'état de session), hors dépôt et supprimé avec la session ; il décrit ce test en texte dans le ticket de correction. Le `developer`, lecteur déclaré, peut lire ce brouillon dans la même session, sans le modifier, et en fait le test définitif du projet. Le `documentarian` n'y a pas accès, ni aucun agent d'une autre session.

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
