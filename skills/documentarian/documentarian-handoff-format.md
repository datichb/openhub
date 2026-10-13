---
name: documentarian-handoff-format
description: "Utiliser en fin de session de documentation invoquée via task : bloc « Retour vers <appelant> »."
annexes: [templates/documentarian-handoff-block.md]
---

# Skill — Format de handoff documentarian → agent appelant

Ce skill est la **source de vérité** pour le format de retour du `documentarian` vers l'agent qui l'a invoqué via `task` (son **appelant**).
Il est injecté dans le `documentarian` et chez ses appelants — producteur et consommateur partagent le même contrat.

---

## Principe fondamental — bloc unique

> **Contrat de handoff :** voir skill `shared/handoff-bloc-unique-rule` pour les règles universelles producer/consumer.
>
> **Rapport déposé :** si l'outil `workflow_report` est disponible, dépose ton rapport complet avec lui **avant** le bloc, puis remplace la section du rapport par `Rapport : rp-N` (identifiant reçu) ; sinon (outil absent, réponse « non enregistré », refus), inclus le rapport complet. Côté coordinateur, `Rapport : rp-N` vaut rapport présent (oh le joint à la carte du checkpoint) : cite l'identifiant au lieu de recopier le rapport.

---

## Format du bloc `## Retour vers <appelant>`

`<appelant>` est l'identifiant de l'agent qui t'a invoqué (`orchestrator-dev`, `orchestrator`, `auditor`, `debugger`…) : la section « Rôle dans cette session » de ton prompt, générée par oh, nomme tes appelants possibles. S'il y en a plusieurs et que le prompt d'invocation ne dit pas lequel t'a lancé, écris `## Retour vers appelant` : l'appelant reconnaît le bloc à son préfixe `## Retour vers`.

> **Template :** format défini dans `templates/documentarian-handoff-block.md` — charger via `read` quand tu produis ce bloc.

**Définitions du statut :**

| Statut | Condition |
|--------|-----------|
| `documenté` | Documentation produite et fichiers mis à jour sans blocage |
| `partiellement-documenté` | Documentation produite mais incomplète — certaines sections manquantes ou en attente de validation |
| `bloqué` | Impossible de produire la documentation — structure introuvable, format inconnu, confirmation en attente |

---

## Règles pour le producteur (documentarian)

- Le contenu de documentation est écrit dans les fichiers (via `write`/`edit`) — il n'est PAS reproduit dans la discussion
- **La section `### Résumé de l'entrée`** doit être suffisamment précise pour que l'appelant puisse l'inclure dans son récap sans relire le fichier
- Si statut = `bloqué` : expliquer clairement la raison du blocage dans le résumé
- **`### Questions bloquantes`** (obligatoire) — liste numérotée des questions nécessitant une réponse avant de poursuivre. Si aucune : écrire `Aucune.`. Le coordinateur **doit** résoudre ces questions avant de continuer le workflow.

---

## Règles pour le consommateur (agent appelant)

### À la réception du bloc de retour du documentarian

Le bloc se reconnaît à son titre, qui commence par `## Retour vers` (suivi de ton identifiant).

1. **Lire le `### Statut`** pour évaluer le résultat :
   - `documenté` → noter la documentation produite (type et fichiers) dans ton récap
   - `partiellement-documenté` → noter les sections manquantes comme point d'attention
   - `bloqué` → noter le blocage dans les points d'attention de ton récap

2. **Intégrer le `### Résumé de l'entrée`** et les `### Fichiers modifiés` dans ton récap.

3. **Si le bloc est absent** → demander explicitement au documentarian de le produire.

> ❌ Ne jamais ignorer un statut `bloqué` — le signaler dans ton récap.
