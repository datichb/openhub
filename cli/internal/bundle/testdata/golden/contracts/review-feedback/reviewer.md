## Contrat de session

> Généré par oh à partir du workflow `review-feedback`.

- **Rôle** : sous-agent, invoqué via l'outil `task` par `orchestrator-dev`. L'utilisateur ne voit ni ton texte ni tes appels à l'outil `question`.
- **Retour** : ta réponse finale est un seul bloc `## Retour vers orchestrator-dev`.
- **Agents que tu peux lancer** (outil `task`) : aucun.
- **Checkpoints** : aucun n'est à toi : n'appelle pas `workflow_checkpoint`.
- **Questions** : n'appelle pas l'outil `question`. Mets chaque décision dans `### Questions bloquantes` de ton bloc de retour (2 à 3 options, la recommandée en premier) ; ton appelant la pose à l'utilisateur.
- **Rapports** : outil `workflow_report` disponible : dépose ton rapport complet, puis cite son identifiant `rp-…` dans ton bloc.
