---
name: pathfinder-handoff-format
description: Format de rapport pathfinder et format d'escalade vers le planner — structure exploitable par l'utilisateur, orchestrator-dev et planner.
---

# Skill — Pathfinder Handoff Format

## Rôle

Ce skill définit le format exact du rapport pathfinder et de son handoff vers le planner.

## Format complet du rapport pathfinder

```markdown
# 🔍 Pathfinder Report

**Feature:** [Nom court de la feature]
**Complexité:** [XS|S|M|L|XL] 
**Date:** [YYYY-MM-DD HH:mm]

---

## 📝 Contexte rapide

[2-3 phrases décrivant ce qui a été compris de la demande utilisateur]

---

## 🔎 Exploration (2-3 min)

### Fichiers/Modules clés identifiés
- `path/to/file.ts` - [raison de pertinence]
- `path/to/module/` - [raison]
- (ou "Aucun fichier spécifique identifié" si pas pertinent)

### Tickets Beads existants
- **bd-123** - [Titre] — [Relation avec cette feature]
- **bd-456** - [Titre] — [Relation]
- (ou "Aucun ticket existant directement lié")

### Patterns/Logiques réutilisables
- [Pattern X dans module Y] - peut être réutilisé pour [aspect]
- [Service Z] - logique similaire disponible
- (ou "Aucun pattern directement réutilisable identifié")

---

## 🎯 Structure proposée (draft)

### Epic suggéré
**[Nom de l'epic si nécessaire]**
(ou "Pas d'epic nécessaire (ticket unique)")

### Tickets estimés (~)

#### 1. **[Titre ticket 1]** (type: feature/task, P1, ~30-60min)
- **Description courte:** [1 phrase claire]
- **Dépend de:** [bd-X ou "aucune"]
- **Notes:** [remarque technique rapide si nécessaire, sinon omettre]

#### 2. **[Titre ticket 2]** (type: task, P2, ~60-120min)
- **Description courte:** [1 phrase]
- **Dépend de:** ticket 1 (dépendance séquentielle)

[...]

**Total estimé:** ~[durée] ([taille XS/S/M/L/XL])

---

## ❓ Questions ouvertes

- [ ] **[Métier]** Question métier si applicable
- [ ] **[Technique]** Question technique si applicable
- [ ] **[Design]** Question UX/UI si applicable
- (ou "Aucune question critique — feature bien définie")

---

## ⚠️ Risques identifiés

- **[Niveau: Faible/Moyen/Élevé]** [Description du risque + impact potentiel]
- (ou "Aucun risque particulier identifié")

---

## 🚦 Signaux détectés

| Signal | Détecté | Détails |
|--------|---------|---------|
| **UX/UI** | ❌ / ⚠️ / ✅ | [Détails si ⚠️ ou ✅] |
| **Sécurité** | ❌ / ⚠️ / ✅ | [Détails] |
| **Performance** | ❌ / ⚠️ / ✅ | [Détails] |
| **Accessibilité** | ❌ / ⚠️ / ✅ | [Détails] |
| **Architecture** | ❌ / ⚠️ / ✅ | [Détails] |

**Légende :**
- ❌ Aucun signal détecté
- ⚠️ Signal faible (à surveiller)
- ✅ Signal fort (nécessite attention)

---

## 🎯 Recommandation

[CHOISIR UNE OPTION CI-DESSOUS]

---

### Option A : Traitement direct

✅ **Traitement direct recommandé**

Cette feature est suffisamment simple et bien définie pour être traitée directement par `orchestrator-dev`.

**Justification :**
- Complexité [XS/S/M]
- Aucun signal critique
- Peu ou pas de questions ouvertes
- Risques faibles

**Prochaine étape suggérée :**
→ Invoquer `orchestrator-dev` avec ce rapport comme contexte

---

### Option B : Escalade au planner

🎯 **Escalade au planner recommandée**

Cette feature présente une complexité ou des signaux nécessitant une planification complète.

**Justification :**
1. [Raison 1 : ex. Complexité L détectée (6+ tickets estimés)]
2. [Raison 2 : ex. Signal architecture fort (nouveau système)]
3. [Raison 3 : ex. Risque sécurité élevé, audit nécessaire]

**Prochaine étape suggérée :**
→ Invoquer `planner` avec le handoff ci-dessous

---

## 📦 Handoff vers planner (si escalade)

[CETTE SECTION N'EST PRÉSENTE QUE SI ESCALADE RECOMMANDÉE]

**Agent source:** pathfinder  
**Feature:** [nom complet]  
**Complexité estimée:** [L/XL]  

### Contexte déjà exploré

**Fichiers clés identifiés :**
- [liste complète avec chemins]

**Tickets Beads liés :**
- [IDs avec titres, ou "aucun"]

**Patterns/logiques réutilisables :**
- [liste complète]

### Structure draft (à valider/affiner par le planner)

[Copie complète de la section "Structure proposée"]

### Questions posées et réponses

- **Q1:** [question posée à l'utilisateur] → **R:** [réponse si disponible, sinon "en attente"]

### Questions restantes (pour le planner)

[Copie de la section "Questions ouvertes"]

### Signaux détectés (détails)

[Copie du tableau des signaux avec détails complets]

### Risques identifiés (détails)

[Copie de la section risques avec détails complets]

### Recommandation pathfinder

Le pathfinder recommande l'escalade pour les raisons suivantes :
[Justification complète reprise de la section Recommandation]

Le planner doit :
- Affiner la structure proposée
- Résoudre les questions ouvertes
- [Action spécifique 1 si applicable : ex. Consulter ux-designer pour les flows]
- [Action spécifique 2 : ex. Envisager audit sécurité]

---
```

