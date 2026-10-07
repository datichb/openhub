> [Read in English](review-feedback.en.md)

# Revue de code et feedback — Guide

## Apercu

openhub fournit un pipeline de revue de code assiste par IA en trois etapes : revue automatisee, publication de merge request et traitement du feedback. Ce guide couvre le workflow complet, de la revue a la correction.

---

## Modes de revue

Lancez une revue de code IA avec le workflow `review` :

```bash
oh run review                                   # Le reviewer propose le choix du mode au demarrage
oh run review -i review_mode=standard           # Revue standard
oh run review -i review_mode=adversarial        # Revue adversariale (cas limites, securite)
oh run review -i review_mode=edge-case          # Revue axee sur les cas limites
oh run review -i review_mode=standard+adversarial  # Modes combines
oh run review -i review_mode=all                # Tous les modes de revue
```

`oh review [--mode …] [--branch …]` reste un alias deprecie de `oh run review`.

**Detection automatique de branche :** Lorsque vous etes sur une branche de fonctionnalite, l'agent de revue detecte automatiquement la branche de base et injecte le contexte `[BRANCH:feature/xyz] [BASE:main]` dans le prompt de revue.

### Entrees et options

| Entree / option | Description |
|--------|-------------|
| `-i review_mode=…` | Mode de revue : `standard`, `adversarial`, `edge-case`, `standard+adversarial`, `all` (vide : choix au demarrage) |
| `-i branch=…` | Branche a examiner (vide : modifications recentes) |
| `-i base=…` | Branche de base (defaut : `main`) |
| `-p`, `--project` | ID du projet |

Voir [Workflows livres › review](../reference/workflows.fr.md#review).

---

## Publication d'une Merge Request

Une fois la revue terminee, publiez les resultats sous forme de merge request GitLab. La publication n'est pas un workflow : elle reste une commande d'oh.

```bash
oh review --publish                          # Creer une MR pour la branche courante
oh review --publish --reviewer alice         # Creer une MR et assigner un relecteur
```

Cette commande :
1. Cree une merge request sur GitLab pour la branche de fonctionnalite courante
2. Assigne optionnellement un relecteur (resout l'ID du membre d'equipe vers l'ID utilisateur GitLab)
3. Fait passer le statut de la reclamation a `review` dans l'etat d'equipe
4. Ajoute un evenement `review.ready` et envoie une notification
5. Applique le label `agent-reviewed` a la MR

> **Remarque :** La fusion reste une action manuelle du developpeur. `--publish` ne fait que creer la MR.

### Options

| Option | Description |
|--------|-------------|
| `--publish` | Creer une merge request GitLab |
| `--reviewer` | ID du membre d'equipe a assigner comme relecteur |

---

## Traitement du feedback

Lorsqu'un relecteur humain laisse des commentaires sur la MR, le workflow `review-feedback` les traite. `oh review feedback <ticket-ou-branche>` lit les discussions sur GitLab puis lance ce workflow (alias deprecie de `oh run review-feedback` : oh affiche un avertissement) :

```bash
oh review feedback BD-42             # Par reference de ticket
oh review feedback feat/auth-flow    # Par nom de branche
```

Le ticket ou la branche est obligatoire. Avec `oh run review-feedback`, les entrees `mr`, `branch` et `feedback` (discussions) sont a fournir vous-meme (`-i cle=valeur`) : voir [Workflows livres › review-feedback](../reference/workflows.fr.md#review-feedback).

### Fonctionnement

1. **Recupere les discussions non resolues de la MR** depuis GitLab
2. **Affiche un apercu** : informations de la MR, nombre de discussions non resolues, auteurs, fichiers concernes
3. **Demande confirmation** avant de lancer le traitement
4. **Lance le workflow `review-feedback`** avec un prompt structure contenant toutes les discussions
5. L'agent lit chaque commentaire, applique les corrections, execute les tests, effectue un commit groupe
6. Repond optionnellement sur chaque fil resolu via `gitlab_reply_to_mr_discussion`

### Limites

- Maximum **30 discussions** par session de feedback
- Maximum **2000 caracteres** par corps de note (tronque avec `[truncated]`)

### Options

| Option | Court | Description |
|--------|-------|-------------|
| `--project` | `-p` | ID du projet |
| `--yes` | | Passer l'invite de confirmation |

---

## Workflow de bout en bout

```
1. oh run ticket                          # Implementer la fonctionnalite
2. oh run review -i review_mode=standard  # L'IA examine le code
3. oh review --publish                    # Creer la MR sur GitLab
4. [Revue humaine sur GitLab]             # Le relecteur laisse des commentaires
5. oh review feedback BD-42               # L'IA traite le feedback (workflow review-feedback)
6. [Approbation humaine]                  # Approbation finale
7. [Fusion par le developpeur]            # Fusion manuelle
```

---

## Prerequis

- **Token GitLab** avec le scope `api` configure via `oh mcp setup gitlab` ou `oh secrets set GITLAB_TOKEN`
- **Serveur MCP GitLab** active dans `hub.toml`
- **Etat d'equipe** initialise (`oh team init`) pour les transitions de statut et les notifications

---

## Depannage

### MR introuvable

```
Error: no merge request found for branch "feat/xyz"
```

Verifiez que la branche a ete poussee vers le depot distant. `oh review feedback` recherche une MR ouverte correspondant au nom de la branche.

### Permissions du token insuffisantes

```
Error: 403 Forbidden
```

Le token GitLab necessite le scope `api`. Les tokens avec `read_api` uniquement ne peuvent pas creer de MR ni lire les discussions.

### Prompt de feedback trop volumineux

Si la MR contient de nombreuses discussions, le prompt peut etre limite. L'agent traite les 30 premieres discussions avec des corps de notes tronques (2000 caracteres max). Pour les revues tres volumineuses, traitez le feedback en plusieurs passes.

---

## Ressources

- [Guide d'integration GitLab](gitlab-integration.fr.md)
- [Reference du serveur MCP Team](../reference/mcp-team.fr.md)
- [Reference CLI](../reference/cli.fr.md)
