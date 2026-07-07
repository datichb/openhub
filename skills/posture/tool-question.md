---
name: tool-question
description: Utilisation de l'outil question d'OpenCode — quand et comment poser des questions structurées à l'utilisateur via l'interface interactive plutôt que dans le texte brut. Couvre le multi-questions, la multi-sélection et la gestion de la saisie libre.
---

# Skill — Outil `question` (OpenCode)

## Rôle

Ce skill définit quand et comment utiliser l'outil **`question`** d'OpenCode pour
interagir avec l'utilisateur de façon structurée, en présentant des choix clairs
plutôt qu'une question ouverte dans le texte.

---

## Schéma de l'outil

L'outil `question` accepte un paramètre **`questions`** qui est un **tableau** — tu peux poser une ou plusieurs questions en un seul appel.

```
question({
  questions: [
    {
      header: "...",      // label court (max 30 caractères)
      question: "...",    // formulation complète
      options: [...],     // 2 à 5 choix
      multiple: false     // optionnel — true pour multi-sélection
    }
  ]
})
```

### Paramètres d'une question

| Paramètre | Requis | Type | Description |
|-----------|--------|------|-------------|
| `header` | ✅ | string | Label court (max 30 caractères), identifie le sujet en un coup d'œil |
| `question` | ✅ | string | Formulation complète et sans ambiguïté de ce qui est demandé |
| `options` | ✅ | array | Liste de 2 à 5 choix (voir structure ci-dessous) |
| `multiple` | ❌ | boolean | `true` pour permettre la sélection de plusieurs options (défaut: `false`) |

### Structure d'une option

| Champ | Requis | Description |
|-------|--------|-------------|
| `label` | ✅ | Texte affiché (1 à 5 mots, concis) |
| `description` | ✅ | Explication courte de ce que ce choix implique |

---

## Quand utiliser `question`

Utiliser l'outil `question` (et non une question en texte libre) dans ces situations :

- **Choix entre plusieurs options** — ex. : mode de workflow, stratégie d'implémentation,
  type de branche, format de sortie
- **Confirmation d'une action à risque** — ex. : suppression, remplacement, migration
- **Collecte d'une préférence** — ex. : langue cible, niveau de détail, priorité
- **Ambiguïté dans les instructions** — ex. : deux interprétations possibles d'une demande
- **Décision qui bloque la suite** — ex. : CP-1, CP-2 dans les workflows orchestrateur
- **Plusieurs décisions liées** — poser toutes les questions indépendantes en un seul appel

---

## Quand NE PAS utiliser `question`

