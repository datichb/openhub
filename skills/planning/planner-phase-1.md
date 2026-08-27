---
name: planner-phase-1
description: Phase 1 (exploration contextuelle — 1.1 projet/tickets, 1.2 codebase, 1.2bis librairies, 1.2ter impacts cascade, 1.3 Figma) et Phase 1.5 (délégation design) du workflow planner.
---

# Phase 1 — Exploration contextuelle

## Objectif
Explorer le projet de manière ciblée selon le type de feature demandé.

> **Si une ambiguité est détectée** sur le périmètre, les besoins ou les parties prenantes à ce stade ou pendant cette phase : charger le skill `shared/elicitation-techniques` (bucket-B) via l'outil `skill` pour choisir la technique d'élicitation adaptée avant de continuer.

## Étape 1.1 — Projet et tickets existants

```bash
# Tickets ouverts — détecter doublons potentiels et dépendances
bd list -s open --json

# Labels disponibles
bd label list-all
```

Analyser :
- Y a-t-il des tickets existants liés à la demande ? (doublons, dépendances, précédents)
- Quels labels sont disponibles pour catégoriser les nouveaux tickets ?

## Étape 1.2 — Exploration adaptative de la codebase

**Annoncer ce qui va être lu avant de le lire** :
> "Je vais explorer [fichiers/répertoires ciblés] pour contextualiser la planification."

Cibler selon la nature de la demande :

| Type de feature | Fichiers structurants à lire en priorité |
|----------------|------------------------------------------|
| API / Backend  | Routes, contrôleurs, services, use cases, modèles, migrations, DTOs |
| Frontend / UI  | Composants concernés, pages, routeur, store Pinia, composables |
| Data / ETL     | Pipelines existants, schémas, config sources/destinations |
| DevOps / Infra | Dockerfiles, CI/CD, scripts de déploiement, config env |
| Full-stack     | Combiner les deux colonnes API + Frontend |
| Transversal    | Architecture overview, config globale, README, ADR existants |

Pour chaque fichier lu, noter :
- Le **pattern architectural** utilisé (use case, port/adapter, aggregate, value object, composant présentationnel/container, etc.)
- Les **dépendances entre couches** (qui appelle qui)
- Les **points d'extension** possibles (interfaces, abstractions existantes)
- Les **tests existants** sur le périmètre concerné

## Recherche de logique existante

Pour toute feature impliquant une logique métier (calcul, transformation, comparaison, validation, règle de gestion) :

1. Identifier les mots-clés du domaine dans la demande (ex : "comparatif", "valeur", "diff", "règle", "calcul")
2. Rechercher activement dans **l'ensemble du codebase** si une logique similaire existe déjà :
   - Backend : services, use cases, value objects, helpers, DTOs avec méthodes
   - Frontend : composables, stores, utilitaires, fonctions de transformation
   - Couches partagées : types communs, libs internes, packages utilitaires
3. Si une logique existante est trouvée : la noter comme **réutilisable** et la mentionner dans le résumé de contexte
4. Si une implémentation similaire existe déjà quelque part et que la feature semble vouloir la dupliquer → **signaler le risque de duplication dans le résumé, quelle que soit la couche concernée**

## Détection des signaux design

Pendant la lecture, **détecter les signaux design** :

**Signaux UX** (au moins un → UX recommandé) :
- La feature introduit ou modifie un parcours utilisateur multi-étapes
- Elle change une interaction existante (ex : radio → checkbox, inline → modal, étape → page dédiée)
- Elle touche un formulaire avec validation, soumission ou gestion d'erreurs non triviale
- Elle implique un flow critique (inscription, paiement, confirmation irréversible)
- Des questions sur "ce que voit l'utilisateur" restent ouvertes après l'exploration

**Signaux UI** (au moins un → UI recommandé) :
- Un composant Vue est modifié en profondeur (structure, props, événements)
- Un nouveau composant visuel est à créer
- Des variantes visuelles ou des états (hover, focus, disabled, error, loading) doivent être spécifiés
- Le design system (DSFR ou interne) est sollicité et les bons composants à utiliser ne sont pas évidents

Lire les fichiers, puis proposer d'aller plus loin si pertinent :
> "J'ai lu [X, Y, Z]. Je pourrais aussi explorer [A, B] si utile."

**⏸️ Ne pas attendre de réponse ici** — continuer directement avec le résumé.

## Déclencheur de pause ⏸️

Si une **information critique** émerge pendant l'exploration qui remet en cause le périmètre ou les hypothèses de départ → utiliser le format de pause inter-étape (contexte en texte + question).

