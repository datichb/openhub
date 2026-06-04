---
name: orchestrator-workflow-modes
description: Source de vérité unique pour les trois modes de workflow (manuel/semi-auto/auto) — tableau des comportements par checkpoint, règles absolues, et blocs question canoniques pour le choix du mode et du QA global. Injecté dans orchestrator et orchestrator-dev pour garantir la cohérence des descriptions et éviter toute désynchro.
---

# Skill — Modes de workflow

Ce skill est la **source de vérité unique** pour les trois modes de workflow.
Il est injecté dans `orchestrator` et `orchestrator-dev` — toute modification ici s'applique aux deux agents.

---

## Les trois modes

| Mode | CP-0 | CP-1 | CP-QA | CP-2 | CP-3 |
|------|------|------|-------|------|------|
| `manuel` _(défaut)_ | ⏸️ pause | ⏸️ pause | ⏸️ pause | ⏸️ pause | ⏸️ pause |
| `semi-auto` | ⏸️ pause | ▶️ auto | ⏸️ pause | ⏸️ **pause** | ▶️ auto |
| `auto` | ⏸️ pause (+ choix QA global) | ▶️ auto | ▶️ valeur fixée en CP-0 | ⏸️ **pause** | ▶️ auto |

> Quand un CP est `▶️ auto`, l'orchestrateur affiche quand même l'information mais enchaîne sans attendre de confirmation.

> **Parallélisme conditionnel (mode `auto` uniquement) :** en mode `auto`, `orchestrator-dev` peut traiter plusieurs tickets simultanément si les 4 critères sont vérifiés : aucune dépendance formelle entre les tickets du lot, agents distincts avec domaines disjoints, pas de fichiers transverses prévisibles, maximum 3 tickets. Le parallélisme ne supprime pas CP-2 — les rapports de review sont présentés en séquentiel dans l'ordre d'arrivée. Voir `orchestrator-dev-protocol` pour le protocole complet.
>
> **Isolation filesystem via worktrees (`worktree.enabled = true`) :** si les worktrees sont activés pour le projet, l'étape 1b utilise `git worktree add` au lieu de `git checkout -b`. Chaque ticket reçoit un répertoire isolé `.worktrees/<slug>/` — les agents `developer-*` travaillent dans leur worktree sans risque de conflit filesystem. À CP-2 après commit validé, le worktree est proposé à la suppression.

---

## Règles absolues sur les modes

- **Le mode par défaut est `manuel`** si rien n'est précisé.
- **CP-2 (commit ou corriger ?) est une pause dans TOUS les modes sans exception** — cette règle ne peut pas être outrepassée, même en mode `auto`.
- Ne jamais passer en mode `semi-auto` ou `auto` sans que ce mode ait été choisi explicitement par l'utilisateur.
- L'utilisateur peut taper "stop" à n'importe quel moment — tous les modes honorent cette commande.

---

## Configuration projet (opencode.json)

Le mode de workflow peut être pré-configuré dans `opencode.json` pour éviter la question interactive au CP-0.

### Format de configuration

```json
{
  "workflow": {
    "defaultMode": "semi-auto",
    "qaEnabled": false
  }
}
```

### Propriétés

| Propriété | Type | Valeurs valides | Description |
|-----------|------|-----------------|-------------|
| `defaultMode` | string | `"manuel"`, `"semi-auto"`, `"auto"` | Mode de workflow appliqué automatiquement au CP-0 |
| `qaEnabled` | boolean | `true`, `false` | QA global activé/désactivé (utilisé uniquement si `defaultMode` = `"auto"`) |

### Comportement

- **`defaultMode` présent et valide** → le mode est appliqué sans question, un message confirme : "Mode de workflow : `<mode>` (configuré dans opencode.json)"
- **`defaultMode` absent ou invalide** → la question CP-0 est posée normalement (compatibilité ascendante)
- **`qaEnabled` défini (en mode `auto`)** → le QA global est configuré sans question, un message confirme : "QA global : `<activé/désactivé>` (configuré dans opencode.json)"
- **`qaEnabled` non défini (en mode `auto`)** → la question QA global est posée normalement

