---
name: subagent-execution-protocol
description: "Protocole d'exécution sub-agent — mécanisme d'interruption, checklist, erreurs fréquentes."
---

# Protocole d'exécution — mode sub-agent

## Principe fondamental

Quand invoqué via `task`, le texte de la session enfant **n'est PAS visible** par l'utilisateur dans la session parent. La seule façon de remonter du contenu est de **terminer la session** avec les blocs structurés que l'agent orchestrator retranscrira.

## Mécanisme d'interruption — à chaque fin de phase

1. Produire le récap complet de la phase en texte
2. Produire le bloc `## Retour intermédiaire vers orchestrator` (avec `task_id`, état de la session)
3. Produire le bloc `## Question pour l'orchestrator` (avec question, options, instruction de reprise)
4. **TERMINER LA SESSION** — ne pas appeler l'outil `question`, ne pas continuer

## Checklist de vérification (avant de terminer)

| Vérification | |
|---|---|
| J'ai produit le récap complet de la phase en texte | ⬜ |
| J'ai produit le bloc `## Retour intermédiaire vers orchestrator` | ⬜ |
| J'ai produit le bloc `## Question pour l'orchestrator` avec question + options + instruction de reprise | ⬜ |
| Le `task_id` est renseigné dans les deux blocs | ⬜ |
| Je vais TERMINER la session — pas appeler l'outil `question` | ⬜ |

## Message de confirmation au démarrage

Quand invoqué depuis un orchestrator, produire d'abord un message de confirmation :
> `✅ [{agent_label}] Mode sub-agent activé. Contexte reçu : {résumé_1_ligne}. Démarrage Phase {N}.`

## Erreurs fréquentes à éviter

| Erreur | Conséquence | Action correcte |
|--------|-------------|-----------------|
| Appeler l'outil `question` | Question invisible pour l'orchestrator | **Terminer** avec les blocs structurés |
| Continuer vers la phase suivante sans produire les blocs | L'orchestrator ne reçoit rien | **Toujours interrompre** à chaque fin de phase |
| Omettre le `task_id` | L'orchestrator ne peut pas reprendre | **Toujours inclure** le sessionID |
