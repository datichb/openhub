# ADR-029 : Support Multi-Équipe

## Statut

Accepted — **Évolué par [ADR-040](./040-workflows-team-state-governance.fr.md)**

## Date

2026-07-29

## Contexte

La configuration hub actuelle suppose une équipe unique par hub (section `[team]` dans `hub.toml`). Les projets peuvent opter pour un repo team-state "custom" via `ProjectTeamConfig.Mode = "custom"`, mais c'est un mécanisme d'échappement plutôt qu'une fonctionnalité multi-equipe de première classe :

- Pas de concept d'identité d'équipe (pas d'ID ni de nom) — une équipe est identifiée uniquement par son URL Git remote
- Le hub ne peut référencer qu'une seule équipe par défaut ; basculer entre équipes nécessite une édition manuelle de la config
- Les utilisateurs travaillant avec plusieurs équipes (consulting, orgs multi-produit) doivent jongler avec des overrides par projet sans visibilité sur leurs appartenances
- La TUI n'a aucune vue pour gérer l'appartenance aux équipes

De plus, la struct `ProjectTeamConfig` mélange la sélection d'équipe (`Mode: "inherit" | "custom" | "disabled"`) avec la configuration de l'équipe (`StateRepo`, `StatePath`, `MemberID`), rendant le modèle difficile à étendre.

## Décision

Introduire un support multi-equipe de première classe :

1. **La config hub évolue** de `[team]` (objet unique) vers `[[teams]]` (tableau TOML). Chaque entrée a un `id` (identifiant court local), `name` (affichage), `state_repo`, `state_path`, `member_id`, et `enabled`.

2. **Les projets référencent les équipes par ID** : `Project.TeamID *string` remplace `ProjectTeamConfig.Mode`. Un `TeamID` nil signifie "projet solo" (pas d'équipe). Une valeur non-nil pointe vers un `teams[].id` dans la config hub.

3. **Migration automatique** : au premier chargement après la mise à jour, la section legacy `[team]` est convertie en entrée `[[teams]]` avec un ID dérive automatiquement (dernier segment du path `state_repo`, debarrasse de `.git`). Les projets avec `Mode: "inherit"` reçoivent `TeamID = <id-derive>`, les projets `Mode: "custom"` sont associés par URL, et `Mode: "disabled"` reçoivent `TeamID = nil`.

4. **Nouvelle vue TUI** : `TeamsView` liste les équipes configurées, affiche le statut de sync, et permet ajouter/retirer/synchroniser.

5. **Nouvelles commandes CLI** : `oh teams list`, `oh teams add`, `oh teams remove`.

6. **Création de projet** : demande toujours "quelle équipe ?" quand plusieurs équipes sont configurées (pas de concept d'équipe par défaut ; choix explicite requis).

## Alternatives Rejetées

| Alternative | Raison du rejet |
|---|---|
| Garder `[team]` unique avec escape hatch `"custom"` par projet | Ne scale pas ; pas de visibilité ; UX confuse |
| Multi-equipe implicite via `StateRepo` par projet uniquement | Pas de vue centrale des équipes ; config dupliquée ; pas d'identité |
| Champ `default_team` pour auto-assignation | Encourage le comportement implicite ; le choix explicite à la création est plus clair |

## Conséquences

### Positives
- Les utilisateurs voient toutes leurs équipes d'un coup et gèrent l'appartenance centralement
- Les projets déclarent leur affiliation explicitement — plus de devinette sur quel team-state s'applique
- L'identité d'équipe (ID + Nom) permet une UX plus riche (filtrage, affichage, notifications)
- Retro-compatible : les utilisateurs mono-equipe ne voient aucune différence après migration

### Négatives / Compromis
- La complexité de hub.toml augmente légèrement (array of tables vs. single table)
- La migration doit gérer les cas limites (URLs dupliquées entre ancien hub + projets custom)
- `ResolveTeamConfig` devient un lookup-par-ID (léger coût perf, négligeable)
- Les clones locaux team-state vivent maintenant sous `~/.oh/team-states/<id>/` (nouvelle convention)
