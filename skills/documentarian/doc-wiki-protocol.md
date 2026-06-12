---
name: doc-wiki-protocol
description: Format canonique du wiki documentaire vivant — structure de chaque page, frontmatter obligatoire, tags de confiance, règles d'enrichissement incrémental, algorithme de mise à jour des god nodes, templates de création de nouvelles pages.
---

# Skill — Protocole du Wiki Documentaire Vivant

## Rôle

Ce skill définit le format exact de chaque page du wiki et les règles d'écriture
que le `documentarian` doit respecter pour maintenir la cohérence du système.

Il est chargé par le `documentarian` uniquement lors d'opérations d'écriture sur le wiki.

---

## Structure du wiki

```
docs/wiki/
├── index.md                    ← carte globale obligatoire
├── technical/
│   ├── architecture.md         ← patterns dominants, découpage, décisions structurantes
│   ├── stack.md                ← stack complète, versions, librairies clés
│   ├── tests.md                ← stratégie, conventions, seuils, frameworks
│   └── conventions.md          ← nommage, git, linting, config, patterns équipe
└── business/
    ├── index.md                ← carte des domaines métier
    └── <domain>.md             ← règles de gestion, flux, entités, risques
```

À la racine du projet :
```
ONBOARDING.md                   ← minimaliste, redirige vers docs/wiki/index.md
```

---

## Frontmatter obligatoire

Chaque page wiki doit commencer par un frontmatter YAML :

```yaml
---
updated: YYYY-MM-DD
confidence: confirmed | mixed | uncertain
agents: [onboarder, developer, reviewer, ...]
---
```

| Champ | Valeurs | Description |
|-------|---------|-------------|
| `updated` | Date ISO | Date de la dernière modification de la page |
| `confidence` | `confirmed` / `mixed` / `uncertain` | Niveau de confiance dominant des enrichissements de la page |
| `agents` | Liste | Agents qui ont contribué à cette page |

**Règle :** mettre à jour `updated` et `agents` à chaque enrichissement de la page.
La valeur `confidence` reflète le niveau le plus bas présent dans la page
(`uncertain` prime sur `mixed`, `mixed` prime sur `confirmed`).

---

## Format des tags de confiance

Chaque enrichissement doit porter un tag de confiance inline en fin de ligne ou de bloc :

### Syntaxe complète

```markdown
- <Description de l'observation>
  — `CONFIRMÉ` · <agent> · <YYYY-MM-DD> · <fichier:ligne>

- <Description d'une observation déduite>
  — `DÉDUIT` · <agent> · <YYYY-MM-DD> · <fichier>

- <Description d'une information incertaine>
  — `INCERTAIN` · <agent> · <YYYY-MM-DD>
```

### Les 3 niveaux

| Tag | Quand l'utiliser | Fichier source |
|-----|-----------------|----------------|
| `` `CONFIRMÉ` `` | Observation directe dans le code — ligne précise identifiée | `fichier:ligne` obligatoire |
| `` `DÉDUIT` `` | Raisonnement contextuel depuis plusieurs fichiers — ligne non précise | `fichier` recommandé |
| `` `INCERTAIN` `` | Hypothèse, information venant d'un commentaire ou d'une convention non codée | Fichier optionnel |

### Exemples concrets

```markdown
- Les tokens JWT expirent après 15min, refresh après 7j
  — `CONFIRMÉ` · developer · 2026-01-15 · src/auth/jwt.service.ts:42

- L'architecture suit un pattern Controller → Service → Repository
  — `DÉDUIT` · onboarder · 2026-01-10 · src/

- Les migrations sont appliquées manuellement en pré-déploiement
  — `INCERTAIN` · reviewer · 2026-01-20
```

---

## Format canonique — `docs/wiki/index.md`

```markdown
---
updated: YYYY-MM-DD
confidence: confirmed | mixed | uncertain
agents: [<agents ayant contribué>]
---

# [Nom du projet] — Index Wiki

## Stack critique
<3-5 lignes condensées : langages, frameworks principaux, BDD — les éléments
qui conditionnent tout le reste>
— `CONFIRMÉ` · <agent> · <date> · <fichier>

## Architecture (résumé)
<2-3 lignes : pattern dominant, découpage, communication entre couches>
— `CONFIRMÉ` · <agent> · <date>

## God nodes — concepts les plus connectés

| Concept | Pages liées | Criticité |
|---------|-------------|-----------|
| <Concept A> | [technical/architecture.md#section](), [business/domain.md]() | Critique |
| <Concept B> | [technical/conventions.md#section](), [business/other.md]() | Haute |

*(Vide si aucun concept n'apparaît dans ≥ 2 pages)*

## Carte des domaines métier

- [<domain>](business/<domain>.md) — <description courte en 1 phrase>

*(Vide si aucun domaine métier détecté)*

## Points critiques actifs 🔴

- <Point critique 1> — `CONFIRMÉ` · <agent> · <date>

*(Vide si aucun point critique actif)*

## Zones d'ombre

- <Zone non résolue 1>
- <Zone non résolue 2>

*(Vide si tout est documenté)*
```

