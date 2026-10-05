---
name: orchestrator-dev-feedback-mode
description: Mini-workflow de correction basé sur le feedback de review humain. Short-circuite le workflow standard quand le prompt contient [MODE:feedback].
condition: mode_feedback
---

# Mode Feedback — Correction depuis review humaine

> Ce skill est chargé automatiquement quand le prompt contient `[MODE:feedback]`.
> Il **remplace** le workflow standard (Steps 1-6) par un workflow de correction ciblée.

## Détection

Si le prompt contient `[MODE:feedback]`, appliquer **ce workflow à la place** du workflow standard `orchestrator-dev-ticket-workflow`.

## Workflow

### Step 1 : Lecture du feedback

Le prompt contient les discussions MR non résolues sous la forme :

```
--- Discussion N ---
Auteur: @username
Fichier: path/to/file.go:42
Commentaire:
<contenu du commentaire>
```

1. Lire chaque discussion attentivement
2. Classifier par sévérité inférée :
   - Mots-clés bloquants (bug, crash, sécurité, faille, critique, bloquant, must, security, critical) → **Critique**
   - Mots-clés importants (manque, devrait, il faut, wrong, missing, should, fix) → **Majeur**
   - Suggestions (suggestion, idéalement, optionnel, nit, could, consider) → **Mineur**
   - Défaut → **Majeur**
3. Présenter la synthèse classée à l'utilisateur

### Step 2 : CP — Confirmation

Utiliser l'outil `question` pour demander confirmation :

```
Feedback MR à traiter :
- X corrections critiques
- Y corrections majeures  
- Z suggestions

Appliquer toutes les corrections ? (les suggestions sont optionnelles)
```

Options :
- "Tout corriger" (recommended) — appliquer critiques + majeurs + suggestions
- "Critiques et majeurs seulement" — ignorer les suggestions
- "Annuler"

### Step 3 : Délégation au developer

Construire un prompt de délégation au format `### Corrections requises` standard :

```
[Retours reviewer humain — Feedback MR]
Ticket : <ID>
Branche : <branch>

Action requise :
1. Appliquer les corrections ci-dessous
2. Vérifier que les tests passent
3. Faire un commit : fix(review): address reviewer feedback

### Corrections requises
- [HUMAN:CRITIQUE] `path/to/file.go:42` — <description de la correction>
- [HUMAN:MAJEUR] `path/to/other.go:15` — <description de la correction>
- [HUMAN:MINEUR] — <suggestion optionnelle>
```

> **Important** : Les findings sont tagués `[HUMAN:xxx]` pour que le developer sache que la source est un reviewer humain. Le protocol `reviewer-reception` s'applique avec un niveau de confiance plus élevé pour les findings humains.

Déléguer via `Task(subagent_type: "developer")`.

### Step 4 : Post-correction

Après que le developer a terminé :

1. Vérifier que les tests passent (lancer le pre-review : lint/types/tests)
2. Si l'outil `gitlab_reply_to_mr_discussion` est disponible, poster une réponse sur chaque thread adressé :
   > `[OpenHub] Corrections appliquées dans le commit <hash>.`
3. Si l'outil `team_notify` est disponible, notifier l'équipe :
   > "MR mise à jour avec les corrections demandées pour <ticket>"
4. Si l'outil `team_review_verdict` est disponible, enregistrer le verdict :
   > `team_review_verdict({ project, ticket_id, verdict: "rejected", reason: "human-feedback-applied" })`
   (Le verdict "rejected" indique que des corrections ont été apportées — le reviewer humain décidera de l'approbation finale)

### Step 5 : Récap

Produire un récapitulatif :
```
## Récap feedback MR

Corrections appliquées : X/Y
- [✓] path/to/file.go:42 — description
- [✓] path/to/other.go:15 — description
- [—] suggestion ignorée

Commit : <hash>
Tests : ✓ passent
Réponses GitLab : ✓ postées (ou — non disponible)
```

## Compteur de cycles

Le compteur de cycles humain est **séparé** du compteur AI :
- Ne pas compter les cycles AI précédents
- Ne pas appliquer la limite de 3 cycles AI
- Avertir si c'est le 3ème cycle humain : "C'est la 3ème itération de feedback humain. Si les corrections ne sont pas satisfaisantes, envisagez une discussion directe."
