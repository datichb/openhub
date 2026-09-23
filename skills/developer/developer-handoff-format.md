---
name: developer-handoff-format
description: Source de vérité pour le format de retour des agents developer-* vers orchestrator-dev. Définit le bloc structuré unique à produire en fin d'implémentation quand invoqué depuis orchestrator-dev. Injecté dans tous les developer-* et dans orchestrator-dev pour garantir que producteur et consommateur partagent le même contrat.
---

# Skill — Format de handoff developer-* → orchestrator-dev

Ce skill est la **source de vérité** pour le format de retour des agents `developer-*` vers `orchestrator-dev`.
Il est injecté dans chaque `developer-*` et dans `orchestrator-dev` — producteur et consommateur partagent le même contrat.

---

## Principe fondamental — bloc unique

> **Contrat de handoff :** voir skill `shared/handoff-bloc-unique-rule` pour les règles universelles producer/consumer.

---

## Format du bloc `## Retour vers orchestrator-dev`

> **Template :** format défini dans `templates/developer-handoff-block.md` — charger via `read` quand tu produis ce bloc.

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

## Exemple

> **Template et exemple complet :** définis dans `templates/developer-handoff-block.md` — charger via `read` quand tu produis ce bloc.

---

## Règles pour le producteur (developer-*)

- **`### Contexte et décisions`** est le champ qui capture le "pourquoi" — minimum 2 entrées pour toute implémentation non triviale, chaque entrée = choix + raison
- **`**Diff résumé**`** : exécuter `git diff --stat HEAD~1` (ou `git diff --stat <branche-base>...HEAD` si plusieurs commits) et coller la sortie sur une seule ligne condensée
- **`**Changements par fichier**`** : pour chaque fichier du diff, lister les symboles changés avec la notation `+/-/~` — ne pas inventer, ne pas résumer arbitrairement
- **`### Critères d'acceptance couverts`** doit être basé sur `bd show <ID>` — cocher chaque critère explicitement
- **`### Points d'attention pour la review`** est critique : c'est ce qui permet au reviewer de concentrer son attention sur les zones sensibles
- **`### Données techniques brutes`** : stacktraces, diffs annotés, extraits de code — uniquement si nécessaire à la review ou au diagnostic. Sinon "Aucune"
- **`### Questions bloquantes`** (obligatoire) — liste numérotée des questions nécessitant une réponse avant de poursuivre. Si aucune : écrire `Aucune.`. Le coordinateur **doit** résoudre ces questions avant de continuer le workflow.
- **Toujours passer le ticket en `review`** avant de produire ce bloc (sauf si statut = `bloqué`)
- Si statut = `bloqué` : exécuter `bd update <ID> -s blocked` + `bd comments add <ID> "Bloqué par : <raison>"` avant de produire le bloc
- Ne jamais dupliquer dans du texte libre ce qui est déjà dans les champs structurés du bloc

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

4. **Traiter les `### Questions bloquantes`** — si la liste n'est pas `Aucune.`, résoudre chaque question avant de continuer le workflow.

5. **Intégrer les données structurées du bloc** (`### Contexte et décisions`, `**Diff résumé**`, `**Changements par fichier**`, `### Critères d'acceptance couverts`, `### Points d'attention`, `### Questions bloquantes`) dans le récap global structuré (section "Récap global — Fin de session").

5. **Si le bloc est absent** → demander explicitement au developer de le produire avant de continuer.

> ❌ Ne jamais passer à la review sans avoir reçu le `### Statut` — une implémentation `bloqué` ne doit pas être soumise au reviewer.
> ❌ Ne jamais ignorer les `### Points d'attention` — les transmettre intégralement au reviewer.
> ❌ Ne jamais transmettre les `**Changements par fichier**` au reviewer à la place d'un vrai diff — toujours passer le nom de branche pour que le reviewer récupère lui-même le diff complet.
