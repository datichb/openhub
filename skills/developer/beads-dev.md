---
name: beads-dev
description: Workflow exécuteur Beads (bd) — clamer, implémenter, passer en review, gérer les blocages, clore les tickets. Règles ai-delegated. Référence complète dans docs/reference/beads-model.md.
---

## Workflow obligatoire

```
1. bd ready --label ai-delegated --json  → tickets délégués à l'agent
2. bd show <ID>                          → lire le détail (description, acceptance, notes, commentaires)
3. bd update <ID> --claim                → clamer avant de commencer
4. [implémenter + tester]
5. bd update <ID> -s review              → passer en review (attente reviewer)
6. [review — verdict de orchestrator-dev]
   → commit  : git commit -m "..." + bd close <ID> --reason "..." --suggest-next
   → corriger : bd comments add <ID> "Retours reviewer : ..." + bd update <ID> -s in_progress + corriger + retour étape 5
   → pre-review échouée : bd comments add <ID> "Pre-review échouée : ..." + corriger + retour étape 5
```

---

## Clamer un ticket

Avant de commencer à implémenter, clame le ticket pour signaler que tu travailles dessus.
`--claim` est atomique : il échoue si un autre acteur a déjà réclamé le ticket.

```bash
bd update <ID> --claim
```

Cette commande met le statut à `in_progress` et t'assigne le ticket en une seule opération.

---

## Passer en review

Après implémentation et tests, signaler que le travail est prêt pour relecture :

```bash
bd update <ID> -s review
```

Le ticket est maintenant en attente de validation par le **reviewer humain**.
Le reviewer consulte l'implémentation et décide :

### Si la review accepte (via instruction commit de l'orchestrator-dev) :

Quand orchestrator-dev te transmet l'instruction de commit, tu exécutes les deux actions :

```bash
git commit -m "<type>(<scope>): <description>"
bd close <ID> --reason "Implemented in commit <hash>" --suggest-next
```

`--suggest-next` affiche les tickets qui viennent d'être débloqués par cette clôture,
ce qui permet de choisir la prochaine tâche sans relancer `bd ready`.

**Après `bd close` — Enrichissement des documents vivants :**

Appliquer le skill `shared/living-docs-enrichment` :
identifier les patterns, conventions ou contraintes techniques découverts pendant l'implémentation
qui méritent d'être capitalisés dans `CONVENTIONS.md` ou `ONBOARDING.md`.
Si aucune découverte pertinente → afficher `> 💾 Documents vivants : aucune nouvelle découverte à capitaliser.`
et passer au ticket suivant.

### Si la review rejette (retours formulés par orchestrator-dev) :

Quand orchestrator-dev te retransmet les retours reviewer dans le prompt de re-délégation,
tu es responsable de :

1. Poser le commentaire Beads avec les retours (tels quels, sans résumer) :
   ```bash
   bd comments add <ID> "Retours reviewer : <contenu intégral des corrections requises>"
   ```
2. Reprendre le ticket en `in_progress` :
   ```bash
   bd update <ID> -s in_progress
   ```
3. Appliquer les corrections
4. Repasser en review : `bd update <ID> -s review`

> **Règle :** Ne jamais résumer ni reformuler les retours dans le commentaire — les copier tels quels depuis le prompt reçu.

> **Limite :** après 3 cycles de correction sans résolution, signaler le blocage à orchestrator-dev.

### Si la pre-review échoue (instruction de l'orchestrator-dev) :

Quand orchestrator-dev te retourne un ticket suite à un échec de pre-review, tu es responsable de :

1. Poser le commentaire Beads avec le détail de l'erreur :
   ```bash
   bd comments add <ID> "Pre-review échouée : <détail de l'erreur>

   Erreur(s) détectée(s) :
   - <check> : <message d'erreur>

   Action requise : corriger les erreurs ci-dessus et repasser en review."
   ```
2. Corriger les erreurs signalées
3. Repasser en review : `bd update <ID> -s review`

---

## Gérer un blocage

Si un ticket est bloqué par une dépendance ou un facteur externe :

```bash
bd update <ID> -s blocked
bd comments add <ID> "Bloqué par : <raison>"
```

Ajouter un label système si applicable :

```bash
bd update <ID> --add-label needs-decision       # en attente d'une décision humaine
bd update <ID> --add-label needs-clarification   # description insuffisante
```

Quand le blocage est résolu, repasser en `in_progress` :

```bash
bd update <ID> -s in_progress
```

> Si aucun ticket n'est débloquable, utiliser `bd ready --label ai-delegated --json`
> pour trouver un autre ticket à traiter en attendant.

---

## Commentaires

Utiliser les commentaires pour tracer les décisions et échanges sans modifier
la description ou les notes du ticket :

```bash
bd comments add <ID> "Texte du commentaire"
```

---

## Règles strictes

- Toujours `bd show <ID>` avant d'implémenter — ne jamais supposer le contenu d'un ticket
- Toujours clamer avant d'implémenter — évite les conflits si plusieurs agents tournent
- Toujours passer en `review` après implémentation — ne jamais clore directement
- Toujours clore explicitement après validation — ne pas laisser de tickets `in_progress` orphelins
- Ne pas modifier le titre ou la description d'un ticket sans y être invité
- Un ticket `closed` ou `cancelled` n'est jamais rouvert — créer un nouveau ticket si nécessaire

---

## Label `ai-delegated` — délégation à l'agent

Seuls les tickets portant le label **`ai-delegated`** te sont assignés au démarrage.
**L'humain décide quels tickets déléguer** — tu n'ajoutes jamais ce label toi-même.

**Tu ne dois JAMAIS :**
- Prendre un ticket sans label `ai-delegated`, sauf si l'utilisateur te le demande
  explicitement dans la conversation
- Ajouter toi-même le label `ai-delegated` sur un ticket

**Commandes utiles :**
```bash
# Voir tes tickets délégués
bd ready --label ai-delegated --json

# L'humain délègue un ticket à l'agent
bd label add <ID> ai-delegated

# L'humain reprend la main sur un ticket
bd update <ID> --remove-label ai-delegated
```
