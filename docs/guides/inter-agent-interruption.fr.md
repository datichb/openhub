---
title: Mécanisme d'interruption de session inter-agents
description: Guide de référence pour le mécanisme qui permet aux agents invoqués via task de remonter des récaps intermédiaires et des questions à l'agent parent, en contournant la limitation technique du tool task qui ne retourne que le dernier message.
---

# Mécanisme d'interruption de session inter-agents

## Pourquoi ce mécanisme

Quand un agent est invoqué via l'outil `task`, seul le **dernier message texte** de la session enfant est retourné à l'agent parent. Tout ce que l'agent affiche en cours de session (récaps de phases, contextes de pause, résultats intermédiaires) est invisible pour l'agent parent et pour l'utilisateur dans la session parent.

Le problème se pose particulièrement pour les agents avec un workflow phasé (planner, onboarder, auditor, debugger) ou avec des checkpoints de validation (orchestrator-dev) : l'utilisateur ne voit rien jusqu'à la fin complète de la session enfant, qui peut durer plusieurs minutes.

## Principe de la solution

Au lieu d'utiliser l'outil `question` (qui pause la session enfant mais reste invisible pour l'agent parent), les agents en mode `orchestrator_feature` :

1. Produisent un bloc `## Retour intermédiaire vers orchestrator` contenant le récap de la phase ou le contexte courant
2. Produisent un bloc `## Question pour l'orchestrator` contenant la question, les options et le `task_id`
3. **Terminent leur session**

L'orchestrateur parent :
1. Reçoit ces blocs dans le message final de la session enfant
2. Affiche le bloc intermédiaire en texte dans la discussion
3. Relaie la question à l'utilisateur via l'outil `question`
4. Re-invoque l'agent avec `task_id` + la réponse — l'agent recharge son historique complet et continue

## Agents implémentant ce mécanisme

| Agent | Granularité | Type d'interruption |
|-------|-------------|---------------------|
| **orchestrator-dev** | CPs à enjeu fort (CP-2, blocage, ticket bloqué) + CPs intermédiaires (CP-1, CP-QA, CP-3, branche) | Systématique et ad hoc |
| **planner** | Fin de chaque phase (0 à 5) + pauses ad hoc | Systématique |
| **pathfinder** | Clarification critique détectée | Ad hoc uniquement |
| **onboarder** | Fin de chaque phase (0 à 4) + pauses ad hoc | Systématique |
| **auditor** (coordinateur) | Fin de chaque phase (0 à 3) + pauses ad hoc | Systématique |
| **debugger** | Fin de chaque phase + confirmations d'actions irréversibles | Systématique |
| **designer** | Clarification critique (design system, informations utilisateur insuffisantes, mode non précisé) | Ad hoc uniquement |

## Format des blocs

### Bloc `## Retour intermédiaire vers orchestrator`

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** <nom de l'agent>
**Phase :** X — <titre de la phase>
**task_id :** <sessionID courant>

**Résumé :** <2-3 phrases sur ce qui a été fait dans cette phase>
**Points clés :** <liste courte — découvertes importantes, décisions, blocages>
```

### Bloc `## Question pour l'orchestrator`

```markdown
## Question pour l'orchestrator

**Phase :** X
**task_id :** <sessionID courant>

**Contexte :** <explication de pourquoi cette question>

**Question :** <texte exact de la question>

**Options :**
- `<label-option-a>` — <description>
- `<label-option-b>` — <description>

**Instruction de reprise :** "Réponse Phase X <agent> : [option]. Reprendre depuis <point d'interruption>."
```

### Variante orchestrator-dev : `## Question pour l'orchestrator` (sans accent)

orchestrator-dev utilise la variante sans accent. Les deux blocs sont sémantiquement équivalents. L'orchestrateur doit détecter les deux.

## Détection du contexte d'invocation

Les agents détectent leur contexte d'invocation via le marqueur dans le prompt :

```
[CONTEXTE] Invoqué depuis l'orchestrateur feature. Tu dois utiliser le mécanisme d'interruption de session...
```

Ce marqueur est injecté par l'agent orchestrator dans chaque invocation `task(...)`.

### Comportement selon le contexte

| Contexte | Appel `question` | Blocs structurés | Terminaison de session |
|----------|-----------------|-----------------|----------------------|
| **standalone** | Utilisé normalement | Non produits | Session reste ouverte |
| **orchestrator_feature** | Interdit | Produits à chaque checkpoint | Obligatoire après les blocs |

## Reprise de session avec task_id

Le `task_id` est l'ID de session OpenCode — une session persistante avec son historique de messages complet.

### Flux de reprise

```
1. Agent (ex: planner Phase 1) produit les blocs + termine
   → task_result contient ## Retour intermédiaire + ## Question pour l'orchestrator (task_id: "sess_abc")

2. Orchestrateur affiche le récap intermédiaire en texte

3. Orchestrateur pose la question via question() → l'utilisateur répond

4. Orchestrateur ré-invoque :
   task(
     subagent_type: "planner",
     task_id: "sess_abc",
     prompt: "Réponse Phase 1 planner : phase-2. Reprendre depuis Phase 2."
   )

5. Le planner recharge l'historique complet de "sess_abc"
   → Reçoit le nouveau message avec la réponse
   → Continue depuis Phase 2 avec tout le contexte précédent
```

### Risque : session introuvable

Si OpenCode redémarre entre la question montante et la ré-invocation, la session peut ne plus exister.

L'orchestrateur doit gérer ce cas : si la ré-invocation avec `task_id` ne produit pas de résultat cohérent, proposer à l'utilisateur de relancer depuis le début.

## Implémenter ce mécanisme dans un nouvel agent

### 1. Ajouter la détection du contexte

Dans le skill workflow de l'agent, ajouter en début de fichier :

```markdown
### Détection du contexte d'invocation

Au démarrage, détecter si le prompt contient `[CONTEXTE] Invoqué depuis l'orchestrateur feature`. Si oui :
- Mémoriser **CONTEXTE = orchestrator_feature**
- Confirmer : `[<agent>] Contexte détecté : mode interruption actif.`
```

### 2. Ajouter le format des blocs

Définir le format des blocs pour chaque checkpoint :

```markdown
### Format de retour — RÈGLE ABSOLUE (orchestrator_feature)

À CHAQUE checkpoint :
1. Produire le récap en texte
2. Produire ## Retour intermédiaire vers orchestrator
3. Produire ## Question pour l'orchestrator
4. TERMINER LA SESSION
```

### 3. Modifier chaque appel à l'outil question

Pour chaque appel `question({...})` :

```markdown
**Si CONTEXTE = standalone :**
```
question({...})  [inchangé]
```

**Si CONTEXTE = orchestrator_feature :**
```markdown
## Retour intermédiaire vers orchestrator
...
## Question pour l'orchestrator
...
```
→ TERMINER LA SESSION
```

### 4. Ajouter dans le fichier agent

Dans `agents/<famille>/<agent>.md`, ajouter une section "Contexte d'invocation" (voir `agents/planning/pathfinder.md` pour exemple).

### 5. Mettre à jour orchestrator-protocol.md

Dans la section d'invocation de l'agent dans `skills/orchestrator/orchestrator-protocol.md` :
- Ajouter le marqueur `[CONTEXTE]` dans l'invocation
- Ajouter la section "Réception d'une question montante depuis <agent>"
- Mettre à jour les templates de retranscription

### 6. Mettre à jour retranscription-coordinateur.md

Ajouter les nouvelles lignes dans le tableau "Règles par type de retour".

## Côté orchestrateur : recevoir et retransmettre

À la réception de chaque résultat d'une invocation `task`, l'agent orchestrator doit :

1. **Détecter le type de retour** :
   - Contient `## Question pour l'orchestrator` (ou `## Question pour l'orchestrator`) → question montante
   - Contient `## Retour vers orchestrator` sans question montante → retour final

2. **Pour un retour final** :
   - Afficher les `## Retour intermédiaire vers orchestrator` en texte, dans l'ordre
   - Afficher le récap narratif complet (contexte, raisonnement, preuves — contenu unique non répété dans le bloc structuré)
   - Afficher le bloc structuré `## Retour vers orchestrator` (tableau de synthèse, métadonnées de routing, statut)
   - Puis seulement appeler `question` pour le checkpoint utilisateur

3. **Pour une question montante** :
   - Afficher le `## Retour intermédiaire vers orchestrator` en texte
   - Lire le `## Question pour l'orchestrator` — récupérer question, options, task_id
   - Relayer la question via `question()`
   - Ré-invoquer avec task_id + réponse + marqueur [CONTEXTE]
   - Répéter jusqu'au retour final

## Limites connues

- **Session introuvable** : si OpenCode redémarre, le `task_id` peut ne plus être valide. Voir "Risque : session introuvable" ci-dessus.
- **Transmission du mode** : pour orchestrator-dev, le mode de workflow (manuel/semi-auto/auto) doit être retransmis dans chaque ré-invocation via `task_id`.
- **Accumulation des blocs** : si plusieurs phases se terminent sans question (transition automatique), les blocs intermédiaires s'accumulent dans le message final — l'agent orchestrator doit tous les afficher dans l'ordre.
