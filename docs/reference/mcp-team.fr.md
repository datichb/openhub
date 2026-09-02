# Serveur MCP : team

## Vue d'ensemble

Le serveur MCP `team` expose les données de collaboration d'équipe aux agents IA via le protocole MCP (stdio JSON-RPC). Il lit depuis le clone local `~/.oh/team-state/` et fournit un accès en lecture seule à tous les agents, avec des capacités d'écriture limitées à certains agents spécifiques.

## Activation

Le serveur MCP team est déployé automatiquement lorsque les fonctionnalités d'équipe sont **activées pour le projet** en cours de déploiement. L'activation est résolue en deux couches :

1. **`.opencode/team.json`** (primaire) — écrit par `oh deploy` avec la configuration entièrement résolue pour ce projet. Présent quand team est activé ; absent quand désactivé.
2. **Section `[team]` de `hub.toml`** (fallback) — utilisé pour la rétrocompatibilité sur les projets pas encore redéployés après l'introduction de la configuration team par projet.

Aucun token n'est requis — le serveur lit depuis le clone local de team-state.

### Cascade de résolution

```
Project mode (stored in SQLite)
  ├── "inherit"  → use hub.toml [team] config as-is
  ├── "custom"   → use project-specific state_repo / member_id
  └── "disabled" → team.json not written, MCP server not injected
```

