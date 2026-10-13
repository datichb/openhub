---
name: dev-standards-dbt
description: "Utiliser quand le projet utilise dbt (staging/mart, schema.yml, tests, matérialisations) et que le ticket touche ce code."
---

# Skill — Standards dbt

> Versions de référence (octobre 2026) : dbt Core 1.12 (dbt Core 2.0 en release candidate sur PyPI).

## Rôle

Ce skill définit les bonnes pratiques pour le développement de transformations SQL
avec dbt (data build tool).
Il complète `dev-standards-python.md` pour les projets analytics et data warehouse.

---

## 🔒 Règles absolues

❌ Ne jamais modifier les données sources brutes — les sources sont en lecture seule dans dbt
❌ Ne jamais committer des données réelles dans git (seeds, fixtures)
✅ Tout modèle `mart` a au minimum `not_null` + `unique` sur sa clé primaire
✅ Tout modèle est documenté dans `schema.yml`

---

## Architecture en layers

Respecter strictement la séparation en trois couches :

```
models/
├── staging/        ← renommage, typage, nettoyage depuis les sources brutes
├── intermediate/   ← jointures, agrégations intermédiaires, logique métier
└── mart/           ← tables exposées aux utilisateurs finaux (BI, API, data science)
```

### staging

- Un modèle par table source (`stg_<source>__<table>.sql`)
- Renommage des colonnes selon les conventions du projet
- Typage explicite (casts)
- Nettoyage basique : trim des chaînes, normalisation des nulls
- Pas de jointures ni d'agrégations — transformation purement structurelle

### intermediate

- Jointures entre modèles staging
- Agrégations et calculs métier intermédiaires
- Nommage : `int_<domaine>_<description>.sql`

### mart

- Tables prêtes à l'emploi pour les utilisateurs finaux
- Nommage : `dim_<entité>.sql` ou `fct_<fait>.sql` (ou `mart_<domaine>.sql`)
- Documentation complète dans `schema.yml`

---

## Documentation — schema.yml

- Chaque modèle a une `description` dans `schema.yml`
- Chaque colonne importante a une `description`
- Les tests dbt obligatoires sur toutes les clés primaires : `not_null` + `unique`

```yaml
# ✅ schema.yml complet — clé `data_tests:` (dbt ≥ 1.8) et `arguments:` (dbt ≥ 1.10.5)
models:
  - name: mart_commandes
    description: "Commandes validées agrégées par client et par mois"
    columns:
      - name: commande_id
        description: "Identifiant unique de la commande"
        data_tests:
          - not_null
          - unique
      - name: statut
        description: "Statut de la commande"
        data_tests:
          - not_null
          - accepted_values:
              arguments:
                values: ['validee', 'annulee', 'en_attente']
      - name: client_id
        description: "Clé étrangère vers dim_clients"
        data_tests:
          - not_null
          - relationships:
              arguments:
                to: ref('dim_clients')
                field: id
      - name: chiffre_affaires
        description: "Montant TTC total des commandes du mois"
        data_tests:
          - not_null
```

- `tests:` reste accepté comme alias mais ne plus l'utiliser — `data_tests:` distingue les tests de données des unit tests (`unit_tests:`)

---

## Matérialisations

- Matérialisation par défaut : `view` — ne changer qu'avec justification
- `table` : quand la vue est trop lente à requêter (agrégations lourdes, jointures multiples)
- `incremental` : quand le volume est trop grand pour recalculer à chaque run
- `ephemeral` : pour les sous-requêtes réutilisées sans besoin de persistance

```sql
-- ✅ Modèle incremental bien configuré
{{ config(
    materialized='incremental',
    unique_key='commande_id',
    on_schema_change='fail'
) }}

SELECT
    commande_id,
    client_id,
    montant_ttc,
    date_commande
FROM {{ ref('stg_commandes') }}
{% if is_incremental() %}
    WHERE date_commande > (SELECT MAX(date_commande) FROM {{ this }})
{% endif %}
```

---

## Modèles SQL

- Un modèle = une responsabilité (pas de modèle "fourre-tout")
- Utiliser `ref()` pour toutes les références inter-modèles — jamais de nom de table en dur
- Utiliser `source()` pour référencer les tables brutes
- CTEs nommées explicitement pour les étapes intermédiaires
- Pas de `SELECT *` — lister les colonnes explicitement

```sql
-- ✅ Modèle staging bien structuré
WITH source AS (
    SELECT * FROM {{ source('crm', 'raw_clients') }}
),

renamed AS (
    SELECT
        client_id                           AS id,
        TRIM(nom)                           AS nom,
        LOWER(email)                        AS email,
        CAST(date_inscription AS DATE)      AS date_inscription,
        statut                              AS statut
    FROM source
    WHERE statut IS NOT NULL
)

SELECT * FROM renamed
```

---

## Tests

### Tests natifs dbt (clé `data_tests:` dans schema.yml)

- `not_null` — obligatoire sur toutes les clés primaires et colonnes critiques
- `unique` — obligatoire sur toutes les clés primaires
- `accepted_values` — pour les colonnes à valeurs contrôlées (statuts, types)
- `relationships` — pour les clés étrangères

### Tests personnalisés (dans tests/)

Pour les règles métier non couvertes par les tests natifs :

```sql
-- tests/valider_montant_positif.sql
-- Test personnalisé : aucune commande ne doit avoir un montant négatif
-- Un test dbt réussit si la requête retourne 0 lignes
SELECT commande_id
FROM {{ ref('mart_commandes') }}
WHERE montant_ttc < 0
```

### Commandes de test

```bash
# Tester un modèle spécifique
dbt test --select mart_commandes

# Tester tous les modèles d'un layer
dbt test --select staging.*

# Tester avec les sources
dbt test --select source:crm
```

---

## Ce que tu ne fais PAS

- Référencer des tables en dur dans les modèles — toujours `ref()` ou `source()`
- Créer des modèles sans documentation dans `schema.yml`
- Utiliser `SELECT *` dans les modèles de mart
- Mettre de la logique métier dans les modèles staging
- Matérialiser en `table` sans mesurer l'impact sur les coûts de compute
