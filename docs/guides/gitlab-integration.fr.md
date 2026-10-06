> [Read in English](gitlab-integration.en.md)

# Integration GitLab — Guide de demarrage

## Vue d'ensemble

L'integration GitLab connecte les agents a vos projets GitLab — issues, merge requests, discussions et approbations — avec support de **GitLab.com** et des instances **self-hosted**. Elle fournit des capacites de **lecture** pour les workflows de planification et d'onboarding, ainsi que des capacites d'**ecriture** optionnelles pour le feedback de developpement et la revue de code.

### Fonctionnalites

**Capacites de lecture :**

- **Metadonnees du projet** : informations du projet, branche par defaut, visibilite, namespace
- **Liste des issues** : filtrage par etat, labels, mots-cles
- **Liste des merge requests** : MRs ouvertes/mergees/fermees avec info de branche et de changements
- **Discussions de MR** : commentaires de revue en fil de discussion
- **Approbations de MR** : etat des approbations, approbateurs requis, regles d'approbation

**Capacites d'ecriture** (necessite `GITLAB_WRITE_ENABLED=true`) :

- **Creation de merge requests** : ouvrir des MRs depuis une branche source vers une branche cible
- **Ajout de notes sur MR** : poster des commentaires sur les merge requests
- **Mise a jour des issues** : changer l'etat, les labels, les assignes, le milestone
- **Assignation de reviewers** : definir les reviewers sur une merge request
- **Ajout de labels** : appliquer des labels aux issues ou MRs
- **Reponse aux discussions de MR** : repondre aux fils de discussion de revue existants

---

## Prerequis

1. Un compte GitLab avec acces au(x) projet(s) cible(s)
2. Un **Personal Access Token** (PAT) avec le scope `api` :
   - Aller sur `<votre-gitlab>/-/profile/personal_access_tokens`
   - Cliquer sur **"Add new token"**
   - Choisir un nom (ex : `openhub`)
   - Selectionner le scope `api` — couvre les issues, MRs, labels, milestones et discussions
   - Definir une date d'expiration
   - Copier le token genere (format : `glpat-xxxxxxxxxxxxxxxxxxxx`)
3. Definir la variable d'environnement `GITLAB_TOKEN` avec votre PAT
4. (Optionnel) Pour les instances self-hosted, definir `GITLAB_URL` avec l'URL de votre instance

---

## Configuration

### 1. Configurer via `oh mcp setup`

```bash
oh mcp setup gitlab
```

L'assistant interactif va :
1. Demander votre **Personal Access Token** GitLab
2. Demander l'**URL de votre instance** (laisser vide pour gitlab.com)
3. Valider la connexion a l'API GitLab
4. Stocker les identifiants de maniere securisee dans le keychain systeme
5. Mettre a jour `hub.toml` avec le bloc `[mcp.gitlab]`

### 2. Configuration manuelle

Definir les variables d'environnement avant de lancer `oh` :

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

| Tool | Description | Utilise par |
|------|-------------|-------------|
| `gitlab_get_project` | Metadonnees du projet (nom, branche par defaut, visibilite, namespace) | Onboarder |
| `gitlab_list_issues` | Liste des issues avec filtres (etat, labels, recherche) | Planner, Pathfinder, Onboarder |
| `gitlab_list_mrs` | Liste des merge requests avec filtres (etat, labels, branche source/cible) | Planner, Pathfinder, Onboarder |
| `gitlab_list_mr_discussions` | Liste des discussions en fil sur une merge request | Pathfinder |
| `gitlab_get_mr_approvals` | Etat des approbations, approbateurs requis et regles pour une MR | Pathfinder |
| `gitlab_create_mr` | Creer une merge request (mode ecriture) | Orchestrator-dev (mode feedback) |
| `gitlab_add_mr_note` | Poster un commentaire sur une merge request (mode ecriture) | Orchestrator-dev (mode feedback) |
| `gitlab_update_issue` | Modifier l'etat, les labels, les assignes, le milestone (mode ecriture) | Orchestrator-dev (mode feedback) |
| `gitlab_assign_reviewer` | Definir les reviewers sur une merge request (mode ecriture) | Review system |
| `gitlab_add_label` | Appliquer des labels aux issues ou merge requests (mode ecriture) | Review system |
| `gitlab_reply_to_mr_discussion` | Repondre a un fil de discussion de MR existant (mode ecriture) | Review system |

---

## Mode ecriture

Par defaut, le serveur MCP GitLab est en **lecture seule**. Pour activer les outils d'ecriture :

```bash
export GITLAB_WRITE_ENABLED=true
```

Ou dans `hub.toml` :

```toml
[mcp.gitlab]
write_enabled = true
```

