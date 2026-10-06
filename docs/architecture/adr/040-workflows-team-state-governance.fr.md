> [Read in English](040-workflows-team-state-governance.en.md)

# ADR-040 — Workflows dans le team-state, gouvernance dans le hub, espace solo

## Statut

Accepté

## Date

2026-10-05

## Contexte

Le dépôt team-state ([ADR-024](./024-team-state-repository.fr.md)) partage déjà les claims, les événements et le wiki d'une équipe ; l'[ADR-029](./029-multi-team-support.fr.md) permet plusieurs équipes par hub, et un projet sans équipe n'a aucun état partagé. La configuration du workflow unique était éparpillée ([ADR-033](./033-config-cascade-enforcement.fr.md)) :

- section `[workflow]` du `config.toml` d'équipe, avec `Enforced` ;
- colonne `projects.workflow_config` en SQLite ;
- surcharges `[workflow.overrides]` de `hub.toml`, qui n'étaient en fait jamais chargées (bogue découvert pendant la migration).

Avec les workflows déclaratifs ([ADR-039](./039-declarative-workflows-oh-v1.fr.md)), il fallait :

- que les équipes et les projets puissent créer, modifier et publier les leurs ;
- que ce qui est chargé soit intègre ;
- que les publications concurrentes et le travail hors ligne soient gérés ;
- que les projets sans équipe fonctionnent de la même façon.

