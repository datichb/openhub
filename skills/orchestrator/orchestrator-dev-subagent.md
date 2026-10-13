---
name: orchestrator-dev-subagent
description: "Utiliser quand orchestrator-dev est invoqué via task : clarifications par bloc question, bloc de retour final obligatoire."
when: role=subagent
---

# Skill — Parcours Orchestrator-dev Sous-agent

> Partie du sous-agent : tu es invoqué via `task` et l'utilisateur ne voit pas l'outil `question`.

## Principe fondamental

Quand l'orchestrator-dev est invoqué via `task`, sa todo list est dans une session **isolée et non visible** par l'utilisateur. L'orchestrator feature est le seul responsable de la liste visible.

**Les checkpoints de ticket passent par `workflow_checkpoint`, appelé par toi-même** : l'outil fonctionne depuis une sous-session, oh rattache l'appel à la session de l'utilisateur et la validation se fait depuis oh ou l'interface, comme pour l'agent d'entrée. Ne relaie jamais un checkpoint à l'orchestrator, ne t'arrête pas pour lui : appelle l'outil et attends sa réponse. N'appelle que les checkpoints de la table des checkpoints de la carte du workflow.

**Confirmer le contexte au démarrage :**
> `[orchestrator-dev] Invoqué depuis l'agent orchestrator feature. Mode de workflow reçu : <valeur canonique>. Checkpoints de ticket par workflow_checkpoint ; bloc ## Retour vers orchestrator en fin de session.`

---

## Démarrage

Le mode de workflow est transmis dans le prompt (`Mode de workflow : <mode>`) — ne pas le redemander. Sans cette ligne, appliquer le mode par défaut de la carte du workflow.

Afficher le récapitulatif des tickets reçus et démarrer directement. Si le prompt contient `Branche de travail : <branche>`, la session a déjà sa branche : ne pas en proposer une autre.

---

## Checkpoints de ticket

Pour chaque ticket, dans l'ordre reçu :

1. `cp-1` (démarrer le ticket) : `workflow_checkpoint({ id: "cp-1", summary: "Ticket #<ID> — <titre>. Agent prévu : developer (<domaine>)." })`.
2. Implémentation par le developer, puis review (skill `orchestrator-dev-ticket-workflow`).
3. `cp-2` (commit ou correction) : `workflow_checkpoint({ id: "cp-2", summary: "Ticket #<ID> — verdict : <commit | corriger | corriger-sécurité>. <synthèse>. Validation = commit puis fermeture." })`. **Un appel par ticket.** Validé → relancer aussitôt le developer : commit, puis fermeture du ticket. Refusé → correction selon la consigne reçue, nouvelle review, puis rappeler `cp-2`.
4. `cp-3` (ticket suivant) : `workflow_checkpoint({ id: "cp-3", summary: "Ticket #<ID> terminé. Suivant : #<ID-suivant>." })`.

---

## Bloc `## Question pour l'orchestrator` — vraies clarifications seulement

Seulement quand une information manque et qu'aucun checkpoint ne la couvre : dépendance non résolue, ticket bloqué, ping-pong reviewer, agent requis absent du workflow (le plafond `limits.max_fix_rounds` passe par le checkpoint de commit). Format : skill `orchestrator-handoff-format`. Produire ce bloc suivi de `## Retour vers orchestrator` (`**Type de récap :** partiel`) ; l'orchestrator relaie la question à l'utilisateur et te ré-invoque avec `task_id` et la réponse.

> ❌ Ce bloc ne pose jamais un checkpoint : `cp-1`, `cp-2`, `cp-3` passent par `workflow_checkpoint`.

### Reprise (ré-invoqué avec task_id)

Quand le prompt de reprise contient `Réponse de l'utilisateur :` : ne pas reposer la question, appliquer la réponse et continuer jusqu'à la fin.

---

## Récap global final

En fin de session (tous les tickets traités ou arrêt demandé), produire **uniquement** le bloc `## Retour vers orchestrator` avec `**Type de récap :** final`.

**Format de sortie :** aucun texte libre (récap narratif, introduction, résumé) avant ou après le bloc. Le bloc contient les sections `### Contexte et décisions par ticket` et `### Points d'attention globaux` qui encodent toutes les informations nécessaires.

> ⚠️ Produire **TOUJOURS** le bloc `## Retour vers orchestrator` — sans exception, même en cas de stop, de ticket bloqué ou de session incomplète.
> ❌ Ne jamais produire de récap global en texte libre — tout est dans le bloc structuré.
