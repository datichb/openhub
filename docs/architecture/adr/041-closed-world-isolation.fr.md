> [Read in English](041-closed-world-isolation.en.md)

# ADR-041 — Monde fermé et vérification d'isolation

## Statut

Accepté

## Date

2026-10-05

## Contexte

Jusqu'à la v4, une session voyait :

- tous les agents du hub déployés dans le projet ;
- les agents natifs d'opencode (`build`, `plan`, `general`, `explore`) — les masquer était une option (`[deploy].disable_native_agents`) ;
- les skills intégrées (`opencode`, `report`) ;
- tout ce que l'utilisateur avait dans `~/.config/opencode`.

Le modèle pouvait appeler n'importe quel agent, et rien ne vérifiait ce qui lui était réellement présenté.

Constats sur opencode 2.0.20 :

- une config en ligne qui désactive les natifs et ne déclare que nos agents et skills ferme le monde, sauf les deux skills intégrées, à refuser explicitement (F5) ;
- `~/.config/opencode` reste chargé (F5) ;
- les permissions **de session** masquent les skills mais **pas les sous-agents**, seules les règles d'agent le font (F21) ;
- `GET /api/skill` liste toutes les skills sans tenir compte des permissions (F28).

Décision D13 : le monde fermé est **obligatoire et non configurable**, mis en œuvre et **vérifié** par chaque adaptateur ; si la vérification échoue, la session ne démarre pas.

## Décision

### 1. Mise en œuvre (adaptateur `opencodev2`)

- **Agents natifs désactivés** : leur liste est découverte au premier démarrage (agents listés hors paquet), mise en cache par version, avec un défaut `build`, `plan`, `general`, `explore`. Les agents système cachés (`compaction`, `title`, `summary`) sont tolérés.
- **Skills** : `skill` refusé sur `*`, puis autorisé pour les skills du paquet seulement (cela cache `opencode` et `report`).
- **Sous-agents** : pour chaque agent, `subagent` refusé sur `*`, puis autorisé vers les cibles du graphe du workflow.
- **Config et données du projet exclues** : `OPENCODE_DISABLE_PROJECT_CONFIG=1`, et un `XDG_DATA_HOME` propre au groupe de serveur.
- **Plugin oh** : `agent.transform` et `skill.transform` retirent tout agent ou skill hors paquet. C'est une deuxième barrière, en plus des règles.
- **Lecture du paquet** : `external_directory` est autorisé sur le dossier des skills du paquet, dont les fichiers sont en lecture seule.
- **Code Mode** : `execute` est refusé quand `code_mode` n'est pas activé.

### 2. Vérification : `Attest`

`Attest` s'exécute après chaque démarrage de serveur et pour chaque nouveau dossier, une fois les agents et les skills chargés. Il compare ce que l'outil expose au paquet :

- **agents** listés par l'outil ;
- **skills visibles pour chaque agent**, jugées sur les **règles effectives** que renvoie `/api/agent` (règles globales, de l'utilisateur et de l'agent fusionnées), avec un repli sur les règles rendues ;
- **serveurs MCP** listés ;
- **plugins** globaux de l'utilisateur : simple avertissement, non bloquant.

Tout élément inattendu (`agent:<id>`, `skill:<id>@<agent>`, `mcp:<nom>`) fait échouer le lancement. Le serveur est alors abandonné (arrêté, jeton révoqué) et un message précis est affiché.

### 3. Un serveur par version de workflow

Comme les permissions de session ne masquent pas les sous-agents (F21), un serveur ne peut pas héberger des sessions dont les mondes diffèrent. La clé de groupe de serveur contient donc le hash du paquet ([ADR-047](./047-session-interaction-daemon.fr.md)).

### 4. Isolation stricte

- `isolation: strict` dans un workflow exige un adaptateur à isolation complète et interdit de répondre « toujours » à une permission.
- Le réglage `[execution] strict_isolation` masque en plus la config opencode de l'utilisateur, pour le runtime local : `XDG_CONFIG_HOME` est remplacé par un miroir sans `opencode/`, si bien que git, gh et les autres outils gardent leur config.
- En conteneur et à distance, la config de l'utilisateur n'est jamais visible.

### 5. Non configurable

`[deploy].disable_native_agents` est supprimé. Les noms des agents natifs n'existent que dans l'adaptateur.

## Conséquences

### Positives

- Le modèle ne voit que ce que le workflow déclare : `cadrage` présente 4 agents et 30 skills au lieu de 20 agents et ~184 skills.
- Une fuite est détectée au lancement, pas découverte en cours de session. Le test de contrat ajoute un agent dans `~/.config/opencode` et vérifie que le lancement échoue.
- La vérification par agent attrape aussi une règle de l'utilisateur qui rendrait une skill visible à l'un de nos agents.
- Deux barrières indépendantes : les règles de la config et le plugin.

### Négatives / Compromis

- Sans isolation stricte, en local, la config opencode de l'utilisateur est chargée. `Attest` refuse ce qui ajoute des agents, des skills ou des MCP visibles, mais ne contrôle pas les autres réglages.
- Le résultat dépend de l'ordre de fusion des règles d'opencode : une règle de l'utilisateur sur un de nos agents passe aujourd'hui avant les nôtres. Le test de contrat `TestContractAttestChecksSkillsPerAgentOnServer` casse si cet ordre change.
- Un plugin déclaré qui ne se charge pas est silencieux (cas de `context-mode` sous V2), car `Attest` ne vérifie pas qu'il est actif. Un plugin dont l'id diffère du nom de son paquet produit un avertissement « hors paquet ».
- Le monde fermé porte sur ce que le modèle voit, pas sur ce que le shell peut faire : en local, l'agent garde les droits de l'utilisateur ([ADR-044](./044-credential-proxy-session-limits.fr.md), `SECURITY`).
- Un serveur par version de workflow coûte de la mémoire (~340 à 440 Mo par serveur).

## Alternatives considérées

| Alternative | Rejetée car |
|---|---|
| Monde fermé optionnel (`disable_native_agents`) | Laisse les natifs et tous les agents visibles par défaut ; c'était la situation de départ. |
| Un serveur partagé avec des permissions de session par workflow | Les permissions de session ne masquent pas les sous-agents (F21). |
| Vérifier seulement la config rendue | Toujours vrai par construction ; ne voit ni la config de l'utilisateur, ni ce que l'outil expose réellement. |
| Masquer `XDG_CONFIG_HOME` par défaut | Le shell de l'agent perdrait la config de git, gh et des autres outils ; d'où un miroir sans `opencode/`, en option. |
| Compter sur le plugin seul | Si le plugin ne se charge pas, rien ne ferme le monde ; les règles de la config restent la barrière principale. |
