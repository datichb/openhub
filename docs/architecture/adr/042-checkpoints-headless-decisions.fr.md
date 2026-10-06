> [Read in English](042-checkpoints-headless-decisions.en.md)

# ADR-042 — Checkpoints à trois niveaux et décisions sans interface

## Statut

Accepté

## Date

2026-10-05

## Contexte

Les checkpoints de l'[ADR-003](./003-orchestrator-checkpoints.fr.md) (`[CP-0]` à `[CP-3]`) et les modes de l'[ADR-006](./006-orchestrator-configurable-mode.fr.md) n'existaient que dans les prompts. L'agent posait la question dans la conversation et attendait une réponse.

Rien ne l'y obligeait :

- un modèle pouvait sauter un checkpoint ou lancer le développeur avant la validation ;
- une décision n'était visible que dans l'interface de la session ;
- avec plusieurs sessions, il fallait passer de fenêtre en fenêtre.

opencode V2 fournit ce qu'il faut pour contrôler cela de l'extérieur : demandes de permission et formulaires pilotables par l'API (F17), plusieurs clients sur la même session dont la première réponse gagne (F20), règles de session (F26), crochets de plugin (F27).

Décision D8 : checkpoints sur trois niveaux, machine à états dans `oh`. Usages S3 à S6 du mode serveur.

## Décision

### 1. Niveau 1 — consignes générées

- Les skills générées depuis le workflow ([ADR-039](./039-declarative-workflows-oh-v1.fr.md)) demandent à l'agent d'appeler `workflow_checkpoint {id, summary}` à chaque checkpoint, et pas l'outil `question`.
- Le prompt initial se termine par la liste des checkpoints à signaler avant chaque agent verrouillé.

### 2. Niveau 2 — outil MCP et permissions

- **Serveur MCP `workflow`** (`oh mcp serve workflow`, ajouté à chaque paquet, sans état) : `workflow_status`, `workflow_checkpoint` et `workflow_outputs`. Il interroge le démon `ohd`, qui identifie la session appelante par le `_meta` de l'appel et rattache une sous-session à sa session racine.
- **Règles du paquet** : `workflow_checkpoint` en `ask`, `workflow_status` et `workflow_outputs` en `allow`.
- **Règles de session**, recalculées selon le mode et l'état :
  - `workflow_checkpoint` en `ask` si un checkpoint attend l'utilisateur dans le mode, sinon `allow` ;
  - `subagent <agent>` refusé tant que son verrou `after:` n'est pas levé ;
  - `subagent *` refusé pendant le coupe-circuit.
- Une règle ne peut pas viser un checkpoint précis : opencode 2.0.20 nomme l'action `workflow_workflow_checkpoint` sur toutes les ressources. Le démon lit donc l'entrée de l'appel :
  - checkpoint automatique dans le mode : il répond `once` lui-même ;
  - checkpoint en pause : il lève une décision ⏸.
- **Contrôle dynamique** par l'API (`PATCH /api/session/{id}` avec les règles), à chaque resynchronisation, checkpoint passé, délégation terminée qui lève un verrou, et pose ou levée du coupe-circuit. Les règles sont aussi appliquées aux sous-sessions.

### 3. Niveau 3 — plugin oh

- `permission.hook("evaluate")` n'est appelé que pour les demandes en `ask`. Le plugin relaie la demande au démon (`POST /oh/v1/hooks/permission`, sur les écoutes du proxy, authentifié par le jeton du proxy du groupe, délai de 2 s).
- La seule décision prise est `allow` pour un checkpoint qui n'attend pas l'utilisateur dans le mode : il n'est alors jamais montré. Sinon la règle tient.
- Sans plugin, le niveau 2 suffit (réponse du démon).

### 4. CheckpointService

`internal/services/checkpoint` tourne dans le démon. Il tient une machine à états par session, enregistrée dans `sessions.checkpoint_state` (migration v35, mise à jour par comparaison-échange, car le démon, la CLI et la TUI écrivent) :

- checkpoints passés ou en attente ;
- validations pas encore vues par l'agent ;
- agents qui ont tourné ;
- compteur du coupe-circuit ;
- frise (200 entrées au plus).

Les identifiants de checkpoint sont insensibles à la casse.

### 5. Coupe-circuit

Au-delà de `circuit_breaker.max_consecutive_subagents` appels `subagent` consécutifs sans intervention de l'utilisateur, une décision ✗ dédiée (`circuit`) est levée et `subagent *` est refusé. Classer la décision lève le refus, avec une consigne facultative.