**Règles spécifiques à `index.md` :**
- C'est la seule page que TOUS les agents lisent systématiquement
- Elle doit rester compacte — 40-80 lignes maximum hors frontmatter
- Ne jamais dupliquer le contenu détaillé des sous-pages
- Les liens vers les sous-pages sont obligatoires dans la table des god nodes

---

## Format canonique — `docs/wiki/technical/conventions.md`

```markdown
---
updated: YYYY-MM-DD
confidence: confirmed | mixed | uncertain
agents: [<agents>]
---

# Conventions — [Nom du projet]

## Linting & formatage
- <Convention 1> — `CONFIRMÉ` · <agent> · <date> · <fichier config>
- <Convention 2> — `DÉDUIT` · <agent> · <date>

## Nommage
- <Convention fichiers/dossiers> — `CONFIRMÉ` · <agent> · <date>
- <Convention variables/fonctions> — `CONFIRMÉ` · <agent> · <date>
- <Convention classes/interfaces> — `CONFIRMÉ` · <agent> · <date>

## Git
- <Format des commits> — `CONFIRMÉ` · <agent> · <date>
- <Convention de branches> — `DÉDUIT` · <agent> · <date>

## Configuration & secrets
- <Convention .env / secrets> — `CONFIRMÉ` · <agent> · <date>

## Patterns spécifiques à l'équipe
- <Pattern observé 1> — `CONFIRMÉ` · <agent> · <date> · <fichier:ligne>
- <Pattern observé 2> — `DÉDUIT` · <agent> · <date>

## À ne pas utiliser
- <Librairie/pattern explicitement exclu> — `CONFIRMÉ` · <agent> · <date>
```

---

## Format canonique — `docs/wiki/technical/architecture.md`

```markdown
---
updated: YYYY-MM-DD
confidence: confirmed | mixed | uncertain
agents: [<agents>]
---

# Architecture — [Nom du projet]

## Structure globale
<Monorepo / monolithe / microservices — avec justification observée>
— `CONFIRMÉ` · <agent> · <date>

## Découpage en couches
<Description du découpage observé — ex: Controller → Service → Repository>
— `CONFIRMÉ` · <agent> · <date> · <dossier>

## Communication entre modules
<HTTP, événements, queues, imports directs — selon ce qui est observé>
— `DÉDUIT` · <agent> · <date>

## Décisions architecturales notables
- <Décision 1 : pourquoi ce pattern a été choisi si documenté>
  — `CONFIRMÉ` · <agent> · <date> · <ADR ou fichier>
- <Décision 2>
  — `INCERTAIN` · <agent> · <date>

## Points de fragilité connus
- <Zone fragile 1> — `CONFIRMÉ` · <agent> · <date> · <fichier>
```

---

## Format canonique — `docs/wiki/technical/stack.md`

```markdown
---
updated: YYYY-MM-DD
confidence: confirmed | mixed | uncertain
agents: [<agents>]
---

# Stack — [Nom du projet]

## Dépendances principales

| Catégorie | Technologie | Version | Notes |
|-----------|-------------|---------|-------|
| Langage | TypeScript | 5.x | |
| Framework | Vue 3 | 3.4.x | |
| BDD | PostgreSQL | 15 | ORM : Prisma |

— `CONFIRMÉ` · <agent> · <date> · package.json

## Librairies clés

- `<lib>` — <rôle dans le projet> — `CONFIRMÉ` · <agent> · <date>
- `<lib>` — <rôle dans le projet> — `CONFIRMÉ` · <agent> · <date>

## Variables d'environnement requises

- `<VAR>` — <description> — `CONFIRMÉ` · <agent> · <date> · .env.example

## Contraintes de version

- <Contrainte 1 si applicable> — `CONFIRMÉ` · <agent> · <date>
```

---

## Format canonique — `docs/wiki/technical/tests.md`

```markdown
---
updated: YYYY-MM-DD
confidence: confirmed | mixed | uncertain
agents: [<agents>]
---

# Stratégie de tests — [Nom du projet]

## Frameworks

- Unitaires : <framework> — `CONFIRMÉ` · <agent> · <date> · <fichier config>
- E2E : <framework ou "Aucun"> — `CONFIRMÉ` · <agent> · <date>

## Organisation

- <Co-localisés `.spec.ts` à côté du code / dossier `tests/` séparé>
  — `CONFIRMÉ` · <agent> · <date>

## Seuil de couverture

- <X% configuré dans <fichier> / Non configuré>
  — `CONFIRMÉ` · <agent> · <date> · <fichier config>

## Philosophie

- <TDD / test-after / BDD — avec signe observé>
  — `DÉDUIT` · <agent> · <date>

## Commandes

```bash
# Tests unitaires
<commande>

# Tests E2E
<commande ou "Non configuré">

# Couverture
<commande ou "Non configuré">
```
— `CONFIRMÉ` · <agent> · <date> · package.json

## Conventions de nommage des tests

- <Convention observée> — `CONFIRMÉ` · <agent> · <date> · <fichier exemple>
```

