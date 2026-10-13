> [Read in English](checkpoints.en.md)

# Référence — Checkpoints, modes et vérification par oh

Un checkpoint est une pause du workflow : l'agent appelle `workflow_checkpoint`, oh applique le comportement du mode (attendre l'utilisateur ou laisser passer) et ouvre les opérations que le checkpoint déverrouille (`commit`, `close`, `create`, `apply`, `push`). Le schéma des champs est dans [Schéma des workflows](workflow-schema.fr.md#checkpoints) ; l'exécution à trois niveaux (prompt, outil MCP, plugin oh) dans l'[ADR-042](../architecture/adr/042-checkpoints-headless-decisions.fr.md).

Chaque pause coûte du temps humain et, à force, une validation machinale : le nombre de pauses est un paramètre de sécurité. Les règles ci-dessous (rapport de modernisation d'octobre 2026, §9, règles CP-R1 à CP-R11) gardent peu de pauses, et mieux informées.

---

## Modes

| Mode | Ce qui met en pause |
|---|---|
| `manuel` | tous les checkpoints, sauf ceux en `skip` |
| `semi-auto` | les décisions humaines (périmètre, plan) et les checkpoints obligatoires ; les étapes intermédiaires passent seules |
| `auto` | **tout sauf les décisions obligatoires** : un commit, une fermeture de ticket, une création de tickets ou une modification d'infrastructure demandent toujours l'utilisateur |

Le mode `auto` n'est donc pas une autonomie complète : un checkpoint obligatoire (ou qui déverrouille une opération) n'est jamais sauté. Au lancement, `oh run` et le récapitulatif de la TUI affichent les **pauses prévues** pour le mode choisi, par exemple :

```
  Pauses prévues 2 par passage : cp-0 (obligatoire) · cp-2 (obligatoire) — auto = tout sauf les décisions obligatoires
```

Un checkpoint conditionnel y figure avec la mention « si sa condition est vraie ». Les checkpoints répétés (un `cp-2` par ticket) comptent une fois par passage.

### Pauses des workflows livrés (semi-auto)

| Workflow | Pauses en semi-auto | Dont irréversibles | Budget |
|---|---|---|---|
| `feature` | `cp-tickets`, `cp-0`, `cp-spec` (si la condition est vraie), `cp-2` | `cp-tickets`, `cp-2` | 4 |
| `ticket` | `cp-2` | `cp-2` | 3 |
| `cadrage` | `cp-scope`, `cp-tickets`, `cp-recap` | `cp-tickets` | 3 |
| `sweep` | `cp-plan`, `cp-commit`, `cp-recap` | `cp-commit` | 3 |
| `review-feedback` | `cp-fix`, `cp-2` | `cp-2` | 3 |
| `quick` | `cp-commit` | `cp-commit` | 3 |
| `debug` | `cp-ticket`, `cp-commit` | `cp-ticket`, `cp-commit` | 4 |
| `infra-change` (manuel seulement) | `cp-plan`, `cp-apply` | `cp-apply` | 3 |
| `benchmark` | `cp-target` | — | 2 |

`cp-feature` (récap de la feature) passe en automatique en semi-auto : un récapitulatif ne protège rien (CP-R1), il reste une pause en `manuel`.

---

## Budget de pauses (CP-R2)

En semi-auto (ou dans le mode par défaut quand semi-auto n'est pas autorisé), un workflow met en pause au plus :

> **2 décisions humaines** (périmètre, compréhension, arbitrage) **+ 1 par checkpoint qui déverrouille une action irréversible** (`unlocks:` : commit, close, create, apply, push).

Un checkpoint `conditional`, ou qui porte un `when:`, compte comme une pause (le pire cas). Le budget est vérifié :

- sur les workflows livrés par le hub, par le test `TestCheckpointPauseBudget` (bloquant) ;
- sur les workflows d'équipe et de projet, par `oh workflow validate` : avertissement `pause_budget_exceeded` (le workflow reste utilisable).

Pour rentrer dans le budget : passer en `auto` les étapes qui ne protègent rien, ou les faire vérifier par oh (`verify:`) plutôt que par une pause.

---

## Vérification lancée par oh (`verify:`)

```yaml
checkpoints:
  cp-2:
    mandatory: true
    unlocks: [commit, close]
    verify: { from: stack, expect: pass }
```

Avant qu'un checkpoint passe, et donc avant d'ouvrir ses déverrouillages, oh lance lui-même la vérification :

1. **Commandes** : `from: stack` prend celles de la stack détectée (`bricks.VerifyRuns`, dérivées des motifs de `bricks.VerifyCommands` : une cible `make test|build|lint|typecheck` d'abord, sinon les scripts de `package.json` avec le gestionnaire du lockfile, `tsc --noEmit` pour TypeScript, `go build ./...` et `go test ./...`, `cargo`, `pytest`, `mvn`, `./gradlew`). `kinds` choisit les types (par défaut `build`, `test`, `typecheck` ; `lint` sur demande). `cmd` ajoute des commandes **écrites dans le document du workflow**, jamais fournies par l'agent ni par une entrée (pas de gabarit). Jamais d'installation ni de publication.
2. **Exécution** : sans shell, dans l'emplacement de la session (ou le sous-dossier `in`), avec un environnement nettoyé (chemins, locale, caches des outils ; ni jetons, ni identifiants, ni variables d'oh), une entrée vide, un délai (`timeout`, 10 min par défaut, 30 min au plus ; le groupe de processus est tué à l'échéance) et une sortie bornée (fin de sortie conservée, extrait de 30 lignes).
3. **Résultat** : `expect: pass` (défaut) exige que toutes les commandes réussissent ; `expect: fail` qu'une échoue (test écrit d'abord, mode TDD) ; un dépassement de délai est toujours un échec. `must_touch` exige en plus que les changements de la session (commits depuis le début, modifications non commitées, fichiers nouveaux) touchent un des chemins donnés.
4. **Réutilisation** : le résultat est lié à l'état du travail (HEAD, modifications, fichiers non suivis). Il est réutilisé tant que cet état ne change pas ; sinon la vérification est relancée.

**Au checkpoint en pause**, la vérification démarre dès que l'agent demande le checkpoint : la carte de décision affiche son résultat (en cours, réussie, échouée avec un extrait). **Si elle échoue** :

- « Valider » est refusé ; la carte propose **« Passer outre la vérification »**, avec une consigne obligatoire dans la TUI. Le passage est tracé dans la frise (`⚑ cp-2`) et dans la trace de la session (événement `overridden`, auteur et message) ;
- une validation donnée ailleurs (dans l'interface de l'outil) ne passe pas outre : l'appel de l'agent échoue avec `ErrVerifyFailed`, un résumé et l'extrait de la sortie, et rien n'est déverrouillé. L'agent corrige, puis redemande le checkpoint.

**Au checkpoint automatique**, la vérification tourne pendant l'appel de l'agent ; au-delà de 20 secondes, l'appel répond « vérification en cours » et l'agent le rappelle un peu plus tard (le résultat est conservé).

Projet sans stack détectée : « rien à vérifier », le checkpoint passe et la carte l'indique.

**Vérification impossible : jamais de passage en silence.** Si l'emplacement de travail est introuvable (dossier supprimé, sous-dossier `in` absent), la vérification ne peut pas tourner : le checkpoint **se met en pause dans tous les modes**, `auto` compris, même s'il passerait seul d'ordinaire. La carte affiche « impossible ici » ; « Valider » est refusé et seul **« Passer outre la vérification »** le laisse passer, tracé comme un échec (`⚑`, événement `overridden`). L'appel de l'agent échoue avec `ErrVerifyUnavailable` et rien n'est déverrouillé. Il en va de même quand l'emplacement est un dépôt git que git refuse de lire (dépôt d'un autre propriétaire, par exemple) : sans état du travail connu, aucun résultat ne peut y être rattaché, la carte affiche « git ne peut pas lire l'état du travail » et aucune commande ne tourne.

Dans un workflow en lecture seule (`risk: read`), un checkpoint vérifié demande toujours l'utilisateur : les commandes ne tournent qu'après sa validation.

**Assouplissement** : une couche d'équipe ou de projet peut renforcer une vérification héritée (ajouter des types, des commandes, des chemins `must_touch`, allonger le délai), jamais la retirer ni l'affaiblir (`loosening`) ; retirer un checkpoint vérifié est refusé (`verified_checkpoint_removed`).

Les workflows livrés vérifient ainsi `cp-2` (`ticket`, `feature`, `review-feedback`) et `cp-commit` (`quick`, `debug`, `sweep`).

### Sessions distantes

Dans une [session distante](../guides/remote-runners.fr.md), la vérification tourne **là où est le travail** : dans le job, sur le clone du projet, avec les mêmes commandes et les mêmes garanties (environnement filtré, délai, sortie bornée, groupe de processus tué à l'échéance), sous le compte `oh` des agents (celui du serveur de l'outil), jamais sous root ; les lectures git de la vérification aussi, comme toutes les autres lectures git d'oh dans le clone (fenêtre `commit`, garde de fermeture, contrôle de la configuration git…, voir [Exécution distante › Lectures git d'oh](../guides/remote-runners.fr.md#6-dans-le-job--oh-runner)). Sans compte `oh` dans l'image, elle tourne sous l'utilisateur du job, comme le serveur.

| Situation dans le job | Ce que fait oh |
|---|---|
| checkpoint `remote: auto`, vérification réussie | validé par le répondeur de politique, puis passé |
| checkpoint `remote: auto`, vérification échouée | jamais passé outre dans le job : l'appel de l'agent est refusé avec l'échec et l'extrait, rien n'est déverrouillé (si la vérification est déjà finie quand le répondeur répond, la décision est refusée : « corriger d'abord ») ; l'agent corrige et redemande |
| vérification impossible (emplacement introuvable, dépôt que git ne peut pas lire) | le checkpoint est **différé** : la session s'arrête proprement, la suite (et le choix de passer outre) se fait sur la machine |
| checkpoint `remote: defer` | différé, comme avant ; la vérification lancée à la demande se termine avant le push, et son résultat remonte |

Le job attend la fin des vérifications en cours avant d'arrêter les processus des agents et de pousser la branche. Le dernier résultat de chaque checkpoint voyage dans `summary.json` (`verifications`) ; `oh session fetch` l'affiche et l'importe, nettoyé et borné : la carte du checkpoint montre « Dans le job distant : … » et la frise garde l'entrée (auteur `remote`). Ce résultat **ne vaut jamais** vérification sur la machine : quand la session reprend en local et redemande le checkpoint, oh vérifie de nouveau, dans le worktree de la branche poussée.

---

## Preuves de l'agent (`evidence`)

`workflow_checkpoint` accepte `evidence: [{cmd, exit}]` : les commandes que l'agent dit avoir lancées et leur code de sortie. oh les recoupe avec les appels shell qu'il a observés dans la session (et ses sous-agents) :

- **observée** : un appel shell a lancé cette commande ;
- **preuve non observée** : aucun appel ne l'a lancée (avertissement sur la carte, dans la frise et dans la réponse à l'agent) ;
- **observée en échec** : l'outil a signalé l'échec de l'appel alors que l'agent annonce 0.

