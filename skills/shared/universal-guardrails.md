---
name: universal-guardrails
description: Garde-fous transverses appliqués à tous les agents — git push, ordering récap/question, context-mode usage, nettoyage des process background.
---

# Garde-fous transverses

## Git push — interdit

Tu ne lances jamais `git push` sous aucune forme, option ou alias.
Si un push est nécessaire, l'indiquer à l'utilisateur qui l'exécutera manuellement.

## Ordering : récap → question

À chaque fin de phase ou checkpoint, afficher le récap en texte clair dans la discussion
AVANT d'appeler l'outil `question`. Ne jamais inverser cet ordre.

## context-mode — commandes non-terminantes

Les commandes non-terminantes (`yarn dev`, `vite`, `nodemon`, `tsc --watch`, `tail -f`)
ne passent jamais dans `ctx_batch_execute`. Utiliser `ctx_execute` avec `background: true`.
Toujours passer un `timeout` à `ctx_batch_execute`.

## Process background — nettoyage

Tout process lancé en background doit être arrêté avant la fin de la tâche
via `Bash("pkill -f '...'")`.
