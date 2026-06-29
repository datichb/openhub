# Référence CLI — commande `oc service`

> [Read in English](services.en.md)

Gestion des services et intégrations externes connectés via le protocole MCP (Model Context Protocol).

---

## Synopsis

```bash
oc service <sous-commande> [service] [options]
```

**Référence rapide** — aussi accessible via `oc help 8` ou `oc service --help` :

| Sous-commande | Description |
|---|---|
| `setup [nom]` | Configure un service interactivement (Figma, GitLab…) |
| `status [nom]` | Vérifie l'état d'un service (config, token, MCP) |
| `list` | Liste les services disponibles et leur état |
| `remove <nom>` | Supprime la configuration d'un service |
| `deploy <nom> [--project ID]` | Déploie le serveur MCP dans un projet |

**Aliases :** `oc figma` · `oc gitlab` · `oc gslides` → `oc service ... <nom>`

---

## `oc service setup`

Configure interactivement un service (credentials, validation, build MCP).

```bash
oc service setup [nom-du-service]
```

**Comportement :**
- Si `nom-du-service` est omis, affiche un menu de sélection parmi les services disponibles.
- Guide l'utilisateur étape par étape pour chaque credential requis par le service.
- Affiche l'aide contextuelle pour chaque credential (comment l'obtenir).
- Si une valeur existante est détectée, propose de la conserver.
- Valide le format des tokens si un `validation_pattern` est défini dans le catalogue.
- Effectue un appel API de validation (si un `validation.endpoint` est défini).
- Valide l'accessibilité de la team/ressource (si un `team_validation.endpoint` est défini) — **bloquant** : le setup ne peut pas se terminer si le Team ID est invalide.
- Sauvegarde les credentials dans `~/.config/opencode/config.json` (section `env`).
- Lance automatiquement le build du serveur MCP si nécessaire.

**Arguments :**

| Argument | Description |
|----------|-------------|
| `nom-du-service` | Identifiant du service (`figma`, `gitlab`, etc.). Optionnel — menu interactif si omis. |

**Exemples :**

```bash
# Mode interactif — menu de sélection
oc service setup

# Configurer Figma directement
oc service setup figma

# Configurer GitLab (alias)
oc gitlab setup

# Mode non-interactif (CI/CD)
FIGMA_PERSONAL_ACCESS_TOKEN=figd_xxx FIGMA_TEAM_ID=123456 \
  OC_NON_INTERACTIVE=1 oc service setup figma
```

---

## `oc service status`

Vérifie l'état d'un ou de tous les services (configuration, validité du token, build MCP).

```bash
oc service status [nom-du-service]
```

**Comportement :**
- Si `nom-du-service` est omis, affiche l'état de tous les services du catalogue.
- Pour chaque service, affiche :
  - Chaque credential : présent (valeur masquée pour les secrets) ou manquant.
  - Validation token : appel API rapide si un `validation.endpoint` est défini.
  - Validation Team ID : vérification d'accessibilité si un `team_validation.endpoint` est défini (Figma uniquement).
  - État du build MCP : présence de `dist/index.js`.

**Arguments :**

| Argument | Description |
|----------|-------------|
| `nom-du-service` | Identifiant du service. Optionnel — tous les services si omis. |

**Exemples :**

```bash
# État de tous les services
oc service status

# État de Figma uniquement
oc service status figma

# Via alias
oc figma status
```

---

## `oc service list`

Liste tous les services disponibles dans le catalogue avec leur état de configuration.

```bash
oc service list
```

**Comportement :**
- Affiche un tableau avec : nom du service, description, état (Configuré / Non configuré).
- Équivalent à `oc service` sans argument.

**Exemples :**

```bash
oc service list
oc service
```

---

## `oc service remove`

Supprime la configuration d'un service (retire les variables d'environnement de `~/.config/opencode/config.json`).

```bash
oc service remove <nom-du-service>
```

**Comportement :**
- Demande confirmation avant suppression.
- Supprime uniquement les clés appartenant au service (les autres services ne sont pas affectés).
- Si le service n'est pas configuré, affiche un avertissement et ne fait rien.

**Arguments :**

| Argument | Description |
|----------|-------------|
| `nom-du-service` | Identifiant du service à supprimer. Obligatoire. |

**Exemples :**

