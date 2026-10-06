> [Read in English](038-sessionspec-tool-adapters.en.md)

# ADR-038 — Modèle neutre `SessionSpec` et adaptateurs d'outil

## Statut

Accepté

## Date

2026-10-05

## Contexte

L'[ADR-036](./036-platform-abstraction-layer.fr.md) a introduit `platform.SessionPlatform` pour découpler `oh` d'opencode. Cette interface reprenait la forme d'opencode V1 : lancement d'une interface par la ligne de commande (`RunInteractive`, `ExecReplace`), exécution sans interface, serveur « parallèle » à part, et `RequiresDeploy()` (les agents étaient déployés dans le projet).

opencode V2 (2.0.20) a cassé ce modèle :

- `opencode --agent X` n'ouvre plus l'interface (F1) : tous les lancements d'oh, en CLI comme en TUI, échouaient.
- Sans serveur dédié, le client parle à un service partagé qui ignore les variables d'environnement du client (F2).
- V2 offre en revanche une API complète : sessions créées avec un identifiant et des permissions choisis par l'appelant, flux d'événements, permissions et formulaires pilotables sans interface, export et import (F12, F17, F18).

La v5 a besoin d'une session par workflow avec un paquet qui lui est propre, d'un monde fermé vérifié, de décisions prises depuis oh, de plusieurs sessions par serveur et de plusieurs environnements d'exécution. La décision D2 impose que toute cette logique reste dans `oh`, avec un adaptateur par outil, pour pouvoir brancher plus tard un autre outil (BL-6).

## Décision

### 1. Modèle neutre (`internal/sessionspec`)

`oh` décrit une session sans rien savoir de l'outil :

