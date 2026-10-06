> [Read in English](044-credential-proxy-session-limits.en.md)

# ADR-044 — Proxy d'identifiants LLM, jetons de session et restrictions I6

## Statut

Accepté

## Date

2026-10-05

## Contexte

Jusqu'à la v4, oh passait la clé du fournisseur LLM à opencode, dans l'environnement ou dans `opencode.json` ([ADR-021](./021-provider-transparent-validation.fr.md)). Un `env` lancé par un agent affichait donc la vraie clé.

Avec la v5, d'autres contraintes s'ajoutent :

- le dossier de données isolé par groupe rend invisibles les identifiants enregistrés par opencode (F14) ;
- Bedrock exige une région (F15) ;
- un modèle indisponible faisait **basculer opencode sur ses modèles gratuits hébergés** : le prompt partait chez un autre fournisseur, sans prévenir (constaté en phase 0) ;
- les conteneurs et les jobs distants ne doivent recevoir aucun secret de la machine (D10) ;
- les restrictions d'usage (I6 : sessions actives, budgets, mémoire) demandaient un point de comptage.

Décision O4 : un proxy d'identifiants intégré à `oh` dès la phase 0.

## Décision

### 1. Proxy et jetons

- `internal/credproxy`, hébergé par le démon `ohd` : une écoute par machine sur `127.0.0.1`, port stable enregistré dans `ohd.json`. Sous Linux, une seconde écoute sur l'adresse vue des conteneurs.
- L'URL du fournisseur pointe vers le proxy (`…/<fournisseur>/…`). opencode ne reçoit qu'un jeton `ohs_…`, dans la variable du fournisseur (par exemple `AWS_BEARER_TOKEN_BEDROCK`).
- **Un jeton par groupe de serveur**, et non par session, car l'environnement du processus `opencode serve` est commun aux sessions du groupe. Il est révoqué à la veille, à l'arrêt et à l'abandon du groupe ; sans jeton valide, le proxy répond 401.

### 2. Source des identifiants

- Cascade lue dans le trousseau : clé du projet, clé du fournisseur pour le projet, clé de l'équipe, clé du hub, puis, pour Bedrock, un profil AWS. Avec un profil, le proxy signe en SigV4, avec un cache par profil et par région.
- Une erreur du trousseau arrête la cascade : seul « non trouvé » fait passer au niveau suivant.
- Région : config d'oh, puis `AWS_REGION` / `AWS_DEFAULT_REGION`, puis profil, puis `us-east-1` avec un avertissement.
- Fournisseur, région, source et empreinte du secret entrent dans la clé de groupe : un changement démarre un nouveau serveur.

### 3. Comportement du proxy

- L'identifiant est appliqué dans le transport, sur la requête finale. Selon le fournisseur, l'en-tête est remplacé (Bearer Bedrock, `x-api-key` Anthropic, Bearer OpenAI et OpenRouter) ou la requête est signée en SigV4. Un échec renvoie 502 sans rien envoyer.
- Flux sans tampon.
- **Liste blanche des chemins d'inférence** par fournisseur, sinon 404. Chaque segment est décodé une seule fois ; `.` et `..` sont refusés.
- Corps limité à 64 Mio (413).
- Avec une liste de modèles active, la clé JSON `model` doit être exacte (doublon ou variante de casse : 400).
- L'usage est compté depuis les réponses, flux compris : Bedrock `converse-stream` et `invoke-with-response-stream` (base64), Anthropic, OpenAI avec `include_usage` ajouté.

### 4. Politique de fournisseur

La config rendue refuse `provider.use` pour tout fournisseur autre que celui de la session (`experimental.policies`) : plus de repli silencieux sur les modèles hébergés par opencode.

### 5. Stockage et sécurité du démon (M12)

