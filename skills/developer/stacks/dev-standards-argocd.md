---
name: dev-standards-argocd
description: Standards ArgoCD / GitOps — principes GitOps, Applications ArgoCD, External Secrets, Vault, sync policies par environnement.
---

# Skill — Standards ArgoCD / GitOps

## Rôle

Ce skill définit les bonnes pratiques pour le GitOps avec ArgoCD et la gestion
des secrets à l'échelle (External Secrets Operator, HashiCorp Vault).
Il complète `dev-standards-kubernetes.md` et `dev-standards-helm.md`.

---

## 🔒 Règles absolues

❌ Jamais de `kubectl apply` manuel sur un cluster géré par ArgoCD — tout passe par Git
❌ Jamais de secrets en clair dans Git, même dans des branches privées
❌ Jamais de sync automatique sur la production sans validation humaine
✅ Le dépôt Git est la source de vérité unique de l'état du cluster
✅ Toute opération manuelle d'urgence est documentée et suivie d'un commit de synchronisation

---

## Principes GitOps

- Le dépôt Git est la **source de vérité unique** de l'état de l'infrastructure
- Tout changement en production passe par une PR reviewée — jamais par une commande manuelle
- L'état réel du cluster doit converger vers l'état décrit dans Git (self-healing)
- Les opérations manuelles d'urgence sont documentées dans un runbook et suivies d'un commit

---

## Applications ArgoCD

```yaml
# ✅ Application ArgoCD — sync auto sur staging, manuel sur prod
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app-production
  namespace: argocd
  annotations:
    notifications.argoproj.io/subscribe.on-sync-failed.slack: platform-alerts
spec:
  project: default
  source:
    repoURL: https://github.com/org/infra
    targetRevision: main
    path: k8s/overlays/production
  destination:
    server: https://kubernetes.default.svc
    namespace: production
  syncPolicy:
    # Production : pas de sync auto — PR + approval obligatoire
    syncOptions:
      - CreateNamespace=true
      - PrunePropagationPolicy=foreground
  # Sur staging : ajouter automated: { prune: true, selfHeal: true }
```

### Sync policies par environnement

| Environnement | Sync | Prune | Self-heal |
|---|---|---|---|
| dev | automatique | true | true |
| staging | automatique | true | true |
| production | **manuel** | false (avec validation) | false |

---

## AppProject — Isolation des équipes

```yaml
# ✅ AppProject pour limiter le périmètre d'une équipe
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: team-backend
  namespace: argocd
spec:
  description: "Applications de l'équipe backend"
  sourceRepos:
    - "https://github.com/org/infra"
  destinations:
    - namespace: "backend-*"
      server: https://kubernetes.default.svc
  clusterResourceWhitelist: []  # pas de ressources cluster-wide
  namespaceResourceWhitelist:
    - group: "apps"
      kind: Deployment
    - group: ""
      kind: Service
```

---

## External Secrets Operator

Synchronise les secrets depuis un gestionnaire externe (Vault, AWS Secrets Manager, GCP Secret Manager, etc.) vers des Secrets Kubernetes. Les secrets ne sont **jamais** dans Git.

```yaml
# ✅ SecretStore — connexion au backend de secrets
apiVersion: external-secrets.io/v1beta1
kind: SecretStore
metadata:
  name: vault-backend
  namespace: production
spec:
  provider:
    vault:
      server: "https://vault.exemple.com"
      path: "secret"
      version: "v2"
      auth:
        kubernetes:
          mountPath: "kubernetes"
          role: "my-app-production"

---
# ✅ ExternalSecret — synchronisation d'un secret spécifique
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: app-secrets
  namespace: production
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: vault-backend
    kind: SecretStore
  target:
    name: app-secrets
    creationPolicy: Owner
  data:
    - secretKey: database-url
      remoteRef:
        key: production/my-app
        property: database_url
    - secretKey: api-key
      remoteRef:
        key: production/my-app
        property: api_key
```

---

## HashiCorp Vault

- Un path par application et par environnement : `secret/data/<env>/<app>/`
- Rotation automatique des credentials de base de données (Dynamic Secrets)
- Audit log activé sur toutes les opérations
- Principe du moindre privilège sur les policies Vault

```hcl
# ✅ Policy Vault minimale pour une application
path "secret/data/production/my-app/*" {
  capabilities = ["read"]
}

path "secret/metadata/production/my-app/*" {
  capabilities = ["list"]
}
```

---

## Observabilité ArgoCD

- Activer les notifications ArgoCD pour alerter sur :
  - Sync failed → canal alertes plateforme
  - App unhealthy → canal alertes plateforme
  - Deployed en production → canal releases
- Monitorer le drift (état OutOfSync) via des métriques Prometheus ArgoCD

---

## Ce que tu ne fais PAS

- Appliquer des manifests en production sans passer par ArgoCD et une PR approuvée
- Stocker des secrets dans Git, même dans un dépôt privé
- Activer le sync automatique en production
- Créer des Applications ArgoCD avec des droits ClusterAdmin sans justification
- Contourner GitOps "pour aller plus vite" lors d'un incident — documenter l'action manuelle immédiatement
