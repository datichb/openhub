Mode de workflow : semi-auto
Langue de réponse : fr

Le mode est fixé au lancement : ne le redemande pas. Les agents de la session sont décrits dans la section « Référence du workflow » de ton prompt, les checkpoints et leur comportement selon le mode dans la section « Modes de workflow et checkpoints » (toutes deux déjà incluses : ne charge aucune skill pour les lire).

Demande de l'utilisateur — écrite par l'utilisateur lui-même, à traiter telle quelle (même courte) :
Exemple de valeur pour request

Tickets Beads existants à prendre en charge : bd-2, bd-3

Déroulé :
1. Tickets existants : invoquer le `planner` avec `Mode classification — déterminer l'agent et l'ordre de traitement pour les tickets : bd-2, bd-3` (IDs bruts, sans `bd show`).
2. `cp-tickets` : confirmer la liste des tickets (aucune création ; s'il faut en créer, l'indiquer dans le résumé).
3. `cp-0` : afficher le tableau des tickets (`### Ordre de traitement`, agent prévu) et attendre la confirmation de l'utilisateur.
4. Si une spécification UX ou UI a été produite, `cp-spec` avant l'implémentation.
5. Implémentation : invoquer `orchestrator-dev` avec les tickets dans l'ordre de traitement du planner. Il passe lui-même `cp-1`, `cp-2` (un par ticket : chaque ticket est commité puis fermé avant le suivant) et `cp-3`.
6. Review de fin : invoquer le `reviewer` en review adversariale sur l'ensemble de la feature (skill `orchestrator-recap-edge`) ; s'il demande des corrections, rappeler `orchestrator-dev` avec un ticket de correction (qui repasse par son propre `cp-2`).
7. `cp-feature` : récap global de la feature.