---

## Format canonique — `docs/wiki/business/index.md`

```markdown
---
updated: YYYY-MM-DD
confidence: confirmed | mixed | uncertain
agents: [<agents>]
---

# Domaines métier — [Nom du projet]

## Vue d'ensemble

<2-3 phrases décrivant le domaine principal et les utilisateurs cibles>
— `CONFIRMÉ` · <agent> · <date>

## Domaines documentés

| Domaine | Fichier | Concepts clés |
|---------|---------|---------------|
| <domain> | [business/<domain>.md](business/<domain>.md) | <Concept A, Concept B> |

## Concepts transversaux

- <Concept présent dans plusieurs domaines> — lié à [<domain1>](business/<domain1>.md) et [<domain2>](business/<domain2>.md)
  — `CONFIRMÉ` · <agent> · <date>

## Utilisateurs cibles

- <Rôle utilisateur 1> — <description courte>
  — `CONFIRMÉ` · <agent> · <date>
```

---

## Format canonique — `docs/wiki/business/<domain>.md`

```markdown
---
updated: YYYY-MM-DD
confidence: confirmed | mixed | uncertain
agents: [<agents>]
---

# [Domaine] — [Nom du projet]

## Rôle de ce domaine
<1-2 phrases : ce que ce domaine gère, son périmètre dans le projet>
— `CONFIRMÉ` · <agent> · <date>

## Entités clés

- `<Entité>` — <description courte + où elle est définie>
  — `CONFIRMÉ` · <agent> · <date> · <fichier>

## Règles de gestion

- <Règle 1 — ex: "Un utilisateur ne peut avoir qu'un seul rôle actif">
  — `CONFIRMÉ` · <agent> · <date> · <fichier:ligne>

- <Règle 2>
  — `DÉDUIT` · <agent> · <date>

## Flux principaux

1. <Flux 1 — ex: "Inscription → Vérification email → Activation compte">
   — `CONFIRMÉ` · <agent> · <date>

## Risques et points d'attention

- <Risque ou zone fragile identifiée>
  — `CONFIRMÉ` · <agent> · <date> · <fichier>

## Liens vers les autres domaines

- Dépend de : [<domain>](business/<domain>.md) — <raison>
- Utilisé par : [<domain>](business/<domain>.md) — <raison>
```

---

## Format canonique — `ONBOARDING.md` (racine, minimaliste)

```markdown
# [Nom du projet]

> Documentation vivante disponible dans [`docs/wiki/index.md`](docs/wiki/index.md)

## Démarrage rapide

<Commande(s) de démarrage — 1-3 lignes maximum>

## Liens

- [Index wiki](docs/wiki/index.md) — vue globale, god nodes, points critiques
- [Conventions](docs/wiki/technical/conventions.md)
- [Architecture](docs/wiki/technical/architecture.md)
```

**Règles :** 15-25 lignes maximum. Pas de duplication du contenu du wiki.
Son seul rôle est d'orienter un humain qui ouvre le projet vers le wiki.

---

## Règles d'enrichissement incrémental

### Règle fondamentale

❌ Ne jamais écraser une page wiki existante en entier
✅ Toujours lire la page avant d'écrire
✅ Ajouter les nouveaux enrichissements à la fin de la section concernée
✅ Si une section n'existe pas, la créer à la fin de la page
✅ Si un enrichissement contredit un existant `CONFIRMÉ`, le signaler à l'utilisateur

### Procédure d'enrichissement

1. Lire la page cible (`Read`)
2. Identifier la section concernée
3. Formuler le nouveau contenu avec le tag de confiance approprié
4. Ajouter en fin de section — jamais au début, jamais en remplacement
5. Mettre à jour le frontmatter (`updated`, `agents`, `confidence`)
6. Si des god nodes sont potentiellement impactés : appliquer l'algorithme de mise à jour de `index.md` (voir skill `wiki-navigation`)

### Détection des contradictions

Avant d'ajouter un enrichissement, vérifier si la même information existe déjà
dans la section avec un tag `CONFIRMÉ`. Si oui :

- L'enrichissement nouveau est une mise à jour → remplacer l'ancien et indiquer
  la date de la mise à jour dans le tag
- L'enrichissement nouveau est une nuance → ajouter en complément avec un tag
  `DÉDUIT` et mentionner la relation

---

## Template de création d'une nouvelle page wiki

Quand une page n'existe pas encore, la créer à partir du template canonique correspondant
(voir sections ci-dessus). Toujours :

1. Créer le dossier parent si nécessaire (`docs/wiki/business/` ou `docs/wiki/technical/`)
2. Écrire le frontmatter en premier
3. Remplir les sections avec les informations disponibles
4. Marquer les informations manquantes avec `*(À documenter)*`
5. Ajouter le nouveau fichier dans `.git/info/exclude` s'il n'est pas déjà exclu
6. Mettre à jour `docs/wiki/index.md` :
   - Pour une nouvelle page `business/<domain>.md` : ajouter dans la carte des domaines
   - Pour toute nouvelle page : réévaluer le tableau des god nodes