- Jetons connus **par leur empreinte** SHA-256 seulement (`proxy_grants`, `servers`). Les valeurs anciennes sont converties au démarrage.
- Les jetons sont restaurés après l'ouverture du socket ; ceux qui sont orphelins sont révoqués.
- Si le port du proxy change, les groupes concernés sont mis en veille et une décision ✗ est levée pour les sessions qui travaillaient.
- Socket Unix en 0600, **UID du pair vérifié** (`LOCAL_PEERCRED`, `SO_PEERCRED`).
- Les routes qui émettent des jetons ou arrêtent le démon sont réservées à la CLI par une **capacité** (`X-Oh-Capability`, dans le trousseau, ou à défaut dans un fichier 0600), jamais mise dans un environnement.
- Les crochets du plugin et des passerelles (`/oh/v1/hooks/*`) sont authentifiés par le jeton du groupe.

### 6. Restrictions I6 (`internal/limits`), désactivées par défaut

- Restrictions disponibles : sessions actives max, budget par session et budget journalier en USD, plafond mémoire, liste de modèles.
- **Cascade** : workflow (`limits.budget_usd`, `limits.models`) > projet > hub (`[limits]`) > équipe recommandé. L'imposé d'équipe est un plafond.
- **Registre d'usage** (migration v40) : `usage_sessions` (par jour et par session, avec la session racine, pour compter le coût des sous-agents), `usage_proxy`, `budget_extra` (relèvements).
- **Dépassement**, contrôlé par le démon en fin d'étape :
  - une décision `$` est levée : relever, arrêter ou classer ;
  - une étape qui démarre pendant qu'une décision `$` est ouverte est interrompue.
- Au-delà du nombre de sessions actives ou du plafond mémoire, le premier prompt est mis en file. Budget journalier épuisé : nouvelle session refusée.
- Commandes : `oh budget show|set|unset|raise`, section Restrictions des réglages TUI, contrôle Doctor.

### 7. Windows

Le démon tourne dans le processus d'oh : le proxy s'arrête quand oh quitte, et les groupes sont mis en veille.

## Conséquences

### Positives

- Aucun secret dans l'environnement de l'outil : `env` dans le shell d'un agent ne montre que `ohs_…` (vérifié en e2e en local, en réel en conteneur, et dans un essai local du job distant).
- Révocation par groupe, comptage en un seul point, et plus de bascule silencieuse vers un autre fournisseur.
- La même mécanique sert aux crochets du plugin et aux passerelles ([ADR-046](./046-beads-gateways.fr.md)).
- Restrictions disponibles sans rien imposer par défaut.

### Négatives / Compromis

- En local, l'agent tourne sous le compte de l'utilisateur : il peut lire le trousseau ou lancer `oh` (documenté dans `SECURITY`). Le mot de passe des serveurs reste en clair dans `oh.db`.
- Un jeton par groupe : le coût par session vient de ce que rapporte opencode, pas du proxy.
- Les budgets sont des **plafonds souples** :
  - contrôlés en fin d'étape, une étape peut dépasser ;
  - un fork recompte le coût de l'historique copié ;
  - le plafond mémoire ne compte pas les conteneurs ;
  - deux lancements simultanés peuvent dépasser le maximum de sessions actives.
- Les profils AWS (SigV4) ne servent pas en CI : clé API uniquement à distance, et pas de registre d'usage dans le job (BL-21).
- La seconde écoute Linux n'est testée qu'en unitaire, et l'e2e du budget n'a pas été fait.

## Alternatives considérées

| Alternative | Rejetée car |
|---|---|
| Passer la clé à opencode (environnement ou config) | Visible par le shell de l'agent et transmise aux conteneurs. |
| Laisser opencode stocker la clé (`auth.json` par groupe) | Secret écrit sur disque, lisible par l'agent ; import piégeux (F14). |
| Un proxy externe (LiteLLM…) | Infrastructure en plus, secret ailleurs, et pas de lien avec les groupes ni avec les budgets d'oh. |
| Un jeton par session | L'environnement du serveur est partagé par toutes les sessions du groupe. |
| Budgets en tokens au proxy | Le premier choix ; remplacé par l'USD calculé depuis le coût rapporté par opencode, plus parlant et commun à tous les fournisseurs. |
| Couper un flux au dépassement | Casse une réponse en cours ; on préfère finir l'étape puis décider. |