Voir [Guide de configuration Team — Configuration par projet](../guides/team-setup.en.md#4b-per-project-team-configuration) pour savoir comment configurer le mode par projet.

```json
// Injected into opencode.json by oh deploy (when team is enabled for the project)
{
  "mcpServers": {
    "team": {
      "command": "oh",
      "args": ["mcp", "serve", "team"]
    }
  }
}
```

Au runtime, le processus du serveur MCP lit `.opencode/team.json` depuis le **répertoire de travail courant** (la racine du projet). Si ce fichier est absent, il se rabat sur la lecture directe de `hub.toml`.

## Outils

### `team_members`

Liste tous les membres de l'équipe.

**Entrée :** `{}` (aucun paramètre)

**Sortie :** Tableau JSON des membres

```json
[
  {
    "ID": "benjamin",
    "DisplayName": "Benjamin",
    "GitLabUsername": "bdatiche",
    "MattermostUsername": "benjamin.datiche",
    "Role": "lead",
    "DefaultMode": "semi-auto"
  }
]
```

**Accès :** Tous les agents

---

### `team_claims`

Liste les revendications de tickets actives.

**Entrée :**
```json
{
  "project": "T-SRU"  // optional, empty = all projects
}
```

**Sortie :** Tableau JSON des revendications

```json
[
  {
    "TicketID": "SRU-142",
    "Project": "T-SRU",
    "ClaimedBy": "benjamin",
    "ClaimedAt": "2026-07-07T14:30:00Z",
    "Worktree": "feat/SRU-142-user-auth",
    "Status": "in_progress",
    "LastActivity": "2026-07-10T09:15:00Z",
    "Labels": ["agent-reviewed"],
    "ExternalIID": 142
  }
]
```

**Notes sur les champs :**

- `Status` — une des cinq valeurs :
  - `planned` — réservé mais pas encore commencé (colonne TODO)
  - `in_progress` — activement en cours de traitement (par défaut à la création de la revendication)
  - `review` — travail terminé, en attente de revue humaine
  - `blocked` — bloqué sur une dépendance externe
  - `done` — accepté et terminé (conservé jusqu'à expiration de `done_retention_days`)
- `Labels` — liste de tags ; valeurs reconnues : `agent-reviewed`, `needs-human-review`, `hub:done`
- `ExternalIID` — numéro d'issue sur le tracker externe (défini par la synchronisation du tracker ; omis si aucune synchronisation n'est configurée)

**Accès :** Tous les agents

---

### `team_wiki_list`

Liste les pages wiki disponibles.

**Entrée :** `{}` (aucun paramètre)

**Sortie :** Tableau JSON des noms de pages (sans l'extension `.md`)

```json
["decisions", "patterns", "onboarding"]
```

**Accès :** Tous les agents

---

### `team_wiki_read`

Lit une page wiki.

**Entrée :**
```json
{
  "page": "decisions"  // required
}
```

**Sortie :** Contenu Markdown de la page (texte brut)

**Accès :** Tous les agents

---

### `team_wiki_write`

Propose une nouvelle entrée au wiki (crée une proposition en attente).

**Entrée :**
```json
{
  "page": "decisions",           // required
  "content": "## New Decision\n\nContent...",  // required, max 200 lines
  "confidence": "CONFIRMED",     // required: CONFIRMED | INFERRED | UNCERTAIN
  "project": "T-SRU"            // required: originating project
}
```

**Sortie :** Message de confirmation

**Accès :** `documentarian` uniquement

**Comportement :**
1. Valide les contraintes de format et de taille
2. Crée un fichier de proposition dans `wiki/.pending/`
3. Émet un événement `wiki.proposal`
4. Envoie une notification Mattermost
5. Retourne une confirmation — ne modifie PAS directement les pages wiki

---

### `team_events`

Liste les événements récents d'activité de l'équipe.

**Entrée :**
```json
{
  "project": "T-SRU",  // optional
  "limit": 20          // optional, default: 20
}
```

**Sortie :** Tableau JSON d'événements (les plus récents en premier)

```json
[
  {
    "ts": "2026-07-07T15:45:00Z",
    "actor": "benjamin",
    "event": "session.complete",
    "project": "T-SRU",
    "ticket": "SRU-142",
    "data": {"duration_min": 75}
  }
]
```

**Accès :** Tous les agents

---

### `team_notify`

Envoie une notification au canal d'équipe (supporte Mattermost, Slack, Discord, Teams).

**Entrée :**
```json
{
  "message": "Implementation of SRU-142 is ready for review"  // required
}
```

**Sortie :** Confirmation

**Accès :** `orchestrator-dev`, `reviewer`, `auditor`

## Configuration des permissions par agent

Dans `opencode.json`, les permissions sont définies par agent :

```json
{
  "agent": {
    "documentarian": {
      "permission": {
        "team_members": "allow",
        "team_claims": "allow",
        "team_wiki_list": "allow",
        "team_wiki_read": "allow",
        "team_wiki_write": "allow",
        "team_events": "allow",
        "team_notify": "deny"
      }
    },
    "orchestrator-dev": {
      "permission": {
        "team_members": "allow",
        "team_claims": "allow",
        "team_wiki_list": "allow",
        "team_wiki_read": "allow",
        "team_wiki_write": "deny",
        "team_events": "allow",
        "team_notify": "allow"
      }
    }
  }
}
```

## Types d'événements

| Type | Déclencheur | Émission auto | Notes |
|------|-------------|---------------|-------|
| `session.complete` | Fin de session | Oui (CLI) | |
| `review.ready` | Le reviewer termine | Oui (CLI) | Le label `agent-reviewed` est automatiquement appliqué à la revendication |
| `audit.finding` | L'auditeur trouve des problèmes | Oui (CLI) | |
| `claim.taken` | `oh claim` | Oui (CLI) | Émis quand un ticket est revendiqué ; `data.ticket` contient l'ID du ticket |
| `claim.conflict` | Revendication sur un ticket déjà pris | Oui (CLI) | |
| `claim.transferred` | `oh claim transfer` | Oui (CLI) | Émis quand une revendication est transférée à un autre membre ; `data.to` contient l'ID du nouveau propriétaire |
| `claim.released` | `oh release` | Oui (CLI) | Émis quand une revendication est libérée |
| `wiki.proposal` | `team_wiki_write` | Oui (MCP) | |
| `wiki.accepted` | `oh team wiki review` | Oui (CLI) | |
| `wiki.rejected` | `oh team wiki review` | Oui (CLI) | |

---

### `team_policies`

Récupère les politiques d'équipe actives (fusion global + surcharges projet).

**Entrée :**
```json
{
  "project": "T-SRU"  // optional, empty = global policies only
}
```

**Sortie :** Tableau JSON des politiques (fusionnées avec les surcharges projet si spécifié)

```json
[
  {
    "Name": "branch_naming",
    "Type": "regex",
    "Rule": "^(feat|fix|hotfix|chore|refactor)/[a-z0-9-]+",
    "Enforcement": "refuse",
    "Message": "Branch must follow pattern: feat/xxx, fix/xxx, etc."
  },
  {
    "Name": "custom_no_console_log",
    "Type": "forbidden_pattern",
    "Patterns": ["console.log", "console.warn"],
    "Scope": "diff_only",
    "Enforcement": "warn",
    "Message": "Remove console.log before commit"
  }
]
```

**Accès :** Tous les agents

**Comportement :**
1. Lit `policies.toml` depuis la racine de team-state
2. Si `project` est spécifié, fusionne avec `projects/<project>/policies-override.toml`
3. Les surcharges ne peuvent que rendre l'application plus stricte (warn → refuse), jamais plus permissive
4. Retourne toutes les politiques actives pour que l'agent les applique (voir skill `team-policies-enforcement`)

---

### `team_takeover_brief`

Lit le brief de reprise pour un ticket (contexte du précédent propriétaire après un transfert).

**Entrée :**
```json
{
  "project": "T-SRU",    // required
  "ticket_id": "bd-42"   // required
}
```

**Sortie :** Contenu Markdown du brief (meilleure version disponible)

Ordre de priorité :
1. `.enriched.md` (version enrichie par IA) si disponible
2. `.md` (résumé généré par template) si disponible
3. `.toml` (données structurées brutes) en dernier recours

**Accès :** Tous les agents

**Comportement :**
1. Cherche dans `projects/<project>/takeover-briefs/` les fichiers correspondant à `<ticket_id>_*`
2. Retourne le brief le plus récent dans le meilleur format disponible
3. Retourne "No takeover brief found" si aucun n'existe

---

### `team_patterns_list`

Liste les patterns de décomposition disponibles dans la bibliothèque de patterns de l'équipe.

**Entrée :**
```json
{
  "tags": ["backend", "api"]  // optional, filters by tag matching
}
```

**Sortie :** Tableau JSON des métadonnées de patterns

```json
[
  {
    "Name": "crud-api",
    "Tags": ["backend", "api", "crud"],
    "Complexity": "medium",
    "Source": "manual",
    "Project": "T-SRU",
    "Validated": true,
    "CreatedAt": "2026-07-10"
  }
]
```

**Accès :** Tous les agents

**Comportement :**
1. Lit `patterns/index.toml` depuis team-state
2. Si `tags` est fourni, retourne les patterns correspondant à >= 2 tags
3. Retourne un message vide si aucun pattern n'existe

---

### `team_patterns_read`

Lit le contenu complet d'un pattern de décomposition.

**Entrée :**
```json
{
  "name": "crud-api"  // required, without .md extension
}
```

**Sortie :** Contenu Markdown complet du fichier de pattern

**Accès :** Tous les agents

---

### `team_patterns_propose`

Propose un nouveau pattern à la bibliothèque (depuis planner ou pathfinder).

**Entrée :**
```json
{
  "name": "integration-externe",       // required
  "tags": ["backend", "integration"],   // required
  "complexity": "high",                 // required: low | medium | high
  "project": "T-SRU",                  // optional
  "content": "# Pattern content..."    // required: full Markdown
}
```

**Sortie :** Message de confirmation

**Accès :** `planner`, `pathfinder`

**Comportement :**
1. Crée le pattern avec `validated = false`
2. Ajoute à `patterns/index.toml`
3. Crée `patterns/<name>.md`
4. Attend la validation humaine via `oh patterns validate <name>`

---

## Intégration de la synchronisation tracker

Lorsque la synchronisation tracker est configurée (GitLab ou Jira), la sortie de `team_claims` peut inclure des labels supplémentaires mirrorés depuis le tracker externe. Par exemple, un label GitLab `hub:done` ou une transition de statut Jira peut être reflétée dans le tableau `Labels` de la revendication correspondante. Le champ `ExternalIID` est rempli automatiquement par le processus de synchronisation et peut être utilisé pour corréler les revendications avec les issues sur le tracker externe.
