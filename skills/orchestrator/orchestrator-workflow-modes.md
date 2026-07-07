---
name: orchestrator-workflow-modes
description: Source de vérité unique pour les trois modes de workflow (manuel/semi-auto/auto) — tableau des comportements par checkpoint, règles absolues, et bloc question canonique pour le choix du mode. Injecté dans orchestrator et orchestrator-dev pour garantir la cohérence des descriptions et éviter toute désynchro.
---

# Skill — Modes de workflow

Ce skill est la **source de vérité unique** pour les trois modes de workflow.
Il est injecté dans `orchestrator` et `orchestrator-dev` — toute modification ici s'applique aux deux agents.

---

## Les trois modes

| Mode | CP-0 | CP-1 | CP-2 | CP-3 |
|------|------|------|------|------|
| `manuel` _(défaut)_ | ⏸️ pause | ⏸️ pause | ⏸️ pause | ⏸️ pause |
| `semi-auto` | ⏸️ pause | ▶️ auto | ⏸️ **pause** | ▶️ auto |
| `auto` | ⏸️ pause | ▶️ auto | ⏸️ **pause** | ▶️ auto |

> Quand un CP est `▶️ auto`, l'agent orchestrator affiche quand même l'information mais enchaîne sans attendre de confirmation.

> **Parallélisme conditionnel (mode `auto` uniquement) :** en mode `auto`, `orchestrator-dev` peut traiter plusieurs tickets simultanément si les 4 critères sont vérifiés : aucune dépendance formelle entre les tickets du lot, agents distincts avec domaines disjoints, pas de fichiers transverses prévisibles, maximum 3 tickets. Le parallélisme ne supprime pas CP-2 — les rapports de review sont présentés en séquentiel dans l'ordre d'arrivée. Voir `orchestrator-dev-protocol` pour le protocole complet.
>
> **Isolation filesystem via worktrees (`worktree.enabled = true`) :** si les worktrees sont activés pour le projet, l'étape 1b utilise `git worktree add` au lieu de `git checkout -b`. Chaque ticket reçoit un répertoire isolé `.worktrees/<slug>/` — les agents `developer-*` travaillent dans leur worktree sans risque de conflit filesystem. À CP-2 après commit validé, le worktree est proposé à la suppression.
>
> **Création des worktrees en mode parallèle — responsabilité de l'orchestrator-dev :** quand le parallélisme est actif et que `worktree.enabled = true`, l'orchestrator-dev **doit créer tous les worktrees lui-même, séquentiellement, avant de lancer les sessions parallèles**. Déléguer la création aux developer agents provoquerait une contention sur `.git/index.lock` (les `git worktree add` concurrents échouent en silence et les agents tombent sur le répertoire partagé, annulant toute isolation). Voir la "Phase 0" dans `orchestrator-dev-protocol` pour le protocole exact.

---

## Règles absolues sur les modes

- **Le mode par défaut est `manuel`** si rien n'est précisé.
- **CP-2 (commit ou corriger ?) est une pause dans TOUS les modes sans exception** — cette règle ne peut pas être outrepassée, même en mode `auto`.
- Ne jamais passer en mode `semi-auto` ou `auto` sans que ce mode ait été choisi explicitement par l'utilisateur.
- L'utilisateur peut taper "stop" à n'importe quel moment — tous les modes honorent cette commande.
- **En mode `auto`, le circuit breaker global de session s'applique** (voir ci-dessous).

## Circuit breaker global — mode auto

En mode `auto`, surveiller le compteur de délégations consécutives sans interaction utilisateur.

**Limite : 12 invocations `task` consécutives sans checkpoint manuel.**

Au 12ème `task` consécutif sans interaction utilisateur (CP-2 exclu, il est toujours manuel) :

```
question({
  questions: [{
    header: "Circuit breaker auto",
    question: "12 délégations consécutives sans interaction. Le mode auto continue sans supervision.\n\nVoulez-vous continuer ?",
    options: [
      { label: "Continuer en mode auto", description: "Poursuivre sans supervision" },
      { label: "Pause — review en cours", description: "Afficher l'état et attendre une instruction" },
      { label: "Arrêter la session", description: "Terminer le workflow immédiatement" }
    ]
  }]
})
```

Réinitialiser le compteur à chaque interaction utilisateur (réponse à un CP-2 ou à une question manuelle).

---

## Configuration projet (opencode.json)

Le mode de workflow peut être pré-configuré dans `opencode.json` pour éviter la question interactive au CP-0.

### Format de configuration

```json
{
  "workflow": {
    "defaultMode": "semi-auto"
  }
}
```

### Propriétés

| Propriété | Type | Valeurs valides | Description |
|-----------|------|-----------------|-------------|
| `defaultMode` | string | `"manuel"`, `"semi-auto"`, `"auto"` | Mode de workflow appliqué automatiquement au CP-0 |

