Mode de workflow : semi-auto
Langue de réponse : fr

Implémentation d'une spec, ticket par ticket, dans cette session (pas de session par ticket).
Tickets à implémenter : bd-2, bd-3

Le mode est fixé au lancement : ne le redemande pas. Les checkpoints et leur comportement selon le mode sont décrits dans la section « Modes de workflow et checkpoints » de ton prompt (déjà incluse : ne charge aucune skill pour la lire).

Déroulé :
1. Plan : `bd show <ID>` pour chaque ticket (détail, état, dépendances). Ordre de traitement : un ticket après ceux dont il dépend, puis par priorité (P0 > P1 > P2 > P3) ; écarte un ticket déjà fermé, en le signalant.
2. `cp-plan` : passe `cp-plan` (`workflow_checkpoint`) avec le tableau des tickets (ordre, agent prévu, tickets écartés). Aucune implémentation avant.
3. Pour chaque ticket, dans l'ordre du plan :
   - `cp-1`, puis router vers l'agent développeur adapté : s'il n'est pas déjà en cours, c'est le développeur qui réserve le ticket (`bd update <ID> --claim`), demande-le-lui dans son prompt ; il implémente, teste et passe le ticket en review, sans committer. Puis pre-review et review.
   - `cp-2` « Commit ou correction » (jamais l'outil `question`) ; tant qu'il n'est pas validé, oh refuse `git commit` et la fermeture du ticket. Validé : relance aussitôt le développeur pour le commit (Conventional Commits terminé par l'ID du ticket entre parenthèses), puis `bd close <ID> --reason "Implemented in commit <hash>"`. « Corriger d'abord » : relance le développeur avec les corrections, nouvelle review, nouveau `cp-2`.
   - `cp-3`, puis le ticket suivant.
4. Revue de l'ensemble : tous les tickets traités (ou arrêt), invoque le `reviewer` en review adversariale sur tous les commits de la session (`git diff <commit de départ>...HEAD`, tickets traités).
5. `cp-final` : verdict de la revue, tickets traités et leurs commits, points ouverts (récap de fin). Si l'utilisateur demande un tour de correction : relance le développeur avec les findings, review, puis `cp-2` (commit de correction) ; un seul tour, puis récap.

Commence par lire les tickets avec `bd show`.
