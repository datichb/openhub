---
name: audit-infra
description: "Utiliser quand l'audit porte sur l'IaC (Terraform, Kubernetes, Helm, ArgoCD) : sécurité CIS/NSA, coût, drift ; outils statiques après confirmation."
---

# Skill — Audit Infrastructure (IaC)

## Référentiels couverts

- **CIS Benchmarks** — AWS, GCP, Azure, Kubernetes
- **NSA/CISA Kubernetes Hardening Guide**
- **OWASP Kubernetes Top 10** et bonnes pratiques Terraform (modules, état, versions)
- **Tarifs publics des fournisseurs** — estimation de coût quand `infracost` est absent

L'audit lit le code IaC du dépôt et ne change rien : ni fichier, ni infrastructure.
L'application d'un changement relève du workflow `infra-change` (agent `operator`),
l'écriture du code IaC de l'agent `developer`.

---

## Méthode

1. **Inventaire** : repérer les racines IaC (`*.tf`, `Chart.yaml`, `kustomization.yaml`,
   manifestes `apiVersion:`, `Pulumi.yaml`, `Application`/`ApplicationSet` ArgoCD) et les
   environnements (dossiers, workspaces, fichiers de valeurs).
2. **Lecture** : appliquer les checklists ci-dessous fichier par fichier.
3. **Outils** : lancer les outils d'analyse disponibles (section Outils) ; chaque commande est
   demandée à l'utilisateur. Un outil absent n'est pas bloquant : le noter dans le périmètre
   non couvert et poursuivre à la lecture.
4. **Rapport** : format `audit-protocol-light`, résultats bruts des outils résumés en annexe
   (règle, ressource, fichier:ligne), sans recopier de valeur sensible.

---

## Checklist — Revue IaC

### Terraform / OpenTofu

- [ ] Versions épinglées : `required_version`, `required_providers` avec contrainte, `.terraform.lock.hcl` versionné
- [ ] État dans un backend distant avec verrouillage (S3 `use_lockfile`, GCS, azurerm, Terraform Cloud), jamais local pour un environnement partagé
- [ ] Environnements séparés (dossiers, workspaces ou comptes distincts), pas de valeurs prod codées dans un module commun
- [ ] Modules réutilisés avec une source versionnée (tag, registre), pas de `ref=main`
- [ ] Variables typées et validées (`validation`), sorties sensibles marquées `sensitive = true`
- [ ] `lifecycle { prevent_destroy = true }` sur les ressources à données (bases, buckets, volumes)
- [ ] Étiquetage cohérent (`default_tags` : propriétaire, environnement, coût)

### Kubernetes

- [ ] `resources.requests` et `resources.limits` sur chaque conteneur
- [ ] `livenessProbe` et `readinessProbe` définies (et `startupProbe` pour un démarrage lent)
- [ ] Images épinglées par tag immuable ou digest, jamais `latest`
- [ ] `PodDisruptionBudget` et plusieurs réplicas pour les services critiques
- [ ] Namespaces dédiés, `ResourceQuota`/`LimitRange` par namespace partagé

### Helm et ArgoCD

- [ ] `Chart.yaml` à jour (`version`, `appVersion`), dépendances épinglées
- [ ] `values.yaml` structuré et documenté, une surcharge par environnement
- [ ] ArgoCD : `syncPolicy` explicite, `prune`/`selfHeal` justifiés, projet (`AppProject`) restreint, health checks

---

## Checklist — Sécurité IaC

Findings 🔴 à remonter systématiquement :

- [ ] Stockage public : bucket S3/GCS/Blob lisible publiquement, ACL `public-read`, absence de « Block Public Access »
- [ ] Secrets en clair : mots de passe, tokens, clés dans les `.tf`, `values.yaml`, manifestes `Secret` versionnés ou `tfvars` commités
- [ ] Réseau ouvert : groupes de sécurité `0.0.0.0/0` sur SSH, RDP ou un port de base de données
- [ ] IAM trop large : `Action: "*"` / `Resource: "*"`, `ClusterRoleBinding` vers `cluster-admin` sans justification
- [ ] Conteneurs privilégiés : `privileged: true`, `hostNetwork`, `hostPath`, `runAsUser: 0`

