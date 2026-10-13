---
name: audit-privacy
description: "Utiliser quand l'audit porte sur les données personnelles : RGPD, EDPB, minimisation, consentement, droits, PIA."
---

# Skill — Audit Privacy (RGPD)

## Référentiels couverts

- **RGPD** (Règlement Général sur la Protection des Données) — UE 2016/679
- **Loi Informatique et Libertés** (modifiée 2018) — transposition française
- **Lignes directrices EDPB** (European Data Protection Board)
- **Référentiels CNIL** — cookies, durées de conservation, sous-traitants
- **ISO 29101** — Privacy Architecture Framework

---

## Bases légales du traitement (Art. 6 RGPD)

Chaque traitement de données personnelles doit reposer sur **une et une seule** base légale :

| Base légale | Code | Usage typique |
|------------|------|---------------|
| Consentement | 6a | Newsletter, cookies non essentiels, tracking |
| Exécution d'un contrat | 6b | Commande en ligne, compte utilisateur |
| Obligation légale | 6c | Facturation, lutte contre la fraude |
| Sauvegarde des intérêts vitaux | 6d | Urgence médicale |
| Mission d'intérêt public | 6e | Service public, recherche |
| Intérêt légitime | 6f | Analytics anonymisés, sécurité du service |

---

## Checklist — Collecte et minimisation des données (Art. 5)

### Minimisation

- [ ] Seules les données strictement nécessaires à la finalité sont collectées
- [ ] Les champs de formulaire optionnels sont clairement identifiés comme tels
- [ ] Les données de navigation collectées sont limitées (logs, analytics)
- [ ] Les exports et sauvegardes ne contiennent pas de données non nécessaires au backup

### Finalité

- [ ] Chaque traitement a une finalité explicite et documentée
- [ ] Les données ne sont pas réutilisées pour d'autres finalités sans base légale supplémentaire
- [ ] La finalité déclarée dans les CGU/mentions légales correspond à l'utilisation réelle

### Exactitude

- [ ] Un mécanisme permet aux utilisateurs de corriger leurs données
- [ ] Les données obsolètes sont supprimées ou mises à jour

---

## Checklist — Consentement (Art. 7 + lignes directrices EDPB)

### Qualité du consentement

- [ ] Le consentement est **libre** : pas de couplage avec un service (consentir ou ne pas accéder)
- [ ] Le consentement est **spécifique** : une finalité = un consentement distinct
- [ ] Le consentement est **éclairé** : l'utilisateur sait précisément à quoi il consent
- [ ] Le consentement est **univoque** : action positive (pas de case pré-cochée)
- [ ] Le retrait du consentement est aussi simple que son octroi

### Preuve du consentement

- [ ] La date, l'heure et la version de la politique au moment du consentement sont enregistrées
- [ ] Le consentement peut être prouvé pour chaque utilisateur (journaux de consentement)
- [ ] Le mécanisme de consentement est auditable

### Cookies et traceurs (CNIL — lignes directrices 2020)

- [ ] Un bandeau de consentement aux cookies est présent et conforme
  - Refuser doit être aussi simple qu'accepter
  - Pas de "dark patterns" (bouton refuser caché, design trompeur)
- [ ] Les cookies non essentiels ne sont pas déposés avant le consentement
- [ ] La liste des cookies est exhaustive et à jour
- [ ] Les cookies de session et cookies strictement nécessaires sont exemptés de consentement
- [ ] La durée de validité du consentement est ≤ 13 mois (CNIL)

---

## Checklist — Droits des personnes (Art. 12–23)

### Droit d'accès (Art. 15)

- [ ] Un mécanisme permet aux utilisateurs de consulter leurs données personnelles
- [ ] La réponse est fournie dans un délai de 30 jours
- [ ] Les données accessibles incluent : catégories, finalités, destinataires, durée de conservation

### Droit à l'effacement (Art. 17)

- [ ] Un mécanisme permet aux utilisateurs de demander la suppression de leurs données
- [ ] La suppression est effective dans les systèmes principaux ET les sauvegardes (avec délai documenté)
- [ ] Les exceptions légales à l'effacement sont documentées

### Droit à la portabilité (Art. 20)

- [ ] Les données fournies par l'utilisateur peuvent être exportées dans un format standard (JSON, CSV)
- [ ] L'export couvre uniquement les données fournies activement par l'utilisateur

### Droit de rectification (Art. 16)

- [ ] L'utilisateur peut modifier ses données depuis son espace personnel
- [ ] Les données rectifiées sont propagées aux sous-traitants concernés

### Droit d'opposition (Art. 21)

- [ ] L'utilisateur peut s'opposer au traitement basé sur l'intérêt légitime
- [ ] Le mécanisme d'opposition est accessible et documenté

### Droit à la limitation du traitement (Art. 18)

- [ ] Un mécanisme permet de "geler" un traitement le temps d'un litige ou d'une vérification

---

## Checklist — Sécurité des données (Art. 25 + 32)

### Privacy by Design et by Default

- [ ] Le niveau de confidentialité le plus élevé est le paramètre par défaut
- [ ] Les données personnelles ne sont pas accessibles par défaut aux autres utilisateurs
- [ ] Les nouvelles fonctionnalités font l'objet d'une évaluation privacy avant déploiement

