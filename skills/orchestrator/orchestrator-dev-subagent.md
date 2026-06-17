---
name: orchestrator-dev-subagent
description: Parcours d'exécution de l'orchestrateur dev en mode sous-agent (invoqué via task depuis l'orchestrateur feature) — CPs à enjeu fort produisent des blocs Question pour l'orchestrator + Retour vers orchestrator (partiel), session terminée après chaque CP à enjeu fort. Bloc Retour vers orchestrator (final) obligatoire en fin de session.
---

# Skill — Parcours Orchestrator-dev Sous-agent

> Ce skill est chargé quand l'orchestrator-dev est invoqué via `task` depuis l'orchestrateur feature. L'orchestrateur injecte `[SKILL:orchestrator/orchestrator-dev-subagent]` dans le prompt.

## Principe fondamental

Quand l'orchestrator-dev est invoqué via `task`, sa todo list est dans une session **isolée et non visible** par l'utilisateur. L'orchestrator feature est le seul responsable de la liste visible.

**Confirmer le contexte au démarrage :**
> `[orchestrator-dev] Contexte détecté : invoqué depuis l'orchestrateur feature. Mode de workflow reçu : <valeur canonique>. Mode interruption actif — CP-1, CP-QA (modes manuel/semi-auto), branche dédiée et CP-2 produisent des blocs ## Question pour l'orchestrator et terminent la session. Le bloc ## Retour vers orchestrator sera produit à chaque arrêt de session.`

---

## Règle absolue — CPs à enjeu fort

Les **CPs à enjeu fort** dans ce mode sont : CP-1 (mode manuel), branche dédiée, CP-QA (modes manuel et semi-auto), CP-2.

Pour chaque CP à enjeu fort : **ne pas poser la question via l'outil `question`**. À la place :
1. Produire le bloc `## Question pour l'orchestrator`
2. Produire le bloc `## Retour vers orchestrator` avec `**Type de récap :** partiel`
3. **TERMINER LA SESSION**

---

## CP-0 — Initialisation depuis l'orchestrateur feature

Le mode de workflow est transmis en paramètre — ne pas le redemander.

**Règle de parsing du mode :**
- Contient `manuel` → mode `manuel`
- Contient `semi-auto` → mode `semi-auto`
- Contient `auto` (mais pas `semi-auto`) → mode `auto`

Si aucune valeur détectée : mode `manuel` par défaut + signal :
> `⚠️ [orchestrator-dev] Mode de workflow non détecté dans le prompt — mode manuel appliqué par défaut.`

Afficher le récapitulatif des tickets reçus et démarrer directement sans redemander le mode.

---

## Format des blocs structurés (CPs à enjeu fort)

### Bloc CP-1 (mode manuel)

```markdown
## Question pour l'orchestrator

**Agent :** orchestrator-dev
**Ticket :** #<ID> — <titre>
**Phase :** CP-1

### Contexte
Prêt à démarrer l'implémentation du ticket #<ID> — <titre>.
<Description courte du ticket issue de bd show>

### Question en attente
Démarrer l'implémentation du ticket #<ID> — <titre> ?

### Options disponibles
- `demarrer` — Déléguer l'implémentation à <developer-xxx>
- `voir-detail` — Afficher le contenu complet du ticket (bd show <ID>)
- `passer` — Ignorer ce ticket et passer au suivant
- `stop` — Arrêter le workflow

### État de la session
**Tickets traités :** [bd-XX ✅, ...]
**En cours :** bd-<ID>
**Tickets restants :** [bd-YY, bd-ZZ, ...]
**task_id :** <task_id de la session en cours>
```
Suivi du bloc `## Retour vers orchestrator` avec `**Type de récap :** partiel`.

→ **TERMINER LA SESSION**

**Instruction de reprise :** "Réponse CP-1 ticket #<ID> : [option choisie]. Reprendre depuis CP-1."

---

### Bloc Branche dédiée

```markdown
## Question pour l'orchestrator

**Agent :** orchestrator-dev
**Ticket :** #<ID> — <titre>
**Phase :** Branche dédiée

### Contexte
Avant de démarrer l'implémentation du ticket #<ID>, une branche dédiée est recommandée.
**Nom de branche calculé :** `<type>/<ticket-id>-<description-courte>`

### Question en attente
Créer une branche dédiée pour le ticket #<ID> ?

### Options disponibles
- `oui-branche` — Créer et basculer sur `<type>/<ticket-id>-<description-courte>` avant de démarrer
- `non-branche` — Rester sur la branche courante

### État de la session
**Tickets traités :** [bd-XX ✅, ...]
**En cours :** bd-<ID>
**Tickets restants :** [bd-YY, bd-ZZ, ...]
**task_id :** <task_id de la session en cours>
```
Suivi du bloc `## Retour vers orchestrator` avec `**Type de récap :** partiel`.

→ **TERMINER LA SESSION**

**Instruction de reprise :** "Réponse branche ticket #<ID> : [option choisie]. Reprendre depuis délégation au developer."

---

### Bloc CP-QA (modes manuel et semi-auto — risque moyen)

```markdown
## Question pour l'orchestrator

**Agent :** orchestrator-dev
**Ticket :** #<ID> — <titre>
**Phase :** CP-QA (risque moyen)

### Contexte
L'implémentation du ticket #<ID> est terminée. Risque moyen détecté : logique métier ou utilitaires modifiés.

### Question en attente
Passer par le QA avant la review ?

### Options disponibles
- `oui-qa` — Invoquer qa-engineer pour vérifier la couverture (recommandé)
- `non-qa` — Passer directement à la review

### État de la session
**Tickets traités :** [bd-XX ✅, ...]
**En cours :** bd-<ID>
**Tickets restants :** [bd-YY, bd-ZZ, ...]
**task_id :** <task_id de la session en cours>
```
Suivi du bloc `## Retour vers orchestrator` avec `**Type de récap :** partiel`.

→ **TERMINER LA SESSION**

**Instruction de reprise :** "Réponse CP-QA ticket #<ID> : [option choisie]. Reprendre depuis QA / review."

---

### Bloc CP-QA (modes manuel et semi-auto — risque faible)

```markdown
## Question pour l'orchestrator

**Agent :** orchestrator-dev
**Ticket :** #<ID> — <titre>
**Phase :** CP-QA (risque faible)

### Contexte
L'implémentation du ticket #<ID> est terminée. Risque faible détecté : UI/doc/config uniquement.

### Question en attente
Passer par le QA avant la review ?

### Options disponibles
- `non-qa` — Passer directement à la review (recommandé)
- `oui-qa` — Invoquer qa-engineer avec le diff et l'ID du ticket

### État de la session
**Tickets traités :** [bd-XX ✅, ...]
**En cours :** bd-<ID>
**Tickets restants :** [bd-YY, bd-ZZ, ...]
**task_id :** <task_id de la session en cours>
```
Suivi du bloc `## Retour vers orchestrator` avec `**Type de récap :** partiel`.

→ **TERMINER LA SESSION**

---

### Bloc CP-2 (tous modes)

CP-2 est **toujours une pause, dans tous les modes**.

```markdown
## Question pour l'orchestrator

**Agent :** orchestrator-dev
**Ticket :** #<ID> — <titre>
**Phase :** CP-2

### Contexte complet
**Synthèse :**
| Sévérité | Nombre | Résumé |
|----------|--------|--------|
<tableau issu du ### Synthèse des problèmes du retour reviewer>

**Verdict reviewer :** <commit | corriger | corriger-sécurité>
**Routing recommandé :** <retour-initial | developer-security>

### Rapport de review complet
<rapport de review intégral copié tel quel — toutes sections, aucune omission, aucune reformulation>

### Question en attente
Quelle suite pour le ticket #<ID> — <titre> ?

### Options disponibles
Les labels sont dynamiques selon le verdict :
- `commit` → `Commit (Recommandé — aucun problème bloquant)` ou `Commit`
- `corriger` → `Corriger (Recommandé — X problèmes à résoudre)` ou `Corriger`
- `corriger-sécurité` → `Corriger (Recommandé — problème de sécurité)` ou `Corriger`

### État de la session
**Tickets traités :** [bd-XX ✅, ...]
**En cours :** bd-<ID>
**Tickets restants :** [bd-YY, bd-ZZ, ...]
**task_id :** <task_id de la session en cours>
```

> ⚠️ Ajouter le bloc `## Retour vers orchestrator` avec `**Type de récap :** partiel` **immédiatement après** le bloc `## Question pour l'orchestrator`.

→ **TERMINER LA SESSION**

**Instruction de reprise :** "Réponse CP-2 ticket #<ID> : [option choisie]. Reprendre depuis commit / correction."

---

## Règle de reprise (ré-invoqué avec task_id)

Quand le prompt de reprise contient `"Réponse de l'utilisateur au CP <phase>"` ou `"Réponse CP-X ticket #<ID>"` :
- **Ne pas reposer la question** — reprendre directement à l'étape suivante selon la réponse reçue
- Appliquer la réponse comme si elle avait été donnée via l'outil `question` en mode standalone
- Continuer le workflow normalement jusqu'au prochain CP à enjeu fort ou jusqu'à la fin

---

## Récap global final

En fin de session (tous les tickets traités ou arrêt demandé), produire le récap global ET le bloc `## Retour vers orchestrator` avec `**Type de récap :** final`.

> ⚠️ **Règle absolue :** produire **TOUJOURS** le bloc `## Retour vers orchestrator` à la fin du récap global — sans exception, même en cas de stop, de ticket bloqué ou de session incomplète.

---

## Comportement des CPs selon le mode (référence rapide)

| CP | `manuel` | `semi-auto` | `auto` |
|----|----------|-------------|--------|
| CP-1 | ⏸️ bloc Question | ▶️ auto | ▶️ auto |
| Branche | ⏸️ bloc Question | ⏸️ bloc Question | ⏸️ bloc Question |
| CP-QA | ⏸️ bloc Question | ⏸️ bloc Question | ▶️ valeur CP-0 |
| CP-2 | ⏸️ bloc Question | ⏸️ **bloc Question** | ⏸️ **bloc Question** |
| CP-3 | ▶️ auto (enchaîne) | ▶️ auto | ▶️ auto |

> CP-2 est toujours une pause dans tous les modes — sans exception.
