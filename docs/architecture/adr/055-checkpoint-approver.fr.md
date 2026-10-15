> [Read in English](055-checkpoint-approver.en.md)

# ADR-055 — Valideur humain d'un checkpoint (`approver:`, CP-R9), étape 1

## Statut

Accepté (étape 1 ; étape 2 à venir)

## Date

2026-10-15

## Contexte

`by:` dit quel **agent** passe un checkpoint (CP-R11). Rien ne disait quel **humain** doit le valider : en équipe, n'importe quel membre qui a la session sous les yeux valide `cp-apply` ou `cp-publish`, et l'historique ne dit pas qui l'a fait. Le rapport de modernisation (CP-R9) demande un valideur par checkpoint (par exemple un lead pour `cp-apply` et `cp-publish`), déclaré par l'équipe.

Deux limites cadrent la décision :

- **En local, oh ne peut pas imposer un valideur.** La personne devant la machine est celle qui lance oh : oh ne connaît d'elle que ce que git et le système lui disent (`user.name`, `user.email`, nom de connexion), qu'elle peut changer. Refuser une validation serait une protection de façade.
- **L'identité d'une personne existe déjà** : l'[ADR-052](052-beads-identity-provenance.fr.md) fixe l'auteur des écritures Beads à `user.name` de git dans l'emplacement de la session (sinon le nom de connexion).

## Décision

### 1. Champ `checkpoints.<id>.approver`

```yaml
checkpoints:
  cp-apply:
    approver: [Grace Hopper, lead@example.com]   # ou une seule identité : approver: Grace Hopper
```

- **Format** : une identité ou une liste d'identités (au plus 20). Une identité est un nom (`user.name` de git, l'identité de l'ADR-052) ou une adresse e-mail (`user.email`, reconnue au `@`), sur une ligne, 100 caractères au plus. Comparaison sans casse ni espaces en trop ; une entrée avec `@` se compare à l'e-mail, les autres au nom.
- **Pas de rôle** : team-state n'a pas d'annuaire de rôles ; un rôle (« lead ») s'écrit comme la liste de ses membres. Un rôle nommé viendra avec un annuaire, s'il y en a un.
- **Couche d'équipe seulement** : le champ n'est accepté que d'un document de la couche `team` (publié ou brouillon d'équipe). Le hub (workflows livrés), une couche de projet et les options de lancement ne l'écrivent pas (`checkpoint_approver_layer`, erreur).
- **Ne fait que se durcir** : une couche de projet ne le retire ni ne le change ; elle ne retire pas le checkpoint (`approver_checkpoint_removed`) et ne le fait pas moins mettre en pause dans un mode (`loosening`), comme pour un checkpoint obligatoire. Une couche d'équipe qui étend un autre document d'équipe peut le changer. Retirer un valideur est un assouplissement dans le résumé d'impact (`checkpoint_approver_removed`) : un brouillon qui le fait est refusé au lancement.
- Avertissements : `checkpoint_approver_no_pause` (le checkpoint ne met en pause dans aucun mode : personne ne sera sollicité), `checkpoint_approver_remote_auto` (en session distante, la politique `remote: auto` le passe seule), `checkpoint_approver_duplicate`.

### 2. Effet en local : afficher, avertir, tracer

- **Carte** (TUI et `oh session approve`) : « Valideur attendu : … ». Si la personne qui valide (identité de l'ADR-052, `user.name <user.email>` de l'emplacement de la session) n'en fait pas partie : avertissement clair, **pas de blocage**. Dans la TUI, `y` demande une confirmation ; le formulaire de décision le rappelle ; la validation par lot (`oh session approve --batch`, TUI `B`) l'affiche pour chaque élément.
- **Traçage, toujours** (avec ou sans valideur) : chaque validation et chaque refus d'un humain enregistrent dans les événements du checkpoint qui (`identity`), quand (`at`), les valideurs attendus (`approver`) et, s'il y a lieu, `approver_mismatch: true` ; `oh session trace` (texte et JSONL) et son résumé les reprennent. Un passage par le mode (`auto`) ou par délégation (CP-R3) n'a pas d'identité. Dans un job distant, la politique `remote: auto` valide sous l'identité `remote` : tracée hors valideur.
- Aucune migration : l'état des checkpoints est un document JSON.

### 3. Étape 2 (non faite) : blocage réel en session distante

Ce qui la complétera :

- une identité **attestée** et non déclarée : le compte de la forge (GitLab) qui répond à une session distante, ou une signature de la validation ;
- un checkpoint avec valideur qui ne passe, en session distante, que sur la réponse d'un valideur : `remote: auto` refusé (aujourd'hui un avertissement), `remote: defer` qui attend la validation d'un valideur, sur sa machine ou depuis la forge ;
- la lecture de `approver_mismatch` déjà tracé pour signaler, au rapatriement d'une session, une validation faite hors valideur.

## Conséquences

### Positives

- En équipe, chaque checkpoint sensible dit qui doit le valider, et l'historique dit qui l'a validé, quand, et si c'était hors valideur.
- La règle de couche est celle de la gouvernance d'équipe ([ADR-040](040-workflows-team-state-governance.fr.md)) : un projet ne peut pas retirer un valideur posé par l'équipe.
- Les données de l'étape 2 sont déjà enregistrées.

### Négatives / compromis

- **Pas de garantie en local** : n'importe qui peut changer `user.name` ou valider quand même ; oh le dit et le trace, rien de plus.
- L'identité est lue avec git à chaque validation (mise en cache une minute par emplacement).
- Une identité écrite à la main (nom ou e-mail) doit correspondre à la configuration git du valideur.

## Alternatives envisagées

| Alternative | Rejetée parce que |
|-------------|-------------------|
| Refuser en local une validation hors valideur | L'identité est déclarée par la personne elle-même : protection de façade, et blocage d'une équipe dont le valideur est absent |
| Rôles nommés (`approver: lead`) | Aucun annuaire de rôles dans team-state ; une liste d'identités suffit et reste vérifiable |
| Valideur déclarable par le projet | Un projet pourrait le retirer ou le remplacer par lui-même : la règle ne serait plus celle de l'équipe |
| Valideur dans le hub | Les workflows livrés ne connaissent pas les personnes d'une équipe |
