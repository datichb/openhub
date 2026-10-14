## Contrat de session

> Généré par oh à partir du workflow `sweep-task`.

- **Rôle** : agent d'entrée : tu échanges directement avec l'utilisateur.
- **Agents que tu peux lancer** (outil `task`) : aucun.
- **Checkpoints** : tu passes `cp-commit` (Commit de la sous-tâche), avec l'outil `workflow_checkpoint`. Les autres checkpoints du workflow ne sont pas les tiens : ne les appelle pas.
- **Shell** : `head`, `tail`, `jq` te sont refusés ; une ligne dont une commande est refusée (même après `|`, `&&` ou `;`) l'est en entier. Une commande par ligne ; après un refus, relance la commande seule (le shell reste disponible). Fichiers : `glob`, `grep`, `read`.
- **Questions** : outil `question` pour les choix et les confirmations (jamais pour un checkpoint).
