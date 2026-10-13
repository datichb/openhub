---
name: living-docs-enrichment
description: "Utiliser après un rapport, diagnostic, plan, ticket ou review : proposer les découvertes à capitaliser dans docs/wiki/."
---

# Skill — Enrichissement du Wiki Documentaire Vivant

## Rôle

Ce skill définit le protocole par lequel tous les agents d'analyse et d'implémentation
enrichissent de manière **incrémentale** le wiki documentaire du projet cible :

```
docs/wiki/
├── index.md                    ← god nodes, points critiques, carte des domaines
├── technical/
│   ├── architecture.md         ← patterns dominants, découpage, décisions structurantes
│   ├── stack.md                ← stack complète, versions, librairies clés
│   ├── tests.md                ← stratégie, conventions, seuils, frameworks
│   ├── conventions.md          ← nommage, git, linting, config, patterns équipe
│   ├── glossary.md             ← terminologie du domaine (optionnel — créé si termes ambigus)
│   └── retro-log.md            ← observations rétrospectives post-feature (optionnel)
├── decisions/
│   └── decisions-rejetees.md   ← concepts rejetés avec raison et cycle de vie (ACTIVE/EXPIRED/SUPERSEDED)
└── business/
    ├── index.md                ← carte des domaines métier
    └── <domain>.md             ← règles de gestion, flux, entités, risques
```

L'enrichissement passe toujours par l'outil **`wiki_propose`** (serveur MCP `wiki` du projet) — jamais écrit directement, jamais délégué au `documentarian`.
L'agent qui applique ce skill ne fait que :
1. Consolider les découvertes issues de son travail
2. Formuler le tag de confiance approprié pour chaque enrichissement
3. Proposer chaque enrichissement avec `wiki_propose` (ÉTAPE 3)

**Chaîne complète :**

```
Agent (auditor, planner, debugger, developer, reviewer…)
  │  applique ce skill après son rapport/livrable
  ↓
wiki_propose  →  file de propositions (wp-<n>), auteur fixé par oh
  ↓
carte du checkpoint de commit : l'utilisateur accepte, modifie ou refuse
  ↓  (acceptées)
oh applique dans docs/wiki/ au passage du checkpoint (committé avec le code)
   — workflow sans commit : la proposition reste en file (doc-sync)
```

---

## Contraintes absolues

