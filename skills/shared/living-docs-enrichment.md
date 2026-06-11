---
name: living-docs-enrichment
description: Compétence d'enrichissement des documents vivants (ONBOARDING.md, CONVENTIONS.md, docs/context/technical.md, docs/context/business/<domaine>.md) — à activer après chaque rapport d'audit, diagnostic, planification, implémentation de ticket, review ou cycle QA. L'agent identifie les découvertes à capitaliser, demande confirmation à l'utilisateur, puis délègue l'écriture au documentarian via l'outil task. Aucune écriture directe.
---

# Skill — Enrichissement des Documents Vivants

## Rôle

Ce skill définit le protocole par lequel tous les agents d'analyse et d'implémentation
enrichissent de manière **incrémentale** les fichiers de contexte du projet cible :
- `ONBOARDING.md` — résumé exécutif compact à la racine
- `CONVENTIONS.md` — conventions de code condensées à la racine
- `docs/context/technical.md` — architecture, tests, librairies
- `docs/context/business/<domaine>.md` — contexte métier par domaine

L'enrichissement est toujours **délégué au `documentarian`** — jamais écrit directement.
L'agent qui applique ce skill ne fait que :
1. Consolider les découvertes issues de son travail
2. Proposer l'enrichissement à l'utilisateur
3. Invoquer le `documentarian` si l'utilisateur accepte

---

## Contraintes absolues

