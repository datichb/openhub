---
name: orchestrator-ticket-routing
description: "Routing des tickets par type et agent — GÉNÉRÉ AUTOMATIQUEMENT depuis le YAML du workflow"
---

# Routing des tickets

## Agents disponibles pour le routing
- `orchestrator-dev` (workflow / primary)
- `developer` (workflow / subagent)
- `reviewer` (workflow / subagent)
- `documentarian` (independent / primary)

## Table de routing par type de ticket

## Séquence de routing standard

1. L'orchestrateur évalue le scoring de complexité du ticket
2. Selon le score et le type, il délègue à l'agent approprié ci-dessus
3. L'agent de planning retourne un rapport avec les sous-tâches
4. L'orchestrateur passe à `orchestrator-dev` pour l'implémentation
