> [Read in English](052-beads-identity-provenance.en.md)

# ADR-052 — Identité et provenance Beads : `--readonly`, auteur, événements de provenance

## Statut

Accepté

## Date

2026-10-12

## Contexte

La passerelle Beads ([ADR-046](046-beads-gateways.fr.md)) vérifie chaque commande `bd` avec la liste blanche du workflow (`beads.allow`) et la table des commandes et options de bd 1.3.1 (`bdargs`). Une revue de bd 1.3.1 face à ce qu'oh utilise a relevé quatre manques :

- **Une seule protection.** Si la table classait mal une commande, ou si une version plus récente de bd changeait une commande, rien n'empêchait une écriture dans une session en lecture seule.
- **La « lecture seule » ne l'était pas.** La liste par défaut (sans bloc `beads:`) contient `comments`, et une entrée d'un seul mot autorise toutes ses sous-commandes : `bd comments add` passait. `ready` laissait passer `ready --claim` de la même façon. Le validateur de `risk: read` utilisait une liste noire (`BeadsWriteCommands`) qui oubliait `note`, `assign`, `tag`, `priority`, `defer`, `todo`, `batch`, `import`, `kv`, `remember`, `set-state`, `heartbeat`, `unclaim`, `rename`, `link`, `config`, `provenance`…
- **Pas d'identité.** Chaque écriture était enregistrée sous l'auteur par défaut de bd (`user.name` de git) : l'historique d'un ticket ne distinguait pas un agent de la personne, ni quel agent.
- **Pas de trace du travail.** Rien ne reliait un ticket à la session qui l'a réservé, au commit sur lequel il a été fermé, ni à la MR d'un job distant ; seulement des notes et `external_ref`.

bd 1.3.1 offre ce qui manque : une option globale `--readonly` (bd refuse les écritures), `--actor` (enregistré dans `bd history --events`), et `bd provenance record`, un journal d'événements liés à un ticket, sans modification possible (`claim`, `commit`, `handoff`… avec une référence `git-sha`, `pr`, `work-id`), idempotent sur `source:issue:kind:ref`.

## Décision

### 1. Une seule liste des commandes de lecture

`bdargs` contient une liste fermée des commandes bd qui n'écrivent jamais (`show`, `list`, `ready`, `search`, `query`, `count`, `children`, `comments`, `dep tree`, `label list-all`, `provenance log`, `history`… ; `ready --claim` est une écriture). Une commande absente de la liste compte comme une écriture.

- **Session en lecture seule** : une session dont la liste ne nomme que des commandes de lecture (liste par défaut comprise) n'exécute que les commandes de cette liste. `comments add` et `ready --claim` sont refusés (`cmd.gateway.beads.read_only`), sur la machine et au rejeu d'un journal distant.
- **`risk: read`** : chaque entrée de `beads.allow` doit nommer une commande de lecture (la liste noire disparaît). `risk: plan` utilise la même règle pour savoir si le workflow écrit.
- La liste par défaut gagne `query`.

### 2. `--readonly` sur chaque lecture

La passerelle ajoute `--readonly` à chaque commande de lecture qu'elle lance (et à son propre `bd statuses`), quel que soit le workflow. Les écritures ne changent pas. Dans un job distant, le mode journal du faux bd refuse une écriture qui porte `--readonly`, et une lecture qu'il ne sait pas servir depuis l'instantané (`query`…) échoue au lieu d'être enregistrée comme une écriture.

`--readonly` est une **deuxième protection** : bd 1.3.1 laisse encore passer `config set`, `hooks install`, `backup` et `import` dans ce mode (testé). La liste blanche et la table restent la première protection.

### 3. Auteur fixé par oh

Chaque écriture lancée par la passerelle porte `--actor=<personne> via oh/<agent>` :

- **personne** : `user.name` de git dans l'emplacement de la session (sinon le nom de connexion), fixé à l'émission de l'autorisation (`Grant.Human`, signé avec le fichier des autorisations) ;
- **agent** : l'agent d'entrée de la session, ou le sous-agent d'une sous-session (`subagent` tant que son nom n'est pas connu).

Les **réservations** (`update --claim`, `--status in_progress`, `ready --claim`) gardent la personne seule : bd fait de l'auteur d'une réservation l'assigné du ticket, et le tableau, `ready --assignee` et la synchronisation des trackers en dépendent. L'agent d'une réservation figure dans son événement de provenance.

