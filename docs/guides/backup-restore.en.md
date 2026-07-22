# Backup & Restore - Guide

> 🇫🇷 [Lire en français](backup-restore.fr.md)

## Overview

`oh export` and `oh import` protect your `oh` configuration and project data from accidental loss, machine failure, or database corruption. Backups are self-contained, checksum-verified archives that can restore a full `oh` installation on a new machine.

---

## What Is Backed Up

A backup archive contains three files:

| File | Description |
|------|-------------|
| `oh.db` | SQLite database — all registered projects, sessions, claims, and events |
| `hub.toml` | Hub configuration (MCP servers, integrations, team config) |
| `secrets.enc` | Encrypted secrets store (tokens, credentials) — already AES-256 encrypted at rest |

> **Note:** Secrets are backed up in their encrypted form. You need your master passphrase to decrypt them after restore. The backup itself does not contain the passphrase.

---

## Creating a Backup

### Default backup

```bash
oh export
```

Creates `oh-backup-<timestamp>.tar.gz` in the current directory.

### Custom output path

```bash
oh export --output my-backup.tar.gz
oh export --output /mnt/backups/oh-$(date +%Y%m%d).tar.gz
```

### What happens

1. `oh` flushes any pending writes to `oh.db`
2. The three files are collected from `~/.oh/`
3. A `manifest.json` is generated with SHA-256 checksums for each file
4. All files are packed into a `.tar.gz` archive

---

## Backup Contents

```
oh-backup-2026-07-22T09-00-00.tar.gz
├── manifest.json      # Checksums + file list + oh version
├── oh.db              # Projects, sessions, claims, events
├── hub.toml           # Hub configuration
└── secrets.enc        # Encrypted secrets
```

`manifest.json` example:

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

## Restoring from Backup

### Standard restore

```bash
oh import my-backup.tar.gz
```

This will:
1. Verify the archive checksum against `manifest.json` (rejects corrupted archives)
2. Check for version compatibility (warns if `oh_version` differs from current)
3. Prompt for confirmation before overwriting existing files
4. Extract files to `~/.oh/`
5. Run a quick integrity check on the restored `oh.db`

### Overwrite without prompt

```bash
oh import my-backup.tar.gz --overwrite
```

### Restore to a custom directory

```bash
oh import my-backup.tar.gz --dir /tmp/oh-restore
```

---

## Checksum Verification

Every `oh import` automatically verifies the SHA-256 checksum of each file against `manifest.json`. If any file is corrupted:

```
Error: checksum mismatch for oh.db (expected abc123..., got xyz789...)
Backup archive may be corrupted. Aborting restore.
```

To verify a backup without restoring:

```bash
oh import my-backup.tar.gz --verify-only
```

---

## Diagnosing DB Issues

If `oh.db` is corrupt or behaving unexpectedly, use `oh repair`:

### Check-only mode (non-destructive)

```bash
oh repair --check-only
```

Runs SQLite integrity checks and reports any issues without modifying the database:
- Page integrity (`PRAGMA integrity_check`)
- Foreign key violations (`PRAGMA foreign_key_check`)
- Index consistency

### Repair mode

```bash
oh repair
```

Attempts to recover the database by:
1. Running `VACUUM` to defragment and rebuild
2. Re-creating damaged indexes
3. Removing orphaned records

> `oh repair` modifies the database. **Create a backup first** with `oh export` before running it.

---

## Recovery Options

| Scenario | Recommended action |
|----------|--------------------|
| Minor DB corruption | `oh repair` |
| Severe DB corruption | `oh import` from latest backup |
| Lost `hub.toml` | `oh import` (restore config only with `--files hub.toml`) |
| New machine setup | Full restore with `oh import` |
| Secrets lost | `oh import` + re-enter master passphrase |

### Restore a single file

```bash
# Restore only hub.toml from backup
oh import my-backup.tar.gz --files hub.toml
```

---

## Disaster Recovery Workflow

Complete step-by-step recovery on a new machine:

```bash
# 1. Install oh on the new machine
curl -sSf https://install.oh.dev | sh

# 2. Copy your backup archive to the new machine
scp old-machine:~/oh-backup-latest.tar.gz .

# 3. Restore from backup
oh import oh-backup-latest.tar.gz

# 4. Re-enter your master passphrase (to decrypt secrets.enc)
oh secrets unlock

# 5. Verify everything is working
oh doctor

# 6. Redeploy to all projects
oh sync --all
```

`oh doctor` checks:
- Database integrity
- Secrets store accessibility
- All registered projects exist on disk
- MCP server configurations are valid

---

## Automation — Daily Backups

Add a cron job to back up `oh` daily:

```bash
# Edit crontab
crontab -e
```

```cron
# Daily oh backup at 2:00 AM, keep last 30 days
0 2 * * * oh export --output /mnt/backups/oh-$(date +\%Y\%m\%d).tar.gz && \
  find /mnt/backups -name "oh-*.tar.gz" -mtime +30 -delete
```

### With verification

```bash
# Verify the backup immediately after creation
0 2 * * * oh export --output /tmp/oh-daily.tar.gz && \
  oh import /tmp/oh-daily.tar.gz --verify-only && \
  mv /tmp/oh-daily.tar.gz /mnt/backups/oh-$(date +\%Y\%m\%d).tar.gz
```

### Backup to a remote location

```bash
# Export and sync to S3
0 2 * * * oh export --output /tmp/oh-backup.tar.gz && \
  aws s3 cp /tmp/oh-backup.tar.gz s3://my-backups/oh/oh-$(date +\%Y\%m\%d).tar.gz && \
  rm /tmp/oh-backup.tar.gz
```

---

## Resources

- [Web Dashboard guide](./dashboard.en.md) — monitoring sessions and metrics
- [`oh export` / `oh import` CLI Reference](../reference/backup.en.md)
- [SQLite integrity checks](https://www.sqlite.org/pragma.html#pragma_integrity_check)
