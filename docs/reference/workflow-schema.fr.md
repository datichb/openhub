> [Read in English](workflow-schema.en.md)

# Référence — Schéma des workflows `oh/v1`

Un workflow est un fichier YAML `apiVersion: oh/v1`, `kind: Workflow`, nommé `<id>.yaml`. Cette page décrit chaque champ : type, valeurs, valeur effective quand le champ est absent, comportement dans un patch (`extends`) et verrouillage (`enforce`).

Le schéma est **figé** (`cli/internal/workflow/schema.go`, type `Spec`) : il n'évolue que par ajouts facultatifs. Ajouts faits après le gel : `risk: plan`, `preconditions`, `enforce`, `limits.models`, `entry.selectable`. Décision : [ADR-039](../architecture/adr/039-declarative-workflows-oh-v1.fr.md).

Voir aussi : [workflows livrés](workflows.fr.md) · [workflows d'équipe](../guides/team-workflows.fr.md) · [CLI workflows](cli-workflows.fr.md).

```bash
oh workflow validate ./mon-wf.yaml       # un fichier
oh workflow validate ticket --json       # un workflow du catalogue
oh workflow show ticket --origin         # valeurs résolues et document d'origine
```

---

## Règles générales

- **Lecture stricte** : un champ inconnu, une clé en double ou une valeur mal typée est une erreur. Toutes les erreurs du fichier sont rapportées, avec ligne et colonne.
- **Un seul document** par fichier (pas de `---` en plus). Le nom du fichier est l'identifiant (`ticket.yaml` ↔ `id: ticket`).
- **Textes traduisibles** (`label`, `description`, `help`) : un texte simple, ou une table par langue `{ fr: …, en: … }`. Sans la langue demandée, oh prend le texte simple, puis `en`, puis `fr`.
- **Cartes ordonnées** : `inputs`, `agents`, `checkpoints` et `preconditions` gardent l'ordre du fichier (ordre de la fiche de lancement, des checkpoints et des tests).
- **Patch** (`extends`) : c'est la **présence** d'un champ dans le fichier qui le remplace, pas sa valeur. Les cartes sont fusionnées par clé ; les listes et les textes sont remplacés en bloc.
- **Sécurité** : `risk`, `isolation`, `runtime.allowed`, `modes.allowed`, `code_mode`, `beads.allow`, checkpoints obligatoires, `remote` et `limits` ne peuvent que **se durcir**. Un assouplissement est une erreur (`loosening`) et la valeur du parent est gardée.
- **Verrou** : un champ de premier niveau cité dans `enforce` d'un document parent ne peut plus être écrit par un document qui l'étend (`enforced_field`).

Dans les tableaux ci-dessous, la colonne **Patch** vaut :

- **remplace** : la valeur écrite remplace celle du parent ;
- **fusion** : fusion par clé (les clés absentes sont héritées) ;
- **durcit** : remplace seulement si la valeur est au moins aussi stricte, sinon erreur `loosening`.

Tous les champs de premier niveau sont verrouillables par `enforce`, sauf `apiVersion`, `kind`, `id`, `version`, `extends` et `enforce`.

---

## En-tête

| Champ | Type | Valeurs | Absent | Patch |
|---|---|---|---|---|
| `apiVersion` | texte | `oh/v1` | obligatoire | — |
| `kind` | texte | `Workflow` | obligatoire | — |
| `id` | texte | kebab-case (`^[a-z0-9]+(-[a-z0-9]+)*$`), égal au nom du fichier | obligatoire | même id que le parent |
| `version` | entier | géré par la publication, jamais à la main | `0` | — |
| `category` | texte | `develop`, `frame`, `quality`, `knowledge`, `other` | rangé dans « autre » par le catalogue | remplace |
| `label` | texte traduisible | nom affiché | l'id est affiché | remplace |
| `description` | texte traduisible | une phrase | vide | remplace |
| `extends` | texte | `hub:<id>`, `team:<id>`, `project:<id>` | définition complète | — |
| `enforce` | liste | champs de premier niveau, ou `"*"` | aucun verrou | les verrous s'additionnent |

### `extends`

- Le parent doit être d'une couche **moins spécifique ou de la même couche** (`extends_more_specific`).
- Un workflow qui porte le même id qu'un workflow d'une couche inférieure **doit l'étendre** (`shadow_without_extends`), et étendre **le plus proche** (`must_extend_nearest`) : un `project:ticket` étend `team:ticket` s'il existe, sinon `hub:ticket`.
- Un autre id est permis : `team:ticket-hotfix` peut étendre `hub:ticket`.

### `enforce`

```yaml
enforce: [checkpoints, modes]   # ou ["*"] pour tout le document
```

- Valeurs : `category`, `label`, `description`, `risk`, `isolation`, `code_mode`, `entry`, `inputs`, `prompt`, `agents`, `checkpoints`, `modes`, `circuit_breaker`, `models`, `skills`, `plugins`, `mcp`, `beads`, `runtime`, `outputs`, `limits`, `preconditions`, ou `"*"`. Une autre valeur : `enforce_unknown_field`.
- Le verrou porte sur le champ de premier niveau entier (`checkpoints` verrouille tous les checkpoints).
- Les verrous s'additionnent le long de la chaîne `extends` ; une couche plus spécifique ne peut pas en retirer.
- Les options de lancement (mode, environnement, entrées) restent permises, dans les limites du workflow.

---

## Sécurité

| Champ | Type | Valeurs | Absent | Patch |
|---|---|---|---|---|
| `risk` | texte | `read` < `plan` < `write` < `publish` | **obligatoire** (`field_required`) | durcit (rang égal ou inférieur) |
| `isolation` | texte | `strict`, `standard` | `standard` | durcit (`strict` ne redevient pas `standard`) |
| `code_mode` | booléen | `true`, `false` | `false` | durcit (`false` ne repasse pas à `true`) |
| `beads.allow` | liste | sous-commandes `bd` | voir ci-dessous | durcit (sous-ensemble du parent) |
| `runtime.default` | texte | `local`, `container`, `remote` | `local` | remplace |
| `runtime.allowed` | liste | `local`, `container`, `remote` | `[runtime.default]` | durcit (sous-ensemble du parent) |
| `limits.budget_usd` | nombre ≥ 0 | USD par session | pas de budget au niveau workflow | durcit (valeur inférieure ou égale) |
| `limits.models` | liste | motifs (`*` joker) | pas de liste au niveau workflow | durcit (motifs couverts par le parent) |

### `risk`

| Valeur | Fichiers | Beads | Contrôles |
|---|---|---|---|
| `read` | aucune modification, pas de shell | lecture seule | aucun agent avec édition ou shell (`read_agent_writes`) ; `beads` obligatoire (`read_beads_unrestricted`), sans commande d'écriture (`read_beads_write`) |
| `plan` | aucune modification, pas de shell | écritures limitées à `beads.allow` | aucun agent avec édition ou shell (`plan_agent_writes`) ; `beads` obligatoire (`plan_beads_unrestricted`), sans `delete` (`plan_beads_delete`) ; avertissement s'il n'y a aucune écriture (`plan_without_beads_write` : `read` suffit) |
| `write` | modifiés | sans restriction de schéma | — |
| `publish` | modifiés, branches poussées, MR ouvertes | idem | — |

Commandes Beads considérées comme des écritures : `create`, `update`, `close`, `reopen`, `delete`, `edit`, `comment`, `comments`, `label`, `dep`, `duplicate`, `supersede`.

Une session qui écrit (`risk` autre que `read`) reçoit un worktree si une autre session qui écrit est active au même endroit (voir [CLI workflows](cli-workflows.fr.md#oh-run)).

### `isolation`

- `strict` : le monde fermé doit être complet. Aucune permission ne peut être accordée « toujours » depuis oh. Si l'adaptateur n'annonce pas une isolation complète, la validation avertit (`isolation_unsupported`) et le lancement est refusé.
- `standard` (ou absent) : une isolation partielle est acceptée.
- Indépendant de `[execution] strict_isolation` de `hub.toml` (qui masque la configuration personnelle de l'outil). Voir [ADR-041](../architecture/adr/041-closed-world-isolation.fr.md).

### `code_mode`

`false` ou absent : l'outil `execute` d'opencode est refusé à tous les agents. `true` : il reste disponible. Le champ entre dans le hash du paquet. C'est un champ de sécurité : une couche qui étend un workflow peut le désactiver, pas l'activer (`loosening`) ; le résumé d'impact de la publication signale son activation.

### `beads`

```yaml
beads: { allow: [show, list, update, close] }
```

- Absent : pas de restriction déclarée. La passerelle Beads applique alors une liste en lecture seule (`show`, `list`, `ready`, `search`, `children`, `comments`, `count`, `status`, `graph`, `history`). Les workflows livrés déclarent tous leur liste.
- `allow: []` : aucune commande permise.
- La liste est appliquée par la passerelle Beads du démon dans tous les environnements (en local, le faux `bd` d'oh passe en tête du `PATH` de la session ; un `bd` appelé par un chemin est refusé) et au rejeu du journal (distant). Voir [ADR-046](../architecture/adr/046-beads-gateways.fr.md).
- Patch : sur un parent sans `beads`, toute liste durcit. Sur un parent avec liste, la nouvelle liste doit en être un sous-ensemble ; `beads: null` est un assouplissement.

### `runtime`

```yaml
runtime: { default: local, allowed: [local, container, remote] }
```

- `runtime.default` doit figurer dans `runtime.allowed` (`runtime_default_not_allowed`).
- Choix au lancement, dans les limites de `allowed` : `--runtime` > runtime par défaut du projet > Réglages > `runtime.default`. Une préférence non autorisée est ignorée (`PickRuntime`) ; `--runtime` non autorisé est une erreur (`session_runtime_not_allowed`).
- Patch : si seul `default` est écrit, la liste effective du parent est gardée (un nouveau défaut n'élargit pas la liste).
- `remote` autorisé ⇒ aucun checkpoint `remote: forbid` qui attend l'utilisateur (`remote_forbidden_checkpoint`).
- Voir [ADR-045](../architecture/adr/045-execution-environments.fr.md).

### `limits`

```yaml
limits:
  budget_usd: 5                       # plafond souple par session
  models: ["eu.anthropic.claude-*"]   # modèles autorisés
```

- S'ajoute aux restrictions I6 du hub, de l'équipe et du projet (`oh budget show`) : la valeur la plus précise l'emporte (workflow > projet > hub > recommandation de l'équipe) ; une valeur imposée par l'équipe reste un plafond. Voir [ADR-044](../architecture/adr/044-credential-proxy-session-limits.fr.md).
- `budget_usd` : plafond souple, vérifié entre deux étapes ; au-delà, une décision `$` demande de relever ou d'arrêter. Valeur négative : `negative_value`.
- `models` : motifs sur l'identifiant envoyé au fournisseur ; le proxy d'identifiants refuse les autres. Un motif vide : `field_required`.

---

## Entrée et modes

| Champ | Type | Valeurs | Absent | Patch |
|---|---|---|---|---|
| `entry.agent` | texte | id d'agent du catalogue, primaire | `conductor` | remplace le bloc `entry` |
| `entry.selectable` | booléen | `true`, `false` | `false` | à écrire avec `entry.agent` |
| `modes.default` | texte | un mode de `modes.allowed` | premier mode autorisé | remplace |
| `modes.allowed` | liste | `manuel`, `semi-auto`, `auto`, ou modes propres | `[manuel, semi-auto, auto]` | durcit (sous-ensemble du parent ; une liste vide vaut tous les modes) |
| `circuit_breaker.max_consecutive_subagents` | entier ≥ 0 | N délégations de suite sans interaction | `0` (coupe-circuit désactivé) | remplace |

### `entry`

- L'agent d'entrée doit être **primaire** (`entry_not_primary`) et, s'il est listé dans `agents`, avoir le rôle `workflow` (`entry_role`). Il ne peut pas être retiré par patch (`entry_disabled`).
- `conductor` : agent générique sans écriture ni shell, qui suit la carte du workflow générée et délègue.
- `selectable: true` : l'agent d'entrée se choisit au lancement (`oh run libre --agent debugger`). Les membres sont alors calculés : l'agent choisi et ceux qu'il peut appeler (permission `task` du catalogue, de proche en proche) ; `agents:` est recalculé de la même façon pour l'agent par défaut. Sur un workflow sans `selectable`, `--agent` est refusé (`session_entry_not_selectable`).
- Patch : le bloc `entry` est remplacé quand `entry.agent` est écrit (ou `entry: null`) ; `entry.selectable` écrit seul change le choix de l'agent et garde l'agent d'entrée du parent.

### `modes`

- Le mode est fixé au lancement (`--mode`, fiche de lancement) et n'est plus demandé par l'agent. Le prompt contient toujours `Mode de workflow : <mode>`.
- Patch : `modes.allowed` ne peut que se restreindre (`loosening` pour un mode absent du parent) ; le résumé d'impact signale tout mode ajouté comme un assouplissement.
- Un mode en double : `duplicate_entry` ; un défaut absent de la liste : `mode_default_not_allowed` ; `--mode` non autorisé : `session_mode_not_allowed`.

### `circuit_breaker`

Après N appels de sous-agents consécutifs sans interaction de l'utilisateur, la session s'arrête sur une décision ✗ (coupe-circuit). Valeur négative : `negative_value`.

---

## Entrées (`inputs`)

```yaml
inputs:
  ticket:
    type: beads-id
    required: true
    label: { fr: Ticket, en: Ticket }
    help: Ticket Beads délégué à l'IA
    picker: { filter: ai-delegated, multi: true }
  branch: { type: string, default: "feat/{{ .ticket }}" }
```

L'id d'une entrée suit `^[a-z][a-z0-9_]*$` (`input_id_invalid`) ; `oh` est réservé au contexte de session (`input_id_reserved`).

| Champ | Type | Absent | Rôle |
|---|---|---|---|
| `type` | texte | obligatoire | voir le tableau des types |
| `required` | booléen | `false` | sans valeur ni défaut : `session_input_missing` |
| `default` | valeur du type | valeur vide du type | peut citer une autre entrée (`{{ .ticket }}`), pas elle-même |
| `label`, `help` | texte traduisible | l'id | fiche de lancement |
| `values` | liste | — | choix d'un `enum` (obligatoire pour `enum`, ignoré sinon) |
| `picker` | table | — | sélecteur de tickets (`beads-id`, `beads-ids` seulement) |
| `max_length` | entier ≥ 0 | `0` = 20 000 caractères | troncature de la valeur injectée |
| `from` | `source(entrée)` | — | valeur calculée par oh au lancement quand elle n'est pas donnée (voir ci-dessous) |

| Type | Valeur | Dans le prompt |
|---|---|---|
| `string` | une ligne de texte | texte, à passer par `data` |
| `text` | texte libre, plusieurs lignes | texte, à passer par `data` |
| `bool` | `true` / `false` (`-i x=true`) | booléen (`false` par défaut) |
| `int` | entier | entier (`0` par défaut) |
| `enum` | une des `values` | texte |
| `path` | chemin, une seule ligne | texte |
| `branch` | nom de branche, une seule ligne | texte |
| `beads-id` | un id Beads (plusieurs si `picker.multi`) | ids joints par `, ` |
| `beads-ids` | liste d'ids Beads (`a,b` ou liste YAML) | liste (`join`) |

- **`picker`** : `filter` (ex. `ai-delegated`), `epic` (restreindre à une epic), `multi` (plusieurs tickets). Une entrée `beads-id` avec `multi: true` donne **une session par ticket** (`--tickets a,b`) ; une entrée `beads-ids` reçoit toute la liste dans une seule session.
- **Contrôles** : un id Beads invalide, ou un `path` / `branch` sur plusieurs lignes, est refusé au rendu. Une valeur ne correspondant pas au type : `session_input_invalid` ; une entrée inconnue : `session_input_unknown` ; un défaut invalide : `input_default_invalid`.
- **Troncature** (O11) : les valeurs `string`, `text`, `path`, `branch` sont coupées à `max_length` caractères, avec la mention `[… tronqué : N caractères sur M]`.
- **Entrées calculées** (`from`) : voir [ci-dessous](#entrées-calculées-from).
- **Patch** : fusion par id, champ par champ (`picker` aussi). Une entrée ne peut pas être retirée.

### Entrées calculées (`from`)

`from: <source>(<entrée>)` demande à oh de calculer l'entrée au lancement, à partir de la valeur d'une autre entrée du même workflow :

```yaml
inputs:
  mr: { type: string, required: true }
  feedback: { type: text, required: true, from: mr.discussions(mr) }
```

| Source | Calcule | Argument |
|---|---|---|
| `mr.discussions(mr)` | les discussions non résolues de la merge request | une merge request : URL, `!iid`, numéro, branche ou ticket |
| `mr.source_branch(mr)` | sa branche | idem |
| `mr.target_branch(mr)` | sa branche cible | idem |
| `ticket.brief(ticket)` | le brief de reprise du ticket (la version enrichie d'abord) | un id de ticket |

- **Noms neutres** : oh résout une source avec la forge du projet (aujourd'hui GitLab : jeton `oh service setup`, projet `tracker_project`) ou son espace d'équipe ; un workflow n'a pas à changer selon la forge. La liste est fermée, et ouverte à tous les workflows (livrés, d'équipe, de projet).
- **Priorité** : une valeur donnée au lancement l'emporte, puis la valeur calculée, puis le défaut.
- **Échec** : si le calcul échoue, le lancement est refusé pour une entrée sans défaut ; sinon le défaut s'applique. La source est lue une seule fois par lancement.
- La fiche de lancement ne demande pas ces entrées (elles restent modifiables).
- Contrôles : `input_from_invalid` (syntaxe), `input_from_unknown_source`, `input_from_unknown_input` (argument absent, ou l'entrée elle-même).

---

## Prompt

```yaml
prompt:
  template: prompts/ticket.md.tmpl   # ou text: "…"
```

| Champ | Type | Rôle |
|---|---|---|
| `template` | chemin | gabarit Go `text/template`, relatif au dossier `workflows/` de la couche qui l'a écrit, sans `..` (`prompt_template_path`) |
| `text` | texte | gabarit en ligne |

- Exactement un des deux (`prompt_both`, `prompt_empty`). Absent : pas de premier message (la session attend l'utilisateur).
- **Variables** : les entrées au premier niveau (`{{ .ticket }}`) ; le contexte de session sous `.oh` : `.oh.project`, `.oh.location`, `.oh.mode`, `.oh.runtime`, `.oh.lang`, `.oh.workflow`. Toute autre variable : `prompt_unknown_variable`.
- **Fonctions** :
  - `{{ data "request" .request }}` écrit la valeur, tronquée. Une entrée calculée (`from:`) est placée entre `<oh:data name="…">` et `</oh:data>` : l'agent la traite comme une donnée, jamais comme une consigne (une balise contenue dans la valeur est neutralisée). Une entrée saisie par l'utilisateur est écrite telle quelle : c'est sa demande. Toute entrée `string` ou `text` doit passer par `data`.
  - `{{ join .tickets ", " }}` joint une liste.
- Une entrée absente vaut son défaut, sinon la valeur vide de son type : `{{ if .request }}…{{ end }}` fonctionne.
- oh termine le prompt par la liste des checkpoints à signaler avant chaque agent verrouillé.
- Patch : le bloc `prompt` est remplacé en bloc.

---

## Agents

```yaml
agents:
  orchestrator-dev: { role: workflow, calls: [developer, reviewer] }
  developer: { role: workflow, after: cp-1 }
  reviewer: { role: workflow, mode: subagent, after: developer }
  documentarian: { role: independent }
```

La clé est l'id d'un agent du catalogue de briques (hub + équipe), en kebab-case (`agent_id_invalid`, `agent_unknown`). Seuls l'agent d'entrée et ces agents sont dans le paquet (monde fermé).

| Champ | Type | Valeurs | Absent | Rôle |
|---|---|---|---|---|
| `role` | texte | `workflow`, `independent`, `disabled` | obligatoire | `workflow` : dans l'enchaînement, doit être atteignable depuis l'entrée (`agent_unreachable`) ; `independent` : disponible à la demande, hors enchaînement ; `disabled` : retiré (patch) |
| `mode` | texte | `primary`, `subagent` | mode de l'agent dans le catalogue | `primary` : l'utilisateur peut le choisir ; `subagent` : appelé seulement par délégation |
| `after` | texte | id de checkpoint ou d'agent | aucun verrou | l'agent ne peut être appelé qu'après ce checkpoint ou cet agent (`after_unknown`, `after_cycle`) |
| `calls` | liste | ids d'agents du workflow | permission `task` de l'agent, restreinte au workflow | délégations permises (`calls_unknown`, `graph_cycle`) |

- **Graphe** : `calls` quand il est écrit, sinon la permission `task` de l'agent, limitée aux membres. L'auto-délégation n'est retenue que si elle est écrite (`reviewer: { calls: [reviewer] }`). La profondeur maximale du graphe devient `experimental.subagent_depth` d'opencode.
- **Patch** : fusion par id, champ par champ ; `role: disabled` retire l'agent (sauf l'agent d'entrée) ; `calls` est remplacé en bloc.

---

## Checkpoints

```yaml
checkpoints:
  cp-2:
    label: { fr: Commit ou correction, en: Commit or fix }
    description: Après la review, demander s'il faut committer ou corriger.
    mandatory: true
    mode: { manuel: pause, semi-auto: pause, auto: pause }
    unlocks: [commit, push, close]
    remote: defer
```

L'id est en kebab-case (`checkpoint_id_invalid`) et ne doit pas être celui d'un agent (`checkpoint_agent_clash`). Les checkpoints sont passés dans l'ordre du fichier.

| Champ | Type | Valeurs | Absent | Patch |
|---|---|---|---|---|
| `label`, `description` | texte traduisible | — | l'id | remplace |
| `mode.<mode>` | texte | `pause`, `auto`, `skip`, `conditional` | `pause` (avertissement `checkpoint_mode_missing`) | fusion par mode ; durcit si `mandatory` |
| `condition` | texte | phrase lue par l'agent | — (obligatoire avec `conditional`) | remplace |
| `mandatory` | booléen | `true`, `false` | `false` | durcit (`true` ne redevient pas `false`) |
| `remote` | texte | `auto` < `defer` < `forbid` | `defer` | durcit |
| `unlocks` | liste | `commit`, `push`, `close` | — (rien de verrouillé) | s'ajoute à la liste héritée (jamais retirée) |
| `disabled` | booléen | `true` | — | retire le checkpoint hérité (patch seulement) |

- **Comportements** : `pause` demande toujours l'utilisateur ; `auto` continue seul ; `skip` saute le checkpoint ; `conditional` demande selon `condition`. oh n'évalue pas la condition : un checkpoint `conditional` demande toujours l'utilisateur quand il est signalé.
- **Ordre de rigueur** : `skip` < `auto` < `conditional` < `pause`. Sur un checkpoint `mandatory`, un patch ne peut pas choisir un comportement moins strict, ni le retirer (`mandatory_checkpoint_removed`) ; `skip` sur un checkpoint obligatoire compte comme une pause.
- **`remote`** (session distante) : `auto` validé automatiquement ; `defer` la session s'arrête proprement et attend l'utilisateur sur la machine ; `forbid` le workflow ne peut pas s'exécuter à distance.
- **`unlocks`** : tant que le checkpoint n'est pas passé, oh refuse ces opérations à **tous les agents** de la session, quel que soit le modèle : `commit` (`git commit`, y compris `env git commit`, `git -c … commit`), `push` (`git push`), `close` (fermeture d'un ticket Beads, refusée aussi par la passerelle Beads). Une fois le checkpoint passé, la fermeture d'un ticket n'est acceptée que si le travail est commité depuis (ou s'il n'y a rien à committer, hors `.beads/`). La fenêtre se referme quand un ticket est fermé ou que le checkpoint est demandé de nouveau (ticket suivant, nouvelle correction). Une opération n'est déverrouillée que par un seul checkpoint (`unlock_duplicate`) ; un checkpoint qui déverrouille ne peut pas être retiré (`unlocking_checkpoint_removed`). Un checkpoint sauté dans le mode ne verrouille rien.
- Un mode qui n'est ni autorisé ni un des trois modes livrés : `checkpoint_mode_unknown` ; `conditional` sans `condition` : `checkpoint_condition_missing`.
- Exécution des checkpoints (3 niveaux : prompt, outil MCP `workflow_checkpoint`, plugin oh) : [ADR-042](../architecture/adr/042-checkpoints-headless-decisions.fr.md).

---

## Préconditions

```yaml
preconditions:
  project-context:
    label: { fr: Aucun contexte projet trouvé, en: No project context found }
    check: { path_exists: [docs/wiki/index.md, ONBOARDING.md] }   # l'un suffit
    on_fail: suggest
    suggest: { workflow: onboarding, resume: true }
```

Testées avant le lancement, dans l'ordre du fichier, dans l'emplacement de la session.

| Champ | Type | Valeurs | Absent |
|---|---|---|---|
| `label` | texte traduisible | message affiché en cas d'échec | l'id |
| `check.path_exists` | liste | chemins relatifs, sans `..` ; l'un d'eux doit exister | obligatoire (`precondition_check_invalid`, `precondition_path_invalid`) |
| `on_fail` | texte | `suggest`, `block` | `suggest` |
| `suggest.workflow` | texte | id d'un autre workflow du catalogue | aucune proposition (`precondition_self`, `precondition_unknown_workflow`) |
| `suggest.resume` | booléen | `true`, `false` | `false` |
| `disabled` | booléen | `true` | retire une précondition héritée (patch seulement) |

- `suggest` : message + proposition de lancer `suggest.workflow` ; l'utilisateur peut continuer. Avec `resume: true`, oh propose de relancer le workflow d'origine (mêmes entrées) une fois l'autre terminé.
- `block` : le lancement est refusé.
- En distant ou sans interface : un échec `suggest` devient un avertissement, un échec `block` arrête la session.
- Patch : fusion par id, champ par champ.

---

## Ressources

| Champ | Type | Absent | Patch |
|---|---|---|---|
| `models.default` | `fournisseur/modèle[#variante]` | cascade sans niveau workflow | remplace |
| `models.agents.<agent>` | `fournisseur/modèle[#variante]` | — | fusion par agent |
| `skills.extra` | liste de références de skills | — | remplace |
| `skills.deny` | liste (référence ou nom seul) | — | remplace |
| `mcp` | liste d'ids de serveurs MCP oh | serveurs MCP du projet | remplace |
| `plugins` | liste | aucun plugin en plus du plugin oh | remplace |
| `beads.allow` | voir [Sécurité](#beads) | | |

- **`models`** : niveau workflow de la [cascade des modèles](model-resolution.fr.md) (workflow·agent > workflow > projet…). Format invalide : `model_invalid` ; agent hors du workflow : `model_agent_unknown`.
- **`skills`** : les skills viennent des agents membres et de leurs dépendances (`requires:`). `extra` en ajoute (`skill_unknown`), `deny` en retire (une référence `chemin/skill` ou un nom seul). Une skill refusée mais requise par une autre : `skill_denied_required` ; dépendance introuvable : `skill_closure` ; identifiant en double : `skill_duplicate`.
- **`mcp`** : ids des serveurs MCP d'oh (`gitlab`, `figma`, `jira`, `team`…). Absent : la session garde les serveurs MCP du projet ; présent (même vide) : seulement ceux de la liste. Le serveur `workflow` (`workflow_status`, `workflow_checkpoint`, `workflow_outputs`) est toujours ajouté.
- **`plugins`** : un id (spécification npm, ex. `context-mode@latest`) ou `{ id, options }`. Installés par opencode au démarrage du serveur ; un plugin qui ne se charge pas n'empêche pas la session. Le module doit exporter le format opencode V2 (`{ id, setup }`).
- Les entrées vides ou en double des listes : `field_required`, `duplicate_entry`.

---

## Sorties (`outputs`)

```yaml
outputs:
  - { id: branch, type: branch, label: { fr: Branche de travail, en: Working branch } }
  - { id: tickets, type: beads-ids }
```

| Champ | Type | Valeurs |
|---|---|---|
| `id` | texte | unique dans la liste |
| `type` | texte | `branch`, `merge_request`, `beads-ids`, `path` |
| `label` | texte traduisible | libellé affiché |

L'agent déclare les valeurs avec l'outil MCP `workflow_outputs`. En fin de session, elles proposent « Enchaîner avec… » (touche `e` de la vue Sessions) et `oh session results`. Patch : liste remplacée en bloc.

---

## Couches et résolution

| Couche | Emplacement |
|---|---|
| `hub` | `workflows/` du dépôt, embarqués, extraits dans `~/.oh/hub/workflows/` (lecture seule) |
| `team` | `team-state/workflows/published/<id>.yaml` (ou espace solo) |
| `project` | `team-state/projects/<projet>/workflows/published/<id>.yaml` |
| brouillon | `workflows/drafts/<membre>/<id>.yaml` de la portée, lancé avec `oh run <id> --draft` |
| session | options de lancement (mode, environnement, entrées, agent d'entrée), jamais enregistrées |

- **Ordre** : hub < team < project < brouillon < session. Un brouillon n'est pas une couche : il remplace, pour son auteur et en local seulement, le document publié de sa couche ; il est refusé s'il assouplit la version publiée.
- **Résolution** : oh suit `extends` jusqu'à la définition racine, applique chaque patch de la racine vers le document demandé, puis les options de session. Chaque valeur garde son origine (`oh workflow show <id> --origin`, éditeur de la TUI).
- **Intégrité** : seuls les fichiers publiés dont l'empreinte correspond à `workflows.lock` sont chargés ; les autres sont ignorés avec un avertissement. Voir [Workflows d'équipe](../guides/team-workflows.fr.md#workflowslock-et-intégrité).
- Gouvernance, brouillons et publication : [ADR-040](../architecture/adr/040-workflows-team-state-governance.fr.md).

---

## Diagnostics principaux

Un diagnostic a une gravité (erreur ou avertissement), un code, le chemin du champ, le document d'origine et sa position. Messages traduits : clés `workflow.diag.<code>`.

| Code | Signification |
|---|---|
| `syntax`, `unknown_field`, `duplicate_key`, `invalid_type` | YAML invalide, champ inconnu, clé en double, valeur mal typée |
| `api_version`, `kind`, `field_required` | en-tête ou champ obligatoire manquant ou faux |
| `multiple_documents`, `empty_document`, `id_filename_mismatch` | un seul workflow par fichier, nommé `<id>.yaml` |
| `id_invalid`, `agent_id_invalid`, `checkpoint_id_invalid`, `precondition_id_invalid`, `input_id_invalid`, `input_id_reserved` | identifiant mal formé ou réservé |
| `enum_invalid`, `enum_values_missing`, `negative_value`, `duplicate_entry`, `field_ignored` | valeur hors liste, `enum` sans `values`, nombre négatif, doublon, champ sans effet pour ce type (avertissement) |
| `unknown_workflow`, `extends_not_found`, `invalid_extends`, `extends_cycle`, `extends_more_specific`, `duplicate_workflow` | chaîne `extends` impossible |
| `shadow_without_extends`, `must_extend_nearest` | même id qu'une couche inférieure sans l'étendre, ou sans étendre la plus proche |
| `loosening` | un champ de sécurité est assoupli : la valeur du parent est gardée |
| `enforced_field`, `enforce_unknown_field` | champ verrouillé par un parent ; valeur d'`enforce` inconnue |
| `entry_not_primary`, `entry_role`, `entry_disabled`, `agent_unknown`, `agent_unreachable` | agent d'entrée ou membre invalide |
| `after_unknown`, `after_cycle`, `calls_unknown`, `graph_cycle` | verrous `after` et délégations |
| `checkpoint_agent_clash`, `checkpoint_mode_unknown`, `checkpoint_mode_missing`, `checkpoint_condition_missing`, `mandatory_checkpoint_removed`, `unlocking_checkpoint_removed`, `unlock_duplicate`, `patch_unknown_checkpoint` | checkpoints |
| `precondition_check_invalid`, `precondition_path_invalid`, `precondition_self`, `precondition_unknown_workflow`, `patch_unknown_precondition` | préconditions |
| `prompt_both`, `prompt_empty`, `prompt_template_path`, `prompt_template_missing`, `prompt_parse`, `prompt_unknown_variable` | prompt |
| `read_agent_writes`, `read_beads_unrestricted`, `read_beads_write` | règles de `risk: read` |
| `plan_agent_writes`, `plan_beads_unrestricted`, `plan_beads_delete`, `plan_without_beads_write` | règles de `risk: plan` |
| `remote_forbidden_checkpoint`, `runtime_default_not_allowed`, `mode_default_not_allowed`, `isolation_unsupported` | environnements, modes, isolation |
| `model_invalid`, `model_agent_unknown`, `skill_unknown`, `skill_unknown_deny`, `skill_closure`, `skill_duplicate`, `skill_denied_required` | ressources |
| `session_mode_not_allowed`, `session_runtime_not_allowed`, `session_input_unknown`, `session_input_invalid`, `session_input_missing`, `session_entry_not_selectable`, `session_entry_unknown` | options de lancement |

---

## Exemple complet

```yaml
apiVersion: oh/v1
kind: Workflow
id: ticket-hotfix                  # = nom du fichier ticket-hotfix.yaml
category: develop
label: { fr: Correctif urgent, en: Hotfix }
description:
  fr: Corriger un ticket urgent, review obligatoire
  en: Fix an urgent ticket, mandatory review
extends: hub:ticket                # hérite de tout le reste
enforce: [checkpoints]             # les projets ne touchent pas aux checkpoints

risk: write                        # ≤ write (parent) : permis
isolation: strict                  # durcit standard → strict
code_mode: false

inputs:                            # fusion par id
  ticket:
    picker: { filter: hotfix }     # seul le filtre change ; multi reste hérité
  severity: { type: enum, values: [p1, p2], default: p1, label: Gravité }

prompt:
  text: |                          # remplace le gabarit hérité
    Mode de workflow : {{ .oh.mode }}
    Langue de réponse : {{ .oh.lang }}
    Corrige le ticket {{ .ticket }} (gravité {{ .severity }}) dans {{ .oh.project }}.
    {{ if .instructions }}{{ data "instructions" .instructions }}{{ end }}

agents:
  documentarian: { role: disabled }       # retiré du paquet
  developer-migrator: { role: disabled }
  reviewer: { mode: subagent }            # seul `mode` est réécrit

checkpoints:
  cp-1:
    mode: { semi-auto: pause }       # plus strict : permis
  cp-3: { disabled: true }           # un ticket à la fois

modes: { default: manuel, allowed: [manuel, semi-auto] }
circuit_breaker: { max_consecutive_subagents: 8 }

models:
  agents: { reviewer: amazon-bedrock/eu.anthropic.claude-opus-4-6-v1 }
mcp: [gitlab]                      # seulement GitLab (+ le serveur workflow)

beads: { allow: [show, list, update, close] }        # sous-ensemble du parent
runtime: { default: local, allowed: [local, container] }  # sans remote
limits:
  budget_usd: 3
  models: ["eu.anthropic.claude-*"]

preconditions:
  changelog:
    label: Pas de CHANGELOG.md
    check: { path_exists: [CHANGELOG.md] }
    on_fail: block

outputs:
  - { id: branch, type: branch }
  - { id: mr, type: merge_request }
```
