> [Read in English](gitlab-integration.en.md)

# Intégration GitLab — Guide de démarrage

## Vue d'ensemble

L'intégration GitLab connecte les agents à vos projets GitLab — issues, merge requests, discussions et approbations — avec support de **GitLab.com** et des instances **self-hosted**. Elle fournit des capacités de **lecture** pour les workflows de planification et d'onboarding, ainsi que des capacités d'**écriture** optionnelles pour le feedback de développement et la revue de code.

### Fonctionnalités

**Capacités de lecture :**

- **Métadonnées du projet** : informations du projet, branche par défaut, visibilité, namespace
- **Liste des issues** : filtrage par état, labels, mots-cles
- **Liste des merge requests** : MRs ouvertes/mergées/fermées avec info de branche et de changements
- **Discussions de MR** : commentaires de revue en fil de discussion
- **Approbations de MR** : état des approbations, approbateurs requis, règles d'approbation

**Capacités d'écriture** (nécessite `GITLAB_WRITE_ENABLED=true`) :

- **Création de merge requests** : ouvrir des MRs depuis une branche source vers une branche cible
- **Ajout de notes sur MR** : poster des commentaires sur les merge requests
- **Mise à jour des issues** : changer l'état, les labels, les assignés, le milestone
- **Assignation de reviewers** : définir les reviewers sur une merge request
- **Ajout de labels** : appliquer des labels aux issues ou MRs
- **Réponse aux discussions de MR** : répondre aux fils de discussion de revue existants

---

## Prérequis

1. Un compte GitLab avec accès au(x) projet(s) cible(s)
2. Un **Personal Access Token** (PAT) avec le scope `api` :
   - Aller sur `<votre-gitlab>/-/profile/personal_access_tokens`
   - Cliquer sur **"Add new token"**
   - Choisir un nom (ex : `openhub`)
   - Sélectionner le scope `api` — couvre les issues, MRs, labels, milestones et discussions
   - Définir une date d'expiration
   - Copier le token généré (format : `glpat-xxxxxxxxxxxxxxxxxxxx`)
3. Définir la variable d'environnement `GITLAB_TOKEN` avec votre PAT
4. (Optionnel) Pour les instances self-hosted, définir `GITLAB_URL` avec l'URL de votre instance

---

## Configuration

### 1. Configurer via `oh mcp setup`

```bash
oh mcp setup gitlab
```

L'assistant interactif va :
1. Demander votre **Personal Access Token** GitLab
2. Demander l'**URL de votre instance** (laisser vide pour gitlab.com)
3. Valider la connexion à l'API GitLab
4. Stocker les identifiants de manière sécurisée dans le keychain système
5. Mettre à jour `hub.toml` avec le bloc `[mcp.gitlab]`

### 2. Configuration manuelle

Définir les variables d'environnement avant de lancer `oh` :

```bash
export GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx

# Self-hosted uniquement :
export GITLAB_URL=https://gitlab.monentreprise.com

# Pour activer les outils d'ecriture :
export GITLAB_WRITE_ENABLED=true
```

---

## Configuration dans hub.toml

```toml
[mcp.gitlab]
enabled = true
# Identifiants via variables d'environnement (recommande) ou keychain
# gitlab_url = "https://gitlab.monentreprise.com"  # omettre pour gitlab.com
write_enabled = false  # Mettre a true pour activer les outils d'ecriture
```

Aucun redéploiement nécessaire (`oh deploy` supprimé en v5) : le changement est pris en compte au prochain lancement de session, quand le paquet de session est reconstruit :

```bash
oh run <workflow>
```

---

## Outils disponibles

| Tool | Description | Utilisé par |
|------|-------------|-------------|
| `gitlab_get_project` | Métadonnées du projet (nom, branche par défaut, visibilité, namespace) | Onboarder |
| `gitlab_list_issues` | Liste des issues avec filtres (état, labels, recherche) | Planner, Pathfinder, Onboarder |
| `gitlab_list_mrs` | Liste des merge requests avec filtres (état, labels, branche source/cible) | Planner, Pathfinder, Onboarder |
| `gitlab_list_mr_discussions` | Liste des discussions en fil sur une merge request | Pathfinder |
| `gitlab_get_mr_approvals` | État des approbations, approbateurs requis et règles pour une MR | Pathfinder |
| `gitlab_create_mr` | Créer une merge request (mode écriture) | Orchestrator-dev (mode feedback) |
| `gitlab_add_mr_note` | Poster un commentaire sur une merge request (mode écriture) | Orchestrator-dev (mode feedback) |
| `gitlab_update_issue` | Modifier l'état, les labels, les assignés, le milestone (mode écriture) | Orchestrator-dev (mode feedback) |
| `gitlab_assign_reviewer` | Définir les reviewers sur une merge request (mode écriture) | Review system |
| `gitlab_add_label` | Appliquer des labels aux issues ou merge requests (mode écriture) | Review system |
| `gitlab_reply_to_mr_discussion` | Répondre à un fil de discussion de MR existant (mode écriture) | Review system |

