---
name: dev-standards-pyspark
description: Standards PySpark — DataFrames API, partitionnement, éviter les UDFs, ML lifecycle avec pipelines sklearn/MLflow, tests locaux.
---

# Skill — Standards PySpark / Spark

> Versions de référence (octobre 2026) : Apache Spark / PySpark 4.2 (Python ≥ 3.10).

## Rôle

Ce skill définit les bonnes pratiques pour le développement de jobs distribués avec
Apache Spark (PySpark) et le cycle de vie des modèles machine learning.
Il complète `dev-standards-python.md`.

---

## 🔒 Règles absolues

❌ Ne jamais entraîner ou déployer des modèles en production sans validation humaine
❌ Ne jamais committer des datasets dans git — utiliser DVC, S3 ou un data lake
❌ Ne jamais utiliser de données personnelles réelles dans les environnements de test
✅ Fixer les seeds aléatoires pour la reproductibilité
✅ Séparer strictement train / validation / test — pas de data leakage

---

## PySpark — DataFrame API

- Utiliser l'API **DataFrame** — pas RDD sauf cas extrême justifié et documenté
- Éviter les UDFs Python si une fonction Spark native existe — les UDFs désactivent les optimisations du Catalyst
- Partitionner les données selon les colonnes de filtrage les plus fréquentes
- Éviter les shuffles inutiles (`groupBy`, `join`) — partitionner en amont
- Tester la logique avec des DataFrames locaux (pas besoin de cluster pour les tests unitaires)
- Utiliser `spark.sql()` pour les transformations complexes documentées
- Mode ANSI activé par défaut depuis Spark 4 : un cast invalide, un débordement ou une division par zéro lève une erreur au lieu de renvoyer `NULL` — utiliser `try_cast` / les fonctions `try_*` quand une conversion tolérante est voulue, ne pas désactiver `spark.sql.ansi.enabled` globalement

```python
from pyspark.sql import SparkSession
from pyspark.sql import functions as F

def calculer_prix_ttc(df):
    """Calcule le prix TTC à partir du prix HT et du taux de TVA."""
    return df.withColumn(
        "prix_ttc",
        F.col("prix_ht") * (1 + F.col("taux_tva"))
    )

# ✅ Préférer les fonctions Spark natives aux UDFs Python
# ❌ UDF Python — lent, contourne le Catalyst optimizer
@F.udf("double")
def calculer_ttc_udf(prix_ht, taux_tva):
    return prix_ht * (1 + taux_tva)
```

---

## Partitionnement et performance

- Partitionner les données en écriture sur les colonnes de filtrage les plus fréquentes
- Utiliser `repartition()` avant les jointures lourdes si les datasets sont déséquilibrés
- Éviter les `collect()` sur de grands DataFrames — ils rapatrient tout en mémoire driver
- Broadcaster les petits DataFrames dans les jointures (`F.broadcast(small_df)`)
- Cacher (`cache()` / `persist()`) uniquement les DataFrames réutilisés plusieurs fois

```python
# ✅ Broadcast join pour un petit référentiel
result = large_df.join(
    F.broadcast(small_ref_df),
    on="client_id",
    how="left"
)

# ✅ Partitionnement en écriture
df.write.partitionBy("annee", "mois").parquet("s3://bucket/commandes/")
```

---

## Machine Learning — Cycle de vie

### Expérimentation

- Tracker toutes les expériences avec **MLflow**, Weights & Biases ou équivalent
- Versionner les datasets utilisés pour l'entraînement
- Fixer les seeds aléatoires pour la reproductibilité (`random_state`, seeds framework)
- Séparer strictement train / validation / test — aucune fuite de données entre les splits

### Pipeline ML

- Encapsuler le pipeline complet (prétraitement + modèle) dans un objet Pipeline
- Séparer le prétraitement (transformations reproductibles) de l'entraînement
- Pas de magic numbers dans les hyperparamètres — utiliser des configs (yaml, dataclass)
- Documenter les choix d'algorithme et les métriques de sélection