## Étape 1.2bis — Analyse des librairies externes (conditionnelle)

**Déclencheur** : si la feature implique le comportement non trivial d'une librairie externe déjà présente dans le codebase — utilisation d'un hook, option d'une méthode, événement spécifique, side effect documenté, comportement version-dépendant.

**Ne pas déclencher si** : la lib n'est utilisée que de façon triviale (imports de types, utilitaires simples), ou si le comportement concerné est déjà parfaitement documenté dans le codebase (tests, commentaires, ADR existants).

**Pour chaque librairie identifiée comme concernée :**

1. Formuler précisément le comportement supposé ou l'usage prévu
2. Lancer une websearch ciblée :
   ```
   "[lib] [méthode/option/hook] [comportement attendu] [version ou année]"
   ```
3. Classer le résultat :
   - **Vérifié** ✅ : comportement confirmé par la doc officielle ou une source fiable
   - **Partiellement vérifié** ⚠️ : comportement probable mais dépendant de la version ou du contexte — préciser la condition
   - **Supposition** ❌ : pas de source trouvée — documenter l'hypothèse explicitement

> ⚠️ **Règle absolue** : ne jamais écrire dans un ticket une description qui suppose le comportement d'une librairie sans l'avoir vérifié. Si la vérification est impossible (websearch sans résultat), le noter comme hypothèse dans les notes du ticket avec le label `needs-clarification`.

**Si une supposition s'avère incorrecte après recherche** → déclencheur de pause ⏸️ avec description de l'impact sur le périmètre du plan.

---

## Étape 1.2ter — Cartographie des impacts en cascade (conditionnelle)

**Déclencheur** : si au moins un fichier **partagé** sera modifié — service, composable, type TypeScript, utilitaire, DTO, interface, repository, store Pinia, hook React, mixin.

**Pour chaque fichier/interface identifié comme à modifier :**

1. Rechercher ses consommateurs directs dans le codebase (imports, injections, usages)
2. Pour chaque consommateur, évaluer :
   - **Impact neutre** : le consommateur n'est pas affecté (pas de changement de signature ni de comportement observable)
   - **Adapté automatiquement** : le consommateur devra être mis à jour — déjà inclus dans un ticket prévu
   - **Ticket séparé nécessaire** : l'impact sur ce consommateur n'est pas couvert par le plan actuel → noter comme futur ticket potentiel

**Format de l'impact map :**
```
[Fichier modifié]   → [Consommateurs identifiés]              → [Évaluation]
UserService         → AuthController, ProfileUseCase            → impact neutre (signature inchangée)
UserDTO             → 4 endpoints + 2 composants Vue            → ticket séparé nécessaire
useFilters.ts       → FilterPanel.vue, SearchPage.vue           → adapté automatiquement (dans ticket bd-X)
```

Les entrées **"ticket séparé nécessaire"** alimentent directement la Phase 3 — elles doivent apparaître dans le plan hiérarchique.

**⏸️ Ne pas attendre de réponse** — continuer avec l'impact map et l'intégrer dans le récap Phase 1.

---

## Étape 1.3 — Exploration Figma (optionnelle)

**Déclencheur** : Si au moins un de ces critères est vrai après l'Étape 1.2 :
- La feature mentionne des composants UI (bouton, formulaire, page, modal, etc.)
- La feature touche l'interface utilisateur
- Des composants Vue/React ont été identifiés en Étape 1.2

**Si le déclencheur est activé :**

> Déléguer à l'agent `designer` avec `Mode: recon` (Phase 1.3 — Exploration Figma).

Ce skill prescrit exactement :
1. `search_figma_files` — rechercher des maquettes liées à la feature
2. `get_file_structure` + `detect_ui_signals` — analyser chaque fichier trouvé (max 3)
3. Enrichir le récap Phase 1 avec les données Figma (URLs, frames, composants, signaux UX/UI)

**Si aucun signal UI / aucun critère activé :** passer directement au récap Phase 1 en notant "Aucune exploration Figma — feature sans composants UI détectés".

**⏸️ Ne pas attendre de réponse** — exécuter l'exploration Figma et intégrer les résultats dans le récap.

## Récap de fin de Phase 1

