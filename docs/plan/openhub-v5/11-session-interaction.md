# 11 — Interaction avec les sessions (simple et multi-session)

## 1. Principe
**oh = tour de contrôle, opencode = cabine** (I1).
- Décisions structurées (checkpoints, questions, permissions) : **depuis n'importe quel client** (TUI oh, interface opencode, navigateur, CLI). Première réponse gagne (F20).
- Conversation libre : dans opencode (TUI ou web).
- Fermer la fenêtre opencode **ne coupe pas** la session.

## 2. Moments d'interaction

| Moment | Où |
|---|---|
| Lancer | oh (fiche) |
| Suivre | oh (Sessions, flux en direct, badge, notifications) |
| Décider (⏸ ? ! $ ✗) | oh (À traiter, fiches) **ou** opencode |
| Converser | opencode (TUI/web) ; consigne courte depuis oh (`m`) |
| Débloquer | oh (interrompre, modèle, consigne) ou opencode |
| Conclure | oh (récap, diff, MR, « Enchaîner avec… ») |
| Reprendre | oh → attacher |

## 3. Ouverture d'une session (I2)
Mode `auto` : **iTerm2** (AppleScript, onglet ; style `tab|split|window`) → **Terminal.app** (`do script`, nouvelle fenêtre ; les onglets exigeraient l'accessibilité) → **tmux** (si `$TMUX`) → **navigateur** (`pair`) → **suspension** (dernier recours).
- Implémentation : étendre `internal/termlaunch` (déjà iTerm/Terminal) ; commande = `opencode --server <url> -s <id>` avec `OPENCODE_SERVER_PASSWORD` dans l'environnement de la commande.
- Autorisation macOS « Automation » : au premier usage ; en cas de refus, passer au mode suivant et le signaler (Doctor).

## 4. Qui peut faire quoi

| Action | TUI opencode | oh | Web/mobile | CLI oh | Notification |
|---|---|---|---|---|---|
| Converser | ✅ | consigne | ✅ | `send` | — |
| Checkpoint | ✅ | ✅ | ✅ | `approve` | ouvre oh |
| Question | ✅ | ✅ | ✅ | `answer` | ouvre oh |
| Permission | ✅ | ✅ | ✅ | `approve` | ouvre oh |
| Suivre N sessions | ❌ | ✅ | onglets | `list/follow` | ✅ |
| Arrêter / modèle | partiel | ✅ | partiel | ✅ | — |

## 5. Multi-session

### 5.1 Topologie (I5)
**Un serveur par groupe = (version du workflow/hash du paquet, projet, runtime).** Sessions du groupe = locations (worktrees) distinctes ; `session.environment`, permissions de session et instructions **propres à chaque session**.
- Rejeté : serveur global multi-workflows (F21 : sous-agents non masquables par session).
- Coût : ~340–440 Mo par serveur ; prêt en 1,4 s (F22).
- Conteneur : un conteneur par groupe (worktrees montés).

### 5.2 Garde-fous (I6, optionnels, désactivés par défaut)
- Sessions actives max (file d'attente + priorité dans `RunService`).
- Budget par session et journalier global (proxy).
- Plafond mémoire (Doctor affiche l'estimation).
- Worktree auto par session qui écrit (O10).
- Avertissement d'attention dans la fiche : « N sessions demandent déjà une action ».

### 5.3 Lancer plusieurs sessions
Multi-sélection dans le sélecteur Beads → N sessions, 1 serveur, N worktrees. Après `cadrage` : « Enchaîner avec ticket × N ».

### 5.4 Équipe
Sessions des autres membres visibles en lecture seule (claims team-state).

## 6. Cycle de vie

```
preparing → active ─┬─ attached (≥1 client)
                    └─ detached (aucun client, elle continue)
           ↓↑
      waiting (⏸ ? ! $ ✗) → décision → active
           ↓
      idle (attend une consigne) ──(5 min sans décision en attente)──▶ sleeping (serveur arrêté)
           ↓                                                         reprise : même paquet, -s id
      completed · failed · stopped → archived (récap, diff, coût)
```
- **Serveur en veille** : quand **toutes** ses sessions sont idle/terminées sans décision en attente depuis 5 min (réglable). Coût LLM nul à l'arrêt ; la reprise ne coûte pas plus (cache de prompt ~5 min de toute façon).
- **Décision en attente, oh ouvert** : serveur maintenu, rappel par notification.
- **Quitter oh** : inactives/en attente → veille immédiate ; qui travaillent → « **finir l'étape puis veille** » (défaut) / « continuer en arrière-plan » / « arrêter maintenant », par session. « Finir l'étape » = attendre `session.execution.succeeded` ou la prochaine décision en attente, puis arrêter le serveur.
- **Arrière-plan, oh fermé** : notifications système pour chaque décision ; veille automatique après 5 min sans réponse.
- **Redémarrage de oh** : réconciliation du registre (`servers`), récap « pendant votre absence » (sessions, décisions, coût).
- **Redémarrage machine** : sessions reprenables (données du groupe + paquet par hash conservés).

## 7. Notifications
osascript `display notification` (ou `terminal-notifier` si installé, pour le clic) + bip ; regroupement ; déclencheurs ⏸ ? ! $ ✗ + fin de session ; jamais de contenu sensible dans le texte.

## 8. CLI
`oh session list [--all] [--json]` · `attach <id> [--how …]` · `follow <id>` · `approve <id> [--decision once|reject] [-m …]` · `answer <id> --field k=v` · `send <id> "…"` · `stop <id>` · `resume <id>` · `open <id> --browser` · `fetch <id>` (distant) · `resolve <id>` (conflits Beads).

## 9. Données
Tables `servers`, colonnes `sessions` (state, group_key, checkpoint_state…), `pending_decisions` (`03` §7). Événements SSE = source temps réel ; endpoints de liste = source de vérité après reconnexion.
