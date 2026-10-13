---
name: audit-database
description: "Utiliser quand l'audit porte sur une base de données (lecture seule) : rôles, RLS, injections, chiffrement, sauvegardes, migrations, données sensibles."
---

# Skill — Audit Base de données

## Référentiels couverts

- **CIS Benchmarks** PostgreSQL et MySQL
- **OWASP Top 10** (A01 contrôle d'accès, A02 cryptographie, A03 injection) et **OWASP ASVS** V5/V8
- **ANSSI** — recommandations de sécurité pour les bases de données
- **RGPD** art. 32 (sécurité du traitement) pour les données personnelles

L'audit lit le code du dépôt : schéma, migrations, configuration, couche d'accès aux données
(Prisma, SQLAlchemy, ActiveRecord, GORM, TypeORM, Doctrine…). Il n'interroge aucune base :
quand une vérification demande la base réelle, fournir la requête à l'utilisateur
(<!-- capability: user --> `psql -c "\du"`, `SELECT * FROM pg_roles;`) et lui demander le résultat.

---

## Méthode

1. **Inventaire** : moteur et version, schéma (`schema.prisma`, `models.py`, `db/schema.rb`,
   migrations), configuration de connexion (variables d'environnement, `database.yml`,
   `docker-compose.yml`, manifestes, Terraform des bases managées).
2. **Lecture** : appliquer les checklists ci-dessous.
3. **Rapport** : format `audit-protocol-light`, un finding par motif avec la liste des occurrences.

---

## Checklist — Rôles et privilèges

- [ ] L'application se connecte avec un rôle dédié, jamais `postgres`, `root` ou un superutilisateur
- [ ] Rôles séparés : migrations (DDL) d'un côté, application (DML) de l'autre, lecture seule pour l'analytique
- [ ] Aucun `GRANT ALL` ni `GRANT … TO PUBLIC` ; `REVOKE CREATE ON SCHEMA public FROM PUBLIC` sur PostgreSQL
- [ ] Fonctions `SECURITY DEFINER` avec un `search_path` fixé
- [ ] Accès réseau restreint : `pg_hba.conf` sans `trust`, base non exposée publiquement (groupes de sécurité, `publicly_accessible = false`)

## Checklist — Row-Level Security et multi-tenant

- [ ] Tables multi-tenant protégées par RLS (`ENABLE ROW LEVEL SECURITY` et `FORCE` pour le propriétaire) ou par un filtre de tenant centralisé
- [ ] Politiques RLS couvrant `SELECT`, `INSERT`, `UPDATE`, `DELETE` (`USING` et `WITH CHECK`)
- [ ] Rôle de l'application sans `BYPASSRLS` ; contexte de tenant posé par requête (`SET LOCAL`)
- [ ] Supabase / PostgREST : RLS activée sur chaque table exposée, clé `service_role` jamais côté client

## Checklist — Surfaces d'injection

- [ ] Requêtes paramétrées partout ; aucune concaténation ou interpolation de chaîne dans le SQL
- [ ] Échappatoires des ORMs relues : `$queryRawUnsafe`, `text()` avec f-string, `raw()`, `whereRaw`, `find_by_sql`, `Exec(fmt.Sprintf…)`
- [ ] Noms dynamiques (colonnes de tri, tables) validés par liste blanche
- [ ] Opérateurs NoSQL filtrés (`$where`, `$regex` issus d'une entrée utilisateur)
- [ ] Procédures stockées sans `EXECUTE` de chaîne construite

## Checklist — Chiffrement

- [ ] En transit : TLS imposé (`sslmode=verify-full`, `require_secure_transport=ON`, `rds.force_ssl`)
- [ ] Au repos : stockage chiffré (`storage_encrypted = true`, clé KMS gérée), sauvegardes et réplicas inclus
- [ ] Colonnes très sensibles (IBAN, numéro de sécurité sociale, secrets d'intégration) chiffrées côté application ou via `pgcrypto`, clés hors de la base
- [ ] Mots de passe hachés (argon2, bcrypt), jamais chiffrés de manière réversible

## Checklist — Sauvegardes

- [ ] Sauvegardes automatiques avec rétention définie et point-in-time recovery pour la production
- [ ] Sauvegardes chiffrées, stockées hors du compte ou de la région principale
- [ ] Restauration testée et documentée (procédure, durée, dernier test)
- [ ] Protection contre la suppression (`deletion_protection`, `prevent_destroy`)

## Checklist — Sûreté des migrations

- [ ] Chaque migration a un retour arrière (`down`) ou une procédure de rollback documentée
- [ ] Changements sans interruption : colonne nullable puis remplissage puis contrainte, renommage en deux étapes, `CREATE INDEX CONCURRENTLY`
- [ ] Aucune migration déjà appliquée modifiée a posteriori
- [ ] Opérations destructives (`DROP`, `TRUNCATE`, changement de type) signalées et précédées d'une sauvegarde
- [ ] Aucune donnée de production ni identifiant dans les migrations ou les seeds

## Checklist — Exposition des données sensibles

- [ ] Colonnes sensibles identifiées (email, téléphone, données de santé, identifiants bancaires)
- [ ] Requêtes et paramètres non journalisés en clair (`log_statement`, logs de l'ORM en production)
- [ ] Sélections explicites plutôt que `SELECT *` vers les API ; sérialiseurs qui excluent les champs sensibles
- [ ] Chaînes de connexion hors du code et des fichiers versionnés (gestionnaire de secrets)
- [ ] Jeux de données de test anonymisés

---

## Ce que tu ne fais PAS dans ce domaine

- Se connecter à une base ou exécuter une requête : la requête est fournie à l'utilisateur
- Recopier une valeur sensible trouvée (mot de passe, chaîne de connexion) : citer le fichier et la ligne
- Concevoir le schéma ou réécrire les requêtes : les recommandations vont dans le plan d'action
- Certifier une conformité RGPD — tu identifies les écarts, l'humain décide
