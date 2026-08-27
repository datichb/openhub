---
name: onboarder-phase-1
description: Phase 1 du workflow onboarder — exploration contextuelle adaptative (stack, profil, tickets, contexte métier, Figma, tests).
---

## Phase 1 — Exploration contextuelle

### Objectif
Explorer le projet de manière adaptative selon la stack détectée.

### ÉTAPE 1.1 — Détecter la stack

**Annoncer avant d'explorer :**
> "Je vais lire les fichiers de configuration à la racine pour identifier la stack."

Lire dans cet ordre (s'arrêter dès que suffisant) :

#### Manifestes de dépendances

```
package.json          → Node.js / JavaScript / TypeScript
pyproject.toml        → Python (Poetry, PDM, Hatch)
requirements.txt      → Python (pip classique)
go.mod                → Go
Gemfile               → Ruby
composer.json         → PHP
Cargo.toml            → Rust
pom.xml               → Java / Kotlin (Maven)
build.gradle          → Java / Kotlin (Gradle)
mix.exs               → Elixir
```

#### Tooling et versions

```
.tool-versions        → versions exactes (asdf)
.nvmrc / .node-version → version Node.js
.python-version       → version Python
```

#### CI / CD

```
.github/workflows/    → GitHub Actions (lire les fichiers *.yml)
.gitlab-ci.yml        → GitLab CI
Jenkinsfile           → Jenkins
.circleci/config.yml  → CircleCI
```

#### Infra et conteneurisation

```
docker-compose.yml / docker-compose.yaml  → services, bases de données
Dockerfile                                → image de base, runtime
terraform/                                → infrastructure as code
k8s/ ou kubernetes/ ou manifests/         → orchestration Kubernetes
helm/                                     → Helm charts
```

#### Détection du profil applicatif

À partir des dépendances lues :

| Dépendance détectée | Profil |
|--------------------|--------|
| `vue`, `@vue/core` | Frontend Vue.js |
| `react`, `react-dom` | Frontend React |
| `@angular/core` | Frontend Angular |
| `next` | Frontend Next.js (SSR) |
| `nuxt` | Frontend Nuxt.js (SSR) |
| `express`, `fastify`, `koa`, `hapi` | Backend Node.js |
| `@nestjs/core` | Backend NestJS |
| `django`, `flask`, `fastapi` | Backend Python |
| `laravel`, `symfony` | Backend PHP |
| `rails` | Backend Ruby on Rails |
| `dbt-core`, `apache-airflow`, `pyspark` | Data / ML |
| `react-native`, `expo` | Mobile React Native |
| `flutter` (pubspec.yaml) | Mobile Flutter |
| `graphql`, `@apollo/server`, `strawberry` | API GraphQL |
| `openapi`, `swagger-ui` | API REST documentée |

**Profil fullstack** : si frontend ET backend sont détectés dans le même dépôt (monorepo).

### ÉTAPE 1.2 — Explorer adaptativement selon le profil

Une fois le profil identifié, cibler les fichiers structurants.
**Annoncer ce qui va être lu avant chaque section.**

#### Profil Frontend Vue.js

SI profil détecté → charger le skill `planning/onboarder-profiles` via `skill` pour les fichiers structurants à explorer selon le profil applicatif.

### ÉTAPE 1.3 — Lire les tickets Beads et ADRs

```bash
# Tickets ouverts — état du backlog
bd list -s open --json

# Tickets récemment clos — ce qui vient d'être livré
bd list -s closed --limit 10 --json
```

Identifier :
- Y a-t-il des tickets de dette / bug / chore non traités en nombre inhabituel ?
- Y a-t-il des patterns récurrents (ex: concentration de bugs sur un module) ?

---

### ÉTAPE 1.4 — Exploration du contexte métier

**Annoncer avant d'explorer :**
> "Je vais analyser le contexte métier du projet pour identifier le domaine et les concepts clés."

#### Analyse du README et de la documentation

Lire dans cet ordre :

```
README.md                              → description, domaine, utilisateurs
docs/glossary.md ou GLOSSARY.md        → terminologie métier
docs/domain/ ou docs/business/         → documentation métier
CONTRIBUTING.md                        → contexte contributeurs
adr/ ou docs/architecture/adr/         → décisions métier
```

#### Analyse sémantique de la codebase

Lire la structure et identifier les patterns :

**Détection du domaine :**

Rechercher des mots-clés de domaines connus dans README.md et le nom du projet :

- **E-commerce** : cart, checkout, payment, product, inventory, shipping, order, catalog
- **Fintech** : transaction, account, balance, transfer, compliance, kyc, wallet, payment
- **Santé** : patient, practitioner, appointment, prescription, diagnosis, medical
- **RH** : employee, payroll, leave, performance, recruitment, timesheet
- **SaaS** : tenant, subscription, billing, feature-flag, organization, plan
- **Éduc** : student, course, lesson, grade, enrollment, teacher
- **Immobilier** : property, listing, rental, lease, tenant, owner

**Extraction des concepts métier :**

Lire 10-15 fichiers représentatifs pour identifier les concepts :

```
src/domain/ ou src/models/ ou src/entities/
src/services/ (noms des services)
src/types/ ou src/interfaces/ (types métier)
src/repositories/ (noms des repositories)
```

Pour chaque fichier, extraire :
- Les noms de classes/interfaces (ex : `class User`, `interface Product`, `type Order`)
- Les concepts récurrents (≥ 3 occurrences dans différents fichiers)
- Les patterns d'architecture (DDD détecté si répertoires `domain/`, `entities/`, `value-objects/`)

**Détection des utilisateurs cibles :**

Rechercher dans README.md :
- "utilisateur", "user", "client", "admin", "administrator"
- "pour les", "à destination de", "conçu pour"
- Identifier les rôles dans le code (`UserRole`, `permissions`, `roles`)

#### Tickets Beads (analyse complémentaire)

Si Beads est initialisé (déjà exploré en ÉTAPE 1.3) :

Analyser les 20 derniers tickets (ouverts + clos) pour identifier :
- Les features récurrentes (patterns métier visibles)
- Les concepts mentionnés dans les titres/descriptions
- Les labels métier custom (`epic:payment`, `domain:user`, etc.)

#### Récap contexte métier

Produire un résumé structuré :

```markdown
**Contexte métier détecté :**
- Domaine(s) : <liste> (ou "Non identifié — projet générique")
- Utilisateurs cibles : <liste> (ou "Non documentés")
- Concepts clés : <liste des concepts détectés> (X concepts récurrents)
- Glossaire : <Présent dans docs/glossary.md (Y termes) / Absent>
- Pattern architecture : <DDD / CQRS / Layered / MVC / Non documenté>
- Features métier récurrentes (Beads) : <patterns identifiés ou "Aucun pattern visible">
```

**Si aucun contexte métier détectable :**
```markdown
**Contexte métier détecté :**
- Non documenté — recommandation : créer docs/glossary.md et documenter les concepts clés dans README
```

---

### ÉTAPE 1.5 — Exploration Figma (optionnelle)

**Déclencheur :**

Lancer uniquement si :
- Le profil détecté en Phase 1.1 contient "Frontend" (Vue.js, React, Angular, etc.)
- OU des composants UI sont présents dans `src/components/` ou équivalent

**Si pas de frontend détecté → skipper Phase 1.5, passer à 1.6.**

**Annoncer avant d'explorer :**
> "Je vais rechercher les maquettes Figma liées au projet."

#### Recherche des fichiers Figma

Stratégie de recherche progressive — s'arrêter à la première tentative qui retourne des résultats :

**Tentative 1 :** `search_figma_files(<nom depuis package.json "name"> ou <nom du dossier racine>)`

**Tentative 2 (si aucun résultat) :** `search_figma_files(<ID du projet — disponible dans le bootstrap prompt>)`

**Tentative 3 (si aucun résultat) :** `search_figma_files(<champ "Nom" du projet — disponible dans le bootstrap prompt>)`

**Si toujours aucun résultat après les 3 tentatives :**

Afficher :
> "J'ai recherché les fichiers Figma avec les termes [terme1], [terme2], [terme3] — aucun résultat."

Puis appeler `question` :

```
question({
  questions: [{
    header: "Fichiers Figma",
    question: "[Onboarder — Phase 1.5 | Projet : <nom>]\nJe n'ai trouvé aucun fichier Figma pour les termes suivants :\n- [terme1]\n- [terme2]\n- [terme3]\n\nComment procéder ?",
    options: [
      { label: "Fournir le nom du fichier", description: "Préciser le nom exact ou l'URL du fichier Figma à analyser" },
      { label: "Pas de maquettes Figma", description: "Ce projet n'a pas de maquettes Figma — passer à Phase 1.6" },
      { label: "Ignorer pour l'instant", description: "Continuer l'onboarding sans les maquettes" }
    ]
  }]
})
```

**Si fichier(s) trouvé(s) (quelle que soit la tentative) :**
→ Continuer vers analyse

#### Analyse des fichiers Figma (max 3 fichiers pertinents)

Pour chaque fichier :

```
get_file_structure(fileId)
→ Obtenir : nom, pages, nombre de composants, date de modification

detect_ui_signals(fileId)
→ Obtenir : complexité, signaux UX/UI

extract_design_tokens(fileId)
→ Obtenir : tokens couleur, typo, spacing, effects
```

#### Identification du design system

**Critères de détection :**
- Fichier Figma nommé "*Design System*" ou "*DS*" ou "*Components*"
- Présence de tokens structurés dans Figma Variables
- Composants nommés selon une convention (DSFR*, Material*, Ant*, Custom*)

**Si design system détecté :**
- Lister les composants principaux (max 10)
- Extraire les design tokens
- Identifier le framework (DSFR / Material / Ant Design / Custom)

**Si pas de design system :**
- Mentionner "Pas de design system centralisé détecté dans Figma"

#### Récap Figma

```markdown
**Maquettes Figma détectées :**
- Fichiers trouvés : X fichiers
  - [Nom fichier 1](URL)
  - [Nom fichier 2](URL)
- Design system : <Oui — Framework : DSFR / Non>
  - Composants disponibles : <liste>
- Design tokens : <X tokens couleur, Y tokens typo, Z tokens spacing / Non configurés>
```

---

### ÉTAPE 1.6 — Exploration de la stratégie de test

**Annoncer avant d'explorer :**
> "Je vais analyser la stratégie de test du projet."

#### Détection des frameworks de test

Lire les fichiers de configuration :

```
vitest.config.ts / vitest.config.js    → Vitest
jest.config.ts / jest.config.js        → Jest
pytest.ini / pyproject.toml            → pytest
phpunit.xml / phpunit.xml.dist         → PHPUnit
playwright.config.ts                   → Playwright (E2E)
cypress.config.ts / cypress.json       → Cypress (E2E)
karma.conf.js                          → Karma (Angular)
```

**Extraire :**
- Nom du framework unitaire
- Nom du framework E2E (si présent)
- Seuil de couverture configuré (`coverage.threshold` ou équivalent)

#### Analyse de l'organisation des tests

Explorer la structure :

```
tests/ ou __tests__/ ou spec/         → dossier dédié
*.test.ts ou *.spec.ts à côté du code → co-localisés
```

**Calculer le ratio test/source :**

```bash
# Compter les fichiers test
find src -name "*.test.*" -o -name "*.spec.*" | wc -l

# Compter les fichiers source (hors tests)
find src -name "*.ts" -o -name "*.js" -o -name "*.py" | grep -v test | grep -v spec | wc -l
```

Interpréter :
- Ratio ≥ 0.8 : Bonne couverture
- Ratio 0.4-0.8 : Couverture partielle
- Ratio < 0.4 : Couverture faible

#### Détection de la philosophie de test

**Indicateurs TDD :**
- Fichiers test créés avant les fichiers source (git log --diff-filter=A)
- Labels Beads `tdd` présents dans les tickets
- Mention "TDD" ou "Test-Driven Development" dans README ou CONTRIBUTING

**Indicateurs BDD :**
- Framework Cucumber / Behave détecté
- Fichiers `.feature` présents
- Syntaxe `Given/When/Then` dans les tests

**Par défaut :**
- "Test-after" — tests écrits après implémentation

#### Récap stratégie de test

```markdown
**Stratégie de test détectée :**
- Frameworks :
  - Unitaires : <Vitest / Jest / pytest / PHPUnit>
  - E2E : <Playwright / Cypress / Aucun>
- Organisation : <Co-localisés (.spec.ts à côté du code) / Dossier tests/ séparé>
- Seuil de couverture : <X% configuré dans vitest.config.ts / Non configuré>
- Ratio test/source : <calculé> — <Bonne / Partielle / Faible> couverture
- Philosophie : <TDD (labels Beads détectés) / BDD (Cucumber) / Test-after>
- Commandes :
  - Tests unitaires : `<npm test / pytest / ...>`
  - Tests E2E : `<npm run test:e2e / ...>`
  - Couverture : `<npm run test:coverage / ...>`
```

---

### Déclencheur de pause ⏸️

**STOP OBLIGATOIRE mid-phase** — Si l'une des conditions suivantes est rencontrée pendant l'exploration, arrêter immédiatement et appeler l'outil `question` avant de continuer :

- Dépendance critique non documentée (version majeure bloquante, breaking change visible)
- Contradiction directe entre deux fichiers de configuration, conventions ou architectures
- Architecture non identifiable après lecture des fichiers structurants principaux
- Accès impossible à un répertoire ou fichier structurant (permissions, fichier absent)
- Décision d'architecture ou de stratégie qui rendrait le rapport incomplet sans clarification

> Afficher le contexte en texte clair (ce qui a été trouvé, pourquoi c'est bloquant), puis appeler `question`. Reprendre l'exploration après réponse.

### Récap de fin de Phase 1


### Question de validation obligatoire


**Si CONTEXTE = standalone :**
```
question({
  questions: [{
    header: "Questions complémentaires",
    question: "[Onboarder — Phase 1 complétée | Projet : <nom>]\nPasser aux questions complémentaires (Phase 2) ?",
    options: [
      { label: "Passer à Phase 2 (Recommandé)", description: "Poser les questions de clarification identifiées" },
      { label: "Explorer davantage", description: "Lire d'autres fichiers avant de poser des questions" }
    ]
  }]
})
```


**Selon la réponse (dans tous les contextes) :**
- **Passer à Phase 2** → Phase 2
- **Explorer davantage** → rester en Phase 1, explorer plus, re-produire le récap

---