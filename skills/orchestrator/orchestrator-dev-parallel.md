---
name: orchestrator-dev-parallel
description: "Utiliser quand orchestrator-dev reçoit 2 tickets ou plus : 4 critères d'éligibilité au parallélisme, puis lancement, un cp-2 et un commit par ticket."
---

## Éligibilité — évaluée avant le premier ticket, quel que soit le mode

**Les 4 critères — tous doivent être vérifiés :**

1. **Pas de dépendance formelle entre tickets du lot** : pour chaque ticket, `bd dep list <ID>` — l'intersection avec les IDs du lot est vide
2. **Domaines disjoints** : tous les tickets sont routés vers des domaines différents de l'agent `developer`, pas de domaine `fullstack` dans le lot
3. **Pas de fichiers transverses prévisibles** : aucune mention de types partagés, migrations de base de données, ou fichiers de configuration globaux dans les descriptions
4. **Budget du lot parallèle ≤ MaxSessionBudget** : estimation de chaque ticket (`bd show <ID> --json` → `estimated_minutes`) convertie en poids — XS (< 30 min) ou S (30-60 min) : 1 pt ; M (60-120 min) : 2 pts ; L (120-240 min) : 3 pts ; XL (> 240 min) : 4 pts ; inconnu : 2 pts. La somme ne doit pas dépasser le budget configuré (défaut : **6 points**) ; le nombre de sessions ne dépasse jamais le plafond technique (défaut : **5**, maximum : **10**).

Annonce le résultat :

```
▶️ [Parallélisme conditionnel] <NB_TICKETS> tickets éligibles — lancement simultané.
Critères vérifiés : (1) dépendances — aucune ✅ (2) agents — disjoints ✅ (3) fichiers — non transverses ✅ (4) budget — <TOTAL_PTS>/<MAX_BUDGET> pts ✅ (<NB_TICKETS> sessions)
```

ou `▶️ [Parallélisme conditionnel] Non éligible — traitement séquentiel. Raison : <critère non vérifié>`, puis le traitement séquentiel normal, ticket par ticket.

<!-- oh:when role=entry -->
**Agent d'entrée, lot éligible :** en mode `manuel` ou `semi-auto`, propose le parallélisme avant de lancer (confirmation hors checkpoint) :

```
question({
  questions: [{
    header: "Parallélisme conditionnel",
    question: "<NB_TICKETS> tickets éligibles au parallélisme (domaines disjoints, aucune dépendance, pas de fichiers transverses).\n\nLancer les tickets en parallèle ?",
    options: [
      { label: "Oui — parallèle", description: "Lancer <NB_TICKETS> sessions simultanées. Chaque ticket garde sa propre validation avant commit." },
      { label: "Non — séquentiel", description: "Traiter les tickets un par un dans l'ordre de priorité" }
    ]
  }]
})
```

Accepté → workflow parallèle ci-dessous ; refusé → séquentiel. **En mode `auto`** : ne pas poser la question, lancer directement en parallèle.
<!-- /oh:when -->
<!-- oh:when role=subagent -->
**Sous-agent** : pas de question ; traitement séquentiel, sauf si ton prompt d'invocation demande le parallèle.
<!-- /oh:when -->

## Workflow parallèle

Ce workflow s'applique quand les 4 critères sont vérifiés et le parallélisme retenu, **quel que soit le mode de workflow** (manuel, semi-auto, auto).

> **Arborescence partagée** : les sessions parallèles travaillent dans le dossier de la session (un worktree, s'il y en a un, est choisi au lancement par oh). Ne crée pas de worktree. Les critères 2 et 3 (domaines disjoints, pas de fichiers transverses) sont la seule protection contre les débordements : chaque developer ne touche que les fichiers de son ticket.

### Lancement simultané

1. `cp-1` de chaque ticket (`workflow_checkpoint`, un appel par ticket, dans l'ordre de traitement).
2. Invoquer N sessions `developer` dans le même appel — chacune reçoit son ticket, son contexte, l'instruction TDD si applicable, et la consigne : « Ne touche que les fichiers de ce ticket ; ne commite pas. » Le nombre de sessions est limité par le budget de points et le plafond technique (critère 4 ci-dessus).

### Attente et agrégation des résultats

Attendre les résultats de toutes les sessions. Pour chaque résultat reçu :

1. Vérifier la présence de l'enveloppe du developer et de son rapport (`rp-…` ou inclus)
2. Si le statut est `bloqué` → traiter comme un "Ticket bloqué" (clarification : outil `question` en agent d'entrée, `### Questions bloquantes` de ton enveloppe en sous-agent)
3. Détecter un éventuel conflit de fichiers : si un `developer` a modifié un fichier déjà modifié par une autre session parallèle, le signaler et traiter ce ticket en premier
4. **Si un résultat est absent ou tronqué** (pas d'enveloppe, indicateurs de progression mais pas de clôture) → appliquer le protocole de retry/recovery (voir skill `orchestrator/error-recovery-protocol`). Si le retry échoue, traiter ce ticket en **fallback séquentiel** après les sessions parallèles restantes — ne pas bloquer les autres sessions.

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
