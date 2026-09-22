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