L'outil ne transmet pas le code de sortie d'une commande qui réussit : oh recoupe la commande, et l'échec quand l'outil le signale. La vérification lancée par oh reste la preuve de référence.

---

## Limite des tours de correction (`limits.max_fix_rounds`)

oh compte les paires developer → reviewer par ticket (le ticket pris et pas encore fermé). La première paire est l'implémentation ; au-delà de `max_fix_rounds` tours de correction, quand le reviewer est lancé pour le dernier tour permis :

- les délégations aux agents `developer*` sont refusées ;
- le checkpoint de commit est marqué **« non résolu »** sur la carte de décision ;
- la frise affiche `⛔ <ticket>`.

Le verrou se lève quand l'utilisateur demande une correction (ou donne une autre consigne) au checkpoint, ou quand le ticket est fermé.

---

## Conditions évaluées par oh (`when:`)

```yaml
  cp-spec:
    mode: { manuel: pause, semi-auto: conditional, auto: conditional }
    when: { paths: ["docs/specs/**"], labels: [ux] }
```

Conditions (une suffit) : `entry` (agent d'entrée de la session), `labels` (étiquette d'un ticket de la session), `paths` (chemin modifié par la session, motifs `*` et `**`), `outputs` (sortie déclarée par `workflow_outputs`). Effet :

