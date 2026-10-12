> 🇫🇷 [Lire en français](audit-tools.fr.md)

# Reference — Audit Tools

Useful commands and tools for each audit domain.
This document is intended for human use — audit agents do not execute these commands
(they perform static analysis of source code only), except the IaC analysis tools of the
`infra` domain, which the `auditor-subagent` may run after your confirmation.

---

## Security

### Searching for secrets in code

```bash
# Search for secret patterns in code
grep -r "password\s*=\s*['\"]" --include="*.{js,ts,php,py}" .
grep -r "AKIA[0-9A-Z]{16}" . # AWS keys
git log --all --full-history -- "*.env"  # .env history
```

### Vulnerable dependency analysis

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
npx lighthouse https://example.com --output=json --quiet

# WebPageTest (API)
curl "https://www.webpagetest.org/runtest.php?url=https://example.com&f=json&k=<API_KEY>"

# Bundle analyzer (webpack)
npx webpack-bundle-analyzer stats.json

# Analyse the weight of a JS dependency
npx bundlephobia <package-name>

# SQL query profiling (Laravel)
php artisan telescope  # or clockwork, debugbar

# EXPLAIN on a slow SQL query
EXPLAIN ANALYZE SELECT ...
```

---

## Accessibility

```bash
# Automated analysis with axe-core (CLI)
npx @axe-core/cli https://example.com

# Lighthouse accessibility audit
npx lighthouse https://example.com --only-categories=accessibility

# W3C HTML validation
npx html-validate "**/*.html"
```

**Online tools:**
- WAVE: https://wave.webaim.org/
- WebAIM Contrast Checker: https://webaim.org/resources/contrastchecker/

**Screen readers for manual testing:**
- macOS: VoiceOver (`Cmd+F5`)
- Windows: NVDA (free) or JAWS
- Linux: Orca

> Automated tools cover approximately 30-40% of WCAG criteria.
> Manual testing (keyboard navigation, screen reader) is essential for genuine compliance.

---

## Ecodesign

```bash
# Écoindex CLI (carbon footprint of a page)
npx ecoindex-cli --url https://example.com

# Bundle analysis
npx bundlesize  # checks configured size limits
npx webpack-bundle-analyzer stats.json

# Lighthouse audit (performance = eco proxy)
npx lighthouse https://example.com --only-categories=performance
```

**Online tools:**
- GreenFrame (continuous analysis): https://greenframe.io/
- CO2.js (footprint calculation in code): `npm install @tgwf/co2`

**Écoindex score interpretation:**

| Grade | Score | Emission per page (gCO2e) |
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
# PHP — complexity and code smells
composer require --dev phpmd/phpmd
vendor/bin/phpmd src text cleancode,codesize,controversial,design,naming,unusedcode

# JavaScript / TypeScript
npx eslint src --max-warnings=0
npx complexity-report --format json src/

# Python — cyclomatic complexity
pip install radon pylint
radon cc src/ -a

# Java
# SonarQube, PMD, Checkstyle

# Code duplication (all languages)
npx jscpd --min-lines 10 --reporters json src/
```

---

## Infrastructure as code (`infra` domain)

```bash
terraform fmt -check -recursive
terraform init -backend=false -lockfile=readonly && terraform validate
tflint --recursive
tfsec .                      # or: trivy config .
checkov -d . --compact
kube-score score k8s/*.yaml
kubesec scan k8s/deployment.yaml
helm lint ./chart && helm template ./chart
infracost breakdown --path infra/
terraform plan -input=false -detailed-exitcode   # drift (exit code 2), read-only credentials
kubectl diff -f k8s/                              # Kubernetes drift (run by you)
```

## Database (`database` domain)

```sql
-- PostgreSQL: roles, privileges and RLS (run by you, results pasted to the session)
SELECT rolname, rolsuper, rolbypassrls FROM pg_roles;
SELECT grantee, table_name, privilege_type FROM information_schema.role_table_grants WHERE grantee = 'PUBLIC';
SELECT relname, relrowsecurity, relforcerowsecurity FROM pg_class WHERE relkind = 'r' AND relnamespace = 'public'::regnamespace;
SHOW ssl;
```

---

## Infrastructure security — RGS checklist (to verify manually)

These points cannot be verified by static analysis. They require an infrastructure
audit or a system configuration review.

### TLS/SSL configuration

- TLS 1.2 minimum — TLS 1.0 and 1.1 disabled (RGS v2.0 §4.2)
- TLS 1.3 recommended for new deployments
- Cryptographic suites compliant with ANSSI recommendations
  - Accepted: `TLS_AES_256_GCM_SHA384`, `TLS_CHACHA20_POLY1305_SHA256`, ECDHE suites
  - Refused: RC4, DES, 3DES, MD5, SHA-1 on signatures
- Perfect Forward Secrecy enabled (ECDHE or DHE suites only)
- Certificate issued by a recognised CA (ANSSI CA or Let's Encrypt for standard usage)
- Certificate validity duration ≤ 1 year recommended
- OCSP Stapling enabled if possible

### Certificate management

- Documented renewal process before expiry
- Monitoring alert on expiry (≥ 30 days in advance)
- Revocation possible and tested (CRL or OCSP available)
- Private keys stored securely (HSM or secure vault)

### Network segmentation

- Application servers not directly accessible from the Internet (DMZ)
- Database accessible only from application servers
- Unnecessary ports closed (application firewall)
- Inter-service communications encrypted (mTLS in microservices)
- WAF (Web Application Firewall) in place in front of exposed services

### Strong authentication (RGS level 1+)

- Administration account authentication with at least 2 factors
- Service accounts with rotating secrets or managed identities
- SSH access by key (not password) and logged

### Logging and traceability

- Access logs retained according to retention policy (RGS: 12 months minimum)
- Log timestamps synchronised (NTP) and signed if necessary
- Logs immutable and not modifiable by application accounts
