---
name: developer-handoff-format
description: Source de vérité pour le format de retour des agents developer-* vers orchestrator-dev. Définit le bloc structuré à produire en fin d'implémentation quand invoqué depuis orchestrator-dev. Injecté dans tous les developer-* et dans orchestrator-dev pour garantir que producteur et consommateur partagent le même contrat.
---

# Skill — Format de handoff developer-* → orchestrator-dev

Ce skill est la **source de vérité** pour le format de retour des agents `developer-*` vers `orchestrator-dev`.
Il est injecté dans chaque `developer-*` et dans `orchestrator-dev` — producteur et consommateur partagent le même contrat.

---

## Quand produire ce bloc

Quand tu es invoqué depuis `orchestrator-dev` (via l'outil `Task`),
tu **dois** produire dans cet ordre :

1. **Le compte rendu d'implémentation complet** — description narrative de ce qui a été fait, décisions prises, fichiers touchés, tests écrits, critères couverts. **Ce compte rendu doit être produit même si l'implémentation est partielle ou bloquée.**
2. **Le bloc `## Retour vers orchestrator-dev`** défini ci-dessous — résumé structuré actionnable.

Ce bloc vient **après** le compte rendu d'implémentation — il en est le résumé structuré. Il ne le remplace pas.

> **Autocontrôle obligatoire avant de produire ce bloc :**
> « Ai-je produit le compte rendu d'implémentation complet avant ce bloc ? Si non, le produire d'abord. »

---

## Format du bloc `## Retour vers orchestrator-dev`

```
---

## Retour vers orchestrator-dev

**Agent :** developer-<type>
**Ticket :** #<ID> — <titre>
**Branche :** <nom de la branche sur laquelle le travail a été effectué>

### Implémentation

**Diff résumé :** <N fichiers modifiés, X insertions, Y suppressions>
<Sortie de `git diff --stat HEAD~1` ou `git diff --stat <base-branch>...HEAD`>

**Changements par fichier :**

`<chemin/vers/fichier.ts>` (+X / -Y)
  + <NomFonction/NomMéthode/NomClasse>
    — <annotation courte si la modification n'est pas triviale>
  ~ <NomFonction/NomMéthode>
    — <ce qui a changé dans cette fonction>
  - <NomFonction supprimée>
    — <raison de la suppression>

`<chemin/vers/autrefichier.ts>` (+X / -Y)
  ~ <NomFonction>
    — <ce qui a changé>

`<chemin/vers/fichier.test.ts>` (+X / -0)
  + "<description du cas de test ajouté>"
  + "<description du cas de test ajouté>"

<Répéter pour chaque fichier modifié>

**Tests écrits :** <oui — N tests (X unitaires, Y intégration) | non — raison>
**Statut Beads :** `review`

### Critères d'acceptance couverts
- [x] <critère 1 du ticket — vérifié>
- [x] <critère 2 — vérifié>
- [ ] <critère 3 — **non couvert** — raison (hors scope, blocage technique, etc.)>
<"Tous les critères d'acceptance sont couverts" si applicable>

### Points d'attention pour la review
- <point 1 — décision technique notable, compromis, dette introduite volontairement>
- <point 2 — zone fragile, dépendance externe, comportement edge-case à vérifier>
<"Aucun point d'attention particulier" si l'implémentation est standard>

### Blocages rencontrés
- <blocage 1 — résolu ou non, et comment>
<"Aucun blocage rencontré" si l'implémentation s'est déroulée normalement>

### Statut
`implémenté` | `partiellement-implémenté` | `bloqué`
```

**Définitions du statut :**

| Statut | Condition |
|--------|-----------|
| `implémenté` | Tous les critères d'acceptance couverts, ticket passé en `review` |
| `partiellement-implémenté` | Implémentation réalisée mais certains critères non couverts — ticket passé en `review` avec la liste des gaps |
| `bloqué` | Implémentation impossible sans déblocage externe — ticket passé en `blocked` |

**Notation des changements par fichier :**

| Symbole | Signification |
|---------|--------------|
| `+` | Symbole ajouté (fonction, méthode, classe, interface, type) |
| `~` | Symbole modifié (signature ou comportement changés) |
| `-` | Symbole supprimé |

**Règles de granularité :**

- **Fichiers de code** (`.ts`, `.js`, `.py`, `.go`, etc.) → lister les symboles nommés (fonctions, méthodes, classes) avec `+/-/~`
- **Fichiers de test** → lister les descriptions des cas de test ajoutés/modifiés entre guillemets
- **Fichiers de config / migration / fixtures / schéma** (sans symboles nommés) → juste le stat `(+X / -Y lignes)` avec une description en prose sur une seule ligne
- **Moins de 3 symboles modifiés dans un fichier** → tous les lister
- **Plus de 5 symboles modifiés dans un fichier** → lister les plus significatifs + `... (+N autres changements mineurs)`

---

## Exemple complet

```
---

## Retour vers orchestrator-dev

**Agent :** developer-backend
**Ticket :** #bd-42 — Fix missing null guard in UserService.findById
**Branche :** fix/bd-42-null-guard-user-service

### Implémentation

**Diff résumé :** 3 fichiers modifiés, 85 insertions, 5 suppressions

**Changements par fichier :**

`src/services/user.service.ts` (+42 / -3)
  + findById(id: string): Promise<User | null>
    — nouvelle méthode principale avec guard null explicite avant accès `.email`
  ~ login(email: string, password: string): Promise<AuthToken>
    — utilise désormais findById au lieu d'un accès direct au repository
  - findByEmail_legacy()
    — supprimé, remplacé par findById (était non typé et sans guard)

`src/repositories/user.repository.ts` (+8 / -2)
  ~ findByEmail(email: string): Promise<User | null>
    — retour explicitement nullable (était `User` implicitement, source du bug)

`tests/unit/user.service.test.ts` (+35 / -0)
  + "devrait retourner null quand l'utilisateur n'existe pas"
  + "devrait retourner l'utilisateur quand l'ID existe"
  + "devrait lever NotFoundException si findById retourne null dans login()"

**Tests écrits :** oui — 3 tests unitaires
**Statut Beads :** `review`

### Critères d'acceptance couverts
- [x] La méthode findById retourne null au lieu de lever une TypeError quand l'utilisateur est inconnu
- [x] login() retourne un 401 explicite sur email inexistant
- [x] Tests unitaires couvrant les cas null et valide

### Points d'attention pour la review
- `findByEmail_legacy()` supprimé — vérifier qu'aucun autre appel n'existait ailleurs dans la codebase (grep effectué, rien trouvé, mais à re-vérifier)
- Le typage de retour `Promise<User | null>` introduit sur `findByEmail` peut affecter d'autres appelants si le null n'est pas géré côté appelant

### Blocages rencontrés
Aucun blocage rencontré.

### Statut
`implémenté`
```

---

## Règles pour le producteur (developer-*)

- **Toujours produire le compte rendu d'implémentation complet** avant ce bloc — même si l'implémentation est partielle ou bloquée. Le compte rendu est obligatoire dans tous les cas.
- **Toujours produire ce bloc** à la suite du compte rendu, quelle que soit la complexité de l'implémentation
- **`**Diff résumé**`** : exécuter `git diff --stat HEAD~1` (ou `git diff --stat <branche-base>...HEAD` si plusieurs commits) et coller la sortie sur une seule ligne condensée
- **`**Changements par fichier**`** : pour chaque fichier du diff, lister les symboles changés avec la notation `+/-/~` — ne pas inventer, ne pas résumer arbitrairement
- **`### Critères d'acceptance couverts`** doit être basé sur `bd show <ID>` — cocher chaque critère explicitement
- **`### Points d'attention pour la review`** est critique : c'est ce qui permet au reviewer de concentrer son attention sur les zones sensibles
- **Toujours passer le ticket en `review`** avant de produire ce bloc (sauf si statut = `bloqué`)
- Si statut = `bloqué` : exécuter `bd update <ID> -s blocked` + `bd comments add <ID> "Bloqué par : <raison>"` avant de produire le bloc

> ❌ Ne jamais produire le bloc handoff sans avoir d'abord produit le compte rendu d'implémentation complet.
> ❌ Ne jamais résumer le compte rendu — le bloc est un résumé structuré, pas un substitut.

---

## Règles pour le consommateur (orchestrator-dev)

### À la réception du bloc `## Retour vers orchestrator-dev` d'un developer

1. **Lire le `### Statut`** pour décider de la suite :
   - `implémenté` ou `partiellement-implémenté` → continuer vers l'étape 3 (QA optionnel) ou l'étape 4 (review)
   - `bloqué` → traiter comme un "Ticket bloqué en cours d'implémentation" (voir section dédiée du protocole)

2. **Transmettre les `### Points d'attention pour la review`** au reviewer à l'étape 4 :
   > Fournir au reviewer : diff + ticket ID + **"Points d'attention signalés par le developer : <liste>"**
   Ces points orientent la review sur les zones sensibles et évitent les faux positifs.

3. **Transmettre les `**Changements par fichier**`** au reviewer à l'étape 4 :
   > Inclure le bloc `**Changements par fichier**` intégral dans le prompt du reviewer — il lui donne une vue structurée du delta sans qu'il ait à parser le diff brut lui-même.

4. **Transmettre les `### Critères d'acceptance couverts`** au qa-engineer si QA activé :
   > Fournir au qa-engineer : diff + ticket ID + **"Critères déjà couverts : <liste des [x]> / Non couverts : <liste des [ ]>"**

5. **Intégrer le `**Diff résumé**` et les `**Changements par fichier**`** dans le compte rendu d'étape (étape 6).

6. **Stocker le compte rendu d'implémentation complet** (texte narratif produit par le developer avant le bloc handoff) pour inclusion dans le récap global (section "Récap global — Fin de session"). Ce compte rendu est transmis intégralement à l'orchestrator feature — il ne doit jamais être réduit aux seules métadonnées du bloc handoff.

7. **Si le bloc est absent** → demander explicitement au developer de le produire avant de continuer.

8. **Si le compte rendu d'implémentation est absent** (le bloc handoff est présent sans compte rendu préalable) → demander explicitement au developer de produire le compte rendu complet avant de continuer.

> ❌ Ne jamais passer à la review sans avoir reçu le `### Statut` — une implémentation `bloqué` ne doit pas être soumise au reviewer.
> ❌ Ne jamais ignorer les `### Points d'attention` — les transmettre intégralement au reviewer.
> ❌ Ne jamais résumer les `**Changements par fichier**` — les transmettre tels quels au reviewer.
> ❌ Ne jamais accepter un bloc handoff sans compte rendu d'implémentation préalable — les deux sont obligatoires.
