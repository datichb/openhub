---
name: dev-standards-pandas
description: "Utiliser quand le projet utilise pandas (vectorisation, schéma, transformations, tests) et que le ticket touche ce code."
---

# Skill — Standards pandas

> Versions de référence (octobre 2026) : pandas 3.0 (Python ≥ 3.11), pandera 0.34.

## Rôle

Ce skill définit les bonnes pratiques pour le traitement de données avec pandas.
Il complète `dev-standards-python.md` et s'applique à tout projet utilisant pandas
pour de la transformation, de l'analyse ou du preprocessing ML.

---

## 🔒 Règles absolues

❌ Ne jamais modifier les données sources brutes — toujours travailler sur des copies
❌ Ne jamais utiliser de données personnelles réelles dans les environnements de test
✅ Valider le schéma en entrée avant toute transformation
✅ Logger le volume (lignes/colonnes) en entrée et en sortie de chaque transformation

---

## Bonnes pratiques de transformation

- Toujours vérifier le schéma à l'entrée (dtypes, colonnes attendues) avant de transformer
- Préférer les opérations vectorisées aux boucles `for` sur les DataFrames
- Éviter `iterrows()` et `itertuples()` sur les grands datasets — utiliser `apply()` avec parcimonie ou la vectorisation native
- Copy-on-Write (défaut depuis pandas 3) : chaque sous-ensemble ou résultat de méthode se comporte comme une copie — plus de `SettingWithCopyWarning`, plus de `.copy()` défensifs ; modifier un objet uniquement directement (`df.loc[mask, "col"] = …`), jamais par affectation chaînée (`df[mask]["col"] = …` n'a aucun effet)
- Les colonnes texte sont inférées avec le dtype dédié `str` (pandas 3) — ne plus tester `dtype == object` pour détecter du texte
- Nommer les étapes de transformation de façon explicite — pas de chaînes de `.pipe()` sans commentaire

```python
# ✅ Vectorisé — rapide et lisible
df["prix_ttc"] = df["prix_ht"] * (1 + df["taux_tva"])

# ❌ Boucle inutile — lent et illisible
for i, row in df.iterrows():
    df.at[i, "prix_ttc"] = row["prix_ht"] * (1 + row["taux_tva"])
```

### Pipeline de transformation

- Structurer les transformations en fonctions pures : `input DataFrame → output DataFrame`
- Chaque fonction a une seule responsabilité (renommage, filtrage, calcul, jointure)
- Utiliser `.pipe()` pour enchaîner les étapes lisiblement

```python
def renommer_colonnes(df: pd.DataFrame) -> pd.DataFrame:
    return df.rename(columns={"montant": "montant_ht", "taux": "taux_tva"})

def calculer_ttc(df: pd.DataFrame) -> pd.DataFrame:
    return df.assign(prix_ttc=df["montant_ht"] * (1 + df["taux_tva"]))

def filtrer_valides(df: pd.DataFrame) -> pd.DataFrame:
    return df[df["statut"] == "valide"]

# ✅ Pipeline lisible
result = (
    df
    .pipe(renommer_colonnes)
    .pipe(filtrer_valides)
    .pipe(calculer_ttc)
)
```

---

## Validation des schémas

- Valider les DataFrames en entrée avec **pandera** ou **pydantic** v2
- Définir un schéma explicite pour chaque source de données
- Signaler (log + raise) les données inattendues — ne jamais les ignorer silencieusement

```python
import pandera as pa

schema = pa.DataFrameSchema(
    {
        "prix_ht": pa.Column(float, pa.Check.greater_than(0)),
        "taux_tva": pa.Column(float, pa.Check.in_range(0, 1)),
        "statut": pa.Column(str, pa.Check.isin(["valide", "annule", "en_attente"])),
    },
    checks=pa.Check(lambda df: df["prix_ht"].notna().all(), error="prix_ht ne doit pas contenir de NaN"),
)

@pa.check_input(schema)
def calculer_ttc(df: pd.DataFrame) -> pd.DataFrame:
    return df.assign(prix_ttc=df["prix_ht"] * (1 + df["taux_tva"]))
```

---

## SQL avec pandas

- Toujours utiliser des requêtes paramétrées (jamais de concaténation de chaînes)
- Nommer les CTEs de façon explicite et documentée
- Éviter `SELECT *` en production — lister les colonnes nécessaires
- Documenter les requêtes complexes (logique métier non évidente)

```sql
-- ✅ CTE nommée et documentée
WITH commandes_du_mois AS (
    -- Commandes validées sur le mois en cours
    SELECT id, client_id, montant_ttc
    FROM commandes
    WHERE statut = 'validee'
      AND date_commande >= DATE_TRUNC('month', CURRENT_DATE)
)
SELECT client_id, SUM(montant_ttc) AS chiffre_affaires
FROM commandes_du_mois
GROUP BY client_id;
```

---

## Tests

- Tester les transformations avec des fixtures de données connues (entrée → sortie attendue)
- Tester les cas limites : DataFrame vide, valeurs nulles, types inattendus, doublons
- Ne pas tester contre une vraie base de données — utiliser SQLite in-memory, DuckDB ou des fichiers parquet de test
- Les fixtures de données sont inline dans les tests — pas de fichiers CSV séparés

```python
import pandas as pd
import pytest

def test_calcul_prix_ttc():
    # Arrange
    df = pd.DataFrame({"prix_ht": [100.0, 50.0], "taux_tva": [0.2, 0.1]})

    # Act
    result = calculer_ttc(df)

    # Assert
    assert result["prix_ttc"].tolist() == [120.0, 55.0]

def test_calcul_prix_ttc_valeurs_nulles():
    df = pd.DataFrame({"prix_ht": [100.0, None], "taux_tva": [0.2, 0.1]})
    with pytest.raises(pa.errors.SchemaError):
        calculer_ttc(df)

def test_calcul_prix_ttc_dataframe_vide():
    df = pd.DataFrame({"prix_ht": [], "taux_tva": []})
    result = calculer_ttc(df)
    assert result.empty
```

---

## Ce que tu ne fais PAS

- Modifier les données sources brutes — travailler sur des copies
- Utiliser `iterrows()` sur des DataFrames volumineux
- Utiliser l'affectation chaînée (`df[mask]["col"] = …`) — sans effet sous Copy-on-Write
- Laisser passer des données invalides sans validation explicite
- Utiliser des données personnelles réelles dans les tests
