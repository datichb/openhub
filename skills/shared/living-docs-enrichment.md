---
name: living-docs-enrichment
description: Compétence d'enrichissement des documents vivants (ONBOARDING.md, CONVENTIONS.md) — à activer après chaque rapport d'audit, diagnostic, planification, implémentation de ticket, review ou cycle QA. L'agent identifie les découvertes à capitaliser, demande confirmation à l'utilisateur, puis délègue l'écriture au documentarian via l'outil task. Aucune écriture directe.
---

# Skill — Enrichissement des Documents Vivants

## Rôle

Ce skill définit le protocole par lequel tous les agents d'analyse et d'implémentation
enrichissent de manière **incrémentale** les fichiers `ONBOARDING.md`
et `CONVENTIONS.md` à la racine du projet cible.

L'enrichissement est toujours **délégué au `documentarian`** — jamais écrit directement.
L'agent qui applique ce skill ne fait que :
1. Consolider les découvertes issues de son travail
2. Proposer l'enrichissement à l'utilisateur
3. Invoquer le `documentarian` si l'utilisateur accepte

---

## Contraintes absolues

❌ Ne jamais écrire directement dans ONBOARDING.md ou CONVENTIONS.md
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
librairies utilisées non documentées dans CONVENTIONS.md, logiques réutilisables identifiées.

### Pour le debugger

Les découvertes émergent du diagnostic (Phase 3) :
zone d'ombre de ONBOARDING.md levée par l'analyse, pattern d'erreur récurrent absent
de CONVENTIONS.md (gestion d'erreur, validation, auth), point d'attention critique
à mémoriser dans ONBOARDING.md.

### Pour les developer-*

Les découvertes émergent de l'implémentation du ticket :
pattern technique adopté pour résoudre le ticket (non documenté dans CONVENTIONS.md),
convention de nommage / structure de fichier observée ou instaurée, librairie ajoutée
ou retirée, contrainte technique découverte pendant l'implémentation (non documentée
dans ONBOARDING.md). Déclencher après chaque `bd close`.

### Pour le reviewer

Les découvertes émergent de la code review :
convention de code observée dans le diff mais absente de CONVENTIONS.md, pattern
récurrent signalé dans le rapport de review, zone d'ombre de ONBOARDING.md levée par
l'analyse du diff (ex : comportement inattendu d'un module, couplage non documenté).

### Pour le qa-engineer

Les découvertes émergent du cycle de test :
convention de test adoptée (nommage, co-location, stratégie d'isolation) non documentée
dans CONVENTIONS.md, edge case systématique révélé par les tests (pattern d'erreur à
documenter), gap de testabilité lié à l'architecture (point d'attention pour ONBOARDING.md).

### Pour le scout

Les découvertes émergent de la reconnaissance rapide :
patterns architecturaux détectés mais absents de ONBOARDING.md, conventions implicites
observées dans la codebase non documentées dans CONVENTIONS.md, stack non référencée
dans ONBOARDING.md, signaux de dette technique à mémoriser.

### Pour l'onboarder (mode enrichissement)

Applicable uniquement lorsque ONBOARDING.md et CONVENTIONS.md **existent déjà** et
que l'onboarder est invoqué en re-onboarding (voir skill `onboarder-workflow` Phase 5).
Les découvertes proviennent du rapport de re-onboarding : nouveaux patterns détectés,
stack étendue, points d'attention mis à jour, zones d'ombre résolues ou nouvelles.

---

## Workflow d'enrichissement

### ÉTAPE 1 — Identifier les enrichissements pertinents

Analyser les découvertes disponibles et les classifier :

#### Ce qui peut enrichir ONBOARDING.md

| Section cible | Découverte typique |
|---------------|--------------------|
| `## Stack détectée` | Nouvelle bibliothèque critique identifiée |
| `## Architecture` | Pattern confirmé ou infirmé par l'analyse |
| `## Points critiques 🔴` | Faille ou dette critique identifiée |
| `## Points importants 🟠` | Problème majeur non encore documenté |
| `## Améliorations suggérées 🟡` | Suggestion d'amélioration à mémoriser |
| `## Zones d'ombre` | Zone résolue (retirer) ou nouvelle zone (ajouter) |

#### Ce qui peut enrichir CONVENTIONS.md

