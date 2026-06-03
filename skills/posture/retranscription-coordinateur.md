---
name: retranscription-coordinateur
description: Protocole de retransmission des retours de sous-agents par les agents coordinateurs (orchestrator, auditor, orchestrator-dev). Complémentaire aux workflows agents (planner-workflow, auditor-workflow) qui définissent comment les agents produisent leurs récaps. Ce skill définit comment les coordinateurs retransmettent ces récaps à l'utilisateur.
---

# Skill — Protocole de retransmission (Coordinateurs)

## Distinction : Producteur vs Consommateur

Ce skill s'adresse aux **agents coordinateurs** (orchestrator, auditor, orchestrator-dev).

| Perspective | Agent concerné | Skill de référence | Responsabilité |
|-------------|---------------|-------------------|----------------|
| **Producteur** | planner, auditor-*, ux-designer, ui-designer, developer-*, qa-engineer, reviewer, debugger, onboarder | `planner-workflow`, `auditor-workflow`, `ux-protocol`, `ui-protocol`, `developer-handoff-format`, `qa-protocol`, `reviewer-protocol`, `debugger-workflow`, `onboarder-workflow` | **Produire** le récap et l'afficher avant d'appeler `question` |
| **Consommateur** | orchestrator, auditor (coordinateur), orchestrator-dev | **Ce skill** (`retranscription-coordinateur`) | **Retransmettre** le récap reçu d'un sous-agent avant d'appeler `question` |

**Exemple de flux complet :**

1. Orchestrator invoque planner via `task`
2. **Planner (producteur)** : suit `planner-workflow` → produit son récap → affiche avant question → retourne à orchestrator
3. **Orchestrator (consommateur)** : suit `retranscription-coordinateur` → reçoit le récap → **le retransmet à l'utilisateur** avant d'appeler question

> ⚠️ Les deux perspectives appliquent la même règle ("afficher avant question"), mais à des moments différents du flux :
> - Le **producteur** l'applique **quand il termine son travail** (interne à sa session)
> - Le **consommateur** l'applique **quand il reçoit le retour** (dans sa propre session avec l'utilisateur)

---

## Règle fondamentale (perspective consommateur)

Quand tu invoques un sous-agent via `task`, tu DOIS retranscrire son retour complet à l'utilisateur **AVANT** de poser toute question.

**Séquence obligatoire — sans exception :**

1. Recevoir le retour du sous-agent
2. **Si le retour contient des blocs `## Retour intermédiaire vers orchestrateur`** → les afficher en texte dans l'ordre, en premier
3. **Afficher le récap final / rapport complet en texte** dans la discussion
4. **Afficher le bloc structuré** (`## Retour vers orchestrator`) dans la discussion
5. **Puis seulement** appeler l'outil `question`

> ❌ Ne jamais appeler `question` comme première action après réception d'un retour
> ❌ Ne jamais résumer le récap — le copier intégralement
> ❌ Ne jamais omettre le bloc structuré
> ❌ Ne jamais sauter les blocs intermédiaires s'ils sont présents

---

## Format de retranscription

### Template standard (retour final)

Utiliser ce template après chaque réception de retour final :

```
**[Retranscription du retour <agent>]**

---

### Blocs intermédiaires (si présents)

<Copier-coller intégral de chaque ## Retour intermédiaire vers orchestrateur, dans l'ordre — uniquement si présents>

---

### <Titre du récap — ex: Récapitulatif de planification, Rapport d'audit, Spec UX>

<Copier-coller intégral du récap narratif reçu>

---

### Bloc structuré

<Copier-coller intégral du bloc `## Retour vers orchestrator` reçu>

---

**[Fin de retranscription]**
```

### Template pour une question montante (planner / scout)

Quand le planner ou le scout termine sa session avec `## Question pour l'orchestrateur` :

```
**[Retranscription — question montante <agent>]**

---

### Récap intermédiaire

<Copier-coller intégral du bloc ## Retour intermédiaire vers orchestrateur>

---

**[Fin de retranscription]**

**Vérification :**
- ✅ Récap intermédiaire complet affiché (aucun résumé, aucune omission)
- ✅ task_id noté pour la ré-invocation : <task_id>

**Maintenant seulement,** utiliser l'outil `question` pour relayer la question à l'utilisateur.
```

