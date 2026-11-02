> [Read in English](060-living-wiki.en.md)

# ADR-060 — Wiki documentaire vivant : les agents proposent, l'utilisateur valide, oh applique et vérifie

## Statut

Accepté — décision appliquée depuis juin 2026 (arborescence, tags, god nodes) et octobre 2026 (serveur MCP `wiki`, file de propositions, vérifications par oh) ; consignée ici après coup, comme l'annonçait [le document d'architecture](../living-wiki.fr.md). S'appuie sur l'[ADR-039](039-declarative-workflows-oh-v1.fr.md) (workflows, `writes:`), l'[ADR-042](042-checkpoints-headless-decisions.fr.md) (cartes de décision) et l'[ADR-047](047-session-interaction-daemon.fr.md) (démon qui sert les outils) ; le wiki d'équipe du team-state reste celui de l'[ADR-024](024-team-state-repository.fr.md)

## Date

2026-11-02

## Contexte

Le contexte d'un projet (stack, conventions, règles métier, décisions) tenait dans quatre fichiers plats (`ONBOARDING.md`, `CONVENTIONS.md`, `docs/context/technical.md`, `docs/context/business/<domaine>.md`). Chaque agent les lisait en entier, sans savoir ce qui était sûr ou supposé. Ils vieillissaient sans que personne le sache, et la capitalisation des découvertes d'une session dépendait de la présence du `documentarian` dans le workflow — absent de 8 workflows sur 14 — ou d'une recopie par les coordinateurs qui perdait l'information (rapport de modernisation, § 5.3 : [`rapport-modernisation-2026-10.md`](../../dev/rapport-modernisation-2026-10.md)).

Il fallait un référentiel de connaissance du projet qui soit lu de façon ciblée (peu de tokens), honnête sur sa fiabilité, enrichi par tous les agents sans que l'un d'eux puisse y écrire n'importe quoi, et vérifié sans demander à un agent de le faire.

## Décision

Le projet porte un **wiki documentaire vivant** dans `docs/wiki/` (détail : [Wiki documentaire vivant](../living-wiki.fr.md)), avec ces règles :

1. **Une arborescence unique** : `index.md` (carte globale lue en premier), `glossary.md`, `technical/`, `business/`, `specs/`, `decisions/`, `postmortems/`. `ONBOARDING.md` n'est plus qu'un résumé qui renvoie au wiki. `wiki_check` signale une page hors de cette arborescence.
2. **Lecture ciblée** : un agent lit `index.md`, puis une seule page ou section (skill `wiki-navigation`, outils `wiki_index` et `wiki_read`). Au niveau d'instructions `index` du projet, l'index et les god nodes entrent dans le prompt.
3. **Tags de confiance** : chaque enrichissement porte `CONFIRMÉ` (observé, avec `fichier:ligne`), `DÉDUIT` ou `INCERTAIN`, l'agent, la date et la source. Le frontmatter de la page résume la confiance.
4. **God nodes calculés par oh**, pas par un agent : un concept cité par au moins deux pages (hors cartes). Le service `services/wiki` les recalcule, avec leur criticité, et réécrit la table de l'index.
5. **L'agent propose, l'utilisateur valide, oh applique** : un agent ne modifie pas le wiki. Il appelle `wiki_propose` (serveur MCP `wiki`, injecté dans le paquet d'un projet qui a un `docs/wiki/`) ; oh fixe l'auteur, nettoie et borne le contenu, refuse toute référence aux données de session d'oh, et met la proposition en file (`wp-<n>`, par projet, en base). La carte du checkpoint de commit affiche les propositions de la session : l'utilisateur accepte, modifie ou refuse chacune (un refus garde son motif, `wiki_pending` le montre aux agents pour qu'ils ne reproposent pas la même chose). **oh applique lui-même** les propositions acceptées au passage du checkpoint, de façon déterministe, avec la ligne de tag écrite par oh : elles sont commitées avec le code. Les workflows sans commit laissent la file au workflow `doc-sync`. Seul le `documentarian` a `wiki_apply`.
6. **Vérification déterministe par oh** (`oh wiki check`, `wiki_check`) : frontmatter, liens, tags, fraîcheur (page plus ancienne que le dernier commit d'un fichier qu'elle cite), god nodes, arborescence. Restreinte à un changement, elle détecte la **dérive** (une page `CONFIRMÉ` que le changement contredit, un god node touché sans mise à jour) : le `reviewer` en tire des findings « Documentation ». `oh wiki check --fix` recalcule confiance et god nodes.
7. **Session distante** : mêmes outils sur le clone du job ; **rien n'est appliqué dans le job**, les propositions reviennent dans l'artefact `wiki.json` et sont validées sur la machine.
8. **Écritures directes limitées** : trois workflows écrivent un dossier du wiki (`spec`, `incident`, `spike`), déclaré par `writes:` et commité seul ; l'`onboarder` crée le wiki, puis ne fait plus que proposer (re-onboarding).

## Conséquences

### Positives

- Les agents lisent peu et savent ce qui est fiable.
- La capitalisation ne dépend plus de la présence du `documentarian` : tous les workflows, y compris en lecture seule, proposent.
- Aucune écriture n'échappe à l'utilisateur ; l'application, la numérotation et les tags sont du code, pas du prompt.
- Le wiki est vérifié sans modèle (`oh wiki check` ne demande aucun modèle) ; le dépôt de oh passe sa propre vérification (`TestOwnWiki`).

### Négatives / compromis

- Un wiki à entretenir : sans validation des propositions ni `doc-sync`, la file s'allonge (les statistiques `oh wiki stats` la montrent).
- La carte du checkpoint de commit porte une charge de décision de plus.
- Le calcul des god nodes est heuristique (termes en gras, identifiants en code) : un concept mal détecté se corrige à la main dans l'index.
- Un wiki non commité n'a rien à synchroniser : la marque de synchronisation (`synced:`) vit dans le dépôt, avec le wiki.

## Alternatives envisagées

| Alternative | Rejetée parce que |
|-------------|-------------------|
| Garder les fichiers plats | Lus en entier à chaque session, sans niveau de confiance ni vérification |
| Laisser les agents écrire le wiki directement | Aucun contrôle de l'utilisateur ; contenu non nettoyé ; conflits entre sessions parallèles |
| Passer par le `documentarian` pour toute écriture | Absent de la plupart des workflows ; un tour d'agent de plus (coût, non-déterminisme) au milieu du workflow |
| Proposer dans le rapport et laisser le coordinateur recopier | Perd l'information (retranscription) — d'où `workflow_report` ([ADR-059](059-return-envelope-and-workflow-report.fr.md)) et le serveur `wiki` |
| God nodes comptés par un agent | Non reproductible ; oh les calcule à partir des pages |
| Un seul wiki pour tous les projets (celui de l'équipe) | Le wiki d'équipe du team-state est transversal ; le wiki du projet doit voyager avec la branche et le code |
