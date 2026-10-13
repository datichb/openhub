---
name: audit-security
description: "Utiliser quand l'audit porte sur la sécurité applicative : OWASP Top 10, CVE des dépendances, secrets, headers HTTP, RGS."
---

# Skill — Audit Sécurité

## Référentiels couverts

- **OWASP Top 10** (2021) — vulnérabilités applicatives les plus critiques
- **OWASP ASVS** (Application Security Verification Standard) — niveau 1 et 2
- **RGS** (Référentiel Général de Sécurité) — exigences ANSSI pour les systèmes d'État
- **CVE / NVD** — vulnérabilités connues dans les dépendances
- **OWASP Dependency-Check** — analyse des dépendances tierces

---

## Checklist — OWASP Top 10 (2021)

### A01 — Broken Access Control

- [ ] Les routes protégées vérifient l'authentification ET les autorisations
- [ ] Absence de référence directe à des objets non contrôlée (IDOR)
  - Exemple : `/api/users/42` accessible sans vérifier que l'appelant possède le compte 42
- [ ] Les rôles sont vérifiés côté serveur (jamais uniquement côté client)
- [ ] Les opérations sensibles (suppression, export) nécessitent une confirmation ou un droit élevé
- [ ] Les répertoires ne sont pas listables (directory listing désactivé)
- [ ] Les fichiers uploadés ne sont pas accessibles depuis une URL prévisible sans contrôle d'accès

### A02 — Cryptographic Failures

- [ ] Aucune donnée sensible en clair dans les logs (mots de passe, tokens, PAN)
- [ ] Les mots de passe sont hashés avec un algorithme adapté (bcrypt, argon2, scrypt)
- [ ] Les tokens JWT utilisent un algorithme sûr (RS256, ES256 — pas `none`, pas HS256 avec secret faible)
- [ ] Les données sensibles au repos sont chiffrées (base de données, backups)
- [ ] TLS est imposé (pas de fallback HTTP en production)
- [ ] Les secrets ne sont jamais versionnés dans le code source

### A03 — Injection

- [ ] Toutes les requêtes SQL utilisent des paramètres liés (pas de concaténation de chaînes)
- [ ] Les entrées utilisateur sont validées et assainies avant usage dans les commandes shell
- [ ] Les templates (Twig, Blade, Jinja2...) échappent les variables par défaut
- [ ] Les requêtes LDAP, XPath, NoSQL utilisent des opérateurs sécurisés
- [ ] Absence d'`eval()` sur des données utilisateur

### A04 — Insecure Design

- [ ] Les flux métier critiques ont une logique de limitation (rate limiting, anti-brute-force)
- [ ] Les fonctions de réinitialisation de mot de passe utilisent des tokens à usage unique et expirés
- [ ] Les multi-étapes (checkout, paiement) ne peuvent pas être court-circuitées par manipulation d'état

### A05 — Security Misconfiguration

- [ ] Les messages d'erreur ne divulguent pas de stack traces, chemins internes, ou noms de tables
- [ ] Les endpoints de debug/admin ne sont pas exposés en production
- [ ] Les valeurs par défaut (admin/admin, clés d'exemple) ont été changées
- [ ] Les headers de sécurité sont présents (voir section Headers HTTP)
- [ ] CORS est configuré avec une allowlist explicite, pas `*`

### A06 — Vulnerable and Outdated Components

- [ ] Les dépendances directes sont à jour (lire les lockfiles et rechercher les CVE ; `npm audit`, `composer audit`… sont à recommander à l'utilisateur, pas à lancer)
- [ ] Absence de dépendances avec CVE critique connue non corrigée
- [ ] Les versions des runtimes (Node.js, PHP, Python...) sont supportées (pas en EOL)
- [ ] Un outil d'analyse des dépendances est intégré en CI (Dependabot, Snyk, etc.)

### A07 — Identification and Authentication Failures

- [ ] Les sessions expirent après inactivité
- [ ] Les tokens de session sont invalidés à la déconnexion côté serveur
- [ ] La réinitialisation de mot de passe ne révèle pas si un email est enregistré (timing attack)
- [ ] Le MFA est disponible pour les comptes à privilèges élevés
- [ ] Les tentatives de connexion échouées déclenchent un verrouillage progressif

### A08 — Software and Data Integrity Failures

- [ ] Les pipelines CI/CD vérifient l'intégrité des artefacts (checksums, signatures)
- [ ] Les dépendances sont épinglées à des versions exactes (lockfiles versionnés)
- [ ] Les mises à jour automatiques de dépendances passent par une review (Dependabot PR)
- [ ] Absence de désérialisation non sécurisée de données non fiables

### A09 — Security Logging and Monitoring Failures

- [ ] Les événements de sécurité sont loggés (tentatives d'auth, accès refusés, erreurs 5xx)
- [ ] Les logs ne contiennent pas de données sensibles (mots de passe, tokens, données perso)
- [ ] Les logs sont centralisés et conservés (pas uniquement sur le serveur applicatif)
- [ ] Des alertes sont configurées sur des patterns suspects

### A10 — Server-Side Request Forgery (SSRF)

- [ ] Les URLs fournies par l'utilisateur ne sont pas utilisées pour des requêtes serveur sans validation
- [ ] Une allowlist de domaines/IPs autorisés est appliquée si des requêtes sortantes sont nécessaires
- [ ] L'accès aux métadonnées cloud (169.254.169.254) est bloqué si non nécessaire

---

## Checklist — Headers HTTP de sécurité

Vérifier la présence et la configuration de ces headers dans les réponses HTTP :

| Header | Valeur recommandée | Impact |
|--------|-------------------|--------|
| `Content-Security-Policy` | Politique explicite, pas `unsafe-inline` | XSS |
| `X-Content-Type-Options` | `nosniff` | MIME sniffing |
| `X-Frame-Options` | `DENY` ou `SAMEORIGIN` | Clickjacking |
| `Strict-Transport-Security` | `max-age=31536000; includeSubDomains` | Downgrade HTTPS |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | Fuite d'URL |
| `Permissions-Policy` | Restriction des APIs navigateur non utilisées | Fingerprinting |
| `Cache-Control` | `no-store` sur les pages authentifiées | Cache poisoning |

---

## Checklist — Secrets et credentials dans le code

Patterns à rechercher dans le code source et l'historique git :

- Chaînes de type `password`, `secret`, `api_key`, `token`, `private_key` assignées en dur
- Clés AWS (`AKIA...`), GCP, Azure dans le code ou les configs
- Tokens GitHub, GitLab, NPM, Docker Hub
- Chaînes de connexion de base de données avec identifiants en dur
- Certificats ou clés privées (PEM) committées
- Fichiers `.env` non exclus du suivi git

> Les patterns de recherche (`grep`, `git log`) sont référencés dans `docs/reference/audit-tools.md`.

---

## Analyse des dépendances

Signaler toute dépendance avec :
- CVE de sévérité **Critical** ou **High** non corrigée
- Dépendance abandonnée (dernière release > 2 ans, pas de mainteneur actif)
- Dépendance non épinglée à une version exacte (risque de supply chain)

> Les commandes d'analyse (`npm audit`, `composer audit`, `pip-audit`, etc.) sont
> référencées dans `docs/reference/audit-tools.md` pour usage humain.

---

## Ce que tu ne fais PAS dans ce domaine

- Accéder à des services live ou tester en conditions réelles (pas de pentest)
- Modifier des fichiers de configuration de sécurité
- Générer ou exposer des secrets, même à titre d'exemple avec des valeurs réelles
- Conclure qu'une application est "sécurisée" — l'audit statique a des limites
