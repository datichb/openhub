# Cadrage : filtrage du réseau sortant des sessions

> **Date** : 2026-11-06
> **Statut** : cadrage partagé en cours, rien n'est développé. Issu de l'axe A de [la réflexion sur l'isolation](reflexion-isolation-securite.md). Les inconnues techniques seront levées par un prototype ; ce cadrage fixe d'abord le comportement attendu, puis ce que le prototype doit prouver.
> **Chemins** : relatifs à `cli/`, état du code au 2026-11-06.

---

## 1. Contexte (Code)

- Aucune option réseau sur `docker`/`podman run` : réseau par défaut du moteur, pas de `--dns`, rien sur IPv6 (`internal/runtime/container/spec.go:223-276`).
- Un seul point de contact avec la machine : le port du proxy du démon (fournisseur LLM, hooks, MCP d'oh, passerelle Beads), joint par `host.docker.internal` / `host.containers.internal`, plus une seconde écoute sur le pont sous Linux (`internal/hostnet/hostnet.go:28-69`).
- Le trafic LLM sort déjà par le démon, vers des hôtes figés en HTTPS (`internal/daemon/daemon.go:657-669`, `internal/credproxy/region.go:72-98`).
- Aucune variable `HTTP(S)_PROXY` passée au conteneur ni au job distant ; le démon respecte celles de son propre environnement (`internal/credproxy/proxy.go:218`).
- Construction d'image et job distant : réseau ouvert.

Détails et options d'architecture (A1 proxy dans le démon, A2 conteneur proxy compagnon, A3 iptables écarté) : [réflexion, axe A](reflexion-isolation-securite.md#a-filtrage-du-réseau-sortant-des-conteneurs--p1-m-à-l-adr).

## 2. Journal du cadrage partagé

### Étape 1 — Menaces, périmètre, comportement par défaut (2026-11-06)

| Question | Décision |
|---|---|
| Menaces couvertes | **Les trois** : exfiltration (envoi du dépôt ou de données hors de la liste), récupération de code (limiter d'où l'agent télécharge et exécute), mouvement latéral (réseau local, adresses privées, métadonnées cloud) |
| Runtimes | **Conteneur d'abord, distant ensuite** ; local hors périmètre (axe H, `sandbox-runtime`) |
| Comportement par défaut | **Progressif** : journaliser d'abord (rien ne casse), puis refus par défaut quand la liste par défaut est éprouvée |

Limite assumée dès maintenant : la récupération de code n'est limitée qu'à la provenance ; un paquet piégé publié sur un registre autorisé (npm, PyPI) passe.

### Étape 2 — Forme des règles (2026-11-06)

| Question | Décision |
|---|---|
| Liste de destinations | **Trois niveaux prédéfinis** : `deny-all` (rien ne sort sauf le démon oh), `balanced` (registres de paquets, forge de l'équipe, documentations courantes), `allow-all` (tout, journalisé) ; **plus des ajouts par domaine** (`exemple.com`, `*.exemple.com` = un niveau, `**.exemple.com` = tous niveaux, port optionnel) |
| Destinations internes | **Refus** des adresses privées (`10/8`, `172.16/12`, `192.168/16`), boucle, lien local et métadonnées (`169.254.169.254`), IPv6 local, **contrôlé sur l'adresse résolue** (protège du *DNS rebinding*) ; **exceptions nommées** possibles (équipe ou projet) pour une forge ou un registre interne. Le démon oh n'est pas une destination : c'est le chemin. |
| Forge et `git push` | **Accepter et documenter** : filtrage par domaine seulement ; un push vers un autre dépôt de la même forge reste possible (limité par l'absence d'identifiants git dans le conteneur) ; filtrage par chemin (inspection TLS) plus tard |

### Étape 3 — Gouvernance de la liste (2026-11-06)

| Question | Décision |
|---|---|
| Modèle | **Le projet ajoute dans un cadre fixé par l'équipe** : l'équipe choisit le niveau et peut **interdire** des destinations ou des motifs (non rouvrables) ; le projet ajoute ses domaines dans ce cadre, chaque ajout est **visible** (facette Réseau, `oh doctor`, journal) ; les refus internes ne se lèvent que par une exception nommée par l'équipe |
| Workflow | **Durcir seulement** (ex. `review` en `deny-all`), jamais ouvrir ; même sémantique que `runtime.allowed` |
| Sans équipe | **Les Réglages jouent le rôle de l'équipe** (niveau, interdictions, exceptions internes) |

Règle de sécurité transverse : la liste d'un projet vit dans la base d'oh (config Exécution du projet), **jamais dans le dépôt** ; elle se modifie par la TUI ou `oh config`, déjà refusé aux agents par le garde-fou global.

### Étape 4 — Mise en place progressive et visibilité (2026-11-06)

| Question | Décision |
|---|---|
| Bascule vers le refus | **Par équipe d'abord** (ou utilisateur sans équipe), aidée d'une suggestion (ex. « aucune sortie hors liste depuis 2 semaines ») ; **une version future** rend le refus par défaut |
| Journal des connexions hors liste | **Partout** : vue de la session, `oh doctor` (résumé récent), action TUI « ajouter ce domaine au projet » depuis le journal (humaine, hors d'atteinte des agents), lignes `network` dans le flux `oh run --json` |
| Réponse à l'agent | **Refus expliqué** : HTTP 403 avec un message clair (« destination non autorisée par oh : demandez à l'utilisateur de l'ajouter ») ; **consigne** dans les skills partagées : ne pas contourner un refus réseau |
| Facette Réseau | **Trois valeurs** : `logged` (« journalisé, non filtré ») pendant la phase de journal, `allowlist` en refus, `open` hors conteneur |

Contenu d'une entrée de journal : session, domaine (et adresse résolue), port, date, décision (`would_deny`, `denied`, `allowed_by_project`). Le cadrage des indications reçoit une valeur de plus pour la facette Réseau (`logged`).

### Étape 5 — Cas particuliers (2026-11-06)

| Question | Décision |
|---|---|
| Construction de l'image | **Ouverte, documentée** : le Dockerfile du projet s'exécute sans agent ; la limite est écrite (`SECURITY.md`, guide conteneur) |
| Vérifications des checkpoints (`verify:`) | **Même politique que la session** (elles exécutent le code modifié par l'agent) |
| Autres protocoles | **HTTP(S) seulement en refus** : le réseau interne n'a pas de route vers l'extérieur, tout le reste est refusé (git par HTTPS ; pas de clés SSH dans le conteneur de toute façon) ; en phase de journal, ces connexions passent et sont journalisées si l'option retenue le permet (à prouver au prototype) |
| Recherche web | **Domaines techniques seulement** : l'activer ajoute les domaines du service de recherche ; `webfetch` d'une page reste soumis à la liste |
| Liste modifiée en cours de session | Objectif : **rechargement à chaud** par le proxy (un domaine ajouté depuis le journal sert tout de suite) ; à vérifier au prototype |

### Étape 6 — Contenu du niveau `balanced` (2026-11-06)

| Question | Décision |
|---|---|
| Registres de paquets | **Selon la stack détectée** du projet (`bricks.DetectSignals`) : npm pour Node, PyPI pour Python, proxy Go, crates.io, Maven Central… |
| Forge de l'équipe | **Incluse** : celle que connaît oh (tracker, MCP GitLab ou GitHub) |
| GitHub | **Seulement si détecté** dans les manifestes du projet (dépendances qui viennent de GitHub) ; sinon à ajouter |
| Documentations | **Hors de `balanced`** ; ajoutées depuis le journal au besoin |

Conséquences : `balanced` n'est pas une liste fixe mais **calculée par projet** (stack + forge) ; elle doit donc apparaître en clair dans le détail de la facette Réseau et dans `oh doctor`, pour que l'utilisateur sache ce qui est ouvert. Un changement de stack (nouveau manifeste) change la liste au lancement suivant.

Fin de la partie « quoi » du filtrage réseau.
