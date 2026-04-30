---
name: dev-standards-terraform
description: Standards Terraform — structure des modules, variables, state remote, cycle de vie des changements, drift detection.
---

# Skill — Standards Terraform

## Rôle

Ce skill définit les bonnes pratiques pour l'infrastructure as code avec Terraform.
Il complète `dev-standards-devops.md`.

---

## 🔒 Règles absolues

❌ Jamais de `terraform apply` manuel sur un environnement de production — tout passe par un pipeline approuvé
❌ Jamais de secrets en clair dans le code Terraform, les variables ou les outputs
❌ Jamais de modification manuelle d'une ressource gérée par Terraform hors du code
✅ Tout changement passe par `terraform plan` relu avant `terraform apply`
✅ Le state est remote avec verrouillage activé

---

## Structure des modules

```
infrastructure/
├── modules/                    ← modules réutilisables
│   ├── network/
│   │   ├── main.tf
│   │   ├── variables.tf
│   │   ├── outputs.tf
│   │   └── README.md
│   ├── database/
│   └── kubernetes-cluster/
├── environments/               ← configurations par environnement
│   ├── dev/
│   │   ├── main.tf             ← appelle les modules
│   │   ├── variables.tf
│   │   └── terraform.tfvars    ← valeurs (non versionné si secrets)
│   ├── staging/
│   └── production/
└── shared/                     ← ressources partagées (DNS, registry, etc.)
```

- Un module = une responsabilité (réseau, base de données, cluster, etc.)
- Les modules sont versionnés et référencés avec une version fixe :
  `source = "git::https://github.com/org/infra-modules.git//network?ref=v1.2.0"`
- Outputs explicites — ne jamais exposer des secrets en output

---

## Variables

- Variables obligatoires documentées avec `description` et `type`
- Utiliser `validation` pour les contraintes critiques
- Les variables sensibles utilisent `sensitive = true`

```hcl
# ✅ Variable bien documentée avec validation
variable "cluster_node_count" {
  description = "Nombre de nœuds workers du cluster Kubernetes"
  type        = number
  default     = 3

  validation {
    condition     = var.cluster_node_count >= 1 && var.cluster_node_count <= 20
    error_message = "Le nombre de nœuds doit être entre 1 et 20."
  }
}

# ✅ Variable sensible
variable "database_password" {
  description = "Mot de passe de la base de données"
  type        = string
  sensitive   = true
}
```

---

## State remote

- State remote avec verrouillage obligatoire : S3 + DynamoDB, GCS, Terraform Cloud, ou équivalent
- Un workspace Terraform = un environnement (dev, staging, production)
- Ne jamais partager le state entre environnements

```hcl
# ✅ Backend S3 avec verrouillage DynamoDB
terraform {
  backend "s3" {
    bucket         = "mon-projet-terraform-state"
    key            = "production/terraform.tfstate"
    region         = "eu-west-1"
    encrypt        = true
    dynamodb_table = "terraform-state-lock"
  }
}
```

---

## Cycle de vie des changements

1. Modifier le code Terraform sur une branche dédiée
2. `terraform plan` — revoir les changements avant d'appliquer
3. PR avec le plan en commentaire automatique (via CI)
4. Review humaine obligatoire pour les changements en production
5. `terraform apply` via pipeline uniquement — pas de `terraform apply` local sur prod

```bash
# ✅ Workflow local (dev/staging uniquement)
terraform init
terraform workspace select staging
terraform plan -out=tfplan
# Relire le plan avant de continuer
terraform apply tfplan
```

---

## Drift detection

- Configurer `terraform plan` en mode lecture seule en CI pour détecter les drifts
- Alerter si des ressources ont été modifiées hors Terraform
- Documenter les exceptions justifiées (ressources managées en dehors de Terraform)

---

## Naming et tags

- Toutes les ressources ont les tags minimaux : `environment`, `project`, `managed-by = terraform`
- Nommage cohérent : `<projet>-<environnement>-<ressource>` (ex: `monapp-production-db`)
- Utiliser des locals pour centraliser les tags communs :

```hcl
locals {
  common_tags = {
    project     = var.project_name
    environment = var.environment
    managed-by  = "terraform"
  }
}
```

---

## Ce que tu ne fais PAS

- Appliquer un plan sans l'avoir relu — même en environnement de dev
- Stocker des secrets en clair dans les fichiers `.tf` ou `.tfvars`
- Utiliser le state local sur les environnements partagés
- Modifier manuellement des ressources gérées par Terraform
- Versionner des fichiers `.terraform.lock.hcl` différents entre environnements (sauf justification)
