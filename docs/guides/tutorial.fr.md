> [Read in English](tutorial.en.md)

# Tutoriel -- Du premier lancement à un ticket implémenté et relu

Ce tutoriel suit un parcours complet avec oh v5 : installer oh et opencode V2, configurer le hub, implémenter un ticket Beads avec le workflow `ticket`, passer ses checkpoints depuis la TUI, puis faire relire la branche avec le workflow `review`. Chaque étape montre ce que vous devez voir.

**Ce que vous saurez faire :**
- Installer `oh` et opencode V2, configurer le hub avec `oh init`
- Créer un ticket Beads et lancer `oh run ticket --tickets <id>`
- Suivre une session et passer ses checkpoints depuis la fiche checkpoint
- Récupérer les résultats et enchaîner avec une review

**Temps nécessaire :** ~20 minutes (selon la taille du ticket).

> **Prérequis :** `git`, `bd` (Beads), une clé API pour un fournisseur LLM (voir [Quel provider choisir ?](#quel-provider-choisir-)) et un projet git (ici `~/workspace/my-app`). Les identifiants, coûts et durées des sorties ci-dessous sont des exemples.

---

## Quel provider choisir ?

```mermaid
flowchart TD
    Start([Je dois choisir un provider]) --> Q1{Avez-vous un<br/>compte AWS avec<br/>acces Bedrock ?}
    Q1 -->|Oui| Bedrock[Amazon Bedrock<br/>Enterprise, facturation a l'usage]
    Q1 -->|Non| Q2{Avez-vous une<br/>cle API Anthropic ?}
    Q2 -->|Oui| Anthropic[Anthropic Direct<br/>Le plus simple, paiement par token]
    Q2 -->|Non| Q3{Voulez-vous un<br/>acces multi-modeles ?}
    Q3 -->|Oui| OpenRouter[OpenRouter<br/>Plusieurs modeles, une seule cle API]
    Q3 -->|Non| Copilot[GitHub Copilot<br/>Utiliser votre abonnement Copilot existant]
```

| Provider | Complexité setup | Ce qu'il vous faut avant de commencer |
|----------|-----------------|---------------------------------------|
| **Anthropic** | Faible | Une clé API depuis [console.anthropic.com](https://console.anthropic.com) |
| **Amazon Bedrock** | Moyenne | Un compte AWS avec l'accès aux modèles Bedrock active |
| **OpenRouter** | Faible | Une clé API depuis [openrouter.ai](https://openrouter.ai) |
| **GitHub Copilot** | Faible | Un abonnement GitHub Copilot actif |

> Les clés restent sur votre machine : le démon d'oh les détient et opencode ne les voit jamais. Vous pourrez changer de provider avec `oh provider setup`.

---

## Étape 1 -- Installer oh et opencode V2

```bash
brew install datichb/tap/openhub
brew install anomalyco/tap/opencode
```

Sans Homebrew : `curl -fsSL https://raw.githubusercontent.com/datichb/openhub/main/install.sh | bash` pour oh, et https://opencode.ai pour opencode.

Vérifier :

```bash
oh version
opencode --version
```

Sortie attendue :

```
oh v5.0.0
  commit:  4fb125b4
  built:   2026-10-06T20:00:33Z
  go:      go1.26.4
  os/arch: darwin/arm64
2.0.20
```

opencode doit être en version 2.x : oh ne prend plus en charge opencode V1 (voir [Migrer vers oh v5](migration-v5.fr.md)).

---

## Étape 2 -- Configurer le hub : `oh init`

```bash
cd ~/workspace/my-app
oh init
```

L'assistant s'ouvre dans le terminal. Sur la page « Bienvenue », choisissez **Développeur solo** (vous pourrez passer en équipe plus tard), puis « Commencer ».

```
◆ OpenHub — Assistant de configuration

 Langue ── Provider ── Équipe ── Projet ── Intégrations

 Comment souhaitez-vous configurer ?
 ▸ Développeur solo  ~2 min
   Membre d'équipe  ~5 min
   Configuration complète  ~5-8 min
```

Puis, étape par étape :

| Étape | Ce que vous saisissez |
|-------|-----------------------|
| **Langue** | Français |
| **Fournisseur IA** | `anthropic`, puis la clé API (rangée dans le trousseau) |
| **Premier projet** | nom `my-app`, chemin `~/workspace/my-app` (proposé depuis le dossier courant) |
| **Intégrations MCP** | GitLab si vous avez un jeton, sinon « Ignorer » |

En parcours solo, l'étape **Équipe** est sautée : oh crée un espace de workflows solo pour le projet. `Ctrl+S` valide une étape, `Ctrl+B` revient en arrière. Un écran de résumé termine l'assistant.

Vérifier ensuite :

```bash
oh doctor
```

Chaque ligne doit être `✔` (ou `⚠` pour ce qui est facultatif, comme le moteur de conteneurs). En particulier : opencode V2, le démon oh et git.

---

## Étape 3 -- Créer un ticket Beads

```bash
bd init
bd create "Ajouter l'export CSV" -p 1 -l ai-delegated \
  -d "Bouton Exporter sur la liste des commandes ; colonnes : id, date, client, total."
```

`bd` affiche l'identifiant du ticket. Dans la suite, il s'appelle `bd-42`. Le libellé `ai-delegated` le fait apparaître dans le sélecteur de tickets de la TUI (filtre par défaut).

---

## Étape 4 -- Inspecter le paquet de session (facultatif)

Rien n'est déployé dans le projet : chaque session part d'un paquet construit au lancement, hors du projet. Pour voir ce que recevra une session `ticket` :

```bash
oh bundle show ticket
```

La commande liste les agents (`orchestrator-dev` en entrée, `developer`, `reviewer`…), les skills, les permissions et les serveurs MCP. Aucun fichier n'est écrit dans `my-app`.

---

## Étape 5 -- Lancer le ticket

Le workflow `ticket` a trois checkpoints : `cp-1` « Démarrer le ticket », `cp-2` « Commit ou correction » (toujours en pause) et `cp-3` « Ticket suivant ». En mode `manuel`, tous attendent votre validation : c'est le plus parlant pour une première fois.

```bash
oh run ticket --tickets bd-42 --mode manuel --recap
```

Sortie attendue :

```
▸ Préparation du workflow ticket…
Lancer ticket (hub:ticket) · my-app
  Mode           manuel
  Exécution      local
  Agents (4)     orchestrator-dev (entrée) · developer · reviewer · documentarian
  Skills (23)    ~14200 tokens (agent d'entrée 5400 + catalogue des skills 8800)
  MCP            gitlab, workflow
  Isolation      full
  Sessions       1 session(s) · 1 serveur · 0 worktree(s)
    bd-42        ~/workspace/my-app (base)
? Lancer ? Oui
✔ Session ouverte (iterm) : ses_2f9c1a7b
```

Un nouvel onglet s'ouvre avec opencode : l'agent `orchestrator-dev` lit le ticket et prépare son plan. Sans `--recap`, la session démarre directement.

> Depuis la TUI, le même lancement se fait par « Démarrer » → `ticket` → fiche de lancement : bouton « Choisir… » pour le ticket, mode `manuel` à l'étape Options, puis `Ctrl+S` (récap) et `Ctrl+S` encore (lancement).

---

## Étape 6 -- Suivre la session

Gardez la fenêtre opencode à côté. Dans un autre terminal :

```bash
oh session list
```

```
SESSION        PROJET  AGENT             ÉTAT          DÉCISIONS  COÛT    DÉBUT
ses_2f9c1a7b   my-app  orchestrator-dev  ⏸ en attente  ⏸ 1        $0.042  06/10 10:02
```

La session attend déjà une décision : `cp-1`. Ouvrez la TUI :

```bash
oh
```

La barre du bas affiche `● 1 ⏸ 1` (une session vivante, une décision en attente). Tapez `sessions` puis `Entree` pour ouvrir la **vue Sessions** :

```
 Sessions
 ─ À traiter (1)
 ▸ ⏸ ticket · my-app  cp-1 « Démarrer le ticket »
      il y a 1m
 ─ En cours (1)
   ⏸ ticket · my-app
      en attente · orchestrator-dev · $0.04 · 1 décision(s) en attente
 ┌ Détail · ticket · my-app ──────────────────────────────────────┐
 │ ticket · manuel · ⌂ · ~/workspace/my-app · 9f3c1d2e…           │
 │ ⏸ en attente · orchestrator-dev · $0.042 · démarrée il y a 1m  │
 │ ⏸ cp-1 → ○ cp-2 → ○ cp-3                                       │
 │ ses_2f9c1a7b                                                   │
 └────────────────────────────────────────────────────────────────┘
```

Sur la ligne de la session, `t` affiche à droite le flux en direct (agent courant, outils, coût).

---

## Étape 7 -- Passer `cp-1` depuis la fiche checkpoint

Placez-vous sur la ligne `⏸ … cp-1` et appuyez sur `Entree`. La **fiche checkpoint** s'ouvre :

```
┌ ⏸ cp-1 · Démarrer le ticket · ticket · my-app ─────────────────────┐
│ bd-42 : export CSV de la liste des commandes. Agent prévu :         │
│ developer. Fichiers visés : src/orders/export.ts, src/orders/List…  │
│                                                                     │
│ ─ Changements                                                       │
│   aucun changement                                                  │
│                                                                     │
│ ─ Derniers messages                                                 │
│   Je propose de confier l'implémentation à developer…               │
│                                                                     │
│ ─ Frise                                                             │
│   ⏸ cp-1 → ○ cp-2 → ○ cp-3                                          │
│                                                                     │
│              [ Décider ]   [ Attacher ]                             │
└─────────────────────────────────────────────────────────────────────┘
```

Choisissez **Décider**. Le formulaire propose :

| Choix | Effet | Message à l'agent |
|-------|-------|-------------------|
| **Valider** | le checkpoint passe, l'agent continue | facultatif |
| **Corriger d'abord** | l'agent corrige puis redemande le checkpoint | obligatoire |
| **Autre consigne** | l'agent suit votre consigne puis redemande | obligatoire |

Choisissez « Valider », ajoutez par exemple « Ajoute aussi un test unitaire », puis validez. La frise devient `✔ cp-1 10:05 → developer` et l'agent `developer` peut commencer (il était verrouillé jusqu'à `cp-1`).

> Raccourci : `y` sur la ligne d'un checkpoint le valide sans ouvrir la fiche. Vous pouvez aussi valider dans la fenêtre opencode : la première réponse gagne.

---

## Étape 8 -- Passer `cp-2` (commit ou correction)

Quand `developer` a fini et que `reviewer` a relu, `cp-2` « Commit ou correction » arrive dans « À traiter » (et une notification système s'affiche). Ouvrez la fiche avec `Entree` :

```
┌ ⏸ cp-2 · Commit ou correction · ticket · my-app ───────────────────┐
│ Review terminée : 0 bloquant, 1 mineur (nommage de exportRows).     │
│                                                                     │
│ ─ Changements                                                       │
│   +142 −18 · 4 fichier(s)                                           │
│   A src/orders/export.ts  +96 −0                                    │
│   M src/orders/List.tsx  +21 −4                                     │
│   A src/orders/export.test.ts  +23 −0                               │
│   M package.json  +2 −14                                            │
│                                                                     │
│ ─ Derniers messages                                                 │
│   reviewer › 1 remarque mineure : renommer exportRows en toCsvRows  │
│                                                                     │
│ ─ Frise                                                             │
│   ✔ cp-1 10:05 → developer → reviewer → ⏸ cp-2 → ○ cp-3             │
│                                                                     │
│        [ Décider ]   [ Diff complet ]   [ Attacher ]                │
└─────────────────────────────────────────────────────────────────────┘
```

« Diff complet » affiche tout le diff. Pour corriger la remarque avant le commit : **Décider** → « Corriger d'abord », message « Renomme exportRows en toCsvRows », valider. L'agent corrige, puis redemande `cp-2` (la frise montre `↺`).

Cette fois, validez depuis la ligne de commande pour voir l'autre chemin :

```bash
oh session inbox
```

```
   DÉCISION                          AGENT             DEMANDE                         DEPUIS
⏸  checkpoint:ses_2f9c1a7b:per_…     orchestrator-dev  cp-2 « Commit ou correction »   il y a 1m0s
```

```bash
oh session approve 2f9c
```

```
cp-2 « Commit ou correction » → once
```

L'agent committe sur la branche de travail. Si `cp-3` « Ticket suivant » apparaît ensuite, validez-le (`y` dans la vue Sessions) : il n'y a qu'un ticket, l'agent s'arrête.

---

## Étape 9 -- Récupérer les résultats

```bash
oh session results 2f9c
```

```
4 fichier(s) modifié(s) · +140 −18 · 182340 tokens · $1.12
Branche : feat/bd-42
  src/orders/export.ts  +94 −0
  src/orders/List.tsx  +21 −4
  src/orders/export.test.ts  +23 −0
  package.json  +2 −14
```

Pour la merge request :

```bash
oh session results 2f9c --mr
```

```markdown
## bd-42 Ajouter l'export CSV

4 fichier(s) modifié(s) · +140 −18 · 182340 tokens · $1.12

### Fichiers modifiés

- `src/orders/export.ts` (+94 −0)
- ...

---
Session oh `ses_2f9c1a7b` · agent orchestrator-dev · branche feat/bd-42
```

Dans la vue Sessions, `o` affiche la même description.

---

## Étape 10 -- Faire relire la branche : `oh run review`

Le workflow `review` relit une branche sans rien modifier. Deux façons de le lancer :

**Depuis la TUI :** sur la session `ticket`, appuyez sur `e` (**Enchaîner avec…**). oh propose les workflows qui prennent une sortie de la session ; choisissez `review` : la fiche de lancement s'ouvre avec `branch = feat/bd-42`. Choisissez le « Type de review » (par exemple `adversarial`), puis `Ctrl+S` deux fois (récap, puis lancement).

**En ligne de commande :**

```bash
oh run review -i branch=feat/bd-42 -i review_mode=adversarial --parent ses_2f9c1a7b
```

```
▸ Préparation du workflow review…
✔ Session ouverte (iterm) : ses_8d41e0c2
```

L'agent `reviewer` lit la branche et rend son rapport dans la fenêtre opencode. Le workflow `review` n'a pas de checkpoint : suivez-le avec `t` dans la vue Sessions ou `oh session follow 8d41`.

---

## Étape 11 -- Terminer

```bash
oh session stop 2f9c
oh session stop 8d41
```

Ou `s` dans la vue Sessions. Si vous quittez la TUI (`Ctrl+Q`) pendant qu'une session travaille encore, oh demande pour chacune : « Finir l'étape, veille », « Arrière-plan » ou « Arrêter maintenant » (`Esc` annule). Une session en veille se reprend avec `oh session attach <id>`.

---

## Étape 12 -- Explorer la suite

| Ce que vous voulez faire | Commande | Guide |
|--------------------------|----------|-------|
| Plusieurs tickets en parallèle | `oh run ticket --tickets bd-43,bd-44` | [Démarrage rapide](getting-started.fr.md#plusieurs-tickets) |
| Isoler les commandes de l'agent | `oh run ticket --tickets bd-42 --runtime container` | [Conteneur](container.fr.md) |
| Lancer sur GitLab CI | `oh run ticket --tickets bd-42 --runtime remote` | [Exécution distante](remote-runners.fr.md) |
| Adapter un workflow pour l'équipe | `oh workflow new` | [Workflows d'équipe](team-workflows.fr.md) |
| Voir tous les workflows livrés | `oh workflow list` | [Workflows livrés](../reference/workflows.fr.md) |
| Limiter les coûts | `oh budget set session_budget_usd 5` | [Sessions v5](sessions-v5.fr.md#restrictions) |
| Maitriser la TUI | `oh` | [Utiliser le TUI](tui-usage.fr.md) |

---

## Référence rapide

```
oh init                                   # assistant de configuration
oh doctor                                 # diagnostic
oh bundle show <workflow>                 # paquet de session d'un workflow
oh run ticket --tickets <id> [--mode manuel]
oh run review -i branch=<branche>
oh session list | inbox                   # sessions, decisions en attente
oh session approve <id> [--decision fix -m "…"]
oh session results <id> [--mr]
oh session attach | stop <id>
oh                                        # TUI : vue Sessions (sessions), fiche checkpoint (Entree)
```

---

**Suite :** le [Démarrage rapide](getting-started.fr.md) resume le parcours et les commandes ; [Sessions v5](sessions-v5.fr.md) détaille les sessions, les checkpoints et les restrictions.
