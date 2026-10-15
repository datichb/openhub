> [Read in English](cli-bricks.en.md)

# Référence CLI — Briques

`oh brick` gère le **catalogue de briques d'équipe** : les agents et les skills de `catalog/` du team-state (espace solo compris), utilisés par les workflows de l'équipe et de ses projets comme les briques du hub. Même cycle que les workflows d'équipe : brouillon validé à l'enregistrement, publication gouvernée, version scellée dans `catalog.lock`, historique. Décision : [ADR-054](../architecture/adr/054-team-brick-catalogue.fr.md). Aucun import de brique externe.

Toutes les commandes acceptent `-p, --project <projet>` (team-state de l'équipe du projet ; défaut : projet du dossier courant, sinon l'équipe active ou l'unique espace solo) et `--team <id>` (une équipe ou un espace solo, sans projet). Ils ne sont pas répétés dans les tableaux.

**Désigner une brique.** Un agent par son identifiant (`hotfixer`), une skill par son chemin sous `skills/`, sans `.md` (`team/hotfix-protocol`). Un identifiant qui désigne à la fois un agent et une skill est refusé : préfixez-le (`agent:<id>`, `skill:<chemin>`). Les noms sont contrôlés : segments en kebab-case (quatre au plus pour une skill), sans `..`, rien sous `templates/`.

## oh brick list

```
oh brick list [--agents|--skills] [--origin] [--json]
```

Liste les briques du contexte : celles du hub et celles du catalogue d'équipe, avec leur type, leur **origine** (`hub`, `équipe`, ou `équipe (remplace hub:<id>)` pour une brique qui remplace une brique du hub), leur version publiée et leur état (`✓`, `!` avertissements, `✗` erreurs). Suivent vos **brouillons** (`✎`), les briques d'un catalogue jamais scellé et les fichiers ignorés par le contrôle d'intégrité.

| Flag | Type | Description |
|------|------|-------------|
| `--agents` | bool | Agents seulement |
| `--skills` | bool | Skills seulement (exclut `--agents`) |
| `--origin` | bool | Ajoute la colonne du fichier d'origine (`catalog/…` du team-state, ou `hub`) |
| `--json` | bool | Sortie JSON : `bricks` (avec `origin`, `team_id`, `source`, `shadows`, `version`, `errors`, `warnings`), `drafts`, `sealed`, `unsealed`, `integrity`, `problems` |

## oh brick show

```
oh brick show <id> [--raw] [--json]
```

Affiche une brique (votre brouillon en priorité, sinon la version publiée, sinon celle du hub) : origine, fichier, version, famille, mode et skills d'un agent, dépendances et agents d'une skill, workflows qui la livrent, coût estimé en tokens, problèmes trouvés.

| Flag | Type | Description |
|------|------|-------------|
| `--raw` | bool | Affiche le fichier tel quel |
| `--json` | bool | Sortie JSON (`ref`, `origin`, `path`, `draft`, `entry`, `diagnostics`) |

## oh brick new

```
oh brick new <agent|skill> <id> [--family <famille>] [--extends] [--file <fichier>|-] [--no-edit]
```

Crée votre brouillon à partir d'un **gabarit valide** (agent `subagent` en lecture seule avec `shared/universal-guardrails` ; skill avec une description « Utiliser quand … »), ouvre `$VISUAL`/`$EDITOR` (sinon `vi`), puis valide. En cas d'erreur, l'éditeur peut être rouvert ; sinon le fichier modifié est conservé et rien n'est enregistré.

Un identifiant d'agent du hub, ou un nom de skill du hub, est refusé : pour **remplacer** une brique du hub, dites-le avec `--extends`. Le brouillon part alors de la brique du hub et commence par `extends: hub:<id>`.

| Flag | Type | Défaut | Description |
|------|------|--------|-------------|
| `--family` | string | `team` | Dossier de l'agent dans `catalog/agents/` |
| `--extends` | bool | `false` | Part de la brique du hub de même identifiant (agent) ou de même nom (skill) et la remplace |
| `--file` | string | | Lire la brique depuis ce fichier (`-` : entrée standard), sans éditeur |
| `--no-edit` | bool | `false` | Enregistrer le gabarit sans ouvrir l'éditeur |

## oh brick edit

```
oh brick edit <id> [--file <fichier>|-] [--family <famille>]
```

Ouvre votre brouillon, sinon la version publiée de la brique d'équipe, et enregistre un brouillon validé. Les briques du hub sont en lecture seule (`oh brick new … --extends`). `--family` déplace un agent dans un autre dossier.

## Validation

Un brouillon est validé contre le hub fusionné avec le catalogue d'équipe publié, avec les règles du hub :

| Brique | Règles (erreurs) |
|--------|------------------|
| skill | celles d'[`oh skill check`](cli-tools.fr.md#oh-skill-check) : frontmatter, `name:` égal au nom du fichier, clés connues, `requires:` et annexes présents, pas de cycle, `when:`, fraîcheur des skills de stack ; **description « Utiliser … » de 200 caractères au plus** |
| agent | frontmatter lisible (`project_instructions` compris), `id:` égal au nom du fichier, `description:`, `mode:` `primary` ou `subagent`, clés connues (`id`, `label`, `description`, `mode`, `model`, `permission_base`, `permission`, `skills`, `native_skills`, `verify`, `stack_skills`, `skills_from_callers`, `project_instructions`, `extends`), skills présentes, `permission_base` existant |
| les deux | identifiant du hub sans `extends: hub:<id>` (`brick_collision`), `extends` qui ne désigne aucune brique du hub (`brick_bad_extends`), nom de skill déjà pris dans l'équipe (`brick_duplicate`) |

Le remplacement d'une brique du hub donne l'avertissement `brick_shadows`.

## oh brick publish

```
oh brick publish <id> -m <message> [-y]
oh brick publish --adopt -m <message> [-y]
```

Publie votre brouillon en version suivante. La commande affiche la version et les constats, puis demande confirmation si la brique **remplace une brique du hub** (sans terminal ou avec `-y` : sans question). La publication :

1. vérifie la gouvernance de l'équipe (`[governance] publish`, comme [`oh workflow publish`](cli-workflows.fr.md#oh-workflow-publish)) ;
2. revalide la brique sur le team-state à jour (pull), reconstruite si un autre membre a poussé avant ;
3. copie la version précédente dans `catalog/history/<agents|skills>/<id>/<v>.md` (et `<v>.lock.toml`) ;
4. écrit la brique, la scelle dans `catalog.lock`, retire votre brouillon, écrit l'événement `brick.published` ;
5. commite et pousse.

Hors ligne, rien n'est publié et le brouillon reste : relancez une fois en ligne.

**Catalogue commité à la main.** Sans `catalog.lock`, les briques existantes restent chargées mais ne sont pas scellées ; la publication d'une autre brique est refusée tant qu'elles ne le sont pas. `--adopt` (sans identifiant) les scelle toutes telles quelles, en une transaction (événement `brick.adopted`), si aucune n'est refusée ni en erreur.

| Flag | Type | Description |
|------|------|-------------|
| `-m, --message` | string | Message de publication (obligatoire) |
| `--adopt` | bool | Scelle tel quel un catalogue commité à la main |
| `-y, --yes` | bool | Sans confirmation |

## oh brick history

```
oh brick history <id> [--json]
```

Versions publiées d'une brique d'équipe, de la plus récente à la plus ancienne : version (`(publiée)` pour la version en service), date, auteur, message.

## Intégrité

Quand `catalog.lock` existe, une brique n'est chargée (validation des workflows, paquets de session) que si son fichier est à l'emplacement enregistré, avec la même empreinte. Sinon elle est ignorée avec un avertissement :

| Code | Cas |
|------|-----|
| `brick_unlocked` | fichier absent de `catalog.lock` (commité sans oh) ou déplacé |
| `brick_hash_mismatch` | fichier modifié à la main depuis sa publication |
| `brick_lock_orphan` | entrée de `catalog.lock` sans fichier |
| `brick_lock_invalid` | `catalog.lock` illisible : toutes les briques d'équipe sont ignorées |
| `brick_unsafe_path` | lien symbolique ou fichier non régulier dans `catalog/` (toujours, lock ou non) |

Ces avertissements apparaissent dans `oh brick list`, `oh workflow list`, le catalogue des workflows de la TUI et `oh doctor`. Pour corriger : republier la brique avec `oh brick edit` puis `oh brick publish`, ou restaurer le fichier (`git checkout`). Les annexes (`catalog/skills/templates/`) ne sont pas scellées.

## oh doctor

Le contrôle « Briques d'équipe (<équipe>) » apparaît pour chaque team-state qui a des briques : nombre de briques en service, briques **invalides** (erreurs des règles de validation du catalogue publié, descriptions de skill en avertissement comme dans `oh skill check`), fichiers ignorés par le contrôle d'intégrité, catalogue jamais scellé (avertissement).
