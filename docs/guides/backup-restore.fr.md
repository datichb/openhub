# Sauvegarde & Restauration - Guide

> 🇬🇧 [Read in English](backup-restore.en.md)

## Vue d'ensemble

`oh export` et `oh import` protègent ta configuration `oh` et tes données de projet contre la perte accidentelle, la défaillance de machine ou la corruption de base de données. Les sauvegardes sont des archives autonomes avec vérification de checksum qui peuvent restaurer une installation `oh` complète sur une nouvelle machine.

---

## Ce qui est Sauvegardé

Une archive de sauvegarde contient trois fichiers :

| Fichier | Description |
|---------|-------------|
| `oh.db` | Base de données SQLite — tous les projets enregistrés, sessions, claims et événements |
| `hub.toml` | Configuration du hub (serveurs MCP, intégrations, config d'équipe) |
| `secrets.enc` | Store de secrets chiffré (tokens, credentials) — déjà chiffré AES-256 au repos |

> **Note :** Les secrets sont sauvegardés sous forme chiffrée. Tu as besoin de ta phrase de passe maître pour les déchiffrer après restauration. La sauvegarde elle-même ne contient pas la phrase de passe.

---

## Créer une Sauvegarde

### Sauvegarde par défaut

```bash
oh export
```

Crée `oh-backup-<horodatage>.tar.gz` dans le répertoire courant.

### Chemin de sortie personnalisé

```bash
oh export --output ma-sauvegarde.tar.gz
oh export --output /mnt/sauvegardes/oh-$(date +%Y%m%d).tar.gz
```

### Ce qui se passe

1. `oh` vide les écritures en attente dans `oh.db`
2. Les trois fichiers sont collectés depuis `~/.oh/`
3. Un `manifest.json` est généré avec les checksums SHA-256 de chaque fichier
4. Tous les fichiers sont packagés dans une archive `.tar.gz`

---

## Contenu de la Sauvegarde

```
oh-backup-2026-07-22T09-00-00.tar.gz
├── manifest.json      # Checksums + liste de fichiers + version oh
├── oh.db              # Projets, sessions, claims, événements
├── hub.toml           # Configuration du hub
└── secrets.enc        # Secrets chiffrés
```

Exemple de `manifest.json` :

```json
{
  "oh_version": "0.9.0",
  "created_at": "2026-07-22T09:00:00Z",
  "files": [
    { "name": "oh.db",       "sha256": "abc123..." },
    { "name": "hub.toml",    "sha256": "def456..." },
    { "name": "secrets.enc", "sha256": "ghi789..." }
  ]
}
```

---

## Restaurer depuis une Sauvegarde

### Restauration standard

```bash
oh import ma-sauvegarde.tar.gz
```

Cette commande va :
1. Vérifier le checksum de l'archive contre `manifest.json` (rejette les archives corrompues)
2. Vérifier la compatibilité de version (avertit si `oh_version` diffère de la version actuelle)
3. Demander confirmation avant d'écraser les fichiers existants
4. Extraire les fichiers dans `~/.oh/`
5. Effectuer une vérification d'intégrité rapide sur `oh.db` restauré

### Écraser sans confirmation

```bash
oh import ma-sauvegarde.tar.gz --overwrite
```

### Restaurer dans un répertoire personnalisé

```bash
oh import ma-sauvegarde.tar.gz --dir /tmp/oh-restore
```

---

## Vérification des Checksums

Chaque `oh import` vérifie automatiquement le checksum SHA-256 de chaque fichier contre `manifest.json`. Si un fichier est corrompu :

```
Error: checksum mismatch for oh.db (expected abc123..., got xyz789...)
Backup archive may be corrupted. Aborting restore.
```

Pour vérifier une sauvegarde sans restaurer :

```bash
oh import ma-sauvegarde.tar.gz --verify-only
```

---

## Diagnostiquer les Problèmes de Base de Données

Si `oh.db` est corrompu ou se comporte de manière inattendue, utilise `oh repair` :

### Mode vérification seule (non destructif)

```bash
oh repair --check-only
```

Exécute les vérifications d'intégrité SQLite et rapporte les problèmes sans modifier la base de données :
- Intégrité des pages (`PRAGMA integrity_check`)
- Violations de clés étrangères (`PRAGMA foreign_key_check`)
- Cohérence des index

### Mode réparation

```bash
oh repair
```

Tente de récupérer la base de données en :
1. Exécutant `VACUUM` pour défragmenter et reconstruire
2. Recréant les index endommagés
3. Supprimant les enregistrements orphelins

> `oh repair` modifie la base de données. **Crée une sauvegarde d'abord** avec `oh export` avant de l'exécuter.

---

## Options de Récupération

| Scénario | Action recommandée |
|----------|-------------------|
| Corruption mineure de la base de données | `oh repair` |
| Corruption sévère de la base de données | `oh import` depuis la dernière sauvegarde |
| `hub.toml` perdu | `oh import` (restaurer uniquement la config avec `--files hub.toml`) |
| Nouvelle machine | Restauration complète avec `oh import` |
| Secrets perdus | `oh import` + re-saisir la phrase de passe maître |

### Restaurer un seul fichier

```bash
# Restaurer uniquement hub.toml depuis la sauvegarde
oh import ma-sauvegarde.tar.gz --files hub.toml
```

---

## Workflow de Récupération après Sinistre

Récupération complète étape par étape sur une nouvelle machine :

```bash
# 1. Installer oh sur la nouvelle machine
curl -sSf https://install.oh.dev | sh

# 2. Copier ton archive de sauvegarde sur la nouvelle machine
scp ancienne-machine:~/oh-backup-latest.tar.gz .

# 3. Restaurer depuis la sauvegarde
oh import oh-backup-latest.tar.gz

# 4. Re-saisir ta phrase de passe maître (pour déchiffrer secrets.enc)
oh secrets unlock

# 5. Vérifier que tout fonctionne
oh doctor

# 6. Redéployer sur tous les projets
oh sync --all
```

`oh doctor` vérifie :
- L'intégrité de la base de données
- L'accessibilité du store de secrets
- L'existence sur disque de tous les projets enregistrés
- La validité des configurations des serveurs MCP

---

## Automatisation — Sauvegardes Quotidiennes

Ajoute une tâche cron pour sauvegarder `oh` quotidiennement :

```bash
# Éditer le crontab
crontab -e
```

```cron
# Sauvegarde oh quotidienne à 2h00, garder les 30 derniers jours
0 2 * * * oh export --output /mnt/sauvegardes/oh-$(date +\%Y\%m\%d).tar.gz && \
  find /mnt/sauvegardes -name "oh-*.tar.gz" -mtime +30 -delete
```

### Avec vérification

```bash
# Vérifier la sauvegarde immédiatement après création
0 2 * * * oh export --output /tmp/oh-daily.tar.gz && \
  oh import /tmp/oh-daily.tar.gz --verify-only && \
  mv /tmp/oh-daily.tar.gz /mnt/sauvegardes/oh-$(date +\%Y\%m\%d).tar.gz
```

### Sauvegarde vers un emplacement distant

```bash
# Exporter et synchroniser vers S3
0 2 * * * oh export --output /tmp/oh-sauvegarde.tar.gz && \
  aws s3 cp /tmp/oh-sauvegarde.tar.gz s3://mes-sauvegardes/oh/oh-$(date +\%Y\%m\%d).tar.gz && \
  rm /tmp/oh-sauvegarde.tar.gz
```

---

## Ressources

- [Guide Tableau de Bord Web](./dashboard.fr.md) — surveiller les sessions et métriques
- [Référence CLI `oh export` / `oh import`](../reference/backup.fr.md)
- [Vérifications d'intégrité SQLite](https://www.sqlite.org/pragma.html#pragma_integrity_check)