## Règles de format

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

## Exemples

### Exemple 1 : Rapport pathfinder avec traitement direct (S)

```markdown
# 🔍 Pathfinder Report

**Feature:** Ajouter champ téléphone au profil utilisateur  
**Complexité:** S  
**Date:** 2026-05-28 14:30  

---

## 📝 Contexte rapide

L'utilisateur souhaite ajouter un champ "numéro de téléphone" optionnel dans le profil utilisateur. Le champ doit être modifiable depuis la page profil et stocké en base.

---

## 🔎 Exploration (1 min)

### Fichiers/Modules clés identifiés
- `src/models/User.ts` - Modèle utilisateur
- `src/components/profile/ProfileForm.tsx` - Formulaire profil
- `migrations/` - Dossier migrations BDD

### Tickets Beads existants
- **bd-67** - Page profil utilisateur — Ticket parent logique

### Patterns/Logiques réutilisables
- Champ email existant dans ProfileForm.tsx - pattern identique

---

## 🎯 Structure proposée (draft)

### Epic suggéré
Pas d'epic nécessaire (feature simple, rattachement à bd-67)

### Tickets estimés (~)

#### 1. **Ajouter champ téléphone au modèle User** (type: task, P1, ~30min)
- **Description courte:** Migration BDD + ajout propriété dans User.ts
- **Dépend de:** aucune

#### 2. **Ajouter input téléphone dans ProfileForm** (type: task, P2, ~45min)
- **Description courte:** Input + validation format téléphone + sauvegarde
- **Dépend de:** ticket 1

**Total estimé:** ~1h15 (S)

---

## ❓ Questions ouvertes

- [ ] **[Métier]** Format de téléphone attendu ? (international ou national uniquement)
- [ ] **[Métier]** Validation stricte du format ou libre ?

---

## ⚠️ Risques identifiés

Aucun risque particulier identifié (feature simple et isolée).

---

## 🚦 Signaux détectés

| Signal | Détecté | Détails |
|--------|---------|---------|
| **UX/UI** | ❌ | Aucun signal, réutilise pattern existant |
| **Sécurité** | ❌ | Données non sensibles, pas d'enjeu |
| **Performance** | ❌ | Aucun impact |
| **Accessibilité** | ❌ | Input standard, accessible |
| **Architecture** | ❌ | Modification mineure, pas de refonte |

---

## 🎯 Recommandation

✅ **Traitement direct recommandé**

Cette feature est suffisamment simple et bien définie pour être traitée directement par `orchestrator-dev`.

**Justification :**
- Complexité S (2 tickets, ~1h15)
- Aucun signal critique détecté
- Questions ouvertes mineures (à poser avant implémentation)
- Risques faibles
- Pattern réutilisable existant

**Prochaine étape suggérée :**
→ Invoquer `orchestrator-dev` avec ce rapport comme contexte

---
```

---

### Exemple 2 : Rapport pathfinder avec escalade (M→L)

