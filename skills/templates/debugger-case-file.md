# Templates — Debugger : Mode Forensique

## Template du case file

```markdown
# Investigation — {slug}

**Ouvert le :** {date}
**Statut :** open

## Contexte
{description du symptôme tel que fourni — hypothèse à valider}

## Stronghold (point d'ancrage)
{première preuve Confirmed — path:line ou commit hash}

## Hypothèses

| ID | Hypothèse | Grade | Statut | Confirme par | Réfute par |
|----|-----------|-------|--------|--------------|------------|
| H1 | {description} | Hypothesized | Open | {ce qui confirmerait} | {ce qui réfuterait} |

## Évidence collectée

| ID | Observation | Grade | Source |
|----|------------|-------|--------|
| E1 | {fait} | Confirmed | {path:line} |

## Timeline des événements
{ordre chronologique des faits Confirmed}

## Missing evidence
{ce qui n'a pas pu être observé — finding en soi}

## Conclusion
{réservé à la Phase 5 — ne pas remplir avant}
```

---

## Résumé de session (reprise)

```markdown
## [Forensique] Résumé de session — {slug}

**Hypothèses open :** {liste H-ID + statut}
**Backlog d'exploration :** {pistes non encore explorées}
**Missing evidence :** {ce qui manque encore}
**Dernière preuve Confirmed :** {E-ID — description courte}
```