| Comportement du mode | Condition vraie | Condition fausse |
|---|---|---|
| `auto` | pause | passe |
| `conditional` | pause | passe |
| `pause`, checkpoint obligatoire ou qui déverrouille | pause | **pause** |
| `skip` | sauté | sauté |

Si les chemins modifiés ne peuvent pas être lus (git refuse le dépôt), une condition `paths` est tenue pour vraie : la pause est ajoutée plutôt que perdue. `when:` ne peut qu'**ajouter** une pause, jamais retirer une pause obligatoire. Sans `when:`, un checkpoint `conditional` demande toujours l'utilisateur (oh ne sait pas évaluer la phrase `condition`). Une couche ne peut pas retirer une condition héritée (`loosening`).

---

## Fenêtres de déverrouillage bornées (CP-R8)

| Opération | Ouverte par | Refermée |
|---|---|---|
| `create` | son checkpoint (`cp-tickets`, `cp-ticket`) | au **checkpoint obligatoire suivant** (`cp-0`, `cp-commit`…) ou quand le checkpoint est redemandé ; pas à la fermeture d'un ticket |
| `commit` | son checkpoint (`cp-2`, `cp-commit`) | **une fois le travail commité** (HEAD a avancé et il ne reste rien à committer : un commit, ou plusieurs à la suite), quand le checkpoint est redemandé, ou à la fermeture du ticket |
| `close` | son checkpoint | à la fermeture d'un ticket, ou quand le checkpoint est redemandé |
| `apply`, `push` | leur checkpoint | quand le checkpoint est redemandé |