```markdown
# 🔍 Pathfinder Report

**Feature:** Système de notifications temps réel  
**Complexité:** L  
**Date:** 2026-05-28 15:00  

---

## 📝 Contexte rapide

L'utilisateur souhaite implémenter un système de notifications en temps réel pour avertir les utilisateurs d'événements importants (messages, alertes système, etc.). Les notifications doivent s'afficher instantanément sans rechargement de page.

---

## 🔎 Exploration (3 min)

### Fichiers/Modules clés identifiés
- `src/backend/` - Aucun module WebSocket/SSE existant
- `src/frontend/components/` - Pas de composant NotificationCenter
- `src/models/` - Pas de modèle Notification
- `migrations/` - Nouvelle table nécessaire

### Tickets Beads existants
Aucun ticket existant directement lié au système de notifications.

### Patterns/Logiques réutilisables
- `src/backend/services/EventBus.ts` - Bus d'événements interne, peut servir de base
- `src/frontend/hooks/usePolling.ts` - Pattern polling existant (à remplacer par temps réel)

---

## 🎯 Structure proposée (draft)

### Epic suggéré
**Système de notifications temps réel**

### Tickets estimés (~)

#### 1. **Design architecture notifications temps réel** (type: task, P1, ~2h)
- **Description courte:** Choix technologie (WebSocket vs SSE), architecture événements, scaling
- **Dépend de:** aucune
- **Notes:** Décision architecture majeure, impact performance

#### 2. **Backend: Serveur WebSocket + gestion événements** (type: feature, P1, ~4h)
- **Description courte:** Serveur WS, authentification connexions, dispatch événements, reconnexion
- **Dépend de:** ticket 1

#### 3. **Backend: Table et modèle Notification** (type: task, P2, ~1h)
- **Description courte:** Migration BDD + modèle Notification + CRUD
- **Dépend de:** aucune (parallèle avec ticket 2)

#### 4. **Frontend: Service NotificationClient** (type: feature, P2, ~3h)
- **Description courte:** Client WS, gestion connexion/reconnexion, store notifs, hooks React
- **Dépend de:** ticket 2

#### 5. **Frontend: UI NotificationCenter** (type: feature, P2, ~2h)
- **Description courte:** Composant centre notifs, dropdown, marquage lu/non-lu, badge compteur
- **Dépend de:** ticket 4
- **Notes:** Signal UI - design nécessaire (position, style, animations)

#### 6. **Tests E2E notifications temps réel** (type: task, P3, ~2h)
- **Description courte:** Tests Cypress/Playwright pour flux complet notif temps réel
- **Dépend de:** ticket 5

**Total estimé:** ~14h (L)

---

## ❓ Questions ouvertes

- [ ] **[Technique]** Technologie temps réel : WebSocket ou SSE ? (impact: bidirectionnel vs unidirectionnel)
- [ ] **[Technique]** Persistance des notifications ? Combien de temps conserver l'historique ?
- [ ] **[Métier]** Types de notifications prévus ? (messages, alertes système, mentions, etc.)
- [ ] **[Design]** Position et style du NotificationCenter ? (intégration design system)
- [ ] **[Performance]** Nombre d'utilisateurs connectés simultanés attendu ? (scaling)

---

## ⚠️ Risques identifiés

- **Moyen** : Gestion des connexions WebSocket (reconnexion auto, heartbeat, timeout)
- **Moyen** : Performance avec nombreux utilisateurs connectés (scaling horizontal nécessaire ?)
- **Élevé** : Décision architecture impacte tout le système (choix réversible difficilement)

---

## 🚦 Signaux détectés

| Signal | Détecté | Détails |
|--------|---------|---------|
| **UX/UI** | ✅ | NotificationCenter nécessite design (position, animations, accessibilité) |
| **Sécurité** | ⚠️ | Authentification des connexions WS à sécuriser (token JWT ?) |
| **Performance** | ✅ | Connexions persistantes, scaling, charge serveur temps réel |
| **Accessibilité** | ⚠️ | Notifications doivent être accessibles (screen reader, focus management) |
| **Architecture** | ✅ | Nouveau système majeur, choix technologique structurant |

---

## 🎯 Recommandation

🎯 **Escalade au planner recommandée**

Cette feature présente une complexité et des signaux nécessitant une planification complète.

**Justification :**
1. **Complexité L** (6 tickets, ~14h estimés)
2. **Signal architecture FORT** - Nouveau système structurant, choix technologique majeur (WebSocket vs SSE)
3. **Signal design** - UI NotificationCenter nécessite consultation `ux-designer` et `ui-designer`
4. **Signal performance** - Connexions temps réel, scaling, nécessite potentiellement `auditor` (domaine performance)
5. **Risques moyens-élevés** - Architecture réversible difficilement, impact global
6. **Questions critiques** - Plusieurs décisions techniques/métier à trancher

**Prochaine étape suggérée :**
→ Invoquer `planner` avec le handoff ci-dessous

---

## 📦 Handoff vers planner

**Agent source:** pathfinder  
**Feature:** Système de notifications temps réel  
**Complexité estimée:** L  

### Contexte déjà exploré

**Fichiers clés identifiés :**
- `src/backend/services/EventBus.ts` - Bus événements existant, réutilisable comme base
- `src/frontend/hooks/usePolling.ts` - Pattern polling actuel à remplacer
- `src/backend/` - Aucun module WS/SSE existant (nouveau)
- `src/frontend/components/` - Aucun NotificationCenter (nouveau)
- `src/models/` - Pas de modèle Notification (nouveau)
- `migrations/` - Nouvelle table BDD nécessaire

**Tickets Beads liés :**
- Aucun ticket existant directement lié

**Patterns/logiques réutilisables :**
- EventBus interne (src/backend/services/EventBus.ts) - architecture événements existante
- Hooks React custom (src/frontend/hooks/) - patterns réutilisables pour useNotifications

### Structure draft (à valider/affiner par le planner)

**Epic suggéré:** Système de notifications temps réel

**Tickets estimés (~):**

1. **Design architecture notifications temps réel** (type: task, P1, ~2h)
   - Description: Choix technologie (WebSocket vs SSE), architecture événements, scaling
   - Dépend de: aucune
   - Notes: Décision architecture majeure, impact performance

2. **Backend: Serveur WebSocket + gestion événements** (type: feature, P1, ~4h)
   - Description: Serveur WS, authentification connexions, dispatch événements, reconnexion
   - Dépend de: ticket 1

3. **Backend: Table et modèle Notification** (type: task, P2, ~1h)
   - Description: Migration BDD + modèle Notification + CRUD
   - Dépend de: aucune (parallèle avec ticket 2)

4. **Frontend: Service NotificationClient** (type: feature, P2, ~3h)
   - Description: Client WS, gestion connexion/reconnexion, store notifs, hooks React
   - Dépend de: ticket 2

5. **Frontend: UI NotificationCenter** (type: feature, P2, ~2h)
   - Description: Composant centre notifs, dropdown, marquage lu/non-lu, badge compteur
   - Dépend de: ticket 4
   - Notes: Signal UI - design nécessaire (position, style, animations)

6. **Tests E2E notifications temps réel** (type: task, P3, ~2h)
   - Description: Tests Cypress/Playwright pour flux complet notif temps réel
   - Dépend de: ticket 5

**Total estimé:** ~14h (L)

### Questions posées et réponses

- **Q1:** Quel est le cas d'usage principal ? → **R:** Notifications instantanées pour messages et alertes système

### Questions restantes (pour le planner)

- [ ] **[Technique]** WebSocket vs SSE ? (impact bidirectionnel vs unidirectionnel)
- [ ] **[Technique]** Persistance notifications ? Durée conservation historique ?
- [ ] **[Métier]** Types de notifications prévus ? (messages, alertes, mentions, etc.)
- [ ] **[Design]** Position et style NotificationCenter ? (design system)
- [ ] **[Performance]** Nombre utilisateurs connectés simultanés ? (scaling)

### Signaux détectés (détails)

| Signal | Statut | Détails |
|--------|--------|---------|
| **UX/UI** | ✅ FORT | NotificationCenter nécessite design complet (position dans layout, animations d'apparition, gestion focus, accessibilité clavier). Consultation `ux-designer` et `ui-designer` recommandée. |
| **Sécurité** | ⚠️ MOYEN | Authentification des connexions WebSocket à sécuriser (token JWT dans handshake ? Validation côté serveur ?). Pas critique mais à adresser. |
| **Performance** | ✅ FORT | Connexions persistantes multiples, charge serveur temps réel, nécessite stratégie scaling (horizontal ? load balancing ?). Consultation `auditor` (domaine performance) potentiellement utile. |
| **Accessibilité** | ⚠️ MOYEN | Notifications doivent être annoncées aux screen readers (aria-live), gestion focus lors apparition, navigation clavier dans le centre de notifs. |
| **Architecture** | ✅ FORT | Nouveau système structurant, choix technologique majeur (WS vs SSE), impact sur toute l'architecture backend/frontend. Décision difficilement réversible. |

### Risques identifiés (détails)

- **Niveau: Moyen** - Gestion des connexions WebSocket complexe (reconnexion automatique, heartbeat, timeout, gestion erreurs réseau). Solution : librairie robuste (Socket.IO ?) ou implémentation maison avec tests extensifs.

- **Niveau: Moyen** - Performance avec nombreux utilisateurs connectés (100 ? 1000 ? 10k ?). Solution : architecture scalable horizontalement dès le départ, ou accepter limite initiale et prévoir évolution.

- **Niveau: Élevé** - Décision architecture (WS vs SSE) impacte tout le système et est difficilement réversible. Solution : Phase 1.5 du planner doit absolument trancher cette question avant implémentation.

### Recommandation pathfinder

Le pathfinder recommande fortement l'escalade au planner complet pour les raisons suivantes :

1. **Complexité L** - 6 tickets, ~14h de travail, nécessite planification détaillée et validation du plan
2. **Signal architecture CRITIQUE** - Choix technologique structurant (WebSocket vs SSE) nécessite analyse approfondie et décision validée
3. **Signaux design FORTS** - UI/UX et accessibilité nécessitent consultation des designers (Phase 1.5 du planner)
4. **Signaux performance** - Audit performance potentiellement utile pour valider l'architecture scaling
5. **Risques moyens-élevés** - Plusieurs points de complexité technique nécessitent planification rigoureuse

Le planner doit :
- **Phase 1.5** : Consulter `ux-designer` pour les flows de notification (quand afficher, comment interagir, où positionner)
- **Phase 1.5** : Consulter `ui-designer` pour le design du NotificationCenter (composants, animations, design system)
- **Phase 2** : Trancher la question WebSocket vs SSE (avec argumentation technique)
- **Phase 3** : Affiner la structure proposée avec enrichissement complet des tickets
- **Phase 4** : Envisager consultation `auditor` (domaine performance) si scaling critique
- **Phase 5** : Créer les tickets Beads enrichis avec routing vers `orchestrator-dev`

---
```