---

## Mode écriture

Par défaut, le serveur MCP GitLab est en **lecture seule**. Pour activer les outils d'écriture :

```bash
export GITLAB_WRITE_ENABLED=true
```

Ou dans `hub.toml` :

```toml
[mcp.gitlab]
write_enabled = true
```

Cela débloque les 6 outils d'écriture : `gitlab_create_mr`, `gitlab_add_mr_note`, `gitlab_update_issue`, `gitlab_assign_reviewer`, `gitlab_add_label` et `gitlab_reply_to_mr_discussion`.

Le mode écriture est utilisé par :
- **Orchestrator-dev** en mode feedback — crée des MRs, poste des notes de revue, met à jour l'état des issues
- **Review system** — assigne des reviewers, applique des labels, répond aux fils de discussion

---

## Exemples d'utilisation

### Lister les issues pour la planification

```
"Liste les issues ouvertes du projet my-group/my-project"
"Montre-moi les bugs avec le label priority::high dans my-group/my-project"
```

Le planner lit les descriptions d'issues, les labels et les milestones pour décomposer le travail en tickets Beads.

### Créer une merge request

```
"Cree une MR de feature/auth vers main dans my-group/my-project"
"Ouvre une merge request pour mes changements"
```

Nécessite le mode écriture. L'orchestrator-dev crée la MR et assigne optionnellement des reviewers.

### Feedback de revue

```
"Poste un feedback de revue sur la MR !42 dans my-group/my-project"
"Reponds a la discussion sur la gestion d'erreurs dans la MR !42"
```

Nécessite le mode écriture. Le review system ajoute des notes et répond aux fils de discussion existants.

---

## GitLab self-hosted

Pour les instances GitLab self-hosted, définir `GITLAB_URL` :

```bash
export GITLAB_URL=https://gitlab.monentreprise.com
```

Exigences :
- L'URL **doit utiliser HTTPS** — les connexions HTTP sont rejetées
- L'URL doit pointer vers la racine de l'instance GitLab (ex : `https://gitlab.monentreprise.com`, pas `https://gitlab.monentreprise.com/api/v4`)
- L'instance GitLab doit être en version **13.0+** (API REST v4)

Si `GITLAB_URL` n'est pas défini, le serveur utilise `https://gitlab.com` par défaut.

---

## Dépannage

### 401 Unauthorized

Le token est invalidé ou expiré :
```bash
oh mcp setup gitlab  # reconfigurer
```
Assurez-vous d'utiliser un **Personal Access Token** (commence par `glpat-`), pas un token OAuth ou un deploy token.

### 403 Forbidden

Le token n'a pas le scope requis ou les permissions sur le projet :
- Vérifier que le token a le scope `api`
- Vérifier que le propriétaire du token a au moins le rôle **Reporter** sur le projet cible
- Pour les opérations d'écriture : le propriétaire du token a besoin du rôle **Developer** ou supérieur

### 404 Projet non trouvé

Le chemin du projet est incorrect ou le token n'a pas accès :
- Vérifier le format : `my-group/my-subgroup/my-project`
- Vérifier que le token peut accéder au projet (essayer `curl -H "PRIVATE-TOKEN: $GITLAB_TOKEN" "$GITLAB_URL/api/v4/projects/my-group%2Fmy-project"`)

### Problèmes d'URL self-hosted

```
Error: GITLAB_URL must use HTTPS
```

Assurez-vous que `GITLAB_URL` commence par `https://`. HTTP n'est pas supporté.

```
Error: cannot reach GitLab API
```

Vérifier que l'URL est correcte et que l'instance est accessible depuis votre machine. Vérifier les exigences VPN ou pare-feu.

### Approbations indisponibles sur le tier Free

L'outil `gitlab_get_mr_approvals` nécessite GitLab **Premium** ou **Ultimate**. Sur GitLab Free (y compris gitlab.com Free), l'API retourne des données d'approbation vides. L'outil fonctionne toujours mais ne retourne aucune règle d'approbation.

---

## Limitations actuelles

- Pas de pagination automatique — les outils de liste retournent jusqu'à 100 résultats par appel
- Pas de retry sur HTTP 429 (rate limiting) — reculer manuellement en cas de dépassement de limites
- Pas d'outils dédiés pour la liste des labels ou milestones — utiliser les filtres de `gitlab_list_issues` ou les métadonnées du projet

---

## Ressources

- [Référence MCP GitLab](../reference/mcp-gitlab.fr.md)
- [Documentation API REST GitLab](https://docs.gitlab.com/ee/api/)
- [Personal Access Tokens GitLab](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html)
- [Guide d'intégration Jira](jira-integration.fr.md)
- [Guide d'intégration GitHub](github-integration.fr.md)

---

## Support

- `oh mcp status gitlab` — vérifier la configuration et l'état de la connexion
- `oh mcp setup gitlab` — reconfigurer le service de manière interactive
- Problème persistant → reporter sur [GitHub Issues](https://github.com/anomalyco/opencode)
