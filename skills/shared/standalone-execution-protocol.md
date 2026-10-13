---
name: standalone-execution-protocol
description: "Utiliser quand tu es l'agent d'entrée d'un agent à phases : récap puis question, checklist commune."
---

# Protocole d'exécution — mode standalone

## Détection du mode d'invocation

- Section « Rôle dans cette session » de ton prompt (générée par oh) : sous-agent → **MODE SUBAGENT** (voir skill `shared/subagent-execution-protocol`)
- Agent d'entrée, ou lancé directement par l'utilisateur → **MODE STANDALONE** (ce protocole)

## Principe fondamental

En mode standalone, le texte de chaque phase est **directement visible** par l'utilisateur. La communication se fait via :
1. Le texte de réponse (récap complet de la phase)
2. L'outil `question` pour les validations et décisions

## Ordering — récap → question

**À CHAQUE fin de phase :**

1. **TOUJOURS** produire le récap complet en texte **AVANT** d'appeler l'outil `question`
2. **PUIS** appeler l'outil `question` pour la validation
3. **JAMAIS** l'inverse — l'utilisateur doit voir le récap avant de décider

## Checklist de vérification (avant chaque appel `question`)

| Vérification | |
|---|---|
| J'ai affiché le récap complet de la phase actuelle en texte | ⬜ |
| Le récap contient toutes les observations, découvertes et décisions | ⬜ |
| Le récap n'est PAS résumé — il est complet et détaillé | ⬜ |
| Le récap est affiché AVANT cet appel à `question`, PAS après | ⬜ |

## Format final (standalone)

En mode standalone, la dernière phase produit le rapport/livrable directement en texte dans la discussion. Pas de bloc `## Retour vers orchestrator`.
