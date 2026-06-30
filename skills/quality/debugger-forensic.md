---
name: debugger-forensic
description: Mode forensique du debugger — activé par le flag --forensic, grading d'évidence (Confirmed/Deduced/Hypothesized), format du case file, protocole Stronghold-first, règles forensiques et intégration avec le workflow standard. Chargé à la demande quand --forensic est présent dans le prompt.
bucket: B
---

# Skill — Debugger : Mode Forensique

## Contexte d'usage

Ce skill est chargé par `debugger-workflow` quand le prompt contient `--forensic`
ou que l'utilisateur demande explicitement une investigation forensique.

---

## Mode Forensique (`--forensic`)

### Activation

Déclenché quand le prompt contient `--forensic` ou que l'utilisateur demande explicitement une investigation forensique.

Confirmer l'activation :
> `[debugger --forensic] Mode forensique actif — investigation basée sur le grading d'évidence. Un case file sera créé dès validation du slug.`

---

### Principe : Stronghold-first

Le mode forensique ne part jamais d'une théorie. Il part d'une **preuve Confirmed** et construit à partir d'elle.

**Règle absolue** : La description de l'utilisateur est une **hypothèse**, pas un fait. La valider ou l'invalider avant d'en déduire quoi que ce soit.

---

### Grading d'évidence

Chaque observation est classée avant d'être utilisée dans le raisonnement :

| Grade | Définition | Format |
|-------|-----------|--------|
| **Confirmed** | Directement observé — citer `path:line` ou hash de commit | `[Confirmed] <fait> — source : <path:line>` |
| **Deduced** | Découle logiquement de preuves Confirmed — montrer la chaîne | `[Deduced] <inférence> — chaîne : <C1> → <C2> → <inférence>` |
| **Hypothesized** | Plausible mais non confirmé — énoncer ce qui confirmerait ou réfuterait | `[Hypothesized] <hypothèse> — confirmerait : <X> / réfuterait : <Y>` |

> ❌ Ne jamais utiliser une observation sans la grader
> ❌ Ne jamais promouvoir une Hypothesized en Deduced sans chaîne de preuves Confirmed
> ✅ Une observation Hypothesized qui ne peut pas être confirmée = **missing evidence** (finding en soi)

---

### Case file

Dès accord sur le slug avec l'utilisateur, créer le case file :

```bash
touch .investigation-{slug}.md
```

**Template du case file :**

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

### Protocole de reprise de session

À chaque reprise d'une investigation en cours, produire obligatoirement le résumé de session :

```markdown
## [Forensique] Résumé de session — {slug}

**Hypothèses open :** {liste H-ID + statut}
**Backlog d'exploration :** {pistes non encore explorées}
**Missing evidence :** {ce qui manque encore}
**Dernière preuve Confirmed :** {E-ID — description courte}
```

---

### Règles forensiques

- **Stronghold-first** : ancrer sur une preuve Confirmed avant tout raisonnement
- **Challenge the premise** : la description de l'utilisateur est une hypothèse à valider
- **Hypothèses jamais supprimées** : update Status → `Confirmed` ou `Refuted`, jamais supprimé
- **Missing evidence = finding** : ce qui n'a pas pu être observé est documenté explicitement
- **Delegation discipline** : > 5 fichiers ou > 10K tokens → déléguer l'analyse de ce périmètre à un subagent avec instructions JSON structurées

---

### Intégration avec le workflow standard

En mode `--forensic`, les phases 0–5 s'appliquent normalement avec les enrichissements suivants :

- **Phase 0** : valider le slug et créer le case file avant de démarrer l'exploration
- **Phase 1** : toute observation est gradée (Confirmed/Deduced/Hypothesized) avant d'être notée
- **Phase 3** : les hypothèses sont enregistrées dans le case file avec leur statut
- **Phase 5** : mettre à jour le case file (statuts finaux, conclusion) avant de produire le rapport
