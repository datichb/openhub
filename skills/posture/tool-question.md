---
name: tool-question
description: Utilisation de l'outil question d'OpenCode — quand et comment poser des questions structurées à l'utilisateur. Couvre le format sous-agent, le condensé enrichi orchestrator, et les bonnes pratiques.
---

# Skill — Outil `question` (OpenCode)

## Quand utiliser `question`

- **Choix entre plusieurs options** — mode de workflow, stratégie, format de sortie
- **Confirmation d'une action à risque** — suppression, remplacement, migration
- **Collecte d'une préférence** — langue, niveau de détail, priorité
- **Ambiguïté dans les instructions** — deux interprétations possibles
- **Décision qui bloque la suite** — checkpoints dans les workflows
- **Plusieurs décisions liées** — poser toutes les questions indépendantes en un seul appel

## Quand NE PAS utiliser `question`

- Informations déjà disponibles dans le contexte ou le codebase
- Clarifications mineures résolvables par une hypothèse raisonnable
- Quand la réponse n'influence pas le résultat final

---

## Bonnes pratiques

| Règle | |
|-------|--|
| Poser plusieurs questions liées et indépendantes en un seul appel | ✅ |
| Utiliser `multiple: true` quand l'utilisateur peut choisir plusieurs options | ✅ |
| Mettre l'option recommandée **en premier** avec `(Recommandé)` dans le label | ✅ |
| Prévoir la gestion des réponses libres (toujours possibles — ajoutées automatiquement) | ✅ |
| Ajouter une option "Autre" ou "Personnalisé" (redondant avec la saisie libre auto) | ❌ |
| Dépasser 5 options par question | ❌ |
| Poser une question déjà répondue dans la session | ❌ |

---

## Questions posées en tant que sous-agent

Quand tu es invoqué par un agent parent, **le champ `question` doit commencer par un bloc
de contexte** pour que l'utilisateur comprenne sans naviguer dans la session enfant.

### Format standard

```
[<Nom de l'agent> — <Phase ou étape> | <Feature ou ticket>]
<Question proprement dite>
```

### Format enrichi — obligatoire quand CONTEXTE = orchestrator_feature

Le texte affiché dans ta session enfant n'est **pas visible** par l'utilisateur dans la session parent.
La question est le **seul contenu qui remonte**. Inclure un condensé structuré (3-5 points) :

```
[<Nom de l'agent> — <Phase> | <Feature>]

**<Titre du condensé> :**
- <découverte clé 1>
- <découverte clé 2>
- <découverte clé 3>

<Question proprement dite>
```

**Règles du condensé :**
- 3 à 5 points maximum — actionnable, pas exhaustif
- Uniquement les informations qui influencent la décision demandée
- Termes exacts (noms de fichiers, patterns, warnings) — ne pas paraphraser
