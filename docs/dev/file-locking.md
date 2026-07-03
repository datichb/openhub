# Verrouillage de fichiers — Architecture et utilisation

> Documentation technique du mécanisme de file locking introduit pour protéger les écritures concurrentes dans les fichiers partagés du hub.

---

## Contexte

Plusieurs fichiers du hub peuvent être modifiés par plusieurs invocations simultanées de `oh` (deux terminaux, scripts en parallèle) :

- `projects/projects.md` — registre des projets (~40 sites d'écriture via `perl -i`)
- `projects/api-keys.local.md` — clés API
- `config/hub.json` — configuration globale

Sans protection, deux processus concurrent peuvent :
1. Lire la même version du fichier
2. Écrire leurs modifications chacun de leur côté
3. Le deuxième écrasement détruit les changements du premier

---

## Solution — `scripts/lib/filelock.sh`

### Stratégie multi-OS

| OS | Mécanisme | Disponibilité |
|---|---|---|
| Linux | `flock` (util-linux) | Systeme — toujours présent |
| macOS | `/usr/bin/lockf` (BSD) | Systeme — toujours présent |
| Fallback | `mkdir` atomique + PID file | Universel (POSIX) |

La détection est automatique au chargement de la librairie. Aucune installation requise.

### Lockfiles

Les lockfiles sont créés dans `$HUB_DIR/.locks/` :

| Nom logique | Fichier lockfile | Protège |
|---|---|---|
| `projects` | `.locks/projects.lock` | `projects.md` |
| `api-keys` | `.locks/api-keys.lock` | `api-keys.local.md` |
| `hub` | `.locks/hub.lock` | `hub.json` |

---

## API

### `_acquire_lock <nom> [timeout_secondes]`

Acquiert un verrou exclusif. Bloque jusqu'à ce que le verrou soit disponible ou que le timeout expire.

```bash
_acquire_lock "projects" 10 || { log_error "timeout"; exit 1; }
# ... opérations sur projects.md ...
_release_lock "projects"
```

- `nom` : identifiant logique du verrou (ex: `"projects"`, `"api-keys"`, `"hub"`)
- `timeout_secondes` : délai maximum d'attente (défaut: 10)
- Retourne 0 si le verrou est acquis, 1 si timeout

### `_release_lock <nom>`

Libère le verrou précédemment acquis.

```bash
_release_lock "projects"
```

### `_with_lock <nom> [timeout] -- <fonction> [args...]`

Exécute une fonction sous verrou, garantit la libération même si la fonction échoue.

```bash
_with_lock "projects" 10 -- _ma_fonction_ecriture "arg1" "arg2"
```

---

## Implémentation dans `project.sh`

Le fichier `scripts/lib/project.sh` expose `_do_locked_projects_write` comme wrapper centralisé. Les scripts de commandes (`cmd-*.sh`) l'utilisent pour protéger leurs écritures directes sur `projects.md` :

```bash
# Dans cmd-init.sh — append d'un bloc projet
_acquire_lock "${OH_LOCK_PROJECTS:-projects}" 10 || exit 1
cat >> "$PROJECTS_FILE" <<EOF
## $PROJECT_ID
...
EOF
_release_lock "${OH_LOCK_PROJECTS:-projects}"

# Dans cmd-remove.sh — suppression d'un bloc projet
_acquire_lock "${OH_LOCK_PROJECTS:-projects}" 10 || exit 1
perl -i.bak -0pe '...' "$PROJECTS_FILE"
_release_lock "${OH_LOCK_PROJECTS:-projects}"
```

**Périmètre actuel du locking :**
- `cmd-init.sh` — append d'un nouveau projet
- `cmd-remove.sh` — suppression d'un bloc projet
- `cmd-project.sh` — renommage d'un projet (projects.md et api-keys.local.md)

**Note :** Les fonctions `_set_project_*` (appelées par `oh agent`, `oh beads`, etc.) peuvent être wrappées via `_do_locked_projects_write` si le besoin de protection en accès concurrent se confirme sur ces chemins d'écriture.

```
Appelant (cmd-init.sh)
  └─ _acquire_lock "projects" 10
  └─ cat >> "$PROJECTS_FILE"  ← écriture protégée
  └─ _release_lock "projects"
```

---

## Compatibilité bash 3.2

- Utilise des numéros de FD fixes (`exec 9>file`) — jamais `{fd}>file` (bash 4.1+)
- La détection `_detect_lock_method` n'utilise pas de tableaux associatifs
- `flock` et `lockf` supportent la syntaxe `exec N>file ; flock N` sur bash 3.2+

---

## Comportement sur NFS

Le fallback `mkdir` est atomique sur les filesystems locaux et NFSv4+. Sur NFSv2/v3, l'atomicité n'est pas garantie. Les lockfiles sont dans `$HUB_DIR` qui est typiquement un filesystem local.

---

## Tests

```bash
# Tests unitaires de filelock
bats tests/test_lib_filelock.bats

# Tests de concurrence (incluent les écritures concurrentes)
bats tests/test_concurrency_sessions.bats
```
