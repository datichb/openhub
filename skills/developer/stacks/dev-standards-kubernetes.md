---
name: dev-standards-kubernetes
description: Standards Kubernetes — structure des manifests, Deployments, RBAC, Network Policies, probes, resource quotas et Kustomize.
---

# Skill — Standards Kubernetes

## Rôle

Ce skill définit les bonnes pratiques pour la configuration et le déploiement
d'applications sur Kubernetes.
Il complète `dev-standards-devops.md`.

---

## 🔒 Règles absolues

❌ Jamais de `kubectl apply` manuel sur un cluster de production — tout passe par GitOps ou un pipeline approuvé
❌ Jamais de secrets en clair dans les manifests ou dans Git
❌ Jamais de tag `latest` dans les images des manifests
❌ Jamais de containers tournant en root sans justification documentée
✅ Tout container a des `requests` et `limits` définis
✅ Tout container a des probes `liveness` et `readiness` définis

---

## Structure des manifests (Kustomize)

```
k8s/
├── base/                       ← configuration commune
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── configmap.yaml
│   └── kustomization.yaml
└── overlays/                   ← surcharges par environnement
    ├── dev/
    │   └── kustomization.yaml  ← patch replicas=1, resources réduits
    ├── staging/
    │   └── kustomization.yaml
    └── production/
        └── kustomization.yaml  ← patch replicas=3, HPA, PDB
```

---

## Deployment

```yaml
# ✅ Deployment bien configuré
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  labels:
    app: my-app
    version: "1.2.3"
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
        version: "1.2.3"
    spec:
      serviceAccountName: my-app  # ServiceAccount dédié — jamais default
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        fsGroup: 1000
      containers:
        - name: my-app
          image: my-registry/my-app:abc1234  # SHA ou tag précis — jamais latest
          resources:
            requests:
              cpu: "100m"
              memory: "128Mi"
            limits:
              cpu: "500m"
              memory: "512Mi"
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 10
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /ready
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
            failureThreshold: 3
          env:
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: app-secrets
                  key: database-url
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop: ["ALL"]
```

---

## RBAC — Principe du moindre privilège

- Un `ServiceAccount` par application — jamais le ServiceAccount `default`
- Les rôles sont définis au niveau namespace (`Role`) sauf nécessité absolue (`ClusterRole`)
- Auditer les `ClusterRoleBinding` régulièrement
- Pas de `verbs: ["*"]` ni de `resources: ["*"]` sauf cas documenté et validé

```yaml
# ✅ ServiceAccount dédié
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-app
  namespace: production

---
# ✅ Role minimal
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: my-app-reader
  namespace: production
rules:
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get", "list"]

---
# ✅ Binding
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: my-app-reader
  namespace: production
subjects:
  - kind: ServiceAccount
    name: my-app
roleRef:
  kind: Role
  name: my-app-reader
  apiGroup: rbac.authorization.k8s.io
```

---

## Network Policies

- Par défaut : `deny all` entre namespaces
- Ouvrir explicitement uniquement les flux nécessaires
- Documenter chaque Network Policy avec sa justification

```yaml
# ✅ Deny-all par défaut, puis ouverture explicite
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: deny-all
  namespace: production
spec:
  podSelector: {}
  policyTypes: [Ingress, Egress]

---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-app-to-db
  namespace: production
spec:
  podSelector:
    matchLabels:
      app: my-app
  policyTypes: [Egress]
  egress:
    - to:
        - podSelector:
            matchLabels:
              app: postgres
      ports:
        - port: 5432
```

---

## Resource Quotas et Limits

- Chaque namespace a un `ResourceQuota` défini
- Les `limits` ne dépassent pas 4x les `requests` (éviter le throttling brutal)
- Définir un `LimitRange` pour les valeurs par défaut

```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: production-quota
  namespace: production
spec:
  hard:
    requests.cpu: "4"
    requests.memory: 8Gi
    limits.cpu: "16"
    limits.memory: 32Gi
    pods: "50"
```

---

## Haute disponibilité

- `replicas: ≥ 2` en production pour tous les services critiques
- `PodDisruptionBudget` sur les services critiques
- `HorizontalPodAutoscaler` pour les services à charge variable

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: my-app-pdb
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: my-app
```

---

## Ce que tu ne fais PAS

- Appliquer des manifests en production sans passer par GitOps ou un pipeline approuvé
- Stocker des secrets en clair dans les manifests ou dans Git
- Utiliser `latest` comme tag d'image
- Créer des containers sans `requests`/`limits`
- Utiliser le ServiceAccount `default` pour les applications
- Créer des ClusterRoles avec `verbs: ["*"]` sans justification documentée