Décisions D5, D7 (gouvernance dans le hub, sans merge request), O12 (résumé d'impact) et O14 (publication).

## Décision

### 1. Arborescence

Le team-state accueille :

- `workflows/published/<id>.yaml`, `workflows/drafts/<membre>/<id>.yaml`, `workflows/prompts/`, `workflows/history/<id>/<version>.{yaml,prompt.md.tmpl,lock.toml}` ;
- les mêmes dossiers sous `projects/<p>/workflows/` pour la couche projet ;
- `catalog/agents/<famille>/<id>.md` et `catalog/skills/<chemin>.md` : briques d'équipe, au format du hub ;
- `workflows.lock` (TOML : `[team.<id>]`, `[projects.<p>.<id>]` avec version, `hash`, `prompt_hash`, auteur, date, message) ;
- les événements `workflow.published`, `workflow.restored` et `workflow.archived`, sous `projects/_team/events/` pour l'équipe ou sous `projects/<p>/events/`.

### 2. Intégrité

- Seuls les fichiers publiés dont le YAML **et** le gabarit de prompt correspondent au `workflows.lock` sont chargés.
- Sinon le fichier est ignoré, avec un avertissement : non verrouillé, hash ou gabarit différent, entrée orpheline, lock illisible. L'avertissement est visible dans le catalogue de la TUI, dans `oh workflow list` et dans Doctor.

### 3. Brouillons

- Un brouillon appartient à un membre. Il est commité et poussé (visible des autres, jamais chargé pour eux) et validé à l'enregistrement avec les autres brouillons du membre ; un brouillon invalide n'est pas écrit.
- Un brouillon peut avoir son propre gabarit de prompt, ce qui ne touche jamais le gabarit publié.
- `oh run <wf> --draft` teste un brouillon : en local ou en conteneur, jamais à distance, et sans pouvoir élargir quoi que ce soit par rapport à la version publiée.

### 4. Publication

`teamstate.Repo.Transact` déroule une transaction : pull → construction → commit → push.

La construction :

1. relit la gouvernance ;
2. calcule l'impact par rapport à la version précédente ;
3. copie la version courante dans `history/` ;
4. écrit la version N+1 dans le texte (commentaires conservés) ;
5. scelle le lock ;
6. **revalide** sur l'état à jour ;
7. signale les workflows qui étendent celui-ci et deviennent invalides ;
8. supprime le brouillon et émet l'événement.

En cas de concurrence ou d'absence de réseau :

- **push refusé** : le commit est abandonné et le cycle refait au-dessus de la publication concurrente, jusqu'à 4 fois ;
- **remote injoignable** : rien n'est commité, l'opération va dans une file locale (`.git/oh-workflow-queue.json`). La file est rejouée à chaque synchronisation de la TUI et par `oh workflow publish --retry`.

Avant publication, un **résumé d'impact** (O12) est affiché. Il couvre le risque, les agents qui écrivent, le distant autorisé, les modes, les checkpoints, Beads, le budget, les MCP, les plugins, le Code Mode, les skills, les entrées et le prompt, ainsi que les « nouvelles briques » d'équipe. Une confirmation est demandée si le workflow est élargi. Le message est obligatoire.

`History`, `Restore` (republie une ancienne version comme nouvelle version) et `Archive` complètent le cycle.

### 5. Gouvernance

- `[governance] publish = "any_member"` dans le `config.toml` de l'équipe : **tout membre** inscrit dans `members.toml` peut publier. C'est la seule valeur prise en charge.
- Une valeur inconnue refuse la publication : on refuse plutôt que d'autoriser.
- La politique est affichée dans le détail de l'équipe (TUI).

### 6. Catalogue de briques d'équipe

- Les agents et skills de `catalog/` sont fusionnés avec ceux du hub dans un dossier en cache (`~/.oh/cache/bricks/<clé>`, reconstruit quand le hub ou les briques changent). Ce dossier sert à la validation et au paquet.
- Une collision d'identifiant avec le hub est **refusée**, sauf `extends: hub:<id>` dans le frontmatter (la brique remplace celle du hub). Une brique refusée est ignorée avec un avertissement.

### 7. Espace solo

- `oh team init --solo [--project <p>]` crée `~/.oh/teams/<id>/` : dépôt git sans remote, membre unique de rôle `lead`, équipe déclarée `solo = true` dans `hub.toml`.
- Un espace solo n'est **jamais l'équipe active** : MCP `team`, claims, board et événements restent coupés. Il ne porte que les workflows et les briques.
- `oh team promote --remote <url>` ajoute le remote et pousse l'historique. L'id, les projets rattachés et les workflows publiés ne changent pas. En cas d'échec, l'espace reste solo.
- La TUI propose l'espace solo au premier lancement et à l'ajout d'un projet sans équipe, ainsi que l'action « Passer en équipe ».

### 8. Migration de l'ancienne configuration (v38)

La migration est automatique au démarrage d'une commande, idempotente et retentée en cas d'échec :

- `[workflow]` d'une équipe et la configuration de chaque projet sont traduits en patch de `feature`, archivés bruts, puis publiés comme brouillon. Un espace solo est créé pour un projet sans équipe. `Enforced` devient `enforce: ["*"]`.
- La migration v38 neutralise `projects.workflow_config` sans perte (copie dans `workflow_config_legacy`).
- Les surcharges de `hub.toml` sont archivées dans `~/.oh/migrated/`, mais **pas chargées**.
- Les anciens champs (`WorkflowTeamConfig`, `Project.WorkflowConfig`, `config.Workflow`) et la vue Workflow sont supprimés.

### 9. Commandes

- CLI : `oh workflow new|edit|diff|publish|history|restore|archive`.
- TUI : catalogue modifiable (brouillons, intégrité), éditeur en 5 sections (Général, Graphe, Entrées & prompt, Ressources, Aperçu du paquet) avec origine et verrous de chaque valeur, écrans Publication et Historique.

## Conséquences

### Positives

- Une équipe partage ses workflows sans merge request ni forge particulière, avec un historique complet et une restauration.
- Ce qui est chargé est vérifié : un fichier modifié à la main est ignoré et signalé.
- Les publications concurrentes sont revalidées, et le travail hors ligne est mis en file.
- Un projet sans équipe utilise exactement le même modèle (espace solo), promouvable sans perte.
- Les équipes peuvent livrer leurs propres agents et skills sans modifier le hub.

### Négatives / Compromis

- Tout membre peut publier ; une gouvernance plus fine (lead seul, approbations) est au backlog (BL-2).
- Pas de protection côté serveur du team-state (BL-5) : quelqu'un qui peut pousser peut écrire un fichier à la main. Le fichier sera ignoré, mais rien ne l'empêche.
- Chaque opération coûte des commandes git (~2 s chacune observées sur la machine de build).
- Les brouillons sont visibles par tous les membres.
- L'impact ne compare pas les permissions propres d'un agent d'une version à l'autre (BL-16), et `enforce:` reste au premier niveau.
- Pas d'invitation des membres depuis oh (BL-15) : `oh team promote` affiche l'URL à transmettre.
- `oh workflow new --file` refuse un document qui référence un gabarit à créer (anomalie Q3-7).
- Les surcharges de `hub.toml` migrées ne sont plus appliquées.

## Alternatives considérées

| Alternative | Rejetée car |
|---|---|
| Gouvernance par merge request sur le team-state | Lourde pour une équipe de 3 à 5 personnes et dépendante de la forge ; la validation par oh, le lock et l'historique suffisent (D7). |
| Workflows d'équipe dans `hub.toml` ou en SQLite | Ni partagés ni versionnés ; c'était la situation de départ. |
| Un dépôt dédié aux workflows | Un dépôt de plus à cloner et à configurer pour chaque membre. |
| Rebaser le commit de publication sur la publication concurrente | Conflits garantis sur `workflows.lock` ; refaire le cycle complet revalide sur l'état réel. |
| Laisser une brique d'équipe masquer celle du hub sans le dire | Comportement invisible ; `extends: hub:<id>` rend le remplacement explicite. |
| Garder les workflows des projets sans équipe en base locale | Deux modèles à maintenir ; l'espace solo donne le même modèle et se promeut sans perte. |