> **Note :** La propriété `qaEnabled` n'est lue et appliquée que si le mode est `auto`. Elle est ignorée pour les modes `manuel` et `semi-auto`.

---

## Bloc question — Choix du mode

### Détection de la configuration projet

Avant de poser la question, vérifier si `opencode.json` contient une configuration `workflow.defaultMode` :

1. Lire `opencode.json` à la racine du projet
2. Vérifier si `workflow.defaultMode` existe et contient une valeur valide : `"manuel"`, `"semi-auto"`, ou `"auto"`
3. **Si valide** → appliquer le mode sans poser la question, afficher :
   > Mode de workflow : `<mode>` (configuré dans opencode.json)
4. **Si absent, invalide, ou `opencode.json` inexistant** → poser la question normalement

> **Cas d'erreur :** si `opencode.json` existe mais contient du JSON invalide (erreur de parsing) ou si une erreur d'accès fichier survient → traiter comme "absent" et poser la question normalement.

### Question interactive (si non configuré)

Utiliser ce bloc exact via l'outil `question` pour demander le mode :

```
question({
  questions: [{
    header: "Mode de workflow",
    question: "Quel mode de workflow pour les phases d'implémentation ?",
    options: [
      { label: "Manuel (Recommandé)", description: "Chaque étape attend ta confirmation — CP-1, CP-QA, CP-2, CP-3 tous en pause" },
      { label: "Semi-auto", description: "CP-1 et CP-3 automatiques, CP-QA et CP-2 (commit) restent manuels" },
      { label: "Auto", description: "Workflow entièrement automatique sauf CP-2 (commit) — QA configurable au démarrage — parallélisme conditionnel disponible pour les tickets indépendants" }
    ]
  }]
})
```

Enregistrer le mode pour toute la session.

---

## Bloc question — QA global (mode `auto` uniquement)

### Détection de la configuration projet

Avant de poser la question, vérifier si `opencode.json` contient une configuration `workflow.qaEnabled` :

1. Vérifier si `workflow.qaEnabled` existe et est **strictement un booléen JSON** (`true` ou `false`)
2. **Si défini** → appliquer la valeur sans poser la question, afficher :
   > QA global : `<activé/désactivé>` (configuré dans opencode.json)
3. **Si absent ou non booléen** → poser la question normalement

> **Type strict :** seuls les booléens JSON `true` et `false` sont acceptés. Toute autre valeur (string `"true"`, nombre `1` ou `0`, `null`, etc.) est ignorée et déclenche la question interactive.

### Question interactive (si non configuré)

En mode `auto`, poser également via l'outil `question` :

```
question({
  questions: [{
    header: "QA global",
    question: "QA activé pour tous les tickets d'implémentation à risque moyen/faible ?",
    options: [
      { label: "Oui (Recommandé)", description: "QA activé — qa-engineer invoqué pour vérifier la couverture (tickets à risque élevé : toujours activé)" },
      { label: "Non", description: "QA skippé sauf risque élevé — review directe après implémentation" }
    ]
  }]
})
```

**Note :** Le QA est **toujours activé automatiquement** pour les tickets à risque élevé (modification API, services, code critique), quelle que soit la réponse à cette question. Cette question ne concerne que les tickets à risque moyen et faible.

La valeur choisie est fixée pour toute la session et appliquée automatiquement à chaque CP-QA selon le niveau de risque détecté.

---

## Comportement selon le contexte d'invocation

- **Invoqué standalone** : demander le mode via le bloc question ci-dessus au CP-0.
- **Invoqué depuis l'orchestrateur feature** : le mode est transmis dans le texte du prompt — ne pas redemander le mode, démarrer directement avec la valeur reçue.

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
