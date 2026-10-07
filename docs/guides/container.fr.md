> [Read in English](container.en.md)

# Exécuter une session dans un conteneur

Avec opencode V2, un workflow qui l'autorise (`runtime.allowed` contient `container`, par exemple `ticket`) peut tourner dans un **conteneur** construit depuis le Dockerfile de développement du projet. Les commandes shell de l'agent s'exécutent alors dans le conteneur. Vos secrets, Beads et les serveurs MCP d'oh restent sur la machine.

```bash
oh run ticket --tickets bd-42 --runtime container
```

## Prérequis

- macOS ou Linux (Windows : local uniquement).
- Un moteur de conteneurs : **Colima** (runtime `docker`), **Podman** (machine démarrée) ou le CLI **Docker**. oh utilise le CLI du moteur, pas d'API Docker.
- Le projet et ses worktrees doivent être dans un dossier partagé avec la VM : sous `$HOME` pour Colima. `$TMPDIR` (`/var/folders/…`) n'est pas partagé avec Colima.

## Configurer le projet

TUI : **Config projet › Exécution**. Les réglages sont enregistrés dans la base d'oh. oh n'écrit rien dans le dépôt.

| Réglage | Rôle | Par défaut |
|---|---|---|
| Dockerfile de dev | Image de base du conteneur. Chemin relatif au projet ou absolu. | Détecté : `Dockerfile.dev`, `dev.Dockerfile`, `.devcontainer/Dockerfile`, `Dockerfile`. Sans fichier : image oh (`debian:bookworm-slim` + `git`, `ca-certificates`, `ripgrep`). |
| Build args | Arguments de construction : `CLÉ=valeur, CLÉ2=valeur` | aucun |
| Volumes de cache | Volumes persistants, séparés par des virgules. Un chemin relatif s'applique à chaque emplacement monté (`node_modules`) ; s'il n'existe pas dans le projet, le moteur crée un dossier vide à cet endroit sur la machine (point de montage). Un chemin absolu est un chemin du conteneur (`/root/.cache`). | aucun |
| Workflow par défaut | Lancé par `oh run` sans argument. Apparaît en tête de « Démarrer » (◆) et des actions du board. | aucun |
| Runtime par défaut | Runtime préféré du projet, utilisé si le workflow l'autorise | Réglages, puis défaut du workflow |

Le Dockerfile est toujours cherché dans le **dossier du projet**, même quand la session tourne dans un worktree. C'est aussi vrai à la reprise d'une session en veille : la reprise relit la config du projet. Si elle a changé, l'image est reconstruite. Les données de la session sont conservées.

### Choix du runtime

Du plus prioritaire au moins prioritaire :

1. `--runtime` ou le choix fait dans la fiche de lancement ;
2. le runtime par défaut du projet ;
3. le runtime par défaut des Réglages ;
4. `runtime.default` du workflow.

Un runtime que le workflow n'autorise pas (`runtime.allowed`) est ignoré.

### Dans la fiche de lancement

L'option **▣ conteneur** indique la disponibilité du moteur. Si la VM est arrêtée ou si la version d'opencode figée ne correspond pas, l'option est marquée ✗ avec la raison et le lancement est refusé. Quand l'option est choisie, une ligne précise l'état :

