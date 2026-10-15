> [Read in English](fan-out.en.md)

# Fan-out : une session enfant par ticket ou par sous-tâche (expérimental)

Le fan-out fait implémenter les tickets d'une spec (`implement-spec`) ou les sous-tâches d'un sweep (`sweep`) par des **sessions enfants**, une par ticket ou sous-tâche, chacune dans sa branche et son worktree, parfois en même temps. oh les merge lui-même dans une branche d'intégration, fait résoudre par le `merger` un conflit qu'il ne résout pas seul, et ne merge le tout dans votre branche qu'après **une seule validation**, `cp-merge`, vérifiée par oh sur la branche d'intégration. Il est **expérimental** et désactivé par défaut.

## Activer

```
oh config set fanout.experimental true
oh config set fanout.parallel 2          # 1 (défaut : l'un après l'autre) à 3 sessions enfants en même temps
oh config set fanout.budget_usd 10       # budget global de la session parente et de ses enfants (défaut 10)
```

Ou dans `hub.toml` :

```toml
[fanout]
experimental = true
parallel = 2
budget_usd = 10
```

Sans `experimental`, un lancement avec `fanout=true` est refusé. Le fan-out ne tourne qu'en local (ni conteneur, ni session distante).

## Lancer

```
oh run implement-spec -i fanout=true                  # tickets et spec de la dernière session spec du projet
oh run implement-spec -i fanout=true -t ev-1 -t ev-2  # ou des tickets donnés
oh run sweep -i goal="migrer les logs vers slog" -i fanout=true   # une session enfant par sous-tâche validée à cp-plan
```

`oh run` démarre aussi, en arrière-plan, le **pilote** du fan-out (`oh fanout drive <session>`, journal dans `~/.oh/sessions/<session>/fanout.log`) : c'est lui qui lance, suit et merge les sessions enfants, que vous gardiez ou non le client de la session parente ouvert. S'il s'est arrêté avant `cp-merge`, le démon d'oh le relance de lui-même quand la session parente reprend ou finit une étape (événement `driver`) ; `oh fanout drive <session>` le relance à la main et reprend là où il en était.

## Déroulé

