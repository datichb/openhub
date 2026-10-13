---
name: orchestrator-dev-parallel
description: "Utiliser quand les 4 critères de parallélisme sont remplis et acceptés : tickets en parallèle, un cp-2 et un commit par ticket."
---

## Workflow parallèle

Ce workflow s'applique quand les 4 critères de parallélisabilité sont vérifiés, **quel que soit le mode de workflow** (manuel, semi-auto, auto).

> **Arborescence partagée** : les sessions parallèles travaillent dans le dossier de la session (un worktree, s'il y en a un, est choisi au lancement par oh). Ne crée pas de worktree. Les critères 2 et 3 (domaines disjoints, pas de fichiers transverses) sont la seule protection contre les débordements : chaque developer ne touche que les fichiers de son ticket.

### Lancement simultané

1. `cp-1` de chaque ticket (`workflow_checkpoint`, un appel par ticket, dans l'ordre de traitement).
2. Invoquer N sessions `developer` dans le même appel — chacune reçoit son ticket, son contexte, l'instruction TDD si applicable, et la consigne : « Ne touche que les fichiers de ce ticket ; ne commite pas. » Le nombre de sessions est limité par le budget de points et le plafond technique (voir critère 4 dans `orchestrator-dev-protocol`).

### Attente et agrégation des résultats

Attendre les résultats de toutes les sessions. Pour chaque résultat reçu :

1. Vérifier la présence du compte rendu d'implémentation + bloc `## Retour vers orchestrator-dev`
2. Si `### Statut` = `bloqué` → traiter comme un "Ticket bloqué" (clarification : outil `question` en agent d'entrée, bloc `## Question pour l'orchestrator` en sous-agent)
3. Détecter un éventuel conflit de fichiers : si un `developer` a modifié un fichier déjà modifié par une autre session parallèle, le signaler et traiter ce ticket en premier
4. **Si un résultat est absent ou tronqué** (pas de bloc de handoff, indicateurs de progression mais pas de clôture) → appliquer le protocole de retry/recovery (voir skill `orchestrator/error-recovery-protocol`). Si le retry échoue, traiter ce ticket en **fallback séquentiel** après les sessions parallèles restantes — ne pas bloquer les autres sessions.

### Pre-review et Review en parallèle

Les phases Pre-review et Review sont lancées pour chaque ticket dès que son implémentation est terminée — sans attendre les autres sessions.

### `cp-2` et commit — un ticket à la fois

Il n'y a pas de validation groupée : **chaque ticket a son propre `cp-2`**, dans l'ordre d'arrivée des reviews.

Pour chaque ticket, l'un après l'autre :
1. Afficher son rapport de review complet.
2. `workflow_checkpoint({ id: "cp-2", summary: "Ticket #<ID> — verdict : <…>. Validation = commit puis fermeture." })`.
3. Validé → relancer le developer de ce ticket (même `task_id`) :
   > « Crée le commit final de ce ticket et clos-le :
   > 1. `git add <fichiers de ce ticket>` puis `git commit -m "<type>(<scope>): <description> (<ID>)"` — seulement les fichiers de ce ticket
   > 2. `bd close <ID> --reason "Implemented in commit <hash>" --suggest-next` »
   Refusé → correction de ce ticket (retours du reviewer + consigne reçue), nouvelle review, puis `cp-2` de nouveau.
4. La fermeture du ticket referme la fenêtre de commit : passer au ticket suivant, qui demande son propre `cp-2`.

### Récap global — synchronisation finale

Le récap global est produit uniquement quand **tous** les tickets du lot sont commités, ignorés ou bloqués.
