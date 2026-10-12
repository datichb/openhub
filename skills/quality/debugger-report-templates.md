---
name: debugger-report-templates
description: Templates de rapport de diagnostic (Phase 5) — structure exacte du rapport, template de ticket Beads de correction, commandes bd create/update, labels, priorités suggérées. Chargé à la demande en Phase 5 après validation explicite.
annexes: [templates/debugger-report.md]
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

⚠️ **RAPPEL** : Le contexte du ticket suggéré (ci-dessus) **doit être affiché en texte** dans la discussion AVANT la demande de confirmation. Si ce n'est pas fait → afficher le ticket suggéré MAINTENANT.

La confirmation dépend du tableau des checkpoints de ta carte du workflow (section « Modes de workflow et checkpoints ») : s'il liste le checkpoint de création du ticket, suis le cas **Checkpoint** plus bas ; sinon (session `libre`, par exemple), suis le cas **Question**.

#### Cas Question — aucun checkpoint de création du ticket dans la carte

Appeler l'outil `question` :

**Si CONTEXTE = standalone :**

> **Template :** format défini dans `templates/debugger-report.md` (section « Question de confirmation — CONTEXTE standalone ») — charger via `read` quand tu produis ce bloc.

**Si CONTEXTE = orchestrator_feature :**

> **Template :** format défini dans `templates/debugger-report.md` (section « Retour intermédiaire + Question — CONTEXTE orchestrator_feature ») — charger via `read` quand tu produis ce bloc.

→ **TERMINER LA SESSION**

#### Cas Checkpoint — la carte liste `cp-ticket` (workflow `debug`)

Ne pose pas de question pour confirmer : la création passe par le checkpoint, et oh refuse `bd create` avant son passage.

> **Template :** format défini dans `templates/debugger-report.md` (section « Checkpoint de création du ticket — workflow debug ») — charger via `read` quand tu produis ce bloc.

- Validé → crée le ticket (commande ci-dessous), aussitôt.
- Refusé → suis la consigne reçue (ajuster le ticket, puis rappeler `workflow_checkpoint` avec `id: "cp-ticket"` ; ou ne pas le créer).

**Si oui (ou checkpoint validé) :**

> **Template :** format défini dans `templates/debugger-report.md` (section « Commandes bd create/update ») — charger via `read` quand tu produis ce bloc.

> Le label `from-diagnostic` signale que le ticket provient d'un rapport de diagnostic.

**Règles :**
- Toujours utiliser `--json` sur `bd create`
- Toujours lire l'ID dans la réponse JSON de `bd create` (tes permissions refusent `jq` et les variables shell)
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
