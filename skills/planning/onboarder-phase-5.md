---
name: onboarder-phase-5
description: "Utiliser avant la phase 5 de l'onboarder quand docs/wiki/index.md n'existe pas : création du wiki (5.1 à 5.6) et récap final."
---

## Phase 5 — Production du livrable (création du wiki)

<!-- oh:when role=entry -->
**Uniquement après validation explicite.**
<!-- /oh:when -->
<!-- oh:when role=subagent -->
La demande de ton appelant vaut validation : écris le wiki, sauf si ton prompt demande de t'arrêter avant.
<!-- /oh:when -->

Charger le skill `doc-wiki-protocol` avant d'écrire quoi que ce soit — il définit
les formats canoniques de chaque page wiki.

`docs/wiki/index.md` existe déjà → c'est un re-onboarding : n'écris rien, charge `onboarder-phase-5-reonboarding` à la place. Sinon → mode création : suivre les étapes 5.1 à 5.5 dans l'ordre.

---

### ÉTAPE 5.1 — Créer `docs/wiki/index.md`

**Format canonique (voir skill `doc-wiki-protocol` — section `docs/wiki/index.md`) :**

```markdown
---
updated: <DATE>
confidence: confirmed
agents: [onboarder]
---

# <NOM_PROJET> — Index Wiki

## Stack critique
<3-5 lignes condensées : langages, frameworks principaux, BDD, infra — les éléments
qui conditionnent tout le reste>
— `CONFIRMÉ` · onboarder · <DATE> · package.json

## Architecture (résumé)
<2-3 lignes : pattern dominant, découpage, communication entre couches>
— `CONFIRMÉ` · onboarder · <DATE>

## God nodes — concepts les plus connectés

| Concept | Pages liées | Criticité |
|---------|-------------|-----------|

*(Rempli après génération des autres pages — voir ÉTAPE 5.6)*

## Carte des domaines métier

- [<domain>](business/<domain>.md) — <description courte>

*(Vide si aucun domaine métier détecté)*

## Points critiques actifs 🔴

<Points critiques détectés en Phase 3 — vide si aucun>
— `CONFIRMÉ` · onboarder · <DATE> · <fichier:ligne si disponible>

## Zones d'ombre

<Ce qui n'a pas pu être déterminé — vide si tout est documenté>
```

**Après écriture :**
- Créer `docs/wiki/` si le dossier n'existe pas
- Ne JAMAIS modifier `.gitignore` ni `.git/` : si le wiki ne doit pas être versionné, le proposer à l'utilisateur dans la passation (ajout de `docs/wiki/` à son `.gitignore`)

---

### ÉTAPE 5.2 — Créer les pages `docs/wiki/technical/`

Créer les 4 pages technical en utilisant les formats canoniques du skill `doc-wiki-protocol`.

**`docs/wiki/technical/stack.md`** — à partir des données Phase 1 :
- Tableau des dépendances principales avec versions (depuis `package.json` ou équivalent)
- Librairies clés et leur rôle
- Variables d'environnement requises (depuis `.env.example`)
- Tag `CONFIRMÉ` sur chaque item avec référence `package.json`

