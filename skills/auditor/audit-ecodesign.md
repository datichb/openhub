---
name: audit-ecodesign
description: "Utiliser quand l'audit porte sur l'éco-conception : RGESN, GreenIT, sobriété, poids et requêtes inutiles."
---

# Skill — Audit Éco-conception

## Référentiels couverts

- **RGESN** (Référentiel Général d'Écoconception des Services Numériques) — DINUM, 2022
- **GreenIT** — bonnes pratiques green-patterns (79 bonnes pratiques)
- **Numérique Responsable** — INR (Institut du Numérique Responsable)
- **Web Sustainability Guidelines** (WSG 1.0) — W3C Community Group
- **Écoindex** — score environnemental des pages web (basé sur DOM, requêtes, poids)

---

## Principes fondamentaux

L'éco-conception numérique vise à **réduire les ressources consommées** tout au long du cycle de vie d'un service :
- Ressources matérielles (CPU, RAM, réseau, stockage)
- Ressources énergétiques (consommation électrique)
- Ressources humaines (charge cognitive, temps utilisateur)

Les optimisations d'éco-conception recoupent souvent les optimisations de performance.
**La sobriété fonctionnelle** (faire moins) est préférée à l'**optimisation technique** (faire mieux).

---

## Checklist — Stratégie et contenu

### Pertinence des fonctionnalités

- [ ] Les fonctionnalités sont justifiées par un besoin utilisateur réel (pas de feature par défaut)
- [ ] Les fonctionnalités peu utilisées sont identifiées et candidates à la suppression
- [ ] Le contenu est à jour — absence de pages zombies jamais visitées
- [ ] Les données collectées sont strictement nécessaires au service (minimisation aussi bonne pour l'éco-conception)

### Contenu multimédia

- [ ] Les vidéos sont hébergées en externe (YouTube, Vimeo) plutôt que sur le serveur applicatif
- [ ] La lecture automatique des vidéos est désactivée par défaut (`autoplay` absent)
- [ ] Les vidéos sont compressées avec des codecs modernes (H.265, AV1, VP9)
- [ ] Les animations sont pertinentes et non décoratives uniquement
- [ ] L'utilisateur peut désactiver les animations (`prefers-reduced-motion` respecté)

---

## Checklist — Ressources front-end

### Images

- [ ] Les images sont redimensionnées à la taille d'affichage (pas d'image 2000px affichée en 200px)
- [ ] Les images utilisent des formats optimisés (WebP, AVIF) — moins de bande passante
- [ ] Le lazy loading est implémenté pour les images hors viewport
- [ ] Les images vectorielles utilisent SVG plutôt que des PNG/JPG pour les icônes
- [ ] Un sprite SVG est utilisé pour les collections d'icônes répétées

### Polices

- [ ] Le nombre de variantes de polices chargées est limité (poids, styles)
- [ ] Les polices système sont préférées aux polices web quand le design le permet
- [ ] Les polices web sont en sous-ensemble (subsetting) pour ne charger que les caractères utilisés
- [ ] `font-display: swap` évite le blocage de rendu (aussi un gain UX)

### JavaScript

- [ ] Le JavaScript est minifié et tree-shaké (dead code éliminé)
- [ ] Les dépendances lourdes inutilisées sont supprimées
- [ ] Les scripts tiers (analytics, chat, publicité) sont évalués pour leur impact
  - Chaque script tiers = requête réseau + CPU + RAM
  - Auditer si leur présence est justifiée et s'ils peuvent être chargés à la demande
- [ ] Le code côté client est réduit au minimum (pas de logique qui devrait être côté serveur)

### CSS

- [ ] Le CSS inutilisé est purgé en production
- [ ] Les animations CSS sont préférées aux animations JavaScript (GPU plutôt que CPU)
- [ ] `will-change` n'est pas utilisé de façon abusive (surcoût mémoire)

---

## Checklist — Architecture et serveur

### Requêtes réseau

- [ ] Le nombre de requêtes HTTP par page est minimisé
  - Cible : ≤ 40 requêtes pour la charge initiale (Écoindex)
- [ ] Les ressources sont regroupées (bundling), mais le code splitting évite les gros chunks
- [ ] Les polices, scripts et styles critiques sont préchargés (`preload`) pour éviter les waterfalls
- [ ] HTTP/2 ou HTTP/3 est utilisé pour le multiplexage des requêtes

### Poids des pages

- [ ] Le poids total de la page est mesuré et limité
  - Cible Écoindex : ≤ 500KB (poids transféré, toutes ressources confondues)
  - Attention : cette cible varie selon le type de service
- [ ] Les ressources sont compressées (gzip ou brotli)

### Cache

- [ ] Les ressources statiques ont des headers de cache longs (immutables si versionnées)
- [ ] Un CDN est utilisé pour éviter les allers-retours vers le serveur d'origine
- [ ] Le cache applicatif (Redis, Memcached) réduit les traitements répétitifs côté serveur

### Base de données

- [ ] Les requêtes sont optimisées pour minimiser les lectures/écritures inutiles
- [ ] Les données sont archivées ou supprimées selon une politique de rétention (pas de stockage illimité)
- [ ] Les exports et rapports lourds sont générés en différé (pas en temps réel si non nécessaire)

### Hébergement

- [ ] L'hébergeur utilise de l'énergie renouvelable ou a un engagement bas carbone documenté
- [ ] Les serveurs sont dimensionnés au besoin réel (pas de sur-provisionnement permanent)
- [ ] L'auto-scaling est configuré pour éteindre les ressources inutilisées
- [ ] Les environnements de dev/staging sont éteints hors des heures de travail si possible

---

## Checklist — Éléments DOM

Le score Écoindex est en partie basé sur la complexité du DOM :

- [ ] Le nombre d'éléments DOM par page est limité
  - Cible : ≤ 1500 éléments DOM (Écoindex)
- [ ] Les listes longues utilisent la virtualisation (seuls les éléments visibles sont dans le DOM)
- [ ] Les composants hors viewport ne sont pas rendus avant interaction (lazy rendering)
- [ ] Les arbres DOM profonds inutilement imbriqués sont aplatis

---

## Checklist — Expérience utilisateur et sobriété

### Efficacité des parcours

- [ ] Le nombre de clics/étapes pour accomplir une tâche est minimisé
- [ ] Les formulaires sont pré-remplis quand l'information est déjà connue
- [ ] La recherche fonctionne efficacement (résultats pertinents dès la 1ère requête)
- [ ] Les erreurs utilisateur génèrent des messages utiles pour éviter les retry inutiles

### Notifications et mises à jour

- [ ] Les notifications push sont opt-in et limitées au strict nécessaire
- [ ] Les mises à jour de données en temps réel sont remplacées par du pull à la demande quand possible
- [ ] Le polling en background est désactivé quand l'onglet est inactif (`visibilitychange`)

---

## Checklist — RGESN (thématiques)

### Thématique 1 — Stratégie et pilotage

- [ ] Une démarche d'éco-conception est documentée et suivie dans le projet
- [ ] L'impact environnemental du service est mesuré (Écoindex, CO2.js, ou équivalent)
- [ ] Les objectifs de réduction sont définis et suivis dans le temps

### Thématique 2 — Spécifications

- [ ] Les spécifications intègrent des critères d'éco-conception (ex: limite de poids de page)
- [ ] Les fonctionnalités à fort impact sont identifiées en amont

### Thématique 3 — Architecture

- [ ] L'architecture minimise les échanges de données entre services
- [ ] Le choix de l'infrastructure prend en compte l'efficacité énergétique

### Thématique 4 — UX/UI

- [ ] Les gabarits de pages sont conçus pour être légers par défaut
- [ ] Le mode sombre est disponible (économie d'énergie sur écrans OLED)
- [ ] Les contenus riches (cartes, animations) sont chargés à la demande

### Thématique 5 — Contenus

- [ ] Les images sont optimisées avant intégration (compression, format, dimensions)
- [ ] Les vidéos sont hébergées sur des plateformes spécialisées plutôt qu'en auto-hébergement lourd

### Thématique 6 — Front-end

- [ ] Le CSS est optimisé (purge, minification)
- [ ] Le JavaScript est minimisé (bundle, tree-shaking)
- [ ] Le cache navigateur est configuré correctement

### Thématique 7 — Back-end

- [ ] Les traitements inutiles sont éliminés (requêtes, calculs, transformations)
- [ ] Les tâches asynchrones sont préférées pour les traitements lourds

### Thématique 8 — Hébergement

- [ ] La PUE (Power Usage Effectiveness) du datacenter est connue (cible : ≤ 1.5)
- [ ] L'hébergeur a une politique d'énergie renouvelable documentée

---

> Les outils de mesure (Écoindex CLI, GreenFrame, CO2.js, webpack-bundle-analyzer, Lighthouse)
> sont référencés dans `docs/reference/audit-tools.md` pour usage humain.

## Ce que tu ne fais PAS dans ce domaine

- Mesurer l'empreinte carbone réelle sans données de trafic
- Imposer des contraintes qui dégradent l'expérience utilisateur sans justification
- Recommander de supprimer des fonctionnalités utiles au nom de la sobriété sans analyse préalable
- Confondre éco-conception et optimisation de performance — les objectifs se recoupent mais ne sont pas identiques
