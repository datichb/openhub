---
name: onboarder-phase-5
description: Phase 5 du workflow onboarder — production du wiki documentaire vivant (étapes 5.1 à 5.7 + récap final).
---

## Phase 5 — Production du livrable

**Uniquement après validation explicite.**

Charger le skill `doc-wiki-protocol` avant d'écrire quoi que ce soit — il définit
les formats canoniques de chaque page wiki.

**Vérifier si `docs/wiki/index.md` existe déjà :**
- **Si oui** → mode re-onboarding : appliquer le skill `shared/living-docs-enrichment`
  avec les découvertes du rapport, puis proposer les 3 options (enrichissement incrémental
  recommandé / réécriture complète / conserver). Ne pas passer aux étapes suivantes
  sans confirmation.
- **Si non** → mode création : suivre les étapes 5.1 à 5.5 dans l'ordre.

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
- Ajouter `docs/wiki/` au `.git/info/exclude` (exclusion locale uniquement)
- Ne JAMAIS modifier `.gitignore`

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
3. Dépendances : `package.json` (optimisation RTK : `rtk json package.json --keys-only` puis lecture ciblée)
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

**⚠️ Si des pages `docs/wiki/technical/` existent déjà (re-onboarding) :**
Appliquer le skill `shared/living-docs-enrichment` avec les nouvelles découvertes.

---

### ÉTAPE 5.3 — Créer les pages `docs/wiki/business/`

À partir des artefacts explorés en Phase 1 (modules, routes, entités, bounded contexts),
identifier les domaines métier du projet.

**Workflow :**

1. Analyser la sémantique de la codebase pour déduire les grands domaines
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

Utiliser les formats canoniques du skill `doc-wiki-protocol` pour chaque page créée.

**Après écriture :**
- Créer `docs/wiki/business/` si le dossier n'existe pas
- Le dossier `docs/wiki/` est déjà dans `.git/info/exclude` (ajouté en ÉTAPE 5.1)

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
- Ajouter `ONBOARDING.md` au `.git/info/exclude` si pas déjà présent
- Ne JAMAIS modifier `.gitignore`

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

### ÉTAPE 5.6 — Mise à jour de `projects.md` (optionnelle)

Si le chemin vers `projects.md` est fourni dans le prompt ET que des champs sont absents ou incomplets :

Afficher le contexte en texte et utiliser l'outil `question` :

```
[Texte de réponse]
## Mise à jour projects.md

J'ai détecté que le champ **Stack** est <absent / incomplet / générique> dans projects.md.

**Stack détectée :**
<stack complète détectée en Phase 1>

[Puis appel outil question]
question({
  questions: [{
    header: "Mise à jour projects.md",
    question: "[Onboarder — Phase 5 : projects.md | Projet : <nom>]\nDes champs sont absents ou incomplets dans projects.md. Mettre à jour ?",
    options: [
      { label: "Oui — mettre à jour", description: "Écrire les champs manquants dans projects.md (Stack en priorité)" },
      { label: "Non", description: "Laisser projects.md tel quel" }
    ]
  }]
})
```

**Uniquement si l'utilisateur valide :**
Mettre à jour les champs manquants dans la section du projet concerné dans `projects.md`.
**Ne jamais modifier `projects.md` sans confirmation explicite.**

---

### ÉTAPE 5.7 — Générer le cache de contexte

**Toujours exécuter cette étape après les pages wiki**, sauf si le CONTEXTE initial contient `no-cache: true`.

Générer `.opencode/context.json` avec :

```json
{
  "version": "2.0",
  "generated_at": "<timestamp ISO8601 UTC>",
  "stack": {
    "languages": ["<langages détectés en Phase 1>"],
    "frameworks": ["<frameworks détectés en Phase 1>"]
  },
  "wiki": {
    "source": "docs/wiki/index.md",
    "hash": "<sha256 du fichier docs/wiki/index.md>"
  },
  "key_files": {
    "<fichier_structurant>": "<sha256>",
    "...": "..."
  }
}
```

**Fichiers structurants à inclure dans `key_files` (ceux qui existent dans le projet) :**

```
package.json, tsconfig.json, tsconfig.base.json, pyproject.toml, Cargo.toml,
go.mod, composer.json, pom.xml, Gemfile, requirements.txt,
eslint.config.js, eslint.config.mjs, .eslintrc.json, .prettierrc, .prettierrc.json,
biome.json, docs/wiki/index.md
```

**Calcul des hashes :**
- Utiliser `shasum -a 256 <fichier>` (macOS) ou `sha256sum <fichier>` (Linux)
- Format : `sha256:<empreinte_hex>`

**Protocole d'écriture :**
1. S'assurer que le dossier `.opencode/` existe à la racine du projet (le créer si absent)
2. Écrire `.opencode/context.json`
3. Ajouter `.opencode/context.json` au `.git/info/exclude` (si pas déjà présent)
4. Ne pas modifier `.gitignore`

**Si `.opencode/context.json` existe déjà :**
- L'écraser sans question (mise à jour normale lors d'un re-onboarding)

**Message de confirmation dans la discussion (pas de question outil) :**
```
✅ Cache de contexte généré : .opencode/context.json
   Stack : <langages et frameworks>
   Fichiers indexés : <N fichiers structurants trouvés>
```

---

### Récap de fin de Phase 5


---

### ⚠️ Vérification visuelle — AVANT de terminer la session

**STOP — Question obligatoire à te poser MAINTENANT :**

> « Mon output contient-il du texte en dehors du bloc `## Retour vers orchestrator` ? »
> → **OUI** : STOP — supprimer le texte libre et vérifier que le rapport d'onboarding est DANS le bloc (section `### Rapport d'onboarding`)
> → **NON** : vérifier que tous les éléments ci-dessous sont présents dans le bloc, puis terminer la session

**Vérifications obligatoires dans le bloc :**
- ✅ Section `### Rapport d'onboarding` présente (contexte de découverte, observations qualitatives)
- ✅ Section `### Stack technique` renseignée
- ✅ Section `### Dette technique détectée` documentée (🔴 critiques, 🟠 importants, 🟡 informatifs)
- ✅ Section `### Zones d'incertitude` signalées

> ❌ Ne JAMAIS écrire de texte en dehors du bloc `## Retour vers orchestrator`
> ❌ Ne JAMAIS produire le rapport d'onboarding en texte libre avant le bloc — il est DANS le bloc (section `### Rapport d'onboarding`)
> ✅ Le bloc unique contient toutes les informations : rapport narratif + données structurées

**Si une section est manquante dans le bloc → la compléter MAINTENANT avant de terminer.**

---

### Format de retour final