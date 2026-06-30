---
name: debugger-report-templates
description: Templates de rapport de diagnostic (Phase 5) — structure exacte du rapport, template de ticket Beads de correction, commandes bd create/update, labels, priorités suggérées. Chargé à la demande en Phase 5 après validation explicite.
bucket: B
---

# Skill — Debugger : Templates de rapport et ticket Beads (Phase 5)

## Contexte d'usage

Ce skill est chargé par `debugger-workflow` en Phase 5 après validation explicite
pour la production du rapport de diagnostic et du ticket Beads.

---

## Phase 5 — Production du livrable

**Uniquement après validation explicite.**

### ÉTAPE 5.1 — Produire le rapport de diagnostic

**Structure exacte :**

```markdown
## [Phase 5] Diagnostic — <titre court du bug>

### Symptôme
<Comportement observé vs attendu, conditions de déclenchement, fréquence>

### Périmètre analysé
<Artefacts fournis : stacktrace, logs, description, ticket Beads — et ce qui n'était PAS disponible>

### Localisation probable
`<chemin/vers/fichier.ts:ligne>` — <description courte>

### Cause racine

#### Hypothèse principale — <probabilité : haute / moyenne / faible>
<Explication en 2-5 phrases>

**Éléments qui l'étayent :**
- <extrait de stacktrace ou log avec référence>
- <observation dans le code>

**Pour confirmer :**
- <action concrète à effectuer>

#### Hypothèse secondaire (si applicable) — <probabilité>
<Même structure>

### Fichiers impliqués
| Fichier | Rôle dans le bug |
|---------|-----------------|
| `src/services/auth.service.ts:47` | Point d'origine probable |
| `src/middleware/auth.middleware.ts:12` | Point de propagation |

### ⚠️ Informations manquantes
<Informations qui n'ont PAS pu être obtenues (fichiers inaccessibles, logs d'infra externe, etc.)>
<Omettre cette section si toutes les informations nécessaires étaient disponibles>

### Ticket de correction suggéré
**Titre :** <titre court et actionnable>
**Type :** bug
**Priorité :** P<0-3>
**Description :** <description du bug et du contexte>
**Acceptance criteria :**
- <critère 1>
- <critère 2>
**Notes techniques :** <cause racine confirmée, fichiers à modifier, points d'attention>
```

### ÉTAPE 5.2 — Proposer la création du ticket Beads

Afficher le contexte en texte :

```markdown
## Ticket de correction suggéré

**Titre :** <titre>
**Type :** bug
**Priorité :** P<X>

**Description :**
<description complète>

**Critères d'acceptance :**
- <critère 1>
- <critère 2>

**Notes techniques :**
<cause racine, fichiers à modifier, points d'attention>
```

⚠️ **RAPPEL** : Le contexte du ticket suggéré (ci-dessus) **doit être affiché en texte** dans la discussion AVANT l'appel `question`. Si ce n'est pas fait → afficher le ticket suggéré MAINTENANT.

Puis appeler l'outil `question` :

**Si CONTEXTE = standalone :**

```
question({
  questions: [{
    header: "Créer ticket Beads",
    question: "[Debugger — Phase 5 : Ticket | Bug : <titre>]\nCréer ce ticket de correction dans Beads ?",
    options: [
      { label: "Oui — créer le ticket", description: "Créer le ticket avec bd create et enrichir description/acceptance/notes techniques" },
      { label: "Non", description: "Ne pas créer de ticket" }
    ]
  }]
})
```

**Si CONTEXTE = orchestrator_feature :**

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** debugger
**Phase :** 5 — Création ticket Beads (action irréversible)
**task_id :** <sessionID courant>

## Ticket de correction suggéré

**Titre :** <titre>
**Type :** bug
**Priorité :** P<X>

**Description :**
<description complète>

**Critères d'acceptance :**
- <critère 1>
- <critère 2>

**Notes techniques :**
<cause racine, fichiers à modifier, points d'attention>

---

## Question pour l'orchestrator

**Phase :** 5
**task_id :** <sessionID courant>

**Contexte :** Rapport de diagnostic produit. Demande de confirmation avant création du ticket Beads (action irréversible).

**Question :** Créer ce ticket de correction dans Beads ?

**Options :**
- `oui-creer-ticket` — Créer le ticket avec bd create et enrichir description/acceptance/notes techniques
- `non` — Ne pas créer de ticket

**Instruction de reprise :** "Réponse Phase 5 debugger : [option]. Reprendre depuis Phase 5 — confirmation ticket Beads."
```
→ **TERMINER LA SESSION**

**Si oui :**

```bash
TICKET=$(bd create "<titre>" -p <priorité> -t bug -l from-diagnostic --json)
ID=$(echo $TICKET | jq -r '.id')
bd update $ID --description "<description>"
bd update $ID --acceptance "<critères d'acceptance>"
bd update $ID --notes "<cause racine, fichiers impliqués, points d'attention>"
```

> Le label `from-diagnostic` signale que le ticket provient d'un rapport de diagnostic.

**Règles :**
- Toujours utiliser `--json` sur `bd create`
- Toujours capturer l'ID via `jq -r '.id'`
- Toujours ajouter `-l from-diagnostic` à la création
- La description est en langage naturel — jamais de code dans les champs Beads
- Afficher l'ID créé à l'utilisateur après création

### Priorités de ticket suggérées

| Critère | Priorité |
|---------|----------|
| Bug bloquant en production, perte de données | P0 |
| Bug affectant un chemin critique, nombreux utilisateurs impactés | P1 |
| Bug isolé, contournement possible | P2 |
| Comportement indésirable mineur, cosmétique | P3 |
