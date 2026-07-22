# Tableau de Bord Web - Guide

> 🇬🇧 [Read in English](dashboard.en.md)

## Vue d'ensemble

`oh serve` lance une interface web légère en lecture seule pour surveiller tes projets et sessions d'agents en temps réel. Il offre une vue d'ensemble de tous les projets enregistrés, leurs sessions récentes et la télémétrie de performance des agents — sans extension de navigateur ni service externe.

---

## Lancer le Tableau de Bord

### Lancement par défaut

```bash
oh serve
```

Démarre le tableau de bord sur `http://127.0.0.1:4747`. Ouvre dans n'importe quel navigateur.

### Port personnalisé

```bash
oh serve --port 9090
```

### Mode lecture-écriture

```bash
oh serve --readonly=false
```

En mode lecture-écriture, les endpoints API acceptent les requêtes `POST` et `DELETE` pour la gestion des sessions. Le mode lecture seule (par défaut) accepte uniquement les requêtes `GET`.

### Arrêter le serveur

Appuie sur `Ctrl+C` dans le terminal où `oh serve` tourne.

---

## Sécurité

Le tableau de bord **se lie toujours à `127.0.0.1`** (localhost uniquement). Il n'est jamais exposé au réseau, même quand `--port` est défini. Il n'y a pas d'authentification car le serveur n'est pas accessible depuis l'extérieur de la machine.

> **Attention :** Ne mets pas le tableau de bord derrière un reverse proxy public sans ajouter une authentification (ex. HTTP Basic Auth via nginx).

---

## Fonctionnalités du Tableau de Bord

### Tableau des projets

Liste tous les projets enregistrés avec `oh`. Colonnes :

| Colonne | Description |
|---------|-------------|
| Nom | Nom du projet |
| Chemin | Chemin absolu sur le disque |
| Dernière session | Horodatage de la session la plus récente |
| Total sessions | Nombre de sessions total |
| Statut | Active / idle |

### Tableau des sessions récentes

Affiche les 50 dernières sessions sur tous les projets. Colonnes :

| Colonne | Description |
|---------|-------------|
| ID session | UUID |
| Projet | Projet parent |
| Agent | Agent ayant exécuté la session |
| Démarré | Horodatage de début |
| Durée | Temps réel |
| Statut | completed / failed / running |

### Tableau de télémétrie des agents

Métriques de performance par agent agrégées sur toutes les sessions :

| Colonne | Description |
|---------|-------------|
| Agent | Nom de l'agent |
| Sessions | Nombre total de sessions |
| Durée moy. | Durée moyenne de session |
| Taux de réussite | % de sessions avec statut = completed |
| Vu en dernier | Horodatage de la session la plus récente |

---

## Endpoints API

Le tableau de bord expose une API REST pour l'accès programmatique :

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `GET` | `/api/v1/health` | Vérification de santé du serveur |
| `GET` | `/api/v1/projects` | Lister tous les projets enregistrés |
| `GET` | `/api/v1/sessions` | Lister les sessions récentes (query : `?project=<nom>&limit=<n>`) |
| `GET` | `/api/v1/metrics/agents` | Agrégats de télémétrie des agents |

### Exemples d'API

```bash
# Vérification de santé
curl http://127.0.0.1:4747/api/v1/health

# Lister tous les projets
curl http://127.0.0.1:4747/api/v1/projects

# Lister les 10 dernières sessions d'un projet spécifique
curl "http://127.0.0.1:4747/api/v1/sessions?project=mon-projet&limit=10"

# Obtenir la télémétrie des agents
curl http://127.0.0.1:4747/api/v1/metrics/agents
```

### Exemple de réponse health

```json
{
  "status": "ok",
  "version": "0.9.0",
  "uptime_seconds": 3600
}
```

### Exemple de réponse sessions

```json
[
  {
    "id": "7c8a-1234-5678-9abc",
    "project": "mon-projet",
    "agent": "planner",
    "started_at": "2026-07-22T09:00:00Z",
    "duration_ms": 45000,
    "status": "completed"
  }
]
```

---

## Télémétrie des Agents

Les données de télémétrie sont enregistrées automatiquement à chaque démarrage de session avec `oh start`. Aucune configuration supplémentaire n'est requise. Chaque session enregistre :
- Nom et version de l'agent
- Heure de démarrage et durée
- Statut de complétion (completed / failed)
- Nombre d'appels d'outils effectués

La télémétrie est stockée localement dans `~/.oh/oh.db` et n'est jamais envoyée à des services externes.

---

## Rafraîchissement Automatique

L'interface du tableau de bord se rafraîchit automatiquement toutes les **30 secondes**. Pour forcer un rafraîchissement immédiat, clique sur le bouton de rafraîchissement en haut à droite ou appuie sur `F5`.

---

## Cas d'Usage

### Surveillance des sessions parallèles

Lors de l'exécution de `oh start --parallel`, ouvre le tableau de bord dans un onglet de navigateur pour surveiller toutes les sessions sans changer de fenêtre TUI. Le tableau des sessions se met à jour en temps réel.

### Revue des performances des agents

Utilise le tableau de télémétrie des agents pour identifier les agents avec des taux d'échec élevés ou des durées inhabituellement longues. Exporte les données via l'API pour une analyse approfondie.

### Exporter les métriques

```bash
# Exporter toute la télémétrie des agents en JSON
curl http://127.0.0.1:4747/api/v1/metrics/agents > metriques-agents.json

# Exporter toutes les sessions des 7 derniers jours (nécessite jq)
curl "http://127.0.0.1:4747/api/v1/sessions?limit=1000" | \
  jq '[.[] | select(.started_at > "2026-07-15T00:00:00Z")]' > sessions-semaine.json
```

---

## Ressources

- [Guide Sauvegarde & Restauration](./backup-restore.fr.md) — comment sauvegarder `oh.db`
- [Référence CLI `oh serve`](../reference/serve.fr.md)
