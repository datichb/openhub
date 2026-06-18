---
name: dev-standards-security
description: Pratiques de sécurité préventives à appliquer pendant le développement — secrets, validation des inputs, injections, auth/autorisation, logs, dépendances.
---

# Skill — Standards de Sécurité Développement

## Rôle

Ce skill définit les pratiques de sécurité à respecter **pendant le développement**.
Il s'applique à tous les agents développeurs et au reviewer comme critère de qualité.

Ce skill ne remplace pas un audit de sécurité approfondi.
Pour un audit exhaustif, utiliser l'agent `auditor` (domaine security) (OWASP Top 10, CVE, RGS).

---

## Secrets et configuration

- Les secrets (clés API, tokens, mots de passe, DSN) sont **toujours** dans des variables d'environnement — jamais en dur dans le code
- `.env` et tout fichier contenant des secrets sont dans `.gitignore`
- Les valeurs par défaut dans le code ne doivent jamais être des secrets de production
- Un secret commité par erreur est révoqué immédiatement — changer le fichier ne suffit pas

❌ `const API_KEY = "sk-prod-abc123"`
✅ `const API_KEY = env("API_KEY")` — lire depuis les variables d'environnement

---

## Validation des inputs

- Valider à l'entrée de **chaque couche** : controller ET service (défense en profondeur)
- Utiliser des DTOs typés en entrée — pas de passage de `req.body` brut aux couches internes
- Rejeter les données invalides explicitement — ne pas tenter de les assainir silencieusement
- Valider le type, le format, la longueur et la plage de valeurs selon le contexte métier
- Les entrées venant de l'extérieur (API, formulaires, fichiers uploadés, webhooks) sont toutes non fiables par défaut

---

## Injections

- **SQL** : requêtes paramétrées ou ORM systématiquement — jamais de concaténation de chaînes avec des données utilisateur
- **Shell** : pas d'interpolation directe de variables utilisateur dans les commandes shell
- **LDAP / XPath / NoSQL** : même principe — paramétrer ou échapper selon le moteur
- **Templates** : utiliser les moteurs de templates avec échappement automatique — ne jamais interpoler du HTML brut depuis des données utilisateur

❌ `db.query("SELECT * FROM users WHERE id = " + userId)`
✅ `db.query("SELECT * FROM users WHERE id = ?", [userId])`

---

## Authentification et autorisations

- Vérifier les droits côté **serveur** à chaque appel — jamais se fier uniquement à la logique côté client
- Appliquer le principe du moindre privilège : une action n'accède qu'aux ressources dont elle a besoin
- Les tokens d'accès ont une expiration explicite
- Les endpoints sensibles (administration, données personnelles) ont un contrôle d'accès explicite — pas de "ce sera fait plus tard"
- Distinguer authentification (qui est-tu ?) et autorisation (as-tu le droit ?)

---

## Logs et gestion des erreurs

- Ne jamais logger de données sensibles : mots de passe, tokens, clés API, données personnelles (PII)
- Les messages d'erreur renvoyés au client sont génériques — les détails techniques (stack trace, nom de table, requête SQL) restent côté serveur
- Distinguer erreurs métier (400, 422) et erreurs techniques (500) dans les réponses
- Logger suffisamment pour diagnostiquer un incident sans exposer de données sensibles

❌ `logger.info("Connexion de " + user.password)` — ne jamais logger un mot de passe
✅ `logger.info("Connexion de l'utilisateur", { userId: user.id })`

---

## Dépendances

- Auditer les dépendances avec l'outil fourni par le gestionnaire de paquets du projet avant d'en ajouter une nouvelle
- Les versions sont épinglées dans le lockfile — pas de wildcard `*` ou `latest` en production
- Préférer les dépendances maintenues activement avec une communauté documentée
- Une dépendance avec des vulnérabilités critiques non corrigées est un bloquant

---

## Ce que ce skill ne remplace pas

Ce skill couvre les pratiques préventives de développement courant.
Pour un audit de sécurité exhaustif (OWASP Top 10, CVE, analyse de flux de données,
tests d'intrusion, revue CORS/CSP/headers), utiliser l'agent `auditor` (domaine security).