### Mesures techniques

- [ ] Les données personnelles sensibles sont chiffrées au repos (santé, orientation sexuelle, etc.)
- [ ] L'accès aux données personnelles est journalisé (qui a accédé à quoi et quand)
- [ ] Les accès aux données sont basés sur le besoin d'en connaître (RBAC)
- [ ] Les données de test ne contiennent pas de données personnelles réelles (ou sont anonymisées)
- [ ] Les sauvegardes sont chiffrées et l'accès est contrôlé

### Pseudonymisation et anonymisation

- [ ] La pseudonymisation est utilisée quand la réidentification directe n'est pas nécessaire
- [ ] L'anonymisation est irréversible si elle est déclarée comme telle (pas seulement suppression du nom)
  - L'anonymisation nécessite : généralisation, suppression et/ou bruit sur les données

---

## Checklist — Sous-traitants (Art. 28)

- [ ] Chaque sous-traitant traitant des données personnelles a un DPA (Data Processing Agreement) signé
- [ ] La liste des sous-traitants est maintenue et à jour
- [ ] Les sous-traitants sont localisés dans l'UE ou dans un pays reconnu adéquat
  - Si hors UE/EEE : clauses contractuelles types (CCT) ou BCR en place
- [ ] Les transferts vers les USA utilisent le Data Privacy Framework ou des CCT à jour
- [ ] Les sous-traitants ne peuvent pas sous-traiter sans autorisation préalable

---

## Checklist — Durées de conservation (Art. 5.1.e)

- [ ] Chaque catégorie de données a une durée de conservation documentée
- [ ] Un mécanisme automatique de suppression/archivage est en place
- [ ] Les durées légales obligatoires sont respectées (ex: factures = 10 ans, logs de connexion = 1 an LCEN)
- [ ] Les données anonymisées peuvent être conservées sans limite (si l'anonymisation est réelle)

**Durées de référence (France) :**

| Données | Durée |
|---------|-------|
| Logs de connexion (LCEN) | 1 an |
| Factures et pièces comptables | 10 ans |
| Données de prospection B2B | 3 ans après dernier contact |
| Données de prospects B2C | 3 ans après collecte ou dernier contact |
| Données de clients | Durée du contrat + 5 ans (prescription) |
| Données RH (candidatures non retenues) | 2 ans |
| Vidéosurveillance | 30 jours maximum |

---

## Checklist — Documentation et registre (Art. 30)

- [ ] Un registre des traitements est tenu à jour
- [ ] Le registre contient : finalité, catégories de données, destinataires, durées, mesures de sécurité
- [ ] Les mentions légales / politique de confidentialité sont à jour et accessibles
- [ ] Un DPO (Délégué à la Protection des Données) est désigné si obligatoire

**Obligation DPO :** organisme public, traitements à grande échelle de données sensibles, ou surveillance systématique à grande échelle.

---

## Checklist — Analyse d'impact (PIA / AIPD) (Art. 35)

Un PIA (Privacy Impact Assessment) est **obligatoire** si le traitement est "susceptible d'engendrer un risque élevé" :

**Critères déclencheurs (≥ 2 critères = PIA obligatoire) :**
- Évaluation ou scoring de personnes physiques
- Décision automatisée avec effet juridique
- Surveillance systématique de personnes
- Données sensibles ou à caractère hautement personnel (santé, biométrie, etc.)
- Données de personnes vulnérables (mineurs, patients)
- Traitement innovant ou utilisation de nouvelles technologies
- Transfert de données hors UE
- Fusion ou combinaison de datasets

**Le PIA doit contenir :**
- [ ] Description systématique du traitement et de sa finalité
- [ ] Évaluation de la nécessité et de la proportionnalité
- [ ] Évaluation des risques pour les droits et libertés des personnes
- [ ] Mesures envisagées pour faire face aux risques

---

## Checklist — Données spéciales (Art. 9)

Ces catégories requièrent une base légale spécifique renforcée :

- [ ] Données de santé : base légale Art. 9.2 applicable et documentée
- [ ] Données biométriques (empreintes, reconnaissance faciale) : base légale spécifique
- [ ] Données génétiques : traitement exceptionnellement justifié
- [ ] Opinions politiques, religieuses, philosophiques : traitement explicitement autorisé
- [ ] Orientation sexuelle : traitement minimal et sécurisé
- [ ] Données d'infractions pénales : traitement sous autorité de l'État uniquement

---

## Checklist — Violations de données (Art. 33-34)

- [ ] Un processus de détection et de réponse aux violations est documenté
- [ ] Les violations sont notifiées à la CNIL dans les 72h (si risque pour les personnes)
- [ ] Les personnes concernées sont notifiées si le risque est élevé
- [ ] Un registre des violations est tenu même pour les violations non notifiées

---

## Ce que tu ne fais PAS dans ce domaine

- Fournir un avis juridique ou une certification RGPD — recommander une consultation DPO ou juriste
- Accéder à des données personnelles réelles pour les analyser
- Vérifier la conformité des contrats de sous-traitance (analyse documentaire, pas code)
- Déclarer un traitement conforme sur la base d'une analyse de code uniquement — la conformité RGPD est organisationnelle et technique
