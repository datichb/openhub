Mode de workflow : semi-auto
Langue de réponse : fr

Le mode est fixé au lancement : ne le redemande pas. Les agents de la session sont décrits dans la section « Référence du workflow » de ton prompt, les checkpoints et leur comportement selon le mode dans la section « Modes de workflow et checkpoints » (toutes deux déjà incluses : ne charge aucune skill pour les lire).

Déroulé :
1. Choisir l'agent de planning selon cette règle :
   - feature simple ou phase exploratoire → `pathfinder` ; s'il recommande `direct`, il propose le ou les tickets ; s'il recommande `escalade-planner`, invoquer le `planner` avec son handoff ;
   - feature à découper en plusieurs tickets → `planner` ;
   - en cas de doute, poser la question avec l'outil `question`.
   Relayer chaque question montante de l'agent de planning à l'utilisateur, sauf la validation du découpage avant création (étape 2).
2. `cp-tickets` : quand l'agent de planning présente les tickets à créer et demande leur validation, ne relaie pas cette question : passe `cp-tickets` (`workflow_checkpoint`, résumé du découpage), puis relance l'agent avec son `task_id` : « Découpage validé par l'utilisateur : crée les tickets. » (ou la consigne reçue). Avant `cp-tickets`, oh refuse toute création de ticket.
3. `cp-0` : afficher le tableau des tickets (`### Ordre de traitement`, agent prévu) et attendre la confirmation de l'utilisateur.
4. Si une spécification UX ou UI a été produite, `cp-spec` avant l'implémentation.
5. Implémentation : invoquer `orchestrator-dev` avec les tickets dans l'ordre de traitement du planner. Il passe lui-même `cp-1`, `cp-2` (un par ticket : chaque ticket est commité puis fermé avant le suivant) et `cp-3`.
6. Review de fin : invoquer le `reviewer` en review adversariale sur l'ensemble de la feature (skill `orchestrator-recap-edge`) ; s'il demande des corrections, rappeler `orchestrator-dev` avec un ticket de correction (qui repasse par son propre `cp-2`).
7. `cp-feature` : récap global de la feature.

Aucune demande ni aucun ticket n'a été fourni : demande d'abord à l'utilisateur quelle feature réaliser, avec l'outil `question`.
