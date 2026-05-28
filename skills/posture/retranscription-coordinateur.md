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

1. Recevoir le retour du sous-agent (récap + bloc structuré)
2. **Afficher le récap complet en texte** dans la discussion
3. **Afficher le bloc structuré** dans la discussion
4. **Puis seulement** appeler l'outil `question`

> ❌ Ne jamais appeler `question` comme première action après réception d'un retour
> ❌ Ne jamais résumer le récap — le copier intégralement
> ❌ Ne jamais omettre le bloc structuré

---

## Format de retranscription

### Template standard

Utiliser ce template après chaque réception de retour :

```
**[Retranscription du retour <agent>]**

---

### <Titre du récap — ex: Récapitulatif de planification, Rapport d'audit, Spec UX>

<Copier-coller intégral du récap narratif reçu>

---

### Bloc structuré

<Copier-coller intégral du bloc `## Retour vers orchestrator` reçu>

---

**[Fin de retranscription]**
```

### Vérification obligatoire avant question

Avant d'appeler `question`, vérifier :

- ✅ Le récap complet est affiché en texte (aucune omission, aucun résumé)
- ✅ Le bloc structuré est affiché en texte avec tous les champs obligatoires
- ✅ Les sections critiques sont présentes (ex : `### Hypothèses et ambiguïtés`, `### Risques identifiés`, `### Contraintes d'implémentation`, etc. selon l'agent)

**Autocontrôle visuel :**

> « Ai-je affiché le récap ET le bloc AVANT d'appeler question ? »
> → NON : STOP — afficher MAINTENANT
> → OUI : continuer

---

## Règles par type de retour

| Agent source | Récap à retranscrire | Sections critiques à vérifier |
|--------------|----------------------|-------------------------------|
| **planner** | Récapitulatif de planification | `### Hypothèses et ambiguïtés`, `### Risques identifiés`, `### Ordre de traitement` |
| **auditor-*** | Rapport d'audit complet | `### Synthèse des problèmes identifiés`, `### Risque résiduel si non corrigé` |
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
