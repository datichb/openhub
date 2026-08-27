---
name: onboarder-phase-3-4
description: Phase 3 (rapport de contexte + matrice agents) et Phase 4 (détection cas particuliers) du workflow onboarder.
---

## Phase 3 — Analyse approfondie : Rapport de contexte

### Objectif
Produire le rapport de contexte structuré avec la carte des agents recommandés.

### Format du rapport

````markdown
## [Phase 3] Rapport de contexte — [Nom du projet]

### Stack

| Catégorie | Technologies détectées |
|-----------|----------------------|
| Langage(s) | [ex: TypeScript 5.x, Python 3.11] |
| Framework(s) | [ex: Vue 3 + Nuxt 4, FastAPI 0.110] |
| Base(s) de données | [ex: PostgreSQL 15, Redis 7] |
| Infrastructure | [ex: Docker, GitHub Actions, Terraform] |
| Tests | [ex: Vitest, pytest, Playwright] |

### Architecture

[Description de la structure : monorepo / monolithe / microservices / BFF / etc.]
[Découpage en couches ou modules observé]
[Communication entre couches (HTTP, événements, queues)]

### Patterns dominants

- [Pattern 1 observé — ex: "Repository pattern pour l'accès aux données"]
- [Pattern 2 observé — ex: "Composables Vue pour la logique partagée"]
- [Convention observée — ex: "Conventional Commits respectés dans le git log"]

### Points d'attention

🔴 **Critiques**
- [Zone à risque élevé — citer le fichier / pattern observé]

🟠 **Importants**
- [Zone fragile — dette technique notable, couplage fort, absence de tests]

🟡 **Améliorations**
- [Opportunité — performance, qualité, éco-conception, accessibilité]

*(Section vide si aucun point détecté — ne pas inventer)*

### Zones d'ombre

- [Ce que l'exploration n'a pas permis de résoudre]
- [ex: "Logique d'authentification dans un service externe non accessible"]
- [ex: "Pas de README — architecture générale non documentée"]

*(Section vide si tout est lisible)*

### Questions de clarification posées

<Récap des questions posées en Phase 2 et des réponses reçues>

### Agents recommandés

#### Prioritaires — zones à risque détectées

| Agent | Pourquoi | Invocation suggérée |
|-------|----------|---------------------|
| `auditor` (security) | [observation concrète] | `"Audite la sécurité de ce projet"` |
| `developer-security` | À invoquer après l'audit pour corriger les failles | `"Implémente le hardening suite à l'audit sécurité"` |

*(Section absente si aucun 🔴/🟠 pertinent)*

#### Recommandés — stack détectée

| Agent | Pourquoi | Invocation suggérée |
|-------|----------|---------------------|
| `developer-frontend` | [stack frontend détectée] | `"Implémente [feature frontend]"` |
| `developer-backend` | [stack backend détectée] | `"Implémente [feature backend]"` |

#### Optionnels — selon les ambitions du projet

| Agent | Pourquoi | Invocation suggérée |
|-------|----------|---------------------|
| `auditor` (accessibility) | [observation] | `"Audite l'accessibilité"` |
| `auditor` (ecodesign) | [observation] | `"Audite l'éco-conception"` |

---

> Ces invocations sont des suggestions — c'est à toi de décider quand et si tu les lances.

### Commandes utiles pour ce projet

```bash
# Démarrer en développement
<commande détectée depuis README ou package.json>

# Tests
<commandes détectées>

# Build
<commande détectée>

# Linting
<commande détectée>

# Beads
bd list -s open           # Tickets ouverts
bd ready                  # Tickets prêts à travailler
```
````

### Matrice de recommandation des agents

#### Agents prioritaires (activés par les points d'attention 🔴/🟠)

| Signal détecté | Agents prioritaires |
|---------------|---------------------|
| Secrets en dur dans le code | `auditor` (security) → `developer-security` |
| Pas de validation des inputs côté serveur | `auditor` (security) → `developer-security` |
| Dépendances avec versions très anciennes (potentiel CVE) | `auditor` (security) |
| Hashing faible ou absent (MD5, SHA1, plain text) | `auditor` (security) → `developer-security` |
| CORS trop permissif (`*`) ou absent | `auditor` (security) → `developer-security` |
| Données personnelles sans chiffrement ni contrôle d'accès | `auditor` (privacy) |
| Pas de tests (dossier `tests/` vide ou absent) | `developer` (avec skill `dev-standards-testing`) |
| Ratio fichiers source / fichiers test très déséquilibré | `developer` (avec skill `dev-standards-testing`) |
| Requêtes N+1 visibles dans les relations ORM | `auditor` (performance) |
| Bundle non optimisé (pas de lazy loading, assets non compressés) | `auditor` (performance) |
| Pas de logs structurés / monitoring absent | `auditor` (observability) |
| Imports circulaires, God classes, couplage fort évident | `auditor` (architecture) |
| Migrations en attente non appliquées | `developer-backend` (traitement prioritaire) |

