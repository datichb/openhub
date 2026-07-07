---
name: documentarian-handoff-format
description: Source de vérité pour le format de retour du documentarian vers orchestrator-dev. Définit le bloc structuré unique à produire en fin de session de documentation quand invoqué depuis orchestrator-dev. Injecté dans le documentarian et dans orchestrator-dev pour garantir que producteur et consommateur partagent le même contrat.
---

# Skill — Format de handoff documentarian → orchestrator-dev

Ce skill est la **source de vérité** pour le format de retour du `documentarian` vers `orchestrator-dev`.
Il est injecté dans le `documentarian` et dans `orchestrator-dev` — producteur et consommateur partagent le même contrat.

---

## Principe fondamental — bloc unique

Quand tu es invoqué depuis `orchestrator-dev` (via l'outil `Task` à l'étape 6 — mise à jour du CHANGELOG),
ton **seul output** est le bloc `## Retour vers orchestrator-dev` défini ci-dessous.

**Règle absolue :** aucun texte avant, après ou en dehors de ce bloc. Le contenu de documentation est déjà écrit dans les fichiers via l'outil `write` — il n'a pas besoin d'être reproduit dans la discussion. Le bloc est autosuffisant.

> **Autocontrôle obligatoire avant de terminer la session :**
> « Mon output contient-il du texte en dehors du bloc `## Retour vers orchestrator-dev` ? Si oui, le supprimer. Le contenu est dans les fichiers, pas dans la discussion. »

---

## Format du bloc `## Retour vers orchestrator-dev`

```
---

## Retour vers orchestrator-dev

**Agent :** documentarian
**Ticket :** #<ID> — <titre> (si applicable, sinon "— demande directe")

### Documentation produite

**Type :** `changelog` | `adr` | `api` | `readme` | `guide` | `slides` | `autre`
**Fichiers modifiés :**
- `<chemin/vers/fichier.md>` — <type d'action : créé | mis à jour | section ajoutée>
- `<chemin/vers/fichier.md>` — <type d'action>
<"Aucun fichier modifié" si rien n'a pu être produit>

### Résumé de l'entrée
<1-3 phrases décrivant ce qui a été documenté — ex : "Ajout de l'entrée CHANGELOG pour la v1.3.0 couvrant les tickets bd-42 et bd-43 (feat: authentification JWT, fix: null guard UserService)">

### Contexte et décisions
- <décision 1 — ex : format Keep a Changelog conservé car déjà en place>
- <décision 2 — ex : entrée regroupée sous une seule version car tickets liés>
<"Aucune décision notable" si documentation standard>

### Statut
`documenté` | `partiellement-documenté` | `bloqué`
```

**Définitions du statut :**

| Statut | Condition |
|--------|-----------|
| `documenté` | Documentation produite et fichiers mis à jour sans blocage |
| `partiellement-documenté` | Documentation produite mais incomplète — certaines sections manquantes ou en attente de validation |
| `bloqué` | Impossible de produire la documentation — structure introuvable, format inconnu, confirmation en attente |

---

## Règles pour le producteur (documentarian)

- **Produire UNIQUEMENT le bloc `## Retour vers orchestrator-dev`** — aucun texte avant ou après
- Le contenu de documentation est écrit dans les fichiers (via `write`/`edit`) — il n'est PAS reproduit dans la discussion
- **La section `### Résumé de l'entrée`** doit être suffisamment précise pour qu'`orchestrator-dev` puisse l'inclure dans le récap global sans relire le fichier
- Si statut = `bloqué` : expliquer clairement la raison du blocage dans le résumé

> ❌ Ne jamais écrire de texte en dehors du bloc de handoff
> ❌ Ne jamais reproduire le contenu de documentation dans la discussion — il est dans les fichiers
> ❌ Ne jamais produire de résumé narratif ou d'introduction avant le bloc

---

## Règles pour le consommateur (orchestrator-dev)

### À la réception du bloc `## Retour vers orchestrator-dev` du documentarian

1. **Lire le `### Statut`** pour évaluer le résultat :
   - `documenté` → noter comme "CHANGELOG mis à jour" dans le récap global
   - `partiellement-documenté` → noter les sections manquantes comme point d'attention
   - `bloqué` → noter le blocage dans les `### Points d'attention globaux` du récap

2. **Intégrer le `### Résumé de l'entrée`** et les `### Fichiers modifiés` dans le récap global structuré.

3. **Si le bloc est absent** → demander explicitement au documentarian de le produire.

> ❌ Ne jamais ignorer un statut `bloqué` — le signaler dans le récap global.
