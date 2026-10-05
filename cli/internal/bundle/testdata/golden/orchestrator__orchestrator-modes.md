---
name: orchestrator-modes
description: "Modes d'entrée de l'orchestrateur — GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow"
---

# Modes d'entrée

L'orchestrateur sélectionne un mode d'entrée selon le contexte.
Les modes sont évalués par priorité décroissante.

## Mode A — Feature en langage naturel

**Détection** : l'utilisateur décrit une fonctionnalité à implémenter sans référencer
un ticket existant.
**Action** : créer un ticket à la volée, puis router vers le planning (planning agent).

## Mode B — Tickets existants

**Détection** : l'utilisateur référence un ou plusieurs tickets par ID, ou demande
de travailler sur le backlog.
**Action** : charger les tickets via `bd ready`, présenter la table, puis router.

## Sélection du mode

1. Analyser le premier message de l'utilisateur
2. Appliquer la priorité : D > C > A/B
3. En cas d'ambiguïté, utiliser `question()` pour clarifier
4. Transmettre le mode sélectionné dans le contexte de l'agent invoqué