En cas d'échec de lecture git, ces protections restent **fermées** : si l'état du travail ne peut pas être lu après un appel shell, la fenêtre `commit` se referme (la frise en garde la raison ; redemander le checkpoint la rouvre) au lieu de rester ouverte ; la fermeture d'un ticket (`bd close`) est refusée tant qu'oh ne peut pas constater un commit non vide depuis le checkpoint et un arbre propre.

---

## Qui passe un checkpoint (`by:`, CP-R11)

Chaque checkpoint nomme les agents qui le passent (`by:` ; sinon déduit du graphe de délégation). Chaque agent ne voit que les siens : dans son contrat de session et dans la table générée des checkpoints. Un checkpoint passé par un autre agent n'est pas refusé : oh l'écrit dans son journal et la réponse de `workflow_checkpoint` porte un avertissement (« vérifie que c'est bien ton checkpoint »).

---

## Carte de décision (CP-R5, partie vérification)

Pour un checkpoint en pause, la carte de la TUI affiche, en plus du résumé de l'agent, des changements, des derniers messages et de la frise :

- **Vérification lancée par oh** : statut, commandes et code de sortie, extrait de la sortie en cas d'échec ; pour une session distante récupérée, le résultat obtenu dans le job ;
- **Preuves de l'agent** : chaque commande annoncée, observée ou non ;
- **Non résolu** : le ticket dont les tours de correction ont atteint la limite.

Choix : Valider · Corriger d'abord · Autre consigne · **Passer outre la vérification** (seulement si elle a échoué ou ne peut pas tourner).
