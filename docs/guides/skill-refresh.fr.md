> [Read in English](skill-refresh.en.md)

# Fraîcheur des skills de stack

Les skills de stack (`skills/developer/stacks/`, une par langage, framework ou outil : React, Go, Terraform…) disent aux agents comment écrire du code pour une version donnée d'une technologie. Quand une version majeure sort, une skill qui n'a pas suivi donne des conseils dépassés : API retirées, options par défaut changées, pratiques remplacées. Ce guide explique comment oh suit l'âge de chaque skill et comment la mettre à jour.

En bref :

1. chaque skill de stack déclare **les versions qu'elle couvre** et **la date de sa dernière revue** ;
2. `oh skill check --stale` liste celles dont la revue date de plus de 90 jours ;
3. le workflow `skill-refresh` fait la veille, propose la mise à jour, attend votre validation, puis écrit et commite ;
4. une tâche mensuelle (CI ou cron) vous prévient quand des skills sont à revoir.

## Les métadonnées de fraîcheur

Deux clés du frontmatter, lues par oh et **jamais livrées aux agents** (elles ne coûtent aucun token dans les prompts) :

```yaml
---
name: dev-standards-react
description: "Utiliser quand le projet utilise React (…) et que le ticket touche ce code."
versions:
  react: "19.3"
  react-compiler: "1.0"
reviewed: 2026-10-12
---
```

| Clé | Format | Rôle |
|---|---|---|
| `versions` | technologie en minuscules (`react`, `spring-boot`, `external-secrets-api`) → version entre guillemets | Versions que décrit le contenu. `"12+"` quand le contenu ne suppose qu'un minimum ; `"5.2 LTS / 6.1"` quand il couvre deux lignes de versions. |
| `reviewed` | date `AAAA-MM-JJ` | Dernière revue du contenu face aux versions publiées. |

Règles :

- toute skill de stack porte les deux clés : sinon `oh skill check` signale `missing_freshness` (erreur) ; une valeur mal formée (clé en majuscules, version vide, date illisible ou dans le futur) est un `invalid_freshness` ;
- chaque version de `versions:` figure aussi sur la ligne « > Versions de référence (…) : … » de la skill, que lisent les agents (vérifié par les tests du hub) ;
- `reviewed:` n'avance que si la skill a vraiment été comparée aux versions publiées. Une skill relue sans source ne change pas de date ;
- une autre skill (hors `developer/stacks/`) peut porter ces clés : elle est alors suivie de la même façon.

## Voir les skills à revoir

```
oh skill check --stale --hub .
```

À lancer à la racine d'une copie de travail du hub (`--hub .`) ; sans `--hub`, oh vérifie le hub installé (`~/.oh/hub`), qui ne bouge qu'avec une mise à jour d'oh.

```
Skills de stack à revoir (41 vérifiées, revue tous les 90 jours au plus, aujourd'hui 2026-10-13) :

  ! developer/stacks/dev-standards-cypress : revue le 2026-04-30 (il y a 166 jours), couvre cypress 12+
  ! developer/stacks/dev-standards-kubernetes : revue le 2026-04-30 (il y a 166 jours), couvre kubernetes 1.21+

  2 skill(s) à revoir : oh run skill-refresh (docs/guides/skill-refresh.fr.md)
```

| Option | Effet |
|---|---|
| `--max-age 180d` | Autre âge maximal : jours (`90`, `90d`) ou semaines (`12w`) |
| `--strict` | Code de sortie 1 s'il y a une skill à revoir (CI, cron) |
| `--json` | Rapport en JSON (`today`, `max_age_days`, `checked`, `skills`) |

**Pourquoi 90 jours ?** La plupart des stacks du hub publient une version mineure tous les un à trois mois (React, Next.js, Vitest, Playwright…), Kubernetes tous les quatre mois, Go et TypeScript tous les six mois. Une revue par trimestre attrape chaque cycle de version sans transformer la veille en travail permanent. Avec une vérification mensuelle, environ un tiers des skills arrive à échéance chaque mois, si les revues sont étalées. 180 jours laisserait passer une version entière de Kubernetes ou de Go.

La commande ne fait aucun appel réseau : elle compare des dates, pas les versions publiées. C'est la veille du workflow `skill-refresh` qui compare aux versions publiées.

## Mettre à jour : le workflow `skill-refresh`