```markdown
## [Phase 1] Exploration contextuelle terminée

**Fichiers explorés :** X fichiers lus
- <fichier 1 — raison de la lecture>
- <fichier 2 — raison de la lecture>
- ...

**Observations principales :**
- Architecture : <pattern détecté — ex : Clean Architecture, use cases>
- Stack : <langages/frameworks identifiés>
- Conventions : <nommage, tests, structure détectés>
- Tests existants : <état de la couverture sur le périmètre>

**Tickets existants liés :**
- bd-X : <titre> — <lien avec la demande>
- (aucun si vide)

**Maquettes Figma explorées :**
- **<Nom fichier>** — <URL Figma> — Frames : X, Composants : Y
- (aucune maquette trouvée — si applicable)

**Signaux design détectés :**
- **UX** : <oui ⚠️ / non> — <raison si oui>
- **UI** : <oui ⚠️ / non> — <raison si oui, avec source : codebase ou Figma>

**Logiques existantes réutilisables :**
- <nom logique> → <fichier:ligne> — <description courte> — <couche>
- Risque de duplication : <oui ⚠️ / non>

**Librairies externes analysées (étape 1.2bis) :**
- `<lib@version>` — comportement de `<méthode/option>` : ✅ vérifié / ⚠️ partiel / ❌ supposé
  → Source : <URL ou "aucune source trouvée — hypothèse documentée">
- (étape non déclenchée — aucune lib externe non triviale concernée)

**Impact en cascade (étape 1.2ter) :**
- `<fichier modifié>` → consommateurs : <liste> → <neutre / adapté (ticket bd-X) / ticket séparé nécessaire>
- (étape non déclenchée — aucun fichier partagé modifié)

**Zones d'ombre identifiées :**
- <zone 1 — ce qui n'a pas pu être déterminé depuis le codebase>
- <zone 2>

**Dépendances techniques identifiées :**
- <dépendance 1 — ex : le module auth n'existe pas encore>

**Risques détectés :**
- <risque 1 — ex : conflit potentiel avec feature en cours>

**Points d'attention :**
- <point 1 — ex : pas de tests sur le module concerné>
```

## Question de validation obligatoire

**Si CONTEXTE = standalone :**

Si **signaux UX ou UI détectés** (depuis codebase ou Figma) :
```
question({
  questions: [{
    header: "Délégation design",
    question: "[Planner — Phase 1 complétée | Feature : <nom>]\n\n**Résumé de l'exploration (X fichiers lus) :**\n- Architecture : <pattern détecté — ex : Clean Architecture, composants Vue>\n- Tests existants : <état — ex : couverture partielle sur le périmètre>\n- Signal <UX/UI> détecté : <raison concrète — ex : nouveau composant formulaire multi-étapes>\n- Zones d'ombre : <liste courte — ex : comportement modal non documenté>\n\nComment procéder ?",
    options: [
      { label: "Phase 1.5 — Délégation design (Recommandé)", description: "Invoquer l'agent designer (mode ux/ui/ux+ui) avant de planifier" },
      { label: "Skip design — Phase 2", description: "Passer aux questions complémentaires sans spec design" },
      { label: "Explorer davantage", description: "Lire d'autres fichiers avant de décider" }
    ]
  }]
})
```

Si **aucun signal design** :
```
question({
  questions: [{
    header: "Questions complémentaires",
    question: "[Planner — Phase 1 complétée | Feature : <nom>]\n\n**Résumé de l'exploration (X fichiers lus) :**\n- Architecture : <pattern détecté>\n- Tests existants : <état>\n- Aucun signal design détecté\n- Zones d'ombre : <liste courte ou 'Aucune'>\n- Tickets existants liés : <IDs ou 'Aucun'>\n\nPasser aux questions complémentaires (Phase 2) ?",
    options: [
      { label: "Passer à Phase 2 (Recommandé)", description: "Poser les questions de clarification identifiées" },
      { label: "Explorer davantage", description: "Lire d'autres fichiers avant de poser des questions" }
    ]
  }]
})
```

**Selon la réponse (dans tous les contextes) :**
- **Phase 1.5** → Phase 1.5 (délégation design)
- **Phase 2** → Phase 2 (questions complémentaires)
- **Explorer davantage** → rester en Phase 1, explorer plus, re-produire le récap

---

# Phase 1.5 — Délégation design (si signaux détectés)

SI signaux UX ou UI détectés à la Phase 1 :
→ Invoquer l'agent `designer` avec le champ `Mode: recon | ux | ui | ux+ui` approprié.
→ Charger le skill `design/design-planner-format` pour les templates de délégation et formats de retour.

**Choix du mode :**
- Signal UX uniquement → `Mode: ux`
- Signal UI uniquement → `Mode: ui`
- Les deux → `Mode: ux+ui`
- Besoin de reconnaissance Figma légère d'abord → `Mode: recon`

Le planner n'a pas accès au MCP Figma — toute exploration Figma est déléguée au `designer`.
