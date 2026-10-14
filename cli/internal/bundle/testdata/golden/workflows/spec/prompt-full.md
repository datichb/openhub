Mode de workflow : semi-auto
Langue de réponse : fr

Spécification d'une feature, sans implémentation : le planner écrit la spec dans le wiki (`docs/wiki/specs/<slug>.md`, seul fichier modifié), puis la découpe en tickets Beads.

Demande de l'utilisateur — écrite par l'utilisateur lui-même, à traiter telle quelle (même courte) :
Exemple de valeur pour request

Déroulé (le planner suit les mêmes étapes, voir la section « Spec écrite » de son prompt ; relance-le chaque fois avec son `task_id`) :
0. Exploration, seulement si la demande compare des conceptions ou demande d'analyser le code avant de spécifier : invoquer d'abord le `pathfinder` (marqueur `[CONTEXTE]`) ; relaie ses questions bloquantes, puis donne l'identifiant de son rapport (`rp-…`) au planner à l'étape 1.
1. Entretien : invoquer le `planner` avec la demande et « Étape : entretien ». Relaie chacune de ses questions bloquantes à l'utilisateur, telle quelle (une question par décision), puis relance-le avec les réponses.
2. `cp-understanding` : quand il rend sa reformulation (statut `compréhension-à-valider`), passe `cp-understanding` en présentant la reformulation, le vocabulaire (mot de la demande → nom dans le code) et les réponses reçues.
3. Spec : relance le planner : « Compréhension validée. Étape : spec. » (ou la consigne reçue). Il écrit la spec, découpage compris.
4. `cp-converge` : quand il rend la spec (statut `spec-à-valider`), passe `cp-converge` : sans pause, oh vérifie le fichier (glossaire, point de test de chaque critère, chaque critère couvert par une tranche, aucune tranche hors spec). Un refus liste les écarts : relance le planner avec cette liste (il corrige, ou pose la question si l'écart demande une décision), puis repasse `cp-converge`.
5. `cp-spec` : passe `cp-spec` (chemin du fichier, critères et leurs points de test), puis relance le planner : « Spec validée. Étape : commit. » (ou la consigne reçue).
6. `cp-tickets` : quand il a commité (statut `découpage-à-valider`), passe `cp-tickets` (les tranches du découpage, un ticket chacune). Avant `cp-tickets`, oh refuse toute création de ticket.
7. Création : relance le planner : « Découpage validé par l'utilisateur. Étape : création. » Si un checkpoint demande de changer la spec ou le découpage, relance le planner avec la consigne (« Étape : spec ») : le fichier modifié repasse par `cp-converge`, `cp-spec` (nouveau commit) et `cp-tickets`.
8. Si la feature a une interface, le `designer` produit la spécification UX/UI (invoqué par le planner ou par toi).
9. Fin : chemin de la spec, tableau des tickets créés (`### Ordre de traitement`, agent prévu). oh relève lui-même la spec et les tickets créés (ne les déclare pas) et proposera à l'utilisateur de lancer `implement-spec` sur ces tickets.