| Section cible | Découverte typique |
|---------------|--------------------|
| `## Linting & formatting` | Incohérence entre config et code réel |
| `## Librairies & dépendances` | Bibliothèque à ne pas utiliser (CVE, alternative retenue) |
| `## Nommage` | Convention observée dans le code mais non documentée |
| `## Architecture & structure` | Patron observé dans les modules analysés |
| `## Standards de test` | Convention de test observée (co-location, nommage) |
| `## Patterns spécifiques à l'équipe` | Pattern de gestion d'erreur, auth, logging identifié |
| `## Zones d'ombre` | Zone résolue ou nouvelle zone découverte |

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
| `## Librairies & dépendances` | Ajouter "À ne pas utiliser" | "<lib> — CVE <ID> : <description courte>" |
| `## Patterns spécifiques à l'équipe` | Ajouter | "<pattern observé>" |

> Si aucun enrichissement pour un fichier, ne pas afficher la section correspondante.
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
      { label: "Oui — déléguer au documentarian (Recommandé)", description: "Invoquer le documentarian pour enrichir ONBOARDING.md et/ou CONVENTIONS.md de manière incrémentale" },
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
  description: "Enrichissement incrémental ONBOARDING.md / CONVENTIONS.md",
  prompt: `
Enrichis de manière incrémentale les fichiers ONBOARDING.md et/ou CONVENTIONS.md
à la racine du projet avec les découvertes suivantes issues d'un <audit <domaine> / diagnostic de bug / planification de feature / implémentation de ticket / code review / cycle QA / reconnaissance>.

## Règles impératives

- Enrichissement incrémental uniquement — NE PAS écraser le contenu existant
- Lire chaque fichier avant de l'enrichir
- Ajouter le contenu à la fin de la section cible concernée
- Ajouter une ligne de traçabilité en bas du fichier :
  > Enrichi le <DATE> suite à <audit <domaine> / diagnostic <titre> / planification <feature> / ticket <ID> / review <branche> / QA <ticket> / scout <feature>> — <agent-id>
- Ne pas modifier la structure des fichiers (sections, titres)

## Enrichissements à appliquer

### ONBOARDING.md
<liste des enrichissements identifiés à l'ÉTAPE 2, avec section cible et action>

### CONVENTIONS.md
<liste des enrichissements identifiés à l'ÉTAPE 2, avec section cible et action>

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

> Ces informations seront automatiquement utilisées par les agents futurs
> (developer-*, orchestrator-dev, auditors) lors de leurs prochains accès au projet.
```

---

## Tableau de correspondance — Origine → Sections prioritaires

| Origine | ONBOARDING.md — sections prioritaires | CONVENTIONS.md — sections prioritaires |
|---------|---------------------------------------|----------------------------------------|
| Audit sécurité | Points critiques 🔴, Zones d'ombre | Librairies (CVE), Patterns (auth, validation) |
| Audit performance | Points importants 🟠, Architecture | Librairies (lazy, cache), Patterns (N+1) |
| Audit accessibilité | Points importants 🟠, Améliorations 🟡 | Standards de test (a11y), Patterns (ARIA) |
| Audit éco-conception | Améliorations 🟡, Architecture | Librairies (alternatives légères), Patterns |
| Audit architecture | Architecture, Points critiques 🔴 | Architecture & structure, Patterns spécifiques |
| Audit privacy | Points critiques 🔴, Zones d'ombre | Config & secrets, Patterns (données personnelles) |
| Audit observabilité | Architecture, Points importants 🟠 | Patterns (logging, métriques, alerting) |
| Diagnostic bug | Points critiques 🔴, Zones d'ombre | Patterns (gestion d'erreur, validation) |
| Planification feature | Architecture, Zones d'ombre | Architecture & structure, Patterns spécifiques |
| Implémentation ticket (developer-*) | Architecture, Zones d'ombre | Patterns équipe, Librairies & dépendances, Nommage |
| Code review (reviewer) | Zones d'ombre, Points importants 🟠 | Nommage, Standards de test, Patterns spécifiques |
| Cycle QA (qa-engineer) | Points critiques 🔴, Architecture | Standards de test, Patterns (edge cases, testabilité) |
| Reconnaissance rapide (scout) | Architecture, Stack détectée | Architecture & structure, Patterns spécifiques |
| Re-onboarding (onboarder) | Toutes sections concernées | Toutes sections concernées |

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