---

## Règles récapitulatives

| Règle | ✅ / ❌ |
|-------|--------|
| Rapport structuré selon le format exact | ✅ |
| Section Handoff uniquement si escalade | ✅ |
| Justification de la recommandation | ✅ |
| Clarté et lisibilité (émojis, tableaux) | ✅ |
| Exploitable par utilisateur/orchestrator-dev/planner | ✅ |
| Rapport verbeux ou détails inutiles | ❌ |
| Recommandation sans justification | ❌ |
| Handoff incomplet si escalade | ❌ |

---

## Bloc `## Retour vers orchestrator` (si invoqué depuis l'agent orchestrator)

Ce bloc est produit **uniquement** quand le pathfinder est invoqué via `task` par l'agent orchestrator (CONTEXTE = orchestrator_feature). Il vient **après** le rapport pathfinder complet.

```markdown
---

## Retour vers orchestrator

**Agent :** pathfinder
**Feature :** <nom complet de la feature>
**Complexité :** <XS|S|M|L|XL>

### Recommandation
`direct` | `escalade-planner`

**Justification :** <raison principale de la recommandation>

### Handoff planner
[Si recommandation = `escalade-planner` : la section `## 📦 Handoff vers planner` du rapport ci-dessus est complète et exploitable directement]
[Si recommandation = `direct` : absent — pas de handoff nécessaire]

### Statut
`reconnaissance-complète` | `reconnaissance-partielle`
```

**Champs obligatoires :**
- `Feature` — nom complet
- `Complexité` — taille estimée (XS/S/M/L/XL)
- `Recommandation` — `direct` ou `escalade-planner`
- `Justification` — raison en 1 phrase
- `Statut` — `reconnaissance-complète` si le rapport couvre tout / `reconnaissance-partielle` si une clarification est en attente

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
| Rapport pathfinder complet | Toujours | standalone + orchestrator_feature |
| `## 📦 Handoff vers planner` | Si escalade recommandée | standalone + orchestrator_feature |
| `## Retour vers orchestrator` | Fin de session | orchestrator_feature uniquement |
| `## Retour intermédiaire vers orchestrator` | Clarification critique détectée | orchestrator_feature uniquement |
| `## Question pour l'orchestrator` | Avec le bloc intermédiaire | orchestrator_feature uniquement |
| Outil `question` | Clarifications ou décisions | **standalone UNIQUEMENT** — jamais en orchestrator_feature |
---
