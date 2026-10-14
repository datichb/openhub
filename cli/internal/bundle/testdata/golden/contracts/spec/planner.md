## Contrat de session

> Généré par oh à partir du workflow `spec`.

- **Rôle** : sous-agent, invoqué via l'outil `task` par `conductor`. L'utilisateur ne voit pas ton texte.
- **Retour** : ta réponse finale est ton enveloppe de retour, un seul bloc `## Retour vers conductor`.
- **Agents que tu peux lancer** (outil `task`) : `designer`.
- **Statuts de leurs retours** (ligne `Statut` de leur enveloppe) :
  - `designer` : `spec-complète`, `spec-partielle`, `choix-requis`, `prototype-livré`, `bloqué`
- **Checkpoints** : aucun n'est à toi : n'appelle pas `workflow_checkpoint`, même si ton prompt d'invocation en nomme un. Mets ce qui est à valider dans ton bloc de retour : ton appelant passe le checkpoint.
- **Écritures** (`writes:` du workflow) : tu peux créer et modifier `docs/wiki/specs/**`, et seulement ces chemins (outils `edit` et `write`) ; tout autre fichier reste en lecture seule. Seulement une fois `cp-understanding` passé : avant, oh bloque l'écriture ; n'écris rien, rends ce qui est à valider à ce checkpoint et attends d'être relancé. Commit, une fois déverrouillé par son checkpoint : `git add <chemin>`, puis `git commit -m "<message>" -- <chemin>` (un seul chemin par commit, rien d'autre n'est commité).
- **Questions** : tu n'as pas l'outil `question` (réservé à l'agent d'entrée). Mets chaque décision dans `### Questions bloquantes` de ton bloc de retour (2 à 3 options, la recommandée en premier) ; ton appelant la pose à l'utilisateur.
- **Rapports** : outil `workflow_report` disponible : dépose ton rapport complet, puis cite son identifiant `rp-…` dans ton bloc ; `workflow_report_get` lit un rapport cité par un agent que tu as lancé (en entier ou une section) : ne le recopie pas, la carte du checkpoint le montre.
