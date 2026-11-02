> [Read in English](056-team-claim-lease.en.md)

# ADR-056 — Claims d'équipe : bail, signal de vie, fin de session et file durable

## Statut

Accepté — évolue l'[ADR-024](024-team-state-repository.fr.md) (claims du team-state) ; complète l'[ADR-028](028-tracker-sync-bidirectional.fr.md) (synchro du tracker) et l'[ADR-047](047-session-interaction-daemon.fr.md) (démon)

## Date

2026-11-02

## Contexte

L'[ADR-024](024-team-state-repository.fr.md) a posé les **claims** (réservations de tickets) dans le team-state : un fichier TOML par ticket, avec un propriétaire et une date d'activité. Un claim était « périmé » (`IsStale`) après `stale_days` jours (3 par défaut) sans activité. Trois défauts sont apparus à l'usage du mode équipe :

- **L'activité n'était jamais écrite** : `last_activity` n'était pas persisté (go-toml ignore un `time.Time` tagué `omitempty`), si bien que la péremption se calculait toujours sur `claimed_at`. Une session de plusieurs jours sur un même ticket paraissait abandonnée.
- **La fin d'une session ne disait rien du claim** : tantôt le claim restait indéfiniment, tantôt il était libéré alors que le travail n'était pas fini ou que la revue attendait. Il n'y avait qu'un transfert silencieux quand un claim périmé était repris.
- **Hors ligne, l'effet de la fin de session était perdu** : la libération ou le changement de statut n'étaient poussés que si le dépôt d'équipe répondait à cet instant.

Il fallait aussi dire ce que veut dire « en revue » : un ticket existe dans Beads (ou le tracker) et dans un claim, qui n'ont pas les mêmes statuts, et la colonne REVIEW du board d'équipe ne se calculait pas de la même manière des deux côtés. Beads 1.3 n'a pas de statut `review` ; le faux `bd` d'un job distant ([ADR-046](046-beads-gateways.fr.md)) rejouait au retour un statut que le projet ne connaît pas.

## Décision

Le claim reçoit un **bail**, que renouvelle le démon, et la fin d'une session suit une règle unique, appliquée par une **file durable**. Le détail utilisateur est dans le guide [`team-setup`](../../guides/team-setup.fr.md#bail-et-fin-de-session).

### 1. Bail

- Deux champs facultatifs sur le claim : `lease_until` (date limite, en chaîne RFC 3339 à la seconde, type `Instant` : un `time.Time` tagué `omitempty` serait ignoré par go-toml) et `session` (la session qui travaille). Un claim sans `lease_until` (écrit par oh 5.0) se juge par l'ancienne règle d'inactivité (`IsStale`, `[takeover] stale_days`) : aucune migration.
- `Claim.IsExpired(now, cfg)` : le bail est dépassé s'il existe, sinon repli sur l'inactivité.
- **Seul un claim avec un propriétaire et du travail commencé a un bail.** Un claim du pool (sans propriétaire), `planned`, dans la colonne initiale ou terminé n'en a pas : il n'expire jamais.
- `last_activity` s'écrit en date-heure TOML native, pas en chaîne : une oh 5.0 refuse sinon le claim entier (« cannot decode TOML string »). Un test relit le claim avec une structure de la 5.0.
- Réglages (`[claim]` du `config.toml` du team-state, partagés par l'équipe) : `lease_hours` (24, de 1 à 168) et `heartbeat_minutes` (60, 0 = désactivé, de 5 à 1440, au plus la moitié du bail).

### 2. Signal de vie et plafonds

- Le démon (`ohd`, ou le processus `oh` sous Windows) renouvelle toutes les `heartbeat_minutes` le bail des claims des **sessions ouvertes** (une session en veille est ouverte), en **un seul commit** par dépôt d'équipe, et **aucun** tant que plus de la moitié du bail reste : le bruit de commits reste faible.
- **Le renouvellement est plafonné**, sinon un claim survivrait indéfiniment à une session que personne ne reprend : `sleeping_renewal_days` (3 par défaut) après la mise en veille, `remote_renewal_days` (7) après le lancement d'une session distante. Ensuite le claim expire normalement.
- **Claim pris à la main** (sans session : `oh team claim`, board, plan du tracker) : bail plus long, `lease_hours_manual` (72, de `lease_hours` à 720), que rien ne renouvelle seul. `oh team claim <ticket>` sur son propre claim le renouvelle (`Repo.RenewClaim`) ; démarrer une session le remplace par celui de la session (`AdoptClaim`).

### 3. Fin de session : `DecideClaimEnd`

Fonction pure (`teamstate.DecideClaimEnd`), sans horloge, git ni fichier : l'appelant rassemble les faits (ticket, claim, checkpoints, travail laissé) et traduit la décision en opérations de la file. La première règle qui s'applique :

1. claim déjà terminal : gardé (la rétention le nettoie) ;
2. ticket clos : claim au statut terminal ;
3. ticket ou claim en revue (label `review` ou `ready-for-review`, ou statut `review`) : **gardé** ;
4. checkpoint en attente de décision : gardé, label `session-ended` ;
5. checkpoint de commit reporté alors que la session laisse du travail (ou qu'on ne sait pas le dire) : gardé, `session-ended` ;
6. état des checkpoints illisible : gardé, `session-ended` (un claim libéré à tort est perdu, un claim gardé à tort expire avec son bail) ;
7. sinon : libéré.

Un claim gardé n'est plus renouvelé par la session terminée : il expire avec son bail si personne ne le reprend.

### 4. File durable

Ce que décide la fin de session (libérer, terminer, marquer) et les renouvellements non poussés sont d'abord écrits dans `.git/oh-claims-queue.json` du clone du team-state (local, jamais commité). Chaque opération porte ses conditions (même membre, même session) ; si le claim a changé entre-temps, l'opération est abandonnée et journalisée, jamais appliquée. La file est vidée en un commit (un pull, un push) à la fin de la session, à chaque battement du démon et par la tâche `claims-queue` du démon (au démarrage, puis toutes les 15 minutes). Hors ligne, rien n'est perdu. Une session qui démarre hors ligne sur un claim déjà détenu le reprend au retour du réseau par une opération `adopt`, qui **annule** les opérations en file des autres sessions sur ce claim (sinon l'ancienne libération s'appliquerait à un claim en travail).

### 5. Expiration et reprise

Un claim expiré n'est pas supprimé tout seul. `oh doctor` le signale (`claims_expired`, avertissement) avec la commande pour le garder ou le libérer, et signale la file qui attend (`claims_queue`, `--fix` l'applique). Un autre membre le reprend par `oh team claim <ticket>` : **confirmation réelle** (`--yes` sans terminal, sinon refus, code 2), brief de reprise généré, nouveau bail ; le transfert silencieux d'un claim périmé disparaît. `[claim] release_expired_on_sync` (désactivé) fait libérer les claims expirés par la synchro du tracker, sauf s'ils sont en revue. Un claim libéré volontairement n'est pas recréé par la synchro (`Repo.ClaimRemovedOnPurpose`, lu dans l'historique git).