Les agents ne peuvent pas choisir l'auteur : `--actor` rejoint les options refusées. Au rejeu, la machine réécrit l'auteur de chaque entrée : sa propre personne, l'agent enregistré par le job, marqué `(remote)` ; une réservation rejouée garde l'auteur propre de bd.

### 4. Provenance enregistrée par oh seulement

oh enregistre des événements avec `bd provenance record --source=oh`. Les agents jamais : `provenance record` est une écriture qu'aucun workflow d'oh n'autorise.

| Événement oh | `--kind` | `--ref-kind` / `--ref` | payload |
|---|---|---|---|
| Réservation via la passerelle | `claim` | `work-id` / ID de session | workflow |
| Fermeture via la passerelle (après le verrou) | `commit` | `git-sha` / HEAD | session, branche |
| Session distante envoyée | `claim` | `work-id` / ID de session | runtime, workflow, pipeline |
| Session distante récupérée, commit poussé | `commit` | `git-sha` / commit poussé | session, branche |
| Session distante récupérée, MR en brouillon | `handoff` | `pr` / URL de la MR | session |

Un échec est seulement journalisé : l'opération qu'il décrit a déjà eu lieu. Le démon du job n'enregistre rien (son bd est un journal) ; la machine enregistre les événements du job quand elle récupère la session.

### 5. Baux entretenus par le démon

Une réservation de bd 1.3.1 porte un bail de 5 minutes, renouvelé par `bd heartbeat` ; une fois expiré, `bd reclaim` (lancé par n'importe qui) rend le ticket. Le démon renouvelle chaque minute, avec la personne de l'autorisation (propriétaire du bail), les baux des tickets que chaque session ouverte a réservés via la passerelle ou que son état de workflow liste comme réservés et non fermés. Il s'arrête à la fin de la session ou quand bd refuse (ticket fermé, rendu, réservé par quelqu'un d'autre).

### 6. Pas de modèle extérieur

`bd find-duplicates --method ai` envoie les tickets à un modèle extérieur (API compatible Anthropic) : la passerelle refuse toute méthode autre que la méthode mécanique par défaut, comme toute donnée qui sortirait du monde fermé.

## Conséquences

### Positives

- Une session en lecture seule ne peut pas écrire dans Beads, même via une commande que la table connaît mal : bd la refuse aussi.
- L'historique d'un ticket dit qui l'a modifié : la personne et l'agent oh, exécutions distantes signalées.
- Un ticket est relié à sa session, son commit et sa MR (`bd provenance log <id>`), sans lire les notes.
- Une seule classification lecture/écriture pour la passerelle, le validateur et le rejeu.

### Négatives / compromis

- **Changement de comportement** : un workflow sans bloc `beads:` ne peut plus ajouter de commentaires ; les workflows `risk: read` qui autorisaient une commande d'écriture absente de l'ancienne liste noire deviennent invalides.
- La liste de lecture est tenue à la main à côté de la table générée ; une nouvelle commande de lecture d'une version plus récente de bd compte comme une écriture tant qu'elle n'est pas ajoutée.
- `bd provenance record` coûte un appel `bd` de plus par réservation et par fermeture.
- Les références `git-sha` doivent faire 40 caractères hexadécimaux : un dépôt en SHA-256 n'enregistre pas d'événement `commit` (journalisé).

## Alternatives envisagées

| Alternative | Rejetée parce que |
|-------------|-------------------|
| `--readonly` sur toutes les commandes d'une session en lecture seule | Équivalent, puisqu'une session en lecture seule n'exécute que des lectures ; `--readonly` sur les seules lectures couvre aussi les sessions qui écrivent |
| Auteur `<agent>` seul | Perd la personne ; une réservation assignerait le ticket à l'agent |
| Même auteur pour les réservations | Change l'assigné des tickets réservés (tableau, `ready --assignee`, synchronisation des trackers) |
| Laisser les agents enregistrer la provenance | Un agent pourrait relier un ticket à n'importe quel commit ou MR ; les faits sont connus d'oh |
| `bd serve` (API HTTP) au lieu de la CLI | Demande un serveur Dolt (les projets oh utilisent Dolt embarqué), sans authentification par défaut sur la boucle locale, refuse `--readonly` ; étudié pour plus tard |
