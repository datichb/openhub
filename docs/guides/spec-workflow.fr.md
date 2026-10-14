> [Read in English](spec-workflow.en.md)

# Écrire une spec avec le workflow `spec`

Le workflow `spec` transforme une demande en une spec écrite dans le wiki du projet (`docs/wiki/specs/<sujet>.md`), puis en tickets Beads, sans implémenter. La spec est vérifiée par oh avant que vous la validiez : un glossaire, un point de test pour chaque critère, un découpage qui couvre chaque critère et rien d'autre.

```bash
oh run spec -i request="Limiter à 3 le nombre de réservations en cours par abonné"
```

En bref :

1. le planner explore le code et vous pose ses questions (une par décision) ;
2. **`cp-understanding`** : vous validez sa reformulation et son vocabulaire (chaque mot de la demande en face des noms du code) ;
3. il écrit la spec : objectif, périmètre, glossaire, critères d'acceptation avec leur point de test, découpage en tranches ;
4. **`cp-converge`**, sans pause : oh vérifie la spec et renvoie au planner ce qui manque ;
5. **`cp-spec`** : vous validez la spec, le planner la commite (ce fichier seul) ;
6. **`cp-tickets`** : vous validez le découpage, le planner crée un ticket par tranche.

En semi-auto, trois pauses : la compréhension, la spec, le découpage.

## Ce que contient la spec

```markdown
## Glossaire
- **Réservation** : file d'attente sur un livre déjà prêté — code : `Reservation` ; à ne pas confondre avec `Reserve`, qui crée un prêt

## Critères d'acceptation
- **CA-1** — un 4ᵉ prêt en cours est refusé avec un message clair · Test : `TestReserve_LimitReached`

## Découpage
- **T1** — plafond des prêts par adhérent · Couvre : CA-1 · Agent prévu : developer (backend)
```

Chaque critère porte un identifiant (`CA-1`) et, sur sa ligne, le test qui le vérifiera (son nom, ou la commande). Chaque tranche cite les critères qu'elle couvre. C'est ce qu'oh vérifie à `cp-converge`, `cp-spec` et `cp-tickets` ([`verify.spec`](../reference/workflow-schema.fr.md#verify--vérification-lancée-par-oh)) : un critère sans point de test, un critère qu'aucune tranche ne couvre ou une tranche hors spec refusent le checkpoint, et le planner corrige avant que la spec vous soit montrée.

## Ce que le planner peut écrire

Le planner reste en lecture seule ; le workflow lui ouvre seulement `docs/wiki/specs/**` (champ [`writes`](../reference/workflow-schema.fr.md#writes-chemins-de-documentation)) : il crée et modifie sa spec, et la commite seule (`git commit -- docs/wiki/specs/<sujet>.md`) une fois `cp-spec` passé. Tout autre fichier, votre travail en cours compris, n'est ni modifié ni commité. Une équipe peut restreindre ces chemins (`docs/wiki/specs/equipe/**`), jamais les élargir.

## Changer la spec en cours de route

Refusez `cp-spec` ou `cp-tickets` avec une consigne (« ajoute une règle : … », « ajoute une tranche pour la CI ») : le planner modifie le fichier, qui repasse par `cp-converge`, `cp-spec` (nouveau commit) et `cp-tickets`. Une tranche demandée qui ne couvre aucun critère est refusée par oh tant que la spec n'a pas le critère correspondant.

## Et `cadrage` ?

`cadrage` reste disponible : exploration par le pathfinder, découpage en tickets, aucun fichier écrit. Choisissez `spec` quand la spec doit rester dans le wiki et servir de référence aux tickets. Détails des deux workflows : [Workflows livrés](../reference/workflows.fr.md#spec).
