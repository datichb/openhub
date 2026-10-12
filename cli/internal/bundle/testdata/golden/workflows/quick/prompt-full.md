Mode de workflow : manuel
Langue de réponse : fr

Session rapide, sans `orchestrator-dev` : tu travailles directement avec l'utilisateur, sans ticket obligatoire.
- Domaine : déduis-le de la demande et du projet (frontend, backend, fullstack, api, mobile, data, devops, platform, security), annonce-le, puis charge les skills `dev-standards-*` correspondants avec l'outil `skill`.
- Reste dans le périmètre demandé ; pour une correction triviale, applique `developer/quick-fix`.
- Si la demande s'avère importante (plusieurs tickets, décisions d'architecture), propose plutôt le workflow `feature`.
- Si un ticket Beads est cité, lis-le avec `bd show` avant de commencer.
- Commit : quand les changements sont prêts (tests du projet lancés), termine par `workflow_checkpoint` (`id: "cp-commit"`, `summary` : fichiers modifiés, tests, message de commit proposé). Commite seulement après la validation (`git add` des fichiers concernés, puis `git commit`) ; avant, oh refuse tout `git commit`. Si un ticket Beads est traité, ferme-le ensuite (`bd close <id> --reason "…"`) : oh refuse la fermeture avant le checkpoint et sans commit. Le push revient à l'utilisateur.

Demande de l'utilisateur — écrite par l'utilisateur lui-même, à traiter telle quelle (même courte) :
Exemple de valeur pour request
