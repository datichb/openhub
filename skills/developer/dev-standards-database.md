---
name: dev-standards-database
description: "Utiliser quand le ticket touche schéma, migrations ou requêtes : contraintes, index, migrations sans coupure, EXPLAIN, N+1."
---

# Skill — Standards base de données

Complète les standards backend du domaine. Pour toute migration destructive (`DROP`, `TRUNCATE`, `DELETE`
de masse, renommage de colonne utilisée ailleurs), le protocole de `dev-standards-migration`
s'applique : dry-run, documentation dans le handoff, ticket bloqué et escalade.

## Données réelles

Tu travailles sur le schéma, les migrations et les requêtes. Les données d'un environnement partagé
(production, recette) ne se modifient qu'avec la confirmation explicite de l'utilisateur ; les tests
utilisent des fixtures.

## Schéma

- Contraintes dans la base, pas seulement dans le code : `NOT NULL` par défaut, `UNIQUE` sur les clés
  métier, `CHECK` sur les domaines de valeurs, clés étrangères avec un `ON DELETE` choisi
  (`RESTRICT` par défaut, `CASCADE` seulement pour une composition).
- Types natifs adaptés : `uuid`, `timestamptz`, `numeric` pour les montants (jamais de flottant),
  `jsonb` pour des données réellement semi-structurées, énumérations ou tables de référence.
- Index : chaque clé étrangère filtrée ou jointe ; index composite dans l'ordre des filtres
  (égalité d'abord, puis plage) ; index partiel pour un sous-ensemble fréquent (`WHERE deleted_at IS NULL`) ;
  index d'expression pour une recherche transformée (`lower(email)`). Un index a un coût en écriture :
  justifie chacun par une requête.
- Colonnes sensibles (e-mail, téléphone, IBAN, données de santé) : signale-les dans le handoff, avec
  le chiffrement ou la pseudonymisation prévus.

## Migrations sans interruption

- Chaque migration a son `down` (ou une procédure de retour documentée) et se teste dans les deux sens.
- Une migration déjà appliquée sur un environnement partagé ne se modifie plus : écris-en une nouvelle.
- Évolutions en plusieurs déploiements (expand / contract) :
  - nouvelle colonne obligatoire : ajout nullable → backfill par lots → `NOT NULL` ;
  - renommage : nouvelle colonne, double écriture, bascule des lectures, suppression de l'ancienne
    (cette dernière étape est destructive : protocole de `dev-standards-migration`) ;
  - index sur une grande table : création concurrente (`CREATE INDEX CONCURRENTLY` sous PostgreSQL),
    hors transaction.
- Schéma et backfill de données dans des migrations séparées ; backfill par lots, rejouable.
- Commandes de migration du projet (`alembic`, `python manage.py migrate`, `rails db:migrate`,
  `flask db`, `npx prisma migrate`…) : exécution sur la base locale ou de test uniquement.

## Requêtes

- Mesure avant d'optimiser : `EXPLAIN ANALYZE` sur des volumes réalistes. Tu n'as pas de client de
  base : fournis la requête exacte à l'utilisateur, qui te colle le plan, ou propose-lui
  `docker compose exec <service> psql …` si le projet a un service de base. <!-- capability: user -->
- Signaux dans le plan : parcours séquentiel sur une grande table, boucle imbriquée sur de gros
  ensembles, tri ou hachage qui déborde sur disque, estimation de lignes très éloignée du réel
  (statistiques à rafraîchir).
- Repères : requête au-delà de 100 ms en p95, ou plus de 1 000 lignes renvoyées sans pagination →
  à traiter. Pagination par curseur (keyset) sur les grandes tables plutôt que `OFFSET`.
- Compare le plan avant/après dans le handoff.

## ORM

- N+1 : une requête par élément d'une liste. Détecte-le dans les logs de requêtes ou un test qui compte
  les requêtes ; corrige par chargement anticipé (`include`, `joinedload`/`selectinload`, `includes`,
  `Preload`) ou par lots.
- Sélectionne les colonnes utiles sur les gros objets ; pas de chargement d'une table entière en mémoire
  pour filtrer côté code.
- Transactions explicites autour des écritures liées ; pas d'appel réseau à l'intérieur d'une transaction.
- Requêtes brutes toujours paramétrées : jamais de concaténation de valeurs dans le SQL.

## Sécurité

- L'application se connecte avec un rôle à privilèges minimaux (pas de superutilisateur, pas de DDL
  hors migrations).
- Aucun identifiant de connexion dans le code ou les migrations : variables d'environnement ou
  gestionnaire de secrets.
- Les logs de requêtes ne contiennent pas les valeurs sensibles.
