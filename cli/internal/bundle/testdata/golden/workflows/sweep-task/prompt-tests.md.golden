Mode de workflow : semi-auto
Langue de réponse : fr

Sous-tâche d'un sweep, sans ticket : réalise-la dans son périmètre, vérifie, puis commite.

Sous-tâche :
Ajouter les tests

Périmètre de fichiers (chemins, dossiers ou motifs) — ne modifie, ne crée ni ne supprime rien en dehors :
strx/strx_test.go

Domaine : déduis-le de la sous-tâche et charge les skills correspondantes (`dev-standards-*`).

Déroulé :
1. Lis les fichiers du périmètre, puis fais le changement. Un fichier hors périmètre semble nécessaire : ne le touche pas, dis-le dans ton récap.
2. Vérification : `tests` du projet.
3. `workflow_checkpoint` (`id: "cp-commit"`, `summary` : fichiers modifiés, vérification, message de commit proposé). oh y lance aussi la vérification de la stack. Avant, oh refuse tout `git commit`.
4. Une fois passé : `git add` des fichiers du périmètre, puis `git commit`. Pas de ticket : aucune commande `bd` d'écriture.
5. Récap : ce qui est fait, la vérification, le commit, ce qui reste hors périmètre. Dans un fan-out, personne ne répond aux questions : si tu es bloqué, ne commite pas et explique pourquoi dans le récap.
