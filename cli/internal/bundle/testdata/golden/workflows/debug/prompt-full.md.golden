Mode de workflow : manuel
Langue de réponse : fr

Debug le problème signalé par l'utilisateur — écrite par l'utilisateur lui-même, à traiter telle quelle (même courte) :
Exemple de valeur pour issue

Session de debug autonome (pas d'orchestrateur) : présente directement à l'utilisateur ton rapport de diagnostic, en commençant par les actions d'urgence si le bug touche la production. Tu ne corriges pas toi-même : propose un ticket de correction, que tu crées après `workflow_checkpoint` (`id: "cp-ticket"`, `summary` : diagnostic et ticket proposé) — oh refuse `bd create` avant ; l'implémentation se fera ensuite avec le workflow `ticket` (ou avec l'agent `developer`, disponible à la demande dans cette session).

Si une correction est faite dans cette session (agent `developer`), elle se termine par `workflow_checkpoint` (`id: "cp-commit"`, `summary` : fichiers modifiés, tests) : le commit n'est possible qu'après sa validation (oh refuse `git commit` avant), puis la fermeture du ticket corrigé (`bd close`, refusée avant le checkpoint et sans commit). Le push revient à l'utilisateur.
