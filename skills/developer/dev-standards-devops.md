---
name: dev-standards-devops
description: Standards DevOps généraux — scripts shell, gestion des secrets, registries d'images, observabilité et principes d'infrastructure as code. Les standards spécifiques aux outils (Docker, GitHub Actions, GitLab CI, Terraform, Kubernetes) sont dans les skills dédiés.
---

# Skill — Standards DevOps

## Rôle

Ce skill définit les bonnes pratiques DevOps générales applicables à tous les projets,
indépendamment des outils utilisés.
Il complète `dev-standards-universal.md`.

Les standards spécifiques aux outils sont dans les skills dédiés :
`dev-standards-docker`, `dev-standards-github-actions`, `dev-standards-gitlab-ci`,
`dev-standards-terraform`, `dev-standards-kubernetes`, etc.

---

## 🔒 Règles absolues

❌ Jamais de secrets, tokens ou credentials dans le code, les configs ou les pipelines
❌ Jamais de `latest` comme tag d'image en production — toujours une version épinglée
❌ Ne jamais pousser directement sur `main`/`master` depuis un pipeline sans validation
✅ Tout changement d'infrastructure critique passe par une review humaine
✅ Les pipelines sont idempotents — relancer n'a pas d'effet de bord

---

## Scripts shell

- Toujours commencer par `#!/usr/bin/env bash` et `set -euo pipefail`
- Pas de `local` en dehors d'une fonction bash
- Toutes les variables sont entre guillemets : `"$variable"` (pas `$variable`)
- Pas de parsing de `ls` — utiliser des globs ou `find`
- Les fonctions ont des noms en `snake_case` et sont documentées
- Les scripts ont un message d'usage (`usage()`) et gèrent `--help`
- Pas de chemins absolus codés en dur — utiliser `$(dirname "$0")` pour les chemins relatifs au script

```bash
#!/usr/bin/env bash
set -euo pipefail

# Description : déploie l'application sur l'environnement cible
# Usage : ./deploy.sh <environment>

usage() {
  echo "Usage: $0 <environment>"
  echo "  environment : staging | production"
  exit 1
}

main() {
  local environment="${1:-}"

  if [[ -z "$environment" ]]; then
    usage
  fi

  case "$environment" in
    staging|production)
      echo "Déploiement sur $environment..."
      ;;
    *)
      echo "Environnement inconnu : $environment" >&2
      usage
      ;;
  esac
}

main "$@"
```

---

## Gestion des secrets

- Les secrets ne sont jamais dans le code source ni dans les fichiers versionnés
- `.env` est dans `.gitignore` — `.env.example` est versionné avec des valeurs fictives
- En production : gestionnaire de secrets (AWS Secrets Manager, HashiCorp Vault, Doppler, etc.)
- En CI/CD : variables d'environnement injectées par le système
- Rotation régulière des secrets critiques (tokens, clés API)
- Principe du moindre privilège : un secret = un service, une portée minimale

---

## Registries d'images

- Taguer les images avec : le SHA de commit + un tag de version sémantique si release
  - `myapp:abc1234` (toujours)
  - `myapp:v1.2.3` (sur release)
  - `myapp:latest` (uniquement en développement local, jamais en production)
- Scanner les images avant push
- Nettoyer régulièrement les images non utilisées (politique de rétention)
- Utiliser un registry privé pour les images propriétaires

---

## Observabilité

- Chaque service expose un endpoint de healthcheck (`/health` ou `/_health`)
- Les logs sont structurés (JSON) avec les champs : `level`, `timestamp`, `message`, `service`, `trace_id`
- Les métriques applicatives sont exposées dans un format standard (Prometheus `/metrics` ou équivalent)
- Les alertes sont définies en code (pas de configuration manuelle dans les UIs)

---

## Infrastructure as Code

- Tout changement d'infrastructure est versionné et reviewé (même les petits scripts)
- Les environnements sont reproductibles : dev ≈ staging ≈ production (différences documentées)
- Les configurations d'environnement sont séparées du code d'infrastructure
- Documenter les prérequis et la procédure de bootstrap dans le README
- Les plans de changement (terraform plan, dry-run) sont reviewés avant application

---

## Ce que tu ne fais PAS

- Modifier directement les configurations de production sans pipeline validé
- Utiliser `--force` sur des opérations git ou des déploiements sans confirmation explicite
- Créer des credentials avec des droits plus larges que nécessaire
- Ignorer les échecs de pipeline "pour aller plus vite"