Dans une copie de travail du hub enregistrée comme projet oh ([`oh project add`](../reference/cli-projects.fr.md#oh-project-add)) :

```
oh run skill-refresh                                   # les skills dont la revue dépasse 90 jours (5 au plus)
oh run skill-refresh -i skills=dev-standards-react     # une skill nommée, même récente
oh run skill-refresh -i max_age_days=60
```

Pour la recherche web, activez-la dans oh (`oh config websearch enable`) ; sans elle, donnez les notes de version vous-même dans l'entrée `notes` (le `documentarian` ne devine jamais une version et ne fait pas avancer `reviewed:` d'une skill qu'il n'a pu vérifier sur aucune source).

Déroulé :

1. **Sélection** : les skills nommées, sinon celles dont `reviewed:` dépasse `max_age_days` ou qui n'ont pas de métadonnées, les plus anciennes d'abord, 5 au plus par session. S'il n'y en a aucune, le `documentarian` le dit et la session s'arrête, sans checkpoint.
2. **Veille** : pour chaque technologie de `versions:`, la dernière version stable et ce qui a changé depuis la version couverte (nouveautés recommandées, API dépréciées ou retirées, configuration par défaut, guides de migration), à partir de vos notes puis des sources officielles (notes de version, changelog, documentation).
3. **Proposition**, sans rien écrire, présentée au checkpoint `cp-refresh` : pour chaque skill, versions avant → après, chaque changement (section, avant → après, raison, source) et la nouvelle date `reviewed`.
4. **Votre décision** : valider ; refuser avec une consigne de correction (le `documentarian` reprend la proposition et repasse par `cp-refresh`) ; ou refuser en bloc (« rien maintenant ») : rien n'est écrit.
5. **Après la validation** : il applique la proposition (contenu, ligne « Versions de référence », `versions:`, `reviewed:`), puis commite les skills seules (`docs(skills): refresh … stack skill(s) (skill-refresh)`). Le push vous revient.

Le contenu ajouté respecte la règle des skills du hub : **en français, inspiré des sources et jamais recopié** (exemples de code réécrits, aucun paragraphe repris tel quel), dans la structure et le ton de la skill, en remplaçant ce qui est périmé plutôt qu'en ajoutant. Une version en prérelease (bêta, release candidate) n'est jamais la référence.

Après le commit, vérifiez le hub avant de pousser :

```
oh skill check --hub .
cd cli && make sync-content && go test ./internal/bundle -run 'Skill|Freshness|Budget'
```

Le test du budget (`budget.golden`) montre si une skill de stack a grossi ; une hausse se justifie dans le message de commit ou la MR.

**Pourquoi le `documentarian` ?** Il rédige déjà la documentation, a la recherche web (`websearch`, `webfetch`), écrit les fichiers et commite, et charge `skill-authoring-protocol` (écrire une skill du hub) et `websearch-usage` (sources, sécurité, restitution). Aucun nouvel agent n'est nécessaire.

## Tâche mensuelle

La mise à jour reste une session supervisée : le checkpoint `cp-refresh` attend une personne dans tous les modes. La tâche planifiée se contente donc de **prévenir**.

### En CI (dépôt du hub)

Le workflow GitHub Actions [`.github/workflows/skill-freshness.yml`](../../.github/workflows/skill-freshness.yml) tourne le 1er de chaque mois (et à la demande, `workflow_dispatch`) : il construit oh, écrit la liste des skills à revoir dans le résumé du job et échoue s'il y en a une (`--strict`). L'échec d'un job planifié est notifié par GitHub. Sur un autre système de CI, le même principe tient en deux commandes :

```
cd cli && make embed-sync && go build -o bin/oh .
OH_HOME=$(mktemp -d) ./bin/oh skill check --stale --hub .. --strict
```

### En local (cron)

```
# le 1er du mois à 9 h : liste des skills à revoir dans ~/.oh/skill-stale.txt
0 9 1 * * cd ~/workspace/openhub && oh skill check --stale --hub . > ~/.oh/skill-stale.txt 2>&1
```

Puis, quand la liste n'est pas vide : `oh run skill-refresh` dans la copie de travail du hub.

## Limites

- `oh skill check --stale` ne compare pas aux dernières versions publiées (aucun appel réseau) : il mesure l'âge des revues. La comparaison est faite par la veille de `skill-refresh`.
- `reviewed:` dit quand une skill a été revue, pas qu'elle est exacte : la validation au checkpoint reste la garantie.

Voir aussi : [`oh skill check`](../reference/cli-tools.fr.md#oh-skill-check) · [workflow `skill-refresh`](../reference/workflows.fr.md#skill-refresh) · [Skills de stack](../architecture/skills.fr.md#skills-de-stack--developerstacks) · [Écrire des skills](authoring-skills.fr.md)