### 6. Décisions sans interface

- Le démon recopie dans `pending_decisions` (migration v33) les demandes en attente de l'outil, d'après le flux SSE et à chaque resynchronisation. Les types sont : checkpoint ⏸, question ?, permission !, budget $, erreur ✗ et coupe-circuit.
- Une demande réglée ailleurs (interface opencode, navigateur) est marquée résolue par l'outil. Les demandes des sous-sessions sont classées sous la session racine.
- `SessionService.Decide` **réserve** la décision en base (opération atomique, la première réponse gagne) avant de répondre à l'outil, et la rouvre si la livraison échoue.
- Réponses possibles :
  - **permission** : `once`, `always` (interdit si `isolation: strict`) ou `reject`, avec un message ;
  - **question** : réponses typées et validées ;
  - **checkpoint** : `approve`, `fix`, `other` ou `reject`.
- opencode 2.0.20 ne transmet pas le message d'une réponse à l'agent :
  - la note d'une validation est rendue par l'outil MCP quand l'appel s'exécute ;
  - la consigne d'un refus est envoyée par un message `steer`.
- oh peut aussi envoyer un message ou un message `synthetic` (S6), interrompre, changer de modèle, compacter ou forker.

### 7. Surfaces

- **TUI** :
  - section « À traiter » de la vue Sessions ;
  - fiche checkpoint : résumé de l'agent, changements, derniers messages, frise `✔ cp-1 → developer → ⏸ cp-2 → ○ cp-3`, et actions Valider, Corriger d'abord ou Autre consigne ;
  - badge `● N ⏸ M` sur tous les écrans.
- **Notifications système** envoyées par le démon, que la TUI soit ouverte ou non (`terminal-notifier`, `osascript`, `notify-send`), sans aucun contenu de la session.
- **CLI** : `oh session inbox|approve|answer|dismiss|send|interrupt|model|compact|fork`.
- **Distant** : un répondeur de politique remplace l'utilisateur ([ADR-045](./045-execution-environments.fr.md)).

## Conséquences

### Positives

- Les checkpoints et les verrous `after:` sont appliqués par oh, pas par la bonne volonté du modèle : `developer` est refusé avant `cp-1` puis autorisé après la validation (e2e Haiku `TestE2ECheckpointGating`).
- Une décision se prend depuis n'importe quel client, et la première réponse gagne.
- Plusieurs sessions sont suivies d'un seul endroit, avec notifications.
- La même chaîne fonctionne en conteneur : le serveur MCP `workflow` passe par la passerelle MCP.
- Le niveau 2 reste un repli si le plugin ne se charge pas.

### Négatives / Compromis

- Le mécanisme repose sur des comportements précis d'opencode 2.0.20 : nom d'action `<serveur>_<outil>`, `evaluate` appelé seulement pour `ask`, message de refus non transmis, l'agent ne voyant que « Unable to execute ». Ces points sont à revérifier à chaque version.
- Un agent qui n'appelle pas le checkpoint reste bloqué devant un agent verrouillé : le refus est sûr, mais la session peut s'arrêter. En recette, il a fallu ajouter le rappel en fin de prompt ou envoyer une consigne. Le nom de l'outil hésite parfois (`workflow_checkpoint` / `workflow_workflow_checkpoint`, anomalie Q3-2).
- La `condition` d'un checkpoint `conditional` n'est pas évaluée : il demande toujours l'utilisateur.
- Trois niveaux, trois écrivains de l'état : la complexité se paie en tests (unitaires, contrat, e2e).
- Le compteur du coupe-circuit est une heuristique : il ne distingue pas les délégations utiles des boucles.

## Alternatives considérées

| Alternative | Rejetée car |
|---|---|
| Garder les checkpoints dans la conversation (outil `question` ou texte) | Rien n'empêche l'agent de les sauter, et la décision n'est visible que dans l'interface de la session. |
| Une règle de permission par checkpoint | opencode ne distingue pas les checkpoints dans l'action (même action, ressources `*`). |
| Contrôle dynamique par `ctx.permission.rules` dans le plugin (F26) | Documenté mais non nécessaire : `PATCH` des règles de session par l'API a été vérifié et ne dépend pas du plugin. |
| Machine à états dans le plugin | Logique métier dans l'outil, perdue à chaque redémarrage du serveur, invisible des autres clients (O6). |
| `--auto` d'opencode pour les sessions sans interface | Approuve aveuglément toutes les demandes (F25) ; le distant utilise un répondeur de politique. |
