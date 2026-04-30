---
name: dev-standards-rails
description: Standards Ruby on Rails — MVC, ActiveRecord, concerns, services objects, callbacks, tests RSpec et bonnes pratiques.
---

# Skill — Standards Ruby on Rails

## Rôle

Ce skill définit les bonnes pratiques pour le développement backend avec Ruby on Rails.
Il complète `dev-standards-backend.md` et `dev-standards-api.md`.

---

## 🔒 Règles absolues

❌ Jamais de logique métier complexe dans les controllers ou les callbacks ActiveRecord
❌ Jamais de secrets en dur dans le code — utiliser les credentials Rails ou les variables d'environnement
❌ Jamais de `rails console` en production pour des modifications de données — créer une migration ou un rake task
✅ Toute migration destructrice est soumise à validation humaine explicite

---

## Structure du projet

```
app/
├── controllers/
│   └── api/
│       └── v1/
│           └── users_controller.rb
├── models/
│   └── user.rb
├── services/               ← service objects (logique métier)
│   └── users/
│       └── create_user.rb
├── serializers/            ← sérialisation des réponses (JSONAPI ou custom)
│   └── user_serializer.rb
├── policies/               ← autorisations (Pundit)
│   └── user_policy.rb
└── queries/                ← query objects (requêtes complexes)
    └── users_query.rb
```

---

## Models ActiveRecord

- Validations dans les models pour l'intégrité des données
- Scopes nommés pour les requêtes fréquentes
- Callbacks (`before_save`, `after_create`, etc.) avec parcimonie — préférer les service objects

```ruby
# ✅ Model bien structuré
class User < ApplicationRecord
  # Associations
  has_many :orders, dependent: :destroy

  # Validations
  validates :email, presence: true, uniqueness: { case_sensitive: false }, format: { with: URI::MailTo::EMAIL_REGEXP }
  validates :name, presence: true, length: { minimum: 2, maximum: 100 }

  # Scopes
  scope :active, -> { where(active: true) }
  scope :recent, -> { order(created_at: :desc) }

  # Normalisation
  before_validation { self.email = email&.downcase&.strip }

  # Méthode métier simple
  def full_display_name
    "#{name} <#{email}>"
  end
end
```

---

## Controllers

```ruby
# ✅ Controller mince — délègue au service
module Api
  module V1
    class UsersController < ApplicationController
      before_action :authenticate_user!
      before_action :set_user, only: [:show, :update, :destroy]

      def show
        render json: UserSerializer.new(@user).serializable_hash
      end

      def create
        result = Users::CreateUser.call(user_params)
        if result.success?
          render json: UserSerializer.new(result.user).serializable_hash, status: :created
        else
          render json: { errors: result.errors }, status: :unprocessable_entity
        end
      end

      private

      def set_user
        @user = User.find(params[:id])
      end

      def user_params
        params.require(:user).permit(:name, :email, :password)
      end
    end
  end
end
```

---

## Service Objects

```ruby
# ✅ Service object — logique métier isolée
module Users
  class CreateUser
    include ActiveModel::Model

    attr_reader :user, :errors

    def self.call(params)
      new(params).call
    end

    def initialize(params)
      @params = params
      @errors = []
    end

    def call
      return failure("Email déjà utilisé") if User.exists?(email: @params[:email])

      ActiveRecord::Base.transaction do
        @user = User.create!(@params)
        UserMailer.welcome_email(@user).deliver_later
      end

      self
    rescue ActiveRecord::RecordInvalid => e
      failure(e.record.errors.full_messages)
    end

    def success?
      @errors.empty?
    end

    private

    def failure(message)
      @errors = Array(message)
      self
    end
  end
end
```

---

## Query Objects

```ruby
# ✅ Query object pour les requêtes complexes
class UsersQuery
  def initialize(relation = User.all)
    @relation = relation
  end

  def active_with_recent_orders(since:)
    @relation
      .active
      .joins(:orders)
      .where(orders: { created_at: since.. })
      .distinct
      .select('users.*, COUNT(orders.id) as orders_count')
      .group('users.id')
  end
end
```

---

## Migrations

```ruby
# ✅ Migration bien structurée
class CreateUsers < ActiveRecord::Migration[7.1]
  def change
    create_table :users, id: :uuid do |t|
      t.string :name, null: false
      t.string :email, null: false
      t.string :password_digest, null: false
      t.boolean :active, default: true, null: false

      t.timestamps
    end

    add_index :users, :email, unique: true
    add_index :users, :active
  end
end
```

- Ne jamais modifier une migration déjà exécutée en production
- Utiliser `reversible` ou `up`/`down` pour les migrations complexes

---

## Tests (RSpec)

```ruby
# ✅ Test de controller avec request specs
RSpec.describe 'POST /api/v1/users', type: :request do
  context 'avec des données valides' do
    let(:params) { { user: { name: 'Alice', email: 'alice@exemple.com', password: 'SecretPass1' } } }

    it 'crée l\'utilisateur et retourne 201' do
      post '/api/v1/users', params: params, as: :json

      expect(response).to have_http_status(:created)
      expect(json_body.dig('data', 'email')).to eq('alice@exemple.com')
      expect(json_body['data']).not_to have_key('password_digest')
    end
  end

  context 'avec un email déjà utilisé' do
    let!(:existing_user) { create(:user, email: 'alice@exemple.com') }

    it 'retourne 422' do
      post '/api/v1/users', params: { user: { name: 'Bob', email: 'alice@exemple.com', password: 'Pass1' } }, as: :json
      expect(response).to have_http_status(:unprocessable_entity)
    end
  end
end
```

---

## Ce que tu ne fais PAS

- Mettre de la logique métier complexe dans les controllers ou les callbacks ActiveRecord
- Utiliser `update_all` ou `delete_all` sans clause `where` soigneusement réfléchie
- Exposer `password_digest` ou données sensibles dans les sérialiseurs
- Ignorer les transactions pour les opérations multi-étapes
- Créer des callbacks ActiveRecord pour des effets de bord (emails, notifications) — utiliser des service objects