Cela debloque les 6 outils d'ecriture : `gitlab_create_mr`, `gitlab_add_mr_note`, `gitlab_update_issue`, `gitlab_assign_reviewer`, `gitlab_add_label` et `gitlab_reply_to_mr_discussion`.

Le mode ecriture est utilise par :
- **Orchestrator-dev** en mode feedback — cree des MRs, poste des notes de revue, met a jour l'etat des issues
- **Review system** — assigne des reviewers, applique des labels, repond aux fils de discussion

---

## Exemples d'utilisation

### Lister les issues pour la planification

```
"Liste les issues ouvertes du projet my-group/my-project"
"Montre-moi les bugs avec le label priority::high dans my-group/my-project"
```

Le planner lit les descriptions d'issues, les labels et les milestones pour decomposer le travail en tickets Beads.

### Creer une merge request

```
"Cree une MR de feature/auth vers main dans my-group/my-project"
"Ouvre une merge request pour mes changements"
```

Necessite le mode ecriture. L'orchestrator-dev cree la MR et assigne optionnellement des reviewers.

### Feedback de revue

```
"Poste un feedback de revue sur la MR !42 dans my-group/my-project"
"Reponds a la discussion sur la gestion d'erreurs dans la MR !42"
```

Necessite le mode ecriture. Le review system ajoute des notes et repond aux fils de discussion existants.

---

## GitLab self-hosted

Pour les instances GitLab self-hosted, definir `GITLAB_URL` :

```bash
export GITLAB_URL=https://gitlab.monentreprise.com
```

Exigences :
- L'URL **doit utiliser HTTPS** — les connexions HTTP sont rejetees
- L'URL doit pointer vers la racine de l'instance GitLab (ex : `https://gitlab.monentreprise.com`, pas `https://gitlab.monentreprise.com/api/v4`)
- L'instance GitLab doit etre en version **13.0+** (API REST v4)

Si `GITLAB_URL` n'est pas defini, le serveur utilise `https://gitlab.com` par defaut.

---

## Depannage

### 401 Unauthorized

Le token est invalide ou expire :
```bash
oh mcp setup gitlab  # reconfigurer
```
Assurez-vous d'utiliser un **Personal Access Token** (commence par `glpat-`), pas un token OAuth ou un deploy token.

### 403 Forbidden

Le token n'a pas le scope requis ou les permissions sur le projet :
- Verifier que le token a le scope `api`
- Verifier que le proprietaire du token a au moins le role **Reporter** sur le projet cible
- Pour les operations d'ecriture : le proprietaire du token a besoin du role **Developer** ou superieur

### 404 Projet non trouve

Le chemin du projet est incorrect ou le token n'a pas acces :
- Verifier le format : `my-group/my-subgroup/my-project`
- Verifier que le token peut acceder au projet (essayer `curl -H "PRIVATE-TOKEN: $GITLAB_TOKEN" "$GITLAB_URL/api/v4/projects/my-group%2Fmy-project"`)

### Problemes d'URL self-hosted

```
Error: GITLAB_URL must use HTTPS
```

Assurez-vous que `GITLAB_URL` commence par `https://`. HTTP n'est pas supporte.

```
Error: cannot reach GitLab API
```

Verifier que l'URL est correcte et que l'instance est accessible depuis votre machine. Verifier les exigences VPN ou pare-feu.

### Approbations indisponibles sur le tier Free

L'outil `gitlab_get_mr_approvals` necessite GitLab **Premium** ou **Ultimate**. Sur GitLab Free (y compris gitlab.com Free), l'API retourne des donnees d'approbation vides. L'outil fonctionne toujours mais ne retourne aucune regle d'approbation.

---

## Limitations actuelles

- Pas de pagination automatique — les outils de liste retournent jusqu'a 100 resultats par appel
- Pas de retry sur HTTP 429 (rate limiting) — reculer manuellement en cas de depassement de limites
- Pas d'outils dedies pour la liste des labels ou milestones — utiliser les filtres de `gitlab_list_issues` ou les metadonnees du projet

---

## Ressources

- [Reference MCP GitLab](../reference/mcp-gitlab.fr.md)
- [Documentation API REST GitLab](https://docs.gitlab.com/ee/api/)
- [Personal Access Tokens GitLab](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html)
- [Guide d'integration Jira](jira-integration.fr.md)
- [Guide d'integration GitHub](github-integration.fr.md)

---

## Support

- `oh mcp status gitlab` — verifier la configuration et l'etat de la connexion
- `oh mcp setup gitlab` — reconfigurer le service de maniere interactive
- Probleme persistant → reporter sur [GitHub Issues](https://github.com/anomalyco/opencode)