```python
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import StandardScaler
from sklearn.linear_model import LogisticRegression
import mlflow

# ✅ Pipeline encapsulé, seed fixe, tracking MLflow
pipeline = Pipeline([
    ("scaler", StandardScaler()),
    ("classifier", LogisticRegression(random_state=42, max_iter=1000)),
])

with mlflow.start_run():
    mlflow.log_params({"random_state": 42, "max_iter": 1000})
    pipeline.fit(X_train, y_train)
    score = pipeline.score(X_test, y_test)
    mlflow.log_metric("accuracy", score)
    mlflow.sklearn.log_model(pipeline, "model")
```

### Mise en production

- Versionner les modèles enregistrés (MLflow Model Registry ou équivalent)
- Monitorer la dérive des données en production (data drift, concept drift)
- Définir des seuils d'alerte et un processus de ré-entraînement documenté
- Documenter les limites connues du modèle (biais, périmètre de validité, performances par sous-groupe)

---

## Tests

### Tests PySpark

- Utiliser `pyspark.testing.assertDataFrameEqual` (Spark 3.5+) ou `chispa` pour comparer les DataFrames
- Initialiser une `SparkSession` locale dans les fixtures pytest (`local[1]`) — pas de cluster requis
- Tester avec des DataFrames de petite taille (< 100 lignes) — les tests Spark sont lents
- Tester les cas limites : DataFrame vide, valeurs nulles, types inattendus

```python
import pytest
from pyspark.sql import SparkSession
from pyspark.testing import assertDataFrameEqual

@pytest.fixture(scope="session")
def spark():
    return (
        SparkSession.builder
        .master("local[1]")
        .appName("tests")
        .getOrCreate()
    )

def test_calculer_prix_ttc(spark):
    # Arrange
    input_df = spark.createDataFrame([{"prix_ht": 100.0, "taux_tva": 0.2}])
    expected_df = spark.createDataFrame([{"prix_ht": 100.0, "taux_tva": 0.2, "prix_ttc": 120.0}])

    # Act
    result_df = calculer_prix_ttc(input_df)

    # Assert
    assertDataFrameEqual(result_df, expected_df)

def test_calculer_prix_ttc_dataframe_vide(spark):
    input_df = spark.createDataFrame([], schema="prix_ht double, taux_tva double")
    result_df = calculer_prix_ttc(input_df)
    assert result_df.count() == 0
```

### Tests ML

- Tester la forme des outputs du modèle (shape, dtype, plage de valeurs)
- Tester la reproductibilité : mêmes inputs + même seed → mêmes outputs
- Tester la robustesse aux inputs limites : features nulles, valeurs hors distribution
- Ne pas tester les métriques ML (accuracy, F1) en test unitaire — elles varient selon les données

```python
def test_modele_shape_output(trained_pipeline, sample_features):
    predictions = trained_pipeline.predict(sample_features)
    assert predictions.shape == (len(sample_features),)

def test_modele_reproductibilite(trained_pipeline, sample_features):
    pred1 = trained_pipeline.predict(sample_features)
    pred2 = trained_pipeline.predict(sample_features)
    np.testing.assert_array_equal(pred1, pred2)

def test_modele_valeurs_nulles(trained_pipeline):
    """Le modèle doit lever une erreur explicite sur des inputs nuls."""
    features_with_nulls = pd.DataFrame({"feature_a": [None], "feature_b": [1.0]})
    with pytest.raises(ValueError):
        trained_pipeline.predict(features_with_nulls)
```

---

## Ce que tu ne fais PAS

- Utiliser des RDDs sans justification documentée
- Écrire des UDFs Python quand une fonction Spark native existe
- Appeler `collect()` sur de grands DataFrames
- Entraîner ou déployer des modèles sans validation humaine
- Committer des datasets dans git
- Utiliser des données personnelles réelles dans les tests