❌ Ne jamais écrire directement dans ONBOARDING.md, CONVENTIONS.md, docs/context/technical.md ou docs/context/business/*.md
❌ Ne jamais invoquer le `documentarian` sans confirmation explicite de l'utilisateur
❌ Ne jamais proposer l'enrichissement **pendant** l'analyse — uniquement après le rapport complet
❌ Toujours **afficher le résumé des enrichissements proposés** en texte avant d'appeler `question`
❌ Si aucune découverte pertinente → ne pas proposer l'enrichissement, afficher simplement :
   `> 💾 Documents vivants : aucune nouvelle découverte à capitaliser.`

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
zone d'ombre de ONBOARDING.md levée par l'analyse, pattern d'erreur récurrent absent
de CONVENTIONS.md (gestion d'erreur, validation, auth), point d'attention critique
à mémoriser dans ONBOARDING.md, règle de gestion métier implicite découverte lors du diagnostic.

### Pour les developer-*

Les découvertes émergent de l'implémentation du ticket :
pattern technique adopté pour résoudre le ticket (non documenté dans CONVENTIONS.md),
convention de nommage / structure de fichier observée ou instaurée, librairie ajoutée
ou retirée (à documenter dans docs/context/technical.md), contrainte technique découverte
(non documentée dans ONBOARDING.md), règle de gestion métier découverte lors de l'implémentation.
Déclencher après chaque `bd close`.

### Pour le reviewer

Les découvertes émergent de la code review :
convention de code observée dans le diff mais absente de CONVENTIONS.md, pattern
récurrent signalé dans le rapport de review, zone d'ombre de ONBOARDING.md levée par
l'analyse du diff (ex : comportement inattendu d'un module, couplage non documenté).

### Pour le qa-engineer

Les découvertes émergent du cycle de test :
convention de test adoptée (nommage, co-location, stratégie d'isolation) non documentée,
edge case systématique révélé par les tests (pattern d'erreur à documenter dans
docs/context/technical.md), gap de testabilité lié à l'architecture.

### Pour le pathfinder

Les découvertes émergent de la reconnaissance rapide :
patterns architecturaux détectés mais absents de ONBOARDING.md, conventions implicites
observées dans la codebase non documentées dans CONVENTIONS.md, stack non référencée
dans ONBOARDING.md, signaux de dette technique à mémoriser, flux métier identifiés
à documenter dans docs/context/business/.

### Pour l'onboarder (mode enrichissement)

Applicable uniquement lorsque les fichiers de contexte **existent déjà** et
que l'onboarder est invoqué en re-onboarding (voir skill `onboarder-workflow` Phase 5).
Les découvertes proviennent du rapport de re-onboarding : nouveaux patterns détectés,
stack étendue, points d'attention mis à jour, zones d'ombre résolues ou nouvelles,
nouveaux domaines métier ou flux identifiés.

---

## Workflow d'enrichissement

### ÉTAPE 1 — Identifier les enrichissements pertinents

Analyser les découvertes disponibles et les classifier :

#### Ce qui peut enrichir ONBOARDING.md — résumé exécutif compact

| Section cible | Découverte typique |
|---------------|--------------------|
| `## Stack détectée` | Nouvelle bibliothèque critique identifiée (ligne condensée) |
| `## Architecture` | Changement architectural majeur (≤ 3 lignes) |
| `## Points critiques 🔴` | Faille critique ou dette bloquante |
| `## Points importants 🟠` | Problème majeur non encore documenté |
| `## Zones d'ombre` | Zone résolue (retirer) ou nouvelle zone (ajouter) |

> `ONBOARDING.md` est un résumé exécutif — ne pas y ajouter de contenu détaillé. Si la découverte est volumineuse, la router vers `docs/context/technical.md` ou `docs/context/business/<domaine>.md`.

#### Ce qui peut enrichir CONVENTIONS.md — conventions condensées

| Section cible | Découverte typique |
|---------------|--------------------|
| `## Linting & formatting` | Incohérence entre config et code réel |
| `## Nommage` | Convention observée dans le code mais non documentée |
| `## Conventions Git` | Pattern de commit ou de branche observé non documenté |
| `## Config & secrets` | Variable d'env requise non référencée |
| `## Patterns spécifiques à l'équipe` | Pattern de gestion d'erreur, auth, logging identifié |
| `## Zones d'ombre` | Zone résolue ou nouvelle zone découverte |

#### Ce qui peut enrichir docs/context/technical.md

| Section cible | Découverte typique |
|---------------|--------------------|
| `## Architecture` | Détail architectural confirmé ou réfuté, pattern structurel observé |
| `## Stratégie de tests` | Convention de test observée (co-location, nommage, mocking) |
| `## Librairies externes clés` | Librairie à éviter (CVE, alternative retenue), nouvelle lib ajoutée |
| `## Design et maquettes` | Nouveau fichier Figma ou token découvert |
| `## Zones d'ombre techniques` | Zone technique résolue ou nouvellement découverte |

#### Ce qui peut enrichir docs/context/business/<domaine>.md

Avant de router vers un fichier de domaine, identifier le domaine concerné :
- Si la découverte correspond à un domaine existant (`docs/context/business/<domaine>.md`) → router vers ce fichier
- Si le domaine n'existe pas encore → le mentionner dans la proposition ; le documentarian crée le fichier avec le template standard si l'utilisateur confirme
- Si le domaine est indéterminable → noter dans `ONBOARDING.md > ## Zones d'ombre`

| Section cible | Découverte typique |
|---------------|--------------------|
| `## Règles de gestion` | Règle métier implicite découverte dans le code |
| `## Flux principaux` | Nouveau flux utilisateur ou système identifié |
| `## Entités clés` | Entité métier centrale non documentée |
| `## Risques et points d'attention` | Risque métier spécifique au domaine |
| `## Zones d'ombre` | Zone d'ombre métier résolue ou nouvellement découverte |

---

### ÉTAPE 2 — Construire le résumé des enrichissements proposés

Avant tout appel à `question`, afficher en texte clair :

```markdown
## 💾 Enrichissement des documents vivants — Découvertes à capitaliser

### Enrichissements proposés pour ONBOARDING.md

| Section | Action | Contenu proposé |
|---------|--------|-----------------|
| `## Points critiques 🔴` | Ajouter | "<titre court de la découverte> (source : audit <domaine>)" |
| `## Zones d'ombre` | Retirer | "<zone levée par l'analyse>" |

### Enrichissements proposés pour CONVENTIONS.md

| Section | Action | Contenu proposé |
|---------|--------|-----------------|
| `## Patterns spécifiques à l'équipe` | Ajouter | "<pattern observé>" |

### Enrichissements proposés pour docs/context/technical.md

| Section | Action | Contenu proposé |
|---------|--------|-----------------|
| `## Librairies externes clés` | Ajouter "À ne pas utiliser" | "<lib> — CVE <ID> : <description courte>" |

### Enrichissements proposés pour docs/context/business/<domaine>.md

| Fichier | Section | Action | Contenu proposé |
|---------|---------|--------|-----------------|
| `auth.md` | `## Règles de gestion` | Ajouter | "<règle métier découverte>" |

> Si aucun enrichissement pour un fichier, ne pas afficher la section correspondante.
> Si un fichier de domaine n'existe pas encore, l'indiquer explicitement : "Créer docs/context/business/<domaine>.md".
```

---

### ÉTAPE 3 — Demander confirmation

Après affichage du résumé, utiliser l'outil `question` :

```
question({
  questions: [{
    header: "Enrichir les docs vivants",
    question: "[<Nom de l'agent> — Post-<audit/diagnostic/planification/implémentation/review/QA> | Projet : <nom>]\nJ'ai identifié X enrichissements à capitaliser (voir résumé ci-dessus). Déléguer l'écriture au documentarian ?",
    options: [
      { label: "Oui — déléguer au documentarian (Recommandé)", description: "Invoquer le documentarian pour enrichir les fichiers de contexte de manière incrémentale" },
      { label: "Non — passer", description: "Conserver les documents tels quels" }
    ]
  }]
})
```

**Selon la réponse :**
- **Oui** → ÉTAPE 4 (déléguer au documentarian)
- **Non** → Fin — afficher `> 💾 Documents vivants conservés tels quels.`

---

### ÉTAPE 4 — Déléguer au documentarian

Invoquer le `documentarian` via l'outil `task` avec un prompt structuré :

```
task({
  subagent_type: "documentarian",
  description: "Enrichissement incrémental docs vivants",
  prompt: `
Enrichis de manière incrémentale les fichiers de contexte du projet listés ci-dessous,
avec les découvertes issues d'un <audit <domaine> / diagnostic de bug / planification de feature
/ implémentation de ticket / code review / cycle QA / reconnaissance>.

## Règles impératives

- Enrichissement incrémental uniquement — NE PAS écraser le contenu existant
- Lire chaque fichier avant de l'enrichir
- Ajouter le contenu à la fin de la section cible concernée
- Si un fichier docs/context/business/<domaine>.md doit être créé : l'initialiser avec le template standard
  (sections : Règles de gestion / Flux principaux / Entités clés / Risques et points d'attention / Zones d'ombre)
- Ajouter une ligne de traçabilité en bas de chaque fichier modifié :
  > Enrichi le <DATE> suite à <audit <domaine> / diagnostic <titre> / planification <feature>
    / ticket <ID> / review <branche> / QA <ticket> / pathfinder <feature>> — <agent-id>
- Ne pas modifier la structure des fichiers (sections, titres)

## Enrichissements à appliquer

### ONBOARDING.md
<liste des enrichissements identifiés à l'ÉTAPE 2, avec section cible et action>

### CONVENTIONS.md
<liste des enrichissements identifiés à l'ÉTAPE 2, avec section cible et action>

### docs/context/technical.md
<liste des enrichissements identifiés à l'ÉTAPE 2, avec section cible et action>

### docs/context/business/<domaine>.md
<liste des enrichissements identifiés à l'ÉTAPE 2, avec fichier cible, section cible et action>

> Si un fichier n'a aucun enrichissement, ne pas le modifier.
`
})
```

---

### ÉTAPE 5 — Confirmer la délégation

Après le retour du `documentarian`, afficher :

```markdown
## ✅ Documents vivants enrichis

Le `documentarian` a enrichi les fichiers suivants :
- `ONBOARDING.md` — X modifications
- `CONVENTIONS.md` — Y modifications
- `docs/context/technical.md` — Z modifications
- `docs/context/business/<domaine>.md` — W modifications

> Ces informations seront automatiquement utilisées par les agents futurs
> (developer-*, orchestrator-dev, auditors) lors de leurs prochains accès au projet.
```

---

## Tableau de correspondance — Origine → Sections prioritaires

| Origine | ONBOARDING.md | CONVENTIONS.md | docs/context/technical.md | docs/context/business/\<domaine\>.md |
|---------|--------------|----------------|--------------------------|--------------------------------------|
| Audit sécurité | Points critiques 🔴, Zones d'ombre | Patterns (auth, validation) | Architecture, Zones d'ombre tech | Risques (domaine auth/billing) |
| Audit performance | Points importants 🟠 | — | Architecture (N+1, cache), Librairies | — |
| Audit accessibilité | Points importants 🟠 | Patterns (ARIA) | Stratégie de tests (a11y) | — |
| Audit éco-conception | — | — | Librairies (alternatives légères) | — |
| Audit architecture | Architecture, Points critiques 🔴 | — | Architecture & structure | — |
| Audit privacy | Points critiques 🔴, Zones d'ombre | Config & secrets | — | Risques (données personnelles) |
| Audit observabilité | Points importants 🟠 | Patterns (logging, alerting) | Architecture (métriques) | — |
| Diagnostic bug | Points critiques 🔴, Zones d'ombre | Patterns (gestion d'erreur) | Zones d'ombre tech | Règles de gestion, Risques |
| Planification feature | Architecture, Zones d'ombre | — | Architecture | Flux, Règles de gestion |
| Implémentation ticket (developer-*) | — | Patterns équipe, Nommage | Librairies | Règles de gestion |
| Code review (reviewer) | Zones d'ombre, Points importants 🟠 | Nommage, Patterns | — | — |
| Cycle QA (qa-engineer) | — | — | Stratégie de tests (edge cases, testabilité) | — |
| Reconnaissance rapide (pathfinder) | Architecture, Stack détectée | — | Architecture & structure | Flux, Entités |
| Re-onboarding (onboarder) | Toutes sections concernées | Toutes sections concernées | Toutes sections concernées | Toutes sections concernées |

---

## Règles de qualité des enrichissements

✅ **Factuel** : basé sur des éléments concrets observés (fichier, ligne, pattern)
✅ **Concis** : une ligne ou un bloc court par enrichissement
✅ **Contextualisé** : indiquer la source — `(audit sécurité)`, `(diagnostic auth)`, `(ticket bd-42)`, etc.
✅ **Non redondant** : vérifier que l'information n'est pas déjà présente
✅ **Actionnable** : compréhensible par un agent futur sans relire le rapport source

❌ Ne pas transmettre le rapport complet au documentarian — seulement les enrichissements ciblés
❌ Ne pas ajouter d'enrichissements subjectifs ou spéculatifs
❌ Ne pas modifier la structure des fichiers via le documentarian — uniquement le contenu
