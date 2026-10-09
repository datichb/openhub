> [Read in English](051-community-skills-disconnection.en.md)

# ADR-051 — Déconnexion du registre de skills communautaires

## Statut

Accepté

## Date

2026-10-09

## Contexte

L'[ADR-027](027-audit-extensibility.fr.md) (juillet 2026) a introduit un « marketplace » de skills : `oh skill add|list|remove|search`, un index communautaire et l'installation dans `~/.oh/skills/<nom>/`. Le calcul `oh skill budget` (budget « Bucket A » par agent, septembre 2026) date de la même époque. Ces deux fonctions précèdent la v5 et ses paquets de session.

L'audit de la v5 (08/10/2026) a montré qu'elles n'ont plus de sens en l'état :

- **Non fonctionnel.** L'index `datichb/oh-skills-index` n'existe pas (404) : `oh skill search` et `oh skill add <nom>` échouent toujours. L'installation depuis un dépôt GitHub, cas documenté, n'écrit aucun fichier (l'archive de GitHub met les fichiers à la racine, le code les ignorait) alors que la commande affiche « installé ».
- **Risqué.** Les noms et les chemins de l'archive n'étaient pas contrôlés : une archive pouvait écrire hors de `~/.oh` (« zip-slip »), `oh skill remove ..` supprimait tout `~/.oh`, `skill_file` pouvait désigner un fichier hors du paquet. Aucune empreinte, aucune signature, branche `main` suivie, `http` accepté.
- **Contraire au modèle v5.** Une skill installée sur une machine entrait dans le catalogue de cette machine : un même workflow d'équipe donnait des paquets différents selon le poste, sans revue ni trace dans le team-state.
- **Doublon.** `oh skill budget` faisait son propre calcul sur l'ancien modèle « Bucket A », alors que `oh bundle show <workflow> --budget` donne le budget du vrai paquet.

Le catalogue d'équipe (`catalog/{agents,skills}/` du team-state, [ADR-040](040-workflows-team-state-governance.fr.md)) couvre déjà le besoin d'ajouter des briques, versionnées et partagées.

## Décision

Ces fonctions historiques sont **déconnectées**, en attendant une refonte de l'ajout de briques :

- Les commandes `oh skill add`, `list`, `remove`, `search` et `budget` sont supprimées (cobra répond « unknown command »). `oh skill` ne garde que `oh skill check`.
- Le package `internal/skillregistry` et l'ancien calcul `internal/bricks/skill_budget.go` sont retirés (ils restent dans l'historique git).
- `~/.oh/skills` n'est plus lu : ni à la construction des paquets, ni à la validation des workflows, ni par `oh skill check`. Un workflow qui cite une ancienne skill communautaire dans `skills.extra` est invalide (« skill inconnue »).
- **Rien n'est supprimé** chez l'utilisateur. `oh doctor` (et la vue Doctor de la TUI) signale par un avertissement les paquets restés dans `~/.oh/skills` et explique comment en garder un (catalogue d'équipe).

Restent inchangés : `oh skill check`, le catalogue d'équipe (validation, paquets, badge « nouvelle brique »), le catalogue des briques de la TUI, `skills.extra` / `skills.deny` des workflows, `oh bundle show --budget` et la cascade des modèles (`oh config model agent|family`).

## Conséquences

### Positives

- Plus de surface d'attaque liée au registre (écriture hors du dossier, suppression de `~/.oh`, contenu non vérifié dans les paquets).
- Un paquet de session ne dépend plus de la machine : il ne vient que du hub, du team-state et du workflow.
- Une seule mesure du budget, celle du vrai paquet.

### Négatives / Compromis

- Plus de moyen d'ajouter une skill sans team-state. Un utilisateur seul peut créer un espace solo (`oh team init --solo`) et utiliser son catalogue.
- Le catalogue d'équipe se modifie encore à la main (commit dans le dépôt team-state) : pas de commande dédiée en CLI ni en TUI.

## Pistes pour la refonte

À reprendre dans une évolution dédiée (nouvel ADR) :

1. **Commande des briques** (`oh brick` ou `oh catalog`), sur le modèle de `oh workflow` : `list [--agents|--skills] [--origin]`, `show`, `new`, `edit`, `publish`, `history` pour le catalogue d'équipe, avec la même gouvernance et le même contrôle d'intégrité.
2. **Import de briques externes vers le catalogue d'équipe** (et non vers la machine) : version fixée par commit, empreinte vérifiée, noms et chemins contrôlés, `https` seulement, revue avant publication.
3. **TUI** : catalogue des briques éditable, liste de choix pour `skills.extra` / `skills.deny` dans l'éditeur de workflow, champ « agent d'entrée » dans la fiche de lancement quand le workflow le permet (`selectable`), action « session libre avec cet agent ».
4. **Contrôles** : identifiants d'agent et de famille vérifiés par `oh config model agent|family` (CLI et vue Modèles).
5. Option d'un **index partagé**, seulement s'il est signé et versionné.

## Alternatives considérées

| Alternative | Rejetée car |
|-------------|-------------|
| Corriger le registre (contrôle des chemins, format racine, index) | Garde un mécanisme par machine, contraire aux paquets reproductibles ; corrige un usage qui n'existe pas (index absent) |
| Garder les commandes avec un message « déconnecté » | Commandes mortes à maintenir ; une commande inconnue est assez claire, l'ADR et `oh doctor` documentent la migration |
| Garder `oh skill budget` jusqu'à la v6 | Calcul différent de celui du paquet réel, alias trompeur ; `oh bundle show --budget` le remplace |
| Supprimer les paquets de `~/.oh/skills` à la mise à jour | Destructif sans accord de l'utilisateur ; un avertissement suffit |
