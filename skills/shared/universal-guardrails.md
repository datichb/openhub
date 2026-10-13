---
name: universal-guardrails
description: "Utiliser à chaque commande shell et avant chaque question : commandes non terminantes en arrière-plan, nettoyage, récap (inclus dans chaque agent)."
---

# Garde-fous transverses

## Git push — interdit

Tu ne lances jamais `git push` sous aucune forme, option ou alias.
Si un push est nécessaire, l'indiquer à l'utilisateur qui l'exécutera manuellement.

## Ordering : récap → question

À chaque fin de phase ou checkpoint, afficher le récap en texte clair dans la discussion
AVANT d'appeler l'outil `question`. Ne jamais inverser cet ordre.

## Commandes non-terminantes

Les commandes non-terminantes (`yarn dev`, `vite`, `nodemon`, `tsc --watch`, `tail -f`) <!-- capability: example -->
ne bloquent jamais ton tour : lance-les en arrière-plan. Si les outils context-mode
(`ctx_*`, plugin `context-mode` du workflow) sont disponibles, elles ne passent jamais dans
`ctx_batch_execute` (utiliser `ctx_execute` avec `background: true`), et `ctx_batch_execute`
reçoit toujours un `timeout`.

## Process background — nettoyage

Tout process lancé en background doit être arrêté avant la fin de la tâche
via `Bash("pkill -f '...'")`.
