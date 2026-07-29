# ADR-029 : Support Multi-Equipe

## Statut

Accepted

## Date

2026-07-29

## Contexte

La configuration hub actuelle suppose une equipe unique par hub (section `[team]` dans `hub.toml`). Les projets peuvent opter pour un repo team-state "custom" via `ProjectTeamConfig.Mode = "custom"`, mais c'est un mecanisme d'echappement plutot qu'une fonctionnalite multi-equipe de premiere classe :

- Pas de concept d'identite d'equipe (pas d'ID ni de nom) — une equipe est identifiee uniquement par son URL Git remote
- Le hub ne peut referencer qu'une seule equipe par defaut ; basculer entre equipes necessite une edition manuelle de la config
- Les utilisateurs travaillant avec plusieurs equipes (consulting, orgs multi-produit) doivent jongler avec des overrides par projet sans visibilite sur leurs appartenances
- La TUI n'a aucune vue pour gerer l'appartenance aux equipes

De plus, la struct `ProjectTeamConfig` melange la selection d'equipe (`Mode: "inherit" | "custom" | "disabled"`) avec la configuration de l'equipe (`StateRepo`, `StatePath`, `MemberID`), rendant le modele difficile a etendre.

## Decision

Introduire un support multi-equipe de premiere classe :

1. **La config hub evolue** de `[team]` (objet unique) vers `[[teams]]` (tableau TOML). Chaque entree a un `id` (identifiant court local), `name` (affichage), `state_repo`, `state_path`, `member_id`, et `enabled`.

2. **Les projets referencent les equipes par ID** : `Project.TeamID *string` remplace `ProjectTeamConfig.Mode`. Un `TeamID` nil signifie "projet solo" (pas d'equipe). Une valeur non-nil pointe vers un `teams[].id` dans la config hub.

3. **Migration automatique** : au premier chargement apres la mise a jour, la section legacy `[team]` est convertie en entree `[[teams]]` avec un ID derive automatiquement (dernier segment du path `state_repo`, debarrasse de `.git`). Les projets avec `Mode: "inherit"` recoivent `TeamID = <id-derive>`, les projets `Mode: "custom"` sont associes par URL, et `Mode: "disabled"` recoivent `TeamID = nil`.

4. **Nouvelle vue TUI** : `TeamsView` liste les equipes configurees, affiche le statut de sync, et permet ajouter/retirer/synchroniser.

5. **Nouvelles commandes CLI** : `oh teams list`, `oh teams add`, `oh teams remove`.

6. **Creation de projet** : demande toujours "quelle equipe ?" quand plusieurs equipes sont configurees (pas de concept d'equipe par defaut ; choix explicite requis).

## Alternatives Rejetees

| Alternative | Raison du rejet |
|---|---|
| Garder `[team]` unique avec escape hatch `"custom"` par projet | Ne scale pas ; pas de visibilite ; UX confuse |
| Multi-equipe implicite via `StateRepo` par projet uniquement | Pas de vue centrale des equipes ; config dupliquee ; pas d'identite |
| Champ `default_team` pour auto-assignation | Encourage le comportement implicite ; le choix explicite a la creation est plus clair |

## Consequences

### Positives
- Les utilisateurs voient toutes leurs equipes d'un coup et gerent l'appartenance centralement
- Les projets declarent leur affiliation explicitement — plus de devinette sur quel team-state s'applique
- L'identite d'equipe (ID + Nom) permet une UX plus riche (filtrage, affichage, notifications)
- Retro-compatible : les utilisateurs mono-equipe ne voient aucune difference apres migration

### Negatives / Compromis
- La complexite de hub.toml augmente legerement (array of tables vs. single table)
- La migration doit gerer les cas limites (URLs dupliquees entre ancien hub + projets custom)
- `ResolveTeamConfig` devient un lookup-par-ID (leger cout perf, negligeable)
- Les clones locaux team-state vivent maintenant sous `~/.oh/team-states/<id>/` (nouvelle convention)