- **`SessionSpec`** : identifiant (`ses_` + 26 caractères, choisi par oh), clé de groupe, workflow (id, couche, version, hash), paquet, dossier de travail (`Location`), agent d'entrée, mode, prompt rendu, modèle, fournisseur (`ProviderSpec` : id, région, URL du proxy, type d'authentification), environnement propre à la session (`SessionEnv`), règles de session (`SessionRules`), runtime et préférence d'ouverture.
- **`BundleSpec`** : contenu compilé du paquet (agents, skills, MCP, plugins, permissions neutres, graphe de délégation `SubagentGraph`, profondeur `MaxDepth`, Code Mode, niveau d'isolation, `StrictIsolation`, description des checkpoints du workflow, modèle par défaut).
- **Permissions neutres** (`PermissionRule` : action, ressource, effet) ; l'action d'un outil MCP s'écrit `mcp:<serveur>/<outil>` et l'adaptateur la traduit.
- Les chemins dépendants de la machine sont des variables (`{{oh.bundle}}`, `{{oh.bin}}`), développées par l'adaptateur au démarrage : le hash du paquet n'en dépend pas.

### 2. Interface d'adaptateur (`internal/adapters`)

`adapters.ToolAdapter` : `Name`, `Detect` (binaire, version, compatibilité), `Capabilities` (isolation, événements, décisions sans interface, plusieurs dossiers par serveur, crochets de plugin, attachement), `Render`, `StartServer` / `StopServer`, `Attest`, `CreateSession`, `SendPrompt`, `AttachCommand`, `Events`, `ActiveSessions`, `Pending`, `Reply`, `Control` (consigne, interruption, modèle, compactage), `Results` (diff, coût, tokens).

Les fonctions qu'un outil peut ne pas avoir sont des **interfaces facultatives**, détectées à l'exécution : `SessionRulesSetter`, `SessionEnvSetter`, `ChildLister`, `Forker`, `TurnWaiter`, `ActionNamer`, `SessionPorter` (export et import), outil Linux pour le conteneur.

Règle : **aucun nom d'agent, d'outil ou d'événement opencode hors de l'adaptateur**. Les événements sont normalisés (`EventKind`), le flux en direct est décodé dans l'adaptateur, les actions sont neutres.

### 3. Implémentation unique : `internal/adapters/opencodev2`

Modèle de lancement (O3), le même pour la CLI et la TUI :

1. `opencode serve` dédié à un groupe de serveur : port libre, mot de passe aléatoire, `XDG_DATA_HOME` propre au groupe, `OPENCODE_DISABLE_PROJECT_CONFIG=1`, config rendue passée par `OPENCODE_CONFIG_CONTENT`, jeton du proxy d'identifiants à la place de la clé du fournisseur.
2. Attente de disponibilité : agents et skills listés (le chargement est asynchrone).
3. `Attest`, qui vérifie le monde fermé.
4. Création de la session par l'API : identifiant choisi par oh, agent, dossier, règles de session. Puis envoi du prompt initial.
5. Ouverture de l'interface : `opencode --server <url> -s <id>`, mot de passe passé dans l'environnement de la commande.

Le plugin oh (TypeScript embarqué, objet `{ id, setup }`) est installé dans le dossier de données du groupe. Il ajoute le corps de l'agent courant au prompt système (`session.hook("context")`, O2 : les agents n'ont pas de `system`, et le prompt de base d'opencode est conservé). Il retire aussi les agents et skills hors paquet. Il ne contient aucune logique métier.

La plage de versions acceptée est déclarée par version d'oh (`opencodev2/compatibility.json` : opencode 2.0.0 → 2.99.99 pour oh 5.0) ; `Detect` refuse une version hors plage.

### 4. Services

Le RunService (`internal/runsvc`), le SessionService (`internal/services/session`), le CheckpointService et le démon `ohd` n'appellent que l'interface. `platform.SessionPlatform`, `SessionServer` et `ParallelRunner` sont supprimés avec opencode V1 ([ADR-048](./048-opencode-v1-abandonment.fr.md)). Le paquet `internal/platform` ne garde que `Credentials` et les types de statistiques ; `StatsProvider` est désormais implémenté par `internal/sessionstats` sur `oh.db`.

## Conséquences

### Positives

- La logique (paquet, checkpoints, décisions, budgets, veille) est écrite une fois dans oh. Ajouter un outil revient à écrire un paquet d'adaptateur, sans toucher aux services, à la CLI ni à la TUI.
- Le même lancement sert à la CLI, à la TUI, au conteneur et au job distant.
- Les décisions (checkpoints, questions, permissions) se prennent depuis oh, sans attacher d'interface, et la première réponse gagne, quel que soit le client.
- Tests en trois couches : golden du rendu, tests de contrat contre un vrai `opencode serve` sans LLM (`-tags integration`, refaits à chaque version d'opencode), e2e Haiku (`-tags integration e2e`).
- `oh` ne lit plus la base d'opencode : coût, tokens et états viennent de l'API et du flux d'événements.

### Négatives / Compromis

- L'interface a la forme d'opencode V2 (serveur HTTP, permissions et formulaires pilotables). Un outil sans mode serveur n'aura que des capacités partielles, et l'interface devra peut-être évoluer : la neutralité n'est pas prouvée tant qu'un seul adaptateur existe.
- Plusieurs points de l'API V2 sont expérimentaux (export, import, `instructions` par session) : les tests de contrat doivent être relancés à chaque version d'opencode.
- Des limites d'opencode 2.0.20 sont compensées dans oh, et ces contournements devront être revus à chaque version :
  - le shell de session ne voit que l'environnement de session (F30) : `PATH`, `HOME` et les variables utiles de la machine sont réinjectés par le RunService ;
  - l'environnement de session n'est pas transmis aux sous-sessions (F31) : le démon l'applique à chaque sous-session qu'il rattache ;
  - le message d'un refus de permission n'atteint pas l'agent : la consigne est envoyée par un message à part ;
  - agents et skills listés de façon asynchrone : attente explicite avant `Attest`.
- Les interfaces facultatives multiplient les chemins : chaque appelant doit gérer leur absence.

## Alternatives considérées

| Alternative | Rejetée car |
|---|---|
| Prolonger `platform.SessionPlatform` (ADR-036) | Calqué sur le lancement en ligne de commande de V1 ; ni création de session par l'API, ni décisions sans interface, ni plusieurs sessions par serveur ; suppose un déploiement dans le projet. |
| Lancer l'interface directement (`opencode --agent`, ou `--standalone` avec la config dans l'environnement) | `--agent` n'existe plus pour l'interface en V2 (F1) ; `--standalone` ne permet ni de piloter la session depuis oh ni d'y attacher plusieurs clients. |
| Passer par le service partagé d'opencode | Il ignore l'environnement du client (F2) : impossible d'isoler la config, les données et les identifiants d'une session. |
| Mettre la logique dans le plugin d'opencode | Liée à un outil, difficile à tester, et le plugin ne voit qu'une session à la fois ; le plugin reste mince et interroge oh (O6). |
| Écrire la config dans le projet (`.opencode/`, `opencode.json`) | Un seul monde par dossier, écritures dans les fichiers de l'utilisateur : voir [ADR-043](./043-session-bundle-deploy-removal.fr.md). |
