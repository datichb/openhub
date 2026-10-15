> [Read in English](054-team-brick-catalogue.en.md)

# ADR-054 — Catalogue des briques d'équipe : commande, gouvernance et intégrité

## Statut

Accepté

## Date

2026-10-15

## Contexte

Le catalogue d'équipe (`catalog/{agents,skills}/` du team-state, [ADR-040](040-workflows-team-state-governance.fr.md)) est le seul moyen d'ajouter des briques depuis la déconnexion du registre de skills communautaires ([ADR-051](051-community-skills-disconnection.fr.md)). Il se modifiait à la main, par un commit dans le dépôt team-state :

- **aucune validation avant publication** : une skill d'équipe trop longue ou sans déclencheur ne donnait qu'un avertissement d'`oh skill check`, un agent au frontmatter illisible était ignoré en silence ;
- **aucune intégrité** : contrairement aux workflows (`workflows.lock`), tout fichier commité dans `catalog/` entrait dans les paquets de session, y compris un fichier modifié à la main ou un lien vers un fichier hors du dépôt ;
- **aucune trace** : ni version, ni auteur, ni historique propre aux briques.

L'ADR-051 a listé une commande des briques comme première piste de refonte, « sur le modèle de `oh workflow` », avec la même gouvernance et le même contrôle d'intégrité.

## Décision

### 1. Commande `oh brick`

| Commande | Rôle |
|---|---|
| `oh brick list [--agents\|--skills] [--origin]` | Briques du hub et de l'équipe, avec leur origine (`hub`, `équipe`, « remplace hub:… »), leur version, leurs problèmes, les brouillons du membre et les fichiers ignorés |
| `oh brick show <id>` | Origine, fichier, version, usages (agents, workflows), coût, problèmes ; `--raw` affiche le fichier |
| `oh brick new <agent\|skill> <id>` | Brouillon à partir d'un gabarit valide ; `--extends` part de la brique du hub et la remplace explicitement |
| `oh brick edit <id>` | Éditeur, validation à l'enregistrement, réouverture en cas d'erreur |
| `oh brick publish <id> -m …` | Publication du brouillon en version suivante ; `--adopt` scelle un catalogue commité à la main |
| `oh brick history <id>` | Versions publiées (auteur, date, message) |

Un identifiant nu désigne un agent ou une skill ; s'il désigne les deux, il faut préciser `agent:<id>` ou `skill:<chemin>`. Le contexte est celui des commandes `oh workflow` : `--project` (le team-state de son équipe, espace solo compris) ou `--team`.

Le catalogue des briques est **un catalogue d'équipe** : il n'y a pas de couche « projet » pour les briques. Un projet utilise les briques de son équipe.

### 2. Arborescence

```
team-state/
├── catalog/
│   ├── agents/<famille>/<id>.md                 # publiés (chargés)
│   ├── skills/<chemin>.md                       # publiés (chargés) ; skills/templates/… : annexes
│   ├── drafts/<membre>/{agents,skills}/…        # brouillons (jamais chargés)
│   └── history/{agents,skills}/<id>/<v>.md      # versions précédentes (+ <v>.lock.toml)
└── catalog.lock                                 # intégrité des briques publiées
```

### 3. Validation

Une brique écrite avec `oh brick` (brouillon, publication) est validée contre le hub fusionné avec le catalogue d'équipe publié, avec les règles du hub :

- **skill** : règles d'`oh skill check` (frontmatter, `name:` égal au nom du fichier, clés connues, `requires:` et annexes présents, pas de cycle, `when:`, fraîcheur des skills de stack) ; la description commence par « Utiliser … » et fait **200 caractères au plus** (erreurs, et non avertissements comme pour une skill commitée à la main) ;
- **agent** : frontmatter lisible, `id:` égal au nom du fichier, `description:` présente, `mode:` `primary` ou `subagent`, clés connues, skills présentes dans le catalogue, `permission_base` existant.

Une brique invalide n'est pas enregistrée. Le catalogue publié est vérifié par `oh doctor` et `oh brick list` avec les règles d'`oh skill check` pour l'équipe (descriptions en avertissement).

### 4. Remplacement d'une brique du hub

