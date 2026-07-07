---
name: pathfinder-handoff-format
description: Format de rapport pathfinder et format d'escalade vers le planner — le rapport complet est intégré dans le bloc structuré. Structure exploitable par l'utilisateur, orchestrator-dev et planner.
---

# Skill — Pathfinder Handoff Format

## Rôle

Ce skill définit le format exact du rapport pathfinder et de son handoff vers le planner.

---

## Principe fondamental — bloc unique

Quand tu es invoqué depuis l'`orchestrator` (CONTEXTE = orchestrator_feature), ton **seul output** est le bloc `## Retour vers orchestrator` défini ci-dessous.

**Règle absolue :** aucun texte avant, après ou en dehors de ce bloc. Le rapport pathfinder complet est **intégré dans le bloc** (section `### Rapport pathfinder complet`), pas produit séparément en texte libre.

En standalone, le rapport est produit directement (sans le bloc `## Retour vers orchestrator`).

> **Autocontrôle obligatoire avant de terminer la session (CONTEXTE = orchestrator_feature) :**
> « Mon output contient-il du texte en dehors du bloc `## Retour vers orchestrator` ? Si oui, le supprimer et vérifier que le rapport est bien dans la section `### Rapport pathfinder complet` du bloc. »

---

## Format du bloc `## Retour vers orchestrator`

```
---

## Retour vers orchestrator

**Agent :** pathfinder
**Feature :** <nom complet de la feature>
**Complexité :** <XS|S|M|L|XL>

### Recommandation
`direct` | `escalade-planner`

**Justification :** <raison principale de la recommandation>

### Handoff planner
<Si recommandation = `escalade-planner` : la section `## 📦 Handoff vers planner` du rapport ci-dessous est complète et exploitable directement>
<Si recommandation = `direct` : "Non applicable — traitement direct recommandé">

### Rapport pathfinder complet

# 🔍 Pathfinder Report

**Feature:** <Nom court de la feature>
**Complexité:** <XS|S|M|L|XL>
**Date:** <YYYY-MM-DD HH:mm>

---

## 📝 Contexte rapide

<2-3 phrases décrivant ce qui a été compris de la demande utilisateur>

---

## 🔎 Exploration

### Fichiers/Modules clés identifiés
- `path/to/file.ts` - <raison de pertinence>
- `path/to/module/` - <raison>
- (ou "Aucun fichier spécifique identifié" si pas pertinent)

### Tickets Beads existants
- **bd-123** - <Titre> — <Relation avec cette feature>
- (ou "Aucun ticket existant directement lié")

### Patterns/Logiques réutilisables
- <Pattern X dans module Y> - peut être réutilisé pour <aspect>
- (ou "Aucun pattern directement réutilisable identifié")

---

## 🎯 Structure proposée (draft)

### Epic suggéré
**<Nom de l'epic si nécessaire>**
(ou "Pas d'epic nécessaire (ticket unique)")

### Tickets estimés (~)

#### 1. **<Titre ticket 1>** (type: feature/task, P1, ~30-60min)
- **Description courte:** <1 phrase claire>
- **Dépend de:** <bd-X ou "aucune">
- **Notes:** <remarque technique rapide si nécessaire, sinon omettre>

#### 2. **<Titre ticket 2>** (type: task, P2, ~60-120min)
- **Description courte:** <1 phrase>
- **Dépend de:** ticket 1

<...>

**Total estimé:** ~<durée> (<taille XS/S/M/L/XL>)

---

## ❓ Questions ouvertes

- [ ] **[Métier]** <Question métier si applicable>
- [ ] **[Technique]** <Question technique si applicable>
- [ ] **[Design]** <Question UX/UI si applicable>
- (ou "Aucune question critique — feature bien définie")

---

## ⚠️ Risques identifiés

- **<Niveau: Faible/Moyen/Élevé>** <Description du risque + impact potentiel>
- (ou "Aucun risque particulier identifié")

---

## 🚦 Signaux détectés

| Signal | Détecté | Détails |
|--------|---------|---------|
| **UX/UI** | ❌ / ⚠️ / ✅ | <Détails si ⚠️ ou ✅> |
| **Sécurité** | ❌ / ⚠️ / ✅ | <Détails> |
| **Performance** | ❌ / ⚠️ / ✅ | <Détails> |
| **Accessibilité** | ❌ / ⚠️ / ✅ | <Détails> |
| **Architecture** | ❌ / ⚠️ / ✅ | <Détails> |

**Légende :**
- ❌ Aucun signal détecté
- ⚠️ Signal faible (à surveiller)
- ✅ Signal fort (nécessite attention)

---

## 🎯 Recommandation

<Option A ou B ci-dessous>

### Option A : Traitement direct
✅ **Traitement direct recommandé**
**Justification :** <raisons>

### Option B : Escalade au planner
🎯 **Escalade au planner recommandée**
**Justification :** <raisons numérotées>

---

## 📦 Handoff vers planner (si escalade)

<UNIQUEMENT SI ESCALADE RECOMMANDÉE>

**Agent source:** pathfinder
**Feature:** <nom complet>
**Complexité estimée:** <L/XL>

### Contexte déjà exploré
<fichiers clés, tickets liés, patterns>

### Structure draft (à valider/affiner par le planner)
<copie de la section Structure proposée>

### Questions posées et réponses
<Q&A si applicable>

### Questions restantes (pour le planner)
<questions ouvertes>

### Signaux détectés (détails)
<tableau signaux avec détails complets>

### Risques identifiés (détails)
<risques avec détails>

### Recommandation pathfinder
<justification + actions recommandées pour le planner>

