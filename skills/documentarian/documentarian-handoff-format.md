---
name: documentarian-handoff-format
description: Source de vérité pour le format de retour du documentarian vers orchestrator-dev. Définit le bloc structuré à produire en fin de session de documentation quand invoqué depuis orchestrator-dev. Injecté dans le documentarian et dans orchestrator-dev pour garantir que producteur et consommateur partagent le même contrat.
---

# Skill — Format de handoff documentarian → orchestrator-dev

Ce skill est la **source de vérité** pour le format de retour du `documentarian` vers `orchestrator-dev`.
Il est injecté dans le `documentarian` et dans `orchestrator-dev` — producteur et consommateur partagent le même contrat.

---

## Quand produire ce bloc

Quand tu es invoqué depuis `orchestrator-dev` (via l'outil `Task` à l'étape 6 — mise à jour du CHANGELOG),
tu **dois** produire dans cet ordre :

1. **Le contenu de documentation produit complet** — le texte intégral de ce qui a été rédigé ou mis à jour (entrée CHANGELOG, ADR, guide, etc.). **Ce contenu doit être présenté dans son intégralité, même s'il est court.**
2. **Le bloc `## Retour vers orchestrator-dev`** défini ci-dessous — résumé structuré actionnable.

> **Autocontrôle obligatoire avant de produire ce bloc :**
> « Ai-je présenté le contenu de documentation complet avant ce bloc ? Si non, le présenter d'abord. »

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

- **Toujours présenter le contenu de documentation complet** avant ce bloc — même si le contenu est court. La présentation du contenu est obligatoire dans tous les cas.
- **Toujours produire ce bloc** à la suite du contenu, même si le statut est `bloqué`
- **La section `### Résumé de l'entrée`** doit être suffisamment précise pour qu'`orchestrator-dev` puisse l'inclure dans le compte rendu d'étape sans relire le fichier
- Si statut = `bloqué` : expliquer clairement la raison du blocage dans le résumé

> ❌ Ne jamais produire le bloc handoff sans avoir d'abord présenté le contenu de documentation complet.
> ❌ Ne jamais résumer le contenu dans le bloc — le bloc est un résumé structuré, pas un substitut.

---

## Règles pour le consommateur (orchestrator-dev)

### À la réception du retour du documentarian

1. **Vérifier la présence du contenu de documentation complet** (présenté avant le bloc) :
   - **Présent** → continuer la vérification suivante
   - **Absent** → demander explicitement au documentarian de présenter le contenu complet avant de continuer.

2. **Détecter la présence du bloc `## Retour vers orchestrator-dev`** :
   - **Présent** → lire le `### Statut` pour évaluer le résultat
   - **Absent** → demander explicitement au documentarian de produire le bloc avant de continuer.

3. **Intégrer le `### Résumé de l'entrée`** et les `### Fichiers modifiés` dans le compte rendu d'étape (étape 6).

4. **Utiliser le `### Statut`** pour informer le compte rendu :
   - `documenté` → noter comme "CHANGELOG mis à jour" dans le compte rendu
   - `partiellement-documenté` → noter les sections manquantes comme point d'attention
   - `bloqué` → noter le blocage dans les `### Points d'attention techniques` du compte rendu d'étape

> ❌ Ne jamais accepter un bloc handoff sans contenu de documentation préalable — les deux sont obligatoires.
> ❌ Ne jamais ignorer un statut `bloqué` — le signaler dans le compte rendu d'étape.