Une brique d'équipe qui porte l'identifiant d'un agent du hub, ou le nom d'une skill du hub, est **refusée** sauf si elle le déclare (`extends: hub:<id>`, règle de l'ADR-040). `oh brick new … --extends` écrit cette déclaration. Le remplacement est **signalé** partout : avertissement à l'enregistrement, confirmation à la publication, colonne d'origine de `oh brick list`, champ `replaces` de l'événement.

### 5. Gouvernance

La publication suit `[governance] publish` du `config.toml` de l'équipe, comme les workflows (`any_member` : membre inscrit dans `members.toml` ; une valeur inconnue bloque). Elle est une transaction du team-state (pull → reconstruction → commit → push, reconstruite si un autre membre a poussé avant) : la brique est revalidée sur l'état à jour, la version précédente est copiée dans `catalog/history/`, la nouvelle est scellée dans `catalog.lock`, le brouillon est retiré, l'événement `brick.published` est écrit dans `projects/_team/events/`. Hors ligne, rien n'est publié et le brouillon reste (pas de file d'attente en partie 1).

### 6. Intégrité (`catalog.lock`)

```toml
[agents.hotfixer]
version = 2
path = "catalog/agents/team/hotfixer.md"
hash = "sha256:…"
published_by = "alice"
published_at = 2026-10-15T10:00:00Z
message = "…"

[skills."team/hotfix-protocol"]
…
```

Quand `catalog.lock` existe, une brique n'est chargée que si son fichier est à l'emplacement enregistré et a la même empreinte ; sinon elle est **ignorée** avec un avertissement (`brick_unlocked`, `brick_hash_mismatch`), comme les entrées sans fichier (`brick_lock_orphan`) et un lock illisible (`brick_lock_invalid`, toutes les briques ignorées). Les avertissements apparaissent dans `oh brick list`, `oh workflow list`, le catalogue des workflows de la TUI et `oh doctor`.

**Transition.** Sans `catalog.lock` (catalogue commité à la main avant cet ADR), les briques restent chargées, signalées « jamais scellées » par `oh doctor` et `oh brick list`. La première publication est refusée tant que d'autres briques ne sont pas scellées : `oh brick publish --adopt -m …` scelle tout le catalogue tel quel, en une transaction (événement `brick.adopted`), si aucune brique n'est refusée ni en erreur.

### 7. Chemins contrôlés

Mêmes exigences que l'ADR-051 pour le registre déconnecté :

- un identifiant est fait de segments en kebab-case (un pour un agent et sa famille, quatre au plus pour une skill), sans `..`, sans chemin absolu, jamais sous `templates/` ; le nom du membre est contrôlé ;
- tout chemin écrit (brouillon, publication, historique, lock) est construit à partir de ces noms, sous `catalog/` ; une entrée de `catalog.lock` ne peut désigner que le fichier publié de sa brique ;
- un **lien symbolique** (ou tout fichier non régulier) du catalogue est ignoré (`brick_unsafe_path`) : rien hors du dépôt n'entre dans un paquet ; un brouillon qui n'est pas un fichier régulier n'est pas publié.

### 8. Hors périmètre

**Aucun import de brique externe** (dépôt, URL, index) : la règle reste qu'aucune skill n'est reprise telle quelle. L'import vers le catalogue d'équipe (piste 2 de l'ADR-051 : version fixée par commit, empreinte, `https`, revue avant publication) est au backlog.

## Conséquences

### Positives

- Les briques d'équipe ont le cycle des workflows : brouillon validé, publication gouvernée, version, historique, intégrité.
- Un fichier modifié à la main ou un lien n'entre plus dans les paquets d'une équipe qui a scellé son catalogue.
- Le remplacement d'une brique du hub ne passe plus inaperçu.

### Négatives / Compromis

- Validation d'une brique plus lente qu'`oh skill check` : le hub est copié avec la brique dans un dossier temporaire (≈ 0,5 s).
- Pendant la transition, un catalogue sans `catalog.lock` reste chargé sans contrôle d'intégrité ; seul `oh doctor` le signale.
- Les annexes (`catalog/skills/templates/`) ne sont ni éditées ni scellées par `oh brick` ; elles restent commitées à la main (les liens y sont refusés).
- Pas d'archivage ni de restauration d'une brique, pas de file d'attente hors ligne, pas de vue d'édition dans la TUI (le catalogue des briques de la TUI reste en lecture, avec l'origine de chaque brique).
- La TUI n'affiche pas encore la version ni le remplacement d'une brique du hub.

## Alternatives considérées

| Alternative | Rejetée car |
|-------------|-------------|
| Réutiliser `workflows.lock` | Mélange deux formats d'entrées ; une brique a un chemin (famille, dossier de skill) qu'un workflow n'a pas |
| Refuser toute brique non scellée dès cet ADR | Casse sans prévenir les catalogues commités à la main ; l'adoption explicite les reprend en une commande |
| Sceller automatiquement les briques existantes à la première publication | Scelle du contenu sans revue ; `--adopt` est un acte explicite d'un membre autorisé |
| Couche « projet » pour les briques | Aucun besoin exprimé ; double la surface d'intégrité ; un projet suit les briques de son équipe |
| Avertissements seulement pour les descriptions de skill écrites avec `oh brick` | Une description longue coûte à chaque session ; la règle du hub s'applique à ce qui est écrit avec oh |

## Liens

- [ADR-040](040-workflows-team-state-governance.fr.md) : workflows dans le team-state, gouvernance, briques d'équipe
- [ADR-051](051-community-skills-disconnection.fr.md) : déconnexion du registre de skills communautaires (pistes pour la refonte)
- [Référence CLI — Briques](../../reference/cli-bricks.fr.md)