```bash
oc service remove figma
oc service remove gitlab
```

---

## Aliases

Les services courants disposent d'aliases pour raccourcir les commandes :

| Alias | Équivalent |
|-------|-----------|
| `oc figma <cmd> [args]` | `oc service <cmd> [args] figma` |
| `oc gitlab <cmd> [args]` | `oc service <cmd> [args] gitlab` |

**Exemples avec aliases :**

```bash
oc figma setup          # = oc service setup figma
oc figma status         # = oc service status figma
oc gitlab setup         # = oc service setup gitlab
oc gitlab status        # = oc service status gitlab
```

---

## Stockage de la configuration

Les credentials sont stockés dans `~/.config/opencode/config.json`, section `env`. Ce fichier est lu automatiquement par opencode au démarrage et les variables sont injectées dans l'environnement des serveurs MCP.

Structure du fichier :

```json
{
  "$schema": "https://opencode.ai/config.json",
  "env": {
    "FIGMA_PERSONAL_ACCESS_TOKEN": "figd_xxx",
    "FIGMA_TEAM_ID": "123456",
    "GITLAB_PERSONAL_ACCESS_TOKEN": "glpat-xxx",
    "GITLAB_BASE_URL": "https://gitlab.mycompany.com"
  }
}
```

---

## Catalogue des services (`config/services.json`)

La commande est pilotée par le catalogue `config/services.json`. Chaque entrée définit :

| Champ | Description |
|-------|-------------|
| `label` | Nom affiché |
| `description_fr` / `description_en` | Description bilingue |
| `mcp_server` | Nom du dossier sous `servers/` |
| `docs_url` | URL de la documentation officielle |
| `validation.endpoint` | URL pour valider le token (optionnel) |
| `validation.header` | Nom du header HTTP pour le token |
| `team_validation.endpoint` | URL pour valider l'accessibilité de la team/ressource (optionnel, template avec `{NOM_CHAMP}`) |
| `team_validation.team_field` | Nom du credential contenant l'ID de la team/ressource |
| `credentials[]` | Liste des credentials requis |

**Ajouter un nouveau service :**

Il suffit d'ajouter une entrée dans `config/services.json`. Aucune modification de code n'est nécessaire.

```json
{
  "services": {
    "mon-service": {
      "label": "Mon Service",
      "description_fr": "Description en français",
      "description_en": "English description",
      "mcp_server": "mon-service-mcp",
      "credentials": [
        {
          "key": "MON_SERVICE_API_TOKEN",
          "label_fr": "Token API",
          "label_en": "API Token",
          "secret": true,
          "required": true,
          "help_fr": "Comment obtenir ce token...",
          "help_en": "How to get this token..."
        }
      ]
    }
  }
}
```

---

## Sélection des MCP par projet

Par défaut, **aucun serveur MCP n'est déployé** sur un projet (opt-in). La sélection est stockée dans le champ `- MCP :` de `projects/projects.md` et appliquée à chaque `oc deploy`.

**Configuration lors de `oc init` :**

L'étape 4 du wizard `oc init` propose :

```
◇  Étape 4/6 — Services MCP
│
│  Activer des intégrations MCP pour ce projet ? [y/N] :
```

Répondre `Y` ouvre un sélecteur multi-choix listant tous les services configurés. Le résultat est persisté immédiatement.

**Configuration manuelle :**

Éditer `projects/projects.md` directement puis relancer `oc deploy <PROJECT_ID>` :

```markdown
## MON-APP
- MCP : figma-mcp,gitlab-mcp    # liste CSV des noms mcp_server
# ou :
- MCP : all                      # déploie tous les MCP disponibles
# ou :
- MCP : none                     # ne déploie aucun MCP (comportement par défaut si champ absent)
```

**Application d'un changement :**

```bash
# Après édition de projects.md :
oc deploy MON-APP
```

Le champ `- MCP :` est lu lors de la phase de déploiement et contrôle quels serveurs sont copiés dans `.opencode/servers/` et configurés dans `opencode.json`.

---

## Voir aussi

- [Guide d'intégration Figma](../guides/figma-integration.fr.md)
- [Guide d'intégration GitLab](../guides/gitlab-integration.fr.md)
- [Référence CLI complète](cli.fr.md)
- [Architecture des serveurs MCP](../../servers/README.md)
