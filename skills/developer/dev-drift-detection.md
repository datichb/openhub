---
name: dev-drift-detection
description: Détecte et gère la dérive architecturale en cours d'implémentation — signaux de déclenchement, 3 options de décision (réviser scope / revert / bifurquer), template de rapport de dérive vers l'orchestrateur. Invocable quand un developer-subagent signale un blocage architectural ou une contradiction spec/réalité.
---

# Skill — Détection de dérive architecturale

## Rôle

Ce skill est activé quand l'implémentation d'un ticket révèle une contradiction entre
la spec planifiée et la réalité du codebase — rendant l'approche initiale non viable ou risquée.

Il guide le developer (ou l'orchestrator-dev) pour diagnostiquer la dérive et proposer
une décision à l'utilisateur avant de continuer.

---

## Signaux de déclenchement

Activer ce skill quand **au moins un** de ces signaux est présent :

- La spec suppose l'existence d'une abstraction, interface ou couche qui n'existe pas dans le codebase réel
- L'implémentation nécessiterait de modifier > 3 fichiers non prévus dans le ticket
- Un contrat d'interface (type, DTO, signature) est incompatible avec la spec et le changer casserait des consommateurs existants
- La feature suppose un pattern architectural (use case, aggregate, port/adapter) qui n'est pas présent dans le projet
- Une dépendance entre tickets rend impossible l'implémentation sans que le ticket précédent soit modifié
- L'estimation initiale est dépassée de plus de 2x sans que la spec soit revisitée

---

## Process de décision

### Étape 1 — Diagnostiquer la dérive

Documenter la dérive avec précision :

```
## Dérive détectée

**Ticket :** #<ID> — <titre>

**Spec prévoyait :**
<ce que la spec dit explicitement ou implicitement>

**Réalité observée :**
<ce qui existe réellement dans le codebase — path:line>

**Écart :**
<ce qui ne correspond pas, et pourquoi l'approche initiale est non viable>

**Impact estimé si on continue sans corriger :**
<risques techniques, dette accumulée, effet sur les autres tickets>
```

### Étape 2 — Formuler les 3 options

Présenter systématiquement les 3 options à l'utilisateur (via l'outil `question` ou le bloc handoff selon le contexte) :

#### Option A — Réviser la tâche Beads (changer le scope)

Modifier le ticket pour refléter la réalité du codebase.
- Adapter les critères d'acceptance
- Réduire ou ajuster le périmètre
- Mettre à jour les notes techniques

**Quand privilégier :** la dérive est mineure, la valeur métier est préservée avec un scope réduit.

#### Option B — Revert + nouvelle approche

Annuler les changements en cours et repartir d'une approche différente.
- `git checkout -- .` ou revert des fichiers modifiés
- Définir une nouvelle approche avant de redeliver au developer

**Quand privilégier :** l'approche actuelle est fondamentalement incorrecte, le revert est propre.

#### Option C — Bifurquer : ticket de refactoring pré-requis

Créer un ticket de refactoring qui prépare le terrain, puis implémenter le ticket original après.
- Créer un nouveau ticket Beads (type `refactoring`, priorité P1, dépendance sur le ticket original)
- Mettre le ticket original en `blocked` le temps que le refactoring soit fait

**Quand privilégier :** l'écart révèle une dette technique réelle qui impactera d'autres features futures.

---

## Format du rapport de dérive

Quand le developer-subagent retourne un status `BLOCKED_ARCHITECTURE`, produire ce rapport :

```markdown
## Rapport de dérive — #<ID>

**Ticket :** #<ID> — <titre>
**Status :** BLOCKED_ARCHITECTURE

### Dérive identifiée
<description précise — spec vs réalité>

### Fichiers concernés
| Fichier | Rôle | Écart |
|---------|------|-------|
| `<path:line>` | <rôle dans la spec> | <ce qui manque ou diffère> |

### Options proposées

**A — Réviser le scope**
> <ce qui changerait dans le ticket>
> Effort estimé : <X>h supplémentaires

**B — Revert + nouvelle approche**
> <approche alternative envisagée>
> Effort estimé : <X>h

**C — Bifurquer (ticket refactoring pré-requis)**
> Titre suggéré : `refactoring: <description courte>`
> Périmètre du refactoring : <fichiers/patterns à corriger>
> Effort estimé refactoring : <X>h | Effort ticket original après : <Y>h

### Recommandation
Option <A/B/C> — <justification en 1-2 phrases>
```

---

## Règles

✅ Toujours présenter les 3 options — ne jamais choisir unilatéralement
✅ Toujours documenter la dérive avec `path:line` (grade Confirmed)
✅ Toujours estimer l'effort pour chaque option
❌ Ne jamais continuer l'implémentation sans résoudre la dérive
❌ Ne jamais supprimer silencieusement des critères d'acceptance pour contourner la dérive
