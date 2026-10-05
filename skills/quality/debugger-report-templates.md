---
name: debugger-report-templates
description: Templates de rapport de diagnostic (Phase 5) — structure exacte du rapport, template de ticket Beads de correction, commandes bd create/update, labels, priorités suggérées. Chargé à la demande en Phase 5 après validation explicite.
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

> **Template :** format défini dans `templates/debugger-report.md` (section « Rapport de diagnostic ») — charger via `read` quand tu produis ce bloc.

### ÉTAPE 5.2 — Proposer la création du ticket Beads

Afficher le contexte en texte :

> **Template :** format défini dans `templates/debugger-report.md` (section « Ticket de correction suggéré — affichage ») — charger via `read` quand tu produis ce bloc.

⚠️ **RAPPEL** : Le contexte du ticket suggéré (ci-dessus) **doit être affiché en texte** dans la discussion AVANT l'appel `question`. Si ce n'est pas fait → afficher le ticket suggéré MAINTENANT.

Puis appeler l'outil `question` :

**Si CONTEXTE = standalone :**

> **Template :** format défini dans `templates/debugger-report.md` (section « Question de confirmation — CONTEXTE standalone ») — charger via `read` quand tu produis ce bloc.

**Si CONTEXTE = orchestrator_feature :**

> **Template :** format défini dans `templates/debugger-report.md` (section « Retour intermédiaire + Question — CONTEXTE orchestrator_feature ») — charger via `read` quand tu produis ce bloc.

→ **TERMINER LA SESSION**

**Si oui :**

> **Template :** format défini dans `templates/debugger-report.md` (section « Commandes bd create/update ») — charger via `read` quand tu produis ce bloc.

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
