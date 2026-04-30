---
name: dev-standards-fastapi
description: Standards FastAPI — Pydantic, async/await, dépendances, routeurs, gestion des erreurs, tests et bonnes pratiques.
---

# Skill — Standards FastAPI

## Rôle

Ce skill définit les bonnes pratiques pour le développement backend avec FastAPI.
Il complète `dev-standards-python.md`, `dev-standards-backend.md` et
`dev-standards-api.md`.

---

## 🔒 Règles absolues

❌ Jamais de logique métier dans les endpoint functions — déléguer aux services
❌ Jamais de secrets en dur dans le code — utiliser `pydantic-settings`
❌ Jamais d'opérations bloquantes dans des fonctions `async` — utiliser `run_in_executor`
✅ Tous les modèles d'entrée sont des classes Pydantic

---

## Structure du projet

```
app/
├── main.py                 ← création de l'app FastAPI
├── config.py               ← Settings avec pydantic-settings
├── dependencies.py         ← dépendances globales (db session, current user)
├── routers/
│   └── users/
│       ├── router.py       ← routes du domaine
│       ├── schemas.py      ← modèles Pydantic (request/response)
│       ├── service.py      ← logique métier
│       └── repository.py   ← accès aux données
├── models/                 ← modèles SQLAlchemy/SQLModel
├── exceptions/             ← exceptions métier custom
└── tests/
```

---

## Configuration avec pydantic-settings

```python
# config.py
from pydantic_settings import BaseSettings, SettingsConfigDict

class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file='.env', env_file_encoding='utf-8')

    database_url: str
    jwt_secret: str
    jwt_expiry_minutes: int = 60
    cors_origins: list[str] = []

    @property
    def async_database_url(self) -> str:
        return self.database_url.replace('postgresql://', 'postgresql+asyncpg://')

settings = Settings()
```

---

## Modèles Pydantic

- Schémas d'entrée et de sortie distincts
- Utiliser `model_config` pour la configuration (pas `class Config` dépréciée)
- Les passwords et données sensibles sont exclus des schémas de réponse

```python
# schemas.py
from pydantic import BaseModel, EmailStr, Field, model_validator
import uuid
from datetime import datetime

class UserCreate(BaseModel):
    email: EmailStr
    name: str = Field(min_length=2, max_length=100)
    password: str = Field(min_length=8)

class UserUpdate(BaseModel):
    name: str | None = Field(default=None, min_length=2, max_length=100)

class UserResponse(BaseModel):
    model_config = {'from_attributes': True}

    id: uuid.UUID
    email: EmailStr
    name: str
    created_at: datetime
    # password jamais exposé
```

---

## Endpoints

```python
# router.py
from fastapi import APIRouter, Depends, HTTPException, status
from .schemas import UserCreate, UserResponse
from .service import UserService
from ..dependencies import get_user_service, get_current_user

router = APIRouter(prefix='/users', tags=['users'])

@router.get('/{user_id}', response_model=UserResponse)
async def get_user(
    user_id: uuid.UUID,
    service: UserService = Depends(get_user_service),
    _: User = Depends(get_current_user),  # auth obligatoire
) -> UserResponse:
    return await service.find_or_raise(user_id)

@router.post('/', response_model=UserResponse, status_code=status.HTTP_201_CREATED)
async def create_user(
    data: UserCreate,
    service: UserService = Depends(get_user_service),
) -> UserResponse:
    return await service.create(data)
```

---

## Injection de dépendances

```python
# dependencies.py
from fastapi import Depends
from sqlalchemy.ext.asyncio import AsyncSession
from .database import get_session
from .routers.users.service import UserService

async def get_user_service(
    session: AsyncSession = Depends(get_session),
) -> UserService:
    return UserService(session)

async def get_current_user(
    token: str = Depends(oauth2_scheme),
    session: AsyncSession = Depends(get_session),
) -> User:
    payload = decode_jwt(token)
    user = await session.get(User, payload['sub'])
    if not user:
        raise HTTPException(status_code=401, detail='Utilisateur introuvable')
    return user
```

---

## Services

```python
# service.py
from fastapi import HTTPException, status

class UserService:
    def __init__(self, session: AsyncSession) -> None:
        self.session = session

    async def find_or_raise(self, user_id: uuid.UUID) -> UserResponse:
        user = await self.session.get(User, user_id)
        if not user:
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail=f'Utilisateur {user_id} introuvable',
            )
        return UserResponse.model_validate(user)

    async def create(self, data: UserCreate) -> UserResponse:
        existing = await self.session.scalar(
            select(User).where(User.email == data.email)
        )
        if existing:
            raise HTTPException(
                status_code=status.HTTP_409_CONFLICT,
                detail='Cet email est déjà utilisé',
            )
        user = User(
            email=data.email,
            name=data.name,
            password=hash_password(data.password),
        )
        self.session.add(user)
        await self.session.commit()
        await self.session.refresh(user)
        return UserResponse.model_validate(user)
```

---

## Gestion des erreurs globales

```python
# main.py
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

app = FastAPI()

@app.exception_handler(ValueError)
async def value_error_handler(request: Request, exc: ValueError) -> JSONResponse:
    return JSONResponse(
        status_code=422,
        content={'error': {'code': 'VALIDATION_ERROR', 'message': str(exc)}},
    )
```

---

## Tests

```python
# tests/test_users.py
import pytest
from httpx import AsyncClient, ASGITransport
from app.main import app

@pytest.mark.asyncio
async def test_create_user_returns_201():
    async with AsyncClient(transport=ASGITransport(app=app), base_url='http://test') as client:
        response = await client.post('/api/users/', json={
            'email': 'alice@exemple.com',
            'name': 'Alice',
            'password': 'SecretPass1',
        })
    assert response.status_code == 201
    assert response.json()['email'] == 'alice@exemple.com'
    assert 'password' not in response.json()
```

---

## Ce que tu ne fais PAS

- Mettre de la logique métier dans les endpoint functions
- Utiliser des fonctions synchrones bloquantes dans des endpoints `async`
- Exposer les passwords ou données sensibles dans les schémas de réponse
- Utiliser `class Config` (déprécié) — utiliser `model_config`
- Ignorer la gestion des transactions dans les opérations multi-étapes