1. **`cp-plan`** (pause) : l'`orchestrator-dev` présente les tickets, leurs dépendances, ceux déjà fermés. Rien ne démarre avant.
2. **Sessions enfants** : oh crée la branche d'intégration `oh/fanout/<session>/integration` depuis votre branche, puis une session `ticket` par ticket, sur `oh/fanout/<session>/<ticket>`, dans un worktree voisin du projet. Un ticket attend ceux dont il dépend ; aucune session pour un ticket fermé. Les enfants héritent du mode de la session parente et des entrées que le workflow leur transmet (`fanout.inputs` : la méthode `tdd` et les consignes pour `implement-spec`), rien d'autre.
3. Chaque enfant implémente, fait relire et passe **son `cp-2` sans pause** : il est remplacé par votre `cp-merge` (substitution de contrôle, CP-R3). oh y lance quand même la vérification (build, tests) dans le worktree de l'enfant ; un échec renvoie le travail à l'enfant. L'enfant commite sur sa branche, mais ne ferme pas son ticket, ne merge pas et ne pousse pas : oh le refuse.
4. **Merge par oh** : un enfant compte comme terminé quand sa session s'arrête, ou quand oh constate la fin de son travail, son tour fini et sans décision en attente : checkpoint de commit passé (son `cp-2` délégué), au moins un commit sur sa branche, rien de non commité. Un enfant inactif sans ce signal n'est arrêté qu'au bout de 10 minutes (garde-fou). oh merge alors sa branche dans la branche d'intégration. Un conflit : oh lance le `merger` (workflow `merge-conflicts`) dans le worktree d'intégration, puis vérifie le résultat (merge commité, les deux côtés présents, aucun marqueur). Sinon l'enfant reste non mergé et la branche d'intégration revient où elle était.
5. Les enfants terminés, oh écrit à la session parente ; le `reviewer` revoit l'ensemble de la branche d'intégration.
6. **`cp-merge`** (pause dans tous les modes) : le tableau des enfants et le verdict de la revue. oh lance la vérification **sur la branche d'intégration** ; elle ne se force pas. Validé, oh merge la branche d'intégration dans votre branche et ferme les tickets mergés. **Refusé avec une consigne** (« Corriger d'abord », vérification en échec ou revue) : oh lance un **tour de correction** sur la branche d'intégration, une session `quick` dans son worktree avec votre consigne et ce qui a échoué ; vous validez son `cp-commit` (jamais délégué), elle commite, oh vérifie le résultat (commits par-dessus la branche d'intégration, aucun merge, rien de non commité) et le reprend, sinon la branche revient où elle était. La session parente fait relire la correction et repasse `cp-merge`, revérifié sur la branche d'intégration. Au plus `limits.max_fix_rounds` tours (3).
7. **`cp-final`** : récap. Les tickets non mergés (conflit non résolu, échec, budget) restent ouverts : relancez `implement-spec` pour eux.

En semi-auto, trois pauses quel que soit le nombre de tickets : `cp-plan`, `cp-merge`, `cp-final` (deux de plus par tour de correction : son `cp-commit` et le nouveau `cp-merge`).

## Sweep en fan-out

`sweep` suit le même déroulé, avec des sous-tâches au lieu de tickets (`fanout.source: tasks`) :

1. **`cp-plan`** (pause) : le `conductor` passe le découpage du `pathfinder` dans l'argument `tasks` du checkpoint (titre, description, périmètre de fichiers, domaine par sous-tâche). oh le **vérifie avant de vous le montrer** : fichiers du dépôt (ou nouveaux fichiers dans un dossier existant), périmètres **disjoints** deux à deux, au plus 12 sous-tâches, domaine connu ; sinon l'appel est refusé et le `conductor` corrige. La carte montre le découpage tel qu'oh l'a lu, et oh n'enregistre que celui-là (sortie `tasks`).
2. **Sessions enfants** : une session `sweep-task` par sous-tâche enregistrée, sur `oh/fanout/<session>/t1`, `t2`…, avec sa sous-tâche, son périmètre, son domaine, l'objectif et la vérification demandée (`verify`). Son `cp-commit` passe sans pause (remplacé par votre `cp-merge`), oh y lance la vérification dans son worktree et **refuse le commit d'un changement hors du périmètre** de la sous-tâche.
3. **Merge par oh**, revue de l'ensemble, **`cp-merge`** vérifié sur la branche d'intégration, tour de correction si vous le refusez avec une consigne (un tour) : comme ci-dessus. Aucun ticket : rien à fermer.
4. **`cp-recap`** : les sous-tâches mergées ou non.

Sans `[fanout] experimental`, `sweep` garde sa version séquentielle (les sous-tâches dans la session, `cp-commit`) et `-i fanout=true` est refusé. Trois pauses en semi-auto dans les deux versions.

## Nettoyage

oh supprime les worktrees et les branches `oh/fanout/…` après le passage de `cp-merge` (tout est alors sur votre branche), et quand la session parente est arrêtée pour de bon (abandon : il arrête aussi les enfants encore en cours) ; `oh gc` reprend ce qui reste (catégorie `fanout`). Un worktree reste tant qu'une session du fan-out y travaille ou qu'il a des modifications non commitées. Une branche ne part que si chacun de ses commits est sur une autre branche : après un abandon, la branche d'intégration et les enfants non mergés restent, et les événements du fan-out disent pourquoi (`kept`).

```
oh fanout clean <session> --dry-run    # ce qui serait supprimé, ce qui reste et pourquoi
oh fanout clean <session> --discard    # supprime aussi les branches non mergées, leur commit noté (événement discarded)
```

Une branche supprimée avec `--discard` se restaure avec `git branch <branche> <commit>` tant que git garde le commit.

## Suivre

```
oh fanout status <session>        # branche d'intégration, chaque enfant (état, session, merge, détail), budget, derniers événements
oh session list                   # les enfants marqués ↳ (avec leur ticket), la parente ⑂
oh session trace <session>        # trace de la parente, avec les événements du fan-out et les checkpoints passés dans chaque enfant
```

États d'un enfant : `pending`, `running`, `merged`, `conflict` (non résolu), `failed` (aucun commit, modifications non commitées, ticket déjà fermé, dépendance non mergée), `budget` (arrêté ou non lancé par le budget global).

## Budget global

La session parente et toutes ses sessions (enfants et merger) partagent `fanout.budget_usd`. Avant chaque enfant, oh compare ce qu'elles ont dépensé au budget : atteint, plus aucun enfant ne démarre ; dépassé pendant un enfant, oh l'arrête. Chaque enfant reçoit ce qui reste comme budget de session. Tout est noté dans les événements du fan-out et dans le message à la session parente.

## Valider par lot

Les checkpoints en attente de plusieurs sessions (enfants qui gardent leur pause, sessions distantes, `cp-merge` de plusieurs fan-outs) se valident d'un coup, chacun restant listé et refusable :

```
oh session approve --batch --fanout <session> --dry-run
oh session approve --batch --fanout <session> --except <décision> --reject <décision>=<consigne>
```

Dans la TUI, touche `B` de la vue Sessions : les checkpoints en attente du fan-out de la session sélectionnée (sinon tous), chacun avec sa session, son résumé et la vérification d'oh, à valider, laisser en attente ou refuser avec une consigne ; une confirmation, puis le résultat de chacun.

## Garanties

Les garde-fous de la substitution de contrôle (déclarée par la couche hub seulement, `cp-merge` obligatoire et vérifié par oh, enfants sans push, merge ni fermeture, merge par oh, traçabilité, drapeau) sont détaillés dans [Checkpoints](../reference/checkpoints.fr.md#fan-out--substitution-de-contrôle-cp-r3). Une couche d'équipe ne peut pas assouplir le fan-out d'`implement-spec` ni celui de `sweep` : elle peut seulement baisser `fanout.parallel` (et, pour `sweep`, `tasks.max` ou la liste `tasks.domains`). Une couche qui durcit le `cp-2` d'`implement-spec` ou le `cp-commit` de `sweep` le garde hors fan-out, mais ces checkpoints n'existent pas dans une session en fan-out : `oh workflow validate` l'avertit (`fanout_never_patched`) et `oh doctor` liste ces couches ; durcissez aussi `cp-merge`.

## Limites (expérimental)

- `feature` est un alias (`spec`, ou `implement-spec` avec `--tickets`) : `oh run feature --tickets … -i fanout=true` lance donc ce fan-out.
- Un enfant qui attend une réponse dans son texte (sans l'outil `question`) n'est arrêté qu'au garde-fou de 10 minutes et reste non mergé.
- Le pilote suit le fan-out jusqu'au passage de `cp-merge` (pour lancer les tours de correction) : s'il s'est arrêté, le démon le relance (ou `oh fanout drive <session>`).
