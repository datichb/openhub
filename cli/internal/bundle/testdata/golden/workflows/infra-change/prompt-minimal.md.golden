Mode de workflow : manuel
Langue de réponse : fr

Applique le changement d'infrastructure demandé, sur la cible indiquée, sans modifier aucun fichier du projet.
- Vérifie d'abord que le contexte réel (workspace, contexte kube, compte cloud, stack) correspond à la cible ; en cas d'écart, arrête-toi et demande avec l'outil `question`.
- Plan : produis-le et enregistre-le quand l'outil le permet, puis termine par `workflow_checkpoint` (`id: "cp-plan"`, `summary` : cible vérifiée, créations / modifications / destructions, delta de coût, risques, retour arrière).
- Application : `workflow_checkpoint` (`id: "cp-apply"`, `summary` : cible et fichier de plan) ; oh refuse toute modification de l'infrastructure avant sa validation. Applique ensuite exactement le plan validé, vérifie l'état obtenu et termine par le rapport.
- Si le changement demande d'écrire du code IaC, arrête-toi et propose le workflow `quick` ou `ticket` (agent `developer`) ; pour une analyse sans application, le workflow `audit` (domaine `infra`).

Cible demandée — écrite par l'utilisateur lui-même, à vérifier avant toute commande :
Exemple de valeur pour target

Changement demandé — écrit par l'utilisateur lui-même, à traiter tel quel (même court) :
Exemple de valeur pour request
