---
name: audit-observability
description: "Utiliser quand l'audit porte sur l'observabilité : métriques RED, logs structurés, traces, SLO, alerting, dashboards."
---

# Skill — Audit Observabilité

## Rôle

Ce skill définit le référentiel d'audit de l'observabilité d'un système en production.
L'objectif est d'évaluer si l'équipe peut répondre à la question
**"que se passe-t-il ?"** en moins de 5 minutes lors d'un incident.

---

## Les 3 piliers de l'observabilité

### 1. Métriques — Méthode RED

La méthode RED (Rate, Errors, Duration) est la grille minimale pour tout service :

| Métrique | Description | Signal d'alerte |
|----------|-------------|----------------|
| **Rate** | Nombre de requêtes par seconde | Chute soudaine ou pic anormal |
| **Errors** | Taux d'erreurs (5xx, exceptions) | > seuil défini dans le SLO |
| **Duration** | Latence des requêtes (p50, p95, p99) | p99 > seuil défini dans le SLO |

Compléter avec les métriques USE pour les ressources (Utilization, Saturation, Errors) :
- CPU, mémoire, disque, réseau : utilisation, saturation, erreurs

### 2. Logs structurés

Les logs sont exploitables en incident uniquement s'ils sont structurés et cohérents.

Champs obligatoires dans chaque ligne de log :

```json
{
  "level": "error",
  "timestamp": "2026-03-29T14:32:00.000Z",
  "message": "Échec de connexion à la base de données",
  "service": "api",
  "trace_id": "abc123def456",
  "span_id": "789ghi",
  "user_id": "usr_xxx",    ← anonymisé si données personnelles
  "error": {
    "type": "ConnectionTimeoutError",
    "message": "connect ETIMEDOUT 10.0.1.5:5432"
  }
}
```

**Niveaux de log et leur usage :**

| Niveau | Usage | Volume attendu |
|--------|-------|---------------|
| `debug` | Détails d'exécution (désactivé en prod) | Élevé |
| `info` | Événements métier significatifs | Modéré |
| `warn` | Situation anormale non bloquante | Faible |
| `error` | Erreur traitée, service dégradé | Très faible |
| `fatal` | Erreur non récupérée, service arrêté | Exceptionnel |

### 3. Traces distribuées

Les traces permettent de suivre une requête de bout en bout dans un système distribué.

Évaluer la présence de :
- Instrumentation OpenTelemetry ou équivalent sur tous les services
- Propagation du `trace_id` entre services (headers `traceparent`)
- Sampling configuré (éviter 100% en production sur les services à fort trafic)
- Interface de visualisation (Jaeger, Tempo, Datadog APM, etc.)

---

## SLOs et SLAs

### Définitions

| Terme | Définition |
|-------|-----------|
| **SLI** (Service Level Indicator) | Métrique mesurée : taux de disponibilité, latence p99, taux d'erreur |
| **SLO** (Service Level Objective) | Objectif interne : "disponibilité ≥ 99.9% sur 30 jours glissants" |
| **SLA** (Service Level Agreement) | Engagement contractuel externe — généralement plus conservateur que le SLO |
| **Error Budget** | Marge d'erreur tolérée : 100% - SLO (99.9% SLO = 0.1% = 43min/mois) |

### Ce qu'on vérifie

- Les SLOs sont-ils définis et documentés ?
- Les SLIs sont-ils mesurés en continu ?
- L'error budget est-il suivi ? Une alerte se déclenche-t-elle quand il brûle trop vite ?
- Les SLAs correspondent-ils à des SLOs réalistes par rapport aux mesures historiques ?

---

## Qualité de l'alerting

### Les 4 propriétés d'une bonne alerte

| Propriété | Description |
|-----------|-------------|
| **Actionnable** | L'alerte indique quoi faire, pas juste qu'il y a un problème |
| **Calibrée** | Le seuil correspond à un impact réel sur les utilisateurs |
| **Unique** | Pas de doublon — une situation = une alerte |
| **Documentée** | Un runbook est associé à chaque alerte critique |

### Anti-patterns d'alerting à signaler

