---
name: dev-standards-sqlalchemy
description: Standards SQLAlchemy — models, sessions, Alembic, requêtes async, transactions et bonnes pratiques.
---

# Skill — Standards SQLAlchemy

## Rôle

Ce skill définit les bonnes pratiques pour l'accès aux données avec SQLAlchemy (v2+).
Il complète `dev-standards-python.md` et `dev-standards-backend.md`.

---

## 🔒 Règles absolues

❌ Jamais de `Base.metadata.create_all()` en production — utiliser Alembic
❌ Jamais de sessions partagées entre threads ou requêtes
❌ Jamais de requêtes string interpolées — utiliser les paramètres liés
✅ Toute migration Alembic est relue avant d'être appliquée en production

---

## Models (SQLAlchemy v2 — Mapped)

```python
# models/user.py
import uuid
from datetime import datetime
from sqlalchemy import String, Boolean, DateTime, func
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column, relationship

class Base(DeclarativeBase):
    pass

class User(Base):
    __tablename__ = "users"

    id: Mapped[str] = mapped_column(String, primary_key=True, default=lambda: str(uuid.uuid4()))
    name: Mapped[str] = mapped_column(String(100), nullable=False)
    email: Mapped[str] = mapped_column(String, unique=True, nullable=False, index=True)
    password: Mapped[str] = mapped_column(String, nullable=False)
    active: Mapped[bool] = mapped_column(Boolean, default=True, index=True)
    created_at: Mapped[datetime] = mapped_column(DateTime, server_default=func.now())
    updated_at: Mapped[datetime] = mapped_column(DateTime, server_default=func.now(), onupdate=func.now())

    orders: Mapped[list["Order"]] = relationship("Order", back_populates="user", lazy="select")

    def __repr__(self) -> str:
        return f"<User id={self.id} email={self.email}>"
```

---

## Sessions — async (avec asyncpg)

```python
# database.py
from sqlalchemy.ext.asyncio import AsyncSession, create_async_engine, async_sessionmaker

engine = create_async_engine(
    settings.async_database_url,
    echo=settings.debug,
    pool_size=10,
    max_overflow=20,
)

AsyncSessionFactory = async_sessionmaker(engine, expire_on_commit=False)

async def get_session() -> AsyncSession:
    async with AsyncSessionFactory() as session:
        yield session
```

---

## Requêtes

```python
# repositories/user_repository.py
from sqlalchemy import select, update
from sqlalchemy.ext.asyncio import AsyncSession

class UserRepository:
    def __init__(self, session: AsyncSession) -> None:
        self.session = session

    async def find_by_id(self, user_id: str) -> User | None:
        result = await self.session.get(User, user_id)
        return result

    async def find_by_email(self, email: str) -> User | None:
        stmt = select(User).where(User.email == email)
        result = await self.session.execute(stmt)
        return result.scalar_one_or_none()

    async def find_active(self, limit: int = 20, offset: int = 0) -> list[User]:
        stmt = (
            select(User)
            .where(User.active == True)
            .order_by(User.created_at.desc())
            .limit(limit)
            .offset(offset)
        )
        result = await self.session.execute(stmt)
        return list(result.scalars())

    async def create(self, data: dict) -> User:
        user = User(**data)
        self.session.add(user)
        await self.session.flush()  # obtenir l'id sans commit
        return user
```

---

## Transactions

```python
# ✅ Transaction explicite avec context manager
async def create_user_with_credit(session: AsyncSession, data: dict) -> User:
    async with session.begin():
        user = User(**data)
        session.add(user)
        await session.flush()

        credit = Credit(user_id=user.id, amount=10, reason="welcome")
        session.add(credit)
        # commit automatique à la sortie du context manager

    return user

# ✅ Rollback explicite en cas d'erreur
async def safe_create(session: AsyncSession, data: dict) -> User | None:
    try:
        async with session.begin():
            user = User(**data)
            session.add(user)
            return user
    except IntegrityError:
        await session.rollback()
        return None
```

---

## Alembic — migrations

```bash
# Générer une migration automatique
alembic revision --autogenerate -m "add_user_phone_field"

# Appliquer les migrations
alembic upgrade head

# Voir l'historique
alembic history --verbose

# Revenir en arrière
alembic downgrade -1
```

```python
# alembic/env.py — configuration avec les modèles
from app.models import Base
target_metadata = Base.metadata
```

- Relire le fichier de migration généré — `--autogenerate` peut manquer des changements
- Les migrations destructrices (suppression de colonne) ont une migration de transition

---

## Tests

```python
# ✅ Test avec base in-memory SQLite
import pytest
from sqlalchemy.ext.asyncio import create_async_engine, AsyncSession, async_sessionmaker

@pytest.fixture
async def session():
    engine = create_async_engine("sqlite+aiosqlite:///:memory:")
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)

    factory = async_sessionmaker(engine, expire_on_commit=False)
    async with factory() as session:
        yield session

    await engine.dispose()

async def test_find_user_returns_none_when_not_found(session: AsyncSession):
    repo = UserRepository(session)
    result = await repo.find_by_id("non-existant")
    assert result is None
```

---

## Ce que tu ne fais PAS

- Utiliser `Base.metadata.create_all()` en production
- Partager une session entre plusieurs requêtes ou threads
- Interpoler des variables dans les requêtes SQL — utiliser les expressions SQLAlchemy
- Oublier `expire_on_commit=False` sur les sessions async (évite les lazy load après commit)
- Créer des queries N+1 — utiliser `selectinload()` ou `joinedload()` explicitement