---

### Statut
`reconnaissance-complète` | `reconnaissance-partielle`
```

---

## Règles de format du rapport pathfinder

### 1. Toujours inclure

- Header complet (Feature, Complexité, Date)
- Sections Exploration, Structure, Questions, Risques, Signaux, Recommandation
- Justification de la recommandation

### 2. Inclure conditionnellement

- Section Handoff : **UNIQUEMENT si escalade recommandée**
- Notes dans les tickets : uniquement si pertinent
- Questions/Risques : mentionner "aucun" si vide

### 3. Clarté

- Utiliser les émojis pour la lisibilité (🔍 📝 🔎 🎯 ❓ ⚠️ 🚦 📦)
- Tableaux pour les signaux (visibilité rapide)
- Listes à puces pour les items
- Gras pour les éléments importants

### 4. Exploitabilité

**Pour l'utilisateur :**
- Rapport lisible en markdown
- Compréhension rapide de la complexité et de la recommandation

**Pour orchestrator-dev (si direct) :**
- Structure draft directement exploitable
- Tickets avec estimations et dépendances
- Contexte suffisant (fichiers, patterns)

**Pour planner (si escalade) :**
- Section Handoff complète et structurée
- Contexte déjà exploré (évite duplication)
- Questions et signaux transmis
- Draft comme base de travail

---

## Règles pour le producteur (pathfinder)

- **En CONTEXTE = orchestrator_feature** : produire UNIQUEMENT le bloc `## Retour vers orchestrator` — aucun texte avant ou après. Le rapport complet est DANS le bloc.
- **En standalone** : produire le rapport directement (sans le bloc `## Retour vers orchestrator`)
- Le rapport intégré dans `### Rapport pathfinder complet` suit exactement le format défini ci-dessus

> ❌ Ne jamais écrire de texte en dehors du bloc de handoff (en contexte orchestrator_feature)
> ❌ Ne jamais résumer le rapport dans le bloc — il doit être complet et exhaustif

---

## Bloc `## Retour intermédiaire vers orchestrator` (clarification en cours de session)

Produit quand le pathfinder détecte une **clarification critique** en cours d'exploration et doit interrompre sa session (CONTEXTE = orchestrator_feature uniquement).

Ce bloc précède toujours un `## Question pour l'orchestrator`.

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** pathfinder
**Phase :** Clarification en cours d'exploration
**task_id :** <sessionID courant>

### Ce qui a été exploré jusqu'ici
- <Observation 1>
- <Observation 2>
- ...

### Problème détecté
<Description précise de l'information manquante ou du point bloquant>

### Impact
<Conséquence sur l'estimation de complexité ou la recommandation>

### Hypothèse possible
<Formulation de l'hypothèse si l'utilisateur préfère continuer sans info>
```

---

## Bloc `## Question pour l'orchestrator` (clarification en cours de session)

Accompagne toujours un `## Retour intermédiaire vers orchestrator`. Permet à l'agent orchestrator de relayer la question à l'utilisateur puis de re-invoquer le pathfinder avec `task_id` + la réponse.

```markdown
## Question pour l'orchestrator

**Phase :** Clarification
**task_id :** <sessionID courant>

**Contexte :** <Description du problème et de son impact — doit permettre à l'utilisateur de comprendre sans avoir vu la session enfant>

**Question :** <Question précise>

**Options :**
- `fournir-information` — <Description de l'option : l'utilisateur fournit l'info>
- `continuer-hypothese` — <Description : continuer avec l'hypothèse [formulation]>

**Instruction de reprise :** "Réponse à la clarification pathfinder : [option]. [Information fournie si applicable]. Reprendre l'exploration depuis le point d'interruption et finaliser le rapport."
```

**Règles :**
- ✅ Toujours inclure le `task_id` (sessionID courant)
- ✅ Le contexte doit être compréhensible sans avoir vu la session enfant
- ✅ L'instruction de reprise doit permettre au pathfinder de reprendre exactement où il s'était arrêté
- ❌ Ne jamais interrompre pour un détail non critique — utiliser une hypothèse documentée à la place

---

## Règles d'utilisation des blocs selon le contexte

| Bloc | Quand le produire | Contexte |
|------|-------------------|----------|
| `## Retour vers orchestrator` (avec `### Rapport pathfinder complet`) | Fin de session | orchestrator_feature uniquement |
| Rapport pathfinder seul (sans bloc wrapping) | Fin de session | standalone uniquement |
| `## Retour intermédiaire vers orchestrator` | Clarification critique détectée | orchestrator_feature uniquement |
| `## Question pour l'orchestrator` | Avec le bloc intermédiaire | orchestrator_feature uniquement |
| Outil `question` | Clarifications ou décisions | **standalone UNIQUEMENT** — jamais en orchestrator_feature |

---

## Règles pour le consommateur (orchestrator)

**Spécificités pathfinder à vérifier :**

- **Champs obligatoires du bloc** : `Feature`, `Complexité`, `Recommandation`, `Justification`, `Rapport pathfinder complet`, `Statut`. Si l'un est absent → demander au pathfinder de compléter.
- **Retranscription** : afficher les champs du bloc de manière formatée dans la discussion (voir skill `retranscription-coordinateur`). Le `### Rapport pathfinder complet` est affiché intégralement.
- **Si `direct`** : transmettre le rapport au `orchestrator-dev` comme contexte.
- **Si `escalade-planner`** : transmettre au planner la section `## 📦 Handoff vers planner` du rapport.
- **Statut** : `reconnaissance-complète` → CP-path normal · `reconnaissance-partielle` → signaler qu'une clarification est en attente.
