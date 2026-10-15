> [Read in English](review-feedback.en.md)

# Revue de code et feedback — Guide

## Aperçu

openhub fournit un pipeline de revue de code assiste par IA en trois étapes : revue automatisée, publication de merge request et traitement du feedback. Ce guide couvre le workflow complet, de la revue à la correction.

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

`oh review [--mode …] [--branch …]` reste un alias déprécié de `oh run review`.

**Détection automatique de branche :** Lorsque vous êtes sur une branche de fonctionnalité, l'agent de revue détecte automatiquement la branche de base et injecte le contexte `[BRANCH:feature/xyz] [BASE:main]` dans le prompt de revue.

### Entrées et options

| Entrée / option | Description |
|--------|-------------|
| `-i review_mode=…` | Mode de revue : `standard`, `adversarial`, `edge-case`, `standard+adversarial`, `all` (vide : choix au démarrage) |
| `-i branch=…` | Branche à examiner (vide : modifications récentes) |
| `-i base=…` | Branche de base (défaut : `main`) |
| `-p`, `--project` | ID du projet |

Voir [Workflows livrés › review](../reference/workflows.fr.md#review).

---

## Publication d'une Merge Request

Une fois la revue terminée, publiez les résultats sous forme de merge request GitLab. La publication n'est pas un workflow : elle reste une commande d'oh.

```bash
oh review --publish                          # Creer une MR pour la branche courante
oh review --publish --reviewer alice         # Creer une MR et assigner un relecteur
```

Cette commande :
1. Crée une merge request sur GitLab pour la branche de fonctionnalité courante
2. Assigne optionnellement un relecteur (résout l'ID du membre d'équipe vers l'ID utilisateur GitLab)
3. Fait passer le statut de la réclamation à `review` dans l'état d'équipe
4. Ajoute un événement `review.ready` et envoie une notification
5. Applique le label `agent-reviewed` à la MR

> **Remarque :** La fusion reste une action manuelle du développeur. `--publish` ne fait que créer la MR.

### Options

| Option | Description |
|--------|-------------|
| `--publish` | Créer une merge request GitLab |
| `--reviewer` | ID du membre d'équipe à assigner comme relecteur |

---

## Traitement du feedback

Lorsqu'un relecteur humain laisse des commentaires sur la MR, le workflow `review-feedback` les traite. `oh review feedback <ticket-ou-branche>` lit les discussions sur GitLab puis lance ce workflow (alias déprécié de `oh run review-feedback` : oh affiche un avertissement) :

```bash
oh review feedback BD-42             # Par reference de ticket
oh review feedback feat/auth-flow    # Par nom de branche
```

Le ticket ou la branche est obligatoire. Avec `oh run review-feedback`, les entrées `mr`, `branch` et `feedback` (discussions) sont à fournir vous-meme (`-i cle=valeur`) : voir [Workflows livrés › review-feedback](../reference/workflows.fr.md#review-feedback).

### Fonctionnement

1. **Récupère les discussions non résolues de la MR** depuis GitLab
2. **Affiche un aperçu** : informations de la MR, nombre de discussions non résolues, auteurs, fichiers concernés
3. **Demande confirmation** avant de lancer le traitement
4. **Lance le workflow `review-feedback`** avec un prompt structure contenant toutes les discussions
5. L'agent lit chaque commentaire, applique les corrections, exécute les tests, effectue un commit groupe
6. Répond optionnellement sur chaque fil résolu via `gitlab_reply_to_mr_discussion`

### Limites

- Maximum **30 discussions** par session de feedback
- Maximum **2000 caractères** par corps de note (tronqué avec `[truncated]`)

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

## Prérequis

- **Token GitLab** avec le scope `api` configuré via `oh mcp setup gitlab` ou `oh secrets set GITLAB_TOKEN`
- **Serveur MCP GitLab** active dans `hub.toml`
- **État d'équipe** initialisé (`oh team init`) pour les transitions de statut et les notifications

---

## Dépannage

### MR introuvable

```
Error: no merge request found for branch "feat/xyz"
```

Vérifiez que la branche a été poussée vers le dépôt distant. `oh review feedback` recherche une MR ouverte correspondant au nom de la branche.

### Permissions du token insuffisantes

```
Error: 403 Forbidden
```

Le token GitLab nécessite le scope `api`. Les tokens avec `read_api` uniquement ne peuvent pas créer de MR ni lire les discussions.

### Prompt de feedback trop volumineux

Si la MR contient de nombreuses discussions, le prompt peut être limité. L'agent traite les 30 premières discussions avec des corps de notes tronqués (2000 caractères max). Pour les revues très volumineuses, traitez le feedback en plusieurs passes.

---

## Ressources

- [Guide d'intégration GitLab](gitlab-integration.fr.md)
- [Référence du serveur MCP Team](../reference/mcp-team.fr.md)
- [Référence CLI](../reference/cli.fr.md)