- `colima 28.1 · mount=virtiofs ✔ · image oh-dev/<projet>:9f3c… en cache` : rien à construire ;
- `couche oh à reconstruire (~40 s)` : seule la couche oh change (par exemple après une mise à jour d'opencode) ;
- `image à construire : environnement du projet + couche oh (~3 min)` : le Dockerfile ou les build args ont changé.

Le temps estimé vient des dernières constructions du projet. Avant la première construction, la ligne indique « plusieurs minutes ». Pendant le lancement, les dernières lignes de la construction s'affichent dans la fiche.

### Identité git

Le `HOME` du conteneur est un volume propre au projet : votre `~/.gitconfig` n'y est pas. Pour que l'agent puisse committer, oh passe au shell de la session (et des sous-agents) votre identité git, lue dans le projet : `GIT_AUTHOR_NAME`, `GIT_AUTHOR_EMAIL`, `GIT_COMMITTER_NAME`, `GIT_COMMITTER_EMAIL`. Aucune autre configuration git n'est copiée.

## Réglages

TUI : **Réglages › Exécution**, ou la section `[execution]` de `~/.oh/hub.toml` :

```toml
[execution]
runtime = "container"        # runtime préféré si le workflow l'autorise et que le projet n'en choisit pas
engine = "colima"            # auto (défaut : Colima, puis Podman, puis Docker) | colima | podman | docker
keep_images = 2              # images gardées par projet et par rôle (base, oh) ; les plus anciennes sont supprimées
tool_version = "2.0.20"  # vide = version du client de la machine
strict_isolation = true      # masque aussi votre config opencode aux serveurs locaux
```

- **Version d'opencode figée** : l'image et le client de la machine doivent avoir la même version. Si la version figée diffère de celle du client (par exemple après une mise à jour Homebrew), les lancements en conteneur sont refusés avec la raison. Réinstallez la version figée ou changez le réglage. Sans version figée, l'image suit le client : une mise à jour d'opencode reconstruit la couche oh au lancement suivant.
- **Isolation stricte** : en local, le serveur reçoit un `XDG_CONFIG_HOME` propre. C'est une copie de votre dossier de configuration sans `opencode/`, avec des liens vers les autres dossiers, donc git, gh… continuent de fonctionner. Les plugins, agents et commandes de votre config opencode globale ne sont plus chargés. En conteneur, cette config n'est jamais visible.
- Le choix du moteur remplace la variable `OH_CONTAINER_ENGINE`, qui n'existe plus.

## Ce qui se passe au lancement

1. **Image** : oh construit une image de base depuis le Dockerfile de dev (`oh-base/<projet>:<hash>`), puis une couche fine (`oh-dev/<projet>:<hash>`). La couche ajoute opencode à la version du client de la machine et le faux `bd`. Les tags dépendent du contenu : une image déjà construite est réutilisée.
2. **Montages** :
   - le projet et les worktrees sous `/work/<nom>`, en lecture-écriture ;
   - le paquet de session sous `/opt/oh/bundle`, en lecture seule ;
   - les données du groupe de serveur sous `/opt/oh/data` ;
   - un `HOME` propre au projet : votre config opencode n'est jamais visible.
3. **Réseau** : le port du serveur n'est publié que sur `127.0.0.1`. Le client opencode tourne sur la machine et s'attache au serveur du conteneur.
4. **Secrets** : aucun secret n'entre dans le conteneur. Le conteneur reçoit un jeton `ohs_…`, que le proxy d'identifiants d'oh échange contre la vraie clé. `bd` passe par la passerelle Beads, avec la liste blanche `beads.allow` du workflow (lecture seule par défaut). Les serveurs MCP d'oh (gitlab, team, workflow…) tournent sur la machine.

## Vérifier avec Doctor

`oh doctor` (ou la vue Doctor de la TUI) vérifie le conteneur et les passerelles :

| Vérification | Ce qui est contrôlé |
|---|---|
| Conteneur : moteur | Moteur détecté, version et détails (`mount=virtiofs`, `rootless=true`). Sans moteur installé, la vérification reste verte : le conteneur est optionnel. |
| Conteneur : version figée | Version figée différente du client de la machine |
| Conteneur : partage de fichiers | Colima : montage `virtiofs` conseillé (`sshfs` et `9p` sont lents) |
| Conteneur : utilisateur (keep-id) | Podman rootless : les fichiers créés dans le conteneur vous appartiennent |
| Conteneur : dossiers partagés avec la VM | Projets et worktrees hors des dossiers partagés : ils apparaîtraient vides |
| Conteneur : image de `<projet>` | `opencode --version` dans les 3 dernières images. Sur une base musl, signale `libstdc++` et `libgcc` manquants. |
| Conteneur : écoute Linux | Linux : adresse de la machine vue des conteneurs (2e écoute du proxy et des passerelles) |
| Passerelle Beads : bd sur la machine | `bd` installé sur la machine (sinon les commandes `bd` de l'agent échouent) |
| Passerelles : démon oh | Démon assez récent pour servir les passerelles |
| Passerelles Beads et MCP : depuis un conteneur | Requête sans jeton depuis un conteneur vers `/oh-gateway/…` et `/oh/v1/hooks/mcp/…` : un 401 prouve que la route est servie à l'adresse utilisée par les conteneurs |

Les sondes lancent quelques conteneurs courts. Elles utilisent l'image `busybox` (téléchargée la première fois) ou la dernière image du projet si elle contient `curl` ou `wget`.

## Dépannage

| Symptôme | Cause probable |
|---|---|
| « conteneur indisponible » dans la fiche | VM arrêtée (`colima start`, `podman machine start`) ou moteur absent |
| « … is not shared with the colima VM » | Projet hors de `$HOME` (Colima) |
| opencode ne démarre pas sur une base Alpine | `libstdc++` et `libgcc` manquants : ajoutez-les au Dockerfile |
