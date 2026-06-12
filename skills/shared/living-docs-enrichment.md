---
name: living-docs-enrichment
description: Compétence d'enrichissement du wiki documentaire vivant (docs/wiki/) — à activer après chaque rapport d'audit, diagnostic, planification, implémentation de ticket, review ou cycle QA. L'agent identifie les découvertes à capitaliser avec leur niveau de confiance, demande confirmation à l'utilisateur, puis délègue l'écriture au documentarian via l'outil task. Aucune écriture directe.
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
│   └── conventions.md          ← nommage, git, linting, config, patterns équipe
└── business/
    ├── index.md                ← carte des domaines métier
    └── <domain>.md             ← règles de gestion, flux, entités, risques
```

L'enrichissement est toujours **délégué au `documentarian`** — jamais écrit directement.
L'agent qui applique ce skill ne fait que :
1. Consolider les découvertes issues de son travail
2. Formuler le tag de confiance approprié pour chaque enrichissement
3. Proposer l'enrichissement à l'utilisateur
4. Invoquer le `documentarian` si l'utilisateur accepte

---

## Contraintes absolues

❌ Ne jamais écrire directement dans `docs/wiki/`
❌ Ne jamais invoquer le `documentarian` sans confirmation explicite de l'utilisateur
❌ Ne jamais proposer l'enrichissement **pendant** l'analyse — uniquement après le rapport complet
❌ Toujours **afficher le résumé des enrichissements proposés** en texte avant d'appeler `question`
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

Les découvertes proviennent de la section `### Découvertes à documenter` de chaque
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

### Pour les developer-*

Les découvertes émergent de l'implémentation du ticket :
pattern technique adopté pour résoudre le ticket (non documenté dans `conventions.md`),
convention de nommage / structure de fichier observée ou instaurée, librairie ajoutée
ou retirée (à documenter dans `stack.md`), contrainte technique découverte
(non documentée dans `architecture.md`), règle de gestion métier découverte lors de l'implémentation.
Déclencher après chaque `bd close`.

### Pour le reviewer

Les découvertes émergent de la code review :
convention de code observée dans le diff mais absente de `conventions.md`, pattern
récurrent signalé dans le rapport de review, zone d'ombre du wiki levée par
l'analyse du diff (ex : comportement inattendu d'un module, couplage non documenté).

### Pour le qa-engineer