❌ Ne jamais écrire directement dans `docs/wiki/`
❌ Ne jamais invoquer le `documentarian` pour enrichir le wiki : `wiki_propose`, l'utilisateur valide à la carte du commit
❌ Ne jamais citer `~/.oh/sessions` (données de session d'oh) dans une proposition : oh la refuse ; écris le constat lui-même, depuis le code
❌ Ne jamais proposer l'enrichissement **pendant** l'analyse — uniquement après le rapport complet
❌ Si aucune découverte pertinente → ne pas proposer l'enrichissement, afficher simplement :
   `> 💾 Wiki documentaire : aucune nouvelle découverte à capitaliser.`

---

## Tags de confiance — choisir le bon niveau

Chaque enrichissement proposé doit porter un tag de confiance :

| Tag | Quand l'utiliser | Fichier source à fournir |
|-----|-----------------|--------------------------|
| `` `CONFIRMÉ` `` | Observation directe dans le code — ligne précise identifiée | `fichier:ligne` obligatoire |
| `` `DÉDUIT` `` | Raisonnement contextuel depuis plusieurs fichiers, ligne non précise | `fichier` recommandé |
| `` `INCERTAIN` `` | Hypothèse, convention non codée, information venant d'un commentaire | Fichier optionnel |

**Format du tag dans le contenu proposé :**
```
<Description de l'enrichissement>
— `CONFIRMÉ` · <agent-id> · <YYYY-MM-DD> · <fichier:ligne>
```

---

## Source des découvertes

### Pour l'auditor coordinateur

Les découvertes proviennent de la section `### Enrichissements wiki proposés` de chaque
rapport de sous-agent reçu en Phase 3. Consolider toutes ces sections avant de
proposer l'enrichissement en Phase 4.

### Pour le planner

Les découvertes émergent de l'exploration contextuelle (Phase 1) :
patterns architecturaux observés, conventions de nommage détectées dans la codebase,
librairies utilisées non documentées, logiques réutilisables identifiées, règles métier
implicites découvertes lors de l'analyse du domaine.

### Pour le debugger

Les découvertes émergent du diagnostic (Phase 3) :
zone d'ombre du wiki levée par l'analyse, pattern d'erreur récurrent absent
de `conventions.md` (gestion d'erreur, validation, auth), point d'attention critique
à mémoriser dans `index.md`, règle de gestion métier implicite découverte lors du diagnostic.

### Pour les developer

Les découvertes émergent de l'implémentation du ticket :
pattern technique adopté pour résoudre le ticket (non documenté dans `conventions.md`),
convention de nommage / structure de fichier observée ou instaurée, librairie ajoutée
ou retirée (à documenter dans `stack.md`), contrainte technique découverte
(non documentée dans `architecture.md`), règle de gestion métier découverte lors de l'implémentation.
Déclencher après chaque fermeture de ticket (`bd close`). <!-- capability: example -->

### Pour le reviewer

Les découvertes émergent de la code review :
convention de code observée dans le diff mais absente de `conventions.md`, pattern
récurrent signalé dans le rapport de review, zone d'ombre du wiki levée par
l'analyse du diff (ex : comportement inattendu d'un module, couplage non documenté).

### Pour le pathfinder

Les découvertes émergent de la reconnaissance rapide :
patterns architecturaux détectés mais absents du wiki, conventions implicites
observées dans la codebase non documentées dans `conventions.md`, stack non référencée
dans `stack.md`, signaux de dette technique à mémoriser, flux métier identifiés
à documenter dans `docs/wiki/business/`.

### Pour l'onboarder (mode enrichissement)

Applicable uniquement lorsque `docs/wiki/index.md` **existe déjà** et
que l'onboarder est invoqué en re-onboarding (voir skill `onboarder-workflow` Phase 5).
Les découvertes proviennent du rapport de re-onboarding : nouveaux patterns détectés,
stack étendue, points d'attention mis à jour, zones d'ombre résolues ou nouvelles,
nouveaux domaines métier ou flux identifiés.

---

## Workflow d'enrichissement

### ÉTAPE 1 — Identifier les enrichissements pertinents

Analyser les découvertes disponibles et les classifier par page wiki cible.
Pour chaque enrichissement, identifier le tag de confiance approprié.

#### Ce qui peut enrichir `docs/wiki/index.md`

| Section cible | Découverte typique | Tag recommandé |
|---------------|--------------------|----------------|
| `## Stack critique` | Nouvelle dépendance critique identifiée | `CONFIRMÉ` avec `package.json` |
| `## Architecture (résumé)` | Changement architectural majeur (≤ 2 lignes) | `CONFIRMÉ` ou `DÉDUIT` |
| `## God nodes` | Concept apparu dans ≥ 2 pages wiki après enrichissement | `DÉDUIT` |
| `## Points critiques actifs 🔴` | Faille critique ou dette bloquante | `CONFIRMÉ` avec fichier:ligne |
| `## Zones d'ombre` | Zone résolue (retirer) ou nouvelle zone (ajouter) | `CONFIRMÉ` ou `INCERTAIN` |

> `index.md` est la carte globale — ne pas y ajouter de contenu détaillé. Si la découverte
> est volumineuse, la router vers la page technique ou métier appropriée.

#### Ce qui peut enrichir `docs/wiki/technical/conventions.md`

| Section cible | Découverte typique | Tag recommandé |
|---------------|--------------------|----------------|
| `## Linting & formatage` | Incohérence entre config et code réel | `CONFIRMÉ` avec fichier config |
| `## Nommage` | Convention observée dans le code mais non documentée | `CONFIRMÉ` avec fichier:ligne |
| `## Git` | Pattern de commit ou de branche observé non documenté | `DÉDUIT` |
| `## Configuration & secrets` | Variable d'env requise non référencée | `CONFIRMÉ` avec `.env.example` |
| `## Patterns spécifiques à l'équipe` | Pattern de gestion d'erreur, auth, logging identifié | `CONFIRMÉ` avec fichier:ligne |
| `## À ne pas utiliser` | Librairie exclue, anti-pattern explicitement évité | `CONFIRMÉ` avec fichier ou ADR |

#### Ce qui peut enrichir `docs/wiki/technical/architecture.md`

| Section cible | Découverte typique | Tag recommandé |
|---------------|--------------------|----------------|
| `## Structure globale` | Confirmation ou correction du pattern architectural | `CONFIRMÉ` avec dossier |
| `## Découpage en couches` | Couche non documentée ou couplage inattendu | `CONFIRMÉ` avec fichier:ligne |
| `## Communication entre modules` | Mécanisme de communication découvert | `DÉDUIT` |
| `## Décisions architecturales notables` | Décision expliquée dans un commentaire ou ADR | `CONFIRMÉ` avec ADR ou fichier |
| `## Points de fragilité connus` | Zone fragile identifiée par audit ou review | `CONFIRMÉ` avec fichier:ligne |

#### Ce qui peut enrichir `docs/wiki/technical/stack.md`

| Section cible | Découverte typique | Tag recommandé |
|---------------|--------------------|----------------|
| `## Dépendances principales` | Dépendance clé ajoutée ou mise à jour | `CONFIRMÉ` avec `package.json` |
| `## Librairies clés` | Librairie critique identifiée (CVE, alternative retenue) | `CONFIRMÉ` avec fichier |
| `## Variables d'environnement requises` | Variable non documentée dans `.env.example` | `CONFIRMÉ` avec fichier:ligne |
| `## Contraintes de version` | Contrainte de version imposée par un outil | `CONFIRMÉ` avec fichier config |

#### Ce qui peut enrichir `docs/wiki/technical/tests.md`

| Section cible | Découverte typique | Tag recommandé |
|---------------|--------------------|----------------|
| `## Frameworks` | Framework de test ajouté ou modifié | `CONFIRMÉ` avec fichier config |
| `## Organisation` | Convention d'organisation des tests observée | `CONFIRMÉ` avec exemple de fichier |
| `## Seuil de couverture` | Seuil configuré ou discuté | `CONFIRMÉ` avec fichier config |
| `## Conventions de nommage des tests` | Pattern de nommage observé dans les tests | `CONFIRMÉ` avec fichier:ligne |

#### Ce qui peut enrichir `docs/wiki/business/<domain>.md`

Avant de router vers un fichier de domaine, identifier le domaine concerné :
- Si la découverte correspond à un domaine existant → router vers ce fichier
- Si le domaine n'existe pas encore → proposer sur `business/<domain>.md` : oh crée la page
  (frontmatter et titre) quand l'utilisateur accepte
- Si le domaine est indéterminable → noter dans `index.md > ## Zones d'ombre`

| Section cible | Découverte typique | Tag recommandé |
|---------------|--------------------|----------------|
| `## Règles de gestion` | Règle métier implicite découverte dans le code | `CONFIRMÉ` avec fichier:ligne |
| `## Flux principaux` | Nouveau flux utilisateur ou système identifié | `DÉDUIT` |
| `## Entités clés` | Entité métier centrale non documentée | `CONFIRMÉ` avec fichier |
| `## Risques et points d'attention` | Risque métier spécifique au domaine | `CONFIRMÉ` ou `DÉDUIT` |
| `## Zones d'ombre` | Zone d'ombre métier résolue ou nouvellement découverte | `CONFIRMÉ` ou `INCERTAIN` |

---

### ÉTAPE 2 — Construire le résumé des enrichissements proposés

Un tableau par page cible, à reprendre dans ton rapport :

| Page | Section | Contenu proposé | Confiance |
|------|---------|-----------------|-----------|
| `technical/conventions.md` | `## Patterns spécifiques à l'équipe` | "<pattern observé>" | `CONFIRMÉ` · src/services/user.service.ts:17 |

---

### ÉTAPE 3 — Proposer avec `wiki_propose`

<!-- oh:when mcp=wiki -->
Un appel par enrichissement :

```
wiki_propose({ page: "technical/conventions.md", section: "Patterns spécifiques à l'équipe",
  content: "<constat factuel, une ligne ou un bloc court>", tag: "CONFIRMÉ", source: "src/services/user.service.ts:17" })
```

- `page` : relative à `docs/wiki/` ; `section` : titre de la section cible (créée à la fin si absente).
- `content` : le constat seul, sans titre ni ligne de tag : oh écrit le tag, ton nom (vu par oh), la date et la source.
- Cite les identifiants `wp-…` rendus dans ton rapport, au lieu de recopier les propositions.
- Ne pose pas de question : l'utilisateur décide à la carte du checkpoint de commit. Sans commit dans le workflow (audit, review…), la proposition reste en file.
- Réponse « non enregistrée » (démon oh indisponible) : liste les enrichissements dans `### Enrichissements wiki proposés` de ton rapport.
<!-- /oh:when -->
<!-- oh:when mcp!=wiki -->
Sans l'outil `wiki_propose` dans cette session : liste les enrichissements, avec leur tag et leur source, dans `### Enrichissements wiki proposés` de ton rapport, et arrête ce skill ici.
<!-- /oh:when -->

---

## Tableau de correspondance — Origine → Pages wiki prioritaires

| Origine | index.md | conventions.md | architecture.md | stack.md | tests.md | glossary.md | business/\<domain\>.md |
|---------|----------|----------------|-----------------|----------|----------|-------------|------------------------|
| Audit sécurité | Points critiques 🔴, Zones d'ombre | Patterns (auth, validation) | Points de fragilité | Librairies (CVE) | — | — | Risques (auth/billing) |
| Audit performance | Points importants 🟠 | — | Architecture (N+1, cache) | Librairies (alternatives) | — | — | — |
| Audit accessibilité | Points importants 🟠 | Patterns (ARIA) | — | — | Conventions a11y | — | — |
| Audit éco-conception | — | — | — | Librairies (légères) | — | — | — |
| Audit architecture | Architecture, Points critiques 🔴 | — | Structure, Décisions, Fragilités | — | — | — | — |
| Audit privacy | Points critiques 🔴, Zones d'ombre | Config & secrets | — | — | — | — | Risques (données) |
| Audit observabilité | Points importants 🟠 | Patterns (logging) | Architecture (métriques) | — | — | — | — |
| Diagnostic bug | Points critiques 🔴, Zones d'ombre | Patterns (gestion d'erreur) | Points de fragilité | — | — | — | Règles, Risques |
| Planification feature | Architecture, Zones d'ombre | — | Architecture, Décisions | Stack | — | Termes ambigus | Flux, Règles |
| Implémentation ticket | — | Patterns, Nommage | — | Librairies | — | Termes métier découverts | Règles de gestion |
| Code review | Zones d'ombre, Points importants 🟠 | Nommage, Patterns | — | — | — | Termes inconsistants | — |
| Reconnaissance rapide (pathfinder) | Stack, Architecture | — | Structure globale | Stack, Dépendances | — | Termes de domaine | Flux, Entités |
| Re-onboarding (onboarder) | Toutes sections | Toutes sections | Toutes sections | Toutes sections | Toutes sections | Toutes sections | Toutes sections |

---

## Règles de qualité des enrichissements

✅ **Factuel** : basé sur des éléments concrets observés (fichier, ligne, pattern)
✅ **Concis** : une ligne ou un bloc court par enrichissement
✅ **Taggé** : chaque enrichissement porte son tag de confiance avec la source
✅ **Non redondant** : vérifier que l'information n'est pas déjà présente dans la page
✅ **Actionnable** : compréhensible par un agent futur sans relire le rapport source

❌ Ne pas mettre le rapport complet dans `wiki_propose` — seulement les enrichissements ciblés
❌ Ne pas ajouter d'enrichissements subjectifs ou spéculatifs
❌ Ne pas proposer de changement de structure des pages (titres, sections existantes) — uniquement du contenu
❌ Ne pas omettre le tag de confiance — chaque enrichissement doit en porter un
