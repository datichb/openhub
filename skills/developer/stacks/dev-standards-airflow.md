---
name: dev-standards-airflow
description: "Utiliser quand le projet utilise Apache Airflow (DAGs, TaskFlow, idempotence, secrets, tests) et que le ticket touche ce code."
versions:
  airflow: "3.3"
reviewed: 2026-10-12
---

# Skill — Standards Apache Airflow

> Versions de référence (octobre 2026) : Apache Airflow 3.3 (Task SDK `airflow.sdk`).

## Rôle

Ce skill définit les bonnes pratiques pour l'orchestration de pipelines avec Apache Airflow.
Il complète `dev-standards-python.md`.

---

## 🔒 Règles absolues

❌ Jamais de logique métier dans la définition du DAG — uniquement l'orchestration
❌ Jamais de secrets en dur dans le code — utiliser Airflow Variables ou Connections
❌ Jamais de `catchup=True` sans analyse préalable des risques de backfill
✅ Chaque tâche est atomique et idempotente
✅ `start_date`, `schedule` et `catchup` sont toujours définis explicitement

---

## Structure d'un DAG

- Un DAG = un pipeline métier distinct — pas de DAG "fourre-tout"
- Toujours définir `start_date`, `schedule` et `catchup=False` explicitement
- Les tâches sont atomiques et idempotentes
- Utiliser **TaskFlow API** (décorateurs `@task`) plutôt que les opérateurs bas niveau quand possible
- Les secrets sont injectés via Airflow Variables ou Connections — jamais en dur

```python
from airflow.sdk import dag, task  # Airflow 3 — remplace airflow.decorators
from datetime import datetime, timedelta
import logging

logger = logging.getLogger(__name__)

@dag(
    schedule="@daily",
    start_date=datetime(2024, 1, 1),
    catchup=False,
    tags=["commandes", "etl"],
    default_args={"retries": 2, "retry_delay": timedelta(minutes=5)},
)
def pipeline_commandes():
    """Pipeline ETL des commandes : extraction, transformation, chargement."""

    @task()
    def extraire() -> list[dict]:
        """Extrait les commandes de la source."""
        logger.info("Extraction des commandes...")
        # extraction depuis la source
        commandes = ...
        logger.info("Commandes extraites : %d", len(commandes))
        return commandes

    @task()
    def transformer(commandes: list[dict]) -> list[dict]:
        """Applique les règles métier de transformation."""
        logger.info("Transformation de %d commandes", len(commandes))
        result = ...
        logger.info("Commandes transformées : %d", len(result))
        return result

    @task()
    def charger(commandes: list[dict]) -> None:
        """Charge les commandes transformées vers la destination."""
        logger.info("Chargement de %d commandes", len(commandes))
        ...

    charger(transformer(extraire()))

pipeline_commandes()
```

---

## Idempotence

Chaque tâche doit pouvoir être re-exécutée sans produire de doublons ou d'état incohérent :

- Utiliser `INSERT ... ON CONFLICT DO UPDATE` (upsert) plutôt que `INSERT`
- Nettoyer les données partielles avant de re-charger (delete + insert ou truncate + insert)
- Documenter la stratégie d'idempotence dans la docstring de chaque tâche

---

## Gestion des secrets

- Les credentials (DB, API keys, tokens) sont dans les **Airflow Connections**
- Les paramètres de configuration (URLs, buckets, noms de tables) sont dans les **Airflow Variables**
- Ne jamais stocker de secrets dans les variables d'environnement du DAG
- Utiliser `BaseHook.get_connection(conn_id)` (importé depuis `airflow.sdk`) pour accéder aux connexions

```python
from airflow.sdk import BaseHook

@task()
def extraire_depuis_api() -> list[dict]:
    conn = BaseHook.get_connection("mon_api_externe")
    # conn.host, conn.login, conn.password, conn.extra_dejson
    ...
```

---

## Gestion des erreurs et retries

- Définir `retries` et `retry_delay` dans `default_args` au niveau du DAG
- Utiliser `on_failure_callback` pour les alertes (Slack, email, PagerDuty)
- Les erreurs transitoires (réseau, timeout) → retries automatiques
- Les erreurs métier (données invalides) → lever une exception explicite sans retry

```python
from airflow.sdk import get_current_context

@task(retries=3, retry_delay=timedelta(minutes=2))
def appeler_api_externe() -> list[dict]:
    context = get_current_context()
    logger.info("Exécution du %s", context["ds"])
    ...
```

---

## Organisation du projet Airflow

```
dags/
├── pipeline_commandes.py       ← définition du DAG
├── pipeline_clients.py
plugins/
├── hooks/                      ← hooks custom
└── operators/                  ← operators custom
include/
├── sql/                        ← requêtes SQL externalisées
└── schemas/                    ← schémas de validation
tests/
├── dags/                       ← tests unitaires des DAGs et tasks
└── integration/                ← tests d'intégration (optionnel)
```

---

## Tests

### Test de structure du DAG

```python
from airflow.dag_processing.dagbag import DagBag  # chemin documenté en Airflow 3

def test_dag_chargement_sans_erreur():
    """Le DAG doit se charger sans erreur d'import."""
    dag_bag = DagBag(dag_folder="dags/", include_examples=False)
    assert len(dag_bag.import_errors) == 0, f"Erreurs d'import : {dag_bag.import_errors}"

def test_dag_structure():
    """Le DAG doit avoir les bonnes tâches et la bonne config."""
    dag_bag = DagBag(dag_folder="dags/", include_examples=False)
    assert "pipeline_commandes" in dag_bag.dags
    dag = dag_bag.dags["pipeline_commandes"]
    assert dag.catchup is False
    assert set(dag.task_ids) == {"extraire", "transformer", "charger"}
```

### Test des tâches isolément

```python
def test_task_transformer_logique_metier():
    """La logique de transformation est testée sans Airflow."""
    # Les @task sont des fonctions Python ordinaires — testables directement
    input_data = [{"montant_ht": 100, "taux_tva": 0.2}]
    result = transformer.function(input_data)  # .function accède à la fonction sous-jacente
    assert result[0]["montant_ttc"] == pytest.approx(120.0)

def test_task_transformer_donnees_vides():
    result = transformer.function([])
    assert result == []
```

---

## Ce que tu ne fais PAS

- Mettre de la logique métier dans la définition du DAG (au niveau global du fichier)
- Stocker des secrets dans le code ou les variables d'environnement du DAG
- Créer des tâches non idempotentes sans documenter la stratégie de re-run
- Utiliser `catchup=True` sans avoir analysé les risques de backfill
- Partager de l'état entre tâches autrement que via XCom ou un stockage externe
