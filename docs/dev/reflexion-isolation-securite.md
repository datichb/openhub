# Réflexion : isolation des sessions et chaîne d'approvisionnement IA

> **Date** : 2026-11-06 (2e passe le même jour : relecture du code réseau d'oh, documentation de Docker Sandboxes, gabarits de conversation des moteurs d'inférence, état du correctif CPython)
> **Statut** : brouillon de réflexion, rien n'est décidé. Chaque axe retenu passera par un ticket, et par un ADR quand il change le modèle de sécurité.
> **Origine** : comparaison du runtime conteneur d'oh avec Docker Sandboxes (`sbx`) et le `devex-starter-kit` d'OCTO ; article RunSybil du 2026-11-03 sur une évasion du bac à sable Jinja par un bug de CPython.

Niveau de confiance : **Code** (lu dans le dépôt), **Doc** (documentation officielle de l'outil), **À confirmer** (à prouver par un prototype).

---

## 1. Contexte

### 1.1 Ce que fait oh aujourd'hui (`--runtime container`)

- Conteneur OCI piloté par la CLI du moteur (Colima, Podman, Docker ; `cli/internal/runtime/container/engine.go`), image `oh-base/<projet>` (Dockerfile de dev) + couche `oh-dev/<projet>` (opencode figé par intégrité, faux `bd`).
- Confinement : `--cap-drop=ALL`, `no-new-privileges`, `--pids-limit=4096`, UID de l'utilisateur, `--rm --init`, `HOME` du groupe vidé à chaque démarrage (S-33), environnement vide passé par `--env-file` (`spec.go:223-276`).
- Fichiers git qui exécutent du code en lecture seule (`.git/config`, `.git/hooks`, `info/attributes`, liens de worktree).
- Aucun secret dans le conteneur : jeton `ohs_…` échangé par le proxy d'identifiants ; Beads et MCP d'oh derrière des passerelles sur la machine.
- Port publié sur `127.0.0.1` seulement.

### 1.2 Le réseau aujourd'hui (Code)

- **Aucune option réseau** sur `run` : réseau par défaut du moteur (pont Docker ou Colima ; `pasta`/`slirp4netns` pour Podman sans root). Pas de `--dns`, rien sur IPv6 (`spec.go:223-276`). Même chose pour les conteneurs de sonde (`runtime.go:208`, `doctor.go:85-162`).
- **Un seul point de contact avec la machine** : le port du proxy du démon, joint par `host.docker.internal` / `host.containers.internal` (`engine.go:38-41`), plus une seconde écoute sur l'adresse du pont sous Linux, vérifiée par `hostnet.CheckListenHost` (`hostnet.go:28-69`). Fournisseur LLM, hooks, MCP d'oh et passerelle Beads passent tous par lui (`credproxy/proxy.go:187,259`, `beadswire/wire.go:52`).
- **Le trafic LLM sort déjà par le démon**, vers des hôtes amont figés et en HTTPS (`daemon.go:657-669`, `credproxy/region.go:72-98`). Le conteneur n'a besoin d'aucune sortie directe vers un fournisseur.
- **Aucune variable `HTTP(S)_PROXY`** n'est passée au conteneur ni au job distant (`envfilter.go:10-11`, `runsvc/env.go:151`, `remote/runner/runtime.go:85-91`). Le démon, lui, respecte celles de son propre environnement (`proxy.go:218`).
- **Construction d'image** : `docker build` sur la machine, réseau ouvert (`image.go:245-280`). Distant : Kaniko ou DinD, réseau ouvert (`ciconfig.go:132-157`).
- **Mode distant** : aucune restriction réseau dans le job `oh-run`.

### 1.3 Comparaison

| Axe | oh | Docker sbx | devex-starter-kit |
|---|---|---|---|
| Frontière | Conteneur (noyau de la VM du moteur partagé) | microVM par sandbox | Conteneur |
| Privilèges | Aucune capability, pas de sudo | Utilisateur `agent` **avec sudo** (Doc) | Non-root, garde `NET_ADMIN`/`NET_RAW` |
| Clé LLM | Jamais dans le conteneur (proxy) | Jamais dans la VM (proxy, en-têtes) | **En clair dans l'environnement** |
| Réseau sortant | **Aucun filtrage** | Refus par défaut, règles par domaine, DNS filtré, proxy HTTP avec CA propre (Doc) | iptables par IP, avec trous (SSH et DNS vers tout, GitHub entier, /24 de la passerelle, pas d'IPv6) |
| Espace de travail | Lecture-écriture, `.git/*` sensibles en lecture seule | Lecture-écriture ou `--clone` (refusé depuis un worktree secondaire, Doc) | Lecture-écriture, hooks compris |
| Persistance | Aucune (`--rm`, `HOME` vidé) | VM persistante jusqu'à `sbx rm` | Volumes persistants |
| Plateformes | macOS, Linux | macOS 14+ Apple Silicon, Windows 11, Ubuntu 24.04+ seulement (Doc) | macOS, Linux, Windows |

### 1.4 Article RunSybil (CPython / Jinja)

- Lecture hors limites dans `_Py_subs_parameters` : évasion de `ImmutableSandboxedEnvironment` de Jinja, exécution de code dans vLLM via un `chat_template.jinja` piégé.
- **Correctif** (PR CPython 155761, fusionnée le 2026-08-14) : 3.13.16, 3.14.8, 3.15 et 3.16. **Pas de report sur 3.10 à 3.12**. Pas de CVE (CPython ne considère pas l'évasion d'un bac à sable comme une vulnérabilité). Aucun avis de vLLM, Hugging Face ni Jinja à ce jour.
- **Moteurs concernés** (exécutent le gabarit en CPython) : vLLM, SGLang, transformers (`apply_chat_template`), LiteLLM (`hf_chat_template`, gabarit téléchargé depuis Hugging Face pour plusieurs fournisseurs). **Non concernés** : TGI (minijinja, Rust), Ollama (Go), llama.cpp (C++).
- Leçon : les bacs à sable de langage sont un effort de bonne volonté, pas une barrière.

---

## 2. Axes de réflexion

Priorité indicative : **P1** à traiter en premier, **P2** ensuite, **P3** à étudier. Effort : S (jours), M (une à deux semaines), L (plus).

### A. Filtrage du réseau sortant des conteneurs — P1, M à L, ADR

**Constat** : un agent (piloté par une injection de prompt ou par un modèle compromis) peut envoyer le contenu du dépôt n'importe où. C'est l'écart principal avec sbx.

**Options d'architecture** :

| Option | Principe | Pour | Contre |
|---|---|---|---|
| A1. Proxy sortant dans le démon | Réseau interne au moteur ; le conteneur ne joint que le démon, qui relaie vers une liste blanche | Un seul point de contact, déjà en place ; politique et journal au même endroit que le proxy d'identifiants | Joignabilité de la machine depuis un réseau interne **non prouvée** sur Colima, Docker Desktop et Podman machine ; sous Linux, l'adresse du pont interne doit passer `CheckListenHost` |
| A2. Conteneur proxy compagnon | Un conteneur proxy (smokescreen, Squid, Envoy) branché sur le réseau interne et sur un réseau normal | Indépendant du moteur, motif connu, pas de `NET_ADMIN` dans le conteneur de l'agent | Un conteneur de plus par groupe ; il doit aussi relayer vers le démon ; image tierce à figer |
| A3. Pare-feu iptables dans le conteneur | Modèle du starter-kit et du devcontainer d'Anthropic | — | Demande `NET_ADMIN`, filtrage par IP fragile, trous constatés : écarté |

**Points de réflexion** :
- **Les outils qui ignorent le proxy** échouent (c'est voulu avec un réseau interne). Node `fetch` et `http` ne lisent `HTTPS_PROXY` qu'avec `NODE_USE_ENV_PROXY=1` (Node 24+) ; le comportement de Bun (opencode) est à vérifier ; curl ignore `HTTP_PROXY` en majuscules ; Yarn Berry a ses propres réglages. Il faudra une liste testée.
- **`NO_PROXY`** doit couvrir l'adresse du démon ; les variables doivent atteindre le serveur opencode et les shells des agents, qui ne reçoivent en conteneur que l'environnement statique (`runsvc/env.go:151`).
- **DNS** : sans résolveur externe dans le conteneur, la résolution se fait côté proxy (évite le tunnel DNS). Vérifier le comportement DNS des réseaux internes selon la version du moteur (CVE-2024-29018 corrigée en Docker 26).
- **IPv6** : à couper explicitement.
- **Liste blanche** : où la déclarer (workflow `network.allow` durci par couche comme `runtime.allowed`, projet, team-state imposé sur le modèle de `url_enforced`) ; liste par défaut (registres de paquets, forge de l'équipe) ; le filtrage par domaine n'empêche pas un `git push` vers un dépôt personnel sur la même forge.
- **TLS** : filtrage sur le nom (CONNECT, SNI) sans inspection, ou CA propre comme sbx ? L'inspection permettrait de filtrer par chemin (dépôts de la forge) mais ajoute une CA dans l'image.
- **Ce qui ouvre ses propres connexions dans le conteneur** : serveurs MCP tiers (`remote` ou locaux), `webfetch`/`websearch` d'opencode, gestionnaires de paquets, `git`, appels d'opencode lui-même (catalogue de modèles ? ; mise à jour automatique déjà coupée, `server.go:110`).
- **Construction** : réseau ouvert au `build`, filtré au `run` (commandes déjà séparées).
- **Sondes Doctor** : elles doivent tourner sur le même réseau que les sessions pour tester le vrai chemin.
- **Distant** : règle équivalente dans le `.gitlab-ci.yml` généré ou dans `oh runner` ; dépend des runners de l'équipe.
- **Promesse** : que dit-on dans `SECURITY.md` après coup ? « Pas d'exfiltration hors de la liste blanche », pas « pas d'exfiltration ».

### B. Mode « clone » de l'espace de travail — P2, M, ADR

**Constat** : en lecture-écriture direct, les fichiers exécutés plus tard sur la machine (`Makefile`, scripts `package.json`, `.vscode/tasks.json`, CI, config des outils IA) restent modifiables. Les `.git/*` sensibles sont déjà protégés.

**Piste** : dépôt monté en lecture seule, travail dans un clone ou un worktree privé, ramené par oh (branche, diff relu au checkpoint). sbx fait la même chose avec un démon git local.

**Points de réflexion** : coût sur le fan-out et les vérifications (`verify:`) ; reprise d'une session en veille ; transfert local ↔ conteneur ; fichiers non suivis (`.env`) toujours lisibles dans le clone ; articulation avec les worktrees d'oh. À trancher après A : une fois le réseau fermé, le risque résiduel est surtout ce qui s'exécute plus tard sur la machine, que les checkpoints montrent déjà dans le diff.

### C. Ce qu'on retient de Docker Sandboxes (inspiration, pas dépendance) — alimente A, B et H

sbx n'est **pas** envisagé comme moteur : c'est un modèle de référence. Ce qu'on peut reprendre dans notre propre runtime :

| Idée de sbx | Chez oh aujourd'hui | Ce qu'on en ferait |
|---|---|---|
| Sortie refusée par défaut, tout le TCP passe par un proxy de la machine | Rien | Cœur de A (option A1 : le démon joue ce rôle) |
| DNS résolu par le résolveur du sandbox, qui applique la politique ; UDP coupé, ICMP bloqué | Rien | Pas de résolveur externe dans le conteneur ; résolution côté proxy ; IPv6 coupé |
| Préréglages `allow-all` / `balanced` / `deny-all` (`sbx policy init`) | — | Trois niveaux nommés plutôt qu'une liste à écrire de zéro ; `balanced` = registres de paquets + forge + docs courantes |
| Règles globales, ou propres à un sandbox ; jokers `*.x` (un niveau) et `**.x` (tous niveaux), ports, CIDR | — | Syntaxe de `network.allow` ; durcissement par couche (hub → équipe → projet → workflow), comme `runtime.allowed` |
| Gouvernance d'organisation : les règles d'autorisation locales sont ignorées, les refus locaux s'appliquent toujours | `url_enforced`, limites d'équipe | Même sémantique pour le team-state : l'équipe autorise, chacun peut seulement restreindre |
| Filtres par méthode, chemin, requête (avec CA propre) | — | Option d'A pour la forge : autoriser `GET` partout, `push` seulement vers les dépôts du projet. Coût : CA dans l'image, à peser |
| Injection des identifiants dans les en-têtes par le proxy, valeur jamais dans le sandbox | ✅ Proxy d'identifiants (LLM), passerelles (MCP, Beads) | Étendre éventuellement à d'autres en-têtes (registre privé npm, forge en lecture) au lieu de les laisser hors d'atteinte |
| Mode `--clone` : dépôt en lecture seule, clone privé exposé par un démon git local | — | Modèle de B ; éviter sa limite (refus depuis un worktree secondaire) |
| Moteur Docker privé par sandbox | Pas de Docker dans le conteneur | À étudier pour les projets qui ont besoin de testcontainers ou de `docker compose` ; ouvre un vrai chantier (rootless DinD, isolation) |
| microVM par sandbox (noyau dédié) | Conteneur, noyau de la VM du moteur partagé | Hors de portée à court terme ; option lointaine : runtime renforcé (gVisor `runsc`, Kata) quand le moteur le propose |
| Journal des connexions refusées (`sbx policy log`, à vérifier) | — | Indispensable à l'adoption de A : voir ce qui casse et l'ajouter à la liste |

**Ce qu'on ne reprend pas** : sudo dans le sandbox (notre conteneur n'a aucune capability), la VM persistante (nous jetons tout à l'arrêt), le transfert d'agent SSH actif par défaut.

### D. Confiance dans le fournisseur de modèle — P2 (doc), revu après la 2e passe

**Correction de la 1re passe** : oh **ne permet pas** de point d'accès personnalisé. Les hôtes amont sont figés dans le démon (Bedrock par région, Anthropic, OpenRouter, OpenAI ; `daemon.go:657-669`, `region.go:72-98`), et rien dans le team-state ne restreint les fournisseurs. La mention « Bearer token (API Gateway / LiteLLM) » de `docs/guides/configuration-guide.*.md:85` est un libellé trompeur : le jeton va à Bedrock.

**Exposition réelle** :
- Bedrock et Anthropic : modèles fermés ; AWS a corrigé.
- **OpenRouter** : route vers des modèles ouverts servis par des hébergeurs tiers, dont certains utilisent vLLM ou SGLang. Un hébergeur compromis lit les prompts et peut renvoyer des appels d'outils fabriqués.

**Pistes** :
1. Corriger le libellé de `configuration-guide.*.md:85` (S).
2. Section « Confiance dans le fournisseur de modèle » dans `SECURITY.md` : un modèle hostile est couvert par les protections qui ne dépendent pas de lui (checkpoints, passerelles, proxy, conteneur, puis A) ; pour OpenRouter, préférer les fournisseurs de premier rang (réglage de routage d'OpenRouter, à vérifier) (S).
3. Si un jour oh accepte des points d'accès personnalisés : avertissement `oh doctor` et liste imposée par l'équipe. Pas avant.

### E. Skills d'audit et de développement — P1, S

**Constat** : `skills/auditor/audit-security.md` ne vérifie que l'échappement Jinja ; `skills/developer/dev-standards-security-hardening.md` ne cite que `pickle`. Un projet IA avec le motif vulnérable passe l'audit.

**Ajouts** :
- un bac à sable de langage (Jinja `SandboxedEnvironment`, RestrictedPython, `simpleeval`, `asteval`, `eval` à globals restreints) n'est jamais une barrière pour du contenu non fiable : isoler au niveau du système ;
- gabarits de conversation et fichiers de modèle traités comme du code : révision Hugging Face figée, relecture, pas de `trust_remote_code=True` ; LiteLLM télécharge des gabarits depuis la branche `main` de Hugging Face pour plusieurs fournisseurs ;
- CPython 3.10 à 3.12 (ou 3.13 < 3.13.16, 3.14 < 3.14.8) signalé quand la charge rend des gabarits ou évalue des expressions non fiables (aussi dans `audit-infra.md`) ;
- éventuellement une stack « Python IA / inférence » détectée (`transformers`, `vllm`, `sglang`, `litellm` dans les dépendances).

Ne pas oublier les golden tests des prompts (`cli/internal/bundle/testdata/golden`) si les skills changent le rendu.

### F. Doc conteneur — en partie fait

- Fait : `HOME` du conteneur décrit comme volume du groupe vidé à chaque démarrage (commit `2c488340`).
- À faire (P1, S) : écrire dès maintenant la limite « pas de filtrage réseau sortant » dans `SECURITY.md` et `docs/guides/container.*.md`, pour ne pas laisser croire que le conteneur protège de l'exfiltration. Même chose pour le mode distant.

### G. Retour à l'équipe DevEx (starter-kit) — hors dépôt, S

Proposer : clé LLM hors du conteneur (proxy local), fermer SSH et DNS vers tout, restreindre le /24 et GitHub, `ip6tables`, `.git/hooks` en lecture seule, `cap-drop` et `no-new-privileges`, volumes de config non persistants ou relus.

### H. Isolation du mode local — P3, nouveau

**Constat** : le mode local n'est pas un bac à sable (`SECURITY.md`) et c'est le mode par défaut des workflows.

**Piste** : `sandbox-runtime` d'Anthropic (`srt`, préversion) confine un processus sans conteneur : `sandbox-exec` sur macOS, bubblewrap sur Linux, réseau seulement par des proxys locaux avec liste blanche par domaine. On pourrait y lancer le serveur opencode local et ses shells, avec le même modèle de liste blanche que A.

**Points de réflexion** : maturité (préversion) ; `sandbox-exec` est déprécié par Apple ; compatibilité avec la passerelle Beads locale, le socket du démon et les outils du développeur ; mutualiser la politique réseau avec A.

### J. Indications de niveau de sécurité — P1

Le conteneur par défaut est écarté (2026-11-06). Les indications de niveau de sécurité ont leur cadrage détaillé : [cadrage-indications-securite.md](cadrage-indications-securite.md) (facettes, catalogue d'indications, surfaces, enregistrement, découpage en lots L0 à L5).

**Au passage** : commentaire périmé `launch/runtime.go:51` (« the container HOME is a project volume »), à corriger avec F.

### I. Points de contrôle annexes — S, à vérifier

- **`webfetch` quand la recherche web est désactivée** : oh n'ajoute la règle qu'à l'activation (`bundle/build.go:180-185`) ; vérifier que la base du monde fermé refuse bien `webfetch` sinon (les golden de `release` et `merge-conflicts` montrent `webfetch * deny`, ceux de `ticket` `allow`).
- **Appels sortants d'opencode** au démarrage (catalogue de modèles, télémétrie) : à lister, ils compteront dans la liste blanche de A.

---

## 3. Étapes avant le développement

1. **Lot documentaire (sans prototype)** : F (limite réseau écrite), D.1 et D.2, E (skills), I (vérification `webfetch`). Petit, peut partir dès que les changements en cours sont fusionnés.
   - En parallèle, **indications de niveau de sécurité (J)** : voir les lots L0 à L5 du [cadrage](cadrage-indications-securite.md#8-surfaces-et-lots). Indépendant de A ; la facette « Réseau » affichera « ouvert » jusqu'à A.
2. **Prototype A (spike, 3 à 5 jours)** dans un worktree jetable :
   - réseau interne + joignabilité du démon sur Colima (virtiofs), Podman machine, Podman sans root sous Linux, Docker sous Linux ;
   - A1 contre A2 : même scénario (npm install, pip install, go mod download, git clone de la forge, appel LLM par le proxy, `bd`, MCP d'oh) ;
   - liste des outils qui respectent `HTTPS_PROXY` dans notre image (Bun/opencode en priorité) ;
   - tentatives d'exfiltration : domaine hors liste, DNS, IPv6, IP directe, `git push` vers un autre dépôt ;
   - coût : temps de démarrage, mémoire du proxy compagnon.
3. **ADR « Réseau sortant des sessions »** à partir du spike, en reprenant les idées de C retenues (préréglages, syntaxe des règles, sémantique de gouvernance, journal des refus) : option retenue, emplacement et durcissement de la liste blanche, liste par défaut, comportement du distant, promesse de `SECURITY.md`, tests (les tests `container` ne tournent pas en CI, `.github/workflows/ci.yml:159` : décider d'un job dédié ou d'une procédure de release).
4. **Décisions reportées après l'ADR** : B (mode clone), H (mode local), moteur Docker privé et runtime renforcé (voir C).
5. **En parallèle** : G (retour DevEx), recensement des équipes qui utilisent OpenRouter avec des modèles ouverts.

## 4. Questions ouvertes transverses

- Quel niveau de garantie annonce-t-on pour le conteneur après A : « isolation du système de fichiers et des secrets » ou aussi « sortie limitée à une liste blanche » ?
- Faut-il étendre `execution.strict_isolation` (réseau filtré obligatoire, puis mode clone) plutôt que d'ajouter des options séparées ?
- La liste blanche réseau est-elle une propriété du workflow (comme `beads.allow`), du projet, ou d'abord de l'équipe ?
- Réseau filtré par défaut pour tous les conteneurs, ou seulement pour les workflows qui le demandent, le temps de constituer la liste par défaut ?

## 5. Sources

- Code : `cli/internal/runtime/container/` (`engine.go`, `spec.go`, `image.go`, `runtime.go`, `doctor.go`), `cli/internal/credproxy/`, `cli/internal/hostnet/`, `cli/internal/daemon/daemon.go`, `cli/internal/runsvc/`, `cli/internal/services/remote/`, `cli/internal/bundle/build.go` ; `SECURITY.md`, `docs/guides/container.*.md`, ADR-045.
- Docker Sandboxes : <https://docs.docker.com/ai/sandboxes/>, `/security/`, `/security/defaults/`, `/security/isolation/`, `/install/`, `/usage/`, `/configuration/credentials/`, `/workflows/automation/`, <https://docs.docker.com/ai/governance/local/network-rules/>.
- Réseaux internes et DNS : <https://docs.docker.com/engine/release-notes/28/>, <https://github.com/moby/moby/security/advisories/GHSA-mq39-4gv4-mvpx>, <https://blog.podman.io/2024/10/podman-5-3-changes-for-improved-networking-experience-with-pasta/>.
- Proxy sortant : <https://github.com/stripe/smokescreen>, <https://github.com/anthropics/sandbox-runtime>, <https://nodejs.org/en/learn/http/enterprise-network-configuration>.
- devex-starter-kit : <https://gitlab.octo.tools/tous-les-octos/devex/devex-starter-kit> (`main`, `a9d5e78`) ; devcontainer d'Anthropic : <https://github.com/anthropics/claude-code/tree/main/.devcontainer>.
- Gabarits de conversation : LiteLLM `litellm/litellm_core_utils/prompt_templates/factory.py` (commit `bbc7de1`) ; TGI (minijinja), Ollama (`template/template.go`), llama.cpp (PR 18462).
- RunSybil, 2026-11-03 : <https://www.runsybil.com/blog/cpython-0-day-vulnerabilities-threaten-the-ai-supply-chain> ; correctif : <https://github.com/python/cpython/pull/155761>.