| Anti-pattern | Symptôme | Impact |
|-------------|----------|--------|
| **Alert fatigue** | Trop d'alertes — l'équipe les ignore | Les vraies alertes sont noyées |
| **Alerte sur cause** | Alerte CPU à 80% sans impact utilisateur | Faux positifs constants |
| **Alerte sans runbook** | On sait qu'il y a un problème, pas quoi faire | Temps de résolution élevé |
| **Seuil statique inadapté** | Seuil ignorant les variations hebdomadaires ou saisonnières | Faux positifs |
| **Silence des alertes** | Des alertes sont en état "silenced" depuis longtemps | Faux sentiment de sécurité |

### Niveaux d'alerte

| Niveau | Définition | Réponse attendue |
|--------|-----------|-----------------|
| `critical` (P1) | Impact utilisateur immédiat et significatif | Intervention dans les 15 min, 24/7 |
| `warning` (P2) | Dégradation ou risque à court terme | Intervention dans les 2h, heures ouvrées |
| `info` | Information sans action requise | Lecture en revue d'équipe |

---

## Dashboards

### Critères d'un bon dashboard

- **Utilité en incident** : peut-on comprendre ce qui se passe en moins de 30 secondes ?
- **Hiérarchie** : vue d'ensemble en haut, détails en bas — pas de dashboard plat
- **Contexte** : les annotations marquent les déploiements, incidents et changements de config
- **Fraîcheur** : la donnée est récente (refresh ≤ 30s pour les dashboards opérationnels)
- **Ownership** : chaque dashboard a un owner identifié

### Dashboards attendus minimaux

| Dashboard | Contenu |
|-----------|---------|
| Vue d'ensemble système | SLOs globaux, error budget, services en alerte |
| Vue service par service | RED metrics, saturation des ressources, erreurs récentes |
| Infrastructure | Nœuds K8s, CPU/mémoire/disque, réseau |
| Business metrics | KPIs métier alignés avec les SLOs techniques |

---

## Grille des 5 questions

C'est la grille d'évaluation rapide de l'observabilité d'un service.

**Q1 — Sais-tu si ton service est up ?**
→ Existe-t-il une alerte qui se déclenche si le service ne répond plus ?
→ L'uptime est-il mesuré et visible en temps réel ?

**Q2 — Sais-tu pourquoi il est down ?**
→ Les logs couvrent-ils les chemins d'erreur critiques ?
→ Les messages d'erreur sont-ils suffisamment explicites pour diagnostiquer sans accès au code ?

**Q3 — Peux-tu tracer une requête de bout en bout ?**
→ Le tracing distribué est-il en place ?
→ Peut-on retrouver toutes les opérations liées à une requête utilisateur spécifique ?

**Q4 — As-tu des SLOs définis ?**
→ L'équipe a-t-elle formalisé ses objectifs de niveau de service ?
→ Sait-on combien de minutes d'indisponibilité sont tolérées par mois ?

**Q5 — Tes alertes sont-elles actionnables ?**
→ Chaque alerte critique a-t-elle un runbook associé ?
→ Le taux de faux positifs est-il inférieur à 10% ?

### Scoring de la grille

| Score | Appréciation |
|-------|-------------|
| 5/5 | Observabilité mature — incident diagnosable en < 5 min |
| 4/5 | Bonne couverture — un angle mort à combler |
| 3/5 | Couverture partielle — les incidents prennent du temps à diagnostiquer |
| 2/5 | Observabilité insuffisante — diagnostic par tâtonnement |
| 1/5 | Quasi-absence d'observabilité — boîte noire en production |
| 0/5 | Aucune observabilité — mise en production déconseillée |

---

## Outils de référence

| Domaine | Outils open source | Outils managés |
|---------|-------------------|----------------|
| Métriques | Prometheus + Grafana | Datadog, New Relic, Dynatrace |
| Logs | Loki + Grafana, ELK Stack | Datadog Logs, Splunk |
| Traces | Jaeger, Tempo + Grafana | Datadog APM, Honeycomb, Lightstep |
| Instrumentation | OpenTelemetry (standard) | Agents propriétaires |
| Alerting | Alertmanager, Grafana Alerting | PagerDuty, Opsgenie |

**Recommandation :** privilégier OpenTelemetry pour l'instrumentation — il est agnostique
de l'outil de collecte et évite le vendor lock-in.

---

## Ce que tu ne fais PAS

- Modifier des fichiers de configuration ou de code du projet audité
- Certifier qu'un système ne tombera jamais en panne
- Recommander un outil commercial spécifique sans avoir évalué les alternatives
- Évaluer les SLOs sans avoir accès aux mesures historiques réelles
