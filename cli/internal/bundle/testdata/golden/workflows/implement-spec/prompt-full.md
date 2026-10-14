Mode de workflow : semi-auto
Langue de réponse : fr

Implémentation d'une spec en fan-out (expérimental) : oh lance lui-même une session enfant par ticket (workflow `ticket`, sa branche et son worktree), une fois `cp-plan` passé ; chaque enfant implémente, fait relire et commite son ticket, et son `cp-2` est remplacé par ton `cp-merge`. oh merge chaque enfant dans une branche d'intégration (un conflit qu'il ne résout pas est confié au `merger`), puis t'écrit le résultat.
Tickets : bd-2, bd-3
Spec des tickets : `src/exemple` (transmets son chemin au reviewer).

Dans cette session, tu ne délègues pas au développeur, et ni toi ni tes sous-agents ne commitent, ne mergent, ne poussent ni ne ferment de ticket : oh le refuse et le fait lui-même. Le mode est fixé au lancement : ne le redemande pas.

Consignes de l'utilisateur — écrite par l'utilisateur lui-même, à traiter telle quelle (même courte) :
Exemple de valeur pour instructions

Déroulé :
1. Plan : `bd show <ID>` pour chaque ticket, une commande par ticket, lancée seule (sans `cd`, `&&` ni `echo` : tu es déjà dans le projet). oh lance un ticket après ceux dont il dépend, et aucun pour un ticket déjà fermé.
2. `cp-plan` : passe `cp-plan` (`workflow_checkpoint`) avec le tableau des tickets (ordre, dépendances, tickets déjà fermés).
3. `cp-plan` validé : termine aussitôt ton tour par une phrase (« Sessions enfants lancées par oh. »), sans rien appeler d'autre. oh t'écrira quand les enfants seront terminés et mergés.
4. Message d'oh reçu (tableau des enfants, branche d'intégration et sa base) : invoque le `reviewer` en review adversariale de l'ensemble, sur la branche d'intégration (`git diff <base>...<branche d'intégration>`, tickets, spec `src/exemple`), sans rien modifier.
5. `cp-merge` : le tableau des enfants (état, merge, conflits résolus) et le verdict de la revue. oh vérifie la branche d'intégration ; validé, oh la merge dans ta branche et ferme les tickets mergés. Si le message d'oh dit que le fan-out s'est arrêté, ne passe pas `cp-merge`.
6. `cp-final` : tickets mergés et fermés par oh, tickets non mergés et pourquoi, points ouverts. Pas de tour de correction dans cette session : propose un nouvel `implement-spec` pour les tickets restants.

Commence par lire les tickets avec `bd show`.
