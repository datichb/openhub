> [Read in English](sessions-v5.en.md)

# Sessions sur le runtime v5 (opencode V2)

Quand opencode V2 est installé, `oh start`, les lancements depuis la TUI, `--parallel`, `--sweep` et les exécutions sans interface passent par le runtime v5. Avec opencode V1, rien ne change.

## Ce qui change

| | opencode V1 | opencode V2 (runtime v5) |
|---|---|---|
| Agents et skills | déployés dans le `.opencode/` du projet | compilés dans un paquet de session hors du projet (`~/.oh/bundles/<hash>`) |
| Agents et skills visibles | tout ce qu'opencode trouve | seulement ceux du paquet (« monde fermé », vérifié à chaque démarrage) |
| Clé LLM | transmise à opencode | jamais transmise à opencode : c'est le proxy du démon oh qui la détient |
| Fenêtre de session | oh est suspendu | nouvel onglet ou nouvelle fenêtre de terminal ; oh reste utilisable |
| Fermeture de la fenêtre | termine la session | la session continue ; on la rouvre avec `oh session attach` |

## Ouverture d'une session

Le client de la session s'ouvre avec la première méthode qui fonctionne, dans cet ordre :

1. iTerm2 (onglet, panneau ou fenêtre : `iterm_style`)
2. Terminal.app
3. tmux (nouvelle fenêtre quand oh tourne dans tmux)
4. navigateur (interface web d'opencode)

La suspension de oh n'est qu'un dernier recours (`attach = "suspend"`).

## Configuration

`~/.oh/config.toml` :

```toml
[session]
attach = "auto"            # auto | iterm | terminal | tmux | browser | suspend
iterm_style = "tab"        # tab | split | window
idle_sleep_minutes = 5     # un serveur inactif se met en veille après N minutes
```

`OH_SESSION_ATTACH` remplace `attach` le temps d'une commande.

## Veille et reprise

- Un serveur se met en veille quand **aucune** session ne travaille, qu'aucun client n'est attaché et que rien ne s'est passé depuis `idle_sleep_minutes`. Une décision qui vous attend (permission, question) garde le serveur éveillé tant que oh est ouvert.
- Une session en veille reprend quand vous la rouvrez : `oh session attach <id>` redémarre son serveur, puis ouvre le client.
- Si vous quittez la TUI pendant que des sessions travaillent, oh vous demande quoi faire. Le choix par défaut est « finir l'étape en cours, puis mettre en veille ».

## Commandes

| Commande | Rôle |
|---|---|
| `oh session list` | sessions et leur état (active, en attente, inactive, en veille, arrêtée) |
| `oh session attach <id>` | ouvrir (ou reprendre) une session |
| `oh session stop <id>` | arrêter une session, et son serveur si aucune autre session ne l'utilise |
| `oh daemon status` | état du démon (serveurs, jetons de proxy) |
| `oh daemon stop [--force]` | arrêter le démon (refusé si des sessions tournent, sauf `--force`) |
| `oh doctor` | vérifications v5 : runtime, démon, git, terminal |

## Clés LLM

La clé est cherchée dans cet ordre : projet, équipe, hub, puis (Bedrock uniquement) le profil AWS avec signature SigV4. Une clé d'équipe se saisit dans **Détail équipe**, touche `K`. Elle est rangée dans le trousseau sous `openhub.team.<équipe>.provider.<fournisseur>.token`.

La région Bedrock vient de la config oh, puis de `AWS_REGION` / `AWS_DEFAULT_REGION`, puis du profil AWS. Si aucune n'est définie, oh utilise `us-east-1` et l'indique dans ses logs.

Changer le fournisseur, la région ou la clé d'un projet démarre un nouveau serveur au lancement suivant. Les sessions en cours gardent leurs réglages jusqu'à la mise en veille de leur serveur.

## Variables d'environnement

| Variable | Effet |
|---|---|
| `OH_HOME` | déplace `~/.oh` (environnements de test) |
| `OH_SESSION_ATTACH` | remplace `[session] attach` |
| `OH_V5=0` | force l'ancien chemin de lancement. **Avec opencode V2 installé, ce chemin ne fonctionne pas** : à réserver au diagnostic. |

## Limites

- **Windows** : les sessions v5 ne sont pas encore prises en charge, faute de démon. Utilisez opencode V1 ou WSL.
- **Sécurité en local** : l'agent tourne sous votre utilisateur. Il peut atteindre le socket du démon et `oh.db`, mais jamais la clé LLM. Voir [SECURITY.fr.md](../../SECURITY.fr.md).
