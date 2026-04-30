---
name: dev-standards-django
description: Standards Django — models, views, serializers, DRF, migrations, permissions, tests et bonnes pratiques.
---

# Skill — Standards Django

## Rôle

Ce skill définit les bonnes pratiques pour le développement backend avec Django
et Django REST Framework (DRF).
Il complète `dev-standards-python.md`, `dev-standards-backend.md` et
`dev-standards-api.md`.

---

## 🔒 Règles absolues

❌ Jamais de logique métier dans les views ou les serializers — extraire dans des services ou des managers
❌ Jamais de `DEBUG=True` en production
❌ Jamais d'accès direct à `request.user` sans vérification des permissions
✅ Toute migration est relue avant d'être appliquée en production

---

## Structure du projet

```
project/
├── config/
│   ├── settings/
│   │   ├── base.py         ← settings communs
│   │   ├── development.py
│   │   └── production.py
│   ├── urls.py
│   └── wsgi.py
├── apps/
│   └── users/
│       ├── models.py
│       ├── serializers.py
│       ├── views.py
│       ├── urls.py
│       ├── services.py     ← logique métier
│       ├── permissions.py  ← permissions custom
│       ├── filters.py      ← filtres DRF
│       ├── admin.py
│       └── tests/
│           ├── test_models.py
│           ├── test_views.py
│           └── test_services.py
└── manage.py
```

---

## Models

- Utiliser des `UUID` comme clés primaires pour les ressources exposées en API
- `created_at` et `updated_at` systématiques via un `BaseModel` abstrait
- Les méthodes métier appartiennent au modèle ou à un manager custom

```python
# ✅ BaseModel réutilisable
import uuid
from django.db import models

class BaseModel(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        abstract = True

# ✅ Model avec manager custom
class UserManager(models.Manager):
    def active(self):
        return self.filter(is_active=True)

class User(BaseModel):
    email = models.EmailField(unique=True)
    name = models.CharField(max_length=100)
    is_active = models.BooleanField(default=True)

    objects = UserManager()

    class Meta:
        ordering = ['-created_at']
        indexes = [models.Index(fields=['email'])]

    def __str__(self) -> str:
        return self.email
```

---

## Serializers (DRF)

- Les serializers valident et transforment — pas de logique métier
- Utiliser `SerializerMethodField` avec parcimonie — extraire dans des propriétés de modèle
- Séparer les serializers d'entrée et de sortie si les formes diffèrent

```python
# ✅ Serializers distincts lecture/écriture
from rest_framework import serializers

class UserWriteSerializer(serializers.ModelSerializer):
    class Meta:
        model = User
        fields = ['email', 'name', 'password']
        extra_kwargs = {'password': {'write_only': True}}

    def validate_email(self, value: str) -> str:
        if User.objects.filter(email=value).exists():
            raise serializers.ValidationError("Cet email est déjà utilisé.")
        return value.lower()

class UserReadSerializer(serializers.ModelSerializer):
    class Meta:
        model = User
        fields = ['id', 'email', 'name', 'created_at']
```

---

## Views (DRF)

- Utiliser les `ViewSet` ou les `APIView` selon la complexité
- Les views délèguent la logique métier aux services
- Les permissions sont déclarées sur les views, pas dans la logique métier

```python
# ✅ ViewSet avec permissions et délégation au service
from rest_framework import viewsets, permissions, status
from rest_framework.response import Response

class UserViewSet(viewsets.ModelViewSet):
    permission_classes = [permissions.IsAuthenticated]
    queryset = User.objects.active()

    def get_serializer_class(self):
        if self.action in ('create', 'update', 'partial_update'):
            return UserWriteSerializer
        return UserReadSerializer

    def perform_create(self, serializer):
        user_service.create_user(serializer.validated_data)

    def perform_update(self, serializer):
        user_service.update_user(self.get_object(), serializer.validated_data)
```

---

## Services

```python
# ✅ Service — logique métier isolée
from django.db import transaction

class UserService:
    @transaction.atomic
    def create_user(self, data: dict) -> User:
        password = data.pop('password')
        user = User(**data)
        user.set_password(password)
        user.save()
        self.send_welcome_email(user)
        return user

    def send_welcome_email(self, user: User) -> None:
        # déléguer à un task Celery ou un service email
        ...

user_service = UserService()
```

---

## Migrations

- Relire chaque migration avant de l'appliquer — surtout en production
- Les migrations destructrices (suppression de colonne, table) ont une migration de transition
- Ne jamais modifier une migration déjà appliquée en production — créer une nouvelle

```bash
# ✅ Workflow migration
python manage.py makemigrations --name "add_user_phone_field"
# Relire 0005_add_user_phone_field.py avant de continuer
python manage.py migrate --plan   # voir ce qui va être appliqué
python manage.py migrate
```

---

## Tests

```python
# ✅ Test de vue avec APIClient
from rest_framework.test import APIClient
from rest_framework import status
import pytest

@pytest.mark.django_db
class TestUserViewSet:
    def test_create_user_returns_201(self, api_client: APIClient):
        response = api_client.post('/api/users/', {
            'email': 'alice@exemple.com',
            'name': 'Alice',
            'password': 'SecretPass1',
        })
        assert response.status_code == status.HTTP_201_CREATED
        assert response.data['email'] == 'alice@exemple.com'
        assert 'password' not in response.data

    def test_create_user_duplicate_email_returns_400(self, api_client, user):
        response = api_client.post('/api/users/', {'email': user.email, 'name': 'Bob', 'password': 'SecretPass1'})
        assert response.status_code == status.HTTP_400_BAD_REQUEST
```

---

## Ce que tu ne fais PAS

- Mettre de la logique métier dans les views ou les serializers
- Utiliser `DEBUG=True` en production
- Laisser `ALLOWED_HOSTS = ['*']` en production
- Omettre les indexes sur les colonnes fréquemment filtrées
- Modifier une migration déjà appliquée en production
