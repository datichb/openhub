> [Read in English](fan-out.en.md)

# Fan-out : une session enfant par ticket (expérimental)

Le fan-out fait implémenter les tickets d'une spec par des **sessions enfants**, une par ticket, chacune dans sa branche et son worktree, parfois en même temps. oh les merge lui-même dans une branche d'intégration, fait résoudre par le `merger` un conflit qu'il ne résout pas seul, et ne merge le tout dans votre branche qu'après **une seule validation**, `cp-merge`, vérifiée par oh sur la branche d'intégration. Il est **expérimental** et désactivé par défaut.

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
```

`oh run` démarre aussi, en arrière-plan, le **pilote** du fan-out (`oh fanout drive <session>`, journal dans `~/.oh/sessions/<session>/fanout.log`) : c'est lui qui lance, suit et merge les sessions enfants, que vous gardiez ou non le client de la session parente ouvert. S'il s'est arrêté (machine éteinte), relancez-le : `oh fanout drive <session>` reprend là où il en était.

## Déroulé

1. **`cp-plan`** (pause) : l'`orchestrator-dev` présente les tickets, leurs dépendances, ceux déjà fermés. Rien ne démarre avant.
2. **Sessions enfants** : oh crée la branche d'intégration `oh/fanout/<session>/integration` depuis votre branche, puis une session `ticket` par ticket, sur `oh/fanout/<session>/<ticket>`, dans un worktree voisin du projet. Un ticket attend ceux dont il dépend ; aucune session pour un ticket fermé. Les enfants héritent du mode et de la méthode (`tdd`) de la session parente.
3. Chaque enfant implémente, fait relire et passe **son `cp-2` sans pause** : il est remplacé par votre `cp-merge` (substitution de contrôle, CP-R3). oh y lance quand même la vérification (build, tests) dans le worktree de l'enfant ; un échec renvoie le travail à l'enfant. L'enfant commite sur sa branche, mais ne ferme pas son ticket, ne merge pas et ne pousse pas : oh le refuse.
4. **Merge par oh** : un enfant terminé, oh merge sa branche dans la branche d'intégration. Un conflit : oh lance le `merger` (workflow `merge-conflicts`) dans le worktree d'intégration, puis vérifie le résultat (merge commité, les deux côtés présents, aucun marqueur). Sinon l'enfant reste non mergé et la branche d'intégration revient où elle était.
5. Les enfants terminés, oh écrit à la session parente ; le `reviewer` revoit l'ensemble de la branche d'intégration.
6. **`cp-merge`** (pause dans tous les modes) : le tableau des enfants et le verdict de la revue. oh lance la vérification **sur la branche d'intégration** ; elle ne se force pas. Validé, oh merge la branche d'intégration dans votre branche et ferme les tickets mergés.
7. **`cp-final`** : récap. Les tickets non mergés (conflit non résolu, échec, budget) restent ouverts : relancez `implement-spec` pour eux.

En semi-auto, trois pauses quel que soit le nombre de tickets : `cp-plan`, `cp-merge`, `cp-final`. Les branches et worktrees `oh/fanout/…` restent après la session : supprimez-les quand vous n'en avez plus besoin (`git worktree remove`, `git branch -D`).

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

## Garanties

Les garde-fous de la substitution de contrôle (déclarée par la couche hub seulement, `cp-merge` obligatoire et vérifié par oh, enfants sans push, merge ni fermeture, merge par oh, traçabilité, drapeau) sont détaillés dans [Checkpoints](../reference/checkpoints.fr.md#fan-out--substitution-de-contrôle-cp-r3). Une couche d'équipe ne peut pas assouplir le fan-out d'`implement-spec` : elle peut seulement baisser `fanout.parallel`.

## Limites (expérimental)

- Pas de tour de correction dans la session parente : un nouvel `implement-spec` pour les tickets restants.
- La validation par lot existe en ligne de commande ; dans la TUI, chaque carte se valide encore une à une.
- `sweep` n'utilise pas encore ce mécanisme. `feature` est un alias (`spec`, ou `implement-spec` avec `--tickets`) : `oh run feature --tickets … -i fanout=true` lance donc ce fan-out.
- Un enfant compte comme terminé quand il reste inactif, sans décision en attente, 20 secondes : un enfant qui attend une réponse dans son texte (sans l'outil `question`) est arrêté et reste non mergé.
