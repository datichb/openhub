# Templates — Debugger : Rapport et ticket Beads (Phase 5)

## Rapport de diagnostic (ÉTAPE 5.1)

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

---

## Ticket de correction suggéré — affichage (ÉTAPE 5.2)

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

---

## Question de confirmation — CONTEXTE standalone (ÉTAPE 5.2)

```
question({
  questions: [{
    header: "Créer ticket Beads",
    question: "[Debugger — Phase 5 : Ticket | Bug : <titre>]\nCréer ce ticket de correction dans Beads ?",
    options: [
      { label: "Oui — créer le ticket", description: "Créer le ticket avec bd create (description, critères d'acceptance, notes techniques)" },
      { label: "Non", description: "Ne pas créer de ticket" }
    ]
  }]
})
```

---

## Retour intermédiaire + Question — CONTEXTE orchestrator_feature (ÉTAPE 5.2)

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
- `oui-creer-ticket` — Créer le ticket avec bd create (description, critères d'acceptance, notes techniques)
- `non` — Ne pas créer de ticket

**Instruction de reprise :** "Réponse Phase 5 debugger : [option]. Reprendre depuis Phase 5 — confirmation ticket Beads."
```

---

## Checkpoint de création du ticket — workflow debug (ÉTAPE 5.2)

Quand la carte du workflow liste `cp-ticket`, après l'affichage du ticket suggéré :

```
workflow_checkpoint({ id: "cp-ticket", summary: "Diagnostic : <symptôme, cause racine (probabilité), fichier:ligne>. Ticket à créer : <titre> — bug, P<X> ; critères : <critère 1>, <critère 2>. Validation = création du ticket." })
```

---

## Commande bd create (si oui)

Un seul appel : `bd create` reçoit la description, les critères d'acceptance et les notes (tes permissions refusent `bd update` et les variables shell) :

```bash
bd create "<titre>" -p <priorité> -t bug -l from-diagnostic \
  --description "<description, avec une section ## Étapes de reproduction (Steps to Reproduce)>" \
  --acceptance "<critères d'acceptance>" \
  --notes "<cause racine, fichiers impliqués, points d'attention>" \
  --json
```

Vérifier ensuite le ticket avec `bd lint <ID>` (l'identifiant est dans la réponse JSON) : il signale une section `Steps to Reproduce` ou des critères d'acceptation manquants (le titre doit contenir « Steps to Reproduce », exigé par bd).