### 6. Modèle de statut : REVIEW

- **Dans Beads, REVIEW est un label** : `in_progress` + `ready-for-review` (pas de statut `review` en Beads 1.3, sauf statut personnalisé du projet). **Dans le claim, REVIEW est le statut `review`**, posé par la publication d'une revue ou le board.
- **Les deux ne se recopient pas.** La seule équivalence est celle de `DecideClaimEnd` : à la fin d'une session, le label (sur le ticket ou le claim) vaut le statut `review` ; dans les deux cas le claim survit.
- **Le faux `bd` d'un job distant refuse un statut que le projet ne configure pas** (code 1, avant toute écriture, casse exacte ; les statuts voyagent dans l'instantané, `Snapshot.Statuses`) : l'agent utilise le label au lieu d'échouer au rejeu.

## Conséquences

### Positives

- Un claim ne survit plus indéfiniment, ne disparaît plus par accident, et le dit : `oh doctor` donne la commande.
- La fin de session est testable (fonction pure) et ne perd rien hors ligne.
- Compatibilité avec la 5.0 : champs ignorés par l'ancienne version, `last_activity` relisible, repli sur `IsStale`.
- Le mode équipe n'est plus signalé comme bêta : ses limites connues sont des règles documentées.

### Négatives / compromis

- **Bruit de commits** dans le team-state (un commit par dépôt à chaque renouvellement utile), borné par la période, la moitié du bail et le réglage `heartbeat_minutes = 0`.
- **Une ancienne oh qui réécrit un claim** efface `lease_until` : le claim retombe sur la règle d'inactivité (sans casse).
- Un claim pris à la main expire s'il n'est pas renouvelé à la main ; une session `sleeping` ou distante cesse de renouveler au plafond, même si elle est reprise plus tard (`oh team claim` ou une nouvelle session renouvelle).
- Un clone superficiel est traité comme un claim perdu ; la fenêtre entre le vidage de la file et l'adoption hors ligne reste étroite.
- Trois mécanismes à comprendre (bail, file, règle de fin) au lieu d'un champ d'activité.

## Alternatives envisagées

| Alternative | Rejetée parce que |
|-------------|-------------------|
| Garder l'inactivité (`stale_days`) comme seule règle | Inobservable (l'activité n'était pas persistée) et trop lente ou trop brutale selon l'équipe ; ne distingue pas une session vivante d'un claim oublié |
| Verrou ou service central pour les claims | Le team-state est un dépôt git sans infrastructure ([ADR-024](024-team-state-repository.fr.md)) ; un service ajouterait un serveur à exploiter |
| Libérer toujours le claim à la fin de la session | Perd le claim d'un ticket en revue ou dont le travail n'est pas commité |
| Ne jamais libérer, laisser le bail expirer | Un ticket terminé proprement resterait réservé jusqu'à l'expiration |
| Renouveler sans plafond | Une session en veille oubliée ou un job distant mort garderait le claim indéfiniment |
| Pousser la fin de session immédiatement, sans file | Perdue hors ligne ou si oh est tué ; la fin de session ne doit pas dépendre du réseau |
| Recopier le label Beads sur le statut du claim (et inversement) | Deux sources écrites par deux chemins : divergences et conflits au rejeu ; l'équivalence reste à un seul endroit |
