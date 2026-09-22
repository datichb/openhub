---
name: retranscription-coordinateur
description: Protocole de retransmission des retours de sous-agents par les agents coordinateurs (orchestrator, auditor, orchestrator-dev). Les sous-agents produisent uniquement des blocs structurés. Ce skill définit comment les coordinateurs retranscrivent ces blocs de manière formatée à l'utilisateur.
---

# Skill — Protocole de retransmission (Coordinateurs)

## Distinction : Producteur vs Consommateur

Ce skill s'adresse aux **agents coordinateurs** (orchestrator, auditor, orchestrator-dev).

| Perspective | Agent concerné | Skill de référence | Responsabilité |
|-------------|---------------|-------------------|----------------|
| **Producteur** | planner, auditor-*, designer, developer-*, reviewer, debugger, onboarder | `*-handoff-format` de chaque agent | **Produire** le bloc de handoff structuré (seul output) |
| **Consommateur** | orchestrator, auditor (coordinateur), orchestrator-dev | **Ce skill** (`retranscription-coordinateur`) | **Retranscrire** les champs du bloc reçu de manière formatée avant d'appeler `question` |

**Flux complet :**

1. Orchestrator invoque planner via `task`
2. **Planner (producteur)** : suit `planner-handoff-format` → produit uniquement le bloc `## Retour vers orchestrator`
3. **Orchestrator (consommateur)** : suit `retranscription-coordinateur` → reçoit le bloc → **retranscrit les champs de manière formatée** à l'utilisateur avant d'appeler `question`

---

## Règle fondamentale (perspective consommateur)

Quand tu invoques un sous-agent via `task`, tu DOIS retranscrire les champs de son bloc de retour à l'utilisateur **AVANT** de poser toute question.

**Séquence obligatoire — sans exception :**

1. Recevoir le retour du sous-agent (= le bloc structuré)
2. **Si le retour contient un `## Retour intermédiaire vers orchestrator`** → l'afficher en texte dans la discussion, en premier
3. **Afficher les champs du bloc structuré de manière formatée** dans la discussion
4. **Puis seulement** appeler l'outil `question`

> ❌ Ne jamais omettre le bloc structuré
> ❌ Ne jamais sauter les blocs intermédiaires s'ils sont présents

---

## Format de retranscription

### Template standard (retour final)

Utiliser ce template après chaque réception de retour final :

```
**[Retranscription du retour <agent>]**

---

### Statut : `<valeur du champ Statut du bloc>`

<Pour chaque section du bloc structuré, l'afficher dans l'ordre avec son titre et son contenu tel quel. Ne pas résumer, ne pas reformuler.>

<Section 1 du bloc — copier le titre et le contenu>

<Section 2 du bloc — copier le titre et le contenu>

<...>

---

**[Fin de retranscription]**
```

**Règle de retranscription :** chaque section du bloc (identifiable par un `###` dans le bloc) est affichée telle quelle. Les tableaux, listes et champs structurés sont copiés sans modification.

### Template pour une question montante (agents avec `## Question pour l'orchestrator`)

Quand un sous-agent termine sa session avec `## Question pour l'orchestrator` :

```
**[Retranscription — question montante <agent>]**

---

### Bloc intermédiaire (si présent)

<Copier intégralement le `## Retour intermédiaire vers orchestrator` s'il est présent>

---

### État de la session

<Copier le champ `### État de la session` du bloc question>

---

**[Fin de retranscription]**

**Vérification :**
- ✅ Bloc intermédiaire affiché (si présent)
- ✅ task_id noté pour la ré-invocation : <task_id>

**Maintenant seulement,** utiliser l'outil `question` pour relayer la question à l'utilisateur.
```

### Cas spécial : CP-2 avec rapport de review

Pour un CP-2, le bloc `## Question pour l'orchestrator` contient un `### Rapport de review complet`. Ce rapport DOIT être affiché dans la discussion AVANT de poser la question :

```
**[Retranscription — CP-2 Ticket #<ID>]**

---

### Contexte

<Copier le `### Contexte complet` du bloc question>

---

### Rapport de review

<Copier intégralement le `### Rapport de review complet` du bloc question — JAMAIS résumer>

---

### État de la session

<Copier le `### État de la session`>

---

**[Fin de retranscription]**

