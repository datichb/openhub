---
name: doc-wiki-protocol
description: "Utiliser pour écrire dans docs/wiki/ : formats canoniques, frontmatter, tags de confiance, god nodes."
annexes: [templates/wiki-pages-index.md, templates/wiki-pages-technical.md, templates/wiki-pages-business.md]
---

# Skill — Protocole du Wiki Documentaire Vivant

## Rôle

Ce skill définit le format exact de chaque page du wiki et les règles d'écriture
que le `documentarian` doit respecter pour maintenir la cohérence du système.

Il est chargé par le `documentarian` lors d'opérations d'écriture sur le wiki, et par l'`onboarder`
à la création du wiki.

Les autres agents n'écrivent pas dans le wiki : ils proposent avec `wiki_propose` ; oh applique
les propositions validées par l'utilisateur à la carte du checkpoint de commit, en suivant ce format
(fin de section, ligne de tag, frontmatter, god nodes). Avec le MCP du wiki, `wiki_pending` liste
les propositions en file, `wiki_apply(ids)` les applique (soumis à l'utilisateur) et `wiki_check`
vérifie le wiki (frontmatter, liens, tags, fraîcheur, god nodes calculés par oh).

---

## Étape -1 — Détection monorepo

Avant toute action, vérifier si le dépôt est un monorepo :

```bash
ls pnpm-workspace.yaml lerna.json nx.json turbo.json 2>/dev/null
ls packages/*/package.json apps/*/package.json 2>/dev/null
cat Cargo.toml 2>/dev/null | head -5  # [workspace] section
```

**Si monorepo détecté :**
1. Identifier le package cible de la tâche en cours (depuis le contexte, le ticket, ou en demandant à l'utilisateur)
2. Définir `$PROJECT_DIR` = chemin du package (ex: `packages/api-gateway/`)
3. Scoper toutes les recherches et chemins à `$PROJECT_DIR` au lieu de la racine du dépôt

**Si pas de monorepo :** continuer normalement avec la racine comme scope.

> **Note :** En monorepo, le wiki reste centralisé à `docs/wiki/` (racine). Les pages peuvent référencer des packages spécifiques via des sections dédiées.

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
├── decisions/
│   └── decisions-rejetees.md   ← concepts rejetés avec raison et cycle de vie (ACTIVE/EXPIRED/SUPERSEDED)
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

## Formats canoniques des pages (annexes à la demande)

Avant de créer une page ou d'en réécrire la structure, lis avec `read` l'annexe qui donne son
format canonique ; un enrichissement d'une page existante n'en a pas besoin (suis ses sections).

| Page | Annexe à lire |
|------|---------------|
| `docs/wiki/index.md`, `ONBOARDING.md` | `templates/wiki-pages-index.md` |
| `technical/conventions.md`, `architecture.md`, `stack.md`, `tests.md`, `review-rules.md` | `templates/wiki-pages-technical.md` |
| `business/index.md`, `business/<domain>.md` | `templates/wiki-pages-business.md` |

**Règles des pages :**
- `index.md` est la seule page que tous les agents lisent systématiquement. Elle doit rester compacte — 40-80 lignes maximum hors frontmatter
- Ne jamais dupliquer le contenu détaillé des sous-pages dans `index.md`
- Les liens vers les sous-pages sont obligatoires dans la table des god nodes
- `technical/review-rules.md` est optionnelle : elle n'existe que si le projet a des règles de review propres (exclusions, patterns intentionnels, seuils) ; le reviewer la consulte à chaque review
- `ONBOARDING.md` (racine) : 15-25 lignes, sans dupliquer le wiki ; il oriente un humain vers `docs/wiki/index.md`

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
6. God nodes : oh les calcule ; après tes écritures, `wiki_check` (ou `oh wiki check --fix`) signale et réécrit le tableau de `index.md`

### Détection des contradictions

Avant d'ajouter un enrichissement, vérifier si la même information existe déjà
dans la section avec un tag `CONFIRMÉ`. Si oui :

- L'enrichissement nouveau est une mise à jour → remplacer l'ancien et indiquer
  la date de la mise à jour dans le tag
- L'enrichissement nouveau est une nuance → ajouter en complément avec un tag
  `DÉDUIT` et mentionner la relation

---

## Création d'une nouvelle page wiki

Quand une page n'existe pas encore, la créer à partir du format canonique de son annexe
(tableau ci-dessus). Toujours :

1. Créer le dossier parent si nécessaire (`docs/wiki/business/`, `docs/wiki/technical/` ou `docs/wiki/decisions/`)
2. Écrire le frontmatter en premier
3. Remplir les sections avec les informations disponibles
4. Marquer les informations manquantes avec `*(À documenter)*`
5. Mettre à jour `docs/wiki/index.md` :
   - Pour une nouvelle page `business/<domain>.md` : ajouter dans la carte des domaines
   - Pour toute nouvelle page : réévaluer le tableau des god nodes
