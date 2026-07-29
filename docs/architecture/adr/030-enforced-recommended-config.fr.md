# ADR-030 : Modele de Configuration Enforced vs Recommended

## Statut

Accepted

## Date

2026-07-29

## Contexte

La cascade de configuration actuelle pour les settings partages d'equipe utilise un modele plat "nil-means-inherit" : si une valeur hub est nil, la valeur team-state est utilisee en fallback. Ce modele a deux problemes :

1. **Pas d'enforcement** : un lead ne peut pas garantir que tous les membres utilisent un service MCP specifique ou un reglage tracker. N'importe quel membre peut overrider localement, cassant potentiellement les workflows d'equipe (ex : desactiver la sync Jira quand l'equipe s'appuie dessus pour la planification).

2. **Semantique floue** : le `config.toml` du team-state ne distingue pas "nous recommandons ceci" (overridable) de "c'est requis" (non-overridable). Le champ `write_recommended` sur les services MCP etait une tentative ad-hoc de semantique advisory, jamais generalisee.

3. **Pas de configuration models au niveau equipe** : la cascade models (Project -> Hub -> Agent Frontmatter) n'a pas d'input equipe. Les equipes ne peuvent pas standardiser quels modeles leurs membres utilisent.

## Decision

Introduire un modele d'enforcement a deux niveaux pour la configuration partagee d'equipe :

### Niveaux d'enforcement

Chaque setting du team-state peut etre :
- **Recommended** (defaut) : fournit une valeur par defaut que le hub ou le projet peut overrider. C'est le fallback quand aucune preference locale n'existe.
- **Enforced** : l'equipe impose cette valeur. Ni le hub ni le projet ne peuvent l'overrider. La TUI affiche un cadenas et desactive l'edition.

### Cascade de resolution (revisee)

```
1. Equipe ENFORCED ? → OUI : utiliser la valeur equipe (verrouillee)
                     → NON : continuer
2. Projet a un override explicite ? → OUI : utiliser la valeur projet
                                     → NON : continuer
3. Hub a une valeur ? → OUI : utiliser la valeur hub
                      → NON : continuer
4. Equipe RECOMMENDED ? → OUI : utiliser la recommandation equipe
                        → NON : utiliser le defaut systeme
```

### Schema TOML pour l'enforcement

Dans team-state `config.toml`, l'enforcement est declare via un champ booleen compagnon `_enforced` :

```toml
[mcp.jira]
enabled = true
enabled_enforced = true   # les membres DOIVENT avoir jira active

[mcp.gitlab]
enabled = true
enabled_enforced = false  # recommandation uniquement
url = "https://gitlab.company.com"
url_enforced = true       # l'URL ne peut pas etre overridee

[models]
default = "claude-sonnet-4-20250514"  # recommande, overridable
```

### Models au niveau equipe (nouveau)

Le `config.toml` du team-state gagne une section `[models]` avec `default`, `[models.families]`, et `[models.agents]` — tous en recommandations (overridables par hub et projet). Pas d'enforcement pour les models (recommandations uniquement).

### Contrat UX

- Champs enforced : affiches avec un cadenas (🔒), non-editables en TUI, toast "Impose par l'equipe X" lors d'une tentative d'edition
- Champs recommended sans override local : affiches avec annotation source `[equipe: recommande]`
- Info de resolution toujours visible : `[enforced: equipe]`, `[hub]`, `[projet]`, `[equipe: recommande]`

## Alternatives Rejetees

| Alternative | Raison du rejet |
|---|---|
| Niveau unique "priorite" (equipe gagne toujours) | Trop restrictif — les membres ont besoin de flexibilite locale pour la plupart des settings |
| Entier de priorite par champ (0-10) | Over-engineering — binaire enforced/recommended couvre tous les cas reels |
| Fichier `enforced.toml` separe | Divise la config inutilement ; le compagnon `_enforced` est co-localise et auto-documente |
| Enforcement comme policy (policies.toml) | Les policies sont pour les patterns de code, pas les valeurs de config ; melanger les deux complexifie les deux systemes |

## Consequences

### Positives
- Les leads peuvent garantir la config critique (tracker sync, services MCP specifiques) sans audit manuel
- Les membres gardent la flexibilite sur les settings non-critiques (models, permissions write)
- La source de resolution est toujours visible en TUI — plus de "pourquoi c'est active ?"
- Les models au niveau equipe reduisent la friction d'onboarding (nouveaux membres ont des defauts sains)

### Negatives / Compromis
- Les champs `_enforced` ajoutent de la verbosite au `config.toml` (acceptable — utilises uniquement pour les settings critiques)
- La logique de resolution devient plus complexe (cascade 4 etapes vs. 2)
- Les admins d'equipe doivent communiquer clairement quels settings sont enforced et pourquoi (gouvernance)
- Le champ `write_recommended` devient redondant (supersede par ce modele generique) — deprecier progressivement
