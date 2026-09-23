---
name: error-recovery-protocol
description: "Protocole de retry/recovery pour les échecs de subagents — classification des erreurs, budget de retry, fallbacks systémiques."
---

# Protocole de retry/recovery

Ce protocole définit le comportement du coordinateur (`orchestrator-dev`) face aux échecs de subagents. Il couvre les erreurs transientes (retryables), permanentes (escalation immédiate), et systémiques (fallback).

## Classification des échecs

| Type | Signal | Action | Max retries |
|------|--------|--------|-------------|
| **Transient** | Handoff absent ou malformé (champs manquants, statut invalide) | Re-invoquer le subagent avec `task_id` + instruction corrective | **2** |
| **Permanent** | Statut `bloqué`, `BLOCKED_ARCHITECTURE`, dépendance non résolue | Escalation immédiate à l'utilisateur | 0 |
| **Systémique** | Context window exhaustion, erreur `task` tool, session perdue | Fallback spécifique (voir ci-dessous) | 1 (nouvelle session) |

## Budget de retry par type de subagent

| Subagent | Handoff absent/malformé | Boucle pre-review -> developer | Cycles review (existant) |
|----------|------------------------|-------------------------------|-------------------------|
| developer | 2 retries | **5 cycles max** | 3 cycles (inchangé) |
| reviewer | 2 retries | N/A | N/A |
| documentarian | 2 retries | N/A | N/A |

Après épuisement du budget de retry, escalader **systématiquement** à l'utilisateur. Ne jamais retenter au-delà du budget.

## Protocole par failure mode

### Handoff absent (pas de bloc `## Retour vers orchestrator-dev`)

1. **Retry 1** : ré-invoquer avec `task_id` + instruction : `"Le handoff structuré est absent. Produire le bloc ## Retour vers orchestrator-dev avec tous les champs obligatoires (voir skill *-handoff-format)."`
2. **Retry 2** : ré-invoquer avec `task_id` + instruction renforcée : `"IMPORTANT : ton seul output doit être le bloc ## Retour vers orchestrator-dev. Aucun texte avant ou après."`
3. **Après 2 échecs** : escalader à l'utilisateur avec le dernier output brut du subagent + options : `Reprendre manuellement` / `Passer ce ticket` / `Stop`

### Handoff malformé (champs manquants ou invalides)

1. **Retry 1** : ré-invoquer avec `task_id` + instruction listant les champs manquants : `"Les champs suivants sont absents ou invalides : <liste>. Compléter le bloc ## Retour vers orchestrator-dev."`
2. **Retry 2** : même instruction renforcée
3. **Après 2 échecs** : escalader à l'utilisateur

### Context window exhaustion / output tronqué

**Signal de détection :** le subagent retourne un résultat qui contient des indicateurs de progression (fichiers modifiés, tests écrits, messages) mais **pas** de bloc `## Retour vers orchestrator-dev` en fin de output, ET la longueur du output est inhabituellement longue.

1. **Sauvegarder** le résultat partiel — extraire ce qui est exploitable (fichiers mentionnés, statut Beads apparent)
2. **Ré-invoquer dans une NOUVELLE session** (pas `task_id` — la session est saturée) avec un prompt réduit :
   - Ticket ID + description courte
   - Liste des fichiers déjà modifiés (extraite du résultat partiel)
   - Instruction : `"Compléter l'implémentation commencée et produire le bloc ## Retour vers orchestrator-dev. Fichiers déjà modifiés : <liste>."`
3. **Si le 2ème essai échoue aussi** : escalader à l'utilisateur avec le résultat partiel
   - Options : `Reprendre manuellement` / `Passer ce ticket` / `Stop`

### Erreur du `task` tool (agent introuvable, permission denied, erreur runtime)

1. **Retry 1** : réessayer l'invocation identique (erreurs réseau, race conditions)
2. **Si 2ème échec** : escalader à l'utilisateur avec le message d'erreur
   - Options : `Réessayer` / `Passer ce ticket` / `Stop`

### Boucle pre-review infinie

Après **5 cycles** pre-review -> retour developer sans résolution des erreurs (lint, types, tests) :

1. Escalader à l'utilisateur : `"Le developer n'arrive pas à résoudre les erreurs de pre-review après 5 tentatives."`
2. Afficher les erreurs récurrentes (extraites du dernier résultat pre-review)
3. Options : `Intervenir manuellement` / `Passer à la review malgré les erreurs` / `Passer ce ticket` / `Stop`

### Session perdue (`task_id` invalide)

> Déjà géré dans `orchestrator-dev-edge-cases` (Cas C). Ce protocole ajoute une nuance :

Avant d'escalader, **tenter UNE ré-invocation** avec `task_id` (erreur transitoire possible). Si la 2ème tentative échoue aussi, appliquer le protocole Cas C existant (relance sans `task_id` ou stop).

### Worktree creation failure (mode parallèle)

| Erreur | Action |
|--------|--------|
| Branche déjà existante | `git worktree remove <path>` + `git branch -D <branch>` puis réessayer |
| Verrou git (`.git/index.lock`) | Attendre 5 secondes, réessayer (max 3 tentatives) |
| Erreur persistante après 3 tentatives | **Fallback séquentiel** pour ce ticket : le retirer du lot parallèle et le traiter en séquentiel après les sessions parallèles restantes |

### Conflit de fichiers entre sessions parallèles

> La détection existe déjà dans `orchestrator-dev-parallel`. Ce protocole ajoute le **traitement** :

1. **Identifier la session prioritaire** : celle qui a le ticket de plus haute priorité (ou le premier lancé)
2. **Mettre en pause la session secondaire** (attendre qu'elle atteigne un CP naturel)
3. **Lancer la review de la session prioritaire en premier**
4. **Après merge de la prioritaire** : ré-invoquer la session secondaire dans un nouveau worktree basé sur la branche mise à jour
5. Si les conflits persistent après resolution : escalader à l'utilisateur

## Règle absolue

> ❌ Ne JAMAIS retenter plus que le budget défini. Après épuisement des retries, l'escalation à l'utilisateur est **obligatoire**. Le coordinateur ne doit jamais boucler indéfiniment sur un échec.
