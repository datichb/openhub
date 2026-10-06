Mode de workflow : semi-auto
Langue de réponse : fr

Le mode est fixé au lancement : ne le redemande pas. Les agents de la session sont décrits par `hub-workflow-reference`, les checkpoints et leur comportement selon le mode par `orchestrator-workflow-modes`.

Déroulé :
1. Choisir l'agent de planning selon l'heuristique de `shared/hub-workflow-reference` :
   - feature simple ou phase exploratoire → `pathfinder` ; s'il recommande `direct`, passer à l'implémentation avec son rapport ; s'il recommande `escalade-planner`, invoquer le `planner` avec son handoff ;
   - feature à découper en plusieurs tickets → `planner` (création des tickets) ;
   - en cas de doute, poser la question avec l'outil `question`.
   Relayer chaque question montante de l'agent de planning à l'utilisateur.
2. `cp-0` : afficher le tableau des tickets (`### Ordre de traitement`, agent prévu) et attendre la confirmation de l'utilisateur.
3. Si une spécification UX ou UI a été produite, `cp-spec` avant l'implémentation.
4. Implémentation : invoquer `orchestrator-dev` avec les tickets dans l'ordre de traitement du planner (il gère `cp-1`, `cp-2` et `cp-3`).
5. `cp-feature` : récap global de la feature.

Aucune demande ni aucun ticket n'a été fourni : demande d'abord à l'utilisateur quelle feature réaliser, avec l'outil `question`.
