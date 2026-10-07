> [Read in English](audit-tools.en.md)

# Référence — Outils d'audit

Commandes et outils utiles pour chaque domaine d'audit.
Ce document est destiné à l'usage humain — les agents d'audit n'exécutent pas ces commandes
(ils font de l'analyse statique du code source uniquement).

---

## Sécurité

### Recherche de secrets dans le code

```bash
# Rechercher des patterns de secrets dans le code
grep -r "password\s*=\s*['\"]" --include="*.{js,ts,php,py}" .
grep -r "AKIA[0-9A-Z]{16}" . # Clés AWS
git log --all --full-history -- "*.env"  # Historique des .env
```

### Analyse des dépendances vulnérables

```bash
# Node.js / npm
npm audit --json

# Node.js / yarn
yarn audit --json

# PHP / Composer
composer audit

# Python / pip
pip-audit

# Ruby / Bundler
bundle audit

# Java / Maven
mvn dependency-check:check

# Rust / Cargo
cargo audit
```

---

## Performance

```bash
# Lighthouse CLI
npx lighthouse https://exemple.com --output=json --quiet

# WebPageTest (API)
curl "https://www.webpagetest.org/runtest.php?url=https://exemple.com&f=json&k=<API_KEY>"

# Bundle analyzer (webpack)
npx webpack-bundle-analyzer stats.json

# Analyse du poids d'une dépendance JS
npx bundlephobia <package-name>

# Profiling requêtes SQL (Laravel)
php artisan telescope  # ou clockwork, debugbar

# EXPLAIN sur une requête SQL lente
EXPLAIN ANALYZE SELECT ...
```

---

## Accessibilité

```bash
# Analyse automatique avec axe-core (CLI)
npx @axe-core/cli https://exemple.com

# Lighthouse accessibility audit
npx lighthouse https://exemple.com --only-categories=accessibility

# Validation HTML W3C
npx html-validate "**/*.html"
```

**Outils en ligne :**
- WAVE : https://wave.webaim.org/
- Vérificateur de contraste WebAIM : https://webaim.org/resources/contrastchecker/

**Lecteurs d'écran pour tests manuels :**
- macOS : VoiceOver (`Cmd+F5`)
- Windows : NVDA (gratuit) ou JAWS
- Linux : Orca

> Les outils automatisés couvrent environ 30-40% des critères WCAG.
> Les tests manuels (navigation clavier, lecteur d'écran) sont indispensables pour une conformité réelle.

---

## Éco-conception

```bash
# Écoindex CLI (empreinte carbone d'une page)
npx ecoindex-cli --url https://exemple.com

# Analyse du bundle
npx bundlesize  # vérifie les limites de taille configurées
npx webpack-bundle-analyzer stats.json

# Audit Lighthouse (performance = proxy éco)
npx lighthouse https://exemple.com --only-categories=performance
```

**Outils en ligne :**
- GreenFrame (analyse continue) : https://greenframe.io/
- CO2.js (calcul d'empreinte dans le code) : `npm install @tgwf/co2`

**Interprétation du score Écoindex :**

| Grade | Score | Émission par page (gCO2e) |
|-------|-------|--------------------------|
| A | 81–100 | ≤ 0.71 |
| B | 61–80 | 0.71–1.06 |
| C | 41–60 | 1.06–1.60 |
| D | 21–40 | 1.60–2.38 |
| E | 1–20 | 2.38–3.57 |
| F | 0 | > 3.57 |

---

## Architecture

```bash
# PHP — complexité et code smells
composer require --dev phpmd/phpmd
vendor/bin/phpmd src text cleancode,codesize,controversial,design,naming,unusedcode

# JavaScript / TypeScript
npx eslint src --max-warnings=0
npx complexity-report --format json src/

# Python — complexité cyclomatique
pip install radon pylint
radon cc src/ -a

# Java
# SonarQube, PMD, Checkstyle

# Duplication de code (tous langages)
npx jscpd --min-lines 10 --reporters json src/
```

---

## Sécurité infra — Checklist RGS (à vérifier manuellement)

Ces points ne peuvent pas être vérifiés par analyse statique. Ils nécessitent un audit
d'infrastructure ou une revue de configuration système.

### Configuration TLS/SSL

- TLS 1.2 minimum — TLS 1.0 et 1.1 désactivés (RGS v2.0 §4.2)
- TLS 1.3 recommandé pour les nouveaux déploiements
- Suites cryptographiques conformes aux recommandations ANSSI
  - Acceptées : `TLS_AES_256_GCM_SHA384`, `TLS_CHACHA20_POLY1305_SHA256`, suites ECDHE
  - Refusées : RC4, DES, 3DES, MD5, SHA-1 sur les signatures
- Perfect Forward Secrecy activée (suites ECDHE ou DHE uniquement)
- Certificat émis par une IGC reconnue (AC ANSSI ou Let's Encrypt pour usages standard)
- Durée de validité du certificat ≤ 1 an recommandé
- OCSP Stapling activé si possible

### Gestion des certificats

- Processus documenté de renouvellement avant expiration
- Alerte de monitoring sur l'expiration (≥ 30 jours à l'avance)
- Révocation possible et testée (CRL ou OCSP disponible)
- Clés privées stockées de façon sécurisée (HSM ou vault sécurisé)

### Cloisonnement réseau

- Serveurs applicatifs non directement accessibles depuis Internet (DMZ)
- Base de données accessible uniquement depuis les serveurs applicatifs
- Ports non nécessaires fermés (firewall applicatif)
- Communications inter-services chiffrées (mTLS en microservices)
- WAF (Web Application Firewall) en place devant les services exposés

### Authentification forte (RGS niveau 1+)

- Authentification des comptes d'administration avec au moins 2 facteurs
- Comptes de service avec secrets rotatifs ou identités managées
- Accès SSH par clé (pas par mot de passe) et journalisés

### Journalisation et traçabilité

- Logs d'accès conservés selon la politique de rétention (RGS : 12 mois minimum)
- Horodatage des logs synchronisé (NTP) et signé si nécessaire
- Logs intègres et non modifiables par les comptes applicatifs
