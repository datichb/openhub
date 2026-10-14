> [Read in English](workflow-schema.en.md)

# Référence — Schéma des workflows `oh/v1`

Un workflow est un fichier YAML `apiVersion: oh/v1`, `kind: Workflow`, nommé `<id>.yaml`. Cette page décrit chaque champ : type, valeurs, valeur effective quand le champ est absent, comportement dans un patch (`extends`) et verrouillage (`enforce`).

Le schéma est **figé** (`cli/internal/workflow/schema.go`, type `Spec`) : il n'évolue que par ajouts facultatifs. Ajouts faits après le gel : `risk: plan`, `preconditions`, `enforce`, `limits.models`, `limits.max_fix_rounds`, `entry.selectable`, `checkpoints.<id>.verify` et `checkpoints.<id>.when` (R3), `checkpoints.<id>.by` (L3), `checkpoints.<id>.wiki` (LD-D), `checkpoints.<id>.method` et les champs `must_touch_tests`, `outside_tests`, `since` de `verify` (R7), `agents.<id>.writes` et `verify.spec` (R4), `agents.<id>.writes_after` et `checkpoints.<id>.after` (r5-fixes). Décision : [ADR-039](../architecture/adr/039-declarative-workflows-oh-v1.fr.md).

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
- **Sécurité** : `risk`, `isolation`, `runtime.allowed`, `modes.allowed`, `code_mode`, `beads.allow`, checkpoints obligatoires (et checkpoints qui portent `unlocks`), `remote`, `limits`, les verrous `after`, les listes `calls`, `circuit_breaker`, les préconditions d'un workflow `risk: publish`, et hors de la couche hub `mcp` et `plugins`, ne peuvent que **se durcir**. Un assouplissement est une erreur (`loosening`) et la valeur du parent est gardée.
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
- Un parent devenu un **alias** (`hub:feature`, `hub:cadrage`) n'est plus un workflow : `extends_alias`, avec les workflows qui déclarent l'alias ; `oh doctor` liste ces documents (voir [`feature`](workflows.fr.md#feature-alias-de-spec--implement-spec)).

### `enforce`

```yaml
enforce: [checkpoints, modes]   # ou ["*"] pour tout le document
```

- Valeurs : `category`, `label`, `description`, `risk`, `isolation`, `code_mode`, `entry`, `inputs`, `prompt`, `agents`, `checkpoints`, `modes`, `circuit_breaker`, `models`, `skills`, `plugins`, `mcp`, `beads`, `runtime`, `outputs`, `limits`, `preconditions`, `headless`, `next`, `aliases`, ou `"*"`. Une autre valeur : `enforce_unknown_field`.
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
| `limits.max_fix_rounds` | entier ≥ 0 | tours de correction par ticket | `0` (pas de plafond) | durcit (valeur inférieure ou égale, jamais `0` ni absente sous un parent plafonné) |
| `checkpoints.<id>.verify` | objet | voir [Checkpoints](#verify--vérification-lancée-par-oh) | pas de vérification | durcit (ne se retire ni ne s'affaiblit) |
| `checkpoints.<id>.when` | objet | voir [Checkpoints](#when--conditions-évaluées-par-oh) | pas de condition | durcit (conditions héritées gardées) |
| `checkpoints.<id>.wiki` | texte | `sync` | rôle ordinaire | remplace |
| `checkpoints.<id>.method` | texte | `tdd` | checkpoint de toutes les sessions | ne se pose pas sur un checkpoint hérité (`loosening`) |

### `risk`

| Valeur | Fichiers | Beads | Contrôles |
|---|---|---|---|
| `read` | aucune modification, pas de shell | lecture seule | aucun agent avec édition ou shell (`read_agent_writes`) ; `beads` obligatoire (`read_beads_unrestricted`), sans commande d'écriture (`read_beads_write`) |
| `plan` | aucune modification, pas de shell | écritures limitées à `beads.allow` | aucun agent avec édition ou shell (`plan_agent_writes`) ; `beads` obligatoire (`plan_beads_unrestricted`), sans `delete` (`plan_beads_delete`) ; avertissement s'il n'y a aucune écriture (`plan_without_beads_write` : `read` suffit) |
| `write` | modifiés | sans restriction de schéma | — |
| `publish` | modifiés, tags créés, MR ouvertes | idem | jamais en distant : `remote` absent de `runtime.allowed` (`publish_remote_runtime`) ; chaque checkpoint qui déverrouille `push`, `tag` ou `mr` porte `remote: forbid` (`publish_checkpoint_remote`) et fait une pause dans tous les modes autorisés (`publish_checkpoint_pause`) |

Commandes Beads considérées comme des écritures : `create`, `update`, `close`, `reopen`, `delete`, `edit`, `comment`, `comments`, `label`, `dep`, `duplicate`, `supersede`.

Seul un workflow `publish` déverrouille `tag` et `mr` (`unlock_requires_publish`). oh refuse aussi de l'envoyer en distant (`oh run --runtime remote`), et le runner du job le refuse avant le clone : la décision de publier se prend toujours sur la machine de l'utilisateur.

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

- Absent : pas de restriction déclarée. La passerelle Beads applique alors une liste en lecture seule (`show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `status`, `graph`, `history`). Les workflows livrés déclarent tous leur liste.
- `allow: []` : aucune commande permise.
- Une liste qui ne nomme que des commandes de lecture (`show`, `list`, `query`, `comments`, `dep tree`…) est en lecture seule : les formes d'écriture de ces commandes sont refusées (`comments add`, `ready --claim`). Avec `risk: read`, chaque entrée doit nommer une commande de lecture (`read_beads_write`). Voir l'[ADR-052](../architecture/adr/052-beads-identity-provenance.fr.md).
- La liste est appliquée par la passerelle Beads du démon dans tous les environnements (en local, le faux `bd` d'oh passe en tête du `PATH` de la session ; un `bd` appelé par un chemin est refusé) et au rejeu du journal (distant). Voir [ADR-046](../architecture/adr/046-beads-gateways.fr.md).
- Patch : un parent sans `beads` vaut la liste en lecture seule par défaut (celle qu'applique la passerelle : `show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `status`, `graph`, `history`) : la liste de l'enfant doit en être un sous-ensemble (`create`, `close`… sont un assouplissement). Sur un parent avec liste, la nouvelle liste doit en être un sous-ensemble ; `beads: null` est un assouplissement.

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
  max_fix_rounds: 3                   # tours de correction developer → reviewer par ticket
```

- S'ajoute aux restrictions I6 du hub, de l'équipe et du projet (`oh budget show`) : la valeur la plus précise l'emporte (workflow > projet > hub > recommandation de l'équipe) ; une valeur imposée par l'équipe reste un plafond. Voir [ADR-044](../architecture/adr/044-credential-proxy-session-limits.fr.md).
- `budget_usd` : plafond souple, vérifié entre deux étapes ; au-delà, une décision `$` demande de relever ou d'arrêter. Valeur négative : `negative_value`.
- `models` : motifs sur l'identifiant envoyé au fournisseur ; le proxy d'identifiants refuse les autres. Un motif vide : `field_required`.
- `max_fix_rounds` : plafond de tours de correction (developer → reviewer) par ticket. C'est une consigne, écrite dans la section « Mode de workflow » des skills générées (« Au plus N tours de correction… ; au-delà, passe le checkpoint de commit en signalant les points non résolus. »), **et un verrou** (R3) : oh compte les paires developer → reviewer du ticket en cours ; au-delà du plafond, il refuse les délégations aux agents `developer*` et marque le checkpoint de commit « non résolu », jusqu'à une consigne de l'utilisateur à ce checkpoint ou la fermeture du ticket ([Checkpoints](checkpoints.fr.md#limite-des-tours-de-correction-limitsmax_fix_rounds)). `0` ou absent : pas de plafond. Une couche ne peut que l'abaisser (`loosening` sinon). Valeur négative : `negative_value`. Les workflows `ticket`, `implement-spec` et `review-feedback` du hub fixent `3`.

---

## Entrée et modes

| Champ | Type | Valeurs | Absent | Patch |
|---|---|---|---|---|
| `entry.agent` | texte | id d'agent du catalogue, primaire | `conductor` | remplace le bloc `entry` |
| `entry.selectable` | booléen | `true`, `false` | `false` | à écrire avec `entry.agent` |
| `modes.default` | texte | un mode de `modes.allowed` | premier mode autorisé | remplace |
| `modes.allowed` | liste | `manuel`, `semi-auto`, `auto`, ou modes propres | `[manuel, semi-auto, auto]` | durcit (sous-ensemble du parent ; une liste vide vaut tous les modes) |
| `circuit_breaker.max_consecutive_subagents` | entier ≥ 0 | N délégations de suite sans interaction | `0` (coupe-circuit désactivé) | durcit (valeur plus basse ; `0` ne remplace pas un coupe-circuit actif) |

### `entry`

- L'agent d'entrée doit être **primaire** (`entry_not_primary`) et, s'il est listé dans `agents`, avoir le rôle `workflow` (`entry_role`). Il ne peut pas être retiré par patch (`entry_disabled`).
- `conductor` : agent générique sans écriture ni shell, qui suit la carte du workflow générée et délègue.
- `selectable: true` : l'agent d'entrée se choisit au lancement (`oh run libre --agent debugger`). Les membres sont alors calculés : l'agent choisi et ceux qu'il peut appeler (permission `task` du catalogue, de proche en proche) ; `agents:` est recalculé de la même façon pour l'agent par défaut. Sur un workflow sans `selectable`, `--agent` est refusé (`session_entry_not_selectable`).
- Patch : le bloc `entry` est remplacé quand `entry.agent` est écrit (ou `entry: null`) ; `entry.selectable` écrit seul change le choix de l'agent et garde l'agent d'entrée du parent.

### `modes`

- Le mode est fixé au lancement (`--mode`, fiche de lancement) et n'est plus demandé par l'agent. Le prompt contient toujours `Mode de workflow : <mode>`.
- Sens des trois modes des workflows du hub : `manuel` s'arrête à chaque checkpoint ; `semi-auto` passe seul les checkpoints courants et s'arrête sur les décisions clés ; **`auto` = tout sauf les décisions obligatoires** : seuls s'arrêtent les checkpoints `mandatory` et ceux qui déverrouillent une action (`unlocks` : `create`, `commit`, `close`, `apply`, `tag`, `mr`), un `conditional` s'arrête quand il est signalé. Un workflow dont chaque étape doit être validée se limite à `manuel` (`infra-change`).
- Patch : `modes.allowed` ne peut que se restreindre (`loosening` pour un mode absent du parent) ; le résumé d'impact signale tout mode ajouté comme un assouplissement.
- Un mode en double : `duplicate_entry` ; un défaut absent de la liste : `mode_default_not_allowed` ; `--mode` non autorisé : `session_mode_not_allowed`.

### `circuit_breaker`

Après N appels de sous-agents consécutifs sans interaction de l'utilisateur, la session s'arrête sur une décision ✗ (coupe-circuit). Valeur négative : `negative_value`.

### `headless`

`headless: true` : personne ne suit la session (`brief-enrich`, lancé sans interface). Le contrat de session le dit à chaque agent : aucune question, une information manquante va dans la sortie ; la condition `when: headless` des skills est vraie. Un checkpoint qui peut attendre l'utilisateur donne l'avertissement `headless_checkpoint_pause`. Patch : remplace.

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
- **`method`** (R7) : l'entrée `method`, de type `enum` (`method_input_type`) aux valeurs `standard` et `tdd`, choisit la **méthode de la session** ; voir [`method`](#method--méthode-de-la-session).

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

#### Sources lues dans une session précédente (R5)

```yaml
inputs:
  tickets: { type: beads-ids, required: true, from: session.output(spec.tickets) }
  digest:  { type: text, from: session.trace(ticket) }
```

| Source | Calcule | Session lue |
|---|---|---|
| `session.output(<workflow>.<sortie>)` | la valeur d'une [sortie](#sorties-outputs) (liste de tickets jointe par des virgules) | la session dont le lancement découle (`oh run … --parent <id>`) si c'est une session de ce workflow qui a cette sortie ; sinon la **dernière** session du projet de ce workflow qui l'a |
| `session.trace()`, `session.trace(<workflow>)` | un résumé de la trace de la session (checkpoints, décisions refusées, appels refusés, skills, délégations, sorties, totaux ; 8 000 caractères au plus), pour une rétrospective | la session parente, sinon la dernière session terminée du projet (de ce workflow) |

- Pas d'argument d'entrée : ces sources se calculent sans autre entrée. Le lancement affiche la session lue (« session.output(spec.tickets) : session ses_… ev-1,ev-2 »).
- Rien trouvé : l'entrée reste vide ; une entrée obligatoire est alors refusée au lancement (`session_input_missing`).
- Contrôles : le workflow nommé existe (`input_from_unknown_workflow`), déclare la sortie (`input_from_unknown_output`), le type de la sortie convient à l'entrée (`input_from_type` : `beads-ids` → `beads-ids`/`beads-id`, `path` → `path`/`string`/`text`, `branch` → `branch`/`string`, `report` → `string`/`text` ; `session.trace` → `text`) ; `session.output(<sortie>)` sans workflow est refusé dans `from:` (`input_from_session_workflow`).

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
- **Variables** : les entrées au premier niveau (`{{ .ticket }}`) ; le contexte de session sous `.oh` : `.oh.project`, `.oh.location`, `.oh.mode`, `.oh.runtime`, `.oh.lang`, `.oh.workflow`, `.oh.branch`, `.oh.memories` (mémoires persistantes du Beads du projet, `bd remember`, déjà entre balises de données ; vide sans mémoire). Toute autre variable : `prompt_unknown_variable`.
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
| `scratch` | table | `{ readers: [<agent>…] }` | pas de brouillons | dossier de brouillons de session de l'agent (voir [ci-dessous](#scratch-brouillons-de-session)) |
| `writes` | liste | chemins sous `docs/` : un fichier, ou un dossier terminé par `/**` | aucune écriture ouverte | chemins de documentation que l'agent peut écrire et commiter (voir [ci-dessous](#writes-chemins-de-documentation)) |
| `writes_after` | texte | id d'un checkpoint | écritures ouvertes dès le début | les chemins `writes` ne s'ouvrent qu'une fois ce checkpoint passé (voir [ci-dessous](#writes-chemins-de-documentation)) |

- **Graphe** : `calls` quand il est écrit, sinon la permission `task` de l'agent, limitée aux membres. L'auto-délégation n'est retenue que si elle est écrite (`reviewer: { calls: [reviewer] }`). La profondeur maximale du graphe devient `experimental.subagent_depth` d'opencode.
- **Patch** : fusion par id, champ par champ ; `role: disabled` retire l'agent (sauf l'agent d'entrée) et le retire des listes `calls` des autres. `after` ne peut pas être retiré ni avancé (seulement reporté à un checkpoint déclaré plus loin) ; `calls` écrit par le parent ne peut que se réduire (sous-ensemble) ; un agent ajouté est accepté.

### `scratch` (brouillons de session)

```yaml
agents:
  debugger: { role: workflow, scratch: { readers: [developer] } }
```

L'agent reçoit un dossier de brouillons pour la session : `~/.oh/sessions/<session>/scratch/<agent>/`, hors du dépôt et de la documentation, créé en 0700 au lancement et supprimé avec la session (`oh gc`, `oh session delete`). Son chemin est donné aux agents par l'entrée `oh.scratch` de l'[état de session](../guides/sessions-v5.fr.md#état-de-session), et une ligne de leur prompt la signale.

| Qui | Lecture | Écriture | Shell |
|---|---|---|---|
| le propriétaire (l'agent qui déclare `scratch`) | son dossier, dans cette session | son dossier, dans cette session | non |
| un lecteur (`readers`) | le dossier du propriétaire, dans cette session | non | non |
| tout autre agent, et tout agent d'une autre session (sessions parallèles comprises) | non | non | non |

- `readers` : d'autres agents du workflow, chacun une fois (`scratch_reader_unknown`, `scratch_reader_owner`, `duplicate_entry`). `scratch: {}` : le propriétaire seul.
- Les règles sont des gardes d'oh, placées après les règles de chaque agent (voir [sécurité](../../SECURITY.fr.md)) : une règle de l'agent qui autorise tout ne les assouplit pas. Le shell n'y accède jamais, propriétaire compris : une règle shell suit le texte brut de la commande, une autorisation par chemin autoriserait n'importe quelle commande qui le cite.
- Un brouillon n'est jamais cité dans la documentation ni le wiki : un lint du contenu le vérifie (`TestScratchNeverInTheDocs`).
- Patch : `scratch: null` retire le dossier, une liste `readers` plus courte durcit, un lecteur que le parent ne déclare pas est un assouplissement (`loosening`) ; un dossier ajouté par la couche est accepté. Un agent désactivé sort des lecteurs.

### `writes` (chemins de documentation)

```yaml
risk: plan
agents:
  planner: { role: workflow, writes: ["docs/wiki/specs/**"] }
```

E8 (R4) : un agent qui ne modifie aucun fichier (le `planner`) peut écrire les chemins de documentation déclarés, et seulement eux. Le paquet ajoute, après les règles propres de l'agent et avant les gardes d'oh :

| Action | Règle |
|---|---|
| `edit`, `write` | les chemins déclarés (`docs/wiki/specs/**` devient `docs/wiki/specs/*`) |
| shell | `git add [--] <chemin>` et `git commit -m "<message>" -- <chemin>` (message entre guillemets, un seul chemin déclaré ou son dossier) : seuls ces chemins sont commités |
| shell, refusé (p4-fixes) | tout autre chemin, avant ou après `--` : argument entre le message et `--` (`-m x src/a.go -- …`, `-p`, `--pathspec-from-file`, `:(glob)…`, détecté par un guillemet de plus que le message), second chemin, `..`, `/.`, guillemet, `$`, accent grave ou redirection après le dossier déclaré ; un message qui contient lui-même un guillemet de son type est refusé aussi |
| shell, refusé | `git commit --amend`, `-a`/`--all`, `--include`/`-i`, `--only`/`-o`, `-C`, `git add -A`/`--all`/`-u`/`.`/`-f` (ils prendraient le travail de l'utilisateur) |

- Le commit reste verrouillé jusqu'au checkpoint qui le déverrouille (`unlocks: [commit]`) ; les gardes d'oh (`.git/`, `.beads/`, hooks, `--no-verify`, brouillons, garde globale) passent après et gagnent. Comme les autres gardes, des règles sur le texte brut des commandes : un garde-fou, pas un bac à sable.
- Le contrat de session de l'agent liste ses écritures, la carte du workflow les rappelle, et la condition `writes` des blocs (`<!-- oh:when writes -->`) ne garde un texte que dans ces sessions : le planner de `libre` garde son prompt « tu ne modifies jamais de fichiers ».
- Chemins : relatifs, sous `docs/`, un fichier ou un dossier terminé par `/**`, aucun autre joker (`writes_path_invalid`), chacun une fois (`duplicate_entry`) ; refusé dans un workflow `risk: read` (`writes_read_only`).
- **`writes_after`** (r5-fixes) : tant que ce checkpoint n'est pas passé (un checkpoint sauté dans le mode compte comme passé), l'écriture des chemins `writes` est refusée à tous les agents de la session, avec un message qui dit d'attendre ce checkpoint : la règle de session de ces chemins vaut « demander », le plugin d'oh transmet la demande et oh la refuse avec son message (sans réponse d'oh, l'utilisateur est sollicité). Un checkpoint du workflow présent dans toutes les sessions (`writes_after_unknown`, `order_method_after`), avec `writes` (`writes_after_without_writes`). `workflow_status` liste les écritures encore fermées (`locked_writes`). Patch : comme `after`, il s'ajoute ou se reporte à un checkpoint déclaré plus loin, jamais retiré ni avancé (`loosening`) ; `writes: []` le retire avec les chemins.
- Patch, **en réduction seulement** : une couche d'équipe ou de projet garde, retire ou restreint les chemins hérités (`docs/wiki/specs/equipe/**` sous `docs/wiki/specs/**`, `writes: []`) ; un chemin que le parent ne couvre pas, `writes` sur un agent qui n'en avait pas ou sur un agent ajouté par la couche, est un assouplissement (`loosening`), et la liste du parent est gardée.

---

## Checkpoints

```yaml
checkpoints:
  cp-2:
    label: { fr: Commit ou correction, en: Commit or fix }
    description: Après la review, demander s'il faut committer ou corriger.
    mandatory: true
    mode: { manuel: pause, semi-auto: pause, auto: pause }
    unlocks: [commit, close]
    remote: defer
```

L'id est en kebab-case (`checkpoint_id_invalid`) et ne doit pas être celui d'un agent (`checkpoint_agent_clash`). Les checkpoints sont passés dans l'ordre du fichier. Les skills générées donnent cet ordre sur une ligne « Séquence », avec les opérations que chaque checkpoint déverrouille (« `cp-1` → `cp-2` [commit, close] → `cp-3` ») : les agents s'y réfèrent au lieu de nommer des checkpoints en dur. Chaque agent n'y voit que **ses** checkpoints (`by:`, CP-R11).

| Champ | Type | Valeurs | Absent | Patch |
|---|---|---|---|---|
| `label`, `description` | texte traduisible | — | l'id | remplace |
| `mode.<mode>` | texte | `pause`, `auto`, `skip`, `conditional` | `pause` (avertissement `checkpoint_mode_missing`) | fusion par mode ; durcit si `mandatory` |
| `condition` | texte | phrase lue par l'agent | — (obligatoire avec `conditional`) | remplace |
| `mandatory` | booléen | `true`, `false` | `false` | durcit (`true` ne redevient pas `false`) |
| `remote` | texte | `auto` < `defer` < `forbid` | `defer` | durcit |
| `unlocks` | liste | `commit`, `push`, `close`, `create`, `apply`, `tag`, `mr`, `merge` (checkpoint de merge d'un fan-out seulement) | — (rien de verrouillé ; `apply`, `tag` et `mr` restent refusés) | s'ajoute à la liste héritée (jamais retirée) |
| `disabled` | booléen | `true` | — | retire le checkpoint hérité (patch seulement ; refusé s'il porte `verify`) |
| `verify` | objet | `from`, `kinds`, `cmd`, `expect`, `in`, `timeout`, `must_touch`, `must_touch_tests`, `outside_tests`, `since`, `spec` | pas de vérification | durcit (une vérification héritée ne se retire ni ne s'affaiblit) |
| `when` | objet | `entry`, `labels`, `paths`, `outputs` | pas de condition | durcit (conditions héritées gardées) |
| `by` | liste | agents membres du workflow | déduit du graphe : les agents qui lancent un agent verrouillé par ce checkpoint (`after:`), sinon l'agent d'entrée | remplace |
| `wiki` | texte | `sync` | la carte montre les propositions du wiki de la session | remplace |
| `method` | texte | `tdd` | checkpoint de toutes les sessions | ne se pose pas sur un checkpoint hérité (`loosening`) ; un checkpoint ajouté peut en porter une |
| `after` | texte | id d'un checkpoint déclaré avant | aucun ordre imposé | durcit : s'ajoute ou se reporte à un checkpoint déclaré plus loin, jamais retiré ni avancé (`loosening`) |
| `fanout` | texte | `only` (sessions en fan-out seulement), `never` (les autres seulement) | checkpoint de toutes les sessions | ne se pose ni ne change sur un checkpoint hérité hors de la couche hub (`loosening`) |

- **`after`** (r5-fixes) : oh refuse le checkpoint tant que celui qu'il attend n'est pas passé (un checkpoint sauté dans le mode, ou absent de la session, compte comme passé) ; le refus nomme le checkpoint à passer d'abord, rien n'est déverrouillé et l'utilisateur n'est pas sollicité. Un checkpoint déclaré avant (`checkpoint_after_order`, `checkpoint_after_unknown`), présent dans toutes les sessions (`order_method_after`). La carte du workflow l'indique sur la ligne du checkpoint.
- **Comportements** : `pause` demande toujours l'utilisateur ; `auto` continue seul ; `skip` saute le checkpoint ; `conditional` demande selon `condition`. oh n'évalue pas la phrase `condition` : sans `when:`, un checkpoint `conditional` demande toujours l'utilisateur quand il est signalé ; avec `when:`, oh l'évalue (voir ci-dessous).
- **Ordre de rigueur** : `skip` < `auto` < `conditional` < `pause`. Sur un checkpoint `mandatory`, un patch ne peut pas choisir un comportement moins strict, ni le retirer (`mandatory_checkpoint_removed`) ; `skip` sur un checkpoint obligatoire compte comme une pause.
- **`remote`** (session distante) : `auto` validé automatiquement ; `defer` la session s'arrête proprement et attend l'utilisateur sur la machine ; `forbid` le workflow ne peut pas s'exécuter à distance.
- **`unlocks`** : tant que le checkpoint n'est pas passé, oh refuse ces opérations à **tous les agents** de la session, quel que soit le modèle : `commit` (`git commit`, y compris `env git commit`, `git -c … commit`, `/usr/bin/git commit`, `sh -c '…'`, un alias défini en ligne de commande, et les commandes qui commitent : `revert`, `cherry-pick`, `merge`, `pull`, `am` ; les motifs sont ancrés sur la sous-commande : `git log --grep commit`, `git diff src/pushX.ts`, `bd list --status closed` restent permis), `push` (`git push` ; avertissement `unlock_push_without_publish` hors `risk: publish` : aucun agent livré ne pousse, le push revient à l'utilisateur), `close` (fermeture d'un ticket Beads, refusée aussi par la passerelle Beads), `create` (création de tickets Beads : `bd create`, `bd q`, `bd create-form`, `bd duplicate`, `bd supersede` ; la création est refusée aussi par la passerelle Beads, et la fenêtre ne se referme pas à la fermeture d'un ticket, mais au checkpoint obligatoire suivant ou quand le checkpoint est redemandé), `apply` (modification de l'infrastructure : `terraform apply|destroy`, `kubectl apply|delete|…`, `helm install|upgrade|…`, commandes create/delete/update d'`aws|gcloud|az`, `pulumi up|destroy`), `tag` (création d'un tag : `git tag` sous toutes ses formes, liste comprise — `git describe --tags` et `git for-each-ref refs/tags` lisent les tags —, `git push --tags|--follow-tags`, `git push origin tag v1`, `git push … refs/tags/…`, `git update-ref refs/tags/…`, écriture dans `.git/refs/tags` ou `packed-refs`, `gh|glab release create`), `mr` (ouverture d'une merge request : `glab mr create|new`, `gh pr create|new`, leurs options globales et `sh -c`, `glab api …merge_requests`, `gh api …/pulls`, `hub pull-request`, option de push `merge_request.create`, outil MCP `gitlab` `create_mr`). `apply`, `tag` et `mr` sont refusés dans tout workflow qui ne déclare aucun checkpoint pour les déverrouiller. Quand le même checkpoint déverrouille `commit` et `tag`, le tag reste refusé tant que la fenêtre du commit est ouverte : il marque toujours le commit de release. Les fenêtres `tag` et `mr` se referment, comme `create`, au checkpoint obligatoire suivant ou quand le checkpoint est redemandé ; celle de `tag` aussi après le premier tag (oh relève les tags locaux à l'ouverture de la fenêtre et la referme dès qu'ils changent, comme `commit` une fois le travail commité). Une fois le checkpoint passé, la fermeture d'un ticket n'est acceptée que si un commit non vide (fichiers suivis hors `.beads/`) a été fait depuis et qu'il ne reste rien à committer ; dans un workflow où aucun checkpoint ne déverrouille `close`, la fermeture n'est pas verrouillée. La fenêtre se referme quand le checkpoint est demandé de nouveau (ticket suivant, nouvelle correction), pour `commit` une fois le travail commité (HEAD a avancé et il ne reste rien à committer) et, pour un checkpoint qui déverrouille `close`, quand un ticket est fermé (fenêtres bornées, CP-R8). Une opération n'est déverrouillée que par un seul checkpoint (`unlock_duplicate`), sauf le commit avec `unlock_again` (ci-dessous) ; un checkpoint qui déverrouille ne peut pas être retiré (`unlocking_checkpoint_removed`). Un checkpoint qui porte `unlocks` est traité comme obligatoire : il n'est jamais sauté (`skip` vaut `pause`, avertissement `unlock_skip`) et un patch ne peut pas l'assouplir (`loosening`). Une opération inconnue de la version d'oh utilisée (écrite par une version plus récente) donne un avertissement (`unlock_unknown`) et ne verrouille rien : mettez oh à jour sur tous les postes de l'équipe.
- **`unlock_again`** (p5-fixes, booléen) : ce checkpoint déverrouille de nouveau le commit qu'un checkpoint précédent déverrouille déjà (`unlocks: [commit]` sur les deux) ; `incident` commite l'atténuation à `cp-mitigated`, puis le postmortem à `cp-postmortem`. La fenêtre du checkpoint précédent ne tient alors qu'un commit : oh ne l'ouvre pas s'il n'y a rien à commiter (la réponse le dit et nomme le checkpoint suivant), la referme après le premier commit (même s'il reste des changements) et, au plus tard, quand le checkpoint suivant est demandé ; les propositions du wiki attendent le dernier checkpoint de commit. La réponse de `workflow_checkpoint` et la carte du workflow disent « en un seul commit », jamais la fermeture d'un ticket. Seul le commit se répète (`unlock_duplicate` pour les autres opérations) ; le commit répété n'ajoute pas de pause irréversible au budget (CP-R2). Diagnostics : `unlock_again_first` (aucun checkpoint avant lui ne déverrouille le commit), `unlock_again_without_commit`. Patch : couche hub seulement (`loosening` ailleurs, sur un checkpoint hérité ou ajouté).
- Un mode qui n'est ni autorisé ni un des trois modes livrés : `checkpoint_mode_unknown` ; `conditional` sans `condition` : `checkpoint_condition_missing`.
- Exécution des checkpoints (3 niveaux : prompt, outil MCP `workflow_checkpoint`, plugin oh) : [ADR-042](../architecture/adr/042-checkpoints-headless-decisions.fr.md). Modes, pauses prévues, vérification, preuves et fenêtres : [Checkpoints](checkpoints.fr.md).

### `verify` — vérification lancée par oh

```yaml
    verify:
      from: stack                 # commandes de la stack détectée par oh
      kinds: [build, test]        # défaut : build, test, typecheck (lint sur demande)
      cmd: ["make e2e"]           # commandes écrites ici, lancées sans shell
      expect: pass                # pass (défaut) | fail (test écrit d'abord)
      in: services/api            # sous-dossier de l'emplacement de la session
      timeout: 15m                # durée Go, défaut 10m, au plus 30m
      must_touch: ["*_test.go"]   # les changements de la session touchent un de ces chemins
      must_touch_tests: true      # … et un fichier de test de la stack (R7)
      outside_tests: warn         # fichiers hors tests : warn (signalés) | refuse
      since: cp-seams             # changements comptés depuis le dernier passage de ce checkpoint
      spec:                       # specs écrites par la session (R4)
        paths: ["docs/wiki/specs/**"]
        breakdown: true           # et leur découpage (passe de convergence)
```

Avant que le checkpoint passe, et avant d'ouvrir ses déverrouillages, oh lance les commandes (environnement nettoyé, délai et sortie bornés) ; un échec refuse le passage (`ErrVerifyFailed`, avec un extrait de la sortie). Au checkpoint en pause, l'utilisateur voit le résultat sur la carte et peut **passer outre** (choix tracé). `cmd` ne vient que du document du workflow : jamais de l'agent ni d'une entrée. Dans un workflow `risk: read`, un checkpoint vérifié demande toujours l'utilisateur. Détails : [Checkpoints](checkpoints.fr.md#vérification-lancée-par-oh-verify).

Diagnostics : `verify_empty` (ni `from`, ni `cmd`, ni `spec`), `enum_invalid` (`from`, `kinds`, `expect`), `field_ignored` (`kinds` sans `from`), `verify_cmd_template`, `verify_in_invalid`, `verify_timeout_invalid`, `field_required` (commande ou chemin vide). Champs du test écrit d'abord (R7) : `must_touch_tests` exige qu'un fichier de test de la stack soit modifié (motifs de `bricks.TestFilePatterns` : `*_test.go`, `*.test.*`, `*.spec.*`, `test_*.py`, `**/tests/**`, `**/src/test/**`…) ; `outside_tests` dit ce que font les autres fichiers modifiés (`warn` : signalés dans le résultat ; `refuse` : refusés ; absent : rien) ; `since` compte les changements depuis le commit relevé au dernier passage d'un checkpoint déclaré avant (sinon depuis le début de la session). Avec `expect: fail`, oh lance d'abord les commandes qui construisent le code (`build`, `typecheck`, `lint`), qui doivent réussir, puis les tests, qui doivent échouer à l'exécution et non à la compilation. Diagnostics : `verify_outside_tests_ignored` (avertissement : `outside_tests` sans `must_touch_tests`), `verify_since_unknown`, `verify_since_method` (le checkpoint de `since` n'existe que dans une méthode que ce checkpoint n'a pas). Détails : [Checkpoints](checkpoints.fr.md#méthode-tdd--cp-seams-et-cp-red-r7).

Specs écrites (R4) : avec `spec`, oh lit les fichiers `.md` modifiés par la session sous `paths` (un lien n'est pas suivi) et vérifie d'abord le format du wiki, comme `wiki_check` : frontmatter présent et lisible, avec les seules clés `updated` (date `AAAA-MM-JJ`), `confidence` (`confirmed`, `mixed` ou `uncertain`, cohérente avec les tags de la page : la plus basse l'emporte) et `agents` (non vide), et lignes de tag de confiance bien formées ; puis (`internal/specdoc`) une section « Glossaire » qui définit au moins un terme, des « Critères d'acceptation » qui portent chacun un identifiant `CA-n` et, sur leur ligne, leur point de test (`· Test : <test ou commande>`, une valeur réelle : ni vide, ni « à définir ») et, avec `breakdown: true`, une section « Découpage » dont chaque tranche cite les critères qu'elle couvre (`Couvre : CA-1, CA-2`) : chaque critère couvert, aucune tranche sans critère ni critère inconnu. Listes et tableaux sont lus, titres en français ou en anglais (`Glossary`, `Acceptance criteria`, `Breakdown`). Aucune spec modifiée, ou un problème : la vérification échoue avec la liste des problèmes. `spec` seul suffit (sans `from` ni `cmd`) ; diagnostics `field_required` (`paths` vide), `verify_spec_path_invalid`.

Dans un patch, la vérification héritée ne peut que se renforcer (même `from`, `expect`, `in` et `since`, types, commandes et chemins conservés, `must_touch_tests` gardé, `outside_tests` au moins aussi strict — absent < `warn` < `refuse` —, délai au moins égal, chemins de `spec` gardés et `breakdown` conservé) : sinon `loosening` ; retirer le checkpoint : `verified_checkpoint_removed`.

### `when` — conditions évaluées par oh

```yaml
    mode: { manuel: pause, semi-auto: conditional, auto: auto }
    when: { entry: [debugger], labels: [ux], paths: ["docs/specs/**"], outputs: [mr] }
```

Une condition suffit. Un checkpoint `auto` met en pause quand une condition est vraie ; un `conditional` ne met en pause que dans ce cas ; un checkpoint en `pause`, obligatoire ou qui déverrouille une opération reste toujours en pause : `when:` ne peut qu'ajouter une pause. Avec `when:`, `condition` n'est plus obligatoire. Diagnostics : `when_empty`, `when_mandatory` (avertissement : sans effet), `when_unknown_agent`, `when_unknown_output` (avertissements), `field_required`.

### `by` — qui passe le checkpoint

```yaml
  cp-2:
    by: [orchestrator-dev]
```

Les agents qui passent le checkpoint (CP-R11). Le contrat de session de chaque agent ne nomme que ses checkpoints, et la table générée des checkpoints ne montre à un agent que les siens. Quand un autre agent passe le checkpoint, oh le laisse passer, l'écrit dans son journal et ajoute un avertissement à la réponse de `workflow_checkpoint`. Tous les checkpoints des workflows livrés l'écrivent (test `TestCheckpointByMembers`). Diagnostics : `checkpoint_by_unknown` (erreur : pas un membre), `checkpoint_by_duplicate` (avertissement).

### `wiki` — synchronisation du wiki

```yaml
  cp-sync:
    unlocks: [commit]
    wiki: sync
```

`sync` (workflow `doc-sync`) : la carte du checkpoint montre **toute la file** des propositions du wiki du projet, et non celles de la session seule ; l'utilisateur décide de chacune. Au passage, oh applique les propositions acceptées et écrit dans le frontmatter de `docs/wiki/index.md` le commit dont la session est partie (`synced:`), point de départ de la synchronisation suivante. Le checkpoint doit déverrouiller le commit : sinon `checkpoint_wiki_commit` (erreur) ; une autre valeur : `enum_invalid`. Voir [Wiki documentaire vivant](../architecture/living-wiki.fr.md#synchronisation--workflow-doc-sync-ld-d).

### `method` — méthode de la session

```yaml
inputs:
  method: { type: enum, values: [standard, tdd], default: standard }
checkpoints:
  cp-seams:
    method: tdd
    mode: { manuel: pause, semi-auto: pause, auto: auto }
```

Un checkpoint qui porte `method` n'existe que dans les sessions lancées avec cette méthode (entrée `method`, `oh run ticket -i method=tdd`, ou la fiche de lancement) : oh évalue la méthode au lancement et le paquet de session ne contient que les checkpoints de la méthode choisie (CP-R7). Une session `standard` a donc exactement le paquet d'un workflow sans ces checkpoints. Seule valeur aujourd'hui : `tdd` (`enum_invalid` sinon). Un tel checkpoint n'ajoute qu'une étape : il ne peut être ni obligatoire ni déverrouiller une opération (`checkpoint_method_mandatory`), aucun agent ne peut l'attendre (`after:`, `checkpoint_method_after`), et l'entrée `method` doit proposer la méthode (`checkpoint_method_no_input`). Dans un patch, poser ou changer `method` sur un checkpoint hérité est un assouplissement (`loosening`) : il disparaîtrait des autres sessions.

### `fanout` — sessions enfants (R6, expérimental)

```yaml
inputs:
  fanout: { type: bool, default: false }
fanout:
  input: fanout            # entrée bool qui fait de la session un fan-out
  tickets: tickets         # entrée beads-ids : une session enfant par ticket
  child: ticket            # workflow de chaque session enfant
  parallel: 3              # sessions enfants en même temps (1 à 3)
  merge: cp-merge          # checkpoint de merge de la session parente
  delegates: [cp-2]        # checkpoints des enfants remplacés par cp-merge (couche hub seulement)
  merger: merge-conflicts  # workflow lancé par oh sur un conflit qu'il ne résout pas
  inputs: [method, instructions]  # entrées de la parente transmises à chaque enfant (même nom)
  fix: { workflow: quick, input: request }  # tour de correction quand cp-merge est refusé
checkpoints:
  cp-2: { fanout: never, … }
  cp-merge:
    fanout: only
    mandatory: true
    unlocks: [merge]
    verify: { from: stack, expect: pass, on: integration }
```

Une session lancée avec l'entrée `fanout` à `true` (`oh run implement-spec -i fanout=true`) est un **fan-out** : oh lance une session enfant par ticket, chacune dans sa branche et son worktree, la merge lui-même dans une branche d'intégration, confie au `merger` un conflit qu'il ne résout pas, et, une fois le checkpoint de merge validé et vérifié **sur la branche d'intégration**, merge celle-ci dans la branche de la session et ferme les tickets mergés. Le fan-out est expérimental et désactivé par défaut (`[fanout] experimental = true`, voir le [guide](../guides/fan-out.fr.md)). oh évalue l'entrée au lancement : une session en fan-out ne garde que les checkpoints sans `fanout` ou `fanout: only` (les agents verrouillés par un checkpoint `fanout: never`, le developer après `cp-1`, sont retirés du paquet) ; les autres sessions gardent exactement le paquet d'avant (`fanout: only` retiré, bloc `fanout` ignoré).

**Substitution de contrôle (CP-R3).** `delegates` liste des checkpoints du workflow enfant qui ne font plus de pause dans les enfants : le checkpoint de merge les remplace. Le schéma l'exige d'un checkpoint de merge **obligatoire**, `fanout: only`, qui déverrouille `merge` et dont la vérification lancée par oh tourne sur la branche d'intégration (`verify.on: integration`, `expect: pass`) ; la délégation n'est déclarée que par la **couche hub** (une couche d'équipe ou de projet, ou un brouillon, qui l'écrit : `fanout_delegates_layer`). Un patch ne peut que durcir le fan-out : seul `parallel` peut baisser ; changer `input`, `tickets`, `child`, `merge`, `merger` ou `delegates`, retirer le bloc, ou poser `fanout:` sur un checkpoint hérité est un `loosening`. Le démon revérifie tout à l'exécution (voir [Checkpoints](checkpoints.fr.md#fan-out--substitution-de-contrôle-cp-r3)).

**Entrées des enfants.** `inputs` liste les entrées de la session parente transmises à chaque enfant sous le même nom, quand la session les a reçues (une valeur par défaut n'est pas transmise : l'enfant garde la sienne) ; rien d'autre de la parente n'arrive aux enfants. Chacune est une entrée de valeur de la parente (`string`, `text`, `bool`, `int`, `enum`, `path`, `branch` ; ni l'entrée du fan-out, ni celle des tickets : `fanout_inputs_parent`), déclarée par le workflow enfant avec le même type (`fanout_inputs_child`, `fanout_inputs_type`) et, pour un `enum`, chacune de ses valeurs (`fanout_inputs_value`). Une couche d'équipe ou de projet peut en transmettre moins, jamais plus (`loosening`).

**Tour de correction.** `fix` nomme le workflow qu'oh lance dans le worktree d'intégration quand le checkpoint de merge est refusé avec une consigne, et son entrée texte qui la reçoit, avec ce qui a échoué à la vérification (`fanout_fix_input`). Ce workflow doit avoir un checkpoint obligatoire qui déverrouille le commit (`fanout_fix_commit`) : sa correction est validée par l'utilisateur, jamais déléguée. oh reprend ses commits, puis le checkpoint de merge se repasse et se revérifie sur la branche d'intégration : la substitution de contrôle ne change pas. Au plus `limits.max_fix_rounds` tours (1 sans limite). Un patch d'une autre couche que le hub ne le change pas (`loosening`).

Opération `merge` : `git merge`, `git pull` et les merges des forges (`glab mr merge`, `gh pr merge`, leurs API, outil MCP `gitlab` `merge_mr`). Seul le checkpoint de merge d'un fan-out la déverrouille (`unlock_merge_fanout_only`), et c'est oh qui merge au passage : aucune fenêtre ne s'ouvre pour les agents.

Diagnostics : `fanout_input`, `fanout_tickets`, `fanout_parallel`, `fanout_unknown_workflow`, `fanout_child_self`, `fanout_child_nested` (un seul niveau d'enfants), `fanout_delegate_unknown` (checkpoint absent du workflow enfant), `fanout_merge_unknown`, `fanout_merge_scope`, `fanout_merge_mandatory`, `fanout_merge_unlocks`, `fanout_merge_verify`, `fanout_merge_verify_on`, `fanout_merge_verify_expect`, `fanout_delegates_layer`, `checkpoint_fanout_without_fanout`, `unlock_merge_fanout_only`, `verify_on_integration_merge_only`, `fanout_inputs_parent`, `fanout_inputs_child`, `fanout_inputs_type`, `fanout_inputs_value`, `fanout_fix_input`, `fanout_fix_commit`.

### Budget de pauses

En semi-auto (ou dans le mode par défaut si semi-auto n'est pas autorisé), un workflow met en pause au plus **2 fois pour des décisions humaines, plus 1 fois par checkpoint qui déverrouille une action irréversible** ; un checkpoint `conditional` ou avec `when:` compte comme une pause. Une méthode (`method`) ajoute au plus 1 pause, pour ses seules sessions. Au-delà : avertissement `pause_budget_exceeded` (bloquant pour les workflows livrés, test `TestCheckpointPauseBudget`). Voir [Checkpoints](checkpoints.fr.md#budget-de-pauses-cp-r2).

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

Testées avant le lancement, dans l'ordre du fichier, dans le dossier du projet. `check` porte au moins un test (`precondition_check_invalid`) ; s'il en porte plusieurs, tous doivent être vrais.

```yaml
preconditions:
  clean-tree:
    label: { fr: L'arbre de travail doit être propre avant une release, en: The work tree must be clean before a release }
    check: { git_clean: true }
    on_fail: block
  release-branch:
    check: { branch: [main, "release/*"] }
    on_fail: block
  tools:
    check: { tools: [goreleaser] }
```

| Champ | Type | Valeurs | Absent |
|---|---|---|---|
| `label` | texte traduisible | message affiché en cas d'échec | l'id |
| `check.path_exists` | liste | chemins relatifs, sans `..` ; l'un d'eux doit exister | — (`precondition_path_invalid`) |
| `check.git_clean` | booléen | `true` : dépôt git sans modification non commitée (indexée, non indexée, ou fichier non suivi et non ignoré ; `.beads/` exclu) | — |
| `check.branch` | liste | noms ou motifs de branche (`release/*`, `path.Match`) ; la branche courante doit correspondre à l'un d'eux, un HEAD détaché à aucun | — (`precondition_branch_invalid`) |
| `check.tools` | liste | noms de commandes (sans chemin ni argument) ; toutes doivent être dans le `PATH` d'oh | — (`precondition_tool_invalid`) |
| `on_fail` | texte | `suggest`, `block` | `suggest` |
| `suggest.workflow` | texte | id d'un autre workflow du catalogue | aucune proposition (`precondition_self`, `precondition_unknown_workflow`) |
| `suggest.resume` | booléen | `true`, `false` | `false` |
| `disabled` | booléen | `true` | retire une précondition héritée (patch seulement ; jamais dans un workflow `risk: publish`) |

- `suggest` : message + proposition de lancer `suggest.workflow` ; l'utilisateur peut continuer. Avec `resume: true`, oh propose de relancer le workflow d'origine (mêmes entrées) une fois l'autre terminé.
- `block` : le lancement est refusé.
- En distant ou sans interface : un échec `suggest` devient un avertissement, un échec `block` arrête la session.
- Message : le libellé, suivi de ce qui a échoué, lu par oh — modifications non commitées (les cinq premières, puis leur nombre), branche courante et branches attendues, HEAD détaché, outils introuvables, dossier hors git. Les lectures git passent par le git sécurisé d'oh (ni hook, ni fsmonitor, ni configuration du dépôt qui lance un programme) ; une lecture qui échoue fait échouer la précondition.
- Patch : fusion par id, champ par champ. Dans un workflow `risk: publish`, les préconditions héritées sont des champs de sécurité : une couche peut en ajouter ou les durcir (`branch` plus étroit, outils ajoutés, `on_fail: block`), jamais les retirer ni les assouplir — `disabled`, `git_clean` retiré, motif de branche que le parent ne couvre pas, outil retiré, chemin ajouté à `path_exists`, `block` changé en `suggest` sont des assouplissements (`loosening`), et la précondition du parent est gardée.

---

## Ressources

| Champ | Type | Absent | Patch |
|---|---|---|---|
| `models.default` | `fournisseur/modèle[#variante]` | cascade sans niveau workflow | remplace |
| `models.agents.<agent>` | `fournisseur/modèle[#variante]` | — | fusion par agent |
| `skills.extra` | liste de références de skills | — | remplace |
| `skills.deny` | liste (référence ou nom seul) | — | remplace |
| `mcp` | liste d'ids de serveurs MCP oh | serveurs MCP du projet | durcit hors couche hub (sous-ensemble de la liste du parent ; parent sans liste : toute liste durcit) |
| `plugins` | liste | aucun plugin en plus du plugin oh | durcit hors couche hub (sous-ensemble du parent : un plugin exécute du code) |
| `beads.allow` | voir [Sécurité](#beads) | | |

- **`models`** : niveau workflow de la [cascade des modèles](model-resolution.fr.md) (workflow·agent > workflow > projet…). Format invalide : `model_invalid` ; agent hors du workflow : `model_agent_unknown`.
- **`skills`** : les skills viennent des agents membres et de leurs dépendances (`requires:`). `extra` en ajoute (`skill_unknown`), `deny` en retire (une référence `chemin/skill` ou un nom seul). Une skill refusée mais requise par une autre : `skill_denied_required` ; dépendance introuvable : `skill_closure` ; identifiant en double : `skill_duplicate`.
- **`mcp`** : ids des serveurs MCP d'oh (`gitlab`, `figma`, `jira`, `team`…). Absent : la session garde les serveurs MCP du projet ; présent (même vide) : seulement ceux de la liste. Le serveur `workflow` (`workflow_status`, `workflow_checkpoint`, `workflow_outputs`, `workflow_report`, `workflow_report_get`, `workflow_reports` : voir [serveur MCP workflow](cli-mcp.fr.md#serveur-mcp-workflow)) est toujours ajouté. En `risk: read`, les serveurs sont lancés **sans leurs outils d'écriture**, quel que soit le réglage du projet (`write_enabled`).
- **`plugins`** : un id (spécification npm, ex. `context-mode@latest`) ou `{ id, options }`. Refusé en `risk: read` pour un plugin dont les outils exécutent des commandes (`context-mode` : `read_plugin_exec`). Installés par opencode au démarrage du serveur ; un plugin qui ne se charge pas n'empêche pas la session. Le module doit exporter le format opencode V2 (`{ id, setup }`).
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
| `type` | texte | `branch`, `merge_request`, `beads-ids`, `path`, `report` (identifiant d'un rapport `rp-…`), `text` (une valeur, par exemple une entrée relevée) |
| `label` | texte traduisible | libellé affiché |
| `record` | texte | absent : déclarée par l'agent ; sinon **relevée par oh** (R5) : `created`, `closed`, `spec`, `report`, `input` |

Sans `record`, l'agent déclare la valeur avec l'outil MCP `workflow_outputs` (la carte du workflow la liste dans « Sorties à déclarer », `workflow_status` dans `expected_outputs`). En fin de session, les sorties proposent « Enchaîner avec… » (touche `e` de la vue Sessions), les étapes [`next`](#composition-next-et-aliases) et `oh session results`. Patch : liste remplacée en bloc.

### Sorties relevées par oh (`record`)

Une sortie `record:` est remplie par oh à partir de ce qu'il voit la session faire ; aucun agent ne peut la déclarer (`workflow_outputs` la refuse) et la carte du workflow ne la mentionne pas. Un workflow sans sortie à déclarer (aucune, ou seulement des sorties `record:`) ne présente pas l'outil `workflow_outputs` à ses agents : oh le leur refuse, ce qui le retire de leurs outils.

| `record` | Type | Ce que oh relève |
|---|---|---|
| `created` | `beads-ids` | les tickets créés par la session à travers la passerelle Beads (`bd create`, `create --file`, `q`, `--json`, `--silent`), ajoutés au fil des créations |
| `closed` | `beads-ids` | les tickets fermés par la session à travers la passerelle Beads |
| `spec` | `path` | les specs vérifiées par oh à un checkpoint `verify.spec` réussi (plusieurs : jointes par des virgules) ; sans checkpoint `verify.spec`, avertissement `output_record_spec_unchecked` |
| `report` | `report` | le dernier rapport déposé avec `workflow_report` |
| `input` | `text` | la valeur de l'entrée de même id, au lancement (sinon sa valeur par défaut), pour la transmettre par `next:` : `spec` relève `method` et la passe à `implement-spec` ; sans entrée de ce nom, `output_record_input_unknown` |

- Le type doit être celui du tableau (`output_record_type`).
- En fin de workflow, oh ne complète plus une sortie relevée avec les tickets fermés (comme il le fait pour une sortie `beads-ids` déclarée).
- Une session distante ne relève pas les créations (les tickets n'y ont qu'un identifiant provisoire).

---

## Composition (`next` et `aliases`)

```yaml
aliases: [cadrage, feature]        # oh run cadrage lance ce workflow
next:
  - workflow: implement-spec
    when: { outputs: [tickets] }   # proposé seulement si la session a cette sortie
    inputs:
      tickets: session.output(tickets)
      spec: session.output(spec)
      method: session.output(method) # relevée au lancement (record: input)
```

### `next` — ce que oh propose ensuite (E4)

| Champ | Type | Rôle |
|---|---|---|
| `workflow` | id | workflow proposé (ou un alias) |
| `label` | texte traduisible | libellé de la proposition (défaut : celui du workflow) |
| `when.outputs` | liste d'ids | la proposition n'est faite que si la session a toutes ces sorties |
| `inputs.<entrée>` | `session.output(<sortie>)` | entrée du workflow proposé, préremplie avec une sortie de la session qui se termine |

- **Une proposition, jamais un lancement** : quand le client d'une session lancée par `oh run` se ferme, oh affiche les sorties de la session et la commande de chaque étape (`oh run implement-spec --parent ses_… --tickets ev-1,ev-2 -i spec=…`) ; `oh session next <id>` les redonne ; la vue Sessions de la TUI les place en tête de « Enchaîner avec… » et dans la notification de fin de session.
- Le lancement proposé porte `--parent` : les sources `session.output(…)` de ses entrées lisent d'abord cette session.
- Contrôles : `next_unknown_workflow`, `next_self`, `duplicate_entry`, `next_unknown_output` (sortie de `when` ou de `inputs` non déclarée), `next_input_invalid` (autre forme que `session.output(<sortie>)`), `next_unknown_input` (entrée absente du workflow proposé), `next_input_type` (type incompatible). Patch : liste remplacée en bloc.

### `aliases` — autres noms du workflow

- `aliases: [cadrage]` ou `aliases: [{ id: feature, with: tickets }]` : `oh run <alias>` (et la fiche de lancement, `oh eval`) lance ce workflow et l'annonce (« cadrage est un alias : lancement de spec »). Les entrées données vont au workflow cible.
- **`with`** : l'alias ne va vers ce workflow que si cette entrée est donnée au lancement (`--tickets` remplit l'entrée de tickets) ; sinon il va vers le workflow qui le déclare sans `with`. Un même alias peut ainsi aller vers deux workflows selon les entrées.
- Un document qui porte l'id de l'alias, dans n'importe quelle couche, l'emporte : l'alias ne s'applique plus (`alias_shadowed`, avertissement).
- Contrôles : `id_invalid`, `alias_self`, `duplicate_entry`, `alias_with_unknown_input` ; sur le catalogue : `alias_ambiguous` (alias déclaré sans `with` par deux workflows), `alias_without_default` (alias qui n'a que des routes `with`). Patch : liste remplacée en bloc.

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
| `unknown_workflow`, `extends_not_found`, `extends_alias`, `invalid_extends`, `extends_cycle`, `extends_more_specific`, `duplicate_workflow` | chaîne `extends` impossible |
| `shadow_without_extends`, `must_extend_nearest` | même id qu'une couche inférieure sans l'étendre, ou sans étendre la plus proche |
| `loosening` | un champ de sécurité est assoupli : la valeur du parent est gardée |
| `enforced_field`, `enforce_unknown_field` | champ verrouillé par un parent ; valeur d'`enforce` inconnue |
| `entry_not_primary`, `entry_role`, `entry_disabled`, `agent_unknown`, `agent_unreachable` | agent d'entrée ou membre invalide |
| `after_unknown`, `after_cycle`, `calls_unknown`, `graph_cycle` | verrous `after` et délégations |
| `scratch_reader_unknown`, `scratch_reader_owner` | lecteur de brouillons absent du workflow, ou propriétaire lui-même |
| `writes_after_unknown`, `writes_after_without_writes`, `checkpoint_after_unknown`, `checkpoint_after_order`, `order_method_after` | ordre des écritures et des checkpoints (r5-fixes) |
| `writes_path_invalid`, `writes_read_only` | chemin d'écriture hors de `docs/` ou avec un joker autre que `/**` ; `writes` dans un workflow `risk: read` |
| `verify_spec_path_invalid` | chemin de `verify.spec` absolu ou hors de l'emplacement |
| `checkpoint_agent_clash`, `checkpoint_mode_unknown`, `checkpoint_mode_missing`, `checkpoint_condition_missing`, `mandatory_checkpoint_removed`, `unlocking_checkpoint_removed`, `unlock_duplicate`, `unlock_again_first`, `unlock_again_without_commit`, `unlock_unknown`, `unlock_skip`, `unlock_push_without_publish`, `unlock_requires_publish`, `publish_checkpoint_remote`, `publish_checkpoint_pause`, `patch_unknown_checkpoint`, `checkpoint_by_unknown`, `checkpoint_by_duplicate`, `checkpoint_wiki_commit`, `headless_checkpoint_pause` | checkpoints |
| `verify_empty`, `verify_cmd_template`, `verify_in_invalid`, `verify_timeout_invalid`, `verified_checkpoint_removed`, `when_empty`, `when_mandatory`, `when_unknown_agent`, `when_unknown_output`, `pause_budget_exceeded` | vérification lancée par oh, conditions, budget de pauses (R3) |
| `fanout_input`, `fanout_tickets`, `fanout_parallel`, `fanout_unknown_workflow`, `fanout_child_self`, `fanout_child_nested`, `fanout_delegate_unknown`, `fanout_merge_unknown`, `fanout_merge_scope`, `fanout_merge_mandatory`, `fanout_merge_unlocks`, `fanout_merge_verify`, `fanout_merge_verify_on`, `fanout_merge_verify_expect`, `fanout_delegates_layer`, `checkpoint_fanout_without_fanout`, `unlock_merge_fanout_only`, `verify_on_integration_merge_only`, `fanout_inputs_parent`, `fanout_inputs_child`, `fanout_inputs_type`, `fanout_inputs_value`, `fanout_fix_input`, `fanout_fix_commit` | fan-out et substitution de contrôle (R6) |
| `method_input_type`, `checkpoint_method_no_input`, `checkpoint_method_mandatory`, `checkpoint_method_after`, `verify_since_unknown`, `verify_since_method`, `verify_outside_tests_ignored` | méthode de la session et test écrit d'abord (R7) |
| `precondition_check_invalid`, `precondition_path_invalid`, `precondition_branch_invalid`, `precondition_tool_invalid`, `precondition_self`, `precondition_unknown_workflow`, `patch_unknown_precondition` | préconditions |
| `prompt_both`, `prompt_empty`, `prompt_template_path`, `prompt_template_missing`, `prompt_parse`, `prompt_unknown_variable` | prompt |
| `next_unknown_workflow`, `next_self`, `next_unknown_output`, `next_input_invalid`, `next_unknown_input`, `next_input_type`, `alias_self`, `alias_shadowed`, `alias_with_unknown_input`, `alias_ambiguous`, `alias_without_default`, `output_record_type`, `output_record_spec_unchecked`, `input_from_type`, `input_from_unknown_workflow`, `input_from_unknown_output`, `input_from_session_workflow` | composition : `next`, `aliases`, sorties relevées par oh, sources `session.*` (R5) |
| `read_agent_writes`, `read_beads_unrestricted`, `read_beads_write`, `read_plugin_exec` | règles de `risk: read` |
| `plan_agent_writes`, `plan_beads_unrestricted`, `plan_beads_delete`, `plan_without_beads_write` | règles de `risk: plan` |
| `remote_forbidden_checkpoint`, `publish_remote_runtime`, `runtime_default_not_allowed`, `mode_default_not_allowed`, `isolation_unsupported` | environnements, modes, isolation |
| `parallel_writers_without_commit_gate` | avertissement : `risk: write`, au moins deux agents qui modifient des fichiers et aucun checkpoint qui déverrouille `commit` (sauf entrée au choix, `libre`) |
| `model_invalid`, `model_agent_unknown`, `skill_unknown`, `skill_unknown_deny`, `skill_closure`, `skill_duplicate`, `skill_denied_required` | ressources |
| `session_mode_not_allowed`, `session_runtime_not_allowed`, `session_input_unknown`, `session_input_invalid`, `session_input_missing`, `verify_cmd_missing` (une entrée `enum` qui vaut `custom` exige l'entrée `<entrée>_cmd` quand le workflow la déclare), `session_entry_not_selectable`, `session_entry_unknown` | options de lancement |

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