### Comportement

- **`defaultMode` présent et valide** → le mode est appliqué sans question, un message confirme : "Mode de workflow : `<mode>` (configuré dans opencode.json)"
- **`defaultMode` absent ou invalide** → la question CP-0 est posée normalement (compatibilité ascendante)

---

## Bloc question — Choix du mode

### Détection de la configuration projet

La configuration de workflow est injectée automatiquement dans la session au démarrage via le champ `instructions` de `opencode.json`.

Si `workflow.defaultMode` est disponible dans le contexte de session :
- Valeur valide (`"manuel"`, `"semi-auto"`, `"auto"`) → appliquer le mode sans poser la question, afficher :
  > Mode de workflow : `<mode>` (configuré dans opencode.json)
- Valeur absente ou invalide → poser la question normalement

> ❌ Ne jamais utiliser l'outil `read` pour accéder à `opencode.json` — le contexte est injecté automatiquement dans la session. Si la valeur n'est pas disponible dans la session, elle est absente : poser la question.
>
> **Note pour `orchestrator-dev`** : si `opencode.json` est explicitement autorisé en lecture directe (permission `read.opencode.json: allow` dans le frontmatter), `orchestrator-dev` peut utiliser l'outil `read` pour lire ce fichier. Cette restriction ne s'applique qu'à l'agent orchestrator feature qui n'a pas cette permission.

### Question interactive (si non configuré)

Utiliser ce bloc exact via l'outil `question` pour demander le mode :

```
question({
  questions: [{
    header: "Mode de workflow",
    question: "Quel mode de workflow pour les phases d'implémentation ?",
    options: [
      { label: "Manuel (Recommandé)", description: "Chaque étape attend ta confirmation — CP-1, CP-2, CP-3 tous en pause" },
      { label: "Semi-auto", description: "CP-1 et CP-3 automatiques, CP-2 (commit) reste manuel" },
      { label: "Auto", description: "Workflow entièrement automatique sauf CP-2 (commit) — parallélisme conditionnel disponible pour les tickets indépendants" }
    ]
  }]
})
```

Enregistrer le mode pour toute la session.

---

## Comportement selon le contexte d'invocation

- **Invoqué standalone** : demander le mode via le bloc question ci-dessus au CP-0.
- **Invoqué depuis l'agent orchestrator feature** : le mode est transmis dans le texte du prompt — ne pas redemander le mode, démarrer directement avec la valeur reçue.

  **Format de transmission requis :** le mode doit figurer dans le prompt sous l'une des trois valeurs canoniques exactes suivantes :
  - `manuel`
  - `semi-auto`
  - `auto`

  Ne jamais transmettre le label brut de l'option d'interface (ex : `"Manuel (Recommandé)"`, `"Semi-auto"`) — normaliser en minuscule avant transmission. Exemple de formulation correcte dans le prompt : `Mode de workflow : semi-auto`.

---

## Option Batch CP-2 (mode `auto` avec parallélisme uniquement)

### Contexte d'application

Cette option ne s'applique que lorsque :
- Le mode de workflow est `auto`
- Le parallélisme conditionnel est actif (4 critères vérifiés)
- Plusieurs tickets atteignent CP-2 simultanément

### Comportement

Lorsque N tickets arrivent au CP-2 avec leur rapport de review :

1. **Évaluation des verdicts** : collecter le verdict (`commit`, `corriger`, `corriger-sécurité`) de chaque rapport
2. **Décision automatique** :
   - **Tous `commit`** → proposer le batch groupé
   - **Au moins un `corriger`** → éclater le batch, traitement séquentiel

### Options du batch (si tous les verdicts sont `commit`)

| Option | Comportement |
|--------|--------------|
| **Commit tous** | Commiter les N tickets en séquence avec leurs messages Conventional Commits respectifs. Tous les tickets sont clos à la fin. |
| **Commit sélectif** | Afficher la liste des tickets (sélection multiple), permettre de choisir lesquels commiter. Les tickets non sélectionnés retournent en mode séquentiel standard. Si aucun ticket sélectionné, retour au choix précédent. |
| **Voir détails** | Passer en mode séquentiel standard : afficher chaque rapport un par un et recueillir une décision individuelle. |

### Éclatement du batch

Si au moins un ticket du lot a un verdict `corriger` ou `corriger-sécurité`, le batch est automatiquement éclaté :
- Les tickets sont présentés un par un dans l'ordre d'arrivée
- Chaque ticket reçoit une décision individuelle (commit ou corriger)
- Le ticket avec verdict `corriger` est retourné au developer avec les corrections requises

### Règle absolue préservée

Le batch ne supprime pas CP-2 — il regroupe la validation pour les cas homogènes.
CP-2 reste une **pause obligatoire** dans tous les modes, batch ou non.
