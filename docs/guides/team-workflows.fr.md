# Workflows d'équipe (team-state)

> v5, phase 2. Les workflows déclaratifs (`apiVersion: oh/v1`) d'une équipe et de ses projets sont rangés dans le dépôt team-state. Ce guide décrit leur emplacement, le contrôle d'intégrité et les verrous.

## Couches

Un workflow est résolu de la couche la plus générale à la plus spécifique :

| Couche | Emplacement |
|---|---|
| `hub` | workflows livrés avec oh (`~/.oh/hub/workflows/`) |
| `team` | `team-state/workflows/published/<id>.yaml` |
| `project` | `team-state/projects/<projet>/workflows/published/<id>.yaml` |

Un workflow d'une couche qui porte le même identifiant qu'un workflow d'une couche inférieure **doit l'étendre** (`extends`), sinon il est refusé. Les champs de sécurité ne peuvent que se durcir.

## Arborescence

```
team-state/
├── workflows/
│   ├── published/<id>.yaml            # versions publiées (chargées)
│   ├── drafts/<membre>/<id>.yaml      # brouillons (jamais chargés pour les autres)
│   ├── prompts/<id>.md.tmpl           # gabarits de prompt
│   └── history/<id>/<version>.yaml    # versions précédentes (+ <version>.prompt.md.tmpl)
├── projects/<projet>/workflows/…      # même structure pour un projet
├── catalog/{agents,skills}/           # briques d'équipe
└── workflows.lock                     # intégrité des fichiers publiés
```

Le chemin d'un gabarit (`prompt.template: prompts/<id>.md.tmpl`) est relatif au dossier `workflows/` de sa portée, quel que soit le sous-dossier du document (publié, brouillon, historique). Il ne peut pas en sortir.

## `workflows.lock` et intégrité

Chaque publication enregistre la version et l'empreinte du fichier publié (et de son gabarit de prompt) :

```toml
[team.ticket-hotfix]
version = 2
hash = "sha256:…"
prompt_hash = "sha256:…"
published_by = "alice"
published_at = 2026-10-06T10:00:00Z
message = "Checkpoint de revue obligatoire"

[projects.web.ticket]
…
```

Au chargement, oh **ignore avec un avertissement** :

- un fichier publié absent de `workflows.lock` (jamais publié depuis oh) ;
- un fichier ou un gabarit modifié à la main (empreinte différente) ;
- une entrée du lock dont le fichier a disparu ;
- tous les workflows de l'équipe si `workflows.lock` est illisible.

Ces avertissements apparaissent dans `oh workflow validate` et dans `oh doctor` (« Workflows d'équipe »). Pour corriger : republier le workflow depuis oh ou restaurer le fichier (`git checkout`).

## Vérifier les workflows d'équipe

```bash
oh workflow validate team:ticket-hotfix            # équipe active
oh workflow validate project:ticket --project web  # couche du projet + son équipe
oh workflow validate --all --project web           # hub, équipe et projet
```

## Verrous (`enforce`)

Un workflow peut verrouiller des champs pour les couches plus spécifiques :

```yaml
apiVersion: oh/v1
kind: Workflow
id: ticket
extends: hub:ticket
enforce: [checkpoints, modes]   # ou ["*"] pour tout le document
```

- Valeurs possibles : les champs de premier niveau du document (`checkpoints`, `modes`, `agents`, `models`, `runtime`…) ou `"*"`.
- Un document qui étend un workflow verrouillé et écrit un champ verrouillé est **refusé** (`enforced_field`). Avec `"*"`, seuls `id`, `version`, `extends` et `enforce` restent permis.
- Les verrous s'additionnent le long de la chaîne `extends` ; une couche plus spécifique ne peut pas les retirer.
- Les options de lancement (mode, environnement, entrées) restent choisies dans les limites du workflow.