#### Agents recommandés (activés par la stack)

| Stack détectée | Agent recommandé |
|---------------|-----------------|
| Vue.js / Nuxt.js | `developer-frontend` |
| React / Next.js / Angular | `developer-frontend` |
| Node.js / NestJS / Express / Fastify | `developer-backend` |
| Python / Django / FastAPI / Flask | `developer-backend` |
| PHP / Laravel / Symfony | `developer-backend` |
| Ruby on Rails | `developer-backend` |
| Frontend + backend dans le même dépôt | `developer-fullstack` |
| API REST documentée (OpenAPI) | `developer-api` |
| API GraphQL | `developer-api` |
| dbt / Airflow / PySpark / notebooks | `developer-data` |
| Docker / GitHub Actions / scripts CI | `developer-devops` |
| Terraform / Kubernetes / Helm / ArgoCD | `developer-platform` |
| React Native / Expo | `developer-mobile` |
| Flutter | `developer-mobile` |
| Parcours utilisateur complexe non documenté | `designer` |
| Incohérences visuelles / absence de design system | `designer` |

#### Agents optionnels (selon les ambitions)

| Observation | Agent optionnel |
|-------------|----------------|
| Aucun attribut ARIA visible, sémantique HTML absente | `auditor` (accessibility) |
| Assets lourds, aucune optimisation visible | `auditor` (ecodesign) |
| SLOs non définis, alerting absent | `auditor` (observability) |
| Architecture non documentée, pas d'ADR | `documentarian` |

### Récap de fin de Phase 3

(Le récap est le rapport lui-même tel que présenté ci-dessus)

### Question de validation obligatoire


**Si CONTEXTE = standalone :**
```
question({
  questions: [{
    header: "Détection cas particuliers",
    question: "[Onboarder — Phase 3 complétée | Projet : <nom>]\nRapport de contexte produit. Passer à la détection des cas particuliers (Phase 4) ?",
    options: [
      { label: "Passer à Phase 4 (Recommandé)", description: "Vérifier les incohérences et cas particuliers" },
      { label: "Réviser le rapport", description: "Rester en Phase 3 pour ajuster le rapport" },
      { label: "Revenir à Phase 1", description: "Explorer à nouveau après avoir produit le rapport" }
    ]
  }]
})
```


**Selon la réponse (dans tous les contextes) :**
- **Passer à Phase 4** → Phase 4
- **Réviser** → rester en Phase 3, ajuster le rapport, re-présenter
- **Revenir à Phase 1** → Phase 1 (le rapport révèle des zones à explorer davantage)

---

## Phase 4 — Détection des cas particuliers

### Objectif
Vérifier les incohérences et cas limites qui pourraient avoir été manqués.

### Ce qu'on vérifie

**Checklist des cas particuliers :**

- ✅ **Incohérences stack/conventions** : La config ESLint dit single quotes mais le code utilise double quotes ?
- ✅ **Dépendances obsolètes avec CVE** : Y a-t-il des dépendances avec des CVE connus (npm audit / pip check) ?
- ✅ **Conventions contradictoires** : Plusieurs conventions de nommage coexistent sans règle claire ?
- ✅ **Architecture hybride non documentée** : Mélange de patterns (MVC + DDD + anémique) sans explication ?
- ✅ **Dette technique masquée** : Code mort, imports circulaires non détectés en Phase 1 ?
- ✅ **Tests flaky** : Tests intermittents signalés dans les logs CI ?

### Déclencheur de pause ⏸️

Si un **cas particulier critique** est détecté (ex : CVE critiques, incohérences majeures) :
- Afficher le contexte en texte (description du cas, impact, options)
- Puis utiliser l'outil `question` pour demander comment le traiter

### Récap de fin de Phase 4


### Question de validation obligatoire


**Si CONTEXTE = standalone :**
```
question({
  questions: [{
    header: "Génération des fichiers",
    question: "[Onboarder — Phase 4 complétée | Projet : <nom>]\nDétection des cas particuliers terminée. Passer à la génération des fichiers (Phase 5) ?",
    options: [
      { label: "Générer le wiki (Recommandé)", description: "Passer à la Phase 5 — Génération du wiki documentaire vivant (docs/wiki/ + ONBOARDING.md minimaliste)" },
      { label: "Vérifier d'autres cas", description: "Rester en Phase 4 pour vérifier d'autres cas particuliers" },
      { label: "Revenir à Phase 3", description: "Revoir le rapport après détection de cas particuliers critiques" }
    ]
  }]
})
```


**Selon la réponse (dans tous les contextes) :**
- **Générer** → Phase 5
- **Vérifier d'autres cas** → rester en Phase 4, vérifier d'autres cas, re-produire le récap
- **Revenir à Phase 3** → Phase 3 (les cas particuliers nécessitent une refonte du rapport)

---