### Vérification obligatoire avant question

Avant d'appeler `question`, vérifier :

- ✅ Les blocs `## Retour intermédiaire vers orchestrateur` sont affichés (si présents)
- ✅ Le récap complet est affiché en texte (aucune omission, aucun résumé)
- ✅ Le bloc structuré est affiché en texte avec tous les champs obligatoires
- ✅ Les sections critiques sont présentes (ex : `### Hypothèses et ambiguïtés`, `### Risques identifiés`, `### Contraintes d'implémentation`, etc. selon l'agent)

### ✅ Checklist visuelle — AVANT CHAQUE APPEL À `question`

**STOP — Vérifier MAINTENANT :**

| Vérification | Fait ? |
|--------------|--------|
| ✅ Les blocs `## Retour intermédiaire vers orchestrateur` sont affichés en texte (si présents) | ⬜ |
| ✅ J'ai affiché le récap narratif complet du sous-agent en texte (copier-coller intégral, non résumé) | ⬜ |
| ✅ J'ai affiché le bloc structuré `## Retour vers orchestrator` en entier | ⬜ |
| ✅ Les sections critiques de ce type de retour sont présentes (voir tableau "Règles par type de retour") | ⬜ |
| ✅ Le contenu est affiché AVANT cet appel à `question`, PAS après | ⬜ |
| ✅ Le récap n'est PAS inclus dans le champ `question` de l'outil | ⬜ |

**Si une seule case est ⬜ (non cochée) → ARRÊTER et afficher le contenu manquant MAINTENANT.**

**Une fois toutes les cases cochées ✅ → Continuer vers l'appel `question`.**

**Autocontrôle visuel :**

> « Ai-je affiché les blocs intermédiaires + le récap + le bloc structuré AVANT d'appeler question ? »
> → NON : STOP — afficher MAINTENANT
> → OUI : continuer

---

## Règles par type de retour

| Agent source | Type de retour | Récap à retranscrire | Sections critiques à vérifier |
|--------------|---------------|----------------------|-------------------------------|
| **planner** (final) | `## Retour vers orchestrator` | Récapitulatif de planification + blocs intermédiaires si présents | `### Hypothèses et ambiguïtés`, `### Risques identifiés`, `### Ordre de traitement` |
| **planner** (question montante) | `## Question pour l'orchestrateur` | `## Retour intermédiaire vers orchestrateur` | Contenu de la phase, contexte de la question, `task_id` |
| **scout** (final) | `## Retour vers orchestrator` | Rapport scout complet + blocs intermédiaires si présents | `## Recommandation`, `## Signaux détectés`, `## Handoff vers planner` si escalade |
| **scout** (question montante) | `## Question pour l'orchestrateur` | `## Retour intermédiaire vers orchestrateur` | Ce qui a été exploré, problème détecté, `task_id` |
| **auditor-*** | Rapport d'audit complet | `## Synthèse des problèmes identifiés`, `## Risque résiduel si non corrigé` |
| **ux-designer, ui-designer** | Spec complète (user flows, wireframes, tokens) | `### Contraintes d'implémentation`, `### Points ouverts` |
| **debugger** | Rapport de diagnostic complet | `### Actions d'urgence si bug en prod`, `### Impact et régressions potentielles` |
| **onboarder** | Rapport d'onboarding complet | `### Zones d'incertitude`, `### Dette technique détectée` |
| **orchestrator-dev** | Récap global complet (tableau + comptes rendus + points d'attention) | `### Détail par ticket`, `### Points d'attention` |
| **reviewer** | Rapport de review complet | `### Synthèse des problèmes`, `### Verdict` |

---

## Ce que tu NE fais JAMAIS

❌ Résumer ou abréger le récap avant de l'afficher  
❌ Poser la question avant d'avoir affiché le contenu  
❌ Inclure le récap ou le bloc structuré dans le champ `question` de l'outil  
❌ Omettre des sections du bloc structuré "parce qu'elles sont vides" (afficher la mention explicite si vide)  
❌ Reformuler le contenu reçu — le copier tel quel  

---

## Exemples

### ❌ INTERDIT

```
[Orchestrator reçoit retour du planner]
→ Appelle directement question({
    questions: [{
      header: "CP-0",
      question: "Le planner a créé 3 tickets. Quelle suite ?",
      options: [...]
    }]
  }) sans afficher le récap
```

**Problème :** L'utilisateur ne voit pas le récap de planification avant de prendre sa décision au CP-0. Il ne connaît pas les tickets créés, les dépendances, les risques identifiés.

---

### ✅ CORRECT

```
[Orchestrator reçoit retour du planner]

**[Retranscription du retour planner]**

---

### Récapitulatif de planification

Le planner a créé 3 tickets pour la feature "Authentification JWT" :

- **bd-42** : Créer l'endpoint POST /auth/login (P1, developer-backend)
  - Critères d'acceptance : endpoint retourne un token JWT valide avec expiration 24h
  - Dépendances : aucune — ticket fondation
  
- **bd-43** : Implémenter le middleware de vérification JWT (P1, developer-backend)
  - Critères d'acceptance : middleware rejette les requêtes sans token ou avec token invalide/expiré
  - Dépendances : bd-42 (consomme le service d'auth créé par bd-42)
  
- **bd-44** : Créer la page de login (P1, developer-frontend)
  - Critères d'acceptance : formulaire login/password, appel API /auth/login, stockage token localStorage
  - Dépendances : bd-42 (consomme l'endpoint créé par bd-42)

**Dépendances identifiées :**
- bd-43 dépend de bd-42 : le middleware consomme le service d'authentification
- bd-44 dépend de bd-42 : la page login consomme l'endpoint d'authentification

**Ordre de traitement recommandé :**
1. bd-42 (ticket fondation, bloquant pour bd-43 et bd-44)
2. bd-43, bd-44 (parallélisables après bd-42)

**Risques identifiés :**
- Aucune stratégie de rotation des tokens définie — risque de tokens compromis non révocables
- Pas de rate limiting spécifié sur l'endpoint /auth/login — risque de brute force

**Hypothèses faites :**
- Hypothèse : la stratégie de refresh tokens sera implémentée ultérieurement
- Hypothèse : le stockage en localStorage est acceptable (alternative : httpOnly cookies)

---

### Bloc structuré

## Retour vers orchestrator

**Agent :** planner
**Feature :** Authentification JWT

### Tickets créés

| ID | Titre | Type | Priorité | Labels | Agent prévu | TDD | Dépend de |
|----|-------|------|----------|--------|-------------|-----|-----------|
| bd-42 | Créer endpoint POST /auth/login | feature | P1 | backend, api, auth | developer-backend | — | — |
| bd-43 | Implémenter middleware vérification JWT | feature | P1 | backend, middleware, auth | developer-backend | ✅ | bd-42 |
| bd-44 | Créer page de login | feature | P1 | frontend, ui, auth | developer-frontend | — | bd-42 |

**Total :** 3 tickets créés (0 epics + 3 tickets fils)

### Dépendances
- `bd-43` dépend de `bd-42` : le middleware consomme le service d'authentification créé par bd-42
- `bd-44` dépend de `bd-42` : la page login consomme l'endpoint créé par bd-42

### Ordre de traitement
1. bd-42 — ticket fondation, bloquant pour bd-43 et bd-44
2. bd-43, bd-44 — parallélisables après bd-42

### Hypothèses et ambiguïtés
- Hypothèse : la stratégie de refresh tokens sera implémentée ultérieurement
- Hypothèse : le stockage en localStorage est acceptable (alternative : httpOnly cookies)

### Estimation globale
**Tickets :** 3 | **Complexité estimée :** moyenne

### Risques identifiés
- Aucune stratégie de rotation des tokens définie — risque de tokens compromis non révocables
- Pas de rate limiting spécifié sur l'endpoint /auth/login — risque de brute force

### Statut
`planification-complète`

---

**[Fin de retranscription]**

→ **Maintenant seulement**, appeler question({
    questions: [{
      header: "CP-0 — Authentification JWT",
      question: "Planification complète : 3 tickets créés (bd-42, bd-43, bd-44). 2 risques identifiés (rotation tokens, rate limiting). Quelle suite ?",
      options: [
        { label: "Démarrer l'implémentation", description: "Router les tickets vers orchestrator-dev en mode manuel" },
        { label: "Réviser la planification", description: "Retourner au planner avec des ajustements" },
        { label: "Ajouter des tickets", description: "Créer des tickets supplémentaires pour les risques identifiés" }
      ]
    }]
  })
```

**Pourquoi c'est correct :**
- ✅ L'utilisateur voit **tout le contexte** avant de décider (tickets, dépendances, risques, hypothèses)
- ✅ Le récap narratif est complet (aucun résumé)
- ✅ Le bloc structuré est affiché en entier (tous les champs obligatoires)
- ✅ La question est posée **après** l'affichage du contenu

---

### ✅ CORRECT — Exemple debugger (Mode D)

```
[Orchestrator reçoit retour du debugger]

**[Retranscription du retour debugger]**

---

### Rapport de diagnostic

## [Phase 5] Diagnostic — TypeError sur l'endpoint POST /api/auth/login

### Symptôme
L'endpoint retourne une erreur 500 avec message "Cannot read property 'id' of undefined" quand l'utilisateur tente de se connecter avec des identifiants valides. Fréquence : systématique (100% des tentatives). Environnement : production.

### Périmètre analysé
Artefacts fournis : stacktrace complète (23 frames), logs applicatifs (fenêtre 5 min), description précise du comportement. Ticket Beads bd-156 consulté.

### Localisation probable
`src/services/auth.service.ts:47` — fonction `authenticateUser`

### Cause racine

#### Hypothèse principale — haute probabilité
Le service d'authentification tente d'accéder à `user.id` alors que la requête BDD retourne `null` si l'utilisateur n'existe pas. Aucune vérification de nullité avant l'accès à la propriété.

**Éléments qui l'étayent :**
- Stacktrace : `TypeError: Cannot read property 'id' of undefined at authenticateUser (auth.service.ts:47)`
- Log BDD : `SELECT * FROM users WHERE email = 'test@example.com' → 0 rows`
- Code source ligne 47 : `const token = generateToken(user.id)` sans vérification préalable

**Pour confirmer :**
- Ajouter un breakpoint ligne 47 et vérifier la valeur de `user`
- Tester avec un email inexistant pour reproduire

#### Hypothèse secondaire — probabilité moyenne
La requête BDD échoue silencieusement et retourne `undefined` au lieu de `null`, ce qui bypasse les vérifications de nullité existantes.

**Éléments qui l'étayent :**
- Pattern observé dans d'autres services utilisant le même ORM

**Pour confirmer :**
- Vérifier les logs ORM pour détecter des erreurs silencieuses

### Fichiers impliqués
| Fichier | Rôle dans le bug |
|---------|-----------------|
| `src/services/auth.service.ts:47` | Point d'origine — accès à `user.id` sans vérification |
| `src/controllers/auth.controller.ts:23` | Point de propagation — retourne 500 au lieu de 401 |

### ⚠️ Informations manquantes
Aucune — tous les artefacts nécessaires étaient disponibles.

### Ticket de correction suggéré
**Titre :** Corriger le TypeError sur /api/auth/login avec identifiants invalides
**Type :** bug
**Priorité :** P0 (bug bloquant en production)
**Description :** L'endpoint d'authentification plante avec une erreur 500 au lieu de retourner un 401 quand les identifiants sont invalides. Impact : tous les utilisateurs avec identifiants incorrects voient une erreur serveur au lieu d'un message d'erreur approprié.
**Acceptance criteria :**
- L'endpoint retourne 401 avec message "Invalid credentials" si l'utilisateur n'existe pas
- L'endpoint retourne 401 avec message "Invalid credentials" si le mot de passe est incorrect
- Aucune exception levée dans les logs applicatifs
**Notes techniques :** Ajouter une vérification `if (!user)` ligne 46 avant d'accéder à `user.id`. Retourner une erreur 401 au controller au lieu de laisser l'exception se propager.

---

### Bloc structuré

## Retour vers orchestrator

**Agent :** debugger
**Problème :** TypeError "Cannot read property 'id' of undefined" sur l'endpoint POST /api/auth/login

### Cause racine
**Hypothèse retenue :** Le service d'authentification tente d'accéder à `user.id` alors que la requête BDD retourne `null` si l'utilisateur n'existe pas. Aucune vérification de nullité avant l'accès à la propriété.
**Niveau de certitude :** confirmé
**Chaîne causale :**
1. L'utilisateur envoie une requête POST /api/auth/login avec un email inexistant
2. La requête BDD `SELECT * FROM users WHERE email = '...'` retourne 0 rows (valeur `null`)
3. Le code ligne 47 tente d'accéder à `user.id` sans vérifier que `user` n'est pas `null`
4. Une exception TypeError est levée et propagée au controller
5. Le controller retourne une erreur 500 au lieu d'un 401

### Hypothèses explorées
- `Accès à user.id sans vérification de nullité` : **confirmée** — stacktrace et code source confirment
- `Requête BDD échoue silencieusement` : **insuffisamment documentée** — logs ORM incomplets, nécessiterait instrumentation supplémentaire
- `Pattern répété dans d'autres services` : **insuffisamment documentée** — nécessiterait audit global du codebase

### Impact et régressions potentielles
- **Authentification compromise** : tous les utilisateurs avec identifiants incorrects voient une erreur 500 au lieu d'un message d'erreur approprié
- **Exposition d'informations sensibles** : la stacktrace complète est retournée dans la réponse 500, exposant la structure interne de l'application
- **Pattern répété** : le même bug pourrait exister dans d'autres endpoints utilisant le même pattern (à auditer)

### Tickets de correction créés

| ID | Titre | Priorité | Labels |
|----|-------|----------|--------|
| bd-157 | Corriger le TypeError sur /api/auth/login avec identifiants invalides | P0 | bug, backend, auth, security, from-diagnostic |

### Actions d'urgence si bug en prod
1. **Hotfix immédiat** : déployer une vérification `if (!user) return { error: 'Invalid credentials', status: 401 }` ligne 46 de `auth.service.ts`
2. **Désactiver l'exposition de stacktraces** : configurer l'environnement de production pour ne pas retourner de stacktraces dans les réponses 500
3. **Monitoring** : ajouter une alerte sur le nombre d'erreurs 500 sur l'endpoint /api/auth/login
4. **Communication** : informer l'équipe support que les utilisateurs voient actuellement des erreurs 500 au lieu de messages d'erreur clairs

### Statut
`diagnostiqué`

---

**[Fin de retranscription]**

**Vérifications effectuées :**
- ✅ Rapport de diagnostic complet copié tel quel (symptôme, cause racine, fichiers, ticket)
- ✅ Bloc structuré avec tous les champs obligatoires présents
- ✅ Sections critiques présentes : Actions d'urgence (4 steps), Impact et régressions (3 points)
- ✅ Statut : `diagnostiqué` (ticket bd-157 créé avec priorité P0)

**Actions d'urgence détectées :** ⚠️ Ce bug est en PRODUCTION avec impact sécurité — présenter les 4 actions d'urgence en PRIORITÉ.

→ **Maintenant seulement**, appeler question({
    questions: [{
      header: "Bug critique en production",
      question: "[Orchestrator — Mode D | Bug : TypeError /api/auth/login]\n\n⚠️ BUG CRITIQUE EN PRODUCTION détecté avec 4 actions d'urgence listées ci-dessus.\n\nTicket de correction bd-157 créé (P0). Comment souhaitez-vous procéder ?",
      options: [
        { label: "Implémenter le hotfix immédiatement", description: "Router bd-157 vers orchestrator-dev en mode manuel avec priorité maximale" },
        { label: "Désactiver la feature en attendant", description: "Désactiver l'endpoint /api/auth/login temporairement" },
        { label: "Voir les détails techniques", description: "Consulter le code source concerné avant de décider" }
      ]
    }]
  })
```

**Pourquoi c'est correct :**
- ✅ Le rapport complet (symptôme, cause, hypothèses, fichiers, ticket) est affiché AVANT la question
- ✅ Le bloc structuré est affiché avec TOUS les champs obligatoires
- ✅ Les actions d'urgence sont présentées en PRIORITÉ avec un avertissement visuel
- ✅ Les vérifications sont listées explicitement pour validation visuelle
- ✅ La question est posée APRÈS l'affichage complet du contexte
- ✅ Les options proposées sont adaptées à la gravité (bug P0 en production)

---

### ❌ INTERDIT — Exemple debugger (ce qui arrive actuellement)

```
[Orchestrator reçoit retour du debugger]

→ Appelle directement question({
    questions: [{
      header: "Bug diagnostiqué",
      question: "Le debugger a terminé le diagnostic. Quelle suite ?",
      options: [
        { label: "Implémenter la correction", description: "Router vers orchestrator-dev" },
        { label: "Voir le rapport", description: "Afficher le rapport de diagnostic" }
      ]
    }]
  }) sans afficher le rapport ni le bloc
```

**Problèmes :**
- ❌ L'utilisateur ne voit PAS le rapport de diagnostic avant de décider
- ❌ L'utilisateur ne connaît PAS la cause racine identifiée
- ❌ L'utilisateur ne connaît PAS les actions d'urgence si le bug est en production
- ❌ L'utilisateur ne connaît PAS le ticket créé ni sa priorité
- ❌ L'utilisateur doit choisir "Voir le rapport" pour obtenir le contexte (ordre inversé)
- ❌ Les sections critiques (impact, régressions, actions d'urgence) sont invisibles au moment de la décision

---

## Injection de ce skill

Ce skill doit être injecté dans les 3 agents coordinateurs :

| Agent | Fichier | Ligne `skills:` | Position recommandée |
|-------|---------|-----------------|----------------------|
| **orchestrator** | `agents/planning/orchestrator.md` | L40 | Après `posture/coordination-only` |
| **orchestrator-dev** | `agents/planning/orchestrator-dev.md` | L49 | Après `posture/coordination-only` |
| **auditor** | `agents/auditor/auditor.md` | L16 | Après `posture/coordination-only` |

**Exemple d'injection (orchestrator.md ligne 40) :**

```yaml
skills: [posture/coordination-only, posture/retranscription-coordinateur, orchestrator/orchestrator-workflow-modes, ...]
```

---

## Relation avec les autres skills

| Skill | Scope | Complémentarité |
|-------|-------|-----------------|
| **coordination-only** | Définit la posture "ne jamais faire le travail soi-même, toujours déléguer" | ✅ Complémentaire — retranscription-coordinateur définit **comment retransmettre** ce que les sous-agents ont produit |
| **tool-question** | Définit **comment utiliser** l'outil `question` (format, multi-questions, etc.) | ✅ Complémentaire — retranscription-coordinateur définit **quoi afficher avant** d'utiliser `question` |
| **planner-workflow** | Définit comment le planner **produit** son récap avant d'appeler question | ✅ Complémentaire — retranscription-coordinateur définit comment l'orchestrator **retransmet** ce récap à l'utilisateur |

> Ce skill ne duplique aucune règle existante — il couvre un aspect spécifique de la chaîne de communication (retransmission par les coordinateurs) qui n'était pas documenté ailleurs.

---

## Référence

**Source de vérité :** Ce skill est la référence unique pour les règles de retransmission des coordinateurs.

**Date de création :** 28 mai 2026  
**Contexte :** Correctif du problème de non-retransmission des récaps dans la chaîne orchestrator → sous-agents (planner, auditor, design, debugger, onboarder)