**`docs/wiki/technical/architecture.md`** — à partir de l'analyse Phase 1 :
- Structure globale (monorepo / monolithe / microservices)
- Découpage en couches observé
- Communication entre modules
- Décisions architecturales notables si documentées (ADR)
- Points de fragilité détectés (🔴/🟠 de Phase 3 liés à l'architecture)

**`docs/wiki/technical/tests.md`** — à partir de l'analyse Phase 1.6 :
- Frameworks (unitaires + E2E)
- Organisation (co-localisés / séparés)
- Seuil de couverture (depuis config)
- Philosophie (TDD / BDD / test-after)
- Commandes (depuis `package.json`)
- Conventions de nommage observées

**`docs/wiki/technical/conventions.md`** — à partir de l'analyse Phase 5 (détection conventions) :

Le protocole de détection des conventions reste identique à l'ancien workflow.
Les sections à générer correspondent au format canonique du skill `doc-wiki-protocol` :

```
## Linting & formatage    ← depuis .eslintrc*, biome.json, .prettierrc*, etc.
## Nommage                ← inféré de 5-10 fichiers représentatifs
## Git                    ← depuis .commitlintrc, git log, CONTRIBUTING.md
## Configuration & secrets ← depuis .env.example
## Patterns spécifiques à l'équipe ← depuis CONTRIBUTING.md, ADR, patterns observés
## À ne pas utiliser      ← librairies ou patterns explicitement exclus
```

**Détection des conventions — protocole complet (identique à l'ancien ÉTAPE 5.2) :**

Lire dans l'ordre :
1. Config linting : `.eslintrc*`, `eslint.config.*`, `.prettierrc*`, `biome.json`, `ruff.toml`
2. Config TypeScript/langage : `tsconfig.json`, `pyproject.toml`
3. Dépendances : `package.json` (lecture ciblée des dépendances ; avec le plugin RTK : `rtk json package.json --keys-only`)
4. Git : `.commitlintrc`, `.husky/`, `CONTRIBUTING.md`, `git log --oneline -20`
5. Nommage : 5-10 fichiers représentatifs dans `src/components/`, `src/services/`, `src/stores/`
6. Architecture : structure `src/`
7. Tests : `vitest.config.ts`, `jest.config.ts`, `pytest.ini`
8. Config/secrets : `.env.example`
9. Patterns équipe : `CONTRIBUTING.md`, `README.md`, `docs/`, `adr/`

Chaque convention détectée porte le tag de confiance approprié :
- Convention issue d'un fichier config → `CONFIRMÉ` avec le fichier config
- Convention inférée de la codebase → `CONFIRMÉ` avec exemple de fichier:ligne
- Convention supposée → `DÉDUIT` avec le fichier source

**`docs/wiki/glossary.md`** (racine du wiki) — **optionnel**, créer uniquement si des termes de domaine ambigus sont détectés en Phase 1 (entités avec noms similaires, abréviations non évidentes, termes métier spécifiques au projet), au format du glossaire de `doc-wiki-protocol` (`- **Terme** : définition`, tagué). Sinon, ne pas la créer : un agent la proposera plus tard.

ADR existants du projet (`docs/adr/`, `adr/`…) : ne pas les déplacer, les citer dans `## Décisions architecturales notables` d'`architecture.md`. `specs/`, `postmortems/` et `retros/` se créent plus tard (workflows `spec`, `incident` et de rétrospective) : pas de dossier vide.

**`docs/wiki/decisions/decisions-rejetees.md`** — toujours créer cette page (même vide) pour que les agents puissent la consulter et l'enrichir :

```markdown
---
updated: <YYYY-MM-DD>
confidence: confirmed
agents: [onboarder]
---

# Décisions rejetées

> Cette page trace les concepts, approches et fonctionnalités explicitement rejetés
> avec la raison du rejet. Consultée par le planner en Phase 4 pour éviter de
> re-proposer des concepts déjà rejetés.

## Entrées

*(Aucune décision rejetée enregistrée pour le moment.)*

<!-- Format pour chaque entrée :
### [Titre du concept rejeté]
- **Date :** YYYY-MM-DD
- **Raison :** <pourquoi ce concept a été rejeté>
- **Tickets liés :** <IDs Beads si applicable>
- **Statut :** ACTIVE | EXPIRED | SUPERSEDED
- **Contexte de réévaluation :** <dans quelles conditions reconsidérer>
— `CONFIRMÉ` · <agent> · YYYY-MM-DD
-->
```

> **Cycle de vie des entrées :**
> - `ACTIVE` : le rejet tient toujours — le planner filtre uniquement sur ce statut
> - `EXPIRED` : le contexte a changé, le rejet n'est plus pertinent — proposé par l'onboarder en re-onboarding (`wiki_propose`) si l'entrée a > 6 mois ou si le contexte technique a évolué
> - `SUPERSEDED` : remplacé par une autre décision — avec référence au ticket/ADR qui a décidé autrement

---

### ÉTAPE 5.3 — Créer les pages `docs/wiki/business/`

À partir des artefacts explorés en Phase 1 (modules, routes, entités, bounded contexts),
identifier les domaines métier du projet.

**Workflow :**

1. Analyser la sémantique de la codebase pour déduire les grands domaines
<!-- oh:when role=entry -->
2. Afficher la proposition en texte :

```
## 🗂️ Domaines métier détectés

J'ai identifié les domaines suivants pour ce projet. Souhaitez-vous ajuster le découpage ?

- `<domain-1>` — <périmètre fonctionnel détecté>
- `<domain-2>` — <périmètre fonctionnel détecté>
```

3. Appeler l'outil `question` :

```
question({
  questions: [{
    header: "Domaines métier",
    question: "[Onboarder — Phase 5 : domaines métier | Projet : <nom>]\nJ'ai détecté <N> domaines métier. Valider ce découpage pour créer les pages docs/wiki/business/ ?",
    options: [
      { label: "Valider ce découpage (Recommandé)", description: "Créer une page par domaine dans docs/wiki/business/" },
      { label: "Modifier les domaines", description: "Ajuster les noms ou le périmètre avant création" },
      { label: "Passer", description: "Créer uniquement docs/wiki/business/general.md avec le contexte métier global" }
    ]
  }]
})
```

4. Selon la réponse :
   - **Valider** → Créer `docs/wiki/business/index.md` + `docs/wiki/business/<domain>.md` pour chaque domaine
   - **Modifier** → Intégrer les ajustements de l'utilisateur, puis créer les fichiers
   - **Passer** → Créer uniquement `docs/wiki/business/general.md`
<!-- /oh:when -->
<!-- oh:when role=subagent -->
2. Créer `docs/wiki/business/index.md` et une page `docs/wiki/business/<domain>.md` par domaine détecté, sans question ; le découpage retenu va dans `### Contexte métier` (hypothèse à valider par ton appelant).
<!-- /oh:when -->

Utiliser les formats canoniques du skill `doc-wiki-protocol` pour chaque page créée.

**Après écriture :**
- Créer `docs/wiki/business/` si le dossier n'existe pas

---

### ÉTAPE 5.4 — Créer `ONBOARDING.md` minimaliste à la racine

```markdown
# <NOM_PROJET>

> Documentation vivante disponible dans [`docs/wiki/index.md`](docs/wiki/index.md)

## Démarrage rapide

<Commandes de démarrage détectées en Phase 1 — 1-3 lignes maximum>

## Liens

- [Index wiki](docs/wiki/index.md) — vue globale, god nodes, points critiques
- [Conventions](docs/wiki/technical/conventions.md)
- [Architecture](docs/wiki/technical/architecture.md)
```

**Règles :** 15-25 lignes maximum. Ne pas dupliquer le contenu du wiki.

**⚠️ Si `ONBOARDING.md` existe déjà :**
Le remplacer par la version minimaliste — il s'agit d'un changement de format intentionnel
(rupture propre vers le wiki). Afficher en texte avant de procéder :

```
## ⚠️ ONBOARDING.md existant — remplacement par version minimaliste

L'ancien ONBOARDING.md sera remplacé par une version minimaliste qui redirige
vers le wiki documentaire vivant (docs/wiki/index.md).
```

**Après écriture :**
- Ne JAMAIS modifier `.gitignore` ni `.git/`

---

### ÉTAPE 5.5 — Mise à jour de `docs/wiki/index.md` — God nodes

Après avoir créé toutes les pages wiki, réévaluer le tableau des god nodes dans `index.md`.

Appliquer l'algorithme du skill `wiki-navigation` :
1. Recenser les concepts mentionnés dans chaque page créée
2. Identifier les concepts cités dans ≥ 2 pages distinctes → candidats god nodes
3. Remplir le tableau des god nodes avec les pages liées et la criticité
4. Mettre à jour le frontmatter `updated`

Si aucun concept n'apparaît dans ≥ 2 pages → laisser le tableau vide avec la note `*(Vide)*`.

---

### ÉTAPE 5.6 — Fin des écritures

Aucune autre écriture : ni `projects.md`, ni cache de contexte (`.opencode/context.json` n'est plus lu par oh), ni `.git/info/exclude`. Tes permissions refusent tout fichier hors de `docs/wiki/`, `ONBOARDING.md` et `CONVENTIONS.md`. Le commit du wiki vient après la validation de la carte de commit (prompt du workflow) ; celui d'`ONBOARDING.md` et de `CONVENTIONS.md` revient à l'utilisateur.

---

### ⚠️ Vérification — AVANT de terminer la session

**Vérifications obligatoires dans ton rapport final :**
- ✅ Section `### Rapport d'onboarding` présente (contexte de découverte, observations qualitatives)
- ✅ Section `### Stack technique` renseignée
- ✅ Section `### Dette technique détectée` documentée (🔴 critiques, 🟠 importants, 🟡 informatifs)
- ✅ Section `### Zones d'incertitude` signalées

<!-- oh:when role=subagent -->
Le rapport est déposé (`workflow_report`), puis ta réponse finale est l'enveloppe ; aucun texte libre à côté.
<!-- /oh:when -->

**Si une section est manquante → la compléter MAINTENANT avant de terminer.**