À vérifier ensuite :

- [ ] `securityContext` : `runAsNonRoot: true`, `allowPrivilegeEscalation: false`, `readOnlyRootFilesystem: true`, `capabilities.drop: [ALL]`
- [ ] `NetworkPolicy` par défaut refusant le trafic, ouvertures explicites
- [ ] Chiffrement au repos (KMS, volumes, bases, buckets) et en transit (TLS, `ssl_enforcement`)
- [ ] Journaux d'audit activés (CloudTrail, Cloud Audit Logs, logs d'audit du control plane)
- [ ] Secrets fournis par un gestionnaire (External Secrets, Vault, Secret Manager), pas par des variables en clair
- [ ] Pod Security Admission (`restricted` ou `baseline`) appliquée par namespace

---

## Coût

- Lancer `infracost breakdown --path <dossier>` (état actuel) ou `infracost diff --path <dossier>`
  (changement en cours) si l'outil et sa clé sont configurés ; sinon estimer à partir des tarifs
  publics en le signalant comme estimation.
- Rapporter le coût mensuel par environnement et les ressources les plus chères.
- Signaler en 🟠 Majeur : instances surdimensionnées sans autoscaling, volumes ou IP orphelins,
  absence de cycle de vie sur les buckets de journaux, NAT Gateway par zone sans besoin.
- Toute hausse supérieure à +20 % du coût mensuel liée au changement audité est un 🟠 Majeur.

---

## Drift

- Terraform/OpenTofu : `terraform plan -input=false -detailed-exitcode` (code 2 : écart entre le
  code et l'infrastructure réelle). Le plan lit l'état distant avec les identifiants de
  l'utilisateur : le proposer seulement si l'utilisateur dispose d'un accès en lecture seule, et
  le signaler dans le périmètre.
- Kubernetes : proposer à l'utilisateur `kubectl diff -f <dossier>` <!-- capability: user --> et lui demander le résultat.
- Chaque ressource modifiée hors du code est un finding (🟠 Majeur, 🔴 si elle touche la sécurité).

---

## Outils (analyse statique, après confirmation)

| Outil | Commande | Cible |
|-------|----------|-------|
| tflint | `tflint --recursive` | Terraform : erreurs, conventions, règles fournisseur |
| tfsec / trivy | `tfsec .` ou `trivy config .` | Terraform, Kubernetes, Helm, Dockerfile |
| checkov | `checkov -d . --compact` | Terraform, Kubernetes, Helm, CloudFormation |
| kube-score | `kube-score score <manifestes>` | Manifestes Kubernetes |
| kubesec | `kubesec scan <manifeste>` | Manifestes Kubernetes |
| Helm | `helm lint <chart>`, `helm template <chart>` | Rendu des charts avant analyse |
| Terraform | `terraform fmt -check -recursive`, `terraform validate` | Syntaxe et cohérence |
| infracost | `infracost breakdown --path <dossier>` | Coût mensuel |

`terraform validate` demande un `terraform init -backend=false -lockfile=readonly` préalable :
il télécharge les fournisseurs dans `.terraform/` sans toucher à l'état distant ni au fichier de
verrouillage. Les autres outils (`kubectl`, commandes cloud, `terraform output`) sont refusés
dans ce domaine : proposer à l'utilisateur la commande et lui demander le résultat.

---

## Ce que tu ne fais PAS dans ce domaine

- Appliquer, détruire ou importer quoi que ce soit (`infra-change` s'en charge)
- Lire des secrets de l'infrastructure réelle ou les recopier dans le rapport
- Certifier une conformité CIS, SOC 2 ou ISO 27001 — tu identifies les écarts, l'humain décide
