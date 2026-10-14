> [Read in English](053-fanout-control-substitution.en.md)

# ADR-053 — Fan-out : sessions enfants pilotées par oh et substitution de contrôle (CP-R3)

## Statut

Accepté (expérimental, derrière `[fanout] experimental`)

## Date

2026-10-14

## Contexte

`implement-spec` (R5) traite les tickets d'une spec l'un après l'autre dans une seule session, avec un `cp-2` par ticket : N tickets donnent N + 2 pauses. Le rapport de modernisation (§ 9.2 à 9.5) demande des sessions enfants, une par ticket, chacune dans son worktree, mergées par oh, et une seule validation groupée ; mais une couche ne peut que durcir, et le `cp-2` d'un enfant est obligatoire. Remplacer ce `cp-2` est un assouplissement, qui ne se défend que si un contrôle au moins aussi fort le remplace et si personne d'autre ne peut le déclarer.

Trois contraintes techniques : le démon ne lance pas de session (seul le processus `oh` a le lanceur et les identifiants) ; un agent ne peut pas attendre longtemps dans un appel d'outil ; et une session inactive s'endort.

## Décision

1. **oh pilote, l'agent ne lance rien.** Le pilote du fan-out (`services/fanout`) tourne dans un processus `oh` : `oh fanout drive`, démarré en arrière-plan par `oh run`, ou le passage d'une évaluation. Après `cp-plan`, il crée la branche d'intégration, lance une session enfant `ticket` par ticket (1 à 3 à la fois, dépendances d'abord), attend la fin de son travail (checkpoint de commit passé, un commit, rien de non commité ; 10 minutes d'inactivité au plus), la merge lui-même (`--no-ff`), lance le `merger` sur un conflit et vérifie son résultat, puis écrit à la session parente, restée inactive pendant ce temps (aucun coût). Tout l'état est dans l'enregistrement `fanout` de la session parente (migration v46), écrit par oh seul : un pilote relancé reprend.
2. **Substitution déclarée par le hub, validée par le schéma.** `fanout.delegates` n'est accepté que d'un document publié de la couche hub (`fanout_delegates_layer`), avec un checkpoint de merge obligatoire, `fanout: only`, qui déverrouille `merge` et vérifié par oh sur la branche d'intégration. Un patch ne peut que baisser `parallel`.
3. **Revérifiée par le démon à chaque appel.** Le `cp-2` d'un enfant ne fait pas de pause seulement si l'enregistrement de l'enfant (écrit par oh au lancement) désigne une session parente en cours, du même projet, lancée avec le drapeau, qui le liste, et dont le paquet délègue ce checkpoint à un checkpoint de merge conforme (`Substitutes`). Sa vérification tourne toujours ; le passage est tracé (`delegated`).
4. **Le contrôle qui remplace est plus fort.** `cp-merge` ne passe qu'une fois tous les enfants terminés, sur une branche d'intégration où oh l'a laissée, après sa vérification sur cette branche, sans « passer outre » ; oh merge alors la branche d'intégration et ferme les tickets mergés.
5. **Aucune session du fan-out ne pousse, ne merge, ne ferme de ticket ni ne déplace ses branches** (règles de session `FanoutRoleRules`, passerelle Beads) ; la parente ne commite pas.
6. **Opération `merge`** verrouillable, déverrouillée par le seul checkpoint de merge d'un fan-out, et faite par oh (aucune fenêtre pour les agents).

## Conséquences

- Une feature de N tickets : 3 pauses en semi-auto (`cp-plan`, `cp-merge`, `cp-final`) au lieu de N + 2. Validation par lot en ligne de commande (`oh session approve --batch`, CP-R6) pour les pauses qui restent.
- Le fingerprint de la configuration git d'une session est maintenant enregistré par worktree : avant, une session lancée dans un autre worktree du dépôt faisait passer la configuration pour modifiée.
- Non fait en R6, ajouté ensuite (lot r6-leftovers) : validation par lot dans la TUI ; tour de correction quand `cp-merge` est refusé, par une session de correction (`fanout.fix`, rôle `fixer`) jamais déléguée, `cp-merge` revérifié ensuite, la substitution inchangée ; nettoyage des worktrees et branches `oh/fanout/…` (jamais une branche non mergée sans `--discard`) ; entrées transmises aux enfants (`fanout.inputs`) ; fin d'un enfant constatée par oh (inactivité : garde-fou de 10 minutes). Puis (lot sweep-fanout) : `sweep` sur le fan-out (`fanout.source: tasks`, enfants `sweep-task` issus du découpage relevé et vérifié par oh à `cp-plan`, `cp-commit` délégué à `cp-merge`, périmètre de chaque enfant vérifié par oh, aucun ticket) ; `feature` est un alias de `spec` → `implement-spec` (lot feature-migration).
