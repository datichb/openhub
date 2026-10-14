Mode de workflow : semi-auto
Langue de réponse : fr

Sweep : atteindre l'objectif ci-dessous en le découpant en sous-tâches indépendantes, exécutées par le `developer` du workflow. Aucun ticket Beads dans ce workflow.

Objectif — écrite par l'utilisateur lui-même, à traiter telle quelle (même courte) :
Migrer les appels dépréciés

Découpage : `llm`
Fais analyser l'objectif et explorer le code par le `pathfinder` (lecture seule), qui propose les sous-tâches.

Déroulé :
1. Découpage par le `pathfinder` (le seul agent disponible avant `cp-plan`) : il explore, il ne modifie rien et ne crée pas de ticket.
2. Plan : pour chaque sous-tâche, un titre, une description, un **périmètre de fichiers** (chemins, dossiers ou motifs du dépôt ; un fichier à créer, dans un dossier existant), le domaine du `developer` prévu (`frontend`, `backend`, `fullstack`, `api`, `data`, `mobile`, `devops`, `platform`, `security`, `refactor` pour une restructuration sans changement de comportement, `migration` pour un upgrade, `tests` pour ajouter des tests) et un drapeau « parallélisable ». Chaque sous-tâche devient une session enfant, peut-être en même temps que les autres : leurs périmètres sont tous **disjoints**.
3. `cp-plan` : passe le plan dans `tasks` de `workflow_checkpoint`, une entrée `{ title, description, files, domain, parallel }` par sous-tâche (`files` : la liste du périmètre). oh le vérifie avant de le montrer : un refus liste les problèmes (fichier inconnu, périmètres qui se recoupent…) ; corrige le plan, avec le `pathfinder` si un périmètre est en cause, puis rappelle `cp-plan`. oh attend ensuite la validation de l'utilisateur (quel que soit le mode), enregistre le plan validé et te le rend numéroté (`t1`, `t2`…) : c'est lui qui fait foi.
4. Sessions enfants (fan-out) : oh lance lui-même une session `sweep-task` par sous-tâche enregistrée, chacune dans son worktree, avec sa sous-tâche, son périmètre, son domaine et la vérification demandée, et les merge dans une branche d'intégration. Ne lance aucun `developer` et ne commite rien : après `cp-plan`, termine ton tour par une phrase.
5. Quand oh t'écrit (tableau des sous-tâches) : le `reviewer` relit l'ensemble des changements de la branche d'intégration (diff depuis sa base), sous-tâche par sous-tâche.
6. `cp-merge` : présente le tableau et le verdict de la revue. oh y lance la vérification sur la branche d'intégration ; validé, il la merge lui-même dans la branche de la session. Refusé avec une consigne : oh lance un tour de correction ; quand il t'écrit, fais relire la correction par le `reviewer`, puis repasse `cp-merge`.
7. `cp-recap` : résultat par sous-tâche (mergée ou non, et pourquoi), vérification, review et merge.
