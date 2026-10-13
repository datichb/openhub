# Consignes pour les assistants de code

Ces consignes complètent CONVENTIONS.md pour le travail assisté.

## Pour les assistants

- Répondre en français ; le code, les identifiants et les commits restent en anglais.
- Lancer make lint et make test avant de déclarer une tâche terminée.
- Ne jamais modifier les fichiers de infra/ sans demande explicite.
- Proposer un plan avant toute modification qui touche plus de trois fichiers.
- Répondre en français ; le code, les identifiants et les commits restent en anglais. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Lancer make lint et make test avant de déclarer une tâche terminée. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Ne jamais modifier les fichiers de infra/ sans demande explicite. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- Proposer un plan avant toute modification qui touche plus de trois fichiers. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)

## Commandes utiles

- make test : tests unitaires de tous les modules.
- make test-integration : tests d'intégration (Docker requis).
- make lint : golangci-lint et eslint.
- make generate : mocks et clients OpenAPI.
- make test : tests unitaires de tous les modules. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- make test-integration : tests d'intégration (Docker requis). (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- make lint : golangci-lint et eslint. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
- make generate : mocks et clients OpenAPI. (rappel 1 : la règle s'applique aussi aux scripts, aux outils internes et aux prototypes qui partent en production.)