- Pour des informations factuelles déjà disponibles dans le contexte ou le codebase
- Pour des clarifications mineures qui peuvent être résolues par une hypothèse raisonnable
  (indiquer l'hypothèse dans la réponse)
- Quand la réponse n'influence pas le résultat final

---

## Option de saisie libre

Par défaut, OpenCode ajoute automatiquement une option **"Type your own answer"** à chaque question.
Cette option permet à l'utilisateur de saisir une réponse libre si aucune des options proposées ne convient.

### Comportement

| Aspect | Comportement |
|--------|--------------|
| Activation | Automatique par défaut (`custom: true` implicite) |
| Texte de l'option | Toujours "Type your own answer" — **non personnalisable** |
| Désactivation | Non documentée dans le schéma public |

### Conséquences pour les agents

- ✅ Les réponses peuvent être des valeurs **hors-liste** — toujours prévoir la gestion de réponses libres
- ✅ L'option est ajoutée automatiquement — **ne pas la dupliquer** manuellement
- ❌ Ne pas ajouter d'option "Autre", "Personnalisé" ou catch-all — ce serait redondant
- ❌ Si une réponse libre n'est pas acceptable pour le workflow, le préciser dans la description de la question

---

## Exemples d'usage

### Question simple — Choix de mode

```
question({
  questions: [{
    header: "Mode de workflow",
    question: "Quel mode de workflow pour cette session ?",
    options: [
      { label: "Manuel (Recommandé)", description: "Chaque étape attend ta confirmation" },
      { label: "Semi-auto", description: "Démarre et enchaîne automatiquement, QA et review restent manuels" },
      { label: "Auto", description: "Workflow entièrement automatique sauf les décisions de commit" }
    ]
  }]
})
```

### Confirmation d'action risquée

```
question({
  questions: [{
    header: "Suppression fichier",
    question: "Le fichier src/legacy/old-service.ts sera supprimé. Confirmes-tu ?",
    options: [
      { label: "Oui, supprimer", description: "Le fichier sera supprimé — irréversible sans git restore" },
      { label: "Non, conserver", description: "La suppression est annulée, le fichier reste en place" }
    ]
  }]
})
```

### Multi-sélection — Choix de features

```
question({
  questions: [{
    header: "Features à implémenter",
    question: "Quelles features veux-tu inclure dans ce sprint ?",
    multiple: true,
    options: [
      { label: "Authentification", description: "Login/logout avec JWT" },
      { label: "Dashboard", description: "Page d'accueil avec métriques" },
      { label: "Notifications", description: "Système de notifications push" },
      { label: "Export PDF", description: "Export des rapports en PDF" }
    ]
  }]
})
```

### Multi-questions — Décisions liées en un seul appel

Quand plusieurs décisions sont indépendantes mais liées au même contexte, les poser en un seul appel :

```
question({
  questions: [
    {
      header: "Mode de workflow",
      question: "Quel mode de workflow pour cette session ?",
      options: [
        { label: "Manuel (Recommandé)", description: "Confirmation à chaque étape" },
        { label: "Auto", description: "Automatique sauf commits" }
      ]
    },
    {
      header: "Branche dédiée",
      question: "Créer une branche dédiée pour ce ticket ?",
      options: [
        { label: "Oui (Recommandé)", description: "Crée feat/bd-42-mon-ticket avant de démarrer" },
        { label: "Non", description: "Rester sur la branche courante" }
      ]
    },
    {
      header: "Tests",
      question: "Générer les tests unitaires ?",
      options: [
        { label: "Oui", description: "Génère les tests avec le code" },
        { label: "Non", description: "Code uniquement, tests manuels" }
      ]
    }
  ]
})
```

### Multi-questions avec multi-sélection combinée

```
question({
  questions: [
    {
      header: "Audits à lancer",
      question: "Quels audits veux-tu lancer sur ce projet ?",
      multiple: true,
      options: [
        { label: "Sécurité", description: "Audit OWASP, CVE, failles connues" },
        { label: "Performance", description: "Web Vitals, N+1, lazy loading" },
        { label: "Accessibilité", description: "WCAG 2.1 AA, RGAA" }
      ]
    },
    {
      header: "Priorité corrections",
      question: "Comment prioriser les corrections ?",
      options: [
        { label: "Critiques d'abord (Recommandé)", description: "Traiter les P0/P1 avant les P2/P3" },
        { label: "Par audit", description: "Finir un audit avant de passer au suivant" }
      ]
    }
  ]
})
```

---

## Questions posées en tant que sous-agent

Quand tu es invoqué par un agent parent (orchestrateur, orchestrator-dev, etc.) et que tu dois
poser une question à l'utilisateur, **le champ `question` doit toujours commencer par un bloc
de contexte** sur une ligne dédiée, afin que l'utilisateur comprenne sans avoir à consulter la
session enfant.

### Format obligatoire

```
[<Nom de l'agent> — <Phase ou étape en cours> | <Feature ou ticket concerné>]
<Question proprement dite>
```

### Format enrichi — obligatoire quand CONTEXTE = orchestrator_feature

Quand tu es invoqué via l'outil `task` depuis un orchestrateur, le texte affiché dans ta session
enfant **n'est pas visible** par l'utilisateur dans la session parent. La question est le **seul
contenu qui remonte** dans la session parent.

Pour cette raison, **si CONTEXTE = orchestrator_feature**, le champ `question` doit inclure
un **condensé structuré des découvertes clés** de la phase en cours (3-5 points maximum) :

```
[<Nom de l'agent> — <Phase> | <Feature>]

**<Titre du condensé — ex : "Résumé de l'exploration" ou "Contexte de la pause"> :**
- <découverte clé 1>
- <découverte clé 2>
- <découverte clé 3>

<Question proprement dite>
```

**Règles du condensé :**
- 3 à 5 points maximum — rester actionnable, pas exhaustif
- Inclure uniquement les informations qui influencent la décision demandée
- Reprendre les termes exacts (noms de fichiers, patterns, warnings) — ne pas paraphraser
- Le récap complet reste affiché en texte dans la session enfant pour la traçabilité

### Exemple — standalone

```
question({
  questions: [{
    header: "Validation du contexte",
    question: "[Planner — Phase 0 : Validation du contexte | Feature : authentification JWT]\nCe contexte correspond-il à votre projet ? Des corrections avant de continuer ?",
    options: [
      { label: "Oui — continuer (Recommandé)", description: "Lancer la phase de discovery" },
      { label: "Corrections à apporter", description: "Préciser ou corriger le contexte avant de continuer" }
    ]
  }]
})
```

### Exemple — invoqué depuis l'agent orchestrator (CONTEXTE = orchestrator_feature)

```
question({
  questions: [{
    header: "Phase 1 complétée",
    question: "[Planner — Phase 1 complétée | Feature : authentification JWT]\n\n**Résumé de l'exploration (8 fichiers lus) :**\n- Architecture : Clean Architecture — use cases + repositories\n- Tests existants : couverture partielle (42%), module auth non couvert\n- Signal UX détecté : formulaire multi-étapes avec validation complexe\n- Zones d'ombre : stratégie refresh tokens non documentée\n\nComment procéder ?",
    options: [
      { label: "Phase 1.5 — Délégation design (Recommandé)", description: "Invoquer designer (Mode: ux) avant de planifier" },
      { label: "Skip design — Phase 2", description: "Passer aux questions sans spec design" }
    ]
  }]
})
```

### Règles

✅ Toujours inclure le bloc de contexte quand tu es invoqué en tant que sous-agent
✅ Le bloc de contexte doit identifier : qui tu es, quelle phase est en cours, la feature ou le ticket concerné
✅ **Si CONTEXTE = orchestrator_feature** : enrichir avec un condensé des découvertes clés (3-5 points)
❌ Ne pas omettre le contexte même pour une question courte — l'utilisateur n'a pas accès à ta session enfant sans naviguer manuellement
❌ Ne pas inclure le récap complet dans le champ `question` — uniquement le condensé actionnable

---

## Format des réponses

Les réponses sont retournées sous forme de **tableau de labels**.

### Question simple (multi-sélection désactivée)

Réponse : `["Manuel (Recommandé)"]`

### Multi-sélection activée

Réponse : `["Authentification", "Dashboard", "Export PDF"]`

### Réponse libre (saisie utilisateur)

Réponse : `["<texte saisi par l'utilisateur>"]`

### Multi-questions

Les réponses sont retournées dans l'ordre des questions posées.

---

## Règles récapitulatives

| Règle | ✅ / ❌ |
|-------|--------|
| Poser plusieurs questions liées et indépendantes en un seul appel | ✅ |
| Utiliser `multiple: true` quand l'utilisateur peut choisir plusieurs options | ✅ |
| Mettre l'option recommandée **en premier** avec `(Recommandé)` dans le label | ✅ |
| Prévoir la gestion des réponses libres (toujours possibles) | ✅ |
| Poser une question déjà répondue dans la session | ❌ |
| Reformuler la même question deux fois sans nouvelle information | ❌ |
| Ajouter une option "Autre" ou "Personnalisé" | ❌ |
| Dépasser 5 options par question (rend le choix difficile) | ❌ |
