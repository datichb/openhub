---
name: developer-handoff-format
description: Source de vérité pour le format de retour des agents developer-* vers orchestrator-dev. Définit le bloc structuré unique à produire en fin d'implémentation quand invoqué depuis orchestrator-dev. Injecté dans tous les developer-* et dans orchestrator-dev pour garantir que producteur et consommateur partagent le même contrat.
---

# Skill — Format de handoff developer-* → orchestrator-dev

Ce skill est la **source de vérité** pour le format de retour des agents `developer-*` vers `orchestrator-dev`.
Il est injecté dans chaque `developer-*` et dans `orchestrator-dev` — producteur et consommateur partagent le même contrat.

---

## Principe fondamental — bloc unique

Quand tu es invoqué depuis `orchestrator-dev` (via l'outil `Task`), ton **seul output** est le bloc `## Retour vers orchestrator-dev` défini ci-dessous.

**Règle absolue :** aucun texte avant, après ou en dehors de ce bloc. Pas de compte rendu narratif, pas d'introduction, pas de résumé, pas de conclusion. Le bloc est autosuffisant.

> **Autocontrôle obligatoire avant de terminer la session :**
> « Mon output contient-il du texte en dehors du bloc `## Retour vers orchestrator-dev` ? Si oui, le supprimer et encoder l'information dans les champs du bloc. »

---

## Format du bloc `## Retour vers orchestrator-dev`

```
---

## Retour vers orchestrator-dev

**Agent :** developer-<type>
**Ticket :** #<ID> — <titre>
**Branche :** <nom de la branche sur laquelle le travail a été effectué>

### Contexte et décisions

- <décision 1 — choix technique + raison concise (2-3 phrases max)>
- <décision 2 — compromis fait + justification>
- <décision 3 — alternative écartée et pourquoi>
<Minimum 2 entrées. Chaque entrée doit capturer le "pourquoi" d'un choix significatif.>
<"Aucune décision notable — implémentation standard suivant les patterns existants" si vraiment trivial>

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

### Données techniques brutes
<Stacktraces, extraits de code significatifs, résultats de commandes constituant une preuve — uniquement si pertinent pour la review ou le diagnostic>
<"Aucune" si non applicable>

### Migration destructive (si applicable)
```
⚠️ MIGRATION DESTRUCTIVE DÉTECTÉE
Type : [DROP COLUMN / TRUNCATE / DELETE masse / ...]
Table(s) impactée(s) : [liste]
Données perdues si exécutée : [estimation / "irréversible"]
Réversibilité : [commande de rollback / non-réversible]
Dry-run output : [résultat de la commande de preview]
```
<Omettre cette section si aucune migration destructive n'est présente>

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

**Agent :** developer (domaine backend)
**Ticket :** #bd-42 — Fix missing null guard in UserService.findById
**Branche :** fix/bd-42-null-guard-user-service

### Contexte et décisions

- Choix de créer une méthode `findById` dédiée plutôt que de patcher `findByEmail_legacy` — l'ancienne méthode n'avait pas de typage strict et propageait le null implicitement. Refactoring plus propre que patch.
- Suppression de `findByEmail_legacy()` — grep confirme aucun autre appelant dans la codebase. La méthode était non typée et source du bug (retour implicitement nullable sans type).
- Typage explicite `Promise<User | null>` sur `findByEmail` du repository — rend le contrat visible pour tout appelant futur.

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

### Données techniques brutes
Aucune

### Blocages rencontrés
Aucun blocage rencontré.

### Statut
`implémenté`
```

---

## Règles pour le producteur (developer-*)

- **Produire UNIQUEMENT le bloc `## Retour vers orchestrator-dev`** — aucun texte avant ou après
- **`### Contexte et décisions`** est le champ qui capture le "pourquoi" — minimum 2 entrées pour toute implémentation non triviale, chaque entrée = choix + raison
- **`**Diff résumé**`** : exécuter `git diff --stat HEAD~1` (ou `git diff --stat <branche-base>...HEAD` si plusieurs commits) et coller la sortie sur une seule ligne condensée
- **`**Changements par fichier**`** : pour chaque fichier du diff, lister les symboles changés avec la notation `+/-/~` — ne pas inventer, ne pas résumer arbitrairement
- **`### Critères d'acceptance couverts`** doit être basé sur `bd show <ID>` — cocher chaque critère explicitement
- **`### Points d'attention pour la review`** est critique : c'est ce qui permet au reviewer de concentrer son attention sur les zones sensibles
- **`### Données techniques brutes`** : stacktraces, diffs annotés, extraits de code — uniquement si nécessaire à la review ou au diagnostic. Sinon "Aucune"
- **Toujours passer le ticket en `review`** avant de produire ce bloc (sauf si statut = `bloqué`)
- Si statut = `bloqué` : exécuter `bd update <ID> -s blocked` + `bd comments add <ID> "Bloqué par : <raison>"` avant de produire le bloc

> ❌ Ne jamais écrire de texte en dehors du bloc de handoff
> ❌ Ne jamais produire de compte rendu narratif, résumé ou introduction avant le bloc
> ❌ Ne jamais dupliquer dans du texte libre ce qui est déjà dans les champs structurés du bloc

---

## Règles pour le consommateur (orchestrator-dev)

### À la réception du bloc `## Retour vers orchestrator-dev` d'un developer

1. **Lire le `### Statut`** pour décider de la suite :
   - `implémenté` ou `partiellement-implémenté` → continuer vers l'étape 3 (QA optionnel) ou l'étape 4 (review)
   - `bloqué` → traiter comme un "Ticket bloqué en cours d'implémentation" (voir section dédiée du protocole)

2. **Transmettre les `### Points d'attention pour la review`** au reviewer à l'étape 4 :
   > Fournir au reviewer : diff + ticket ID + **"Points d'attention signalés par le developer : <liste>"**
   Ces points orientent la review sur les zones sensibles et évitent les faux positifs.

3. **Transmettre le nom de la branche** au reviewer à l'étape 4 — le reviewer récupère lui-même le diff complet via ses propres outils (`git diff`). Les `**Changements par fichier**` sont conservés pour le compte rendu d'étape (étape 6) uniquement.

4. **Intégrer les données structurées du bloc** (`### Contexte et décisions`, `**Diff résumé**`, `**Changements par fichier**`, `### Critères d'acceptance couverts`, `### Points d'attention`) dans le récap global structuré (section "Récap global — Fin de session").

5. **Si le bloc est absent** → demander explicitement au developer de le produire avant de continuer.

> ❌ Ne jamais passer à la review sans avoir reçu le `### Statut` — une implémentation `bloqué` ne doit pas être soumise au reviewer.
> ❌ Ne jamais ignorer les `### Points d'attention` — les transmettre intégralement au reviewer.
> ❌ Ne jamais transmettre les `**Changements par fichier**` au reviewer à la place d'un vrai diff — toujours passer le nom de branche pour que le reviewer récupère lui-même le diff complet.
