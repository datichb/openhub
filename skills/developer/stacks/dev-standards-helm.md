---
name: dev-standards-helm
description: Standards Helm — structure d'un chart, valeurs par environnement, secrets via External Secrets, helm diff, versioning SemVer.
---

# Skill — Standards Helm

## Rôle

Ce skill définit les bonnes pratiques pour la création et la maintenance de charts Helm.
Il complète `dev-standards-kubernetes.md`.

---

## 🔒 Règles absolues

❌ Jamais de secrets en clair dans `values.yaml` ou tout fichier versionné
❌ Jamais de `helm upgrade` en production sans `helm diff` préalable
❌ Jamais de chart sans `Chart.yaml` versionné en SemVer
✅ Les secrets passent par External Secrets Operator ou un mécanisme équivalent
✅ Chaque release est nommée de façon cohérente

---

## Structure d'un chart

```
my-chart/
├── Chart.yaml              ← métadonnées (nom, version SemVer, description, appVersion)
├── values.yaml             ← valeurs par défaut (pas de secrets)
├── values-staging.yaml     ← surcharges staging
├── values-prod.yaml        ← surcharges production (pas de secrets)
└── templates/
    ├── _helpers.tpl        ← templates réutilisables (labels, noms)
    ├── deployment.yaml
    ├── service.yaml
    ├── ingress.yaml
    ├── configmap.yaml
    ├── externalsecret.yaml ← référence vers un gestionnaire de secrets externe
    ├── serviceaccount.yaml
    └── hpa.yaml            ← optionnel
```

---

## Chart.yaml

```yaml
# ✅ Chart.yaml complet
apiVersion: v2
name: my-app
description: Application principale du projet
type: application
version: 1.3.0       # version du chart — SemVer
appVersion: "2.1.4"  # version de l'application déployée
maintainers:
  - name: platform-team
    email: platform@exemple.com
```

---

## values.yaml

- Valeurs par défaut documentées avec des commentaires
- Jamais de secrets — utiliser des références à External Secrets
- Structure cohérente avec les templates

```yaml
# ✅ values.yaml bien structuré
replicaCount: 2

image:
  repository: my-registry/my-app
  tag: ""           # surchargé au déploiement avec le SHA de commit
  pullPolicy: IfNotPresent

resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi

autoscaling:
  enabled: false
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70

ingress:
  enabled: true
  className: nginx
  host: app.exemple.com

externalSecrets:
  secretStoreName: vault-backend  # nom du SecretStore configuré dans le cluster
  refreshInterval: 1h
```

---

## Templates

- Utiliser `_helpers.tpl` pour les labels communs et les noms de ressources
- Préfixer toutes les ressources avec `{{ include "my-chart.fullname" . }}`
- Utiliser `{{ .Values.xxx | required "message" }}` pour les valeurs obligatoires

```yaml
# ✅ _helpers.tpl — labels communs
{{- define "my-chart.labels" -}}
helm.sh/chart: {{ include "my-chart.chart" . }}
app.kubernetes.io/name: {{ include "my-chart.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}
```

---

## Secrets via External Secrets

```yaml
# ✅ ExternalSecret dans un template Helm
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: {{ include "my-chart.fullname" . }}-secrets
spec:
  refreshInterval: {{ .Values.externalSecrets.refreshInterval }}
  secretStoreRef:
    name: {{ .Values.externalSecrets.secretStoreName }}
    kind: SecretStore
  target:
    name: {{ include "my-chart.fullname" . }}-secrets
  data:
    - secretKey: database-url
      remoteRef:
        key: {{ .Release.Namespace }}/{{ include "my-chart.name" . }}
        property: database_url
```

---

## Déploiement

```bash
# ✅ Workflow de déploiement sécurisé
# 1. Voir les différences avant d'appliquer
helm diff upgrade my-app-production ./my-chart \
  -f values.yaml \
  -f values-prod.yaml \
  --set image.tag=abc1234

# 2. Appliquer après validation
helm upgrade --install my-app-production ./my-chart \
  -f values.yaml \
  -f values-prod.yaml \
  --set image.tag=abc1234 \
  --namespace production \
  --atomic \
  --timeout 5m
```

- Nommer les releases de façon cohérente : `<app>-<env>` (ex: `api-production`)
- Utiliser `--atomic` pour le rollback automatique en cas d'échec
- Passer le tag d'image en `--set` depuis le pipeline (jamais hardcodé dans values.yaml)

---

## Versioning

- Incrémenter `version` dans `Chart.yaml` à chaque modification du chart (SemVer)
- `appVersion` reflète la version de l'application — distinct de la version du chart
- Publier les charts dans un registry OCI ou un repository Helm dédié

---

## Ce que tu ne fais PAS

- Stocker des secrets en clair dans `values.yaml` ou tout fichier versionné
- Faire un `helm upgrade` en production sans `helm diff`
- Utiliser `latest` comme tag d'image dans les values
- Créer des templates sans passer par `_helpers.tpl` pour les labels
- Omettre `--atomic` sur les déploiements critiques