Les découvertes émergent du cycle de test :
convention de test adoptée (nommage, co-location, stratégie d'isolation) non documentée,
edge case systématique révélé par les tests (pattern d'erreur à documenter dans `tests.md`),
gap de testabilité lié à l'architecture.

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
- Si le domaine n'existe pas encore → le mentionner dans la proposition ; le `documentarian`
  crée le fichier avec le template standard du skill `doc-wiki-protocol` si l'utilisateur confirme
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

Avant tout appel à `question`, afficher en texte clair :

```markdown
## 💾 Enrichissement du wiki documentaire — Découvertes à capitaliser

### Enrichissements proposés pour `docs/wiki/index.md`

| Section | Action | Contenu proposé | Confiance |
|---------|--------|-----------------|-----------|
| `## Points critiques actifs 🔴` | Ajouter | "<titre court>" | `CONFIRMÉ` · src/auth/jwt.service.ts:42 |
| `## Zones d'ombre` | Retirer | "<zone levée>" | `CONFIRMÉ` |

### Enrichissements proposés pour `docs/wiki/technical/conventions.md`

| Section | Action | Contenu proposé | Confiance |
|---------|--------|-----------------|-----------|
| `## Patterns spécifiques à l'équipe` | Ajouter | "<pattern observé>" | `CONFIRMÉ` · src/services/user.service.ts:17 |

### Enrichissements proposés pour `docs/wiki/technical/architecture.md`

| Section | Action | Contenu proposé | Confiance |
|---------|--------|-----------------|-----------|
| `## Points de fragilité connus` | Ajouter | "<zone fragile>" | `CONFIRMÉ` · src/controllers/auth.controller.ts:23 |

### Enrichissements proposés pour `docs/wiki/technical/stack.md`

| Section | Action | Contenu proposé | Confiance |
|---------|--------|-----------------|-----------|
| `## Librairies clés` | Ajouter "À éviter" | "lodash 4.17.20 — CVE-2024-1234" | `CONFIRMÉ` · package.json |

### Enrichissements proposés pour `docs/wiki/business/<domain>.md`

| Fichier | Section | Action | Contenu proposé | Confiance |
|---------|---------|--------|-----------------|-----------|
| `auth.md` | `## Règles de gestion` | Ajouter | "<règle métier>" | `CONFIRMÉ` · src/auth/auth.service.ts:89 |

> Si aucun enrichissement pour une page, ne pas afficher la section correspondante.
> Si une page `business/<domain>.md` n'existe pas encore, l'indiquer explicitement.
```

---

### ÉTAPE 3 — Demander confirmation

Après affichage du résumé, utiliser l'outil `question` :

```
question({
  questions: [{
    header: "Enrichir le wiki",
    question: "[<Nom de l'agent> — Post-<audit/diagnostic/planification/implémentation/review/QA> | Projet : <nom>]\nJ'ai identifié X enrichissements à capitaliser dans le wiki (voir résumé ci-dessus). Déléguer l'écriture au documentarian ?",
    options: [
      { label: "Oui — déléguer au documentarian (Recommandé)", description: "Invoquer le documentarian pour enrichir le wiki de manière incrémentale" },
      { label: "Non — passer", description: "Conserver le wiki tel quel" }
    ]
  }]
})
```

**Selon la réponse :**
- **Oui** → ÉTAPE 4 (déléguer au documentarian)
- **Non** → Fin — afficher `> 💾 Wiki documentaire conservé tel quel.`

---

### ÉTAPE 4 — Déléguer au documentarian

Invoquer le `documentarian` via l'outil `task` avec un prompt structuré :

```
task({
  subagent_type: "documentarian",
  description: "Enrichissement incrémental wiki documentaire",
  prompt: `
Enrichis de manière incrémentale les pages du wiki documentaire du projet listées ci-dessous,
avec les découvertes issues d'un <audit <domaine> / diagnostic de bug / planification de feature
/ implémentation de ticket / code review / cycle QA / reconnaissance>.

## Règles impératives

- Charger le skill doc-wiki-protocol avant d'écrire (il définit les formats canoniques)
- Enrichissement incrémental uniquement — NE PAS écraser le contenu existant
- Lire chaque page wiki avant de l'enrichir
- Ajouter le contenu à la fin de la section cible concernée
- Chaque enrichissement doit porter le tag de confiance fourni ci-dessous (format exact : voir doc-wiki-protocol)
- Si une page docs/wiki/business/<domain>.md doit être créée : l'initialiser avec le template standard du skill doc-wiki-protocol
- Mettre à jour le frontmatter (updated, agents, confidence) de chaque page modifiée
- Après tout enrichissement : réévaluer le tableau des god nodes dans docs/wiki/index.md
  (algorithme dans le skill wiki-navigation — un concept cité dans ≥ 2 pages devient god node)
- Ne pas modifier la structure des pages (sections, titres existants)

## Enrichissements à appliquer

### docs/wiki/index.md
<liste des enrichissements identifiés à l'ÉTAPE 2, avec section cible, action, contenu et tag de confiance>

### docs/wiki/technical/conventions.md
<liste des enrichissements identifiés à l'ÉTAPE 2>

### docs/wiki/technical/architecture.md
<liste des enrichissements identifiés à l'ÉTAPE 2>

### docs/wiki/technical/stack.md
<liste des enrichissements identifiés à l'ÉTAPE 2>

### docs/wiki/technical/tests.md
<liste des enrichissements identifiés à l'ÉTAPE 2>

### docs/wiki/business/<domain>.md
<liste des enrichissements identifiés à l'ÉTAPE 2, avec fichier cible, section cible, action, contenu et tag>

> Si une page n'a aucun enrichissement, ne pas la modifier.
`
})
```

---

### ÉTAPE 5 — Confirmer la délégation

Après le retour du `documentarian`, afficher :

```markdown
## ✅ Wiki documentaire enrichi

Le `documentarian` a enrichi les pages suivantes :
- `docs/wiki/index.md` — X modifications (god nodes mis à jour : oui/non)
- `docs/wiki/technical/conventions.md` — Y modifications
- `docs/wiki/technical/architecture.md` — Z modifications
- `docs/wiki/technical/stack.md` — W modifications
- `docs/wiki/technical/tests.md` — V modifications
- `docs/wiki/business/<domain>.md` — U modifications

> Ces informations seront automatiquement utilisées par les agents futurs
> (developer-*, orchestrator-dev, auditors) lors de leurs prochains accès au projet.
```

---

## Tableau de correspondance — Origine → Pages wiki prioritaires

| Origine | index.md | conventions.md | architecture.md | stack.md | tests.md | business/\<domain\>.md |
|---------|----------|----------------|-----------------|----------|----------|------------------------|
| Audit sécurité | Points critiques 🔴, Zones d'ombre | Patterns (auth, validation) | Points de fragilité | Librairies (CVE) | — | Risques (auth/billing) |
| Audit performance | Points importants 🟠 | — | Architecture (N+1, cache) | Librairies (alternatives) | — | — |
| Audit accessibilité | Points importants 🟠 | Patterns (ARIA) | — | — | Conventions a11y | — |
| Audit éco-conception | — | — | — | Librairies (légères) | — | — |
| Audit architecture | Architecture, Points critiques 🔴 | — | Structure, Décisions, Fragilités | — | — | — |
| Audit privacy | Points critiques 🔴, Zones d'ombre | Config & secrets | — | — | — | Risques (données) |
| Audit observabilité | Points importants 🟠 | Patterns (logging) | Architecture (métriques) | — | — | — |
| Diagnostic bug | Points critiques 🔴, Zones d'ombre | Patterns (gestion d'erreur) | Points de fragilité | — | — | Règles, Risques |
| Planification feature | Architecture, Zones d'ombre | — | Architecture, Décisions | Stack | — | Flux, Règles |
| Implémentation ticket | — | Patterns, Nommage | — | Librairies | — | Règles de gestion |
| Code review | Zones d'ombre, Points importants 🟠 | Nommage, Patterns | — | — | — | — |
| Cycle QA | — | — | — | — | Frameworks, Conventions | — |
| Reconnaissance rapide (pathfinder) | Stack, Architecture | — | Structure globale | Stack, Dépendances | — | Flux, Entités |
| Re-onboarding (onboarder) | Toutes sections | Toutes sections | Toutes sections | Toutes sections | Toutes sections | Toutes sections |

---

## Règles de qualité des enrichissements

✅ **Factuel** : basé sur des éléments concrets observés (fichier, ligne, pattern)
✅ **Concis** : une ligne ou un bloc court par enrichissement
✅ **Taggé** : chaque enrichissement porte son tag de confiance avec la source
✅ **Non redondant** : vérifier que l'information n'est pas déjà présente dans la page
✅ **Actionnable** : compréhensible par un agent futur sans relire le rapport source

❌ Ne pas transmettre le rapport complet au documentarian — seulement les enrichissements ciblés
❌ Ne pas ajouter d'enrichissements subjectifs ou spéculatifs
❌ Ne pas modifier la structure des pages via le documentarian — uniquement le contenu
❌ Ne pas omettre le tag de confiance — chaque enrichissement doit en porter un
