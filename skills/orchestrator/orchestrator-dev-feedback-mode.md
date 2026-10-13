---
name: orchestrator-dev-feedback-mode
description: "Utiliser quand le prompt contient [MODE:feedback] : classer le feedback, cp-fix, corrections, review, cp-2, commit."
---

# Mode Feedback — Correction depuis review humaine

> Ce skill est chargé automatiquement quand le prompt contient `[MODE:feedback]`.
> Il **remplace** le workflow standard (étapes 1 à 6) par un workflow de correction ciblée. Il n'y a **pas de ticket Beads** : le travail est identifié par la MR et sa branche.

## Détection

Si le prompt contient `[MODE:feedback]`, appliquer **ce workflow à la place** du workflow standard `orchestrator-dev-ticket-workflow`.

## Workflow

### Étape 1 : Lecture du feedback

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
3. Présenter la synthèse classée dans la discussion (une ligne par discussion : sévérité, fichier, correction prévue)

### Étape 2 : `cp-fix` — corrections à appliquer

```
workflow_checkpoint({ id: "cp-fix", summary: "Feedback MR : X critiques, Y majeures, Z suggestions. Proposé : tout corriger (les suggestions sont optionnelles)." })
```

La réponse peut restreindre les corrections (« critiques et majeures seulement ») ou arrêter : applique-la. Le `developer` et le `reviewer` ne peuvent pas être lancés avant `cp-fix`.

### Étape 3 : Correction par le developer (sans commit)

Déléguer via `task(subagent_type: "developer")` avec ce prompt :

```
[Retours reviewer humain — Feedback MR]
MR : <ref>
Branche : <branch>

Action requise :
1. Appliquer les corrections ci-dessous (pas de ticket : saute les étapes `bd`)
2. Lancer les tests du projet (et le lint s'il existe) ; corriger ce qui échoue
3. Ne pas commiter : rendre la main avec la liste des fichiers modifiés et le résultat des tests

### Corrections requises
- [HUMAN:CRITIQUE] `path/to/file.go:42` — <description de la correction>
- [HUMAN:MAJEUR] `path/to/other.go:15` — <description de la correction>
- [HUMAN:MINEUR] — <suggestion optionnelle>
```

> **Important** : Les findings sont tagués `[HUMAN:xxx]` pour que le developer sache que la source est un reviewer humain. Le protocole `reviewer-reception` s'applique avec un niveau de confiance plus élevé pour les findings humains.

Tu ne lances aucune commande toi-même : les tests sont lancés par le developer, tu lis leur résultat dans son retour.

### Étape 4 : Nouvelle review

Déléguer via `task(subagent_type: "reviewer")` :

```
[SKILL:reviewer/reviewer-subagent] [MODE:feedback]
Branche : <branch> — base : <base>
Diff : git diff <base>...HEAD, plus les modifications non commitées de l'arborescence (git diff, git status)
Vérifie que chaque retour ci-dessous est traité, et qu'aucune régression n'est introduite.

### Retours à vérifier
<la liste des corrections requises, telle quelle>
```

Retour `corriger` → renvoyer au developer (étape 3) avec les retours du reviewer, puis nouvelle review — au plus le nombre de tours de correction indiqué dans la section « Mode de workflow » ; au-delà, passer à `cp-2` en listant les points non résolus dans le `summary`.

### Étape 5 : `cp-2` — commit

Afficher le rapport de review intégralement, puis :

```
workflow_checkpoint({ id: "cp-2", summary: "Feedback MR : X/Y corrections appliquées, tests <résultat>, verdict du reviewer : <verdict>. Validation = commit." })
```

Validé → relancer aussitôt le developer : « `git add` des fichiers corrigés, puis `git commit -m "fix(review): address reviewer feedback"` (hooks compris, jamais `--no-verify`). » Avant `cp-2`, oh refuse tout `git commit`. Refusé → correction selon la consigne reçue (étape 3), nouvelle review, puis `cp-2` de nouveau.

### Étape 6 : Récap

Le push et les réponses aux fils de la MR **reviennent à l'utilisateur** (oh refuse `git push` dans ce workflow ; aucun agent ne répond sur la MR) :

```
## Récap feedback MR

Corrections appliquées : X/Y
- [✓] path/to/file.go:42 — description
- [✓] path/to/other.go:15 — description
- [—] suggestion ignorée

Commit : <hash>
Tests : ✓ passent
Review : <verdict>

À faire par toi : pousser la branche (`git push`), puis répondre aux fils et les résoudre sur la MR.
```

## Compteur de cycles

Le compteur de cycles humain est **séparé** du compteur AI :
- Ne pas compter les cycles AI précédents
- La limite de tours de correction (developer → reviewer) de la section « Mode de workflow » s'applique aux tours de cette session
- Avertir si c'est le 3ème cycle humain : "C'est la 3ème itération de feedback humain. Si les corrections ne sont pas satisfaisantes, envisagez une discussion directe."
