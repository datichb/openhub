# Conventions du projet Atlas

Ce document fixe les règles de code et de revue du projet Atlas. Elles priment sur les habitudes personnelles ; une exception se discute en revue et se trace dans un ADR.

## Nommage

- Les fichiers Go sont en snake_case, les paquets en un seul mot minuscule.
- Les composants React sont en PascalCase, un composant par fichier, le fichier porte le nom du composant.
- Les hooks commencent par use et vivent dans src/hooks/ ; un hook par fichier.
- Les constantes exportées sont en PascalCase côté Go et en SCREAMING_SNAKE_CASE côté TypeScript.
- Les tables SQL sont au pluriel, en snake_case ; les colonnes de clé étrangère se terminent par _id.
- Les branches suivent le format feat/<ticket>-<résumé>, fix/<ticket>-<résumé> ou chore/<résumé>.
- Les fichiers Go sont en snake_case, les paquets en un seul mot minuscule. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Les composants React sont en PascalCase, un composant par fichier, le fichier porte le nom du composant. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Les hooks commencent par use et vivent dans src/hooks/ ; un hook par fichier. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Les constantes exportées sont en PascalCase côté Go et en SCREAMING_SNAKE_CASE côté TypeScript. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Les tables SQL sont au pluriel, en snake_case ; les colonnes de clé étrangère se terminent par _id. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Les branches suivent le format feat/<ticket>-<résumé>, fix/<ticket>-<résumé> ou chore/<résumé>. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)

## Architecture

- Le backend suit Controller → Service → Repository ; un contrôleur n'appelle jamais un dépôt directement.
- Les services reçoivent leurs dépendances par constructeur ; aucune variable globale mutable.
- Les erreurs métier sont typées (ErrNotFound, ErrConflict, ErrForbidden) et traduites en HTTP dans le contrôleur.
- Le frontend sépare les pages (src/pages), les composants partagés (src/components) et l'état serveur (TanStack Query).
- Toute communication entre services passe par la file d'événements ; pas d'appel HTTP synchrone entre services.
- Les migrations de schéma sont versionnées dans db/migrations et appliquées par l'outil de migration au démarrage.
- Le backend suit Controller → Service → Repository ; un contrôleur n'appelle jamais un dépôt directement. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Les services reçoivent leurs dépendances par constructeur ; aucune variable globale mutable. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Les erreurs métier sont typées (ErrNotFound, ErrConflict, ErrForbidden) et traduites en HTTP dans le contrôleur. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Le frontend sépare les pages (src/pages), les composants partagés (src/components) et l'état serveur (TanStack Query). (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Toute communication entre services passe par la file d'événements ; pas d'appel HTTP synchrone entre services. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Les migrations de schéma sont versionnées dans db/migrations et appliquées par l'outil de migration au démarrage. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)

## Tests

- Chaque correctif de bug arrive avec un test qui échoue avant le correctif.
- Les tests unitaires Go sont table-driven ; les sous-tests portent un nom lisible.
- Les tests d'intégration utilisent une base PostgreSQL éphémère (testcontainers), jamais la base partagée.
- Côté frontend, Vitest et Testing Library ; on teste le comportement visible, pas l'implémentation.
- La couverture minimale est de 80 % sur les paquets de domaine ; elle est vérifiée en CI.
- Les tests de bout en bout Playwright couvrent les parcours critiques : inscription, paiement, export.
- Chaque correctif de bug arrive avec un test qui échoue avant le correctif. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Les tests unitaires Go sont table-driven ; les sous-tests portent un nom lisible. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Les tests d'intégration utilisent une base PostgreSQL éphémère (testcontainers), jamais la base partagée. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Côté frontend, Vitest et Testing Library ; on teste le comportement visible, pas l'implémentation. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- La couverture minimale est de 80 % sur les paquets de domaine ; elle est vérifiée en CI. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Les tests de bout en bout Playwright couvrent les parcours critiques : inscription, paiement, export. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)

## Git et revue

