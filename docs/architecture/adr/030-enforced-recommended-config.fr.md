# ADR-030 : Modèle de Configuration Enforced vs Recommended

## Statut

Accepted

## Date

2026-07-29

## Contexte

La cascade de configuration actuelle pour les settings partagés d'équipe utilise un modèle plat "nil-means-inherit" : si une valeur hub est nil, la valeur team-state est utilisée en fallback. Ce modèle a deux problèmes :

1. **Pas d'enforcement** : un lead ne peut pas garantir que tous les membres utilisent un service MCP spécifique ou un réglage tracker. N'importe quel membre peut overrider localement, cassant potentiellement les workflows d'équipe (ex : désactiver la sync Jira quand l'équipe s'appuie dessus pour la planification).

2. **Sémantique floue** : le `config.toml` du team-state ne distingue pas "nous recommandons ceci" (overridable) de "c'est requis" (non-overridable). Le champ `write_recommended` sur les services MCP était une tentative ad-hoc de sémantique advisory, jamais généralisée.

3. **Pas de configuration models au niveau équipe** : la cascade models (Project -> Hub -> Agent Frontmatter) n'a pas d'input équipe. Les équipes ne peuvent pas standardiser quels modèles leurs membres utilisent.

## Décision

Introduire un modèle d'enforcement à deux niveaux pour la configuration partagée d'équipe :

### Niveaux d'enforcement

Chaque setting du team-state peut être :
- **Recommended** (défaut) : fournit une valeur par défaut que le hub ou le projet peut overrider. C'est le fallback quand aucune préférence locale n'existe.
- **Enforced** : l'équipe impose cette valeur. Ni le hub ni le projet ne peuvent l'overrider. La TUI affiche un cadenas et désactive l'édition.

### Cascade de résolution (révisée)

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

### Schéma TOML pour l'enforcement

Dans team-state `config.toml`, l'enforcement est déclaré via un champ booléen compagnon `_enforced` :

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

### Models au niveau équipe (nouveau)

Le `config.toml` du team-state gagne une section `[models]` avec `default`, `[models.families]`, et `[models.agents]` — tous en recommandations (overridables par hub et projet). Pas d'enforcement pour les models (recommandations uniquement).

### Contrat UX

- Champs enforced : affichés avec un cadenas (🔒), non-editables en TUI, toast "Imposé par l'équipe X" lors d'une tentative d'édition
- Champs recommended sans override local : affichés avec annotation source `[equipe: recommande]`
- Info de résolution toujours visible : `[enforced: equipe]`, `[hub]`, `[projet]`, `[equipe: recommande]`

## Alternatives Rejetées

| Alternative | Raison du rejet |
|---|---|
| Niveau unique "priorité" (équipe gagne toujours) | Trop restrictif — les membres ont besoin de flexibilité locale pour la plupart des settings |
| Entier de priorité par champ (0-10) | Over-engineering — binaire enforced/recommended couvre tous les cas réels |
| Fichier `enforced.toml` séparé | Divise la config inutilement ; le compagnon `_enforced` est co-localise et auto-documente |
| Enforcement comme policy (policies.toml) | Les policies sont pour les patterns de code, pas les valeurs de config ; mélanger les deux complexifie les deux systèmes |

## Conséquences

### Positives
- Les leads peuvent garantir la config critique (tracker sync, services MCP spécifiques) sans audit manuel
- Les membres gardent la flexibilité sur les settings non-critiques (models, permissions write)
- La source de résolution est toujours visible en TUI — plus de "pourquoi c'est activé ?"
- Les models au niveau équipe réduisent la friction d'onboarding (nouveaux membres ont des defauts sains)

### Négatives / Compromis
- Les champs `_enforced` ajoutent de la verbosité au `config.toml` (acceptable — utilisés uniquement pour les settings critiques)
- La logique de résolution devient plus complexe (cascade 4 étapes vs. 2)
- Les admins d'équipe doivent communiquer clairement quels settings sont enforced et pourquoi (gouvernance)
- Le champ `write_recommended` devient redondant (remplacé par ce modèle générique) — deprecier progressivement