→ **Maintenant seulement**, appeler `question`.
```

---

## Vérification obligatoire avant question

Avant d'appeler `question`, vérifier :

- ✅ Les blocs `## Retour intermédiaire vers orchestrator` sont affichés (si présents)
- ✅ Les champs du bloc structuré sont affichés dans la discussion (retranscription formatée)
- ✅ Les sections critiques sont présentes (voir tableau ci-dessous)
- ✅ Le contenu est affiché AVANT cet appel à `question`, PAS après
- ✅ Le contenu n'est PAS inclus dans le champ `question` de l'outil

### ✅ Checklist visuelle — AVANT CHAQUE APPEL À `question`

**STOP — Vérifier MAINTENANT :**

| Vérification | Fait ? |
|--------------|--------|
| ✅ Les blocs `## Retour intermédiaire vers orchestrator` sont affichés en texte (si présents) | ⬜ |
| ✅ J'ai affiché les champs du bloc structuré de manière formatée (retranscription mécanique, non résumée) | ⬜ |
| ✅ Les sections critiques de ce type de retour sont présentes (voir tableau "Sections critiques par agent") | ⬜ |
| ✅ Le contenu est affiché AVANT cet appel à `question`, PAS après | ⬜ |
| ✅ Le contenu n'est PAS inclus dans le champ `question` de l'outil | ⬜ |

**Si une seule case est ⬜ (non cochée) → ARRÊTER et afficher le contenu manquant MAINTENANT.**

---

## Sections critiques par agent source

| Agent source | Sections critiques à vérifier dans le bloc |
|--------------|---------------------------------------------|
| **planner** (final) | `### Récapitulatif de planification`, `### Tickets créés`, `### Hypothèses et ambiguïtés`, `### Risques identifiés`, `### Ordre de traitement` |
| **pathfinder** (final) | `### Rapport pathfinder complet`, `### Recommandation` |
| **onboarder** (final) | `### Rapport d'onboarding`, `### Zones d'incertitude`, `### Dette technique détectée` |
| **auditor** (final) | `### Rapport d'audit complet`, `### Synthèse des problèmes identifiés`, `### Risque résiduel si non corrigé` |
| **debugger** (final) | `### Rapport de diagnostic complet`, `### Actions d'urgence si bug en prod`, `### Impact et régressions potentielles` |
| **designer** (final) | `### Spec complète`, `### Contraintes d'implémentation`, `### Points ouverts` |
| **orchestrator-dev** (final) | `### Détail par ticket`, `### Contexte et décisions par ticket`, `### Points d'attention globaux` |
| **reviewer** | `### Rapport complet`, `### Verdict`, `### Corrections requises` |

---

## Ce que tu NE fais JAMAIS

❌ Résumer ou abréger les champs du bloc avant de les afficher
❌ Inclure les champs du bloc dans le champ `question` de l'outil
❌ Omettre des sections du bloc "parce qu'elles sont vides" (afficher la mention explicite si vide)
❌ Reformuler le contenu reçu — le copier tel quel

---

## Rappel d'application

- ❌ Appeler `question` sans avoir affiché le bloc structuré = l'utilisateur décide sans contexte.
- ✅ Toujours : retranscrire le bloc complet (template standard ci-dessus) → puis seulement `question`.

---

## Injection de ce skill

Ce skill doit être injecté dans les 3 agents coordinateurs :

| Agent | Fichier | Position recommandée |
|-------|---------|---------------------|
| **orchestrator** | `agents/planning/orchestrator.md` | Après `posture/coordination-only` |
| **orchestrator-dev** | `agents/planning/orchestrator-dev.md` | Après `posture/coordination-only` |
| **auditor** | `agents/auditor/auditor.md` | Après `posture/coordination-only` |

---

## Relation avec les autres skills

| Skill | Scope | Complémentarité |
|-------|-------|-----------------|
| **coordination-only** | Définit la posture "ne jamais faire le travail soi-même" | ✅ Complémentaire — retranscription-coordinateur définit **comment afficher** ce que les sous-agents ont produit |
| **tool-question** | Définit **comment utiliser** l'outil `question` | ✅ Complémentaire — retranscription-coordinateur définit **quoi afficher avant** d'utiliser `question` |
| **subagent-concision-posture** | Définit ce que les sous-agents **ne produisent pas** (tout hors bloc) | ✅ Complémentaire — garantit que le retour reçu EST le bloc structuré |
| **`*-handoff-format`** | Définit le format du bloc produit par chaque agent | ✅ Complémentaire — retranscription-coordinateur définit comment ce bloc est affiché à l'utilisateur |
