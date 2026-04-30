---
name: dev-standards-python
description: Standards Python — style, typage, environnements, gestion des erreurs, organisation du code, bonnes pratiques générales indépendantes du domaine métier.
---

# Skill — Standards Python

## Rôle

Ce skill définit les conventions Python à respecter sur tous les projets utilisant
ce langage, quel que soit le domaine (web, data, ML, scripting, CLI).
Il complète `dev-standards-universal.md` et s'applique dès que Python est détecté
dans la stack du projet.

Les standards spécifiques à des frameworks ou outils (Django, FastAPI, pandas, dbt,
Airflow, PySpark) sont définis dans les skills dédiés correspondants.

---

## Version et environnement

- Python ≥ 3.10 — utiliser les features modernes (match/case, type hints natifs, `X | Y`)
- Environnement isolé obligatoire : `venv`, `poetry` ou `uv` — jamais d'installation globale
- Le fichier de dépendances (`pyproject.toml`, `requirements.txt`) est versionné avec le projet
- Les versions des dépendances sont épinglées dans le lockfile (`poetry.lock`, `uv.lock`)
- Un fichier `.python-version` ou `.tool-versions` fixe la version du runtime

---

## Style et formatage

- **Formatage** : `ruff format` (remplace `black`) — configuration dans `pyproject.toml`
- **Lint** : `ruff check` (remplace `flake8` + `isort` + `pyupgrade`)
- Indentation : 4 espaces — jamais de tabulations
- Longueur de ligne : 88–120 caractères selon la configuration projet
- Les imports sont organisés : stdlib → tiers → internes, séparés par une ligne vide

```python
# ✅ Imports organisés
import os
import sys
from pathlib import Path

import httpx
from pydantic import BaseModel

from myapp.core.domain import User
from myapp.utils.formatting import format_currency
```

---

## Typage

- Toutes les fonctions publiques ont des annotations de type en entrée et en sortie
- `mypy` ou `pyright` activé en mode strict sur les modules critiques
- Pas de `# type: ignore` sans commentaire explicatif
- Utiliser `TypeVar`, `Generic`, `Protocol` pour les abstractions réutilisables
- Les types complexes sont aliasés pour la lisibilité

```python
# ✅ Typage complet
def compute_discount(price: float, rate: float) -> float:
    if not 0 <= rate <= 1:
        raise ValueError(f"Le taux doit être entre 0 et 1, reçu : {rate}")
    return price * (1 - rate)

# ❌ Pas de typage
def compute_discount(price, rate):
    return price * (1 - rate)
```

### Types modernes (Python ≥ 3.10)

```python
# ✅ Union moderne
def find_user(user_id: str) -> User | None: ...

# ✅ Match/case pour les discriminated unions
match event.type:
    case "created":
        handle_created(event)
    case "deleted":
        handle_deleted(event)
    case _:
        logger.warning("Type d'événement inconnu : %s", event.type)
```

---

## Nommage

| Élément | Convention | Exemple |
|---|---|---|
| Variables, fonctions | `snake_case` | `user_id`, `compute_total` |
| Classes | `PascalCase` | `UserRepository`, `PaymentService` |
| Constantes | `SCREAMING_SNAKE_CASE` | `MAX_RETRIES`, `DEFAULT_TIMEOUT` |
| Modules / fichiers | `snake_case` | `payment_service.py` |
| Paramètres privés | `_prefixe` | `_cache`, `_session` |
| Dunder methods | `__nom__` | `__init__`, `__repr__` |

---

## Gestion des erreurs

- Les exceptions sont des classes qui héritent de `Exception` (ou d'une classe métier de base)
- Chaque module/domaine définit ses propres exceptions — pas de levée de `Exception` brute
- Le bloc `except` est toujours spécifique — pas de `except Exception:` sans re-raise ou logging
- Les erreurs métier sont distinguées des erreurs techniques

```python
# ✅ Hiérarchie d'exceptions métier
class AppError(Exception):
    """Base pour toutes les erreurs métier de l'application."""

class NotFoundError(AppError):
    def __init__(self, resource: str, resource_id: str) -> None:
        super().__init__(f"{resource} introuvable : {resource_id}")
        self.resource = resource
        self.resource_id = resource_id

class ValidationError(AppError):
    def __init__(self, field: str, message: str) -> None:
        super().__init__(f"Validation échouée sur '{field}' : {message}")
        self.field = field

# ✅ Catch spécifique
try:
    user = user_repo.find_by_id(user_id)
except NotFoundError:
    return Response(status=404, body={"error": "Utilisateur introuvable"})
except ValidationError as e:
    logger.warning("Validation échouée", extra={"field": e.field})
    return Response(status=422, body={"error": str(e)})
```

---

## Logging

- Pas de `print()` dans le code de production — utiliser le module `logging`
- Le logger est créé au niveau du module : `logger = logging.getLogger(__name__)`
- Utiliser les niveaux sémantiques : `DEBUG`, `INFO`, `WARNING`, `ERROR`, `CRITICAL`
- Les messages de log utilisent le formatage `%s` (lazy) plutôt que les f-strings
- Ne jamais logger de données sensibles (mots de passe, tokens, PII)

```python
import logging

logger = logging.getLogger(__name__)

# ✅ Lazy formatting
logger.info("Traitement de la commande %s pour l'utilisateur %s", order_id, user_id)

# ❌ F-string (évalué même si le niveau est désactivé)
logger.debug(f"Payload reçu : {payload}")
```

---

## Organisation du code

- Une classe par fichier pour les classes de domaine importantes
- Les fichiers utilitaires groupent des fonctions de même nature
- Les modules `__init__.py` n'exposent que l'API publique du package — pas d'implémentation
- Les imports circulaires sont un signal d'architecture à corriger, pas à contourner
- Les dataclasses ou `pydantic.BaseModel` sont préférés aux dicts bruts pour les structures de données

```python
# ✅ Dataclass pour les value objects
from dataclasses import dataclass

@dataclass(frozen=True)
class Money:
    amount: float
    currency: str

    def __post_init__(self) -> None:
        if self.amount < 0:
            raise ValueError("Le montant ne peut pas être négatif")
```

---

## Tests

- Framework de test : `pytest` (standard de facto)
- Les tests sont co-localisés avec le code source ou dans un dossier `tests/` miroir
- Chaque test est indépendant — pas d'état partagé entre tests sans fixtures explicites
- Les fixtures `pytest` remplacent les `setUp`/`tearDown` de `unittest`
- Les mocks utilisent `unittest.mock` ou `pytest-mock` (`mocker` fixture)

```python
# ✅ Test avec fixture et mock
def test_send_welcome_email_on_user_creation(mocker):
    # Arrange
    mock_mailer = mocker.patch("myapp.services.mailer.send_email")
    service = UserService(mailer=mock_mailer)

    # Act
    service.create_user(email="alice@example.com", name="Alice")

    # Assert
    mock_mailer.assert_called_once_with(
        to="alice@example.com",
        subject="Bienvenue !",
    )
```

---

## Ce que tu ne fais PAS

- Installer des dépendances globalement — toujours dans un environnement isolé
- Utiliser `print()` en production
- Lever `Exception` brute — créer des classes d'exception spécifiques
- Écrire des fonctions sans annotations de type sur les projets typés
- Ignorer les warnings `mypy`/`pyright` avec `# type: ignore` sans explication
- Utiliser des imports relatifs implicites (`from module import *`)