- Les commits suivent Conventional Commits, en anglais, avec le ticket en pied de message.
- Une PR ne dépasse pas 400 lignes modifiées hors fichiers générés ; au-delà, on découpe.
- Deux approbations sont requises sur les dossiers billing/ et auth/ ; une ailleurs.
- On ne force jamais un push sur main ; on rebase sa branche avant la revue.
- Le message de PR décrit le pourquoi, le comment, les risques et la manière de tester.
- Les fichiers générés (mocks, clients OpenAPI) sont régénérés par make generate, jamais édités à la main.
- Les commits suivent Conventional Commits, en anglais, avec le ticket en pied de message. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Une PR ne dépasse pas 400 lignes modifiées hors fichiers générés ; au-delà, on découpe. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Deux approbations sont requises sur les dossiers billing/ et auth/ ; une ailleurs. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- On ne force jamais un push sur main ; on rebase sa branche avant la revue. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Le message de PR décrit le pourquoi, le comment, les risques et la manière de tester. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Les fichiers générés (mocks, clients OpenAPI) sont régénérés par make generate, jamais édités à la main. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)

## Sécurité

- Aucun secret dans le dépôt ; les secrets passent par le coffre et les variables d'environnement.
- Toute entrée utilisateur est validée à la frontière (schéma zod côté frontend, validateur côté backend).
- Les requêtes SQL sont paramétrées ; la concaténation de chaînes dans une requête est refusée en revue.
- Les journaux ne contiennent ni jeton, ni mot de passe, ni donnée personnelle en clair.
- Les dépendances sont mises à jour chaque semaine par le bot ; une faille critique se corrige sous 48 heures.
- Les en-têtes de sécurité (CSP, HSTS, X-Frame-Options) sont posés par le proxy, testés en CI.
- Aucun secret dans le dépôt ; les secrets passent par le coffre et les variables d'environnement. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Toute entrée utilisateur est validée à la frontière (schéma zod côté frontend, validateur côté backend). (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Les requêtes SQL sont paramétrées ; la concaténation de chaînes dans une requête est refusée en revue. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Les journaux ne contiennent ni jeton, ni mot de passe, ni donnée personnelle en clair. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Les dépendances sont mises à jour chaque semaine par le bot ; une faille critique se corrige sous 48 heures. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Les en-têtes de sécurité (CSP, HSTS, X-Frame-Options) sont posés par le proxy, testés en CI. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)

## Performance

- Une page ne charge pas plus de 200 Ko de JavaScript compressé au premier affichage.
- Les requêtes de liste sont paginées par curseur ; jamais de OFFSET au-delà de la première page.
- Les index sont ajoutés avec la requête qui les justifie, et leur plan d'exécution est joint à la PR.
- Le cache applicatif a une durée de vie explicite et une clé versionnée.
- Les images passent par le service de redimensionnement ; jamais d'image originale servie au client.
- Le budget de latence p95 de l'API publique est de 300 ms ; une régression bloque la mise en production.
- Une page ne charge pas plus de 200 Ko de JavaScript compressé au premier affichage. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Les requêtes de liste sont paginées par curseur ; jamais de OFFSET au-delà de la première page. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Les index sont ajoutés avec la requête qui les justifie, et leur plan d'exécution est joint à la PR. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Le cache applicatif a une durée de vie explicite et une clé versionnée. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Les images passent par le service de redimensionnement ; jamais d'image originale servie au client. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Le budget de latence p95 de l'API publique est de 300 ms ; une régression bloque la mise en production. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)

## Documentation

- Chaque décision d'architecture fait l'objet d'un ADR dans docs/adr, au format MADR.
- Le README de chaque service décrit son rôle, ses dépendances, son lancement local et ses variables.
- Les endpoints publics sont décrits en OpenAPI 3.1 ; la spécification fait foi sur le code.
- Le CHANGELOG suit Keep a Changelog ; une entrée par PR visible des utilisateurs.
- Le wiki docs/wiki est enrichi par les agents ; les tags de confiance y sont obligatoires.
- Les schémas sont en Mermaid, versionnés à côté du texte qui les explique.
